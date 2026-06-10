package attachment

import (
	"context"
	"io"
	"time"
)

type Meta struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType,omitempty"`
	Size        int64     `json:"size,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Store interface {
	Put(ctx context.Context, filename, contentType string, r io.Reader, size int64) (*Meta, error)
	Get(ctx context.Context, id string) (io.ReadCloser, *Meta, error)
	Head(ctx context.Context, id string) (*Meta, error)
}
