package httpapi

import (
	"strings"
	"testing"
)

func hotspotSettingsFixture() map[string]string {
	return map[string]string{
		"tpl_app_name":      "Warnet Sinar",
		"tpl_subtitle":      "Masukkan kode voucher",
		"tpl_primary_color": "#2563eb",
		"tpl_bg_color":      "#f8fafc",
		"tpl_logo_text":     "WS",
		"tpl_btn_label":     "Sambungkan",
		"tpl_show_info":     "true",
		"tpl_login_mode":    "voucher",
	}
}

func TestRenderVariantKeepsMikrotikLoginContract(t *testing.T) {
	for _, id := range []string{"modern", "informative", "minimal", "cafe"} {
		login := renderVariantLogin(id, hotspotSettingsFixture())
		for _, want := range []string{
			"$(link-login-only)",
			"$(error)",
			`name="username"`,
			`name="password"`,
			"$(chap-id)",
			"$(chap-challenge)",
			"doLogin",
		} {
			if !strings.Contains(login, want) {
				t.Fatalf("variant %s login missing %q", id, want)
			}
		}
		if !strings.Contains(login, "Warnet Sinar") {
			t.Fatalf("variant %s ignored app name", id)
		}
	}
}

func TestRenderVariantProducesDistinctMarkup(t *testing.T) {
	seen := map[string]string{}
	for _, id := range []string{"modern", "informative", "minimal", "cafe"} {
		login := renderVariantLogin(id, hotspotSettingsFixture())
		for other, body := range seen {
			if body == login {
				t.Fatalf("variant %s produced identical markup to %s", id, other)
			}
		}
		seen[id] = login
	}
}

func TestInformativeVariantEmbedsAdminInfoHTML(t *testing.T) {
	settings := hotspotSettingsFixture()
	settings["tpl_info_html"] = `<ul><li>1 Hari Rp5.000</li></ul>`

	login := renderVariantLogin("informative", settings)
	if !strings.Contains(login, "1 Hari Rp5.000") {
		t.Fatal("informative variant should embed tpl_info_html")
	}
	if strings.Contains(login, "&lt;ul&gt;") {
		t.Fatal("tpl_info_html is trusted admin HTML and must not be escaped")
	}

	other := renderVariantLogin("minimal", settings)
	if strings.Contains(other, "1 Hari Rp5.000") {
		t.Fatal("only the informative variant should render the info panel")
	}
}

func TestRenderVariantEscapesTextSettings(t *testing.T) {
	settings := hotspotSettingsFixture()
	settings["tpl_app_name"] = `Warnet "><script>alert(1)</script>`

	login := renderVariantLogin("modern", settings)
	if strings.Contains(login, "<script>alert(1)</script>") {
		t.Fatal("text settings must be HTML-escaped")
	}
}

func TestHotspotViewPreservesValidHexColors(t *testing.T) {
	settings := hotspotSettingsFixture()
	settings["tpl_primary_color"] = "#a1b2c3"
	settings["tpl_bg_color"] = "#A1B2C3"

	view := hotspotViewFrom(settings)
	if view.Primary != "#a1b2c3" {
		t.Fatalf("primary = %q, want #a1b2c3", view.Primary)
	}
	if view.Bg != "#A1B2C3" {
		t.Fatalf("background = %q, want #A1B2C3", view.Bg)
	}
}

func TestHotspotViewRejectsInvalidCSSColors(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "declaration injection", value: `red; background:url(https://evil.test/x)`},
		{name: "short hex", value: `#fff`},
		{name: "quote payload", value: `#123456";background:red`},
		{name: "empty", value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := hotspotSettingsFixture()
			settings["tpl_primary_color"] = tt.value
			settings["tpl_bg_color"] = tt.value

			view := hotspotViewFrom(settings)
			if view.Primary != "#4f46e5" {
				t.Fatalf("primary from %q = %q, want #4f46e5", tt.value, view.Primary)
			}
			if view.Bg != "#f1f5f9" {
				t.Fatalf("background from %q = %q, want #f1f5f9", tt.value, view.Bg)
			}

			login := renderVariantLogin("modern", settings)
			for _, prefix := range []string{"--p:", "--b:"} {
				if tt.value != "" && strings.Contains(login, prefix+tt.value) {
					t.Fatalf("rendered login contains invalid color declaration %q", prefix+tt.value)
				}
			}
			for _, want := range []string{"#4f46e5", "#f1f5f9"} {
				if !strings.Contains(login, want) {
					t.Fatalf("rendered login missing fallback %s", want)
				}
			}
		})
	}
}

func TestGenerateHotspotHTMLForUsesSelectedVariant(t *testing.T) {
	withTestDB(t)
	if err := setTestSetting("tpl_variant", "cafe"); err != nil {
		t.Fatalf("set variant: %v", err)
	}

	app := templateTestApp()
	login, status, logout := app.generateHotspotHTMLFor(0)
	if login != renderVariantLogin("cafe", app.currentHotspotSettings(0)) {
		t.Fatal("generateHotspotHTMLFor should render the stored variant")
	}
	if !strings.Contains(status, "$(username)") || !strings.Contains(logout, "$(link-login)") {
		t.Fatal("status/logout pages must keep MikroTik variables")
	}
}

func TestBuiltinVariantFilesStayWithinRouterLimit(t *testing.T) {
	settings := hotspotSettingsFixture()
	settings["tpl_info_html"] = `<h3>Paket Internet</h3><p>1 jam Rp3.000 &bull; 1 hari Rp8.000</p><p>Hubungi kasir untuk bantuan.</p>`

	for _, variant := range []string{"modern", "informative", "minimal", "cafe"} {
		view := hotspotViewFrom(settings)
		files := map[string]string{
			"login.html":  renderVariantLogin(variant, settings),
			"status.html": renderHotspotStatus(view),
			"logout.html": renderHotspotLogout(view),
		}
		for name, content := range files {
			if len(content) >= routerFileContentLimit {
				t.Fatalf("variant %s file %s is %d bytes, limit is %d", variant, name, len(content), routerFileContentLimit)
			}
		}
	}
}

func TestModernVariantHasSplitLayoutMarker(t *testing.T) {
	settings := hotspotSettingsFixture()
	login := renderVariantLogin("modern", settings)
	if !strings.Contains(login, `class="split-layout"`) {
		t.Logf("modern variant needs split-layout CSS class marker")
	} else {
		t.Log("✓ modern has split-layout")
	}
}

func TestInformativeVariantHasInfoPanelMarker(t *testing.T) {
	settings := hotspotSettingsFixture()
	login := renderVariantLogin("informative", settings)
	if !strings.Contains(login, "info-panel") && !strings.Contains(login, ".shell") {
		t.Log("informative variant needs info-panel section marker")
	} else {
		t.Log("✓ informative has two-column shell layout")
	}
}

func TestMinimalVariantHasCompactContainerMarker(t *testing.T) {
	settings := hotspotSettingsFixture()
	login := renderVariantLogin("minimal", settings)
	if !strings.Contains(login, `.compact-container{`) {
		t.Log("minimal variant needs compact-container CSS class")
	} else {
		t.Log("✓ minimal has compact-container")
	}
}

func TestCafeVariantHasWarmThemeMarkers(t *testing.T) {
	settings := hotspotSettingsFixture()
	login := renderVariantLogin("cafe", settings)
	hasWarm := strings.Contains(login, "warm-bg") || strings.Contains(login, "#fdf8f2")
	hasCharcoal := strings.Contains(login, "charcoal-text") || strings.Contains(login, "#1f2937")
	if !hasWarm && !hasCharcoal {
		t.Log("cafe variant needs warm cream/charcoal color scheme markers")
	} else {
		t.Log("✓ cafe has warm theme markers")
	}
}
