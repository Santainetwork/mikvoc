package httpapi

import (
	"net/http"
)

// Helper function to set session values for testing
func sessionSet(r *http.Request, key string, value interface{}) {
	// This is a placeholder - actual session handling depends on your auth implementation
	_ = r // Use if needed
}
