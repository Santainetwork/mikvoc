package authn

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if !IsBcryptHash(hash) {
		t.Fatalf("expected bcrypt hash, got %q", hash)
	}
	if !VerifyPassword(hash, "rahasia123") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "salah") {
		t.Fatal("wrong password accepted")
	}
}

func TestVerifyPlaintextForMigration(t *testing.T) {
	if !VerifyPassword("admin", "admin") {
		t.Fatal("legacy plaintext should verify during migration")
	}
}
