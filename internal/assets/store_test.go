package assets

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreAcceptsImagesAndIsolatesScopes(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "db.sqlite.assets"))
	for _, tc := range []struct {
		kind Kind
		name string
		make func(*bytes.Buffer) error
	}{
		{Logo, "png", func(b *bytes.Buffer) error { return png.Encode(b, testImage(2, 3)) }},
		{Background, "jpeg", func(b *bytes.Buffer) error { return jpeg.Encode(b, testImage(2, 3), nil) }},
		{Logo, "gif", func(b *bytes.Buffer) error { return gif.Encode(b, testImage(2, 3), nil) }},
	} {
		var b bytes.Buffer
		if err := tc.make(&b); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Write(0, tc.kind, bytes.NewReader(b.Bytes()), 1<<20); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if _, err := s.Write(7, Logo, bytes.NewReader([]byte("x")), 1<<20); err == nil {
		t.Fatal("router write unexpectedly accepted invalid image")
	}
	if _, err := s.Read(7, Background); err == nil {
		t.Fatal("scopes not isolated")
	}
}

func TestStoreRejectsContentAndLimits(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "assets"))
	for name, data := range map[string][]byte{
		"webp": []byte("RIFFxxxxWEBP"), "svg": []byte("<svg></svg>"), "malformed": {1, 2, 3}, "empty": nil,
	} {
		if _, err := s.Write(0, Logo, bytes.NewReader(data), 1<<20); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, testImage(2, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(0, Logo, bytes.NewReader(b.Bytes()), int64(b.Len()-1)); err == nil {
		t.Fatal("byte limit not enforced")
	}
}

func TestStoreEnforcesIntrinsicKindLimits(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "assets"))
	for _, tc := range []struct {
		kind  Kind
		limit int64
		data  []byte
	}{
		{Logo, 1 << 20, noisyPNG(t, 2048, 2048)}, {Background, 5 << 20, noisyPNG(t, 2048, 2048)},
	} {
		if int64(len(tc.data)) <= tc.limit {
			t.Fatalf("fixture too small: %d", len(tc.data))
		}
		if _, err := s.Write(0, tc.kind, bytes.NewReader(tc.data), 1<<30); err == nil {
			t.Errorf("%s intrinsic limit not enforced", tc.kind)
		}
	}
	if _, err := s.Write(0, Kind("other"), bytes.NewReader([]byte("x")), 1<<30); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestStoreRejectsDimensionAndPixelLimits(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "assets"))
	for _, data := range [][]byte{largePNG(t, 4097, 1), largePNG(t, 4097, 4097)} {
		if _, err := s.Write(0, Logo, bytes.NewReader(data), 1<<20); err == nil {
			t.Fatal("oversized dimensions accepted")
		}
	}
}

func TestStoreSetsSecureModes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets")
	s := New(root)
	var b bytes.Buffer
	if err := png.Encode(&b, testImage(2, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(4, Logo, bytes.NewReader(b.Bytes()), 1<<20); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{root, filepath.Join(root, "routers"), filepath.Join(root, "routers", "4"), filepath.Join(root, "routers", "4", "logo.png")} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != map[bool]os.FileMode{true: 0700, false: 0600}[st.IsDir()] {
			t.Errorf("%s mode %o", p, st.Mode().Perm())
		}
	}
}

func TestStoreRejectsParentSymlinkAndUnexpectedWalkEntries(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := New(filepath.Join(link, "assets")).Write(0, Logo, strings.NewReader("bad"), 1<<20); err == nil {
		t.Fatal("parent symlink accepted")
	}
	root := filepath.Join(t.TempDir(), "assets")
	s := New(root)
	if err := os.MkdirAll(filepath.Join(root, "routers", "3junk"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "routers", "3junk", "logo.png"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Walk(); err == nil {
		t.Fatal("noncanonical router path accepted")
	}
	if err := os.RemoveAll(filepath.Join(root, "routers", "3junk")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "global"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "global", "fifo"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Walk(); err == nil {
		t.Fatal("unexpected directory accepted")
	}
}

func TestStoreReplacementFailurePreservesPriorAndRemoveWalk(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets")
	s := New(root)
	var b bytes.Buffer
	if err := png.Encode(&b, testImage(2, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(3, Logo, bytes.NewReader(b.Bytes()), 1<<20); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(3, Logo, strings.NewReader("bad"), 1<<20); err == nil {
		t.Fatal("bad replacement accepted")
	}
	a, err := s.Read(3, Logo)
	if err != nil || !bytes.Equal(a.Bytes, b.Bytes()) {
		t.Fatalf("prior asset lost: %v", err)
	}
	files, err := s.Walk()
	if err != nil || len(files) != 1 || files[0].Path != "routers/3/logo.png" || !bytes.Equal(files[0].Bytes, b.Bytes()) {
		t.Fatalf("walk: %+v, %v", files, err)
	}
	if err := s.Remove(3, Logo); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveRouter(3); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectsSymlinks(t *testing.T) {
	root := filepath.Join(t.TempDir(), "assets")
	s := New(root)
	if err := os.MkdirAll(filepath.Join(root, "global"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), filepath.Join(root, "global", "logo.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(0, Logo); err == nil {
		t.Fatal("symlink read accepted")
	}
	if _, err := s.Walk(); err == nil {
		t.Fatal("symlink walk accepted")
	}
}

func testImage(w, h int) image.Image {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	m.Set(0, 0, color.White)
	return m
}

func largePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, testImage(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func noisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	pixels := make([]byte, w*h*3)
	if _, err := rand.Read(pixels); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			m.SetRGBA(x, y, color.RGBA{pixels[i], pixels[i+1], pixels[i+2], 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
