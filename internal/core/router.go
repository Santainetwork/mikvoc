package core

type Router struct {
	ID              int
	Name            string
	IP              string
	Port            string
	Username        string
	Password        string
	SortOrder       int
	VoucherTemplate string
}

type SystemResource struct {
	Version     string
	BoardName   string
	Uptime      string
	CPULoad     string
	FreeMemory  string
	TotalMemory string
}

type RouterStatus struct {
	ID        int
	Name      string
	Connected bool
	Error     string
}
