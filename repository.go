package main

import (
	"context"
	"errors"
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
			logrus.Error("path is required")
			return nil, errors.New("path is required")
		}
		return NewFileRepository(*path)
	case "git":
		if *path == "" {
			logrus.Error("path is required")
			return nil, errors.New("path is required")
		}
		if *URL == "" {
			logrus.Error("URL is required")
			return nil, errors.New("URL is required")
		}
		return NewGitRepository(*URL, *path)
	case "http":
		if *URL == "" {
			logrus.Error("URL is required")
			return nil, errors.New("URL is required")
		}
		return NewWebRepository(*URL)
	default:
		return NewFileRepository(*path)
	}
}
