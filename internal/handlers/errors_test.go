package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		message    string
		wantStatus int
	}{
		{
			name:       "bad request",
			status:     http.StatusBadRequest,
			code:       ErrCodeBadRequest,
			message:    "Invalid input",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			code:       ErrCodeNotFound,
			message:    "Resource not found",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal error",
			status:     http.StatusInternalServerError,
			code:       ErrCodeInternal,
			message:    "Something went wrong",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.status, tt.code, tt.message)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			var response APIError
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Code != tt.code {
				t.Errorf("Expected code %s, got %s", tt.code, response.Code)
			}
			if response.Message != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, response.Message)
			}
			if response.Status != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, response.Status)
			}
		})
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequest(w, "Invalid data")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response APIError
	_ = json.NewDecoder(w.Body).Decode(&response)
	if response.Message != "Resource not found" {
		t.Errorf("Expected default message, got %s", response.Message)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"hello": "world"}
	WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	_ = json.NewDecoder(w.Body).Decode(&response)
	if response["hello"] != "world" {
		t.Errorf("Expected hello=world, got %v", response)
	}
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSuccess(w, map[string]int{"count": 42})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestWriteCreated(t *testing.T) {
	w := httptest.NewRecorder()
	WriteCreated(w, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		w := httptest.NewRecorder()
		Unauthorized(w, "Token expired")

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}

		var response APIError
		_ = json.NewDecoder(w.Body).Decode(&response)
		if response.Message != "Token expired" {
			t.Errorf("Expected message 'Token expired', got %s", response.Message)
		}
		if response.Code != ErrCodeUnauthorized {
			t.Errorf("Expected code %s, got %s", ErrCodeUnauthorized, response.Code)
		}
	})

	t.Run("empty message uses default", func(t *testing.T) {
		w := httptest.NewRecorder()
		Unauthorized(w, "")

		var response APIError
		_ = json.NewDecoder(w.Body).Decode(&response)
		if response.Message != "Authentication required" {
			t.Errorf("Expected default message 'Authentication required', got %s", response.Message)
		}
	})
}

func TestForbidden(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		w := httptest.NewRecorder()
		Forbidden(w, "Admin only")

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
		}

		var response APIError
		_ = json.NewDecoder(w.Body).Decode(&response)
		if response.Message != "Admin only" {
			t.Errorf("Expected message 'Admin only', got %s", response.Message)
		}
	})

	t.Run("empty message uses default", func(t *testing.T) {
		w := httptest.NewRecorder()
		Forbidden(w, "")

		var response APIError
		_ = json.NewDecoder(w.Body).Decode(&response)
		if response.Message != "Access denied" {
			t.Errorf("Expected default message 'Access denied', got %s", response.Message)
		}
	})
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()
	Conflict(w, "Already exists")

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
	}

	var response APIError
	_ = json.NewDecoder(w.Body).Decode(&response)
	if response.Code != ErrCodeConflict {
		t.Errorf("Expected code %s, got %s", ErrCodeConflict, response.Code)
	}
}

func TestInternalError(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		w := httptest.NewRecorder()
		InternalError(w, "Database down")

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}

		var response APIError
		_ = json.NewDecoder(w.Body).Decode(&response)
		if response.Message != "Database down" {
			t.Errorf("Expected message 'Database down', got %s", response.Message)
		}
	})

	t.Run("empty message uses default", func(t *testing.T) {
		w := httptest.NewRecorder()
		InternalError(w, "")

		var response APIError
		_ = json.NewDecoder(w.Body).Decode(&response)
		if response.Message != "An internal error occurred" {
			t.Errorf("Expected default message 'An internal error occurred', got %s", response.Message)
		}
	})
}

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	ValidationError(w, "Name is required")

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("Expected status %d, got %d", http.StatusUnprocessableEntity, w.Code)
	}

	var response APIError
	_ = json.NewDecoder(w.Body).Decode(&response)
	if response.Code != ErrCodeValidation {
		t.Errorf("Expected code %s, got %s", ErrCodeValidation, response.Code)
	}
	if response.Message != "Name is required" {
		t.Errorf("Expected message 'Name is required', got %s", response.Message)
	}
}

func TestNotFound_WithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "App not found")

	var response APIError
	_ = json.NewDecoder(w.Body).Decode(&response)
	if response.Message != "App not found" {
		t.Errorf("Expected message 'App not found', got %s", response.Message)
	}
}

func TestWriteError_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "test")

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
}

func TestWriteError_ErrorField(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusNotFound, ErrCodeNotFound, "test")

	var response APIError
	_ = json.NewDecoder(w.Body).Decode(&response)
	if response.Error != "Not Found" {
		t.Errorf("Expected Error 'Not Found', got %s", response.Error)
	}
}
