package routeros

import (
	"fmt"
	"strings"
	"time"
)

type PPPSecret struct {
	ID            string
	Name          string
	Password      string
	Service       string
	Profile       string
	LocalAddress  string
	RemoteAddress string
	Comment       string
	CallerID      string
	Routes        string
	LimitBytesIn  string
	LimitBytesOut string
	Disabled      bool
	LastLoggedOut string
}

type PPPProfile struct {
	ID             string
	Name           string
	LocalAddress   string
	RemoteAddress  string
	Bridge         string
	IncomingFilter string
	OutgoingFilter string
	AddressList    string
	DNSServer      string
	WINSServer     string
	ChangeTCPMSS   string
	UseUPnP        string
	RateLimit      string
	OnlyOne        string
	IsDefault      bool
}

type PPPActive struct {
	ID        string
	Name      string
	Service   string
	CallerID  string
	Address   string
	Uptime    string
	Encoding  string
	SessionID string
}

const pppSecretProplist = ".id,name,password,service,profile,local-address,remote-address,comment,caller-id,routes,limit-bytes-in,limit-bytes-out,disabled,last-logged-out"

func pppSecretFromRow(r map[string]string) PPPSecret {
	return PPPSecret{
		ID:            r[".id"],
		Name:          r["name"],
		Password:      r["password"],
		Service:       r["service"],
		Profile:       r["profile"],
		LocalAddress:  r["local-address"],
		RemoteAddress: r["remote-address"],
		Comment:       r["comment"],
		CallerID:      r["caller-id"],
		Routes:        r["routes"],
		LimitBytesIn:  r["limit-bytes-in"],
		LimitBytesOut: r["limit-bytes-out"],
		Disabled:      r["disabled"] == "true",
		LastLoggedOut: r["last-logged-out"],
	}
}

func (c *Client) GetPPPSecrets(profile string) ([]PPPSecret, error) {
	params := map[string]string{".proplist": pppSecretProplist}
	if profile != "" && profile != "all" {
		params["?profile"] = profile
	}
	rows, err := c.Run("/ppp/secret/print", params)
	if err != nil {
		return nil, err
	}
	out := make([]PPPSecret, 0, len(rows))
	for _, r := range rows {
		out = append(out, pppSecretFromRow(r))
	}
	return out, nil
}

func (c *Client) GetPPPSecretByName(name string) (PPPSecret, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return PPPSecret{}, fmt.Errorf("empty name")
	}
	rows, err := c.Run("/ppp/secret/print", map[string]string{
		".proplist": pppSecretProplist,
		"?name":     name,
	})
	if err != nil {
		return PPPSecret{}, err
	}
	if len(rows) == 0 {
		return PPPSecret{}, fmt.Errorf("not found")
	}
	return pppSecretFromRow(rows[0]), nil
}

func (c *Client) AddPPPSecret(s PPPSecret) error {
	params := map[string]string{
		"name":     s.Name,
		"password": s.Password,
	}
	if s.Service != "" {
		params["service"] = s.Service
	}
	if s.Profile != "" {
		params["profile"] = s.Profile
	}
	if s.LocalAddress != "" {
		params["local-address"] = s.LocalAddress
	}
	if s.RemoteAddress != "" {
		params["remote-address"] = s.RemoteAddress
	}
	if s.Comment != "" {
		params["comment"] = s.Comment
	}
	if s.CallerID != "" {
		params["caller-id"] = s.CallerID
	}
	if s.Routes != "" {
		params["routes"] = s.Routes
	}
	if s.LimitBytesIn != "" {
		params["limit-bytes-in"] = s.LimitBytesIn
	}
	if s.LimitBytesOut != "" {
		params["limit-bytes-out"] = s.LimitBytesOut
	}
	if s.Disabled {
		params["disabled"] = "yes"
	}
	_, err := c.Run("/ppp/secret/add", params)
	return err
}

func (c *Client) UpdatePPPSecret(s PPPSecret) error {
	if s.ID == "" {
		return fmt.Errorf("missing id")
	}
	params := map[string]string{".id": s.ID}
	if s.Name != "" {
		params["name"] = s.Name
	}
	if s.Password != "" {
		params["password"] = s.Password
	}
	if s.Service != "" {
		params["service"] = s.Service
	}
	if s.Profile != "" {
		params["profile"] = s.Profile
	}
	params["local-address"] = s.LocalAddress
	params["remote-address"] = s.RemoteAddress
	params["comment"] = s.Comment
	params["caller-id"] = s.CallerID
	params["routes"] = s.Routes
	if s.LimitBytesIn != "" {
		params["limit-bytes-in"] = s.LimitBytesIn
	}
	if s.LimitBytesOut != "" {
		params["limit-bytes-out"] = s.LimitBytesOut
	}
	if s.Disabled {
		params["disabled"] = "yes"
	} else {
		params["disabled"] = "no"
	}
	_, err := c.Run("/ppp/secret/set", params)
	return err
}

func (c *Client) RemovePPPSecret(id string) error {
	_, err := c.Run("/ppp/secret/remove", map[string]string{".id": id})
	return err
}

func (c *Client) SetPPPSecretDisabled(id string, disabled bool) error {
	val := "no"
	if disabled {
		val = "yes"
	}
	_, err := c.Run("/ppp/secret/set", map[string]string{".id": id, "disabled": val})
	return err
}

func (c *Client) GetPPPActive() ([]PPPActive, error) {
	rows, err := c.Run("/ppp/active/print", map[string]string{
		".proplist": ".id,name,service,caller-id,address,uptime,encoding,session-id",
	})
	if err != nil {
		return nil, err
	}
	out := make([]PPPActive, 0, len(rows))
	for _, r := range rows {
		out = append(out, PPPActive{
			ID:        r[".id"],
			Name:      r["name"],
			Service:   r["service"],
			CallerID:  r["caller-id"],
			Address:   r["address"],
			Uptime:    r["uptime"],
			Encoding:  r["encoding"],
			SessionID: r["session-id"],
		})
	}
	return out, nil
}

func (c *Client) KickPPPActive(id string) error {
	_, err := c.Run("/ppp/active/remove", map[string]string{".id": id})
	return err
}

func (c *Client) GetPPPProfiles() ([]PPPProfile, error) {
	rows, err := c.Run("/ppp/profile/print")
	if err != nil {
		return nil, err
	}
	out := make([]PPPProfile, 0, len(rows))
	for _, r := range rows {
		out = append(out, pppProfileFromRow(r))
	}
	return out, nil
}

func pppProfileFromRow(r map[string]string) PPPProfile {
	return PPPProfile{
		ID:             r[".id"],
		Name:           r["name"],
		LocalAddress:   r["local-address"],
		RemoteAddress:  r["remote-address"],
		Bridge:         r["bridge"],
		IncomingFilter: r["incoming-filter"],
		OutgoingFilter: r["outgoing-filter"],
		AddressList:    r["address-list"],
		DNSServer:      r["dns-server"],
		WINSServer:     r["wins-server"],
		ChangeTCPMSS:   r["change-tcp-mss"],
		UseUPnP:        r["use-upnp"],
		RateLimit:      r["rate-limit"],
		OnlyOne:        r["only-one"],
		IsDefault:      r["name"] == "default",
	}
}

func (c *Client) CreatePPPProfile(p PPPProfile) error {
	params := map[string]string{"name": p.Name}
	if p.LocalAddress != "" {
		params["local-address"] = p.LocalAddress
	}
	if p.RemoteAddress != "" {
		params["remote-address"] = p.RemoteAddress
	}
	if p.Bridge != "" {
		params["bridge"] = p.Bridge
	}
	if p.IncomingFilter != "" {
		params["incoming-filter"] = p.IncomingFilter
	}
	if p.OutgoingFilter != "" {
		params["outgoing-filter"] = p.OutgoingFilter
	}
	if p.RateLimit != "" {
		params["rate-limit"] = p.RateLimit
	}
	if p.OnlyOne != "" {
		params["only-one"] = p.OnlyOne
	}
	if p.DNSServer != "" {
		params["dns-server"] = p.DNSServer
	}
	if p.WINSServer != "" {
		params["wins-server"] = p.WINSServer
	}
	if p.AddressList != "" {
		params["address-list"] = p.AddressList
	}
	if p.ChangeTCPMSS != "" {
		params["change-tcp-mss"] = p.ChangeTCPMSS
	}
	if p.UseUPnP != "" {
		params["use-upnp"] = p.UseUPnP
	}
	_, err := c.Run("/ppp/profile/add", params)
	return err
}

func (c *Client) UpdatePPPProfile(p PPPProfile) error {
	if p.ID == "" {
		return fmt.Errorf("missing id")
	}
	params := map[string]string{".id": p.ID}
	if p.Name != "" {
		params["name"] = p.Name
	}
	params["local-address"] = p.LocalAddress
	params["remote-address"] = p.RemoteAddress
	params["bridge"] = p.Bridge
	params["rate-limit"] = p.RateLimit
	if p.OnlyOne != "" {
		params["only-one"] = p.OnlyOne
	}
	params["dns-server"] = p.DNSServer
	params["wins-server"] = p.WINSServer
	params["incoming-filter"] = p.IncomingFilter
	params["outgoing-filter"] = p.OutgoingFilter
	params["address-list"] = p.AddressList
	if p.ChangeTCPMSS != "" {
		params["change-tcp-mss"] = p.ChangeTCPMSS
	}
	if p.UseUPnP != "" {
		params["use-upnp"] = p.UseUPnP
	}
	_, err := c.Run("/ppp/profile/set", params)
	return err
}

func (c *Client) RemovePPPProfile(id string) error {
	_, err := c.Run("/ppp/profile/remove", map[string]string{".id": id})
	return err
}

func (c *Client) GetBridges() ([]string, error) {
	rows, err := c.Run("/interface/bridge/print", map[string]string{".proplist": "name"})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if name := strings.TrimSpace(row["name"]); name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

func PPPBatchComment(profile, label string) string {
	now := time.Now()
	base := fmt.Sprintf("ppp-%s-%02d.%02d.%02d", strings.ToUpper(strings.TrimSpace(profile)), now.Month(), now.Day(), now.Year()%100)
	label = strings.TrimSpace(label)
	if label == "" {
		return base
	}
	return base + "-" + label
}

func FormatPPPSecretName(prefix string, n, pad int) string {
	prefix = strings.TrimSpace(prefix)
	if pad < 1 {
		pad = 1
	}
	return fmt.Sprintf("%s%0*d", prefix, pad, n)
}

type PPPGenerateOptions struct {
	Qty     int
	Prefix  string
	Start   int
	Pad     int
	Profile string
	Service string
	Comment string
}

func (c *Client) GeneratePPPSecrets(opts PPPGenerateOptions) (created [][2]string, comment string, skipped int, err error) {
	if opts.Qty < 1 {
		opts.Qty = 1
	}
	if opts.Qty > 500 {
		opts.Qty = 500
	}
	if opts.Start < 0 {
		opts.Start = 0
	}
	if opts.Pad < 1 {
		opts.Pad = 1
	}
	if opts.Service == "" {
		opts.Service = "pppoe"
	}
	if opts.Profile == "" {
		opts.Profile = "default"
	}
	comment = strings.TrimSpace(opts.Comment)
	if comment == "" {
		comment = PPPBatchComment(opts.Profile, "")
	}

	existing, e := c.GetPPPSecrets("")
	if e != nil {
		return nil, "", 0, e
	}
	taken := map[string]bool{}
	for _, s := range existing {
		taken[s.Name] = true
	}

	created = make([][2]string, 0, opts.Qty)
	n := opts.Start
	for len(created) < opts.Qty {
		name := FormatPPPSecretName(opts.Prefix, n, opts.Pad)
		n++
		if taken[name] {
			skipped++
			continue
		}
		sec := PPPSecret{
			Name:     name,
			Password: name,
			Service:  opts.Service,
			Profile:  opts.Profile,
			Comment:  comment,
		}
		if e := c.AddPPPSecret(sec); e != nil {
			return created, comment, skipped, e
		}
		taken[name] = true
		created = append(created, [2]string{name, name})
	}
	return created, comment, skipped, nil
}
