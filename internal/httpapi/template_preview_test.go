package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"mikvoc/internal/middleware"
)

func withoutSessionStore(t *testing.T) {
	t.Helper()
	old := middleware.Store
	middleware.Store = nil
	t.Cleanup(func() { middleware.Store = old })
}

func initSessionForTest(t *testing.T, secret string) {
	t.Helper()
	old := middleware.Store
	middleware.InitSession(secret)
	t.Cleanup(func() { middleware.Store = old })
}

func TestPreviewHotspotHTMLReplacesVariablesAndConditionals(t *testing.T) {
	input := `$(if error)<p>$(error)</p>$(endif)$(if chap-id)<script>$(chap-id) $(chap-challenge)</script>$(endif)<form action="$(link-login-only)"><input value="$(link-orig)"><b>$(username) $(ip) $(mac) $(uptime) $(bytes-in-nice) $(bytes-out-nice)</b><a href="$(link-login)">login</a><a href="$(link-logout)">logout</a>`
	got := previewHotspotHTML(input)

	for _, want := range []string{"Username atau password salah", "preview-user", "192.168.10.2", "AA:BB:CC:DD:EE:FF", "1h23m", "12.3 MiB", "4.5 MiB"} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview missing sample %q: %s", want, got)
		}
	}
	if strings.Contains(got, "$(") || strings.Contains(got, "<!--") || strings.Contains(got, "-->") {
		t.Fatalf("preview left variable or malformed conditional comments: %s", got)
	}
}

func TestPreviewHotspotHTMLRemovesMultipleConditionalTokensWithoutComments(t *testing.T) {
	got := previewHotspotHTML(`$(if error)error$(endif)$(if chap-id)chap$(else)plain$(endif)$(if login-by == 'mac')mac$(endif)`)
	if got != "errorchapplainmac" {
		t.Fatalf("conditional preview = %q", got)
	}
}

func TestHandleTemplatePreviewReturnsSandboxedShell(t *testing.T) {
	rec := httptest.NewRecorder()
	(&App{}).HandleTemplatePreview(rec, httptest.NewRequest(http.MethodGet, "/template/preview", nil))
	body := rec.Body.String()

	for _, want := range []string{`<title>Preview Template Hotspot</title>`, `src="/template/preview/frame"`, `sandbox="allow-scripts allow-forms"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("shell missing %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"allow-same-origin", "allow-top-navigation", "allow-popups", "allow-downloads"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("shell contains forbidden sandbox capability %q", forbidden)
		}
	}
}

func TestTemplatePreviewFrameRequiresTemplateService(t *testing.T) {
	rec := httptest.NewRecorder()
	(&App{}).HandleTemplatePreviewFrame(rec, httptest.NewRequest(http.MethodGet, "/template/preview/frame", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestTemplatePreviewAssetRequiresTemplateService(t *testing.T) {
	rec := httptest.NewRecorder()
	req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/template/preview/assets/site.css", nil), map[string]string{"path": "site.css"})
	(&App{}).HandleTemplatePreviewAsset(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleTemplatePreviewFrameSetsHeadersAndRendersSamples(t *testing.T) {
	withTestDB(t)
	withoutSessionStore(t)
	rec := httptest.NewRecorder()
	rec.Header().Set("X-Frame-Options", "DENY")
	templateTestApp().HandleTemplatePreviewFrame(rec, httptest.NewRequest(http.MethodGet, "/template/preview/frame", nil))

	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("X-Frame-Options = %q, want SAMEORIGIN", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline' https: http:; script-src 'unsafe-inline' https: http:; img-src data: blob: https: http:; font-src data: https: http:; form-action 'none'; frame-ancestors 'self'" {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if strings.Contains(rec.Body.String(), "$(") {
		t.Fatalf("frame left MikroTik variables: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `<base href="/template/preview/assets/">`) {
		t.Fatalf("frame missing asset base: %s", rec.Body.String())
	}
}

func TestHandleTemplatePreviewFrameShowsInvalidCustomTemplateError(t *testing.T) {
	withTestDB(t)
	withoutSessionStore(t)
	if err := setTestSetting("tpl_variant", "custom"); err != nil {
		t.Fatal(err)
	}
	if err := setTestSetting("tpl_custom_login_html", `<p>broken</p>`); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	templateTestApp().HandleTemplatePreviewFrame(rec, httptest.NewRequest(http.MethodGet, "/template/preview/frame", nil))
	if !strings.Contains(rec.Body.String(), "Template custom tidak valid") {
		t.Fatalf("invalid custom error not visible: %s", rec.Body.String())
	}
}

func TestInjectPreviewAssetBase(t *testing.T) {
	const base = `<base href="/template/preview/assets/">`
	for _, tc := range []struct {
		name, input, want string
	}{
		{"head", `<html><head><title>x</title></head><body>body</body></html>`, `<html><head>` + base + `<title>x</title></head><body>body</body></html>`},
		{"uppercase head", `<HTML><HEAD></HEAD><BODY>x</BODY></HTML>`, `<HTML><HEAD>` + base + `</HEAD><BODY>x</BODY></HTML>`},
		{"head attributes", `<html lang="id"><head data-theme="x"><title>x</title></head><body>body</body></html>`, `<html lang="id"><head data-theme="x">` + base + `<title>x</title></head><body>body</body></html>`},
		{"html attributes", `<html lang="id"><body>body</body></html>`, `<html lang="id">` + base + `<body>body</body></html>`},
		{"html only", `<html><body><headword>untouched</headword></body></html>`, `<html>` + base + `<body><headword>untouched</headword></body></html>`},
		{"comment", `<!-- <head>fake</head> --><html><head><title>x</title></head></html>`, `<!-- <head>fake</head> --><html><head>` + base + `<title>x</title></head></html>`},
		{"script", `<html><script>const fake = '<head data-x=">">';</script><head><title>x</title></head></html>`, `<html><script>const fake = '<head data-x=">">';</script><head>` + base + `<title>x</title></head></html>`},
		{"style", `<html><style>x::after{content:'<head>'}</style><head></head></html>`, `<html><style>x::after{content:'<head>'}</style><head>` + base + `</head></html>`},
		{"textarea", `<html><textarea><head>fake</head></textarea><head></head></html>`, `<html><textarea><head>fake</head></textarea><head>` + base + `</head></html>`},
		{"quoted attribute", `<html data-fake='<head data-x=">">'><body>body</body></html>`, `<html data-fake='<head data-x=">">'>` + base + `<body>body</body></html>`},
		{"fragment", `<form>body</form>`, base + `<form>body</form>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := injectPreviewAssetBase(tc.input); got != tc.want {
				t.Fatalf("injectPreviewAssetBase() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHandleTemplatePreviewAssetServesOnlyPackageAssets(t *testing.T) {
	withTestDB(t)
	withoutSessionStore(t)
	if err := setTestSetting("tpl_variant", "custom"); err != nil {
		t.Fatal(err)
	}
	if err := setTestSetting("tpl_custom_assets_zip", encodedZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "css/site.css", body: []byte("body{color:red}")},
		zipEntry{name: "js/app.js", body: []byte("window.preview=true")},
		zipEntry{name: "img/logo.png", body: []byte{0x89, 0x50, 0x4e, 0x47}},
		zipEntry{name: "docs/info.html", body: []byte("<h1>asset document</h1>")},
		zipEntry{name: "img/icon.svg", body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
	)); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path, contentType, body string
	}{
		{"css/site.css", "text/css; charset=utf-8", "body{color:red}"},
		{"JS/APP.JS", "text/javascript; charset=utf-8", "window.preview=true"},
		{"img/logo.png", "image/png", string([]byte{0x89, 0x50, 0x4e, 0x47})},
		{"docs/info.html", "text/html; charset=utf-8", "<h1>asset document</h1>"},
		{"img/icon.svg", "image/svg+xml", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/template/preview/assets/"+tc.path, nil), map[string]string{"path": tc.path})
			templateTestApp().HandleTemplatePreviewAsset(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
				t.Fatalf("asset response = %d %q", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != tc.contentType {
				t.Fatalf("Content-Type = %q, want %q", got, tc.contentType)
			}
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("security headers = %#v", rec.Header())
			}
			csp := rec.Header().Get("Content-Security-Policy")
			if !strings.Contains(csp, "sandbox allow-scripts allow-forms") || strings.Contains(csp, "allow-same-origin") || !strings.Contains(csp, "form-action 'none'") {
				t.Fatalf("asset CSP = %q", csp)
			}
		})
	}

	for _, assetPath := range []string{"missing.css", "login.html", "status.html", "logout.html", "../login.html", `..\login.html`, "hotspot/css/site.css"} {
		t.Run("reject "+assetPath, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{"path": assetPath})
			templateTestApp().HandleTemplatePreviewAsset(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("asset %q status = %d, want 404", assetPath, rec.Code)
			}
		})
	}
}

func TestTemplatePreviewFrameRouteIsProtectedGET(t *testing.T) {
	initSessionForTest(t, "template-preview-frame-route-test-secret")
	router := mux.NewRouter()
	(&App{}).RegisterRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/template/preview/frame", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated GET = %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/template/preview/frame", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestTemplatePreviewAssetRouteIsProtectedGET(t *testing.T) {
	initSessionForTest(t, "template-preview-asset-route-test-secret")
	router := mux.NewRouter()
	(&App{}).RegisterRoutes(router)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/template/preview/assets/app.js", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated GET = %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
}
