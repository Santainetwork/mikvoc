package service

import (
	"net"
	"strings"

	"mikvoc/internal/core"
)

type RouterManagementService struct {
	pool *Pool
}

func NewRouterManagement(pool *Pool) *RouterManagementService {
	if pool == nil {
		pool = NewPool()
	}
	return &RouterManagementService{pool: pool}
}

func (s *RouterManagementService) ListHosts(routerID int) ([]core.HotspotHost, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.ListHotspotHosts()
}

func (s *RouterManagementService) MakeHostBinding(routerID int, id, bindingType string) error {
	id = strings.TrimSpace(id)
	bindingType = strings.TrimSpace(bindingType)
	if id == "" || !validBindingType(bindingType) {
		return core.ErrInvalidInput
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.MakeHotspotHostBinding(id, bindingType)
}

func (s *RouterManagementService) ListIPBindings(routerID int) ([]core.IPBinding, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.ListIPBindings()
}

func validateIPBinding(binding *core.IPBinding, requireID bool) error {
	binding.ID = strings.TrimSpace(binding.ID)
	binding.MACAddress = strings.TrimSpace(binding.MACAddress)
	binding.Address = strings.TrimSpace(binding.Address)
	binding.ToAddress = strings.TrimSpace(binding.ToAddress)
	binding.Type = strings.TrimSpace(binding.Type)
	if requireID && binding.ID == "" {
		return core.ErrInvalidInput
	}
	if binding.MACAddress == "" && binding.Address == "" {
		return core.ErrInvalidInput
	}
	if binding.MACAddress != "" {
		if _, err := net.ParseMAC(binding.MACAddress); err != nil {
			return core.ErrInvalidInput
		}
	}
	if binding.Address != "" && !isIPv4(binding.Address) {
		return core.ErrInvalidInput
	}
	if binding.ToAddress != "" && !isIPv4(binding.ToAddress) {
		return core.ErrInvalidInput
	}
	if !validBindingType(binding.Type) {
		return core.ErrInvalidInput
	}
	return nil
}

func validBindingType(value string) bool {
	return value == "regular" || value == "bypassed" || value == "blocked"
}

func (s *RouterManagementService) AddIPBinding(routerID int, binding core.IPBinding) error {
	if err := validateIPBinding(&binding, false); err != nil {
		return err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.AddIPBinding(binding)
}

func (s *RouterManagementService) SetIPBinding(routerID int, binding core.IPBinding) error {
	if err := validateIPBinding(&binding, true); err != nil {
		return err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.SetIPBinding(binding)
}

func (s *RouterManagementService) RemoveIPBinding(routerID int, id string) error {
	return s.withID(routerID, id, funcIDBinding)
}

func (s *RouterManagementService) ListCookies(routerID int) ([]core.HotspotCookie, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.ListHotspotCookies()
}

func (s *RouterManagementService) RemoveCookie(routerID int, id string) error {
	return s.withID(routerID, id, funcIDCookie)
}

func normalizeLogLimit(limit int) (int, error) {
	if limit == 0 {
		return 100, nil
	}
	if limit < 1 || limit > 200 {
		return 0, core.ErrInvalidInput
	}
	return limit, nil
}

func (s *RouterManagementService) ListSystemLogs(routerID int, topic, search string, limit int) ([]core.SystemLog, error) {
	limit, err := normalizeLogLimit(limit)
	if err != nil {
		return nil, err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.ListSystemLogs(strings.TrimSpace(topic), strings.TrimSpace(search), limit)
}

func (s *RouterManagementService) ListHotspotServers(routerID int) ([]core.HotspotServer, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.ListHotspotServers()
}

func validateHotspotServer(server *core.HotspotServer, requireID bool) error {
	server.ID, server.Name = strings.TrimSpace(server.ID), strings.TrimSpace(server.Name)
	server.Interface, server.Profile = strings.TrimSpace(server.Interface), strings.TrimSpace(server.Profile)
	if (requireID && server.ID == "") || server.Name == "" || server.Interface == "" || server.Profile == "" {
		return core.ErrInvalidInput
	}
	return nil
}

func (s *RouterManagementService) AddHotspotServer(routerID int, server core.HotspotServer) error {
	if err := validateHotspotServer(&server, false); err != nil {
		return err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.AddHotspotServer(server)
}

func (s *RouterManagementService) SetHotspotServer(routerID int, server core.HotspotServer) error {
	if err := validateHotspotServer(&server, true); err != nil {
		return err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.SetHotspotServer(server)
}

func (s *RouterManagementService) RemoveHotspotServer(routerID int, id string) error {
	return s.withID(routerID, id, funcIDServer)
}

func (s *RouterManagementService) ListHotspotServerProfiles(routerID int) ([]core.HotspotServerProfile, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.ListHotspotServerProfiles()
}

var loginByValues = map[string]bool{
	"cookie": true, "http-chap": true, "http-pap": true, "https": true,
	"mac": true, "mac-cookie": true, "trial": true,
}

func validateHotspotServerProfile(profile *core.HotspotServerProfile, requireID bool) error {
	profile.ID, profile.Name = strings.TrimSpace(profile.ID), strings.TrimSpace(profile.Name)
	profile.HotspotAddress, profile.LoginBy = strings.TrimSpace(profile.HotspotAddress), strings.TrimSpace(profile.LoginBy)
	if (requireID && profile.ID == "") || profile.Name == "" {
		return core.ErrInvalidInput
	}
	if profile.HotspotAddress != "" && !isIPv4(profile.HotspotAddress) {
		return core.ErrInvalidInput
	}
	if profile.LoginBy != "" {
		hasHTTP, hasCookie := false, false
		for _, value := range strings.Split(profile.LoginBy, ",") {
			value = strings.TrimSpace(value)
			if !loginByValues[value] {
				return core.ErrInvalidInput
			}
			hasHTTP = hasHTTP || value == "http-chap" || value == "http-pap" || value == "https"
			hasCookie = hasCookie || value == "cookie"
		}
		if hasCookie && !hasHTTP {
			return core.ErrInvalidInput
		}
	}
	return nil
}

func isIPv4(value string) bool {
	ip := net.ParseIP(value)
	return ip != nil && ip.To4() != nil
}

func (s *RouterManagementService) AddHotspotServerProfile(routerID int, profile core.HotspotServerProfile) error {
	if err := validateHotspotServerProfile(&profile, false); err != nil {
		return err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.AddHotspotServerProfile(profile)
}

func (s *RouterManagementService) SetHotspotServerProfile(routerID int, profile core.HotspotServerProfile) error {
	if err := validateHotspotServerProfile(&profile, true); err != nil {
		return err
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.SetHotspotServerProfile(profile)
}

func (s *RouterManagementService) RemoveHotspotServerProfile(routerID int, id string) error {
	return s.withID(routerID, id, funcIDProfile)
}

func (s *RouterManagementService) ListIPPools(routerID int) ([]string, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.GetIPPools()
}

func (s *RouterManagementService) ListInterfaces(routerID int) ([]core.RouterInterface, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	rows, err := cl.GetInterfaces()
	if err != nil {
		return nil, err
	}
	out := make([]core.RouterInterface, len(rows))
	for i, row := range rows {
		out[i] = core.RouterInterface{Name: row["name"], Type: row["type"], Running: row["running"] == "true"}
	}
	return out, nil
}

const (
	funcIDBinding = iota
	funcIDCookie
	funcIDServer
	funcIDProfile
)

func (s *RouterManagementService) withID(routerID int, id string, operation int) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return core.ErrInvalidInput
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	switch operation {
	case funcIDBinding:
		return cl.RemoveIPBinding(id)
	case funcIDCookie:
		return cl.RemoveHotspotCookie(id)
	case funcIDServer:
		return cl.RemoveHotspotServer(id)
	default:
		return cl.RemoveHotspotServerProfile(id)
	}
}
