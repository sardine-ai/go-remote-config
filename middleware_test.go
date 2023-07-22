package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth(t *testing.T) {
	// Create a test handler to be wrapped by the Auth middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Create a new request with an invalid X-API-KEY header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-KEY", "invalid")

	// Create a new response recorder to capture the response
	rr := httptest.NewRecorder()

	// Call the Auth middleware with the test handler and invalid auth key
	Auth(testHandler, "correct-key").ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	// Check the response body
	if body := rr.Body.String(); body != "Unauthorized\n" {
		t.Errorf("expected body %q, got %q", "Unauthorized\n", body)
	}

	// Create a new request with a valid X-API-KEY header
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-KEY", "correct-key")

	// Reset the response recorder
	rr = httptest.NewRecorder()

	// Call the Auth middleware with the test handler and valid auth key
	Auth(testHandler, "correct-key").ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, status)
	}

	// Check the response body
	if body := rr.Body.String(); body != "OK" {
		t.Errorf("expected body %q, got %q", "OK", body)
	}
}
