package repository

import "mikvoc/internal/core"

type RouterRepo interface {
	ListRouters() ([]core.Router, error)
	GetRouter(id int) (*core.Router, error)
	SaveRouter(r *core.Router) error
	DeleteRouter(id int) error
}

type SettingRepo interface {
	GetSetting(key string) string
	SetSetting(key, value string) error
	GetAllSettings() map[string]string
	GetRouterSetting(routerID int, key string) string
	SetRouterSetting(routerID int, key, value string) error
	GetRouterSettings(routerID int) map[string]string
	SetTemplateSettings(routerID int, updates map[string]string) error
	GetRouterVoucherTemplate(routerID int) string
	SetRouterVoucherTemplate(routerID int, tmplID string) error
}

type AdminRepo interface {
	GetAdmin() (username, passwordHash string)
	SetAdmin(username, passwordHash string) error
	ListAdmins() ([]core.Admin, error)
	GetAdminByUsername(username string) (*core.Admin, error)
	GetAdminByID(id int) (*core.Admin, error)
	CreateAdmin(username, passwordHash string, role core.AuditRole) error
	UpdateAdmin(id int, username, passwordHash string, role core.AuditRole) error
	DeleteAdmin(id int) error
	CountOwners() (int, error)
}

type AuditRepo interface {
	AddAudit(adminID int, adminName, action, target string) error
	ListAudit(limit int) ([]core.AuditEntry, error)
}

type SaleRepo interface {
	AddSale(routerID int, username, profile, comment string, price int) error
	AddSaleWithTime(routerID int, username, profile, comment string, price int, timestamp string) error
	AddSaleWithTimeIdempotent(routerID int, username, profile, comment string, price int, timestamp, sourceKey string) (bool, error)
	GetSales(routerID int, from, to string) ([]core.Sale, error)
	GetSalesTotalByDay(routerID int, from, to string) ([]map[string]interface{}, error)
}

var _ RouterRepo = (*Store)(nil)
var _ SettingRepo = (*Store)(nil)
var _ AdminRepo = (*Store)(nil)
var _ AuditRepo = (*Store)(nil)
var _ SaleRepo = (*Store)(nil)
