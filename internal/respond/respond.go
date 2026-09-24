// Package respond provides helpers for writing consistent JSON responses.
package respond

import (
	"encoding/json"
	"net/http"
)

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// Error writes a structured error JSON response.
func Error(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	JSON(w, status, errorEnvelope{
		Error: errorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
