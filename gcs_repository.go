package main

import (
	"context"
	"io"
	"net/url"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type GCSRepository struct {
	lastUpdateSeconds int64
	data              string
	bucket            string
	key               string
}

func (g *GCSRepository) getData(ctx context.Context) (string, error) {
	if ((time.Now().Unix() - g.lastUpdateSeconds) < 10) && g.data != "" {
		logrus.Debug("returning cached file")
		return g.data, nil
	}
	logrus.Debug("fetching file")

	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		logrus.Debug("error creating client")
		return "", err
	}
	defer client.Close()

	bucket := client.Bucket(g.bucket)
	obj := bucket.Object(g.key)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		logrus.Debug("error creating reader")
		return "", err
	}
	defer reader.Close()

	logrus.Debug("reading file")
	data, err := io.ReadAll(reader)
	if err != nil {
		logrus.Debug("error reading file")
		return "", err
	}

	logrus.Debug("setting data")
	g.data = string(data)
	logrus.Debug("setting lastUpdateSeconds")
	g.lastUpdateSeconds = time.Now().Unix()
	return g.data, nil
}

func (g *GCSRepository) getType() string {
	return "gcs"
}

func (g *GCSRepository) getPath() string {
	return g.bucket + "/" + g.key
}

func (g *GCSRepository) getUrl() *url.URL {
	return nil
}

func NewGCSRepository(bucket, key string) (Repository, error) {
	return &GCSRepository{bucket: bucket, key: key}, nil
}
