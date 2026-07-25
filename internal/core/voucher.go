package core

type VoucherSpec struct {
	Qty            int
	Profile        string
	Server         string
	Mode           string
	Prefix         string
	CharMode       string
	TimeLimitStr   string
	Comment        string
	Length         int
	DataLimitBytes int64
}

type VoucherResult struct {
	Username string
	Password string
}

// VoucherBatch is the result of a bulk generate including Mikhmon batch comment.
type VoucherBatch struct {
	Comment string
	Items   []VoucherResult
}

type Sale struct {
	ID        int
	RouterID  int
	Username  string
	Profile   string
	Comment   string
	Price     int
	CreatedAt string
}
