package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Local stores files under a confined root directory.
type Local struct {
	root    string
	baseURL string
}

func newLocal(options LocalOptions) (*Local, error) {
	root := options.Root
	if root == "" {
		root = "./storage"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create root: %w", err)
	}
	return &Local{root: absolute, baseURL: strings.TrimSuffix(options.BaseURL, "/")}, nil
}

// resolve confines name to the disk root. Names are slash-separated relative
// paths; anything absolute, escaping, or malformed is rejected. Symlinks
// inside the root are followed — keep untrusted content roots free of them.
func (l *Local) resolve(name string) (string, string, error) {
	if name == "" || strings.ContainsAny(name, "\x00\\") {
		return "", "", fmt.Errorf("storage: invalid path %q", name)
	}
	if path.IsAbs(name) {
		return "", "", fmt.Errorf("storage: path %q must be relative", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", fmt.Errorf("storage: path %q escapes the storage root", name)
	}
	full := filepath.Join(l.root, filepath.FromSlash(cleaned))
	if full != l.root && !strings.HasPrefix(full, l.root+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("storage: path %q escapes the storage root", name)
	}
	return full, cleaned, nil
}

// Put writes atomically: the content lands in a temp file that is renamed
// into place, so readers never observe partial writes.
func (l *Local) Put(_ context.Context, name string, r io.Reader, _ ...PutOption) error {
	full, _, err := l.resolve(name)
	if err != nil {
		return err
	}
	parent := filepath.Dir(full)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("storage: create directory: %w", err)
	}
	staging, err := os.CreateTemp(parent, ".upload-*")
	if err != nil {
		return fmt.Errorf("storage: stage file: %w", err)
	}
	if _, err := io.Copy(staging, r); err != nil {
		staging.Close()
		os.Remove(staging.Name())
		return fmt.Errorf("storage: write file: %w", err)
	}
	if err := staging.Close(); err != nil {
		os.Remove(staging.Name())
		return fmt.Errorf("storage: close staged file: %w", err)
	}
	if err := os.Rename(staging.Name(), full); err != nil {
		os.Remove(staging.Name())
		return fmt.Errorf("storage: publish file: %w", err)
	}
	return nil
}

func (l *Local) Get(_ context.Context, name string) (io.ReadCloser, error) {
	full, _, err := l.resolve(name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return file, err
}

func (l *Local) Exists(_ context.Context, name string) (bool, error) {
	full, _, err := l.resolve(name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(full)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (l *Local) Size(_ context.Context, name string) (int64, error) {
	full, _, err := l.resolve(name)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(full)
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (l *Local) Delete(_ context.Context, name string) error {
	full, _, err := l.resolve(name)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (l *Local) URL(_ context.Context, name string) (string, error) {
	_, cleaned, err := l.resolve(name)
	if err != nil {
		return "", err
	}
	if l.baseURL == "" {
		return "", ErrURLUnavailable
	}
	return l.baseURL + "/" + cleaned, nil
}
