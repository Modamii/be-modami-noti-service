package httputil

import (
	"encoding/json"
	"net/http"
)

// Response is the standard API response envelope.
type Response struct {
	Data   interface{} `json:"data"`
	Meta   *Meta       `json:"meta,omitempty"`
	Errors []APIError  `json:"errors,omitempty"`
}

// Meta contains pagination metadata.
type Meta struct {
	Total      int64  `json:"total"`
	Page       int    `json:"page,omitempty"`
	PerPage    int    `json:"per_page,omitempty"`
	TotalPages int    `json:"total_pages,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// APIError represents a single error in the response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// RespondJSON writes a JSON response with the given status code, data, and optional meta.
func RespondJSON(w http.ResponseWriter, status int, data interface{}, meta *Meta) {
	resp := Response{Data: data, Meta: meta}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// RespondError writes an error JSON response.
func RespondError(w http.ResponseWriter, status int, errs ...APIError) {
	resp := Response{Errors: errs}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// RespondNoContent writes a 204 No Content response.
func RespondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ErrBadRequest returns a 400 error response.
func ErrBadRequest(w http.ResponseWriter, msg string) {
	RespondError(w, http.StatusBadRequest, APIError{Code: "BAD_REQUEST", Message: msg})
}

// ErrNotFound returns a 404 error response.
func ErrNotFound(w http.ResponseWriter, msg string) {
	RespondError(w, http.StatusNotFound, APIError{Code: "NOT_FOUND", Message: msg})
}

// ErrInternal returns a 500 error response.
func ErrInternal(w http.ResponseWriter, msg string) {
	RespondError(w, http.StatusInternalServerError, APIError{Code: "INTERNAL_ERROR", Message: msg})
}
