package config

import (
	"net/url"
	"strings"
)

type AttachmentKind string

const (
	AttachmentLocal AttachmentKind = "local"
	AttachmentS3    AttachmentKind = "s3"
)

type AttachmentStoreConfig struct {
	Kind      AttachmentKind
	RawDSN    string
	LocalPath string
	S3        S3Config
}

type S3Config struct {
	Endpoint  string
	Bucket    string
	Prefix    string
	AccessKey string
	SecretKey string
	Region    string
	PathStyle bool
}

const defaultAttachmentDSN = "file://./data/attachments"

func ParseAttachmentDSN(raw string) AttachmentStoreConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = defaultAttachmentDSN
	}

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "file://") || strings.HasPrefix(lower, "file:") {
		path := extractAttachmentFilePath(raw)
		return AttachmentStoreConfig{Kind: AttachmentLocal, RawDSN: raw, LocalPath: path}
	}

	if strings.HasPrefix(lower, "s3://") {
		return parseS3DSN(raw)
	}

	return AttachmentStoreConfig{Kind: AttachmentLocal, RawDSN: raw, LocalPath: raw}
}

func parseS3DSN(raw string) AttachmentStoreConfig {
	u, err := url.Parse(raw)
	if err != nil {
		return AttachmentStoreConfig{Kind: AttachmentLocal, RawDSN: raw, LocalPath: "./data/attachments"}
	}

	cfg := AttachmentStoreConfig{
		Kind:   AttachmentS3,
		RawDSN: raw,
		S3: S3Config{
			Region:    u.Query().Get("region"),
			PathStyle: u.Query().Get("path_style") != "false",
		},
	}
	if cfg.S3.Region == "" {
		cfg.S3.Region = "us-east-1"
	}

	if u.User != nil {
		cfg.S3.AccessKey = u.User.Username()
		cfg.S3.SecretKey, _ = u.User.Password()
	}

	cfg.S3.Endpoint = "http://" + u.Host
	path := strings.Trim(u.Path, "/")
	if path != "" {
		parts := strings.SplitN(path, "/", 2)
		cfg.S3.Bucket = parts[0]
		if len(parts) == 2 {
			cfg.S3.Prefix = strings.TrimSuffix(parts[1], "/")
		}
	}
	return cfg
}

func extractAttachmentFilePath(raw string) string {
	if strings.HasPrefix(raw, "file://") {
		return raw[len("file://"):]
	}
	if strings.HasPrefix(raw, "file:") {
		return raw[len("file:"):]
	}
	return raw
}
