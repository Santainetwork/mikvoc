package service

import (
	"fmt"

	"mikvoc/internal/repository"
)

type TemplateService struct {
	pool     *Pool
	settings repository.SettingRepo
}

func NewTemplate(pool *Pool, settings repository.SettingRepo) *TemplateService {
	return &TemplateService{pool: pool, settings: settings}
}

func (t *TemplateService) Settings(routerID int) map[string]string {
	return t.settings.GetRouterSettings(routerID)
}

func (t *TemplateService) Save(routerID int, updates map[string]string) error {
	return t.settings.SetTemplateSettings(routerID, updates)
}

func (t *TemplateService) VoucherTemplate(routerID int) string {
	return t.settings.GetRouterVoucherTemplate(routerID)
}

func (t *TemplateService) SetVoucherTemplate(routerID int, templateID string) error {
	return t.settings.SetRouterVoucherTemplate(routerID, templateID)
}

func (t *TemplateService) Push(routerID int, loginHTML, statusHTML, logoutHTML string) (string, error) {
	cl, err := t.pool.RequireClient(routerID)
	if err != nil {
		return "", err
	}
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
	if err := cl.SetFileContent(hotspotDir+"/login.html", loginHTML); err != nil {
		return hotspotDir, fmt.Errorf("login.html (%s): %w", hotspotDir, err)
	}
	if err := cl.SetFileContent(hotspotDir+"/status.html", statusHTML); err != nil {
		return hotspotDir, fmt.Errorf("status.html (%s): %w", hotspotDir, err)
	}
	if err := cl.SetFileContent(hotspotDir+"/logout.html", logoutHTML); err != nil {
		return hotspotDir, fmt.Errorf("logout.html (%s): %w", hotspotDir, err)
	}
	return hotspotDir, nil
}
