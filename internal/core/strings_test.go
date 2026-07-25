package core

import "testing"

func TestContainsIgnoreCase(t *testing.T) {
	if !ContainsIgnoreCase("Alice", "ali") {
		t.Fatal("should match")
	}
	if ContainsIgnoreCase("bob", "alice") {
		t.Fatal("should not match")
	}
	if !ContainsIgnoreCase("x", "") {
		t.Fatal("empty substr matches")
	}
}
