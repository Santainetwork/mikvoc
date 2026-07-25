package core

type User struct {
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

type UserProfile struct {
	ID           string
	Name         string
	RateLimit    string
	SharedUsers  string
	OnLogin      string
	AddressPool  string
	ParentQueue  string
	Validity     string
	ExpiredMode  string
	Price        string
	SellingPrice string
	GracePeriod  string
	LockMac      bool
	HasMonitor   bool
	IsDefault    bool
}

type ActiveSession struct {
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

type UserFilters struct {
	Profile string
	Comment string
	Search  string
	Expired bool
	IDs     []string
}

type UserListOptions struct {
	Profile string
	Limit   int
	Cursor  string
}
