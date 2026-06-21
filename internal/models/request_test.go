package models

import (
	"testing"
)

func TestV1Request_EffectiveMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{"Empty Method Defaults to GET", "", "GET"},
		{"GET Remains GET", "GET", "GET"},
		{"POST Remains POST", "POST", "POST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &V1Request{Method: tt.method}
			if got := req.EffectiveMethod(); got != tt.expected {
				t.Errorf("EffectiveMethod() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestV1Request_EffectiveTimeout(t *testing.T) {
	tests := []struct {
		name       string
		maxTimeout int
		expected   int
	}{
		{"Zero defaults to 60000", 0, 60000},
		{"Negative defaults to 60000", -100, 60000},
		{"Valid timeout remains", 30000, 30000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &V1Request{MaxTimeout: tt.maxTimeout}
			if got := req.EffectiveTimeout(); got != tt.expected {
				t.Errorf("EffectiveTimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}
