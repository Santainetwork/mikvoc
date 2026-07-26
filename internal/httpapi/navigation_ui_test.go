package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestThemeAndSidebarPersistenceMarkup(t *testing.T) {
	head := readUIFile(t, "../../web/templates/partials/head.html")
	login := readUIFile(t, "../../web/templates/pages/login.html")
	layout := readUIFile(t, "../../web/templates/layouts/base.html")
	sidebar := readUIFile(t, "../../web/templates/partials/sidebar.html")
	mainJS := readUIFile(t, "../../web/static/js/main.js")

	if !strings.Contains(head, "mikvocTheme") {
		t.Fatal("head missing persistent theme key")
	}
	if !strings.Contains(login, "data-server-theme") || !strings.Contains(layout, "data-server-theme") {
		t.Fatal("root pages missing server theme fallback")
	}
	if strings.Contains(login, `<html lang="id" class="dark">`) {
		t.Fatal("login still forces dark theme")
	}
	for _, category := range []string{"main", "hotspot", "pppoe", "system"} {
		if !strings.Contains(sidebar, `data-sidebar-section="`+category+`"`) {
			t.Fatalf("sidebar missing category %q", category)
		}
	}
	if !strings.Contains(mainJS, "mikvocSidebarSections") {
		t.Fatal("main.js missing sidebar persistence key")
	}
}

func readUIFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
