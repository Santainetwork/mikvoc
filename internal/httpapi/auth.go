package httpapi

import (
	"net/http"

	"mikvoc/internal/middleware"
)

func (a *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	sess, _ := middleware.Store.Get(r, middleware.SessionName)
	if auth, ok := sess.Values["authenticated"].(bool); ok && auth {
		http.Redirect(w, r, "/routers", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if a.Auth == nil {
			a.renderStandalone(w, r, "login.html", TemplateData{
				Title: "Login — MikVoc",
				Flash: "Auth service tidak tersedia.",
			})
			return
		}

		admin, err := a.Auth.Login(username, password)
		if err != nil || admin == nil {
			a.renderStandalone(w, r, "login.html", TemplateData{
				Title: "Login — MikVoc",
				Flash: "Username atau password salah.",
			})
			return
		}

		// Bind CSRF into the same session write as auth so later POSTs validate.
		csrfTok := middleware.EnsureCSRFToken(w, r)
		sess, _ = middleware.Store.Get(r, middleware.SessionName)
		sess.Values["authenticated"] = true
		sess.Values["admin_user"] = admin.Username
		sess.Values["admin_id"] = admin.ID
		sess.Values["admin_role"] = string(admin.Role)
		if csrfTok != "" {
			sess.Values["csrf_token"] = csrfTok
		}
		_ = sess.Save(r, w)
		a.audit(r, "login", admin.Username)
		http.Redirect(w, r, "/routers", http.StatusSeeOther)
		return
	}
	a.renderStandalone(w, r, "login.html", TemplateData{Title: "Login — MikVoc"})
}

func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := middleware.Store.Get(r, middleware.SessionName)
	sess.Options.MaxAge = -1
	_ = sess.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
