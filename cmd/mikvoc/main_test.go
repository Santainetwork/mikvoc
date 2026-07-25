package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mikvoc/internal/core"
)

func TestWriteVersion(t *testing.T) {
	var out bytes.Buffer
	writeVersion(&out)
	if got, want := out.String(), "MikVoc v"+core.Version+"\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestProfileCloneRouteIsRegistered(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "httpapi", "server.go")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	src := string(b)
	for _, want := range []string{`/hotspot/profiles/clone`, `HandleProfileClone`, `Methods(http.MethodPost)`} {
		if !strings.Contains(src, want) {
			t.Fatalf("expected server.go to contain %q", want)
		}
	}
}
