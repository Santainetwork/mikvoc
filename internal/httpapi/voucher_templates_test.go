package httpapi

import (
	"path/filepath"
	"strings"
	"testing"

	"mikvoc/internal/database"
	"mikvoc/internal/routeros"
)

func TestVoucherContextUsesConfiguredLogoSources(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantImg string
	}{
		{
			name:    "linked logo URL",
			value:   "https://cdn.example.test/mikvoc/logo.svg?theme=dark&v=1",
			wantImg: `<img src="https://cdn.example.test/mikvoc/logo.svg?theme=dark&amp;v=1" alt="logo" style="height:30px;border:0;">`,
		},
		{
			name:    "uploaded logo data URL",
			value:   "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB",
			wantImg: `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB" alt="logo" style="height:30px;border:0;">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTestDatabase(t)
			if err := database.SetSetting("tpl_app_name", "Configured Hotspot"); err != nil {
				t.Fatalf("set app name setting: %v", err)
			}
			if err := database.SetSetting("tpl_logo_url", tt.value); err != nil {
				t.Fatalf("set logo setting: %v", err)
			}

			ctx := newVoucherContext(0, "Fallback Router")
			if ctx.LogoURL != tt.value {
				t.Fatalf("expected LogoURL from tpl_logo_url to be %q, got %q", tt.value, ctx.LogoURL)
			}

			html := generateVoucherDocument("classic", ctx, []routeros.HotspotUser{{
				ID:       "*1",
				Name:     "ABC123",
				Password: "ABC123",
				Profile:  "1d",
			}}, testVoucherMetas(), false)
			assertContains(t, html, tt.wantImg)
			assertNotContains(t, html, `<span class="logo-text">NET</span>`)
		})
	}
}

func TestVoucherContextPerRouterIdentityNotShared(t *testing.T) {
	withTestDatabase(t)
	if err := database.SetSetting("tpl_app_name", "Global Hotspot"); err != nil {
		t.Fatalf("set global app name: %v", err)
	}
	if err := database.SetSetting("tpl_logo_url", "https://cdn.example.test/global-logo.png"); err != nil {
		t.Fatalf("set global logo: %v", err)
	}
	if err := database.SetSetting("tpl_dns_name", "global.local"); err != nil {
		t.Fatalf("set global dns: %v", err)
	}

	if err := database.SetRouterSetting(1, "tpl_app_name", "Warnet Alfa"); err != nil {
		t.Fatalf("set router1 app name: %v", err)
	}
	if err := database.SetRouterSetting(1, "tpl_logo_url", "https://cdn.example.test/alfa-logo.png"); err != nil {
		t.Fatalf("set router1 logo: %v", err)
	}
	if err := database.SetRouterSetting(1, "tpl_dns_name", "alfa.local"); err != nil {
		t.Fatalf("set router1 dns: %v", err)
	}

	if err := database.SetRouterSetting(2, "tpl_app_name", "Cafe Beta"); err != nil {
		t.Fatalf("set router2 app name: %v", err)
	}
	if err := database.SetRouterSetting(2, "tpl_logo_url", "https://cdn.example.test/beta-logo.png"); err != nil {
		t.Fatalf("set router2 logo: %v", err)
	}

	router1 := newVoucherContext(1, "fallback")
	if router1.BrandName != "Warnet Alfa" {
		t.Fatalf("router1 BrandName = %q, want %q", router1.BrandName, "Warnet Alfa")
	}
	if router1.LogoURL != "https://cdn.example.test/alfa-logo.png" {
		t.Fatalf("router1 LogoURL = %q, want alfa logo", router1.LogoURL)
	}
	if router1.DNSName != "alfa.local" {
		t.Fatalf("router1 DNSName = %q, want alfa.local", router1.DNSName)
	}

	router2 := newVoucherContext(2, "fallback")
	if router2.BrandName != "Cafe Beta" {
		t.Fatalf("router2 BrandName = %q, want %q", router2.BrandName, "Cafe Beta")
	}
	if router2.LogoURL != "https://cdn.example.test/beta-logo.png" {
		t.Fatalf("router2 LogoURL = %q, want beta logo", router2.LogoURL)
	}
	if router2.DNSName != "global.local" {
		t.Fatalf("router2 DNSName = %q, want global fallback global.local", router2.DNSName)
	}

	noRouter := newVoucherContext(0, "fallback")
	if noRouter.BrandName != "Global Hotspot" {
		t.Fatalf("no-router BrandName = %q, want global fallback", noRouter.BrandName)
	}
}

func TestClassicVoucherUsesMikhmonTableLayout(t *testing.T) {
	html := generateVoucherDocument("classic", testVoucherContext(), []routeros.HotspotUser{
		{ID: "*1", Name: "ABC123", Password: "ABC123", Profile: "1d", LimitUptime: "30m", LimitBytesTotal: "536870912"},
		{ID: "*2", Name: "user01", Password: "secret01", Profile: "1d"},
	}, testVoucherMetas(), true)

	assertCount(t, html, `<table class="voucher" style=" width: 220px;">`, 2)
	assertContains(t, html, `.qrcode{
		height:80px;
		width:80px;
	}`)
	assertContains(t, html, `style="text-align: left; font-size: 14px; font-weight:bold; border-bottom: 1px black solid;"`)
	assertContains(t, html, `style=" text-align: center; width: 210px; font-size: 12px;"`)
	assertContains(t, html, `<table style="width:100%;">`)

	assertContains(t, html, `Kode Voucher`)
	assertInOrder(t, html,
		`<td font-size: 12px;>Kode Voucher</td>`,
		`style="width:100%; border: 1px solid black; font-weight:bold; font-size:16px;">ABC123</td>`,
	)
	assertContains(t, html, `style="width:100%; border: 1px solid black; font-weight:bold; font-size:16px;">ABC123</td>`)
	assertInOrder(t, html,
		`<td>Username</td>`,
		`style="border: 1px solid black; font-weight:bold;">user01</td>`,
		`<td>Password</td>`,
		`style="border: 1px solid black; font-weight:bold;">secret01</td>`,
	)

	assertContains(t, html, `<canvas class="qrcode"`)
	assertContains(t, html, `size: 256`)
	assertContains(t, html, `hotspot.local/login?username=ABC123`)
	assertContains(t, html, `password=ABC123`)
	assertContains(t, html, `style="border-top: 1px solid black;font-weight:bold; font-size:16px">1d 30m 512.00 MB Rp 2.000</td>`)
	assertInOrder(t, html,
		`<td colspan="2" style="border-top: 1px solid black;font-weight:bold; font-size:16px">1d 30m 512.00 MB Rp 2.000</td>`,
		`<td colspan="2" style="font-weight:bold; font-size:12px">Login: http://hotspot.local</td>`,
	)
	assertNotContains(t, html, `class="v-body"`)
	assertNotContains(t, html, `class="cred qr"`)
}

func TestCompactVoucherUsesMikhmonSmallLayout(t *testing.T) {
	html := generateVoucherDocument("compact", testVoucherContext(), []routeros.HotspotUser{
		{ID: "*1", Name: "ABC123", Password: "ABC123", Profile: "1d", LimitUptime: "30m", LimitBytesTotal: "536870912"},
		{ID: "*2", Name: "user01", Password: "secret01", Profile: "1d"},
	}, testVoucherMetas(), true)

	assertCount(t, html, `<table class="voucher" style=" width: 160px;">`, 2)
	assertContains(t, html, `style=" text-align: center; width: 150px;"`)
	assertContains(t, html, `<tr style="color: black; font-size: 11px;">`)
	assertContains(t, html, `<tr style="color: black; font-size: 14px;">`)

	assertInOrder(t, html,
		`<td >Kode Voucher</td>`,
		`style="width:100%; border: 1px solid black; font-weight:bold;">ABC123</td>`,
		`colspan="2" style="border: 1px solid black; font-weight:bold;">1d 30m 512.00 MB Rp 2.000</td>`,
	)
	assertContains(t, html, `style="width:100%; border: 1px solid black; font-weight:bold;">ABC123</td>`)
	assertInOrder(t, html,
		`<td style="width: 50%">Username</td>`,
		`<td>Password</td>`,
		`style="border: 1px solid black; font-weight:bold;">user01</td>`,
		`style="border: 1px solid black; font-weight:bold;">secret01</td>`,
		`colspan="2" style="border: 1px solid black; font-weight:bold;">1d Rp 2.000</td>`,
	)
	assertContains(t, html, `colspan="2" style="border: 1px solid black; font-weight:bold;">1d 30m 512.00 MB Rp 2.000</td>`)
	assertNotContains(t, html, `class="strip"`)
	assertNotContains(t, html, `class="qrcode"`)
	assertNotContains(t, html, `Login: http://hotspot.local`)
}

func TestThermalVoucherUsesMikhmonThermalLayout(t *testing.T) {
	html := generateVoucherDocument("thermal", testVoucherContext(), []routeros.HotspotUser{
		{ID: "*1", Name: "ABC123", Password: "ABC123", Profile: "1d", LimitUptime: "30m"},
		{ID: "*2", Name: "user01", Password: "secret01", Profile: "1d"},
	}, testVoucherMetas(), true)

	assertCount(t, html, `<table class="voucher" style=" width: 180px;">`, 2)
	assertContains(t, html, `.qrcode{
		height:100px;
		width:100px;
	}`)
	assertContains(t, html, `style="text-align: center; font-size: 14px; font-weight:bold;">My Hotspot</td>`)
	assertContains(t, html, `style="text-align: center; font-size: 14px; font-weight:bold; border-bottom: 1px black solid;"`)
	assertNotContains(t, html, `<br>`)
	assertContains(t, html, `style=" text-align: center; width: 170px; font-size: 12px;"`)

	assertInOrder(t, html,
		`<td font-size: 12px;>Kode Voucher</td>`,
		`style="width:100%; border: 1px solid black; font-weight:bold; font-size:16px;">ABC123</td>`,
	)
	assertInOrder(t, html,
		`<td style="width: 50%">Username</td>`,
		`<td >Password</td>`,
		`style="border: 1px solid black; font-weight:bold;">user01</td>`,
		`style="border: 1px solid black; font-weight:bold;">secret01</td>`,
	)

	assertContains(t, html, `<canvas class="qrcode"`)
	assertContains(t, html, `size: 256`)
	assertContains(t, html, `hotspot.local/login?username=ABC123`)
	assertContains(t, html, `style="border-top: 1px solid black;font-weight:bold; font-size:16px">1d 30m Rp 2.000</td>`)
	assertInOrder(t, html,
		`<td colspan="2" style="border-top: 1px solid black;font-weight:bold; font-size:16px">1d 30m Rp 2.000</td>`,
		`<td colspan="2" style="font-weight:bold; font-size:12px">Login: http://hotspot.local</td>`,
	)
	assertNotContains(t, html, `class="receipt"`)
	assertNotContains(t, html, `<table class="meta">`)
}

func TestVoucherTemplatesAutoEscapeXSS(t *testing.T) {
	payload := `<script>alert(1)</script>`
	attrPayload := `"><img onerror=alert(1) src=x>`
	ctx := voucherContext{
		BrandName: payload,
		LogoText:  attrPayload,
		DNSName:   "hotspot.local",
	}
	users := []routeros.HotspotUser{{
		ID:       "*xss",
		Name:     payload,
		Password: attrPayload,
		Profile:  "1d",
		Comment:  payload,
	}}
	metas := map[string]voucherProfileMeta{"1d": {Price: "1000", Validity: "1d"}}

	for _, tmpl := range []string{"classic", "thermal", "grid", "compact"} {
		t.Run(tmpl, func(t *testing.T) {
			html := generateVoucherDocument(tmpl, ctx, users, metas, true)
			assertNotContains(t, html, `<script>alert(1)</script>`)
			assertNotContains(t, html, `"><img onerror=alert(1) src=x>`)
			assertContains(t, html, `&lt;script&gt;alert(1)&lt;/script&gt;`)
			assertContains(t, html, `/static/js/qrious.min.js`)
			if tmpl != "compact" {
				assertContains(t, html, `class="qrcode"`)
			}
		})
	}
}

func TestGridVoucherRendersCards(t *testing.T) {
	html := generateVoucherDocument("grid", testVoucherContext(), []routeros.HotspotUser{
		{ID: "*1", Name: "ABC123", Password: "ABC123", Profile: "1d"},
		{ID: "*2", Name: "user01", Password: "secret01", Profile: "1d", Comment: "batch-a"},
	}, testVoucherMetas(), true)

	assertContains(t, html, `class="sheet"`)
	assertCount(t, html, `class="card"`, 2)
	assertContains(t, html, `class="code">ABC123</div>`)
	assertContains(t, html, `PW: secret01`)
	assertNotContains(t, html, `batch-a`)
	assertNotContains(t, html, `<div class="lbl">Batch</div>`)
	assertContains(t, html, `page-break-inside:avoid`)
}

func testVoucherContext() voucherContext {
	return voucherContext{BrandName: "My Hotspot", LogoText: "NET", DNSName: "hotspot.local"}
}

func testVoucherMetas() map[string]voucherProfileMeta {
	return map[string]voucherProfileMeta{"1d": {Price: "2000", Validity: "1d"}}
}

func withTestDatabase(t *testing.T) {
	t.Helper()
	oldDB := database.DB
	path := filepath.Join(t.TempDir(), "mikvoc.db")
	if err := database.Init(path, "test-secret"); err != nil {
		t.Fatalf("init test database: %v", err)
	}
	newDB := database.DB
	t.Cleanup(func() {
		if newDB != nil {
			_ = newDB.Close()
		}
		database.DB = oldDB
	})
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected generated HTML to contain %q", want)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("expected generated HTML not to contain %q", want)
	}
}

func assertCount(t *testing.T, got, want string, count int) {
	t.Helper()
	if actual := strings.Count(got, want); actual != count {
		t.Fatalf("expected generated HTML to contain %q %d times, got %d", want, count, actual)
	}
}

func assertInOrder(t *testing.T, got string, wants ...string) {
	t.Helper()
	start := 0
	for _, want := range wants {
		idx := strings.Index(got[start:], want)
		if idx < 0 {
			t.Fatalf("expected generated HTML to contain %q after byte %d", want, start)
		}
		start += idx + len(want)
	}
}
