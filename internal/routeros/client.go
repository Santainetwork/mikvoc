// Package api implements the MikroTik RouterOS API binary protocol client.
// Compatible with RouterOS v6.45+ and v7.10+.
package routeros

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Client is a thread-safe MikroTik RouterOS API client.
type Client struct {
	mu         sync.Mutex
	conn       net.Conn
	reader     *bufio.Reader
	addr       string
	connected  bool
	rosVersion string // detected RouterOS version
}

// NewClient creates a new API client (not connected yet).
func NewClient(ip, port string) *Client {
	return &Client{
		addr: net.JoinHostPort(ip, port),
	}
}

// Connect establishes TCP connection and logs in.
func (c *Client) Connect(username, password string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := net.DialTimeout("tcp", c.addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	c.conn = conn
	c.reader = bufio.NewReader(conn)

	// Login sequence
	reply, err := c.sendCommand([]string{"/login", "=name=" + username, "=password=" + password})
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	// RouterOS 6 uses challenge-response MD5 login
	if len(reply) > 0 {
		challenge := ""
		for _, sentence := range reply {
			for _, word := range sentence {
				if strings.HasPrefix(word, "=ret=") {
					challenge = strings.TrimPrefix(word, "=ret=")
				}
			}
		}
		if challenge != "" {
			// MD5 challenge login (RouterOS 6)
			md5hash := md5.New()
			md5hash.Write([]byte{0})
			md5hash.Write([]byte(password))
			b, _ := hex.DecodeString(challenge)
			md5hash.Write(b)
			hashStr := "00" + hex.EncodeToString(md5hash.Sum(nil))
			reply, err = c.sendCommand([]string{"/login", "=name=" + username, "=response=" + hashStr})
			if err != nil {
				return fmt.Errorf("login md5: %w", err)
			}
		}
	}

	// Verify login success
	for _, sentence := range reply {
		for _, word := range sentence {
			if word == "!trap" || strings.Contains(word, "invalid user") {
				return errors.New("login failed: invalid credentials")
			}
		}
	}

	c.connected = true
	conn.SetDeadline(time.Time{}) // clear deadline after login

	// Detect RouterOS version
	res, _ := c.run("/system/resource/print")
	for _, row := range res {
		if v, ok := row["version"]; ok {
			c.rosVersion = v
		}
	}
	return nil
}

// IsConnected returns true if client has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// ROSVersion returns the detected RouterOS version string.
func (c *Client) ROSVersion() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rosVersion
}

// Close closes the connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.connected = false
	}
}

// Run executes a RouterOS API command and returns parsed results.
// It is the primary method to interact with the router.
func (c *Client) Run(cmd string, params ...map[string]string) ([]map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.run(cmd, params...)
}

// SetFileContent sets the content of a file on the Mikrotik router.
// The filename is the full path, e.g. "hotspot/login.html".
func (c *Client) SetFileContent(filename, content string) error {
	if len(content) >= 4096 {
		return fmt.Errorf("ukuran file melebihi batas 4KB API Mikrotik (ukuran: %d bytes). Harap jangan gunakan base64 untuk logo.", len(content))
	}
	// Temukan file ID terlebih dahulu
	res, err := c.Run("/file/print", map[string]string{
		"?name": filename,
	})
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return errors.New("File tidak ditemukan: " + filename)
	}
	id := res[0][".id"]

	_, err = c.Run("/file/set", map[string]string{
		".id":      id,
		"contents": content,
	})
	return err
}

func routerOSCommandWords(cmd string, params ...map[string]string) []string {
	words := []string{cmd}
	if len(params) == 0 {
		return words
	}

	for k, v := range params[0] {
		key := strings.TrimPrefix(k, "=")
		if strings.HasPrefix(key, "?") {
			words = append(words, key+"="+v)
			continue
		}
		words = append(words, "="+key+"="+v)
	}
	return words
}

// run is the internal (non-locking) implementation.
func (c *Client) run(cmd string, params ...map[string]string) ([]map[string]string, error) {
	if !c.connected {
		return nil, errors.New("not connected to router")
	}

	words := routerOSCommandWords(cmd, params...)

	c.conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.conn.SetDeadline(time.Time{})

	sentences, err := c.sendCommand(words)
	if err != nil {
		c.connected = false
		return nil, err
	}

	var results []map[string]string
	for _, sentence := range sentences {
		row := make(map[string]string)
		isDone := false
		isTrap := false
		trapMsg := ""
		for _, word := range sentence {
			if word == "!done" {
				isDone = true
				continue
			}
			if word == "!trap" {
				isTrap = true
				continue
			}
			if word == "!re" {
				continue
			}
			if strings.HasPrefix(word, "=message=") && isTrap {
				trapMsg = strings.TrimPrefix(word, "=message=")
			}
			if strings.HasPrefix(word, "=") {
				parts := strings.SplitN(word[1:], "=", 2)
				if len(parts) == 2 {
					row[parts[0]] = parts[1]
				}
			}
		}
		if isTrap && trapMsg != "" {
			return nil, fmt.Errorf("router error: %s", trapMsg)
		}
		if len(row) > 0 {
			results = append(results, row)
		}
		_ = isDone
	}
	return results, nil
}

// sendCommand writes the API sentence and reads responses until !done or !trap.
func (c *Client) sendCommand(words []string) ([][]string, error) {
	// Write sentence
	var buf bytes.Buffer
	for _, word := range words {
		writeWord(&buf, word)
	}
	buf.Write([]byte{0}) // end of sentence
	if _, err := c.conn.Write(buf.Bytes()); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read reply sentences
	var sentences [][]string
	for {
		sentence, done, err := c.readSentence()
		if err != nil {
			return sentences, err
		}
		sentences = append(sentences, sentence)
		if done {
			break
		}
	}
	return sentences, nil
}

// readSentence reads one API sentence from the connection.
// Returns (words, isDoneOrTrap, error).
func (c *Client) readSentence() ([]string, bool, error) {
	var words []string
	done := false
	for {
		word, err := c.readWord()
		if err != nil {
			return words, false, err
		}
		if word == "" {
			break // end of sentence
		}
		words = append(words, word)
		if word == "!done" || word == "!trap" || word == "!fatal" {
			done = true
		}
	}
	return words, done, nil
}

// readWord reads one length-prefixed word from the API stream.
func (c *Client) readWord() (string, error) {
	length, err := c.readLength()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.reader, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readLength reads the variable-length word length prefix.
func (c *Client) readLength() (uint32, error) {
	b, err := c.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	switch {
	case b&0x80 == 0:
		return uint32(b), nil
	case b&0xC0 == 0x80:
		b2, err := c.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		return uint32(b&^0xC0)<<8 | uint32(b2), nil
	case b&0xE0 == 0xC0:
		buf := make([]byte, 2)
		if _, err := c.reader.Read(buf); err != nil {
			return 0, err
		}
		return uint32(b&^0xE0)<<16 | uint32(buf[0])<<8 | uint32(buf[1]), nil
	case b&0xF0 == 0xE0:
		buf := make([]byte, 3)
		if _, err := c.reader.Read(buf); err != nil {
			return 0, err
		}
		return uint32(b&^0xF0)<<24 | uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2]), nil
	default:
		buf := make([]byte, 4)
		if _, err := c.reader.Read(buf); err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint32(buf), nil
	}
}

// writeWord encodes a word with the RouterOS length prefix into buf.
func writeWord(buf *bytes.Buffer, word string) {
	l := len(word)
	switch {
	case l < 0x80:
		buf.WriteByte(byte(l))
	case l < 0x4000:
		buf.WriteByte(byte((l >> 8) | 0x80))
		buf.WriteByte(byte(l))
	case l < 0x200000:
		buf.WriteByte(byte((l >> 16) | 0xC0))
		buf.WriteByte(byte(l >> 8))
		buf.WriteByte(byte(l))
	default:
		buf.WriteByte(byte((l >> 24) | 0xE0))
		buf.WriteByte(byte(l >> 16))
		buf.WriteByte(byte(l >> 8))
		buf.WriteByte(byte(l))
	}
	buf.WriteString(word)
}
