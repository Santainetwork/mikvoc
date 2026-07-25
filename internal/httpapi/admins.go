package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"mikvoc/internal/authn"
	"mikvoc/internal/core"
	"mikvoc/internal/middleware"
)

func (a *App) HandleAdmins(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store unavailable", http.StatusInternalServerError)
		return
	}
	admins, err := a.Store.ListAdmins()
	if err != nil {
		a.setFlash(w, r, "Error: "+err.Error())
		admins = nil
	}
	selfID := middleware.AdminIDFromRequest(r)
	a.render(w, r, "admins.html", TemplateData{
		Title:      "Admin — MikVoc",
		ActiveMenu: "admins",
		Data: map[string]any{
			"Admins": admins,
			"SelfID": selfID,
		},
	})
}

func (a *App) HandleAdminCreate(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	role := core.AuditRole(r.FormValue("role"))
	switch role {
	case core.RoleOwner, core.RoleOperator, core.RoleViewer:
	default:
		role = core.RoleViewer
	}
	if username == "" || password == "" {
		a.setFlash(w, r, "Username dan password wajib diisi.")
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	hash, err := authn.HashPassword(password)
	if err != nil {
		a.setFlash(w, r, "Error hash password: "+err.Error())
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	if err := a.Store.CreateAdmin(username, hash, role); err != nil {
		a.setFlash(w, r, "Gagal buat admin: "+err.Error())
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	a.audit(r, "admin_create", username+" ("+string(role)+")")
	a.setFlash(w, r, "Admin ditambahkan.")
	http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
}

func (a *App) HandleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	var id int
	fmt.Sscanf(r.FormValue("id"), "%d", &id)
	if id == 0 {
		a.setFlash(w, r, "Admin tidak valid.")
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	existing, err := a.Store.GetAdminByID(id)
	if err != nil || existing == nil {
		a.setFlash(w, r, "Admin tidak ditemukan.")
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		username = existing.Username
	}
	role := core.AuditRole(r.FormValue("role"))
	switch role {
	case core.RoleOwner, core.RoleOperator, core.RoleViewer:
	default:
		role = existing.Role
	}
	if existing.Role == core.RoleOwner && role != core.RoleOwner {
		n, _ := a.Store.CountOwners()
		if n <= 1 {
			a.setFlash(w, r, "Tidak bisa ubah role: harus ada minimal satu owner.")
			http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
			return
		}
	}
	hash := ""
	if pw := r.FormValue("password"); pw != "" {
		h, herr := authn.HashPassword(pw)
		if herr != nil {
			a.setFlash(w, r, "Error hash password: "+herr.Error())
			http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
			return
		}
		hash = h
	}
	if err := a.Store.UpdateAdmin(id, username, hash, role); err != nil {
		a.setFlash(w, r, "Gagal update: "+err.Error())
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	a.audit(r, "admin_update", fmt.Sprintf("%s id=%d role=%s", username, id, role))
	a.setFlash(w, r, "Admin diperbarui.")
	http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
}

func (a *App) HandleAdminDelete(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	var id int
	fmt.Sscanf(r.FormValue("id"), "%d", &id)
	selfID := middleware.AdminIDFromRequest(r)
	if id == 0 {
		a.setFlash(w, r, "Admin tidak valid.")
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	if id == selfID {
		a.setFlash(w, r, "Tidak bisa hapus akun sendiri.")
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	existing, err := a.Store.GetAdminByID(id)
	if err != nil || existing == nil {
		a.setFlash(w, r, "Admin tidak ditemukan.")
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	if existing.Role == core.RoleOwner {
		n, _ := a.Store.CountOwners()
		if n <= 1 {
			a.setFlash(w, r, "Tidak bisa hapus owner terakhir.")
			http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
			return
		}
	}
	if err := a.Store.DeleteAdmin(id); err != nil {
		a.setFlash(w, r, "Gagal hapus: "+err.Error())
		http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
		return
	}
	a.audit(r, "admin_delete", fmt.Sprintf("%s id=%d", existing.Username, id))
	a.setFlash(w, r, "Admin dihapus.")
	http.Redirect(w, r, "/settings/admins", http.StatusSeeOther)
}

func (a *App) HandleAudit(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		http.Error(w, "store unavailable", http.StatusInternalServerError)
		return
	}
	entries, err := a.Store.ListAudit(100)
	if err != nil {
		a.setFlash(w, r, "Error: "+err.Error())
		entries = nil
	}
	a.render(w, r, "audit.html", TemplateData{
		Title:      "Audit Log — MikVoc",
		ActiveMenu: "audit",
		Data: map[string]any{
			"Entries": entries,
		},
	})
}
