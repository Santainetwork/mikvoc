package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
	"mikvoc/internal/routeros"
)

func TestHandleGenerateRequiresTemplateService(t *testing.T) {
	rec := httptest.NewRecorder()
	(&App{}).HandleGenerate(rec, httptest.NewRequest(http.MethodGet, "/hotspot/generate", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHandleGenerateUsesActiveRouterMergedLoginMode(t *testing.T) {
	withTestDB(t)
	middleware.InitSession("generate-router-settings")
	if err := database.SetSetting("tpl_login_mode", "voucher"); err != nil {
		t.Fatal(err)
	}
	router := &database.Router{Name: "Seven", IP: "127.0.0.1", Port: "8728", Username: "admin"}
	if err := database.SaveRouter(router); err != nil {
		t.Fatal(err)
	}
	if err := database.SetRouterSetting(router.ID, "tpl_login_mode", "member"); err != nil {
		t.Fatal(err)
	}

	app := templateTestApp()
	if err := app.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/hotspot/generate", nil)
	sessionRec := httptest.NewRecorder()
	app.SetSessionRouterID(sessionRec, req, router.ID)
	req.Header.Set("Cookie", sessionRec.Header().Get("Set-Cookie"))
	rec := httptest.NewRecorder()
	app.HandleGenerate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `name="mode" value="up"`) || !strings.Contains(body, "Member Only") {
		t.Fatalf("active router login mode not rendered: %s", body)
	}
}

func TestGenerateOptionsUsesProfileValidityWhenTimeLimitBlank(t *testing.T) {
	req := newGenerateRequest(url.Values{
		"qty":        {"2"},
		"profile":    {"1d"},
		"server":     {"all"},
		"mode":       {"vc"},
		"length":     {"6"},
		"charmode":   {"lower"},
		"time_limit": {""},
	})

	opts := generateOptionsFromRequest(req, []routeros.HotspotUserProfile{
		{Name: "1d", Validity: "1d"},
	})

	if opts.TimeLimitStr != "1d" {
		t.Fatalf("expected blank time_limit to fall back to profile validity, got %q", opts.TimeLimitStr)
	}
}

func TestGenerateOptionsKeepsManualTimeLimitOverride(t *testing.T) {
	req := newGenerateRequest(url.Values{
		"qty":        {"2"},
		"profile":    {"1d"},
		"server":     {"all"},
		"mode":       {"vc"},
		"length":     {"6"},
		"charmode":   {"lower"},
		"time_limit": {"12h"},
	})

	opts := generateOptionsFromRequest(req, []routeros.HotspotUserProfile{
		{Name: "1d", Validity: "1d"},
	})

	if opts.TimeLimitStr != "12h" {
		t.Fatalf("expected manual time_limit to override profile validity, got %q", opts.TimeLimitStr)
	}
}

func TestGenerateOptionsLeavesTimeLimitBlankWithoutProfileValidity(t *testing.T) {
	req := newGenerateRequest(url.Values{
		"qty":        {"2"},
		"profile":    {"no-validity"},
		"server":     {"all"},
		"mode":       {"vc"},
		"length":     {"6"},
		"charmode":   {"lower"},
		"time_limit": {""},
	})

	opts := generateOptionsFromRequest(req, []routeros.HotspotUserProfile{
		{Name: "no-validity"},
	})

	if opts.TimeLimitStr != "" {
		t.Fatalf("expected time_limit to remain blank without profile validity, got %q", opts.TimeLimitStr)
	}
}

func TestGenerateTemplateShowsProfileValidityHint(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/pages/generate.html")
	if err != nil {
		t.Fatalf("read generate template: %v", err)
	}
	html := string(b)

	for _, want := range []string{
		`data-validity="{{.Validity}}"`,
		`id="profile-validity-hint"`,
		`updateProfileValidityHint`,
		`Masa aktif profil`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected generate template to contain %q", want)
		}
	}
}

func newGenerateRequest(form url.Values) *http.Request {
	req := httptest.NewRequest("POST", "/hotspot/generate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		panic(err)
	}
	return req
}
