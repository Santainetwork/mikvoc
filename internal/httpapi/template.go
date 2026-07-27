package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/gorilla/mux"

	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
)

var (
	previewConditionalToken = regexp.MustCompile(`\$\((?:if [^)]*|else|endif)\)`)
)

const maxLogoUploadBytes = 512 * 1024
const maxTemplateRequestBytes = middleware.TemplateUploadMaxBytes
const previewAssetCSP = "sandbox allow-scripts allow-forms; default-src 'none'; style-src 'unsafe-inline' http: https:; script-src 'unsafe-inline' http: https:; img-src data: blob: http: https:; font-src data: http: https:; form-action 'none'"

var templateSettingKeys = []string{
	"tpl_app_name", "tpl_subtitle", "tpl_bg_color", "tpl_primary_color",
	"tpl_logo_text", "tpl_show_info", "tpl_btn_label", "tpl_login_mode",
	"tpl_redirect_url", "tpl_dns_name", "tpl_variant", "tpl_info_html",
	"tpl_custom_login_html", "tpl_custom_status_html", "tpl_custom_logout_html",
}

type TemplateEditorData struct {
	Settings         map[string]string
	VoucherTemplates []VoucherTemplate
	Variants         []HotspotVariant
	FileInfos        []struct {
		Name     string
		Size     int
		Asset    bool
		TooLarge bool
	}
	AssetNames        []string
	PushBlockedReason string
	RouterID          int
	RouterName        string
}

func templateEditorViewData(routerID int) TemplateEditorData {
	settings := database.GetRouterSettings(routerID)
	defaults := map[string]string{
		"tpl_app_name":      "Hotspot Login",
		"tpl_subtitle":      "Masukkan username dan password untuk akses internet",
		"tpl_bg_color":      "#f1f5f9",
		"tpl_primary_color": "#4f46e5",
		"tpl_logo_text":     "NET",
		"tpl_btn_label":     "Connect",
		"tpl_show_info":     "true",
		"voucher_template":  "classic",
	}
	for key, value := range defaults {
		if settings[key] == "" {
			settings[key] = value
		}
	}
	settings["tpl_variant"] = normalizeHotspotVariant(settings["tpl_variant"])

	data := TemplateEditorData{
		Settings:         settings,
		VoucherTemplates: BuiltinVoucherTemplates,
		Variants:         BuiltinHotspotVariants,
		RouterID:         routerID,
	}
	var pkg templateFileSet
	var err error
	if settings["tpl_variant"] == "custom" {
		pkg, err = storedAssetPackage(settings)
	}
	if err != nil {
		data.PushBlockedReason = err.Error()
		return data
	}
	assembled, err := assembleTemplateFilesWithPackage(settings, &pkg)
	if err != nil {
		data.PushBlockedReason = err.Error()
		return data
	}
	if settings["tpl_variant"] == "custom" {
		for setting, name := range map[string]string{
			"tpl_custom_login_html":  "login.html",
			"tpl_custom_status_html": "status.html",
			"tpl_custom_logout_html": "logout.html",
		} {
			if strings.TrimSpace(settings[setting]) == "" {
				settings[setting] = string(getTemplateFileFold(pkg, name))
			}
		}
	}
	for _, file := range assembled.Files {
		data.FileInfos = append(data.FileInfos, struct {
			Name     string
			Size     int
			Asset    bool
			TooLarge bool
		}{file.Name, len(file.Content), file.Asset, len(file.Content) >= routerFileContentLimit})
		if file.Asset {
			data.AssetNames = append(data.AssetNames, file.Name)
		}
	}
	sort.Slice(data.FileInfos, func(i, j int) bool {
		return strings.ToLower(data.FileInfos[i].Name) < strings.ToLower(data.FileInfos[j].Name)
	})
	sort.Slice(data.AssetNames, func(i, j int) bool {
		return strings.ToLower(data.AssetNames[i]) < strings.ToLower(data.AssetNames[j])
	})
	if err := assembled.PushCheck(); err != nil {
		data.PushBlockedReason = err.Error()
	}
	return data
}

// HandleTemplateEditor renders the template editor page and handles saving/pushing.
// Template settings are stored per-router (router_settings table) with global fallback.
func (a *App) HandleTemplateEditor(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, maxTemplateRequestBytes)
		mediaType := ""
		if contentType := r.Header.Get("Content-Type"); contentType != "" {
			var err error
			mediaType, _, err = mime.ParseMediaType(contentType)
			if err != nil {
				a.setFlash(w, r, "Gagal membaca form template: "+err.Error())
				http.Redirect(w, r, "/template", http.StatusSeeOther)
				return
			}
		}
		if strings.EqualFold(mediaType, "multipart/form-data") {
			if err := r.ParseMultipartForm(maxAssetCompressedBytes + maxLogoUploadBytes + 64*1024); err != nil {
				a.setFlash(w, r, "Gagal membaca form template: "+err.Error())
				http.Redirect(w, r, "/template", http.StatusSeeOther)
				return
			}
		} else if err := r.ParseForm(); err != nil {
			a.setFlash(w, r, "Gagal membaca form template: "+err.Error())
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}

		action := r.FormValue("action")
		routerID := sessionRouterID(r)

		if action != "save" && action != "push" {
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}
		a.templateEditMu.Lock()
		defer a.templateEditMu.Unlock()
		uploadedLogo, err := logoDataURLFromRequest(r, "tpl_logo_upload")
		if err != nil {
			a.setFlash(w, r, "Gagal upload logo: "+err.Error())
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}
		uploadedAssets, err := readAssetZipFromRequest(r, "tpl_assets_zip")
		if err != nil {
			a.setFlash(w, r, "Gagal upload aset: "+err.Error())
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}
		_, updates, assembled, err := stageTemplateSettings(currentHotspotSettings(routerID), r.Form, uploadedLogo, uploadedAssets)
		if err != nil {
			a.setFlash(w, r, "Gagal menyimpan template: "+err.Error())
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}
		var loginHTML, statusHTML, logoutHTML string
		if action == "push" {
			if err = assembled.PushCheck(); err != nil {
				a.setFlash(w, r, "Gagal upload: "+err.Error())
				http.Redirect(w, r, "/template", http.StatusSeeOther)
				return
			}
			loginHTML = string(assembled.Get("login.html"))
			statusHTML = string(assembled.Get("status.html"))
			logoutHTML = string(assembled.Get("logout.html"))
		}
		if err := database.SetTemplateSettings(routerID, updates); err != nil {
			a.setFlash(w, r, "Gagal menyimpan template: "+err.Error())
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}
		a.InvalidateSettingsCache()

		if action == "push" {
			if a.Template != nil {
				hotspotDir, err := a.Template.Push(routerID, loginHTML, statusHTML, logoutHTML)
				if err != nil {
					if hotspotDir == "" {
						a.setFlash(w, r, "Gagal upload: Router tidak terhubung.")
					} else {
						a.setFlash(w, r, "Gagal upload "+err.Error())
					}
				} else {
					a.setFlash(w, r, fmt.Sprintf("Template berhasil diupload ke Mikrotik (%s).", hotspotDir))
				}
			} else {
				cl := a.Pool.Client(sessionRouterID(r))
				if cl == nil || !cl.IsConnected() {
					a.setFlash(w, r, "Gagal upload: Router tidak terhubung.")
				} else {
					hotspotDir := "hotspot"
					profiles, err := cl.Run("/ip/hotspot/profile/print")
					if err == nil {
						for _, p := range profiles {
							dir := p["html-directory"]
							if dir != "" {
								hotspotDir = dir
								break
							}
						}
					}

					err1 := cl.SetFileContent(hotspotDir+"/login.html", loginHTML)
					err2 := cl.SetFileContent(hotspotDir+"/status.html", statusHTML)
					err3 := cl.SetFileContent(hotspotDir+"/logout.html", logoutHTML)

					if err1 != nil {
						a.setFlash(w, r, "Gagal upload login.html ("+hotspotDir+"): "+err1.Error())
					} else if err2 != nil {
						a.setFlash(w, r, "Gagal upload status.html ("+hotspotDir+"): "+err2.Error())
					} else if err3 != nil {
						a.setFlash(w, r, "Gagal upload logout.html ("+hotspotDir+"): "+err3.Error())
					} else {
						a.setFlash(w, r, fmt.Sprintf("Template berhasil diupload ke Mikrotik (%s).", hotspotDir))
					}
				}
			}
		} else {
			a.setFlash(w, r, "Pengaturan template berhasil disimpan untuk router ini.")
		}

		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	routerID := sessionRouterID(r)
	data := templateEditorViewData(routerID)
	if rt := a.routerFor(r); rt != nil {
		data.RouterName = rt.Name
	}

	a.render(w, r, "template_editor.html", TemplateData{
		Title:      "Template Editor — MikVoc",
		ActiveMenu: "template",
		Data:       data,
	})
}

func sanitizeVariantInput(raw string) (string, error) {
	variant := strings.ToLower(strings.TrimSpace(raw))
	if !validHotspotVariant(variant) {
		return "", fmt.Errorf("varian template tidak dikenal: %q", raw)
	}
	return variant, nil
}

func assetZipSettingValue(raw []byte) (string, error) {
	if _, err := validateAssetZip(raw); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func readAssetZipFromRequest(r *http.Request, field string) ([]byte, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) || strings.Contains(err.Error(), "multipart") {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxAssetCompressedBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > maxAssetCompressedBytes {
		return nil, fmt.Errorf("ukuran ZIP maksimal %d MiB", maxAssetCompressedBytes>>20)
	}
	return raw, nil
}

func stageTemplateSettings(current map[string]string, form url.Values, uploadedLogo string, uploadedAssets []byte) (map[string]string, map[string]string, templateFileSet, error) {
	staged := make(map[string]string, len(current)+len(templateSettingKeys)+3)
	for key, value := range current {
		staged[key] = value
	}
	updates := make(map[string]string, len(templateSettingKeys)+3)
	for _, key := range templateSettingKeys {
		updates[key] = form.Get(key)
		staged[key] = updates[key]
	}
	variant, err := sanitizeVariantInput(form.Get("tpl_variant"))
	if err != nil {
		return nil, nil, templateFileSet{}, err
	}
	updates["tpl_variant"] = variant
	staged["tpl_variant"] = variant
	logo := uploadedLogo
	if logo == "" {
		logo = strings.TrimSpace(form.Get("tpl_logo_url"))
	}
	updates["tpl_logo_url"] = logo
	staged["tpl_logo_url"] = logo
	var uploadedPackage *templateFileSet
	if len(uploadedAssets) > 0 {
		pkg, err := validateAssetZip(uploadedAssets)
		if err != nil {
			return nil, nil, templateFileSet{}, err
		}
		uploadedPackage = &pkg
		names := make([]string, len(pkg.Files))
		for i, file := range pkg.Files {
			names[i] = file.Name
		}
		sort.Strings(names)
		updates["tpl_custom_assets_zip"] = base64.StdEncoding.EncodeToString(uploadedAssets)
		updates["tpl_custom_assets_manifest"] = strings.Join(names, "\n")
		staged["tpl_custom_assets_zip"] = updates["tpl_custom_assets_zip"]
		staged["tpl_custom_assets_manifest"] = updates["tpl_custom_assets_manifest"]
	} else if form.Get("remove_assets") == "1" {
		updates["tpl_custom_assets_zip"] = ""
		updates["tpl_custom_assets_manifest"] = ""
		staged["tpl_custom_assets_zip"] = ""
		staged["tpl_custom_assets_manifest"] = ""
		empty := templateFileSet{}
		uploadedPackage = &empty
	}
	assembled, err := assembleTemplateFilesWithPackage(staged, uploadedPackage)
	if err != nil {
		return nil, nil, templateFileSet{}, err
	}
	return staged, updates, assembled, nil
}

func logoDataURLFromRequest(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		// No file uploaded (OK), or form was URL-encoded (no multipart parts at all).
		if errors.Is(err, http.ErrMissingFile) || strings.Contains(err.Error(), "multipart") {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxLogoUploadBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", nil
	}
	if len(data) > maxLogoUploadBytes {
		return "", fmt.Errorf("ukuran file maksimal %d KB", maxLogoUploadBytes/1024)
	}
	return logoDataURL(header.Filename, data)
}

func logoDataURL(filename string, data []byte) (string, error) {
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("file %s bukan gambar", filename)
	}
	if contentType == "image/svg+xml" {
		return "", fmt.Errorf("SVG tidak didukung untuk upload logo")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// HandleSetVoucherTemplate saves the selected voucher print template for the active router.
func (a *App) HandleSetVoucherTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	tmplID := r.FormValue("voucher_template")
	// Validate it's a known ID
	valid := false
	for _, t := range BuiltinVoucherTemplates {
		if t.ID == tmplID {
			valid = true
			break
		}
	}
	if !valid {
		tmplID = "classic"
	}
	routerID := sessionRouterID(r)
	if err := database.SetRouterVoucherTemplate(routerID, tmplID); err != nil {
		a.setFlash(w, r, "Gagal simpan voucher template: "+err.Error())
	} else {
		a.setFlash(w, r, "Template cetak voucher diperbarui untuk router ini: "+tmplID)
	}
	a.InvalidateSettingsCache()
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}

func (a *App) HandleTemplatePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Preview Template Hotspot</title><style>*{box-sizing:border-box}html,body,iframe{width:100%;height:100%;margin:0;border:0}body{overflow:hidden}</style></head><body><iframe title="Preview Template Hotspot" src="/template/preview/frame" sandbox="allow-scripts allow-forms"></iframe></body></html>`))
}

func (a *App) HandleTemplatePreviewFrame(w http.ResponseWriter, r *http.Request) {
	routerID := sessionRouterID(r)
	loginHTML, _, _ := generateHotspotHTMLFor(routerID)
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline' https: http:; script-src 'unsafe-inline' https: http:; img-src data: blob: https: http:; font-src data: https: http:; form-action 'none'; frame-ancestors 'self'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(injectPreviewAssetBase(previewHotspotHTML(loginHTML))))
}

func (a *App) HandleTemplatePreviewAsset(w http.ResponseWriter, r *http.Request) {
	name, err := normalizeAssetName(mux.Vars(r)["path"])
	if err != nil || isStandardHotspotFile(name) || strings.EqualFold(name, "hotspot") || strings.HasPrefix(strings.ToLower(name), "hotspot/") {
		http.NotFound(w, r)
		return
	}
	set, err := assembleTemplateFiles(currentHotspotSettings(sessionRouterID(r)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var asset *templateFile
	for i := range set.Files {
		if set.Files[i].Asset && set.Files[i].Name == name {
			asset = &set.Files[i]
			break
		}
	}
	if asset == nil {
		for i := range set.Files {
			if set.Files[i].Asset && strings.EqualFold(set.Files[i].Name, name) {
				asset = &set.Files[i]
				break
			}
		}
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(asset.Name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", previewAssetCSP)
	_, _ = w.Write(asset.Content)
}

func injectPreviewAssetBase(html string) string {
	const base = `<base href="/template/preview/assets/">`
	htmlEnd := -1
	raw := ""
	for pos := 0; pos < len(html); {
		rel := strings.IndexByte(html[pos:], '<')
		if rel < 0 {
			break
		}
		start := pos + rel
		if strings.HasPrefix(html[start:], "<!--") {
			end := strings.Index(html[start+4:], "-->")
			if end < 0 {
				break
			}
			pos = start + 4 + end + 3
			continue
		}
		tag, next, ok := scanHTMLTag(html, start)
		if !ok {
			pos = next
			continue
		}
		pos = next
		if raw != "" {
			if tag.end && tag.name == raw {
				raw = ""
			}
			continue
		}
		if !tag.end && (tag.name == "script" || tag.name == "style" || tag.name == "textarea") {
			if !tag.selfClosing {
				raw = tag.name
			}
			continue
		}
		if tag.end {
			continue
		}
		if tag.name == "html" && htmlEnd < 0 {
			htmlEnd = next
		}
		if tag.name == "head" {
			return html[:next] + base + html[next:]
		}
	}
	if htmlEnd >= 0 {
		return html[:htmlEnd] + base + html[htmlEnd:]
	}
	return base + html
}

func previewHotspotHTML(html string) string {
	replacer := strings.NewReplacer(
		"$(if error)", "",
		"$(if chap-id)", "",
		"$(endif)", "",
		"$(error)", "Username atau password salah",
		"$(chap-id)", "preview-chap-id",
		"$(chap-challenge)", "preview-challenge",
		"$(link-login-only)", "#",
		"$(link-login)", "#",
		"$(link-logout)", "#",
		"$(link-orig)", "#",
		"$(username)", "preview-user",
		"$(ip)", "192.168.10.2",
		"$(mac)", "AA:BB:CC:DD:EE:FF",
		"$(uptime)", "1h23m",
		"$(bytes-in-nice)", "12.3 MiB",
		"$(bytes-out-nice)", "4.5 MiB",
	)
	return previewConditionalToken.ReplaceAllString(replacer.Replace(html), "")
}

// generateHotspotHTML generates login/status/logout HTML using the active router's settings.
func generateHotspotHTML() (login, status, logout string) {
	return generateHotspotHTMLFor(0)
}

// generateHotspotHTMLFor generates HTML using settings for a specific router (0 = global fallback).
func generateHotspotHTMLFor(routerID int) (login, status, logout string) {
	settings := currentHotspotSettings(routerID)
	variant := normalizeHotspotVariant(settings["tpl_variant"])
	if variant == "custom" {
		return customHotspotHTMLFor(settings)
	}
	v := hotspotViewFrom(settings)
	return renderVariantLogin(variant, settings), renderHotspotStatus(v), renderHotspotLogout(v)
}
