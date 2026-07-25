package service

import (
	"fmt"
	"strings"

	"mikvoc/internal/core"
)

type ProfileService struct {
	pool *Pool
}

func NewProfile(pool *Pool) *ProfileService {
	return &ProfileService{pool: pool}
}

func (s *ProfileService) List(routerID int) ([]core.UserProfile, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	profiles, err := cl.GetProfiles()
	if err != nil {
		return nil, err
	}
	out := make([]core.UserProfile, len(profiles))
	for i, p := range profiles {
		out[i] = toCoreProfile(p)
	}
	return out, nil
}

func (s *ProfileService) Create(routerID int, name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.CreateProfile(name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod)
}

func (s *ProfileService) Update(routerID int, id, name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.UpdateProfile(id, name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod)
}

func (s *ProfileService) ListPools(routerID int) ([]string, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.GetIPPools()
}

func (s *ProfileService) ListQueues(routerID int) ([]string, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.GetSimpleQueues()
}

func (s *ProfileService) Remove(routerID int, id string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.RemoveProfile(id)
}

func (s *ProfileService) Clone(routerID int, sourceID, newName string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("nama profil baru wajib diisi")
	}
	if strings.TrimSpace(sourceID) == "" {
		return core.ErrInvalidInput
	}
	rows, err := cl.Run("/ip/hotspot/user/profile/print", map[string]string{"?.id": sourceID})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return core.ErrNotFound
	}
	params, err := buildProfileCloneParams(rows[0], newName)
	if err != nil {
		return err
	}
	_, err = cl.Run("/ip/hotspot/user/profile/add", params)
	return err
}

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

func (s *ProfileService) SetMonitor(routerID int, profileName string, enable bool) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	if profileName == "" {
		return core.ErrInvalidInput
	}
	schedulerName := "mikvoc-monitor-" + strings.ReplaceAll(profileName, " ", "_")

	if !enable {
		rows, err := cl.Run("/system/scheduler/print", map[string]string{"?name": schedulerName})
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			_, err = cl.Run("/system/scheduler/remove", map[string]string{".id": rows[0][".id"]})
			return err
		}
		return nil
	}

	script := `:local profileName "` + profileName + `"
:foreach u in=[/ip hotspot user find profile=$profileName] do={
  :local uptime [/ip hotspot user get $u uptime]
  :local limitUptime [/ip hotspot user get $u limit-uptime]
  :if ($limitUptime != "0s" and $uptime >= $limitUptime) do={
    /ip hotspot user remove $u
  }
}`
	rows, err := cl.Run("/system/scheduler/print", map[string]string{"?name": schedulerName})
	if err == nil && len(rows) > 0 {
		_, err = cl.Run("/system/scheduler/set", map[string]string{
			".id":      rows[0][".id"],
			"on-event": script,
		})
		return err
	}
	_, err = cl.Run("/system/scheduler/add", map[string]string{
		"name":     schedulerName,
		"interval": "01:00:00",
		"on-event": script,
		"comment":  "MikVoc Monitor: " + profileName,
	})
	return err
}
