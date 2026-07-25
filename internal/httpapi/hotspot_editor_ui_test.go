package httpapi

import (
	"encoding/base64"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
)

func TestTemplateEditorSourceHasVariantAndDeliveryControls(t *testing.T) {
	raw, err := os.ReadFile("../../web/templates/pages/template_editor.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	for _, want := range []string{
		`<fieldset`, `<legend`, `name="tpl_variant"`, `data-variant-panel="informative"`,
		`name="tpl_info_html"`, `data-variant-panel="custom"`, `name="tpl_custom_login_html"`,
		`name="tpl_custom_status_html"`, `name="tpl_custom_logout_html"`,
		`id="assets-zip-input"`, `name="tpl_assets_zip"`, `accept=".zip"`, `name="remove_assets"`,
		`/template/download`, `4096`, `Winbox`, `WebFig`, `.PushBlockedReason`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("template editor missing %q", want)
		}
	}
	for _, want := range []string{
		`querySelectorAll('input[name="tpl_variant"]')`, `data-variant-panel`, `toggleAttribute('hidden'`,
		`data-selected`, `logo-upload-input`, `assets-zip-input`, `master.appendChild(upload)`,
		`document.createElement('img')`, `preview.replaceChildren(img)`,
		`Array.from(forms).every(f => f.reportValidity())`, `logo.files[0].size`, `assets.files[0].size`,
		`toLowerCase().endsWith('.zip')`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("template editor JS missing %q", want)
		}
	}
	if strings.Contains(html, "preview.innerHTML") {
		t.Fatal("logo preview must not send user data through innerHTML")
	}
	for id, name := range map[string]string{
		"tpl-app-name":       "tpl_app_name",
		"tpl-subtitle":       "tpl_subtitle",
		"tpl-btn-label":      "tpl_btn_label",
		"tpl-login-mode":     "tpl_login_mode",
		"tpl-redirect-url":   "tpl_redirect_url",
		"tpl-dns-name":       "tpl_dns_name",
		"logo-upload-input":  "tpl_logo_upload",
		"logo-url-input":     "tpl_logo_url",
		"logo-text-input":    "tpl_logo_text",
		"color-primary":      "tpl_primary_color",
		"color-bg":           "tpl_bg_color",
		"tpl-show-info":      "tpl_show_info",
		"tpl-info-html":      "tpl_info_html",
		"custom-login-html":  "tpl_custom_login_html",
		"custom-status-html": "tpl_custom_status_html",
		"custom-logout-html": "tpl_custom_logout_html",
		"assets-zip-input":   "tpl_assets_zip",
		"remove-assets":      "remove_assets",
	} {
		control := regexp.MustCompile(`(?s)<(?:input|select|textarea)[^>]*(?:id="` + regexp.QuoteMeta(id) + `"[^>]*name="` + regexp.QuoteMeta(name) + `"|name="` + regexp.QuoteMeta(name) + `"[^>]*id="` + regexp.QuoteMeta(id) + `")[^>]*>`)
		if !strings.Contains(html, `for="`+id+`"`) || !control.MatchString(html) {
			t.Errorf("control %q lacks matching label/id/name for %q", id, name)
		}
	}
}

func TestTemplateEditorViewDataUsesDefaultsAndAssembledFiles(t *testing.T) {
	withTestDB(t)
	if err := database.SetTemplateSettings(7, map[string]string{
		"tpl_variant":   " informative ",
		"tpl_info_html": "<strong>Paket</strong>",
	}); err != nil {
		t.Fatal(err)
	}

	data := templateEditorViewData(7)
	if data.RouterID != 7 || data.Settings["tpl_variant"] != "informative" {
		t.Fatalf("router/variant = %d/%q", data.RouterID, data.Settings["tpl_variant"])
	}
	if data.Settings["tpl_app_name"] != "Hotspot Login" || data.Settings["voucher_template"] != "classic" {
		t.Fatalf("defaults missing: %#v", data.Settings)
	}
	if len(data.Variants) != 5 {
		t.Fatalf("variants = %d, want 5", len(data.Variants))
	}
	if got := fileInfoNames(data.FileInfos); got != "login.html|logout.html|status.html" {
		t.Fatalf("file order = %q", got)
	}
	if data.PushBlockedReason != "" {
		t.Fatalf("push blocked: %s", data.PushBlockedReason)
	}
}

func TestTemplateEditorViewDataCustomAssetsAreSortedAndBlockPush(t *testing.T) {
	withTestDB(t)
	raw := buildZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "z.js", body: []byte("z")},
		zipEntry{name: "css/a.css", body: []byte("a")},
		zipEntry{name: "img/my logo.png", body: []byte("png")},
	)
	if err := database.SetTemplateSettings(0, map[string]string{
		"tpl_variant":                "custom",
		"tpl_custom_login_html":      validCustomLogin,
		"tpl_custom_assets_zip":      base64.StdEncoding.EncodeToString(raw),
		"tpl_custom_assets_manifest": "stale.txt",
	}); err != nil {
		t.Fatal(err)
	}

	data := templateEditorViewData(0)
	if data.Settings["tpl_variant"] != "custom" {
		t.Fatalf("variant = %q", data.Settings["tpl_variant"])
	}
	if got := strings.Join(data.AssetNames, "|"); got != "css/a.css|img/my logo.png|z.js" {
		t.Fatalf("assets = %q", got)
	}
	if !strings.Contains(data.PushBlockedReason, "aset") {
		t.Fatalf("push reason = %q", data.PushBlockedReason)
	}
	if got := fileInfoNames(data.FileInfos); got != "css/a.css|img/my logo.png|login.html|logout.html|status.html|z.js" {
		t.Fatalf("file order = %q", got)
	}
}

func TestTemplateEditorViewDataHydratesBlankCustomSourceFromZIP(t *testing.T) {
	withTestDB(t)
	login := strings.Replace(validCustomLogin, "</form>", `<p>Tea &amp; "WiFi"</p></form>`, 1)
	status := `<main>Status & "online"</main>`
	raw := buildZip(t,
		zipEntry{name: "login.html", body: []byte(login)},
		zipEntry{name: "status.html", body: []byte(status)},
		zipEntry{name: "css/site.css", body: []byte("body{}")},
	)
	seedSettings(t, 0, map[string]string{
		"tpl_variant":           "custom",
		"tpl_custom_assets_zip": base64.StdEncoding.EncodeToString(raw),
	})

	data := templateEditorViewData(0)
	if data.Settings["tpl_custom_login_html"] != login || data.Settings["tpl_custom_status_html"] != status {
		t.Fatalf("hydrated source = %q / %q", data.Settings["tpl_custom_login_html"], data.Settings["tpl_custom_status_html"])
	}
	if data.Settings["tpl_custom_logout_html"] != "" {
		t.Fatalf("absent logout source = %q, want blank", data.Settings["tpl_custom_logout_html"])
	}
	if got := database.GetRouterSetting(0, "tpl_custom_login_html"); got != "" {
		t.Fatalf("view hydration persisted login source = %q", got)
	}

	body := renderTemplateEditorGET(t, 0)
	if !strings.Contains(body, html.EscapeString(login)) || strings.Contains(body, login) {
		t.Fatal("ZIP login source was not safely escaped in textarea")
	}

	form := templateForm("save", "custom")
	form.Set("tpl_custom_login_html", data.Settings["tpl_custom_login_html"])
	form.Set("tpl_custom_status_html", data.Settings["tpl_custom_status_html"])
	form.Set("tpl_custom_logout_html", data.Settings["tpl_custom_logout_html"])
	form.Set("remove_assets", "1")
	postTemplate(t, 0, form, nil)
	assertSettings(t, 0, map[string]string{
		"tpl_custom_assets_zip": "", "tpl_custom_login_html": login, "tpl_custom_status_html": status,
	})
}

func TestTemplateEditorViewDataSurvivesInvalidCustom(t *testing.T) {
	withTestDB(t)
	if err := database.SetTemplateSettings(0, map[string]string{
		"tpl_variant":           "custom",
		"tpl_custom_login_html": "invalid",
	}); err != nil {
		t.Fatal(err)
	}

	data := templateEditorViewData(0)
	if data.Settings["tpl_variant"] != "custom" || !strings.Contains(data.PushBlockedReason, "login.html") {
		t.Fatalf("invalid custom data = %#v", data)
	}
}

func TestTemplateEditorGETRendersBuiltInVariantAndDeliveryState(t *testing.T) {
	withTestDB(t)
	seedSettings(t, 9, map[string]string{
		"tpl_variant":      "informative",
		"tpl_info_html":    `<strong>Paket & Aman</strong><script>alert("x")</script>`,
		"voucher_template": "classic",
	})
	body := renderTemplateEditorGET(t, 9)

	variantRE := regexp.MustCompile(`(?s)<input[^>]+type="radio"[^>]+name="tpl_variant"[^>]+value="(modern|informative|minimal|cafe|custom)"[^>]*>`)
	if got := len(variantRE.FindAllString(body, -1)); got < 5 {
		t.Fatalf("variant radios = %d, want at least 5", got)
	}
	if !regexp.MustCompile(`(?s)<input[^>]+name="tpl_variant"[^>]+value="informative"[^>]*checked`).MatchString(body) {
		t.Fatal("stored informative variant is not checked")
	}
	for _, want := range []string{
		`name="voucher_template"`, `Template Cetak Voucher`, `<fieldset`, `<legend`,
		`grid grid-cols-1 sm:grid-cols-2`, `4096 byte`, `href="/template/download"`, `Winbox`, `WebFig`,
		`data-template-action="save"`, `data-template-action="push"`,
		`data-variant-panel="informative"`, `data-variant-panel="custom" hidden`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered template missing %q", want)
		}
	}
	if strings.Contains(buttonTag(t, body, "save"), "disabled") {
		t.Fatal("save button disabled for built-in variant")
	}
	if strings.Contains(buttonTag(t, body, "push"), "disabled") {
		t.Fatal("push button disabled for valid built-in variant")
	}
	wantEscaped := html.EscapeString(`<strong>Paket & Aman</strong><script>alert("x")</script>`)
	if !strings.Contains(body, wantEscaped) {
		t.Fatalf("informative textarea did not safely escape content: want %q", wantEscaped)
	}
	if regexp.MustCompile(`data-variant-panel="informative"[^>]*hidden`).MatchString(body) {
		t.Fatal("informative panel hidden while selected")
	}
}

func TestTemplateEditorGETKeepsSaveEnabledForInvalidCustom(t *testing.T) {
	withTestDB(t)
	seedSettings(t, 11, map[string]string{
		"tpl_variant":           "custom",
		"tpl_custom_login_html": "",
	})
	body := renderTemplateEditorGET(t, 11)

	if !strings.Contains(body, `Push akan divalidasi saat dikirim:`) || !strings.Contains(body, `login.html`) {
		t.Fatal("invalid custom warning reason not rendered")
	}
	if strings.Contains(buttonTag(t, body, "save"), "disabled") {
		t.Fatal("save button disabled for invalid custom settings")
	}
	push := buttonTag(t, body, "push")
	if strings.Contains(push, "disabled") || strings.Contains(push, `aria-disabled`) {
		t.Fatalf("push button disabled by stale saved state: %s", push)
	}
	if !strings.Contains(body, `Perbaiki field atau hapus aset, lalu Push; atau unduh Paket ZIP.`) {
		t.Fatal("push warning lacks recovery guidance")
	}
	if !regexp.MustCompile(`data-variant-panel="informative"[^>]*hidden`).MatchString(body) {
		t.Fatal("informative panel visible for custom selection")
	}
	if regexp.MustCompile(`data-variant-panel="custom"[^>]*hidden`).MatchString(body) {
		t.Fatal("custom panel hidden while selected")
	}
}

func renderTemplateEditorGET(t *testing.T, routerID int) string {
	t.Helper()
	oldStore := middleware.Store
	middleware.InitSession("template-editor-ui-test-secret")
	t.Cleanup(func() { middleware.Store = oldStore })

	req := httptest.NewRequest(http.MethodGet, "/template", nil)
	cookieRec := httptest.NewRecorder()
	sess, err := middleware.Store.Get(req, middleware.SessionName)
	if err != nil {
		t.Fatal(err)
	}
	sess.Values["authenticated"] = true
	sess.Values["router_id"] = routerID
	if err := sess.Save(req, cookieRec); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range cookieRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	NewApp(nil, nil, nil, nil, nil, nil, nil).HandleTemplateEditor(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /template status = %d, body = %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func buttonTag(t *testing.T, body, action string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<button[^>]+data-template-action="` + regexp.QuoteMeta(action) + `"[^>]*>`)
	match := re.FindString(body)
	if match == "" {
		t.Fatalf("%s button not found", action)
	}
	return match
}

func fileInfoNames(in []struct {
	Name     string
	Size     int
	Asset    bool
	TooLarge bool
}) string {
	names := make([]string, len(in))
	for i := range in {
		names[i] = in[i].Name
	}
	return strings.Join(names, "|")
}
