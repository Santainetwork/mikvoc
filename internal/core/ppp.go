package core

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

type PPPFilters struct {
	Profile  string
	Search   string
	Comment  string
	Disabled *bool
	IDs      []string
}

type PPPGenerateSpec struct {
	Qty     int
	Prefix  string
	Start   int
	Pad     int
	Profile string
	Service string
	Comment string
}

type PPPGenerateResult struct {
	Username string
	Password string
}

type PPPGenerateBatch struct {
	Comment string
	Items   []PPPGenerateResult
	Skipped int
}
