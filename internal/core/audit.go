package core

type AuditRole string

const (
	RoleOwner    AuditRole = "owner"
	RoleOperator AuditRole = "operator"
	RoleViewer   AuditRole = "viewer"
)

type AuditEntry struct {
	ID        int
	AdminID   int
	AdminName string
	Action    string
	Target    string
	CreatedAt string
}

type Admin struct {
	ID           int
	Username     string
	PasswordHash string
	Role         AuditRole
}
