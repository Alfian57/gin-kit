package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewS3ValidatesOptions(t *testing.T) {
	for _, test := range []struct {
		name    string
		options S3Options
	}{
		{"missing endpoint", S3Options{Bucket: "b", AccessKey: "a", SecretKey: "s"}},
		{"missing bucket", S3Options{Endpoint: "s3.example.com", AccessKey: "a", SecretKey: "s"}},
		{"missing credentials", S3Options{Endpoint: "s3.example.com", Bucket: "b"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newS3(test.options); err == nil {
				t.Fatal("expected constructor error")
			}
		})
	}
}

func TestNewRejectsUnknownDriver(t *testing.T) {
	if _, err := New(Options{Driver: "ftp"}); err == nil {
		t.Fatal("unknown driver accepted")
	}
}

func TestS3PublicBaseURLBypassesPresigning(t *testing.T) {
	disk, err := newS3(S3Options{
		Endpoint: "s3.example.com", Bucket: "assets",
		AccessKey: "key", SecretKey: "secret",
		PublicBaseURL: "https://cdn.example.com/",
	})
	if err != nil {
		t.Fatal(err)
	}
	url, err := disk.URL(context.Background(), "avatars/user-1.png")
	if err != nil || url != "https://cdn.example.com/avatars/user-1.png" {
		t.Fatalf("url = %q err=%v", url, err)
	}
}

func TestS3PresignedURLUsesConfiguredTTL(t *testing.T) {
	disk, err := newS3(S3Options{
		Endpoint: "s3.example.com", Bucket: "assets", Region: "us-east-1",
		AccessKey: "key", SecretKey: "secret",
		UsePathStyle: true, PresignTTL: 30 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	url, err := disk.URL(context.Background(), "doc.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "X-Amz-Expires=1800") || !strings.Contains(url, "doc.pdf") {
		t.Fatalf("presigned url unexpected: %s", url)
	}
}

func TestS3DefaultsApplied(t *testing.T) {
	disk, err := newS3(S3Options{
		Endpoint: "s3.example.com", Bucket: "assets",
		AccessKey: "key", SecretKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if disk.presignTTL != 15*time.Minute {
		t.Fatalf("presign TTL default = %s", disk.presignTTL)
	}
}
