package httpapi

import (
	"strings"
	"testing"
)

func TestFileSetGet(t *testing.T) {
	files := templateFileSet{Files: []templateFile{
		{Name: "login.html", Content: []byte("login")},
		{Name: "status.html", Content: []byte("status")},
	}}

	if got := string(files.Get("status.html")); got != "status" {
		t.Fatalf("Get(status.html) = %q, want status", got)
	}
	if got := files.Get("missing.html"); got != nil {
		t.Fatalf("Get(missing.html) = %q, want nil", got)
	}
}

func TestFileSetHasAssets(t *testing.T) {
	plain := templateFileSet{Files: []templateFile{{Name: "login.html", Content: []byte("login")}}}
	if plain.HasAssets() {
		t.Fatal("asset-free file set reported assets")
	}

	withAsset := templateFileSet{Files: []templateFile{
		{Name: "login.html", Content: []byte("login")},
		{Name: "logo.png", Content: []byte("png"), Asset: true},
	}}
	if !withAsset.HasAssets() {
		t.Fatal("file set with asset reported no assets")
	}
}

func TestFileSetOversizedPreservesOrderAndRejectsBoundary(t *testing.T) {
	files := templateFileSet{Files: []templateFile{
		{Name: "small.html", Content: make([]byte, routerFileContentLimit-1)},
		{Name: "login.html", Content: make([]byte, routerFileContentLimit)},
		{Name: "status.html", Content: make([]byte, routerFileContentLimit+10)},
	}}

	got := files.Oversized()
	if len(got) != 2 {
		t.Fatalf("Oversized() returned %d files, want 2", len(got))
	}
	if got[0].Name != "login.html" || got[1].Name != "status.html" {
		t.Fatalf("Oversized() order = [%q, %q], want [login.html, status.html]", got[0].Name, got[1].Name)
	}
}

func TestFileSetPushCheckAcceptsSmallAssetFreeFiles(t *testing.T) {
	files := templateFileSet{Files: []templateFile{{Name: "login.html", Content: []byte("login")}}}
	if err := files.PushCheck(); err != nil {
		t.Fatalf("PushCheck() = %v, want nil", err)
	}
}

func TestFileSetPushCheckRejectsOversizedFile(t *testing.T) {
	files := templateFileSet{Files: []templateFile{
		{Name: "status.html", Content: make([]byte, routerFileContentLimit+10)},
		{Name: "logout.html", Content: make([]byte, routerFileContentLimit+20)},
	}}
	err := files.PushCheck()
	if err == nil {
		t.Fatal("PushCheck() accepted oversized file")
	}
	message := err.Error()
	for _, want := range []string{"status.html", "4106", "4096", "ZIP"} {
		if !strings.Contains(message, want) {
			t.Fatalf("PushCheck() error %q does not mention %q", message, want)
		}
	}
	if strings.Contains(message, "logout.html") {
		t.Fatalf("PushCheck() selected the second oversized file: %q", message)
	}
}

func TestFileSetPushCheckRejectsAssetsBeforeOversizedFiles(t *testing.T) {
	files := templateFileSet{Files: []templateFile{
		{Name: "login.html", Content: make([]byte, routerFileContentLimit)},
		{Name: "logo.png", Content: []byte("png"), Asset: true},
	}}
	err := files.PushCheck()
	if err == nil {
		t.Fatal("PushCheck() accepted assets")
	}
	message := err.Error()
	for _, want := range []string{"Unduh Paket ZIP", "Winbox/WebFig"} {
		if !strings.Contains(message, want) {
			t.Fatalf("PushCheck() error %q does not mention %q", message, want)
		}
	}
	if strings.Contains(message, "login.html") {
		t.Fatalf("PushCheck() checked oversized files before assets: %q", message)
	}
}
