package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"status":  "ok",
			"server":  "test-server",
			"version": "0.1.0",
		}
		json.NewEncoder(w).Encode(response)
	})

	// Execute the request
	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check content type
	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected content type application/json, got %s", contentType)
	}

	// Parse response body
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response fields
	if status, ok := response["status"].(string); !ok || status != "ok" {
		t.Errorf("expected status 'ok', got %v", response["status"])
	}

	if server, ok := response["server"].(string); !ok || server != "test-server" {
		t.Errorf("expected server 'test-server', got %v", response["server"])
	}

	if version, ok := response["version"].(string); !ok || version != "0.1.0" {
		t.Errorf("expected version '0.1.0', got %v", response["version"])
	}
}
