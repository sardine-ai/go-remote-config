package main

import (
	"errors"
	"flag"
	"github.com/divakarmanoj/go-remote-config-server/source"
	"github.com/go-http-utils/etag"
	"github.com/sirupsen/logrus"
	"net/http"
)

var authKey = flag.String("auth_key", "", "auth key for the server")

var repoType = flag.String("repo_type", "", "repository type")

var path = flag.String("path", "", "path to the file")

var URL = flag.String("url", "", "url to the file")

var repository source.Repository

func main() {
	flag.Parse()
	var err error
	repository, err = NewRepository(*repoType)
	if err != nil {
		logrus.WithError(err).Fatal("error creating repository")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", MainHandler)
	handler := etag.Handler(mux, false)
	if *authKey != "" {
		handler = Auth(handler, *authKey)
	}
	err = http.ListenAndServe(":8090", etag.Handler(mux, false))
	if err != nil {
		logrus.WithError(err).Fatal("error starting server")
	}
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		ReadRemoteConfig(w, r)
	case "HEAD":
		ReadRemoteConfig(w, r)
	}
}

func NewRepository(repoType string) (source.Repository, error) {
	switch repoType {
	case "fs":
		if *path == "" {
			logrus.Error("path is required")
			return nil, errors.New("path is required")
		}
		return source.NewFileRepository(*path)
	case "git":
		if *path == "" {
			logrus.Error("path is required")
			return nil, errors.New("path is required")
		}
		if *URL == "" {
			logrus.Error("URL is required")
			return nil, errors.New("URL is required")
		}
		return source.NewGitRepository(*URL, *path)
	case "http":
		if *URL == "" {
			logrus.Error("URL is required")
			return nil, errors.New("URL is required")
		}
		return source.NewWebRepository(*URL)
	default:
		return source.NewFileRepository(*path)
	}
}
