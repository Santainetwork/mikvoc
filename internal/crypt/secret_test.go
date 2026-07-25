package crypt

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	c, err := New("secret-test")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := c.Encrypt("router-pass")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(encrypted) || encrypted == "router-pass" {
		t.Fatalf("expected encrypted value, got %q", encrypted)
	}
	plaintext, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "router-pass" {
		t.Fatalf("got %q", plaintext)
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	c, _ := New("secret-test")
	plaintext, err := c.Decrypt("legacy-pass")
	if err != nil || plaintext != "legacy-pass" {
		t.Fatalf("got %q, %v", plaintext, err)
	}
}

func TestWrongSecretFails(t *testing.T) {
	first, _ := New("secret-one")
	second, _ := New("secret-two")
	encrypted, _ := first.Encrypt("router-pass")
	if _, err := second.Decrypt(encrypted); err == nil {
		t.Fatal("expected decrypt error with wrong secret")
	}
}
