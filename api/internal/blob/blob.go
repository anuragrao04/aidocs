package blob

import "context"

type Store interface {
	Put(ctx context.Context, key string, contentType string, body []byte) error
	Get(ctx context.Context, key string) ([]byte, string, error)
	Delete(ctx context.Context, key string) error
}
