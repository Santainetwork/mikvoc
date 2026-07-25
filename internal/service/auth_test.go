package service

import (
	"testing"

	"mikvoc/internal/authn"
	"mikvoc/internal/core"
)

type fakeAdmin struct {
	user, hash string
	role       core.AuditRole
	id         int
	byUser     map[string]*core.Admin
}

func (f *fakeAdmin) GetAdmin() (string, string) { return f.user, f.hash }
func (f *fakeAdmin) SetAdmin(u, h string) error { f.user, f.hash = u, h; return nil }
func (f *fakeAdmin) ListAdmins() ([]core.Admin, error) {
	return nil, nil
}
func (f *fakeAdmin) GetAdminByUsername(username string) (*core.Admin, error) {
	if f.byUser != nil {
		if a, ok := f.byUser[username]; ok {
			cp := *a
			return &cp, nil
		}
		return nil, core.ErrNotFound
	}
	if username != f.user {
		return nil, core.ErrNotFound
	}
	role := f.role
	if role == "" {
		role = core.RoleOwner
	}
	return &core.Admin{ID: f.id, Username: f.user, PasswordHash: f.hash, Role: role}, nil
}
func (f *fakeAdmin) GetAdminByID(id int) (*core.Admin, error) {
	return nil, core.ErrNotFound
}
func (f *fakeAdmin) CreateAdmin(username, passwordHash string, role core.AuditRole) error {
	return nil
}
func (f *fakeAdmin) UpdateAdmin(id int, username, passwordHash string, role core.AuditRole) error {
	if passwordHash != "" {
		f.hash = passwordHash
	}
	f.user = username
	f.role = role
	if f.byUser != nil {
		if a, ok := f.byUser[username]; ok {
			if passwordHash != "" {
				a.PasswordHash = passwordHash
			}
			a.Role = role
		}
	}
	return nil
}
func (f *fakeAdmin) DeleteAdmin(id int) error  { return nil }
func (f *fakeAdmin) CountOwners() (int, error) { return 1, nil }

func TestAuthLogin_WrongPassword(t *testing.T) {
	hash, err := authn.HashPassword("correct")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewAuth(&fakeAdmin{user: "admin", hash: hash, id: 1})
	_, err = svc.Login("admin", "wrong")
	if err != core.ErrUnauthorized {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthLogin_Success(t *testing.T) {
	hash, err := authn.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewAuth(&fakeAdmin{user: "admin", hash: hash, id: 1, role: core.RoleOperator})
	admin, err := svc.Login("admin", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Username != "admin" {
		t.Fatalf("username=%q", admin.Username)
	}
	if admin.ID != 1 {
		t.Fatalf("id=%d", admin.ID)
	}
	if admin.Role != core.RoleOperator {
		t.Fatalf("role=%q", admin.Role)
	}
}

func TestAuthLogin_MigratePlaintext(t *testing.T) {
	fake := &fakeAdmin{user: "admin", hash: "plainpass", id: 1}
	svc := NewAuth(fake)
	admin, err := svc.Login("admin", "plainpass")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Username != "admin" {
		t.Fatalf("username=%q", admin.Username)
	}
	if !authn.IsBcryptHash(fake.hash) {
		t.Fatalf("expected bcrypt migration, got %q", fake.hash)
	}
	if !authn.VerifyPassword(fake.hash, "plainpass") {
		t.Fatal("migrated hash does not verify")
	}
}

func TestAuthLogin_WrongUser(t *testing.T) {
	svc := NewAuth(&fakeAdmin{user: "admin", hash: "x"})
	_, err := svc.Login("other", "x")
	if err != core.ErrUnauthorized {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthLogin_EmptyRoleBecomesOwner(t *testing.T) {
	hash, err := authn.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewAuth(&fakeAdmin{user: "admin", hash: hash, id: 2, role: ""})
	admin, err := svc.Login("admin", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Role != core.RoleOwner {
		t.Fatalf("role=%q want owner", admin.Role)
	}
}
