package httpapi

import (
	"fmt"
	"net/http"
	"strings"
)

func buildProfileCloneParams(source map[string]string, newName string) (map[string]string, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, fmt.Errorf("nama profil baru wajib diisi")
	}

	params := map[string]string{"name": newName}
	skip := map[string]struct{}{
		".id":     {},
		"name":    {},
		"numbers": {},
		"default": {},
		"dynamic": {},
		"invalid": {},
	}
	for key, value := range source {
		if _, ok := skip[key]; ok {
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		params[key] = value
	}
	return params, nil
}

func (a *App) HandleProfileClone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	sourceID := strings.TrimSpace(r.FormValue("source_id"))
	newName := strings.TrimSpace(r.FormValue("name"))
	if sourceID == "" {
		a.setFlash(w, r, "Error: Sumber profil tidak valid.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}
	if newName == "" {
		a.setFlash(w, r, "Error: Nama profil baru wajib diisi.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	if a.Profiles != nil {
		if err := a.Profiles.Clone(sessionRouterID(r), sourceID, newName); err != nil {
			a.setFlash(w, r, "Error: Gagal clone profil ("+err.Error()+")")
		} else {
			a.setFlash(w, r, "Success: Profil berhasil di-clone menjadi "+newName+".")
		}
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	cl := a.clientFor(r)
	if cl == nil || !cl.IsConnected() {
		a.setFlash(w, r, "Error: Tidak terhubung ke router.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	rows, err := cl.Run("/ip/hotspot/user/profile/print", map[string]string{"?.id": sourceID})
	if err != nil {
		a.setFlash(w, r, "Error: Gagal membaca profil sumber ("+err.Error()+")")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}
	if len(rows) == 0 {
		a.setFlash(w, r, "Error: Profil sumber tidak ditemukan.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	params, err := buildProfileCloneParams(rows[0], newName)
	if err != nil {
		a.setFlash(w, r, "Error: "+err.Error())
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	if _, err := cl.Run("/ip/hotspot/user/profile/add", params); err != nil {
		a.setFlash(w, r, "Error: Gagal clone profil ("+err.Error()+")")
	} else {
		a.setFlash(w, r, "Success: Profil berhasil di-clone menjadi "+newName+".")
	}
	http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
}
