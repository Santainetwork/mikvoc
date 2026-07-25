package middleware

import (
	"net/http"
	"strings"

	"mikvoc/internal/core"
)

func RoleLevel(r core.AuditRole) int {
	switch r {
	case core.RoleOwner:
		return 3
	case core.RoleOperator:
		return 2
	case core.RoleViewer:
		return 1
	default:
		return 0
	}
}

func RoleFromRequest(r *http.Request) core.AuditRole {
	if Store == nil {
		return ""
	}
	sess, err := Store.Get(r, SessionName)
	if err != nil {
		return ""
	}
	if s, ok := sess.Values["admin_role"].(string); ok && s != "" {
		return core.AuditRole(s)
	}
	if sess.Values["authenticated"] == true {
		return core.RoleOwner
	}
	return ""
}

func AdminIDFromRequest(r *http.Request) int {
	if Store == nil {
		return 0
	}
	sess, err := Store.Get(r, SessionName)
	if err != nil {
		return 0
	}
	if id, ok := sess.Values["admin_id"].(int); ok {
		return id
	}
	return 0
}

func AdminNameFromRequest(r *http.Request) string {
	if Store == nil {
		return ""
	}
	sess, err := Store.Get(r, SessionName)
	if err != nil {
		return ""
	}
	if s, ok := sess.Values["admin_user"].(string); ok {
		return s
	}
	return ""
}

func RequireRole(min core.AuditRole) func(http.Handler) http.Handler {
	need := RoleLevel(min)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromRequest(r)
			if RoleLevel(role) < need {
				if wantsJSON(r) {
					http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
					return
				}
				http.Error(w, "Akses ditolak: peran tidak cukup.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireOwner(next http.Handler) http.Handler {
	return RequireRole(core.RoleOwner)(next)
}

func BlockViewerWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		role := RoleFromRequest(r)
		if RoleLevel(role) < RoleLevel(core.RoleOperator) {
			if wantsJSON(r) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			http.Error(w, "Akses ditolak: viewer hanya boleh membaca.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	return strings.HasPrefix(r.URL.Path, "/api/")
}
