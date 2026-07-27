package assets

import (
	"bytes"
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
