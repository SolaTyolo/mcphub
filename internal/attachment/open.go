package attachment

import (
	"context"
	"fmt"

	"github.com/SolaTyolo/mcphub/internal/config"
)

func Open(ctx context.Context, cfg config.AttachmentStoreConfig) (Store, error) {
	switch cfg.Kind {
	case config.AttachmentS3:
		return NewS3Store(ctx, cfg.S3)
	case config.AttachmentLocal:
		path := cfg.LocalPath
		if path == "" {
			return nil, fmt.Errorf("local attachment store path is required")
		}
		return NewLocalStore(path)
	default:
		return nil, fmt.Errorf("unsupported attachment store %q", cfg.RawDSN)
	}
}
