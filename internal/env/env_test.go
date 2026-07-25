package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetsVars(t *testing.T) {
	os.Unsetenv("MIKVOC_PORT")
	os.Unsetenv("MIKVOC_DB")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("MIKVOC_PORT=9090\nMIKVOC_DB=/tmp/test.db\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MIKVOC_PORT"); got != "9090" {
		t.Fatalf("MIKVOC_PORT = %q", got)
	}
	if got := os.Getenv("MIKVOC_DB"); got != "/tmp/test.db" {
		t.Fatalf("MIKVOC_DB = %q", got)
	}
}

func TestLoadDoesNotOverrideExisting(t *testing.T) {
	os.Setenv("MIKVOC_PORT", "7777")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("MIKVOC_PORT=9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MIKVOC_PORT"); got != "7777" {
		t.Fatalf("expected existing env 7777, got %q", got)
	}
	os.Unsetenv("MIKVOC_PORT")
}

func TestLoadMissingFileIsNoop(t *testing.T) {
	if err := Load("/nonexistent/.env"); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
}

func TestLoadStripsQuotesAndComments(t *testing.T) {
	os.Unsetenv("MIKVOC_QUOTED")
	os.Unsetenv("MIKVOC_COMMENTED")
	os.Unsetenv("MIKVOC_SKIP")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `# comment line
MIKVOC_QUOTED="value with spaces"
MIKVOC_COMMENTED=simple
#MIKVOC_SKIP=this
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MIKVOC_QUOTED"); got != "value with spaces" {
		t.Fatalf("quoted = %q", got)
	}
	if got := os.Getenv("MIKVOC_COMMENTED"); got != "simple" {
		t.Fatalf("simple = %q", got)
	}
	if got := os.Getenv("MIKVOC_SKIP"); got != "" {
		t.Fatalf("commented line should not set var, got %q", got)
	}
}
