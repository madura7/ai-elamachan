// Package apierr writes canonical ADR-0003 error envelopes.
package apierr

import (
	"encoding/json"
	"net/http"
)

type errDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errBody struct {
	Error errDetail `json:"error"`
}

// Write encodes { "error": { "code": ..., "message": ... } } and sets the status.
func Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errBody{Error: errDetail{Code: code, Message: message}})
}
