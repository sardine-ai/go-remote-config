package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadRemoteConfig(t *testing.T) {
	// Create a test server to serve the test data
	testData := "foo: bar\n"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testData))
	}))
	defer testServer.Close()

	// Create a new repository with the test server URL
	var err error
	repository, err = source.NewWebRepository(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new request to the test server
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new response recorder to capture the response
	rr := httptest.NewRecorder()

	// Call the ReadRemoteConfig function with the test request and response recorder
	ReadRemoteConfig(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, status)
	}

	// Check the response body
	if body := rr.Body.String(); body != testData {
		t.Errorf("expected body %q, got %q", testData, body)
	}
}
