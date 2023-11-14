package server

import (
	"context"
	"github.com/sardine-ai/go-remote-config/source"
	"github.com/go-http-utils/etag"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type Server struct {
	Repositories    []source.Repository
	RefreshInterval time.Duration
	cancel          context.CancelFunc
	AuthKey         string
}

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
	}
	for _, repo := range server.Repositories {
		err := repo.Refresh()
		if err != nil {
			logrus.WithError(err).Error("error refreshing repository")
		}
	}
	for _, repo := range server.Repositories {
		go refresh(ctx, repo, refreshInterval)
	}
	return server
}

func refresh(ctx context.Context, repository source.Repository, refreshInterval time.Duration) {
	ticker := time.NewTicker(refreshInterval)
	for {
		select {
		case <-ticker.C:
			err := repository.Refresh()
			if err != nil {
				logrus.WithError(err).Error("error refreshing repository")
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (s *Server) Stop() {
	s.cancel()
}

func (s *Server) Start(addr string) {
	logrus.Info("Starting server")

	handlers := s.CreateHandlers()
	handler := etag.Handler(handlers, false)
	if s.AuthKey != "" {
		handler = Auth(handler, s.AuthKey)
	}

	err := http.ListenAndServe(addr, handler)
	if err != nil {
		logrus.WithError(err).Fatal("error starting server")
	}
}

func (s *Server) CreateHandlers() http.Handler {
	mux := http.NewServeMux()
	for _, repo := range s.Repositories {
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
		if key != authKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
