package httpapi

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	body []byte
	mode os.FileMode
}

func TestValidateAssetZip(t *testing.T) {
	t.Run("accepts flat files in archive order", func(t *testing.T) {
		raw := buildZip(t,
			zipEntry{name: "login.html", body: []byte("login")},
			zipEntry{name: "css/site.css", body: []byte("css")},
			zipEntry{name: "img/logo.png", body: []byte("png")},
		)

		got, err := validateAssetZip(raw)
		if err != nil {
			t.Fatalf("validateAssetZip() error = %v", err)
		}
		if len(got.Files) != 3 {
			t.Fatalf("validateAssetZip() returned %d files, want 3", len(got.Files))
		}
		wantNames := []string{"login.html", "css/site.css", "img/logo.png"}
		for i, want := range wantNames {
			if got.Files[i].Name != want {
				t.Fatalf("file %d name = %q, want %q", i, got.Files[i].Name, want)
			}
			if got.Files[i].Asset != (want != "login.html") {
				t.Fatalf("file %q Asset = %v", want, got.Files[i].Asset)
			}
		}
	})

	t.Run("strips one shared top directory", func(t *testing.T) {
		raw := buildZip(t,
			zipEntry{name: "theme/", mode: os.ModeDir | 0755},
			zipEntry{name: "theme/login.html", body: []byte("login")},
			zipEntry{name: "theme/status.html", body: []byte("status")},
			zipEntry{name: "theme/logout.html", body: []byte("logout")},
			zipEntry{name: "theme/app.js", body: []byte("js")},
		)

		got, err := validateAssetZip(raw)
		if err != nil {
			t.Fatalf("validateAssetZip() error = %v", err)
		}
		wantNames := []string{"login.html", "status.html", "logout.html", "app.js"}
		if len(got.Files) != len(wantNames) {
			t.Fatalf("validateAssetZip() returned %d files, want %d", len(got.Files), len(wantNames))
		}
		for i, want := range wantNames {
			if got.Files[i].Name != want {
				t.Fatalf("file %d name = %q, want %q", i, got.Files[i].Name, want)
			}
			if got.Files[i].Asset != (want == "app.js") {
				t.Fatalf("file %q Asset = %v", want, got.Files[i].Asset)
			}
		}
	})

	t.Run("requires login", func(t *testing.T) {
		assertZipRejected(t, buildZip(t, zipEntry{name: "status.html", body: []byte("status")}))
	})

	for _, name := range []string{
		"../login.html",
		"/login.html",
		`C:\login.html`,
		"dir/../login.html",
		`..\login.html`,
		`dir\..\login.html`,
		`\login.html`,
		`\\server\share\login.html`,
	} {
		name := name
		t.Run("rejects unsafe path "+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			assertZipRejected(t, buildZip(t, zipEntry{name: name, body: []byte("login")}))
		})
	}

	t.Run("does not strip mixed top directories", func(t *testing.T) {
		assertZipRejected(t, buildZip(t,
			zipEntry{name: "theme/login.html", body: []byte("login")},
			zipEntry{name: "other/style.css", body: []byte("css")},
		))
	})

	t.Run("rejects hotspot prefix after shared root normalization", func(t *testing.T) {
		for _, name := range []string{"hotspot/site.css", "HotSpot/site.css"} {
			assertZipRejected(t, buildZip(t,
				zipEntry{name: "login.html", body: []byte("login")},
				zipEntry{name: name, body: []byte("css")},
			))
		}
	})

	t.Run("accepts hotspot as one shared top directory", func(t *testing.T) {
		got, err := validateAssetZip(buildZip(t,
			zipEntry{name: "hotspot/login.html", body: []byte("login")},
			zipEntry{name: "hotspot/css/site.css", body: []byte("css")},
		))
		if err != nil {
			t.Fatalf("validateAssetZip() error = %v", err)
		}
		if len(got.Files) != 2 || got.Files[0].Name != "login.html" || got.Files[1].Name != "css/site.css" {
			t.Fatalf("normalized files = %#v", got.Files)
		}
	})

	for _, ext := range []string{"php", "sh", "exe", "zip"} {
		ext := ext
		t.Run("rejects "+ext, func(t *testing.T) {
			assertZipRejected(t, buildZip(t,
				zipEntry{name: "login.html", body: []byte("login")},
				zipEntry{name: "nested/payload." + ext, body: []byte("payload")},
			))
		})
	}

	t.Run("rejects more than 100 entries including directories", func(t *testing.T) {
		entries := []zipEntry{{name: "login.html", body: []byte("login")}}
		for i := 0; i < maxAssetEntries; i++ {
			entries = append(entries, zipEntry{name: "dir" + itoa(i) + "/", mode: os.ModeDir | 0755})
		}
		assertZipRejected(t, buildZip(t, entries...))
	})

	t.Run("rejects more than 8 MiB uncompressed", func(t *testing.T) {
		assertZipRejected(t, buildZip(t,
			zipEntry{name: "login.html", body: []byte("login")},
			zipEntry{name: "large.txt", body: bytes.Repeat([]byte("a"), maxAssetUncompressedBytes)},
		))
	})

	t.Run("rejects more than 2 MiB raw", func(t *testing.T) {
		raw := append(buildZip(t, zipEntry{name: "login.html", body: []byte("login")}), make([]byte, maxAssetCompressedBytes+1)...)
		assertZipRejected(t, raw)
	})

	t.Run("rejects malformed zip", func(t *testing.T) {
		assertZipRejected(t, []byte("not a zip"))
	})

	t.Run("rejects duplicate normalized names", func(t *testing.T) {
		assertZipRejected(t, buildZip(t,
			zipEntry{name: "theme/login.html", body: []byte("first")},
			zipEntry{name: `theme\login.html`, body: []byte("second")},
		))
	})

	t.Run("rejects case insensitive duplicate names", func(t *testing.T) {
		assertZipRejected(t, buildZip(t,
			zipEntry{name: "login.html", body: []byte("login")},
			zipEntry{name: "css/App.css", body: []byte("first")},
			zipEntry{name: "css/app.css", body: []byte("second")},
		))
	})

	for _, tc := range []struct {
		name string
		mode os.FileMode
	}{
		{name: "symlink", mode: os.ModeSymlink | 0777},
		{name: "device", mode: os.ModeDevice | 0600},
		{name: "fifo", mode: os.ModeNamedPipe | 0600},
		{name: "socket", mode: os.ModeSocket | 0600},
	} {
		tc := tc
		t.Run("rejects non-regular "+tc.name, func(t *testing.T) {
			raw := buildZip(t,
				zipEntry{name: "login.html", body: []byte("login")},
				zipEntry{name: "special.txt", body: []byte("special"), mode: tc.mode},
			)
			zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
			if err != nil {
				t.Fatalf("NewReader(): %v", err)
			}
			if got := zr.File[1].FileInfo().Mode() & os.ModeType; got != tc.mode&os.ModeType {
				t.Skipf("archive/zip mode roundtrip = %v, want %v", got, tc.mode&os.ModeType)
			}
			assertZipRejected(t, raw)
		})
	}

	for _, name := range []string{
		"bad\x00name/login.html",
		"bad\nname/login.html",
		"bad\rname/login.html",
		"bad\x1fname/login.html",
		"bad\x7fname/login.html",
		"bad\xffname/login.html",
	} {
		name := name
		t.Run("rejects invalid path bytes "+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			assertZipRejected(t, buildZip(t, zipEntry{name: name, body: []byte("login")}))
		})
	}

	t.Run("accepts case insensitive extensions", func(t *testing.T) {
		got, err := validateAssetZip(buildZip(t,
			zipEntry{name: "login.html", body: []byte("login")},
			zipEntry{name: "LOGO.PNG", body: []byte("png")},
		))
		if err != nil {
			t.Fatalf("validateAssetZip() error = %v", err)
		}
		if len(got.Files) != 2 || got.Files[1].Name != "LOGO.PNG" {
			t.Fatalf("validateAssetZip() files = %#v", got.Files)
		}
	})

	t.Run("classifies uppercase status and logout as standard files", func(t *testing.T) {
		got, err := validateAssetZip(buildZip(t,
			zipEntry{name: "login.html", body: []byte("login")},
			zipEntry{name: "STATUS.HTML", body: []byte("status")},
			zipEntry{name: "Logout.Html", body: []byte("logout")},
		))
		if err != nil {
			t.Fatalf("validateAssetZip() error = %v", err)
		}
		if got.Files[1].Asset || got.Files[2].Asset {
			t.Fatalf("uppercase standard flags = [%v, %v], want false", got.Files[1].Asset, got.Files[2].Asset)
		}
	})
}

func assertZipRejected(t *testing.T, raw []byte) {
	t.Helper()
	if _, err := validateAssetZip(raw); err == nil {
		t.Fatal("validateAssetZip() accepted invalid archive")
	}
}

func buildZip(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q): %v", entry.name, err)
		}
		if _, err := w.Write(entry.body); err != nil {
			t.Fatalf("Write(%q): %v", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close ZIP: %v", err)
	}
	return buf.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
