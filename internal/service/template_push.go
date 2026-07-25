package service

import "fmt"

type TemplateService struct {
	pool *Pool
}

func NewTemplate(pool *Pool) *TemplateService {
	return &TemplateService{pool: pool}
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
