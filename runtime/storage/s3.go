package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3 stores files in any S3-compatible object store (AWS S3, MinIO,
// Cloudflare R2, DigitalOcean Spaces).
type S3 struct {
	client        *minio.Client
	bucket        string
	presignTTL    time.Duration
	publicBaseURL string
}

func newS3(options S3Options) (*S3, error) {
	if options.Endpoint == "" || options.Bucket == "" {
		return nil, errors.New("storage: the s3 driver requires an endpoint and a bucket")
	}
	if options.AccessKey == "" || options.SecretKey == "" {
		return nil, errors.New("storage: the s3 driver requires access credentials")
	}
	useSSL := true
	if options.UseSSL != nil {
		useSSL = *options.UseSSL
	}
	lookup := minio.BucketLookupAuto
	if options.UsePathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(options.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(options.AccessKey, options.SecretKey, ""),
		Secure:       useSSL,
		Region:       options.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 client: %w", err)
	}
	presignTTL := options.PresignTTL
	if presignTTL <= 0 {
		presignTTL = 15 * time.Minute
	}
	return &S3{
		client:        client,
		bucket:        options.Bucket,
		presignTTL:    presignTTL,
		publicBaseURL: strings.TrimSuffix(options.PublicBaseURL, "/"),
	}, nil
}

func (s *S3) Put(ctx context.Context, name string, r io.Reader, opts ...PutOption) error {
	options := applyPutOptions(opts)
	_, err := s.client.PutObject(ctx, s.bucket, name, r, options.Size, minio.PutObjectOptions{
		ContentType: options.ContentType,
	})
	return err
}

func (s *S3) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, mapS3Error(err)
	}
	// GetObject is lazy; Stat surfaces missing keys immediately.
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, mapS3Error(err)
	}
	return object, nil
}

func (s *S3) Exists(ctx context.Context, name string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, name, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	if errors.Is(mapS3Error(err), ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (s *S3) Size(ctx context.Context, name string) (int64, error) {
	info, err := s.client.StatObject(ctx, s.bucket, name, minio.StatObjectOptions{})
	if err != nil {
		return 0, mapS3Error(err)
	}
	return info.Size, nil
}

func (s *S3) Delete(ctx context.Context, name string) error {
	return s.client.RemoveObject(ctx, s.bucket, name, minio.RemoveObjectOptions{})
}

func (s *S3) URL(ctx context.Context, name string) (string, error) {
	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/" + strings.TrimPrefix(name, "/"), nil
	}
	presigned, err := s.client.PresignedGetObject(ctx, s.bucket, name, s.presignTTL, nil)
	if err != nil {
		return "", err
	}
	return presigned.String(), nil
}

func mapS3Error(err error) error {
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.StatusCode == 404 {
		return ErrNotFound
	}
	return err
}
