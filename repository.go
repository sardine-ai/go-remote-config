package main

import (
	"errors"
	"net/url"
)

type Repository interface {
	getData() (string, error)
	getType() string
	getPath() string
	getUrl() *url.URL
}

func NewRepository(repoType string) (Repository, error) {
	switch repoType {
	case "file":
		return NewFileRepository(*path)
	case "git":
		if *URL == "" {
			return nil, errors.New("url is required for git repositories")
		}
		return NewGitRepository(*URL, *path)
	default:
		return NewFileRepository(*path)
	}
}
