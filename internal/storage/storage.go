package storage

import (
	"context"
	"io"
)

type Storage interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error)
	DeleteObjectsByPrefix(ctx context.Context, prefix string) error
}
