// Package controller holds one HTTP controller per API endpoint.
package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultLimit  = int32(25)
	maxLimit      = int32(100)
	presignExpiry = 15 * time.Minute
)

// writeJSON writes a JSON body with the given status.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		fmt.Printf("write response: %v\n", err)
	}
}

// writeError writes a JSON error body with the given status.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
