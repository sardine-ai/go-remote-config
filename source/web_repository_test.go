package source

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestWebRepositoryRefresh tests basic refresh functionality
func TestWebRepositoryRefresh(t *testing.T) {
	// Create a mock server that returns YAML config
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte("key: value\nnested:\n  foo: bar\n"))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	repo := &WebRepository{
		Name: "test",
		URL:  serverURL,
	}

	err := repo.Refresh()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Test GetData
	val, ok := repo.GetData("key")
	if !ok {
		t.Fatal("Expected 'key' to exist")
	}
	if val != "value" {
		t.Errorf("Expected 'value', got '%v'", val)
	}

	// Test nested data
	nested, ok := repo.GetData("nested")
	if !ok {
		t.Fatal("Expected 'nested' to exist")
	}
	nestedMap, ok := nested.(map[string]interface{})
	if !ok {
		t.Fatal("Expected nested to be a map")
	}
	if nestedMap["foo"] != "bar" {
		t.Errorf("Expected nested.foo = 'bar', got '%v'", nestedMap["foo"])
	}

	// Test GetRawData
	rawData := repo.GetRawData()
	if string(rawData) != "key: value\nnested:\n  foo: bar\n" {
		t.Errorf("Expected raw data to match, got: %s", string(rawData))
	}
}

// TestWebRepositoryGetName tests the GetName method
func TestWebRepositoryGetName(t *testing.T) {
	repo := &WebRepository{
		Name: "test-repo",
	}

	if repo.GetName() != "test-repo" {
		t.Errorf("Expected 'test-repo', got '%s'", repo.GetName())
	}
}

// TestWebRepositoryWithAPIKey tests that X-API-Key header is sent when APIKey is configured
func TestWebRepositoryWithAPIKey(t *testing.T) {
	receivedAPIKey := ""

	// Create a mock server that checks for the X-API-Key header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte("key: value\n"))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	repo := &WebRepository{
		Name:   "test",
		URL:    serverURL,
		APIKey: "secret-api-key",
	}

	err := repo.Refresh()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if receivedAPIKey != "secret-api-key" {
		t.Errorf("Expected X-API-Key to be 'secret-api-key', got '%s'", receivedAPIKey)
	}
}

// TestWebRepositoryWithoutAPIKey tests that no X-API-Key header is sent when APIKey is empty
func TestWebRepositoryWithoutAPIKey(t *testing.T) {
	receivedAPIKey := ""
	headerPresent := false

	// Create a mock server that checks for the X-API-Key header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("X-API-Key")
		_, headerPresent = r.Header["X-Api-Key"]
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte("key: value\n"))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	repo := &WebRepository{
		Name: "test",
		URL:  serverURL,
		// No APIKey set
	}

	err := repo.Refresh()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if headerPresent || receivedAPIKey != "" {
		t.Errorf("Expected no X-API-Key header, but got '%s'", receivedAPIKey)
	}
}

// TestWebRepositoryAPIKeyAuth tests authentication using X-API-Key with a server that requires it
func TestWebRepositoryAPIKeyAuth(t *testing.T) {
	requiredKey := "valid-api-key"

	// Create a mock server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != requiredKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte("authenticated: true\n"))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)

	// Test with correct API key
	repo := &WebRepository{
		Name:   "test",
		URL:    serverURL,
		APIKey: requiredKey,
	}

	err := repo.Refresh()
	if err != nil {
		t.Fatalf("Expected no error with valid API key, got: %v", err)
	}

	val, ok := repo.GetData("authenticated")
	if !ok || val != true {
		t.Error("Expected 'authenticated: true' in response")
	}
}

// TestWebRepositoryAPIKeyAuthFailure tests that authentication fails without correct API key
func TestWebRepositoryAPIKeyAuthFailure(t *testing.T) {
	requiredKey := "valid-api-key"

	// Create a mock server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != requiredKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte("authenticated: true\n"))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)

	// Test with wrong API key
	repo := &WebRepository{
		Name:   "test",
		URL:    serverURL,
		APIKey: "wrong-key",
	}

	err := repo.Refresh()
	// The refresh should succeed (HTTP request completes) but with error response body
	// which will cause YAML unmarshal to fail since "Unauthorized\n" is not valid YAML
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

// TestWebRepositoryInvalidURL tests behavior with invalid URL
func TestWebRepositoryInvalidURL(t *testing.T) {
	invalidURL, _ := url.Parse("http://localhost:99999")
	repo := &WebRepository{
		Name: "test",
		URL:  invalidURL,
	}

	err := repo.Refresh()
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

// TestWebRepositoryInvalidYAML tests behavior with invalid YAML response
func TestWebRepositoryInvalidYAML(t *testing.T) {
	// Create a mock server that returns invalid YAML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid: yaml: content: ["))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	repo := &WebRepository{
		Name: "test",
		URL:  serverURL,
	}

	err := repo.Refresh()
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

// TestWebRepositoryGetDataMissing tests GetData for non-existent key
func TestWebRepositoryGetDataMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("key: value\n"))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	repo := &WebRepository{
		Name: "test",
		URL:  serverURL,
	}

	_ = repo.Refresh()

	_, ok := repo.GetData("nonexistent")
	if ok {
		t.Error("Expected 'nonexistent' key to not exist")
	}
}
