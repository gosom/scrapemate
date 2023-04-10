package filecache

import (
	"context"

	"github.com/gosom/scrapemate"
)

var _ scrapemate.Cacher = (*FileCache)(nil)

// FileCache is a file cache
type FileCache struct {
	folder string
}

// NewFileCache creates a new file cache
func NewFileCache(folder string) *FileCache {
	return &FileCache{folder: folder}
}

// Get gets a value from the cache
func (c *FileCache) Get(ctx context.Context, key string) (scrapemate.Response, error) {
	panic("not implemented")
}

// Set sets a value to the cache
func (c *FileCache) Set(ctx context.Context, key string, value scrapemate.Response) error {
	panic("not implemented")
}
