package attachment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	cfg "github.com/SolaTyolo/mcphub/internal/config"
)

type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3Store(ctx context.Context, c cfg.S3Config) (*S3Store, error) {
	if c.Bucket == "" {
		return nil, fmt.Errorf("s3 attachment store requires bucket in DSN (s3://key:secret@host/bucket/prefix)")
	}
	if c.Endpoint == "" {
		return nil, fmt.Errorf("s3 attachment store requires endpoint host in DSN")
	}

	endpoint := c.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = c.PathStyle
	})

	store := &S3Store{client: client, bucket: c.Bucket, prefix: strings.Trim(c.Prefix, "/")}
	if err := store.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *S3Store) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	return err
}

func (s *S3Store) objectKey(id, suffix string) string {
	parts := []string{}
	if s.prefix != "" {
		parts = append(parts, s.prefix)
	}
	parts = append(parts, id, suffix)
	return strings.Join(parts, "/")
}

func (s *S3Store) Put(ctx context.Context, filename, contentType string, r io.Reader, size int64) (*Meta, error) {
	id := uuid.NewString()
	meta := &Meta{
		ID:          id,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		CreatedAt:   time.Now().UTC(),
	}

	contentKey := s.objectKey(id, "content")
	putIn := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(contentKey),
		Body:        r,
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"filename": filename,
		},
	}
	if _, err := s.client.PutObject(ctx, putIn); err != nil {
		return nil, err
	}

	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(contentKey),
	})
	if err == nil && head.ContentLength != nil {
		meta.Size = *head.ContentLength
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.objectKey(id, "meta.json")),
		Body:        strings.NewReader(string(metaBytes)),
		ContentType: aws.String("application/json"),
	}); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *S3Store) Head(ctx context.Context, id string) (*Meta, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(id, "meta.json")),
	})
	if err != nil {
		return nil, fmt.Errorf("attachment not found")
	}
	defer out.Body.Close()
	var meta Meta
	if err := json.NewDecoder(out.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *S3Store) Get(ctx context.Context, id string) (io.ReadCloser, *Meta, error) {
	meta, err := s.Head(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(id, "content")),
	})
	if err != nil {
		return nil, nil, err
	}
	return out.Body, meta, nil
}

func validateID(id string) error {
	if id == "" || strings.Contains(id, "/") {
		return fmt.Errorf("invalid attachment id")
	}
	return nil
}
