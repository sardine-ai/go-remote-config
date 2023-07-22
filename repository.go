package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"net/url"
)

type Repository interface {
	getData(ctx context.Context) (string, error)
	getType() string
	getPath() string
	getUrl() *url.URL
}

func NewRepository(repoType string) (Repository, error) {
	switch repoType {
	case "fs":
		if *path == "" {
			logrus.Fatal("path is required")
		}
		return NewFileRepository(*path)
	case "git":
		if *path == "" {
			logrus.Fatal("path is required")
		}
		if *URL == "" {
			logrus.Fatal("URL is required")
		}
		return NewGitRepository(*URL, *path)
	case "http":
		if *URL == "" {
			logrus.Fatal("URL is required")
		}
		return NewWebRepository(*URL)
	default:
		return NewFileRepository(*path)
	}
}
