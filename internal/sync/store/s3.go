package store

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultS3Region = "us-east-1"

// S3Options configures an S3-compatible backup store.
type S3Options struct {
	Endpoint  string
	Region    string
	Bucket    string
	Prefix    string
	AccessKey string
	SecretKey string
}

type s3Store struct {
	client *minio.Client
	bucket string
	prefix string
}

// NewS3 creates a Store for S3-compatible object storage (path-style).
func NewS3(opts S3Options) (Store, error) {
	if strings.TrimSpace(opts.Endpoint) == "" {
		return nil, fmt.Errorf("s3 endpoint required")
	}
	if strings.TrimSpace(opts.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket required")
	}
	if strings.TrimSpace(opts.AccessKey) == "" {
		return nil, fmt.Errorf("s3 access key required")
	}
	if strings.TrimSpace(opts.SecretKey) == "" {
		return nil, fmt.Errorf("s3 secret key required")
	}

	endpoint, secure, err := parseS3Endpoint(opts.Endpoint)
	if err != nil {
		return nil, err
	}
	region := strings.TrimSpace(opts.Region)
	if region == "" {
		region = defaultS3Region
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure:       secure,
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}

	return &s3Store{
		client: client,
		bucket: opts.Bucket,
		prefix: NormalizePrefix(opts.Prefix),
	}, nil
}

func parseS3Endpoint(raw string) (host string, secure bool, err error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("invalid s3 endpoint: %w", err)
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("invalid s3 endpoint host")
	}
	secure = u.Scheme != "http"
	return u.Host, secure, nil
}

func (s *s3Store) objectKey(name string) string {
	return s.prefix + name
}

func (s *s3Store) listAll(ctx context.Context) ([]BackupInfo, error) {
	var backups []BackupInfo
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    s.prefix,
		Recursive: false,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("s3 list: %w", obj.Err)
		}
		name, err := NameFromKey(obj.Key, s.prefix)
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{
			Name:      name,
			FileSize:  obj.Size,
			CreatedAt: obj.LastModified,
		})
	}
	SortBackupsDesc(backups)
	return backups, nil
}

func (s *s3Store) HealthCheck(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("s3 health: %w", err)
	}
	if !exists {
		return fmt.Errorf("s3 bucket does not exist: %s", s.bucket)
	}
	return nil
}

func (s *s3Store) Upload(ctx context.Context, name string, r io.Reader, size int64) (*BackupInfo, error) {
	name, err := SanitizeName(name)
	if err != nil {
		return nil, err
	}
	info, err := s.client.PutObject(ctx, s.bucket, s.objectKey(name), r, size, minio.PutObjectOptions{
		ContentType: "application/zip",
	})
	if err != nil {
		return nil, fmt.Errorf("s3 upload: %w", err)
	}
	return &BackupInfo{
		Name:      name,
		FileSize:  info.Size,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (s *s3Store) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	name, err := SanitizeName(name)
	if err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, s.objectKey(name), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 download: %w", err)
	}
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, fmt.Errorf("s3 download: %w", err)
	}
	return obj, nil
}

func (s *s3Store) ListBackups(ctx context.Context, limit, offset int) ([]BackupInfo, error) {
	backups, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	return PaginateBackups(backups, limit, offset), nil
}

func (s *s3Store) DeleteBackup(ctx context.Context, name string) error {
	name, err := SanitizeName(name)
	if err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, s.objectKey(name), minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}
