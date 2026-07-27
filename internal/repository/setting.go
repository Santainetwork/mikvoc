package repository

import "mikvoc/internal/database"

func (s *Store) GetSetting(key string) string {
	return database.GetSetting(key)
}

func (s *Store) SetSetting(key, value string) error {
	return database.SetSetting(key, value)
}

func (s *Store) GetAllSettings() map[string]string {
	return database.GetAllSettings()
}

func (s *Store) GetRouterSetting(routerID int, key string) string {
	return database.GetRouterSetting(routerID, key)
}

func (s *Store) SetRouterSetting(routerID int, key, value string) error {
	return database.SetRouterSetting(routerID, key, value)
}

func (s *Store) GetRouterSettings(routerID int) map[string]string {
	return database.GetRouterSettings(routerID)
}

func (s *Store) SetTemplateSettings(routerID int, updates map[string]string) error {
	return database.SetTemplateSettings(routerID, updates)
}

func (s *Store) GetRouterVoucherTemplate(routerID int) string {
	return database.GetRouterVoucherTemplate(routerID)
}

func (s *Store) SetRouterVoucherTemplate(routerID int, tmplID string) error {
	return database.SetRouterVoucherTemplate(routerID, tmplID)
}
