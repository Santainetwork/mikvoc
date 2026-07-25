package httpapi

import (
	"net/http"

	"github.com/gorilla/mux"

	"mikvoc/internal/middleware"
)

func (a *App) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/healthz", a.HandleHealthz).Methods(http.MethodGet)
	r.HandleFunc("/metrics", a.HandleMetrics).Methods(http.MethodGet)

	loginLimiter := middleware.NewLoginLimiter()
	r.HandleFunc("/login", loginLimiter.Wrap(a.HandleLogin)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/logout", a.HandleLogout).Methods(http.MethodGet, http.MethodPost)

	protected := r.NewRoute().Subrouter()
	protected.Use(middleware.RequireAuth)
	protected.Use(middleware.BlockViewerWrites)

	protected.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/routers", http.StatusSeeOther)
	}).Methods(http.MethodGet)

	protected.HandleFunc("/routers", a.HandleRouters).Methods(http.MethodGet)
	protected.HandleFunc("/dashboard", a.HandleDashboard).Methods(http.MethodGet)
	protected.HandleFunc("/api/traffic", a.HandleTrafficAPI).Methods(http.MethodGet)
	protected.HandleFunc("/api/dashboard", a.HandleDashboardAPI).Methods(http.MethodGet)
	protected.HandleFunc("/api/test-router", a.HandleTestRouterAPI).Methods(http.MethodPost)

	protected.HandleFunc("/api/hotspot/users", a.HandleUsersJSON).Methods(http.MethodGet)
	protected.HandleFunc("/api/hotspot/user", a.HandleUserGet).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/users", a.HandleUsers).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/users/print", a.HandlePrint).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/users/export-csv", a.HandleExportCSV).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/users/import-csv", a.HandleImportCSV).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/export-script", a.HandleExportScript).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/users/quickprint", a.HandleQuickPrint).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/users/edit", a.HandleUserEdit).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/bulk-disable", a.HandleUserBulkDisable).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/bulk-profile", a.HandleUserBulkProfile).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/remove", a.HandleUserRemove).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/disable", a.HandleUserDisable).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/reset", a.HandleUserReset).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/remove-expired", a.HandleRemoveExpired).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/users/remove-comment", a.HandleRemoveComment).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/generate", a.HandleGenerate).Methods(http.MethodGet, http.MethodPost)
	protected.HandleFunc("/hotspot/active", a.HandleActiveUsers).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/active/kick", a.HandleKickUser).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/profiles", a.HandleProfiles).Methods(http.MethodGet)
	protected.HandleFunc("/hotspot/profiles/create", a.HandleProfileCreate).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/profiles/clone", a.HandleProfileClone).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/profiles/update", a.HandleProfileUpdate).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/profiles/remove", a.HandleProfileRemove).Methods(http.MethodPost)
	protected.HandleFunc("/hotspot/profiles/monitor", a.HandleSetMonitorProfile).Methods(http.MethodPost)

	protected.HandleFunc("/report", a.HandleReport).Methods(http.MethodGet)
	protected.HandleFunc("/report/purge-scripts", a.HandleSalesPurge).Methods(http.MethodPost)

	protected.HandleFunc("/ppp/secrets", a.HandlePPPSecrets).Methods(http.MethodGet)
	protected.HandleFunc("/ppp/secrets/create", a.HandlePPPSecretCreate).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/secrets/edit", a.HandlePPPSecretEdit).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/secrets/remove", a.HandlePPPSecretRemove).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/secrets/disable", a.HandlePPPSecretDisable).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/secrets/export-csv", a.HandlePPPExportCSV).Methods(http.MethodGet)
	protected.HandleFunc("/ppp/secrets/import-csv", a.HandlePPPImportCSV).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/generate", a.HandlePPPGenerate).Methods(http.MethodGet, http.MethodPost)
	protected.HandleFunc("/ppp/active", a.HandlePPPActive).Methods(http.MethodGet)
	protected.HandleFunc("/ppp/active/kick", a.HandlePPPKick).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/profiles", a.HandlePPPProfiles).Methods(http.MethodGet)
	protected.HandleFunc("/ppp/profiles/create", a.HandlePPPProfileCreate).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/profiles/update", a.HandlePPPProfileUpdate).Methods(http.MethodPost)
	protected.HandleFunc("/ppp/profiles/remove", a.HandlePPPProfileRemove).Methods(http.MethodPost)

	protected.HandleFunc("/settings", a.HandleSettings).Methods(http.MethodGet, http.MethodPost)
	protected.HandleFunc("/settings/backup", a.HandleBackup).Methods(http.MethodGet)
	protected.HandleFunc("/settings/restore", a.HandleRestore).Methods(http.MethodPost)
	protected.HandleFunc("/settings/switch-router", a.HandleSwitchRouter).Methods(http.MethodPost)

	protected.HandleFunc("/settings/audit", a.HandleAudit).Methods(http.MethodGet)

	ownerOnly := protected.PathPrefix("/settings/admins").Subrouter()
	ownerOnly.Use(middleware.RequireOwner)
	ownerOnly.HandleFunc("", a.HandleAdmins).Methods(http.MethodGet)
	ownerOnly.HandleFunc("/", a.HandleAdmins).Methods(http.MethodGet)
	ownerOnly.HandleFunc("/create", a.HandleAdminCreate).Methods(http.MethodPost)
	ownerOnly.HandleFunc("/update", a.HandleAdminUpdate).Methods(http.MethodPost)
	ownerOnly.HandleFunc("/delete", a.HandleAdminDelete).Methods(http.MethodPost)

	protected.HandleFunc("/template", a.HandleTemplateEditor).Methods(http.MethodGet, http.MethodPost)
	protected.HandleFunc("/template/download", a.HandleTemplateDownload).Methods(http.MethodGet)
	protected.HandleFunc("/template/voucher-template", a.HandleSetVoucherTemplate).Methods(http.MethodPost)
	protected.HandleFunc("/template/preview/assets/{path:.*}", a.HandleTemplatePreviewAsset).Methods(http.MethodGet)
	protected.HandleFunc("/template/preview/frame", a.HandleTemplatePreviewFrame).Methods(http.MethodGet)
	protected.HandleFunc("/template/preview", a.HandleTemplatePreview).Methods(http.MethodGet)
}
