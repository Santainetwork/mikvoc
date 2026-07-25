package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
)

func readPackageZip(t *testing.T, raw []byte) ([]string, map[string][]byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open ZIP: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	contents := make(map[string][]byte, len(zr.File))
	for _, file := range zr.File {
		r, err := file.Open()
		if err != nil {
			t.Fatalf("open %q: %v", file.Name, err)
		}
		body, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("read %q: %v", file.Name, err)
		}
		names = append(names, file.Name)
		contents[file.Name] = body
	}
	return names, contents
}

func TestBuildTemplateZipUsesRelativeNamesAndPreservesContent(t *testing.T) {
	binary := []byte{0x00, 0xff, 0x89, 0x50, 0x4e, 0x47}
	set := templateFileSet{Files: []templateFile{
		{Name: "login.html", Content: []byte("login")},
		{Name: "status.html", Content: []byte("status")},
		{Name: "logout.html", Content: []byte("logout")},
		{Name: "css/site.css", Content: binary},
	}}
	raw, err := buildTemplateZip(set)
	if err != nil {
		t.Fatalf("buildTemplateZip() error = %v", err)
	}
	names, contents := readPackageZip(t, raw)
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	wantSorted := []string{"css/site.css", "login.html", "logout.html", "status.html"}
	if strings.Join(sorted, "|") != strings.Join(wantSorted, "|") {
		t.Fatalf("ZIP names = %v, want %v", sorted, wantSorted)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "hotspot/") {
			t.Fatalf("ZIP name has hotspot prefix: %q", name)
		}
	}
	if !bytes.Equal(contents["css/site.css"], binary) || string(contents["login.html"]) != "login" {
		t.Fatalf("ZIP content changed: %#v", contents)
	}
}

func TestBuildTemplateZipPreservesSetOrder(t *testing.T) {
	set := templateFileSet{Files: []templateFile{
		{Name: "z.js", Content: []byte("z")},
		{Name: "login.html", Content: []byte("login")},
		{Name: "a.css", Content: []byte("a")},
	}}
	raw, err := buildTemplateZip(set)
	if err != nil {
		t.Fatal(err)
	}
	names, _ := readPackageZip(t, raw)
	if got, want := strings.Join(names, "|"), "z.js|login.html|a.css"; got != want {
		t.Fatalf("ZIP order = %q, want %q", got, want)
	}
}

func TestBuildTemplateZipRejectsUnsafeAndDuplicateNames(t *testing.T) {
	for _, set := range []templateFileSet{
		{Files: []templateFile{{Name: "../login.html"}}},
		{Files: []templateFile{{Name: "login.html"}, {Name: "LOGIN.HTML"}}},
		{Files: []templateFile{{Name: "hotspot/login.html"}}},
		{Files: []templateFile{{Name: "HOTSPOT/css/a.css"}}},
	} {
		if _, err := buildTemplateZip(set); err == nil {
			t.Fatalf("buildTemplateZip(%v) accepted invalid set", set.Files)
		}
	}
}

func TestHandleTemplateDownloadBuildsGlobalCustomPackageWithoutRouter(t *testing.T) {
	withTestDB(t)
	middleware.InitSession("test-secret")
	assetZip := buildZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "css/site.css", body: []byte{0x00, 0xff, 0x01}},
	)
	for key, value := range map[string]string{
		"tpl_variant":           "custom",
		"tpl_custom_login_html": validCustomLogin,
		"tpl_custom_assets_zip": base64.StdEncoding.EncodeToString(assetZip),
	} {
		if err := database.SetSetting(key, value); err != nil {
			t.Fatal(err)
		}
	}

	app := &App{}
	rec := httptest.NewRecorder()
	app.HandleTemplateDownload(rec, httptest.NewRequest(http.MethodGet, "/template/download", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("Content-Length = %q, want %d", got, rec.Body.Len())
	}
	disposition := rec.Header().Get("Content-Disposition")
	if !regexp.MustCompile(`^attachment; filename="mikvoc-hotspot-[0-9]{8}-[0-9]{6}\.zip"$`).MatchString(disposition) {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
	if strings.ContainsAny(disposition, "/\\\r\n") {
		t.Fatalf("Content-Disposition contains unsafe character: %q", disposition)
	}
	names, _ := readPackageZip(t, rec.Body.Bytes())
	if got, want := strings.Join(names, "|"), "login.html|status.html|logout.html|css/site.css"; got != want {
		t.Fatalf("ZIP entries = %q, want %q", got, want)
	}
}

func TestHandleTemplateDownloadRedirectsInvalidCustomTemplate(t *testing.T) {
	withTestDB(t)
	middleware.InitSession("test-secret")
	if err := database.SetSetting("tpl_variant", "custom"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/template/download", nil)
	(&App{}).HandleTemplateDownload(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/template" {
		t.Fatalf("response = %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("Content-Type") == "application/zip" {
		t.Fatal("invalid custom template returned ZIP")
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("invalid custom template did not set session cookie")
	}
	flashReq := httptest.NewRequest(http.MethodGet, "/template", nil)
	for i := len(cookies) - 1; i >= 0; i-- {
		if cookies[i].Name == middleware.SessionName {
			flashReq.AddCookie(cookies[i])
			break
		}
	}
	sess, err := middleware.Store.Get(flashReq, middleware.SessionName)
	if err != nil {
		t.Fatalf("decode flash session: %v", err)
	}
	flash, _ := sess.Values["flash"].(string)
	if !strings.Contains(flash, "Gagal membuat paket template:") || !strings.Contains(flash, "login.html") {
		t.Fatalf("flash = %q", flash)
	}
}

func TestTemplateDownloadRouteIsProtectedGET(t *testing.T) {
	middleware.InitSession("template-download-route-test-secret")
	router := mux.NewRouter()
	(&App{}).RegisterRoutes(router)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/template/download", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated GET = %d Location %q, want %d Location /login", rec.Code, rec.Header().Get("Location"), http.StatusSeeOther)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/template/download", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
