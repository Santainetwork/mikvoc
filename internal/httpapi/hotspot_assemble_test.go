package httpapi

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const validCustomLogin = `<html><form action="$(link-login-only)"><input name="username"></form></html>`

func customSettings() map[string]string {
	settings := hotspotSettingsFixture()
	settings["tpl_variant"] = "custom"
	return settings
}

func encodedZip(t *testing.T, entries ...zipEntry) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(buildZip(t, entries...))
}

func TestAssembleBuiltinProducesExactlyThreeFilesWithoutAssets(t *testing.T) {
	settings := hotspotSettingsFixture()
	settings["tpl_variant"] = " minimal "

	set, err := assembleTemplateFiles(settings)
	if err != nil {
		t.Fatalf("assembleTemplateFiles() error = %v", err)
	}
	want := []string{"login.html", "status.html", "logout.html"}
	if len(set.Files) != len(want) {
		t.Fatalf("files = %d, want %d", len(set.Files), len(want))
	}
	for i, name := range want {
		if set.Files[i].Name != name {
			t.Fatalf("file %d = %q, want %q", i, set.Files[i].Name, name)
		}
	}
	if set.HasAssets() {
		t.Fatal("built-in files reported assets")
	}
}

func TestAssembleCustomRequiresLogin(t *testing.T) {
	if _, err := assembleTemplateFiles(customSettings()); err == nil || !strings.Contains(err.Error(), "login.html") {
		t.Fatalf("error = %v, want actionable login.html error", err)
	}
}

func TestAssembleCustomRejectsMalformedLogin(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_login_html"] = `<html><input name="username"></html>`
	if _, err := assembleTemplateFiles(settings); err == nil || !strings.Contains(err.Error(), "$(link-login-only)") {
		t.Fatalf("error = %v, want missing MikroTik action error", err)
	}

	settings["tpl_custom_login_html"] = `<form action="$(link-login-only)"></form>`
	if _, err := assembleTemplateFiles(settings); err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("error = %v, want missing username error", err)
	}
}

func TestAssembleValidateCustomLoginAcceptsQuoteAndWhitespaceVariants(t *testing.T) {
	for _, html := range []string{
		`<form action='$(link-login-only)'><input name='username'></form>`,
		"<form action=\"$(link-login-only)\"><input name \t = \t \"username\"></form>",
		`<form method="post" class="login" action = '$(link-login-only)'><input type="text" name = "username"></form>`,
	} {
		if err := validateCustomLogin(html); err != nil {
			t.Fatalf("validateCustomLogin(%q) error = %v", html, err)
		}
	}
	if err := validateCustomLogin(" \n\t "); err == nil {
		t.Fatal("validateCustomLogin() accepted blank HTML")
	}
}

func TestAssembleValidateCustomLoginRejectsInertOrUnrelatedMarkers(t *testing.T) {
	tests := map[string]string{
		"comment":            `<!-- <form action="$(link-login-only)"><input name="username"></form> -->`,
		"script string":      `<script>const form = '<form action="$(link-login-only)"><input name="username"></form>'</script>`,
		"style text":         `<style>x{content:'<form action="$(link-login-only)"><input name="username"></form>'}</style>`,
		"textarea text":      `<textarea><form action="$(link-login-only)"><input name="username"></form></textarea>`,
		"unrelated attrs":    `<form data-action="$(link-login-only)"><input data-name="username"></form>`,
		"wrong form action":  `<form action="/login" data-target="$(link-login-only)"><input name="username"></form>`,
		"non-input username": `<form action="$(link-login-only)"><textarea name="username"></textarea></form>`,
		"input outside form": `<input name="username"><form action="$(link-login-only)"></form>`,
	}
	for name, html := range tests {
		t.Run(name, func(t *testing.T) {
			if err := validateCustomLogin(html); err == nil {
				t.Fatalf("validateCustomLogin() accepted %s markers", name)
			}
		})
	}
}

func TestAssembleValidateCustomLoginRejectsTagsEmbeddedInAttributes(t *testing.T) {
	html := `<div data-example='<form action="$(link-login-only)"><input name="username"></form>'>example</div>`
	if err := validateCustomLogin(html); err == nil {
		t.Fatal("validateCustomLogin() accepted tags embedded in an attribute value")
	}
}

func TestAssembleValidateCustomLoginRejectsNestedForms(t *testing.T) {
	for name, html := range map[string]string{
		"inner valid": `<form action="/wrong"><form action="$(link-login-only)"><input name="username"></form></form>`,
		"outer valid": `<form action="$(link-login-only)"><form action="/wrong"><input name="username"></form></form>`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateCustomLogin(html); err == nil {
				t.Fatal("validateCustomLogin() accepted ambiguous nested forms")
			}
		})
	}
}

func TestAssembleValidateCustomLoginParsesQuotedGreaterThanAndUnquotedAttributes(t *testing.T) {
	valid := []string{
		`<form data-note="1 > 0" action="$(link-login-only)"><input title='x > y' name="username"></form>`,
		`<form action=$(link-login-only)><input name=username /></form>`,
		`<FORM METHOD=post ACTION='$(link-login-only)'><INPUT TYPE=text NAME='UsErNaMe'></FORM>`,
	}
	for _, html := range valid {
		if err := validateCustomLogin(html); err != nil {
			t.Fatalf("validateCustomLogin(%q) error = %v", html, err)
		}
	}
}

func TestAssembleCustomEditorLoginVerbatimWithGeneratedFallbacks(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_login_html"] = validCustomLogin

	set, err := assembleTemplateFiles(settings)
	if err != nil {
		t.Fatalf("assembleTemplateFiles() error = %v", err)
	}
	if got := string(set.Get("login.html")); got != validCustomLogin {
		t.Fatalf("login = %q, want editor content verbatim", got)
	}
	if !strings.Contains(string(set.Get("status.html")), "$(username)") {
		t.Fatal("status.html did not use generated fallback")
	}
	if !strings.Contains(string(set.Get("logout.html")), "$(link-login)") {
		t.Fatal("logout.html did not use generated fallback")
	}
}

func TestAssembleCustomIncludesBinaryAssetsInArchiveOrder(t *testing.T) {
	binary := []byte{0, 1, 2, 0xff}
	settings := customSettings()
	settings["tpl_custom_assets_zip"] = encodedZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "img/logo.png", body: binary},
		zipEntry{name: "css/site.css", body: []byte("body{}")},
	)

	set, err := assembleTemplateFiles(settings)
	if err != nil {
		t.Fatalf("assembleTemplateFiles() error = %v", err)
	}
	wantNames := []string{"login.html", "status.html", "logout.html", "img/logo.png", "css/site.css"}
	for i, want := range wantNames {
		if set.Files[i].Name != want {
			t.Fatalf("file %d = %q, want %q", i, set.Files[i].Name, want)
		}
	}
	if !set.HasAssets() || !set.Files[3].Asset || !set.Files[4].Asset {
		t.Fatal("ZIP assets lost Asset flags")
	}
	if !bytes.Equal(set.Get("img/logo.png"), binary) {
		t.Fatalf("binary asset = %v, want %v", set.Get("img/logo.png"), binary)
	}
}

func TestAssembleCustomEditorStandardsOverrideZipWhileAssetsSurvive(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_assets_zip"] = encodedZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "status.html", body: []byte("zip status")},
		zipEntry{name: "logout.html", body: []byte("zip logout")},
		zipEntry{name: "app.js", body: []byte("zip asset")},
	)
	editorLogin := strings.Replace(validCustomLogin, "<html>", "<html data-source=\"editor\">", 1)
	settings["tpl_custom_login_html"] = editorLogin
	settings["tpl_custom_status_html"] = "editor status"
	settings["tpl_custom_logout_html"] = "editor logout"

	set, err := assembleTemplateFiles(settings)
	if err != nil {
		t.Fatalf("assembleTemplateFiles() error = %v", err)
	}
	for name, want := range map[string]string{
		"login.html": editorLogin, "status.html": "editor status", "logout.html": "editor logout", "app.js": "zip asset",
	} {
		if got := string(set.Get(name)); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	if len(set.Files) != 4 || set.Files[3].Name != "app.js" || !set.Files[3].Asset {
		t.Fatalf("files = %#v, want standards then surviving asset", set.Files)
	}
}

func TestAssembleCustomUsesZipStandardsWhenEditorsBlank(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_login_html"] = " \n "
	settings["tpl_custom_status_html"] = "\t"
	settings["tpl_custom_logout_html"] = " "
	settings["tpl_custom_assets_zip"] = encodedZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "status.html", body: []byte("zip status")},
		zipEntry{name: "logout.html", body: []byte("zip logout")},
	)

	set, err := assembleTemplateFiles(settings)
	if err != nil {
		t.Fatalf("assembleTemplateFiles() error = %v", err)
	}
	if got := string(set.Get("login.html")); got != validCustomLogin {
		t.Fatalf("login = %q, want ZIP login", got)
	}
	if got := string(set.Get("status.html")); got != "zip status" {
		t.Fatalf("status = %q, want ZIP status", got)
	}
	if got := string(set.Get("logout.html")); got != "zip logout" {
		t.Fatalf("logout = %q, want ZIP logout", got)
	}
}

func TestAssembleStoredAssetPackageRejectsCorruption(t *testing.T) {
	for name, encoded := range map[string]string{
		"base64": "%%%", "zip": base64.StdEncoding.EncodeToString([]byte("not a zip")),
	} {
		t.Run(name, func(t *testing.T) {
			settings := customSettings()
			settings["tpl_custom_assets_zip"] = encoded
			if _, err := assembleTemplateFiles(settings); err == nil {
				t.Fatal("assembleTemplateFiles() accepted corrupt stored package")
			}
		})
	}
}

func TestAssembleStoredAssetPackageEmptyReturnsEmptySet(t *testing.T) {
	set, err := storedAssetPackage(map[string]string{"tpl_custom_assets_zip": " \n "})
	if err != nil || len(set.Files) != 0 {
		t.Fatalf("storedAssetPackage(empty) = %#v, %v; want empty set", set, err)
	}
}

func TestAssembleStoredAssetPackageRejectsOversizedEncodingBeforeDecode(t *testing.T) {
	settings := map[string]string{
		"tpl_custom_assets_zip": strings.Repeat("A", base64.StdEncoding.EncodedLen(maxAssetCompressedBytes)+1),
	}
	_, err := storedAssetPackage(settings)
	if err == nil || !strings.Contains(err.Error(), "terlalu besar") {
		t.Fatalf("error = %v, want actionable oversized package error", err)
	}
}

func TestAssembleStoredAssetPackageRejectsWhitespace(t *testing.T) {
	settings := map[string]string{"tpl_custom_assets_zip": "AAAA\nAAAA"}
	_, err := storedAssetPackage(settings)
	if err == nil || !strings.Contains(err.Error(), "spasi") {
		t.Fatalf("error = %v, want actionable whitespace error", err)
	}
}

func TestAssembleCustomHotspotHTMLMalformedSettingsShowsExplicitError(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_login_html"] = "malformed"
	login, status, logout := customHotspotHTMLFor(settings)
	v := hotspotViewFrom(settings)
	if !strings.Contains(login, "Template custom tidak valid") || !strings.Contains(login, "$(link-login-only)") {
		t.Fatalf("error login = %q, want explicit escaped validation reason", login)
	}
	if strings.Contains(login, `action="$(link-login-only)"`) || login == renderModernLogin(v) {
		t.Fatal("invalid custom preview looked like a valid generated login")
	}
	if status != renderHotspotStatus(v) || logout != renderHotspotLogout(v) {
		t.Fatal("invalid custom preview lost safe status/logout pages")
	}
}

func TestAssembleStandardHotspotFileIsCaseInsensitive(t *testing.T) {
	for _, name := range []string{"login.html", "STATUS.HTML", "Logout.Html"} {
		if !isStandardHotspotFile(name) {
			t.Fatalf("isStandardHotspotFile(%q) = false", name)
		}
	}
	if isStandardHotspotFile("assets/login.html") {
		t.Fatal("nested login.html classified as standard")
	}
}

func TestPushableHotspotHTMLRejectsInvalidCustom(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_login_html"] = "invalid"
	if _, _, _, err := pushableHotspotHTML(settings); err == nil || !strings.Contains(err.Error(), "login.html") {
		t.Fatalf("error = %v, want custom validation error", err)
	}
}

func TestPushableHotspotHTMLRejectsAssets(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_assets_zip"] = encodedZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "logo.png", body: []byte("png")},
	)
	if _, _, _, err := pushableHotspotHTML(settings); err == nil || !strings.Contains(err.Error(), "Unduh Paket ZIP") {
		t.Fatalf("error = %v, want assets push error", err)
	}
}

func TestPushableHotspotHTMLRejectsOversizedFileWithDetails(t *testing.T) {
	settings := customSettings()
	settings["tpl_custom_login_html"] = validCustomLogin + strings.Repeat("x", routerFileContentLimit)
	if _, _, _, err := pushableHotspotHTML(settings); err == nil || !strings.Contains(err.Error(), "login.html") || !strings.Contains(err.Error(), "4096") {
		t.Fatalf("error = %v, want oversized login details", err)
	}
}

func TestPushableHotspotHTMLReturnsValidBuiltin(t *testing.T) {
	settings := hotspotSettingsFixture()
	settings["tpl_variant"] = "minimal"
	login, status, logout, err := pushableHotspotHTML(settings)
	if err != nil {
		t.Fatalf("pushableHotspotHTML() error = %v", err)
	}
	if login != renderVariantLogin("minimal", settings) || status == "" || logout == "" {
		t.Fatal("pushableHotspotHTML() returned wrong built-in pages")
	}
}
