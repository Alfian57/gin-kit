// Package storage provides a Laravel-style disk abstraction with a
// path-confined local driver and an S3-compatible driver.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	// ErrNotFound is returned by every driver for missing files.
	ErrNotFound = errors.New("storage: file not found")
	// ErrURLUnavailable is returned by URL when no public base URL is
	// configured for the disk.
	ErrURLUnavailable = errors.New("storage: no public URL configured")
)

// Disk stores and retrieves files under slash-separated paths.
type Disk interface {
	Put(ctx context.Context, path string, r io.Reader, opts ...PutOption) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Exists(ctx context.Context, path string) (bool, error)
	Size(ctx context.Context, path string) (int64, error)
	// Delete removes the file and is a no-op for missing paths.
	Delete(ctx context.Context, path string) error
	// URL returns a public or presigned URL for the file.
	URL(ctx context.Context, path string) (string, error)
}

type PutOptions struct {
	ContentType string
	// Size is the number of bytes when known, or -1 to stream.
	Size int64
}

type PutOption func(*PutOptions)

func WithContentType(contentType string) PutOption {
	return func(o *PutOptions) { o.ContentType = contentType }
}

func WithSize(size int64) PutOption {
	return func(o *PutOptions) { o.Size = size }
}

func applyPutOptions(opts []PutOption) PutOptions {
	options := PutOptions{Size: -1}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

type LocalOptions struct {
	// Root is the directory that confines every file, defaulting to
	// ./storage. It is created when missing.
	Root string
	// BaseURL makes URL() return BaseURL + "/" + path when set.
	BaseURL string
}

type S3Options struct {
	Endpoint  string // host[:port] without scheme, e.g. s3.amazonaws.com
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	// UseSSL defaults to true.
	UseSSL *bool
	// UsePathStyle addresses buckets as endpoint/bucket (MinIO style).
	UsePathStyle bool
	// PresignTTL bounds presigned URLs, defaulting to 15 minutes.
	PresignTTL time.Duration
	// PublicBaseURL makes URL() return a plain public URL instead of a
	// presigned one.
	PublicBaseURL string
}

type Options struct {
	// Driver selects the disk: "local" (default) or "s3".
	Driver string
	Local  LocalOptions
	S3     S3Options
}

// New builds a Disk for the configured driver.
func New(options Options) (Disk, error) {
	switch options.Driver {
	case "", "local":
		return newLocal(options.Local)
	case "s3":
		return newS3(options.S3)
	default:
		return nil, fmt.Errorf("storage: unknown driver %q", options.Driver)
	}
}
