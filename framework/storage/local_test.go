package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLocal(t *testing.T, baseURL string) *Local {
	t.Helper()
	disk, err := newLocal(LocalOptions{Root: t.TempDir(), BaseURL: baseURL})
	if err != nil {
		t.Fatal(err)
	}
	return disk
}

func TestLocalRoundTrip(t *testing.T) {
	ctx := context.Background()
	disk := testLocal(t, "")

	if err := disk.Put(ctx, "avatars/user-1.png", strings.NewReader("image-bytes")); err != nil {
		t.Fatal(err)
	}
	reader, err := disk.Get(ctx, "avatars/user-1.png")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(reader)
	reader.Close()
	if string(content) != "image-bytes" {
		t.Fatalf("content mismatch: %q", content)
	}
	if exists, _ := disk.Exists(ctx, "avatars/user-1.png"); !exists {
		t.Fatal("stored file reported missing")
	}
	if size, _ := disk.Size(ctx, "avatars/user-1.png"); size != int64(len("image-bytes")) {
		t.Fatalf("size = %d", size)
	}
	if err := disk.Delete(ctx, "avatars/user-1.png"); err != nil {
		t.Fatal(err)
	}
	if err := disk.Delete(ctx, "avatars/user-1.png"); err != nil {
		t.Fatal("delete must be idempotent")
	}
	if _, err := disk.Get(ctx, "avatars/user-1.png"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file error = %v", err)
	}
	if _, err := disk.Size(ctx, "avatars/user-1.png"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file size error = %v", err)
	}
}

func TestLocalRejectsTraversal(t *testing.T) {
	ctx := context.Background()
	disk := testLocal(t, "")
	for _, name := range []string{
		"", ".", "..", "../escape", "a/../../escape", "/etc/passwd",
		"a\\..\\..\\escape", "nul\x00byte", "nested/../../escape",
	} {
		t.Run(name, func(t *testing.T) {
			if err := disk.Put(ctx, name, strings.NewReader("x")); err == nil {
				t.Fatalf("traversal path %q accepted by Put", name)
			}
			if _, err := disk.Get(ctx, name); err == nil || errors.Is(err, ErrNotFound) {
				t.Fatalf("traversal path %q not rejected by Get: %v", name, err)
			}
		})
	}
	// Nothing may have escaped the root.
	if _, err := os.Stat(filepath.Join(filepath.Dir(disk.root), "escape")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a traversal path escaped the storage root")
	}
}

func TestLocalPutIsAtomic(t *testing.T) {
	ctx := context.Background()
	disk := testLocal(t, "")
	failing := io.MultiReader(strings.NewReader("partial"), &failingReader{})
	if err := disk.Put(ctx, "doc.txt", failing); err == nil {
		t.Fatal("failing reader accepted")
	}
	entries, err := os.ReadDir(disk.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".upload-") {
			t.Fatalf("temp file leaked: %s", entry.Name())
		}
		if entry.Name() == "doc.txt" {
			t.Fatal("partial file published")
		}
	}
}

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) { return 0, errors.New("stream broke") }

func TestLocalURL(t *testing.T) {
	ctx := context.Background()
	withURL := testLocal(t, "https://cdn.example.com/files/")
	url, err := withURL.URL(ctx, "avatars/user-1.png")
	if err != nil || url != "https://cdn.example.com/files/avatars/user-1.png" {
		t.Fatalf("url = %q err=%v", url, err)
	}
	withoutURL := testLocal(t, "")
	if _, err := withoutURL.URL(ctx, "file.txt"); !errors.Is(err, ErrURLUnavailable) {
		t.Fatalf("missing base URL error = %v", err)
	}
}
