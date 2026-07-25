// Package api - high-level hotspot operations on top of the Client.
package routeros

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// HotspotUser represents a MikroTik hotspot user entry.
type HotspotUser struct {
	ID              string
	Name            string
	Password        string
	Profile         string
	Server          string
	Comment         string
	LimitUptime     string
	LimitBytesTotal string
	Uptime          string
	BytesIn         string
	BytesOut        string
	Disabled        bool
	MacAddress      string
}

// HotspotUserProfile represents a user profile.
type HotspotUserProfile struct {
	ID           string
	Name         string
	RateLimit    string
	SharedUsers  string
	OnLogin      string
	AddressPool  string
	ParentQueue  string
	Validity     string
	ExpiredMode  string
	LockMac      bool
	Price        string
	SellingPrice string
	GracePeriod  string
	HasMonitor   bool // true if a MikVoc monitor scheduler exists for this profile
	IsDefault    bool // true if this is the built-in Mikrotik default profile
}

// HotspotActive represents an active hotspot session.
type HotspotActive struct {
	ID       string
	User     string
	Server   string
	IP       string
	MacAddr  string
	Uptime   string
	IdleTime string
	BytesIn  string
	BytesOut string
	Comment  string
}

// SystemResource holds basic router resource info.
type SystemResource struct {
	Version     string
	BoardName   string
	Uptime      string
	CPULoad     string
	FreeMemory  string
	TotalMemory string
}

// --- Users ---

// CountUsers returns the total number of hotspot users efficiently.
func (c *Client) CountUsers() int {
	rows, err := c.Run("/ip/hotspot/user/print", map[string]string{"count-only": ""})
	if err == nil && len(rows) > 0 {
		if ret, ok := rows[0]["ret"]; ok {
			var count int
			fmt.Sscanf(ret, "%d", &count)
			return count
		}
	}
	return 0
}

// CountActiveUsers returns the total number of active hotspot users efficiently.
func (c *Client) CountActiveUsers() int {
	rows, err := c.Run("/ip/hotspot/active/print", map[string]string{"count-only": ""})
	if err == nil && len(rows) > 0 {
		if ret, ok := rows[0]["ret"]; ok {
			var count int
			fmt.Sscanf(ret, "%d", &count)
			return count
		}
	}
	return 0
}

const hotspotUserProplist = ".id,name,password,profile,server,comment,limit-uptime,limit-bytes-total,uptime,bytes-in,bytes-out,disabled,mac-address"

func hotspotUserFromRow(r map[string]string) HotspotUser {
	return HotspotUser{
		ID:              r[".id"],
		Name:            r["name"],
		Password:        r["password"],
		Profile:         r["profile"],
		Server:          r["server"],
		Comment:         r["comment"],
		LimitUptime:     r["limit-uptime"],
		LimitBytesTotal: r["limit-bytes-total"],
		Uptime:          r["uptime"],
		BytesIn:         r["bytes-in"],
		BytesOut:        r["bytes-out"],
		Disabled:        r["disabled"] == "true",
		MacAddress:      r["mac-address"],
	}
}

// GetUsers fetches all hotspot users, optionally filtered by profile.
func (c *Client) GetUsers(profile string) ([]HotspotUser, error) {
	params := map[string]string{
		".proplist": hotspotUserProplist,
	}
	if profile != "" && profile != "all" {
		params["?profile"] = profile
	}
	rows, err := c.Run("/ip/hotspot/user/print", params)
	if err != nil {
		return nil, err
	}
	users := make([]HotspotUser, 0, len(rows))
	for _, r := range rows {
		users = append(users, hotspotUserFromRow(r))
	}
	return users, nil
}

// GetUserByName fetches a single hotspot user by exact name.
func (c *Client) GetUserByName(name string) (HotspotUser, error) {
	if strings.TrimSpace(name) == "" {
		return HotspotUser{}, fmt.Errorf("name required")
	}
	rows, err := c.Run("/ip/hotspot/user/print", map[string]string{
		"?name":     name,
		".proplist": hotspotUserProplist,
	})
	if err != nil {
		return HotspotUser{}, err
	}
	if len(rows) == 0 {
		return HotspotUser{}, fmt.Errorf("not found")
	}
	return hotspotUserFromRow(rows[0]), nil
}

// AddUser creates a new hotspot user.
func (c *Client) AddUser(u HotspotUser) error {
	params := map[string]string{
		"name":     u.Name,
		"password": u.Password,
		"profile":  u.Profile,
	}
	if u.Server != "" {
		params["server"] = u.Server
	}
	if u.LimitUptime != "" {
		params["limit-uptime"] = u.LimitUptime
	}
	if u.LimitBytesTotal != "" {
		params["limit-bytes-total"] = u.LimitBytesTotal
	}
	if u.Comment != "" {
		params["comment"] = u.Comment
	}
	if u.MacAddress != "" {
		params["mac-address"] = u.MacAddress
	}
	_, err := c.Run("/ip/hotspot/user/add", params)
	return err
}

// RemoveUser deletes a hotspot user by ID.
func (c *Client) RemoveUser(id string) error {
	_, err := c.Run("/ip/hotspot/user/remove", map[string]string{"=.id": id})
	return err
}

// RemoveExpiredUsers removes users marked as expired by RouterOS/Mikhmon conventions.
func (c *Client) RemoveExpiredUsers(rosVersion string) (int, error) {
	users, err := c.GetUsers("")
	if err != nil {
		return 0, err
	}
	now := time.Now()
	removed := 0
	for _, u := range users {
		if !HotspotUserExpiredAt(u, now) {
			continue
		}
		if e := c.RemoveUser(u.ID); e == nil {
			removed++
		}
	}
	return removed, nil
}

// SetUserDisabled enables or disables a hotspot user.
func (c *Client) SetUserDisabled(id string, disabled bool) error {
	val := "false"
	if disabled {
		val = "true"
	}
	_, err := c.Run("/ip/hotspot/user/set", map[string]string{
		"=.id":      id,
		"=disabled": val,
	})
	return err
}

// UpdateUser updates editable fields of a hotspot user by ID.
// Empty fields are skipped (not cleared on the router).
func (c *Client) UpdateUser(id string, u HotspotUser) error {
	params := map[string]string{"=.id": id}
	if u.Password != "" {
		params["=password"] = u.Password
	}
	if u.Profile != "" {
		params["=profile"] = u.Profile
	}
	if u.Server != "" {
		params["=server"] = u.Server
	}
	if u.Comment != "" {
		params["=comment"] = u.Comment
	}
	if u.LimitUptime != "" {
		params["=limit-uptime"] = u.LimitUptime
	}
	if u.LimitBytesTotal != "" {
		params["=limit-bytes-total"] = u.LimitBytesTotal
	}
	if u.MacAddress != "" {
		params["=mac-address"] = u.MacAddress
	}
	_, err := c.Run("/ip/hotspot/user/set", params)
	return err
}

// ResetUser resets a user's byte/time counters.
func (c *Client) ResetUser(id string) error {
	_, err := c.Run("/ip/hotspot/user/reset-counters", map[string]string{"=.id": id})
	return err
}

// --- Profiles ---

// GetProfiles fetches all hotspot user profiles, including monitor status.
func (c *Client) GetProfiles() ([]HotspotUserProfile, error) {
	rows, err := c.Run("/ip/hotspot/user/profile/print")
	if err != nil {
		return nil, err
	}

	// Fetch existing MikVoc monitor schedulers
	monitorSet := map[string]bool{}
	schedulers, _ := c.Run("/system/scheduler/print")
	for _, s := range schedulers {
		name := s["name"]
		comment := s["comment"]
		if strings.HasPrefix(name, "mikvoc-monitor-") || strings.HasPrefix(comment, "MikVoc Monitor:") {
			// Extract profile name from comment
			profName := strings.TrimPrefix(comment, "MikVoc Monitor: ")
			if profName != "" {
				monitorSet[profName] = true
			}
			// Also try from scheduler name
			schedProfName := strings.ReplaceAll(strings.TrimPrefix(name, "mikvoc-monitor-"), "_", " ")
			if schedProfName != "" {
				monitorSet[schedProfName] = true
			}
		}
	}

	profiles := make([]HotspotUserProfile, 0, len(rows))
	for _, r := range rows {
		name := r["name"]
		p := HotspotUserProfile{
			ID:          r[".id"],
			Name:        name,
			RateLimit:   r["rate-limit"],
			SharedUsers: r["shared-users"],
			OnLogin:     r["on-login"],
			AddressPool: r["address-pool"],
			ParentQueue: r["parent-queue"],
			HasMonitor:  monitorSet[name],
			IsDefault:   name == "default" && r["rate-limit"] == "" && r["on-login"] == "",
		}
		applyProfileConfigFromOnLogin(&p)
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// CreateProfile adds a new hotspot profile, with extended attributes stored in on-login.
func (c *Client) CreateProfile(name, sharedUsers, rateLimit, addressPool, parentQueue, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod string) error {
	params := map[string]string{
		"name":               name,
		"status-autorefresh": "1m",
	}
	if sharedUsers != "" {
		params["shared-users"] = sharedUsers
	}
	if rateLimit != "" {
		params["rate-limit"] = rateLimit
	}
	if addressPool != "" && addressPool != "none" {
		params["address-pool"] = addressPool
	}
	if parentQueue != "" && parentQueue != "none" {
		params["parent-queue"] = parentQueue
	}

	params["on-login"] = buildProfileOnLoginScript(name, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod)

	_, err := c.Run("/ip/hotspot/user/profile/add", params)
	return err
}

// RemoveProfile deletes a user profile by ID.
func (c *Client) RemoveProfile(id string) error {
	_, err := c.Run("/ip/hotspot/user/profile/remove", map[string]string{"=.id": id})
	return err
}

// UpdateProfile updates an existing hotspot user profile by ID.
func (c *Client) UpdateProfile(id, name, sharedUsers, rateLimit, addressPool, parentQueue, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod string) error {
	params := map[string]string{
		"=.id":               id,
		"=status-autorefresh": "1m",
	}
	if sharedUsers != "" {
		params["=shared-users"] = sharedUsers
	}
	if rateLimit != "" {
		params["=rate-limit"] = rateLimit
	} else {
		params["=rate-limit"] = ""
	}
	if addressPool != "" && addressPool != "none" {
		params["=address-pool"] = addressPool
	} else {
		params["=address-pool"] = "none"
	}
	if parentQueue != "" && parentQueue != "none" {
		params["=parent-queue"] = parentQueue
	} else {
		params["=parent-queue"] = "none"
	}

	params["=on-login"] = buildProfileOnLoginScript(name, expiredMode, validity, lockMac, price, sellingPrice, gracePeriod)

	_, err := c.Run("/ip/hotspot/user/profile/set", params)
	return err
}

func (c *Client) GetIPPools() ([]string, error) {
	rows, err := c.Run("/ip/pool/print", map[string]string{".proplist": "name"})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if n := strings.TrimSpace(r["name"]); n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

func (c *Client) GetSimpleQueues() ([]string, error) {
	rows, err := c.Run("/queue/simple/print", map[string]string{".proplist": "name,dynamic"})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r["dynamic"] == "true" {
			continue
		}
		if n := strings.TrimSpace(r["name"]); n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// --- Active Sessions ---

// GetActiveUsers fetches all currently active hotspot sessions.
func (c *Client) GetActiveUsers(server string) ([]HotspotActive, error) {
	params := map[string]string{}
	if server != "" && server != "all" {
		params["?server"] = server
	}
	rows, err := c.Run("/ip/hotspot/active/print", params)
	if err != nil {
		return nil, err
	}
	actives := make([]HotspotActive, 0, len(rows))
	for _, r := range rows {
		actives = append(actives, HotspotActive{
			ID:       r[".id"],
			User:     r["user"],
			Server:   r["server"],
			IP:       r["address"],
			MacAddr:  r["mac-address"],
			Uptime:   r["uptime"],
			IdleTime: r["idle-time"],
			BytesIn:  r["bytes-in"],
			BytesOut: r["bytes-out"],
		})
	}
	return actives, nil
}

// KickActiveUser removes an active session by ID.
func (c *Client) KickActiveUser(id string) error {
	_, err := c.Run("/ip/hotspot/active/remove", map[string]string{"=.id": id})
	return err
}

// --- Hotspot Servers ---

// GetServers fetches hotspot server list.
func (c *Client) GetServers() ([]map[string]string, error) {
	return c.Run("/ip/hotspot/print")
}

// --- System ---

// GetSystemResource fetches router resource info.
func (c *Client) GetSystemResource() (*SystemResource, error) {
	rows, err := c.Run("/system/resource/print")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data from /system/resource/print")
	}
	r := rows[0]
	return &SystemResource{
		Version:     r["version"],
		BoardName:   r["board-name"],
		Uptime:      r["uptime"],
		CPULoad:     r["cpu-load"],
		FreeMemory:  r["free-memory"],
		TotalMemory: r["total-memory"],
	}, nil
}

// GetSchedulers fetches all system schedulers.
func (c *Client) GetSchedulers() ([]map[string]string, error) {
	return c.Run("/system/scheduler/print")
}

// --- Voucher Generation ---

// GenerateOptions holds options for bulk voucher generation.
type GenerateOptions struct {
	Qty            int
	Profile        string
	Server         string
	Mode           string // "up" = user+pass separate, "vc" = user=pass
	Prefix         string
	Length         int
	CharMode       string // "lower", "upper", "upplow", "mix", "num"
	TimeLimitStr   string
	DataLimitBytes int64
	Comment        string
}

// charsets for voucher generation (filtered: no 0, 1, O, l, q, Q)
const (
	charsLower = "abcdefghijkmnoprstuvwxyz23456789"
	charsUpper = "ABCDEFGHIJKMNOPRSTUVWXYZ23456789"
	charsMixed = "abcdefghijkmnoprstuvwxyzABCDEFGHIJKMNOPRSTUVWXYZ23456789"
	charsNum   = "23456789"
)

func randStr(n int, charset string) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// BatchComment builds a Mikhmon-style batch comment for a generate run:
// vc-PROFILE-MM.DD.YY[-label] or up-PROFILE-MM.DD.YY[-label].
func BatchComment(opts GenerateOptions) string {
	modePrefix := "vc"
	if opts.Mode == "up" {
		modePrefix = "up"
	}
	comment := fmt.Sprintf("%s-%s-%s", modePrefix, opts.Profile, time.Now().Format("01.02.06"))
	if opts.Comment != "" {
		comment += "-" + opts.Comment
	}
	return comment
}

// GenerateUsers generates voucher users in bulk on the router.
// Returns generated (name, password) pairs and the batch comment applied to all users.
func (c *Client) GenerateUsers(opts GenerateOptions) ([][2]string, string, error) {
	charset := charsLower
	switch opts.CharMode {
	case "upper":
		charset = charsUpper
	case "upplow", "mix":
		charset = charsMixed
	case "num":
		charset = charsNum
	}

	comment := BatchComment(opts)

	var generated [][2]string
	seen := map[string]bool{}

	for len(generated) < opts.Qty {
		var username, password string
		candidate := opts.Prefix + randStr(opts.Length, charset)

		if seen[candidate] {
			continue
		}
		seen[candidate] = true

		if opts.Mode == "vc" {
			username = candidate
			password = candidate
		} else {
			username = candidate
			password = randStr(opts.Length, charsNum)
		}

		user := HotspotUser{
			Name:     username,
			Password: password,
			Profile:  opts.Profile,
			Server:   opts.Server,
			Comment:  comment,
		}
		if opts.TimeLimitStr != "" {
			user.LimitUptime = opts.TimeLimitStr
		}
		if opts.DataLimitBytes > 0 {
			user.LimitBytesTotal = fmt.Sprintf("%d", opts.DataLimitBytes)
		}

		if err := c.AddUser(user); err != nil {
			if strings.Contains(err.Error(), "already have") || strings.Contains(err.Error(), "same name") {
				seen[username] = true
				continue
			}
			return generated, comment, err
		}
		generated = append(generated, [2]string{username, password})
	}
	return generated, comment, nil
}
