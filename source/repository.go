package source

import (
	"context"
	"net/url"
)

type Repository interface {
	GetData(ctx context.Context) (string, error)
	GetType() string
	GetPath() string
	GetUrl() *url.URL
}
