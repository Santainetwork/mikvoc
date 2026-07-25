// Package core provides domain types and sentinel errors for MikVoc.
package core

import (
	"errors"
	"net/http"
)

var (
	ErrNotConnected  = errors.New("router not connected")
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
)

func StatusOf(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch {
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrNotConnected):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
