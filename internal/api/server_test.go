package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/annurdien/stealth/internal/models"
)

func TestHealthHandler(t *testing.T) {
	// Setup Fiber App without a session manager for the health check
	app := NewServer(nil, "1.0.0")

	// Create a new HTTP request targeting the health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	// Perform the request
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	// Verify HTTP Status
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	// Read and verify Response Body
	body, _ := io.ReadAll(resp.Body)
	var healthResp models.HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if healthResp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", healthResp.Status)
	}
}

func TestIndexHandler(t *testing.T) {
	app := NewServer(nil, "vTest")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var indexResp models.IndexResponse
	if err := json.Unmarshal(body, &indexResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if indexResp.Version != "vTest" {
		t.Errorf("Expected version 'vTest', got '%v'", indexResp.Version)
	}
}
