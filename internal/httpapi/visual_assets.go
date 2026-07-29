package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	
	"mikvoc/internal/assets"
)

// HandleBackgroundUpload handles POST /template/background-upload uploads background images
func (a *App) HandleBackgroundUpload(w http.ResponseWriter, r *http.Request) {
	if a.Template == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if a.Assets == nil {
		http.Error(w, "asset store unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	routerID := sessionRouterID(r)
	a.templateEditMu.Lock()
	defer a.templateEditMu.Unlock()

	file, header, err := r.FormFile("background")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) || strings.Contains(err.Error(), "multipart") {
			http.Redirect(w, r, "/template", http.StatusSeeOther)
			return
		}
		a.setFlash(w, r, "Gagal baca file: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBackgroundUploadBytes+1))
	if err != nil {
		a.setFlash(w, r, "Error membaca file: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	if len(data) == 0 {
		a.setFlash(w, r, "File tidak valid atau kosong")
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	if len(data) > maxBackgroundUploadBytes {
		a.setFlash(w, r, fmt.Sprintf("Ukuran file maksimal %d MiB", maxBackgroundUploadBytes>>20))
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	contentType := http.DetectContentType(data)
	validTypes := []string{"image/gif", "image/png", "image/jpeg"}
	valid := false
	for _, t := range validTypes {
		if contentType == t {
			valid = true
			break
		}
	}
	if !valid {
		a.setFlash(w, r, fmt.Sprintf("Format %s tidak didukung. Gunakan GIF/PNG/JPEG", header.Filename))
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	uuidStr := uuid.New().String()
	ext := ".gif"
	if strings.HasPrefix(contentType, "image/png") {
		ext = ".png"
	} else if strings.HasPrefix(contentType, "image/jpeg") {
		ext = ".jpg"
	}
	storageKey := fmt.Sprintf("local/background-%s%s", uuidStr, ext)
	
	asset, err := a.Assets.Write(routerID, assets.Background, bytes.NewReader(data), maxBackgroundUploadBytes)
	if err != nil {
		a.setFlash(w, r, "Gagal simpan aset: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	// Update storage key with actual extension from write
	storageKey = asset.Path
	
	focalX := strings.TrimSpace(r.FormValue("focalX"))
	focalY := strings.TrimSpace(r.FormValue("focalY"))
	
	var focalXInt, focalYInt int
	_, err = fmt.Sscanf(focalX, "%d", &focalXInt)
	if err != nil || focalXInt < 0 || focalXInt > 100 {
		focalXInt = 50
	}
	_, err = fmt.Sscanf(focalY, "%d", &focalYInt)
	if err != nil || focalYInt < 0 || focalYInt > 100 {
		focalYInt = 50
	}

	updates := map[string]string{
		"tpl_bg_image":    storageKey,
		"tpl_focal_x":     strconv.Itoa(focalXInt),
		"tpl_focal_y":     strconv.Itoa(focalYInt),
	}
	
	settings := a.currentHotspotSettings(routerID)
	for k, v := range settings {
		if _, exists := updates[k]; !exists {
			updates[k] = v
		}
	}
	
	if err := a.Template.Save(routerID, updates); err != nil {
		a.setFlash(w, r, "Gagal menyimpan template: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	a.InvalidateSettingsCache()
	a.setFlash(w, r, "Background berhasil diupload dan disimpan")
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}

// HandleFocalPosition handles focal point adjustments
func (a *App) HandleFocalPosition(w http.ResponseWriter, r *http.Request) {
	if a.Template == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if a.Assets == nil {
		http.Error(w, "asset store unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	routerID := sessionRouterID(r)
	a.templateEditMu.Lock()
	defer a.templateEditMu.Unlock()

	focalXStr := strings.TrimSpace(r.FormValue("focalX"))
	focalYStr := strings.TrimSpace(r.FormValue("focalY"))

	var focalX, focalY int
	
	if focalXStr == "" || focalYStr == "" {
		a.setFlash(w, r, "Sumbu X dan Y harus diisi")
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	_, err := fmt.Sscanf(focalXStr, "%d", &focalX)
	if err != nil || focalX < 0 || focalX > 100 {
		a.setFlash(w, r, "Nilai sumbu X harus angka 0-100")
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	_, err = fmt.Sscanf(focalYStr, "%d", &focalY)
	if err != nil || focalY < 0 || focalY > 100 {
		a.setFlash(w, r, "Nilai sumbu Y harus angka 0-100")
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	if focalX < 0 {
		focalX = 0
	} else if focalX > 100 {
		focalX = 100
	}
	if focalY < 0 {
		focalY = 0
	} else if focalY > 100 {
		focalY = 100
	}

	updates := map[string]string{
		"tpl_focal_x": strconv.Itoa(focalX),
		"tpl_focal_y": strconv.Itoa(focalY),
	}
	
	settings := a.currentHotspotSettings(routerID)
	for k, v := range settings {
		if _, exists := updates[k]; !exists {
			updates[k] = v
		}
	}
	
	if err := a.Template.Save(routerID, updates); err != nil {
		a.setFlash(w, r, "Gagal menyimpan posisi focal: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	a.InvalidateSettingsCache()
	a.setFlash(w, r, "Posisi focal point diperbarui")
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}

// HandleLogoUpload handles POST /template/logo-upload uploads logo images
func (a *App) HandleLogoUpload(w http.ResponseWriter, r *http.Request) {
	if a.Template == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if a.Assets == nil {
		http.Error(w, "asset store unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	routerID := sessionRouterID(r)
	a.templateEditMu.Lock()
	defer a.templateEditMu.Unlock()

	data, err := logoDataURLFromRequest(r, "logo")
	if err != nil {
		a.setFlash(w, r, "Gagal upload logo: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	if data == "" {
		// No file uploaded - OK
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}

	updates := map[string]string{
		"tpl_logo_url": data,
	}
	
	settings := a.currentHotspotSettings(routerID)
	for k, v := range settings {
		if _, exists := updates[k]; !exists {
			updates[k] = v
		}
	}
	
	if err := a.Template.Save(routerID, updates); err != nil {
		a.setFlash(w, r, "Gagal menyimpan template: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	
	a.InvalidateSettingsCache()
	a.setFlash(w, r, "Logo berhasil diupload dan disimpan")
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}

// HandleServeManagedAsset serves managed assets (background, logo) from local storage
func (a *App) HandleServeManagedAsset(w http.ResponseWriter, r *http.Request) {
	if a.Template == nil || a.Assets == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	
	vars := mux.Vars(r)
	scope := vars["scope"]  // "global" or router ID
	kind := vars["kind"]   // "background" or "logo"
	ext := vars["ext"]     // "jpg" or "png" or "gif"
	
	// Validate inputs - scope must be "global" or numeric router ID (no leading zeros)
	routerID := 0
	if scope != "global" {
		if _, err := fmt.Sscanf(scope, "%d", &routerID); err != nil || routerID <= 0 || len(scope) > 1 && scope[0] == '0' {
			http.NotFound(w, r)
			return
		}
	}
	
	if kind != "background" && kind != "logo" {
		http.NotFound(w, r)
		return
	}
	
	if ext != "jpg" && ext != "png" && ext != "gif" {
		http.NotFound(w, r)
		return
	}
	
	assetKind := assets.Kind(kind)
	asset, err := a.Assets.Read(routerID, assetKind)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	
	// Set proper headers for security and cache control
	contentType := mime.TypeByExtension("." + ext)
	if contentType == "" {
		switch ext {
		case "jpg":
			contentType = "image/jpeg"
		case "png":
			contentType = "image/png"
		case "gif":
			contentType = "image/gif"
		default:
			contentType = "application/octet-stream"
		}
	}
	
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	// Add CSP with form-action restriction
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; script-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'")
	
	_, _ = w.Write(asset.Bytes)
}
