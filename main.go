package main

import (
	"flag"
	"github.com/go-http-utils/etag"
	"github.com/sirupsen/logrus"
	gorm "gorm.io/gorm"
	"net/http"
)

var db *gorm.DB

var authKey = flag.String("auth_key", "", "auth key for the server")

var repoType = flag.String("repo_type", "", "repository type")

var path = flag.String("path", "", "path to the file")

var URL = flag.String("url", "", "url to the file")

var repository Repository

func main() {
	flag.Parse()
	var err error
	if *path == "" {
		logrus.Fatal("path is required")
	}
	repository, err = NewRepository(*repoType)
	if err != nil {
		logrus.WithError(err).Fatal("error creating repository")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", MainHandler)

	http.ListenAndServe(":8080", etag.Handler(mux, false))
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		ReadRemoteConfig(w, r)
	case "HEAD":
		ReadRemoteConfig(w, r)
	}
}
