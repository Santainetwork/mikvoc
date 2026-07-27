package assets

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Kind string

const (
	Logo       Kind = "logo"
	Background Kind = "background"
)

type Asset struct {
	RouterID int
	Kind     Kind
	Ext      string
	Path     string
	Bytes    []byte
}

type Store struct{ root string }

func New(root string) *Store { return &Store{root: filepath.Clean(root)} }

func (s *Store) Write(routerID int, kind Kind, r io.Reader, maxBytes int64) (Asset, error) {
	if routerID < 0 || !validKind(kind) || maxBytes <= 0 {
		return Asset{}, fmt.Errorf("invalid asset scope")
	}
	limit := int64(1 << 20)
	if kind == Background {
		limit = 5 << 20
	}
	if maxBytes > limit {
		maxBytes = limit
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return Asset{}, err
	}
	if int64(len(data)) > maxBytes {
		return Asset{}, fmt.Errorf("asset exceeds byte limit")
	}
	ext, err := validate(data)
	if err != nil {
		return Asset{}, err
	}
	dir := s.dir(routerID)
	if err := s.ensureDirs(dir); err != nil {
		return Asset{}, err
	}
	if err := s.rejectEntry(dir); err != nil {
		return Asset{}, err
	}
	tmp, err := os.CreateTemp(dir, ".asset-")
	if err != nil {
		return Asset{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Asset{}, err
	}
	newPath := filepath.Join(dir, string(kind)+ext)
	if !contained(s.root, newPath) {
		return Asset{}, fmt.Errorf("path escapes store")
	}
	if err = os.Rename(tmpName, newPath); err != nil {
		return Asset{}, err
	}
	for _, old := range []string{".png", ".jpg", ".gif"} {
		if old != ext {
			_ = removeRegular(filepath.Join(dir, string(kind)+old))
		}
	}
	return Asset{RouterID: routerID, Kind: kind, Ext: ext, Bytes: data}, nil
}

func (s *Store) Read(routerID int, kind Kind) (Asset, error) {
	if routerID < 0 || !validKind(kind) {
		return Asset{}, fmt.Errorf("invalid asset scope")
	}
	dir := s.dir(routerID)
	if err := s.rejectEntry(dir); err != nil {
		return Asset{}, err
	}
	for _, ext := range []string{".png", ".jpg", ".gif"} {
		p := filepath.Join(dir, string(kind)+ext)
		st, err := os.Lstat(p)
		if err == nil {
			if !st.Mode().IsRegular() {
				return Asset{}, fmt.Errorf("asset is not regular")
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return Asset{}, err
			}
			return Asset{RouterID: routerID, Kind: kind, Ext: ext, Bytes: b}, nil
		}
		if !os.IsNotExist(err) {
			return Asset{}, err
		}
	}
	return Asset{}, os.ErrNotExist
}

func (s *Store) Remove(routerID int, kind Kind) error {
	if routerID < 0 || !validKind(kind) {
		return fmt.Errorf("invalid asset scope")
	}
	if err := s.rejectEntry(s.dir(routerID)); err != nil {
		return err
	}
	for _, ext := range []string{".png", ".jpg", ".gif"} {
		if err := removeRegular(filepath.Join(s.dir(routerID), string(kind)+ext)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RemoveRouter(routerID int) error {
	if routerID <= 0 {
		return fmt.Errorf("invalid router id")
	}
	p := s.dir(routerID)
	if err := s.rejectEntry(p); err != nil {
		return err
	}
	return os.RemoveAll(p)
}

func (s *Store) Walk() ([]Asset, error) {
	var out []Asset
	if err := s.rejectEntry(s.root); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(s.root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == s.root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in asset store")
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(s.root, p)
			if rel != "global" && rel != "routers" && !(strings.HasPrefix(filepath.ToSlash(rel), "routers/") && strings.Count(filepath.ToSlash(rel), "/") == 1 && canonicalID(filepath.Base(rel))) {
				return fmt.Errorf("unexpected asset directory")
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("unexpected non-regular asset")
		}
		rel, _ := filepath.Rel(s.root, p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 2 && len(parts) != 3 {
			return fmt.Errorf("unexpected asset path")
		}
		var id int
		filename := parts[len(parts)-1]
		kind := Kind(strings.TrimSuffix(filename, filepath.Ext(filename)))
		if len(parts) == 2 {
			if parts[0] != "global" {
				return fmt.Errorf("unexpected asset path")
			}
		} else {
			if parts[0] != "routers" {
				return fmt.Errorf("unexpected asset path")
			}
			if !canonicalID(parts[1]) {
				return fmt.Errorf("unexpected asset path")
			}
			if _, err := fmt.Sscan(parts[1], &id); err != nil || id <= 0 {
				return fmt.Errorf("unexpected asset path")
			}
		}
		if !validKind(kind) || (filepath.Ext(filename) != ".png" && filepath.Ext(filename) != ".jpg" && filepath.Ext(filename) != ".gif") {
			return fmt.Errorf("unexpected asset path")
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, Asset{RouterID: id, Kind: kind, Ext: filepath.Ext(p), Path: filepath.ToSlash(rel), Bytes: b})
		return nil
	})
	return out, err
}

func (s *Store) dir(id int) string {
	if id == 0 {
		return filepath.Join(s.root, "global")
	}
	return filepath.Join(s.root, "routers", fmt.Sprint(id))
}
func (s *Store) ensureDirs(dir string) error {
	if err := checkAncestors(s.root); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := checkAncestors(dir); err != nil {
		return err
	}
	for p := dir; contained(s.root, p); p = filepath.Dir(p) {
		if err := os.Chmod(p, 0700); err != nil {
			return err
		}
		if p == s.root {
			break
		}
	}
	return nil
}
func (s *Store) rejectEntry(p string) error {
	if !contained(s.root, p) {
		return fmt.Errorf("path escapes store")
	}
	st, err := os.Lstat(p)
	if err == nil && st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink scope")
	}
	return nil
}
func validKind(k Kind) bool { return k == Logo || k == Background }
func contained(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator))
}
func removeRegular(p string) error {
	st, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("asset is not regular")
	}
	return os.Remove(p)
}
func validate(b []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "png" && format != "jpeg" && format != "gif") {
		return "", fmt.Errorf("invalid image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
		return "", fmt.Errorf("image dimensions exceed limit")
	}
	if _, _, err := image.Decode(bytes.NewReader(b)); err != nil {
		return "", fmt.Errorf("invalid image")
	}
	if format == "jpeg" {
		return ".jpg", nil
	}
	return "." + format, nil
}

func checkAncestors(p string) error {
	p, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	for cur := p; ; cur = filepath.Dir(cur) {
		st, err := os.Lstat(cur)
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink ancestor")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil
		}
	}
}

func canonicalID(s string) bool {
	if s == "0" || s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
