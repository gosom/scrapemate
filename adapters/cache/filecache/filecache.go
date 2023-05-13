package filecache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/cache"
)

var _ scrapemate.Cacher = (*FileCache)(nil)

// FileCache is a file cache
type FileCache struct {
	folder string
}

// NewFileCache creates a new file cache
func NewFileCache(folder string) (*FileCache, error) {
	const permissions = 0o777
	if err := os.MkdirAll(folder, permissions); err != nil {
		return nil, fmt.Errorf("cannot create cache dir %w", err)
	}

	return &FileCache{folder: folder}, nil
}

// Get gets a value from the cache
func (c *FileCache) Get(_ context.Context, key string) (scrapemate.Response, error) {
	file := filepath.Join(c.folder, key)

	f, err := os.Open(file)
	if err != nil {
		return scrapemate.Response{}, fmt.Errorf("cannot open file %s: %w", file, err)
	}

	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return scrapemate.Response{}, fmt.Errorf("cannot read file %s: %w", file, err)
	}

	decompressed, err := cache.Decompress(data)
	if err != nil {
		return scrapemate.Response{}, fmt.Errorf("cannot decompress file %s: %w", file, err)
	}

	var response scrapemate.Response

	if err := json.Unmarshal(decompressed, &response); err != nil {
		return scrapemate.Response{}, fmt.Errorf("cannot unmarshal file %s: %w", file, err)
	}

	return response, nil
}

// Set sets a value to the cache
func (c *FileCache) Set(_ context.Context, key string, value *scrapemate.Response) error {
	f, err := os.Create(filepath.Join(c.folder, key))
	if err != nil {
		return fmt.Errorf("cannot create file %w", err)
	}

	defer f.Close()

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cannot marshal response %w", err)
	}

	compressed, err := cache.Compress(data)
	if err != nil {
		return fmt.Errorf("cannot compress data %w", err)
	}

	if _, err := f.Write(compressed); err != nil {
		return fmt.Errorf("cannot write to file %w", err)
	}

	return nil
}

// Close closes the file cache
func (c *FileCache) Close() error {
	return nil
}
