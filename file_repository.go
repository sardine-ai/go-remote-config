package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type FileRepository struct {
	lastUpdateSeconds int64
	data              string
	path              string
	url               *url.URL
}

func (f *FileRepository) getData(ctx context.Context) (string, error) {
	if ((time.Now().Unix() - f.lastUpdateSeconds) < 10) && f.data != "" {
		logrus.WithContext(ctx).Debug("returning cached file")
		return f.data, nil
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		panic(err)
		return "", err
	}
	f.data = string(data)
	f.lastUpdateSeconds = time.Now().Unix()
	return string(data), nil
}

func (f *FileRepository) getType() string {
	return "file"
}

func (f *FileRepository) getPath() string {
	return f.path
}

func (f *FileRepository) getUrl() *url.URL {
	return f.url
}

func NewFileRepository(path string) (Repository, error) {
	toURL, err := filePathToURL(path)
	if err != nil {
		return nil, err
	}
	path, err = makeAbsoluteFilePath(path)
	if err != nil {
		return nil, err
	}
	return &FileRepository{path: path, url: toURL}, nil
}

func filePathToURL(filePath string) (*url.URL, error) {
	// Convert file path to absolute path, if it's not already absolute.
	absPath, err := makeAbsoluteFilePath(filePath)
	if err != nil {
		return nil, err
	}

	// Create a URL from the absolute path.
	fileURL := &url.URL{
		Scheme: "file",
		Path:   absPath,
	}

	return fileURL, nil
}

func makeAbsoluteFilePath(filePath string) (string, error) {
	// Convert the input file path to an absolute path.
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		logrus.WithError(err).Error("error getting absolute path")
		return "", err
	}

	return absPath, nil
}
