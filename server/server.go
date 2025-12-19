package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-http-utils/etag"
	"github.com/sardine-ai/go-remote-config/source"
	"github.com/sirupsen/logrus"
)

// Server serves configuration data over HTTP with automatic refresh.
type Server struct {
	Repositories    []source.Repository
	RefreshInterval time.Duration
	cancel          context.CancelFunc
	AuthKey         string
	wg              sync.WaitGroup

	// Mutex protects httpServer and repoStatus
	mu               sync.RWMutex
	httpServer       *http.Server
	repoStatus       map[string]*RepositoryStatus
	shutdownTimeout  time.Duration
}

// RepositoryStatus tracks the health status of a repository.
type RepositoryStatus struct {
	Name            string    `json:"name"`
	LastRefreshTime time.Time `json:"last_refresh_time"`
	LastRefreshErr  string    `json:"last_refresh_error,omitempty"`
	RefreshCount    int64     `json:"refresh_count"`
	RefreshErrors   int64     `json:"refresh_errors"`
	IsHealthy       bool      `json:"is_healthy"`
}

// NewServer creates a new configuration server with the given repositories.
func NewServer(ctx context.Context, repository []source.Repository, refreshInterval time.Duration) *Server {
	if refreshInterval < 5*time.Second {
		logrus.Warn("refresh interval too low, setting it to 5 seconds")
		refreshInterval = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(ctx)
	server := &Server{
		Repositories:    repository,
		RefreshInterval: refreshInterval,
		cancel:          cancel,
		repoStatus:      make(map[string]*RepositoryStatus),
		shutdownTimeout: 30 * time.Second,
	}

	// Initialize status tracking for each repository
	for _, repo := range server.Repositories {
		server.repoStatus[repo.GetName()] = &RepositoryStatus{
			Name: repo.GetName(),
		}
	}

	// Initial refresh
	for _, repo := range server.Repositories {
		err := repo.Refresh()
		if err != nil {
			logrus.WithError(err).WithField("repository", repo.GetName()).Error("error refreshing repository")
			server.recordRefreshError(repo.GetName(), err)
		} else {
			server.recordRefreshSuccess(repo.GetName())
		}
	}

	// Start background refresh goroutines
	for _, repo := range server.Repositories {
		server.wg.Add(1)
		go server.refresh(ctx, repo, refreshInterval)
	}
	return server
}

// refresh periodically refreshes a repository and tracks its status.
func (s *Server) refresh(ctx context.Context, repository source.Repository, refreshInterval time.Duration) {
	defer s.wg.Done()
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := repository.Refresh()
			if err != nil {
				logrus.WithError(err).WithField("repository", repository.GetName()).Error("error refreshing repository")
				s.recordRefreshError(repository.GetName(), err)
			} else {
				s.recordRefreshSuccess(repository.GetName())
			}
		case <-ctx.Done():
			return
		}
	}
}

// recordRefreshSuccess records a successful refresh for a repository.
func (s *Server) recordRefreshSuccess(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status, ok := s.repoStatus[name]; ok {
		status.LastRefreshTime = time.Now()
		status.LastRefreshErr = ""
		status.RefreshCount++
		status.IsHealthy = true
	}
}

// recordRefreshError records a failed refresh for a repository.
func (s *Server) recordRefreshError(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status, ok := s.repoStatus[name]; ok {
		status.LastRefreshErr = err.Error()
		status.RefreshErrors++
		status.IsHealthy = false
	}
}

// GetRepositoryStatus returns the status of all repositories.
func (s *Server) GetRepositoryStatus() map[string]*RepositoryStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid races
	result := make(map[string]*RepositoryStatus)
	for k, v := range s.repoStatus {
		statusCopy := *v
		result[k] = &statusCopy
	}
	return result
}

// IsHealthy returns true if all repositories are healthy.
func (s *Server) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, status := range s.repoStatus {
		if !status.IsHealthy {
			return false
		}
	}
	return len(s.repoStatus) > 0
}

// IsReady returns true if at least one repository has been successfully refreshed.
func (s *Server) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, status := range s.repoStatus {
		if status.RefreshCount > 0 && status.IsHealthy {
			return true
		}
	}
	return false
}

// Stop gracefully stops the server and waits for all goroutines to finish.
func (s *Server) Stop() {
	s.cancel()
	s.wg.Wait()
}

// Start starts the HTTP server and blocks until it's stopped.
// Returns an error if the server fails to start.
// Use StartWithGracefulShutdown for production deployments.
func (s *Server) Start(addr string) error {
	logrus.Info("Starting server on ", addr)

	handlers := s.CreateHandlers()
	handler := etag.Handler(handlers, false)
	if s.AuthKey != "" {
		handler = Auth(handler, s.AuthKey)
	}

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  3 * time.Minute,
		WriteTimeout: 3 * time.Minute,
		IdleTimeout:  10 * time.Minute,
	}

	// Store the server reference with proper locking
	s.mu.Lock()
	s.httpServer = httpServer
	s.mu.Unlock()

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Error("error starting server")
		return fmt.Errorf("server failed to start: %w", err)
	}
	return nil
}

// StartWithGracefulShutdown starts the server and handles OS signals for graceful shutdown.
// This is the recommended way to run the server in production.
// It blocks until the server is stopped via SIGINT or SIGTERM.
func (s *Server) StartWithGracefulShutdown(addr string) error {
	// Channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Channel to receive server errors
	errChan := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		if err := s.Start(addr); err != nil {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		logrus.WithField("signal", sig).Info("Received shutdown signal, initiating graceful shutdown")
	case err := <-errChan:
		return err
	}

	// Graceful shutdown
	return s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	// Stop refresh goroutines first
	s.Stop()

	// Get the HTTP server with proper locking
	s.mu.RLock()
	httpServer := s.httpServer
	s.mu.RUnlock()

	if httpServer == nil {
		return nil
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	logrus.Info("Shutting down HTTP server...")
	if err := httpServer.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Error during server shutdown")
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	logrus.Info("Server shutdown complete")
	return nil
}

// CreateHandlers creates the HTTP handlers including health and readiness endpoints.
func (s *Server) CreateHandlers() http.Handler {
	mux := http.NewServeMux()

	// Health endpoint - returns 200 if server is running and all repos are healthy
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if s.IsHealthy() {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":       "healthy",
				"repositories": s.GetRepositoryStatus(),
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":       "unhealthy",
				"repositories": s.GetRepositoryStatus(),
			})
		}
	})

	// Readiness endpoint - returns 200 if at least one repo has been refreshed
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if s.IsReady() {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		}
	})

	// Status endpoint - detailed status of all repositories
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy":      s.IsHealthy(),
			"ready":        s.IsReady(),
			"repositories": s.GetRepositoryStatus(),
		})
	})

	// Repository endpoints
	for _, repo := range s.Repositories {
		repo := repo // Capture loop variable to avoid closure bug
		mux.HandleFunc("/"+repo.GetName(), func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" && r.Method != "HEAD" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			response := repo.GetRawData()
			_, err := w.Write(response)
			if err != nil {
				logrus.WithError(err).Error("error writing response")
			}
		})
	}
	return mux
}

func Auth(next http.Handler, authKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check banner api key
		key := r.Header.Get("X-API-KEY")
		if key == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(key), []byte(authKey)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
