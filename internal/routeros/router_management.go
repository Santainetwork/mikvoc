package routeros

import (
	"fmt"
	"strings"

	"mikvoc/internal/core"
)

const (
	hotspotHostProplist          = ".id,mac-address,address,to-address,server,uptime,idle-time,authorized,bypassed"
	ipBindingProplist            = ".id,mac-address,address,to-address,server,type,comment,disabled"
	hotspotCookieProplist        = ".id,user,mac-address,expires-in"
	systemLogProplist            = ".id,time,topics,message"
	hotspotServerProplist        = ".id,name,interface,address-pool,profile,idle-timeout,keepalive-timeout,disabled"
	hotspotServerProfileProplist = ".id,name,hotspot-address,dns-name,html-directory,login-by,http-cookie-lifetime,rate-limit"
)

func hotspotHostFromRow(r map[string]string) core.HotspotHost {
	return core.HotspotHost{
		ID: r[".id"], MACAddress: r["mac-address"], Address: r["address"], ToAddress: r["to-address"],
		Server: r["server"], Uptime: r["uptime"], IdleTime: r["idle-time"],
		Authorized: r["authorized"] == "true", Bypassed: r["bypassed"] == "true",
	}
}

func ipBindingFromRow(r map[string]string) core.IPBinding {
	return core.IPBinding{
		ID: r[".id"], MACAddress: r["mac-address"], Address: r["address"], ToAddress: r["to-address"],
		Server: r["server"], Type: r["type"], Comment: r["comment"], Disabled: r["disabled"] == "true",
	}
}

func hotspotCookieFromRow(r map[string]string) core.HotspotCookie {
	return core.HotspotCookie{ID: r[".id"], User: r["user"], MACAddress: r["mac-address"], ExpiresIn: r["expires-in"]}
}

func hotspotServerFromRow(r map[string]string) core.HotspotServer {
	return core.HotspotServer{
		ID: r[".id"], Name: r["name"], Interface: r["interface"], AddressPool: r["address-pool"],
		Profile: r["profile"], IdleTimeout: r["idle-timeout"], KeepaliveTimeout: r["keepalive-timeout"],
		Disabled: r["disabled"] == "true",
	}
}

func hotspotServerProfileFromRow(r map[string]string) core.HotspotServerProfile {
	return core.HotspotServerProfile{
		ID: r[".id"], Name: r["name"], HotspotAddress: r["hotspot-address"], DNSName: r["dns-name"],
		HTMLDirectory: r["html-directory"], LoginBy: r["login-by"], CookieLifetime: r["http-cookie-lifetime"],
		RateLimit: r["rate-limit"],
	}
}

func (c *Client) ListHotspotHosts() ([]core.HotspotHost, error) {
	rows, err := c.Run("/ip/hotspot/host/print", map[string]string{".proplist": hotspotHostProplist})
	if err != nil {
		return nil, err
	}
	out := make([]core.HotspotHost, len(rows))
	for i, row := range rows {
		out[i] = hotspotHostFromRow(row)
	}
	return out, nil
}

func (c *Client) MakeHotspotHostBinding(id, bindingType string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing id")
	}
	_, err := c.Run("/ip/hotspot/host/make-binding", map[string]string{".id": id, "type": bindingType})
	return err
}

func (c *Client) ListIPBindings() ([]core.IPBinding, error) {
	rows, err := c.Run("/ip/hotspot/ip-binding/print", map[string]string{".proplist": ipBindingProplist})
	if err != nil {
		return nil, err
	}
	out := make([]core.IPBinding, len(rows))
	for i, row := range rows {
		out[i] = ipBindingFromRow(row)
	}
	return out, nil
}

func ipBindingParams(b core.IPBinding) map[string]string {
	if b.Server == "" {
		b.Server = "all"
	}
	return map[string]string{
		"mac-address": b.MACAddress, "address": b.Address, "to-address": b.ToAddress,
		"server": b.Server, "type": b.Type, "comment": b.Comment, "disabled": boolWord(b.Disabled),
	}
}

func (c *Client) AddIPBinding(b core.IPBinding) error {
	_, err := c.Run("/ip/hotspot/ip-binding/add", ipBindingParams(b))
	return err
}

func (c *Client) SetIPBinding(b core.IPBinding) error {
	b.ID = strings.TrimSpace(b.ID)
	if b.ID == "" {
		return fmt.Errorf("missing id")
	}
	params := ipBindingParams(b)
	params[".id"] = b.ID
	_, err := c.Run("/ip/hotspot/ip-binding/set", params)
	return err
}

func (c *Client) RemoveIPBinding(id string) error {
	return c.removeRouterManagementItem("/ip/hotspot/ip-binding/remove", id)
}

func (c *Client) ListHotspotCookies() ([]core.HotspotCookie, error) {
	rows, err := c.Run("/ip/hotspot/cookie/print", map[string]string{".proplist": hotspotCookieProplist})
	if err != nil {
		return nil, err
	}
	out := make([]core.HotspotCookie, len(rows))
	for i, row := range rows {
		out[i] = hotspotCookieFromRow(row)
	}
	return out, nil
}

func (c *Client) RemoveHotspotCookie(id string) error {
	return c.removeRouterManagementItem("/ip/hotspot/cookie/remove", id)
}

func filterSystemLogs(rows []map[string]string, topic, search string, limit int) []core.SystemLog {
	topic, search = strings.ToLower(strings.TrimSpace(topic)), strings.ToLower(strings.TrimSpace(search))
	out := make([]core.SystemLog, 0, min(len(rows), limit))
	for i := len(rows) - 1; i >= 0 && len(out) < limit; i-- {
		row := rows[i]
		if topic != "" && !strings.Contains(strings.ToLower(row["topics"]), topic) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(row["message"]), search) {
			continue
		}
		out = append(out, core.SystemLog{ID: row[".id"], Time: row["time"], Topics: row["topics"], Message: row["message"]})
	}
	return out
}

func (c *Client) ListSystemLogs(topic, search string, limit int) ([]core.SystemLog, error) {
	rows, err := c.Run("/log/print", map[string]string{".proplist": systemLogProplist})
	if err != nil {
		return nil, err
	}
	return filterSystemLogs(rows, topic, search, limit), nil
}

func (c *Client) ListHotspotServers() ([]core.HotspotServer, error) {
	rows, err := c.Run("/ip/hotspot/print", map[string]string{".proplist": hotspotServerProplist})
	if err != nil {
		return nil, err
	}
	out := make([]core.HotspotServer, len(rows))
	for i, row := range rows {
		out[i] = hotspotServerFromRow(row)
	}
	return out, nil
}

func hotspotServerParams(server core.HotspotServer) map[string]string {
	if server.AddressPool == "" {
		server.AddressPool = "none"
	}
	if server.IdleTimeout == "" {
		server.IdleTimeout = "5m"
	}
	if server.KeepaliveTimeout == "" {
		server.KeepaliveTimeout = "none"
	}
	return map[string]string{
		"name": server.Name, "interface": server.Interface, "address-pool": server.AddressPool,
		"profile": server.Profile, "idle-timeout": server.IdleTimeout,
		"keepalive-timeout": server.KeepaliveTimeout, "disabled": boolWord(server.Disabled),
	}
}

func (c *Client) AddHotspotServer(server core.HotspotServer) error {
	_, err := c.Run("/ip/hotspot/add", hotspotServerParams(server))
	return err
}

func (c *Client) SetHotspotServer(server core.HotspotServer) error {
	server.ID = strings.TrimSpace(server.ID)
	if server.ID == "" {
		return fmt.Errorf("missing id")
	}
	params := hotspotServerParams(server)
	params[".id"] = server.ID
	_, err := c.Run("/ip/hotspot/set", params)
	return err
}

func (c *Client) RemoveHotspotServer(id string) error {
	return c.removeRouterManagementItem("/ip/hotspot/remove", id)
}

func (c *Client) ListHotspotServerProfiles() ([]core.HotspotServerProfile, error) {
	rows, err := c.Run("/ip/hotspot/profile/print", map[string]string{".proplist": hotspotServerProfileProplist})
	if err != nil {
		return nil, err
	}
	out := make([]core.HotspotServerProfile, len(rows))
	for i, row := range rows {
		out[i] = hotspotServerProfileFromRow(row)
	}
	return out, nil
}

func hotspotServerProfileParams(profile core.HotspotServerProfile) map[string]string {
	if profile.HotspotAddress == "" {
		profile.HotspotAddress = "0.0.0.0"
	}
	if profile.HTMLDirectory == "" {
		profile.HTMLDirectory = "hotspot"
	}
	if profile.LoginBy == "" {
		profile.LoginBy = "http-chap,cookie"
	}
	if profile.CookieLifetime == "" {
		profile.CookieLifetime = "3d"
	}
	return map[string]string{
		"name": profile.Name, "hotspot-address": profile.HotspotAddress, "dns-name": profile.DNSName,
		"html-directory": profile.HTMLDirectory, "login-by": profile.LoginBy,
		"http-cookie-lifetime": profile.CookieLifetime, "rate-limit": profile.RateLimit,
	}
}

func (c *Client) AddHotspotServerProfile(profile core.HotspotServerProfile) error {
	_, err := c.Run("/ip/hotspot/profile/add", hotspotServerProfileParams(profile))
	return err
}

func (c *Client) SetHotspotServerProfile(profile core.HotspotServerProfile) error {
	profile.ID = strings.TrimSpace(profile.ID)
	if profile.ID == "" {
		return fmt.Errorf("missing id")
	}
	params := hotspotServerProfileParams(profile)
	params[".id"] = profile.ID
	_, err := c.Run("/ip/hotspot/profile/set", params)
	return err
}

func (c *Client) RemoveHotspotServerProfile(id string) error {
	return c.removeRouterManagementItem("/ip/hotspot/profile/remove", id)
}

func (c *Client) removeRouterManagementItem(command, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing id")
	}
	_, err := c.Run(command, map[string]string{".id": id})
	return err
}

func boolWord(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
