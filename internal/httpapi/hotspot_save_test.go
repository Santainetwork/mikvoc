package httpapi

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
)

func TestTemplateSettingKeysComplete(t *testing.T) {
	want := []string{
		"tpl_app_name", "tpl_subtitle", "tpl_bg_color", "tpl_primary_color",
		"tpl_logo_text", "tpl_show_info", "tpl_btn_label", "tpl_login_mode",
		"tpl_redirect_url", "tpl_dns_name", "tpl_variant", "tpl_info_html",
		"tpl_custom_login_html", "tpl_custom_status_html", "tpl_custom_logout_html",
		"tpl_bg_image", "tpl_focal_x", "tpl_focal_y",
	}
	if strings.Join(templateSettingKeys, "\n") != strings.Join(want, "\n") {
		t.Fatalf("templateSettingKeys = %q, want %q", templateSettingKeys, want)
	}
}

func TestTemplateEditorRequiresTemplateService(t *testing.T) {
	rec := httptest.NewRecorder()
	(&App{}).HandleTemplateEditor(rec, httptest.NewRequest(http.MethodGet, "/template", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestVoucherTemplateSetterRequiresTemplateService(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/template/voucher-template", strings.NewReader("voucher_template=classic"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	(&App{}).HandleSetVoucherTemplate(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestSanitizeVariantInput(t *testing.T) {
	got, err := sanitizeVariantInput("  InFoRmAtIvE ")
	if err != nil || got != "informative" {
		t.Fatalf("sanitizeVariantInput() = %q, %v", got, err)
	}
	for _, invalid := range []string{"", "unknown"} {
		if _, err := sanitizeVariantInput(invalid); err == nil {
			t.Fatalf("sanitizeVariantInput(%q) succeeded", invalid)
		}
	}
}

func TestAssetZipSettingValue(t *testing.T) {
	raw := buildZip(t,
		zipEntry{name: "img/logo.png", body: []byte("png")},
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "css/site.css", body: []byte("body{}")},
	)
	encoded, err := assetZipSettingValue(raw)
	if err != nil {
		t.Fatalf("assetZipSettingValue() error = %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("stored ZIP is not the original base64 payload: %v", err)
	}
	if _, err := assetZipSettingValue([]byte("broken")); err == nil {
		t.Fatal("assetZipSettingValue() accepted corrupt ZIP")
	}
}

func TestReadAssetZipFromRequestBoundsInput(t *testing.T) {
	request := multipartTemplateRequest(t, templateForm("save", "modern"), map[string]uploadPart{
		"tpl_assets_zip": {name: "assets.zip", body: bytes.Repeat([]byte("x"), maxAssetCompressedBytes+1)},
	})
	if err := request.ParseMultipartForm(maxTemplateRequestBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := readAssetZipFromRequest(request, "tpl_assets_zip"); err == nil {
		t.Fatal("oversized asset upload accepted")
	}
}

func TestStageTemplateSettingsRejectsBrokenCustom(t *testing.T) {
	current := hotspotSettingsFixture()
	form := templateForm("save", "custom")
	form.Set("tpl_custom_login_html", "<html>broken</html>")
	if _, _, _, err := stageTemplateSettings(current, form, "", nil); err == nil {
		t.Fatal("broken custom login staged")
	}
}

func TestInvalidZipPostLeavesTemplateSettingsUnchanged(t *testing.T) {
	withTestDB(t)
	seedSettings(t, 0, map[string]string{
		"tpl_app_name": "Old", "tpl_variant": "modern",
		"tpl_custom_assets_zip": "old-zip", "tpl_custom_assets_manifest": "old.txt",
	})
	form := templateForm("save", "custom")
	form.Set("tpl_app_name", "New")
	form.Set("tpl_custom_login_html", validCustomLogin)
	postTemplate(t, 0, form, map[string]uploadPart{"tpl_assets_zip": {name: "bad.zip", body: []byte("bad")}})
	assertSettings(t, 0, map[string]string{
		"tpl_app_name": "Old", "tpl_variant": "modern",
		"tpl_custom_assets_zip": "old-zip", "tpl_custom_assets_manifest": "old.txt",
	})
}

func TestValidZipPostStoresGlobalCustomSettingsAndManifest(t *testing.T) {
	withTestDB(t)
	raw := buildZip(t,
		zipEntry{name: "z/image.png", body: []byte("png")},
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "a/site.css", body: []byte("css")},
	)
	form := templateForm("save", "custom")
	form.Set("tpl_info_html", "<b>Info</b>")
	form.Set("tpl_custom_login_html", validCustomLogin)
	form.Set("tpl_custom_status_html", "status-custom")
	form.Set("tpl_custom_logout_html", "logout-custom")
	postTemplate(t, 0, form, map[string]uploadPart{"tpl_assets_zip": {name: "theme.zip", body: raw}})
	settings := database.GetRouterSettings(0)
	decoded, err := base64.StdEncoding.DecodeString(settings["tpl_custom_assets_zip"])
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("stored ZIP invalid: %v", err)
	}
	wantManifest := []string{"a/site.css", "login.html", "z/image.png"}
	if got := strings.Split(settings["tpl_custom_assets_manifest"], "\n"); strings.Join(got, "|") != strings.Join(wantManifest, "|") {
		t.Fatalf("manifest = %q, want %q", got, wantManifest)
	}
	assertSettings(t, 0, map[string]string{
		"tpl_variant": "custom", "tpl_info_html": "<b>Info</b>",
		"tpl_custom_login_html": validCustomLogin, "tpl_custom_status_html": "status-custom",
		"tpl_custom_logout_html": "logout-custom",
	})
}

func TestValidZipPostStoresRouterSpecificSettings(t *testing.T) {
	withTestDB(t)
	routerID := saveTestRouter(t)
	raw := buildZip(t, zipEntry{name: "login.html", body: []byte(validCustomLogin)})
	form := templateForm("save", "custom")
	form.Set("tpl_app_name", "Router Theme")
	form.Set("tpl_custom_login_html", validCustomLogin)
	postTemplate(t, routerID, form, map[string]uploadPart{"tpl_assets_zip": {name: "theme.zip", body: raw}})
	if got := database.GetRouterSetting(routerID, "tpl_app_name"); got != "Router Theme" {
		t.Fatalf("router app name = %q", got)
	}
	if database.GetRouterSetting(routerID, "tpl_custom_assets_zip") == "" {
		t.Fatal("router ZIP was not stored")
	}
}

func TestAssetRemovalAndNoUploadSemantics(t *testing.T) {
	t.Run("remove clears only asset settings", func(t *testing.T) {
		withTestDB(t)
		seedSettings(t, 0, map[string]string{"tpl_custom_assets_zip": "old", "tpl_custom_assets_manifest": "old.txt"})
		form := templateForm("save", "custom")
		form.Set("tpl_custom_login_html", validCustomLogin)
		form.Set("remove_assets", "1")
		postTemplate(t, 0, form, nil)
		assertSettings(t, 0, map[string]string{"tpl_custom_assets_zip": "", "tpl_custom_assets_manifest": "", "tpl_custom_login_html": validCustomLogin})
	})
	t.Run("no upload preserves assets", func(t *testing.T) {
		withTestDB(t)
		raw := buildZip(t, zipEntry{name: "login.html", body: []byte(validCustomLogin)})
		seedSettings(t, 0, map[string]string{"tpl_custom_assets_zip": base64.StdEncoding.EncodeToString(raw), "tpl_custom_assets_manifest": "login.html"})
		form := templateForm("save", "custom")
		form.Set("tpl_custom_login_html", validCustomLogin)
		postTemplate(t, 0, form, nil)
		assertSettings(t, 0, map[string]string{"tpl_custom_assets_zip": base64.StdEncoding.EncodeToString(raw), "tpl_custom_assets_manifest": "login.html"})
	})
}

func TestUnknownActionDoesNotWrite(t *testing.T) {
	withTestDB(t)
	seedSettings(t, 0, map[string]string{"tpl_app_name": "Old", "tpl_variant": "modern"})
	form := templateForm("destroy", "cafe")
	form.Set("tpl_app_name", "New")
	postTemplate(t, 0, form, nil)
	assertSettings(t, 0, map[string]string{"tpl_app_name": "Old", "tpl_variant": "modern"})
}

func TestInvalidCustomAndLogoPostsDoNotWrite(t *testing.T) {
	for _, tc := range []struct {
		name  string
		form  url.Values
		parts map[string]uploadPart
	}{
		{name: "custom", form: func() url.Values {
			f := templateForm("save", "custom")
			f.Set("tpl_custom_login_html", "broken")
			return f
		}()},
		{name: "logo", form: templateForm("save", "cafe"), parts: map[string]uploadPart{"tpl_logo_upload": {name: "logo.txt", body: []byte("not image")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTestDB(t)
			seedSettings(t, 0, map[string]string{"tpl_app_name": "Old", "tpl_variant": "modern", "tpl_logo_url": "old-logo"})
			tc.form.Set("tpl_app_name", "New")
			postTemplate(t, 0, tc.form, tc.parts)
			assertSettings(t, 0, map[string]string{"tpl_app_name": "Old", "tpl_variant": "modern", "tpl_logo_url": "old-logo"})
		})
	}
}

func TestPushValidationFailureLeavesPriorTemplateSettingsUnchanged(t *testing.T) {
	priorAssets := buildZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "old/site.css", body: []byte("old")},
	)
	newAssets := buildZip(t,
		zipEntry{name: "login.html", body: []byte(validCustomLogin)},
		zipEntry{name: "new/site.css", body: []byte("new")},
	)
	for _, tc := range []struct {
		name      string
		form      url.Values
		parts     map[string]uploadPart
		flashText string
	}{
		{
			name: "invalid custom login",
			form: func() url.Values {
				form := templateForm("push", "custom")
				form.Set("tpl_app_name", "Changed invalid custom")
				form.Set("tpl_custom_login_html", "<html>broken</html>")
				return form
			}(),
			flashText: "login.html",
		},
		{
			name: "asset package",
			form: func() url.Values {
				form := templateForm("push", "custom")
				form.Set("tpl_app_name", "Changed assets")
				form.Set("tpl_custom_login_html", validCustomLogin)
				return form
			}(),
			parts:     map[string]uploadPart{"tpl_assets_zip": {name: "new.zip", body: newAssets}},
			flashText: "ZIP",
		},
		{
			name: "oversized rendered file",
			form: func() url.Values {
				form := templateForm("push", "modern")
				form.Set("tpl_app_name", strings.Repeat("x", routerFileContentLimit))
				return form
			}(),
			flashText: "4096",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTestDB(t)
			seedSettings(t, 0, map[string]string{
				"tpl_app_name": "Prior app", "tpl_variant": "modern", "tpl_logo_url": "prior-logo",
				"tpl_custom_login_html":      validCustomLogin,
				"tpl_custom_assets_zip":      base64.StdEncoding.EncodeToString(priorAssets),
				"tpl_custom_assets_manifest": "login.html\nold/site.css",
			})
			before := database.GetRouterSettings(0)
			rec := postTemplateResponse(t, 0, tc.form, tc.parts)
			after := database.GetRouterSettings(0)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("settings changed after rejected push\nbefore: %#v\nafter:  %#v", before, after)
			}
			if flash := responseFlash(t, rec); !strings.Contains(flash, tc.flashText) {
				t.Fatalf("flash = %q, want substring %q", flash, tc.flashText)
			}
		})
	}
}

func TestTemplateSaveWaitsForDedicatedEditLock(t *testing.T) {
	withTestDB(t)
	oldStore := middleware.Store
	middleware.InitSession("template-lock-test-secret")
	t.Cleanup(func() { middleware.Store = oldStore })
	app := templateTestApp()
	app.templateEditMu.Lock()
	done := make(chan struct{})
	req := multipartTemplateRequest(t, templateForm("save", "modern"), nil)
	go func() {
		rec := httptest.NewRecorder()
		app.HandleTemplateEditor(rec, req)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("save completed while template edit lock was held")
	case <-time.After(30 * time.Millisecond):
	}
	app.templateEditMu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("save did not resume after template edit lock release")
	}
}

func TestConcurrentTemplateSavesProduceOneCompleteRequest(t *testing.T) {
	withTestDB(t)
	oldStore := middleware.Store
	middleware.InitSession("template-concurrent-test-secret")
	t.Cleanup(func() { middleware.Store = oldStore })
	app := templateTestApp()
	forms := []url.Values{templateForm("save", "modern"), templateForm("save", "cafe")}
	for i, form := range forms {
		marker := []string{"first", "second"}[i]
		for _, key := range templateSettingKeys {
			if key != "tpl_variant" {
				form.Set(key, marker+":"+key)
			}
		}
		form.Set("tpl_logo_url", marker+":logo")
	}
	start := make(chan struct{})
	requests := make([]*http.Request, len(forms))
	for i, form := range forms {
		requests[i] = multipartTemplateRequest(t, form, nil)
	}
	var wg sync.WaitGroup
	for _, req := range requests {
		req := req
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			app.HandleTemplateEditor(httptest.NewRecorder(), req)
		}()
	}
	close(start)
	wg.Wait()
	got := database.GetRouterSettings(0)
	marker := strings.SplitN(got["tpl_app_name"], ":", 2)[0]
	for _, key := range templateSettingKeys {
		if key == "tpl_variant" {
			continue
		}
		if got[key] != marker+":"+key {
			t.Fatalf("mixed final settings: %s=%q, marker=%q", key, got[key], marker)
		}
	}
	if got["tpl_logo_url"] != marker+":logo" {
		t.Fatalf("mixed logo = %q, marker=%q", got["tpl_logo_url"], marker)
	}
}

func TestTemplatePostContentTypeParsing(t *testing.T) {
	t.Run("mixed case multipart", func(t *testing.T) {
		withTestDB(t)
		oldStore := middleware.Store
		middleware.InitSession("template-content-type-test-secret")
		t.Cleanup(func() { middleware.Store = oldStore })
		req := multipartTemplateRequest(t, templateForm("save", "modern"), nil)
		req.Header.Set("Content-Type", strings.Replace(req.Header.Get("Content-Type"), "multipart/form-data", "Multipart/Form-Data", 1))
		rec := httptest.NewRecorder()
		templateTestApp().HandleTemplateEditor(rec, req)
		if got := database.GetSetting("tpl_variant"); got != "modern" {
			t.Fatalf("tpl_variant = %q, want modern", got)
		}
	})
	t.Run("malformed leaves settings unchanged", func(t *testing.T) {
		withTestDB(t)
		oldStore := middleware.Store
		middleware.InitSession("template-content-type-test-secret")
		t.Cleanup(func() { middleware.Store = oldStore })
		seedSettings(t, 0, map[string]string{"tpl_app_name": "prior", "tpl_variant": "modern"})
		before := database.GetRouterSettings(0)
		req := httptest.NewRequest(http.MethodPost, "/template", strings.NewReader("action=save&tpl_variant=cafe"))
		req.Header.Set("Content-Type", `multipart/form-data; boundary="unterminated`)
		rec := httptest.NewRecorder()
		templateTestApp().HandleTemplateEditor(rec, req)
		if rec.Code != http.StatusSeeOther || !reflect.DeepEqual(database.GetRouterSettings(0), before) {
			t.Fatalf("malformed POST status=%d settings=%#v", rec.Code, database.GetRouterSettings(0))
		}
	})
}

type uploadPart struct {
	name string
	body []byte
}

func templateForm(action, variant string) url.Values {
	form := url.Values{"action": {action}, "tpl_variant": {variant}}
	for _, key := range []string{"tpl_app_name", "tpl_subtitle", "tpl_bg_color", "tpl_primary_color", "tpl_logo_text", "tpl_show_info", "tpl_btn_label", "tpl_login_mode", "tpl_redirect_url", "tpl_dns_name", "tpl_info_html", "tpl_custom_login_html", "tpl_custom_status_html", "tpl_custom_logout_html", "tpl_logo_url"} {
		if !form.Has(key) {
			form.Set(key, "")
		}
	}
	return form
}

func multipartTemplateRequest(t *testing.T, form url.Values, parts map[string]uploadPart) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	for field, part := range parts {
		file, err := writer.CreateFormFile(field, part.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(part.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/template", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func postTemplate(t *testing.T, routerID int, form url.Values, parts map[string]uploadPart) {
	t.Helper()
	postTemplateResponse(t, routerID, form, parts)
}

func postTemplateResponse(t *testing.T, routerID int, form url.Values, parts map[string]uploadPart) *httptest.ResponseRecorder {
	t.Helper()
	oldStore := middleware.Store
	middleware.InitSession("template-save-test-secret")
	t.Cleanup(func() { middleware.Store = oldStore })
	req := multipartTemplateRequest(t, form, parts)
	if routerID > 0 {
		cookieRec := httptest.NewRecorder()
		sess, _ := middleware.Store.Get(req, middleware.SessionName)
		sess.Values["router_id"] = routerID
		if err := sess.Save(req, cookieRec); err != nil {
			t.Fatal(err)
		}
		for _, cookie := range cookieRec.Result().Cookies() {
			req.AddCookie(cookie)
		}
	}
	rec := httptest.NewRecorder()
	templateTestApp().HandleTemplateEditor(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
	}
	return rec
}

func responseFlash(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/template", nil)
	cookies := rec.Result().Cookies()
	for i := len(cookies) - 1; i >= 0; i-- {
		if cookies[i].Name == middleware.SessionName {
			req.AddCookie(cookies[i])
			break
		}
	}
	sess, err := middleware.Store.Get(req, middleware.SessionName)
	if err != nil {
		t.Fatal(err)
	}
	flash, _ := sess.Values["flash"].(string)
	return flash
}

func seedSettings(t *testing.T, routerID int, settings map[string]string) {
	t.Helper()
	if err := database.SetTemplateSettings(routerID, settings); err != nil {
		t.Fatal(err)
	}
}

func assertSettings(t *testing.T, routerID int, want map[string]string) {
	t.Helper()
	got := database.GetRouterSettings(routerID)
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if got[key] != want[key] {
			t.Errorf("%s = %q, want %q", key, got[key], want[key])
		}
	}
}

func saveTestRouter(t *testing.T) int {
	t.Helper()
	router := &database.Router{Name: "test", IP: "127.0.0.1", Port: "8728", Username: "admin"}
	if err := database.SaveRouter(router); err != nil {
		t.Fatal(err)
	}
	return router.ID
}
