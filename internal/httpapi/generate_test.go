package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"mikvoc/internal/routeros"
)

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
