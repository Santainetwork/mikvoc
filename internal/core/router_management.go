package core

type HotspotHost struct {
	ID         string
	MACAddress string
	Address    string
	ToAddress  string
	Server     string
	Uptime     string
	IdleTime   string
	Authorized bool
	Bypassed   bool
}

type IPBinding struct {
	ID         string
	MACAddress string
	Address    string
	ToAddress  string
	Server     string
	Type       string
	Comment    string
	Disabled   bool
}

type HotspotCookie struct {
	ID         string
	User       string
	MACAddress string
	ExpiresIn  string
}

type SystemLog struct {
	ID      string
	Time    string
	Topics  string
	Message string
}

type HotspotServer struct {
	ID               string
	Name             string
	Interface        string
	AddressPool      string
	Profile          string
	IdleTimeout      string
	KeepaliveTimeout string
	Disabled         bool
}

type HotspotServerProfile struct {
	ID             string
	Name           string
	HotspotAddress string
	DNSName        string
	HTMLDirectory  string
	LoginBy        string
	CookieLifetime string
	RateLimit      string
}

type RouterInterface struct {
	Name    string
	Type    string
	Running bool
}
