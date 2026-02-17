package handlers

import (
	"encoding/json"
	"net/http"
)

// APIError represents a standardized API error response
type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Status  int    `json:"status"`
}

// Common error codes
const (
	ErrCodeBadRequest   = "BAD_REQUEST"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeConflict     = "CONFLICT"
	ErrCodeInternal     = "INTERNAL_ERROR"
	ErrCodeValidation   = "VALIDATION_ERROR"
)

// WriteError writes a JSON error response
func WriteError(w http.ResponseWriter, status int, code, message string) {
	apiErr := APIError{
		Error:   http.StatusText(status),
		Message: message,
		Code:    code,
		Status:  status,
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiErr)
}

// BadRequest writes a 400 error
func BadRequest(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, message)
}

// Unauthorized writes a 401 error
func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Authentication required"
	}
	WriteError(w, http.StatusUnauthorized, ErrCodeUnauthorized, message)
}

// Forbidden writes a 403 error
func Forbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Access denied"
	}
	WriteError(w, http.StatusForbidden, ErrCodeForbidden, message)
}

// NotFound writes a 404 error
func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Resource not found"
	}
	WriteError(w, http.StatusNotFound, ErrCodeNotFound, message)
}

// Conflict writes a 409 error
func Conflict(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusConflict, ErrCodeConflict, message)
}

// InternalError writes a 500 error
func InternalError(w http.ResponseWriter, message string) {
	if message == "" {
		message = "An internal error occurred"
	}
	WriteError(w, http.StatusInternalServerError, ErrCodeInternal, message)
}

// ValidationError writes a 422 error
func ValidationError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusUnprocessableEntity, ErrCodeValidation, message)
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteSuccess writes a success response
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

// WriteCreated writes a 201 created response
func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, data)
}
