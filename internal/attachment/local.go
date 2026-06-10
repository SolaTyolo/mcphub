package attachment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type LocalStore struct {
	base string
}

func NewLocalStore(base string) (*LocalStore, error) {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	return &LocalStore{base: base}, nil
}

func (s *LocalStore) Put(ctx context.Context, filename, contentType string, r io.Reader, size int64) (*Meta, error) {
	_ = ctx
	id := uuid.NewString()
	dir := filepath.Join(s.base, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	meta := &Meta{
		ID:          id,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		CreatedAt:   time.Now().UTC(),
	}

	contentPath := filepath.Join(dir, "content")
	f, err := os.Create(contentPath)
	if err != nil {
		return nil, err
	}
	written, err := io.Copy(f, r)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	meta.Size = written

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaBytes, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return meta, nil
}

func (s *LocalStore) Head(ctx context.Context, id string) (*Meta, error) {
	_ = ctx
	return s.loadMeta(id)
}

func (s *LocalStore) Get(ctx context.Context, id string) (io.ReadCloser, *Meta, error) {
	_ = ctx
	meta, err := s.loadMeta(id)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(filepath.Join(s.base, id, "content"))
	if err != nil {
		return nil, nil, err
	}
	return f, meta, nil
}

func (s *LocalStore) loadMeta(id string) (*Meta, error) {
	if id == "" || filepath.Base(id) != id {
		return nil, fmt.Errorf("invalid attachment id")
	}
	data, err := os.ReadFile(filepath.Join(s.base, id, "meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("attachment not found")
		}
		return nil, err
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
