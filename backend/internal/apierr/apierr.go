// Package apierr provides a shared JSON error response helper used by multiple
// route handlers so every error envelope has the same shape.
package apierr

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Error body `json:"error"`
}

type body struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write writes a JSON error envelope with the given HTTP status, machine code,
// and human-readable message.
func Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: body{Code: code, Message: message}})
}
