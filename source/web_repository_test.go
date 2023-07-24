package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebRepository(t *testing.T) {
	// Create a test server to serve the test data
	testData := "hello, world!"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testData))
	}))
	defer testServer.Close()

	// Create a new web repository
	repo, err := NewWebRepository(testServer.URL)

	// Test the GetUrl() function
	url := repo.GetUrl()
	if url.String() != testServer.URL {
		t.Errorf("expected %q, got %q", testServer.URL, url.String())
	}

	// Test the GetData() function
	data, err := repo.GetData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != testData {
		t.Errorf("expected %q, got %q", testData, string(data))
	}
}
