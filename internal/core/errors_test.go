package core

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestStatusOf_Nil(t *testing.T) {
	if got := StatusOf(nil); got != http.StatusOK {
		t.Fatalf("StatusOf(nil) = %d, want %d", got, http.StatusOK)
	}
}

func TestStatusOf_Sentinels(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{ErrInvalidInput, http.StatusBadRequest},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrForbidden, http.StatusForbidden},
		{ErrNotFound, http.StatusNotFound},
		{ErrConflict, http.StatusConflict},
		{ErrNotConnected, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		if got := StatusOf(c.err); got != c.want {
			t.Errorf("StatusOf(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestStatusOf_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("lookup failed: %w", ErrNotFound)
	if got := StatusOf(wrapped); got != http.StatusNotFound {
		t.Errorf("StatusOf(wrapped ErrNotFound) = %d, want %d", got, http.StatusNotFound)
	}
}

func TestStatusOf_Unknown(t *testing.T) {
	if got := StatusOf(errors.New("boom")); got != http.StatusInternalServerError {
		t.Errorf("StatusOf(unknown) = %d, want %d", got, http.StatusInternalServerError)
	}
}
