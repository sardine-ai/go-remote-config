package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sardine-ai/go-remote-config/source"
)

// mockRepository is a thread-safe mock repository for testing
type mockRepository struct {
	mu           sync.RWMutex
	name         string
	data         map[string]interface{}
	rawData      []byte
	refreshCount int
	shouldError  bool
	refreshDelay time.Duration
}

func newMockRepository(name string) *mockRepository {
	return &mockRepository{
		name: name,
		data: map[string]interface{}{
			"key": "value",
		},
		rawData: []byte("key: value\n"),
	}
}

func (m *mockRepository) GetName() string {
	return m.name
}

func (m *mockRepository) GetData(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *mockRepository) GetRawData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rawData
}

func (m *mockRepository) Refresh() error {
	if m.refreshDelay > 0 {
		time.Sleep(m.refreshDelay)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshCount++
	if m.shouldError {
		return errors.New("mock refresh error")
	}
	return nil
}

func (m *mockRepository) getRefreshCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.refreshCount
}

func (m *mockRepository) setError(shouldError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = shouldError
}

// TestServerHealthEndpoint tests the /health endpoint
func TestServerHealthEndpoint(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	// Test healthy state
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", result["status"])
	}
}

// TestServerHealthEndpointUnhealthy tests /health when repository is unhealthy
func TestServerHealthEndpointUnhealthy(t *testing.T) {
	repo := newMockRepository("test")
	// Make repo fail from the start
	repo.setError(true)
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 10*time.Second)
	defer server.Stop()

	// The initial refresh failed, so server should be unhealthy
	handler := server.CreateHandlers()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["status"] != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%v'", result["status"])
	}
}

// TestServerReadyEndpoint tests the /ready endpoint
func TestServerReadyEndpoint(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["status"] != "ready" {
		t.Errorf("Expected status 'ready', got '%s'", result["status"])
	}
}

// TestServerStatusEndpoint tests the /status endpoint
func TestServerStatusEndpoint(t *testing.T) {
	repo1 := newMockRepository("repo1")
	repo2 := newMockRepository("repo2")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo1, repo2}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if result["healthy"] != true {
		t.Errorf("Expected healthy=true, got %v", result["healthy"])
	}
	if result["ready"] != true {
		t.Errorf("Expected ready=true, got %v", result["ready"])
	}

	repos, ok := result["repositories"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected repositories in response")
	}
	if _, ok := repos["repo1"]; !ok {
		t.Error("Expected repo1 in repositories")
	}
	if _, ok := repos["repo2"]; !ok {
		t.Error("Expected repo2 in repositories")
	}
}

// TestServerRepositoryEndpoint tests the repository config endpoint
func TestServerRepositoryEndpoint(t *testing.T) {
	repo := newMockRepository("config")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	req := httptest.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "key: value\n" {
		t.Errorf("Expected 'key: value\\n', got '%s'", string(body))
	}
}

// TestServerMethodNotAllowed tests that non-GET/HEAD methods are rejected
func TestServerMethodNotAllowed(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	endpoints := []string{"/health", "/ready", "/status", "/test"}

	for _, method := range methods {
		for _, endpoint := range endpoints {
			req := httptest.NewRequest(method, endpoint, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: Expected status 405, got %d", method, endpoint, resp.StatusCode)
			}
		}
	}
}

// TestServerAuthMiddleware tests the authentication middleware
func TestServerAuthMiddleware(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	server.AuthKey = "secret-key"
	defer server.Stop()

	handler := server.CreateHandlers()
	handler = Auth(handler, server.AuthKey)

	// Test without auth key
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth key, got %d", w.Result().StatusCode)
	}

	// Test with wrong auth key
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-KEY", "wrong-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 with wrong auth key, got %d", w.Result().StatusCode)
	}

	// Test with correct auth key
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-KEY", "secret-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected 200 with correct auth key, got %d", w.Result().StatusCode)
	}
}

// TestServerHealthEndpointsBypassAuth tests that health endpoints don't require authentication
func TestServerHealthEndpointsBypassAuth(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	server.AuthKey = "secret-key"
	defer server.Stop()

	handler := server.CreateHandlers()
	handler = Auth(handler, server.AuthKey)

	// Health endpoints should work without auth key
	healthEndpoints := []string{"/health", "/ready", "/status"}
	for _, endpoint := range healthEndpoints {
		req := httptest.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("%s: Expected 200 without auth key, got %d", endpoint, w.Result().StatusCode)
		}
	}

	// Config endpoint should still require auth
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("/test: Expected 401 without auth key, got %d", w.Result().StatusCode)
	}
}

// TestServerStop tests that Stop() properly cleans up
func TestServerStop(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 10*time.Second)

	// Initial refresh should have happened
	initialCount := repo.getRefreshCount()
	if initialCount < 1 {
		t.Errorf("Expected at least 1 refresh, got %d", initialCount)
	}

	// Stop the server
	server.Stop()

	// Verify stop completed (wg.Wait() returned)
	// The server should be stopped now
	if server.cancel == nil {
		t.Error("Expected cancel to be set")
	}
}

// TestServerIsHealthy tests the IsHealthy method
func TestServerIsHealthy(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	if !server.IsHealthy() {
		t.Error("Expected server to be healthy initially")
	}
}

// TestServerIsReady tests the IsReady method
func TestServerIsReady(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	if !server.IsReady() {
		t.Error("Expected server to be ready after initial refresh")
	}
}

// TestServerGetRepositoryStatus tests the GetRepositoryStatus method
func TestServerGetRepositoryStatus(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	status := server.GetRepositoryStatus()
	if len(status) != 1 {
		t.Errorf("Expected 1 repository status, got %d", len(status))
	}

	repoStatus, ok := status["test"]
	if !ok {
		t.Fatal("Expected 'test' repository in status")
	}

	if repoStatus.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", repoStatus.Name)
	}
	if repoStatus.RefreshCount != 1 {
		t.Errorf("Expected refresh count 1, got %d", repoStatus.RefreshCount)
	}
	if !repoStatus.IsHealthy {
		t.Error("Expected repository to be healthy")
	}
}

// TestServerRefreshRaceCondition tests concurrent access to server status
func TestServerRefreshRaceCondition(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 10*time.Second)
	defer server.Stop()

	var wg sync.WaitGroup
	const numGoroutines = 50

	// Start goroutines that read status concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = server.IsHealthy()
				_ = server.IsReady()
				_ = server.GetRepositoryStatus()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()
}

// TestServerMultipleRepositories tests server with multiple repositories
func TestServerMultipleRepositories(t *testing.T) {
	repo1 := newMockRepository("repo1")
	repo2 := newMockRepository("repo2")
	repo3 := newMockRepository("repo3")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo1, repo2, repo3}, 1*time.Second)
	defer server.Stop()

	if !server.IsHealthy() {
		t.Error("Expected server with multiple repos to be healthy")
	}

	status := server.GetRepositoryStatus()
	if len(status) != 3 {
		t.Errorf("Expected 3 repository statuses, got %d", len(status))
	}

	handler := server.CreateHandlers()

	// Test each repository endpoint
	for _, name := range []string{"repo1", "repo2", "repo3"} {
		req := httptest.NewRequest("GET", "/"+name, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for /%s, got %d", name, w.Result().StatusCode)
		}
	}
}

// TestServerOneRepoFailsHealthCheck tests that one failing repo marks server unhealthy
func TestServerOneRepoFailsHealthCheck(t *testing.T) {
	repo1 := newMockRepository("repo1")
	repo2 := newMockRepository("repo2")
	// Make repo1 fail from the start
	repo1.setError(true)
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo1, repo2}, 10*time.Second)
	defer server.Stop()

	// Server should be unhealthy because repo1 failed initial refresh
	if server.IsHealthy() {
		t.Error("Expected server to be unhealthy when one repo fails")
	}

	// But still ready (repo2 is working)
	if !server.IsReady() {
		t.Error("Expected server to still be ready with one working repo")
	}
}

// TestServerStartReturnsError tests that Start returns error properly
func TestServerStartReturnsError(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	// Try to start on an invalid address
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start("invalid-address:99999999")
	}()

	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error for invalid address")
		}
	case <-time.After(2 * time.Second):
		// May timeout waiting for error, which is acceptable
	}
}

// TestServerShutdown tests graceful shutdown
func TestServerShutdown(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)

	// Start server in background
	go func() {
		_ = server.Start("127.0.0.1:0") // Use port 0 for random available port
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown should complete without error
	err := server.Shutdown()
	if err != nil {
		t.Errorf("Expected no error on shutdown, got: %v", err)
	}
}

// TestServerRefreshIntervalMinimum tests that refresh interval is enforced to minimum
func TestServerRefreshIntervalMinimum(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()

	// Try to create server with 1 second refresh (below 5 second minimum)
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	// The refresh interval should be set to 5 seconds minimum
	if server.RefreshInterval != 5*time.Second {
		t.Errorf("Expected refresh interval to be 5s, got %v", server.RefreshInterval)
	}
}

// TestServerConcurrentHTTPRequests tests concurrent HTTP requests
func TestServerConcurrentHTTPRequests(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	var wg sync.WaitGroup
	const numGoroutines = 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// Test different endpoints
				endpoints := []string{"/health", "/ready", "/status", "/test"}
				for _, endpoint := range endpoints {
					req := httptest.NewRequest("GET", endpoint, nil)
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)
				}
			}
		}()
	}

	wg.Wait()
}

// TestServerHEADRequests tests that HEAD requests work for all endpoints
func TestServerHEADRequests(t *testing.T) {
	repo := newMockRepository("test")
	ctx := context.Background()
	server := NewServer(ctx, []source.Repository{repo}, 1*time.Second)
	defer server.Stop()

	handler := server.CreateHandlers()

	endpoints := []string{"/health", "/ready", "/status", "/test"}
	for _, endpoint := range endpoints {
		req := httptest.NewRequest("HEAD", endpoint, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("HEAD %s: Expected 200, got %d", endpoint, w.Result().StatusCode)
		}
	}
}
