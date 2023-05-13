package leveldbcache

import (
	"context"
	"encoding/json"

	"github.com/gosom/scrapemate"
	"github.com/syndtr/goleveldb/leveldb"
)

var _ scrapemate.Cacher = (*LevelDBCache)(nil)

// LevelDBCache is a cache that uses LevelDB as a backend.
type LevelDBCache struct {
	db *leveldb.DB
}

// NewLevelDBCache creates a new LevelDBCache.
func NewLevelDBCache(path string) (*LevelDBCache, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	return &LevelDBCache{db: db}, nil
}

// Get gets a value from the cache.
func (c *LevelDBCache) Get(_ context.Context, key string) (scrapemate.Response, error) {
	data, err := c.db.Get([]byte(key), nil)
	if err != nil {
		return scrapemate.Response{}, err
	}

	var response scrapemate.Response
	if err := json.Unmarshal(data, &response); err != nil {
		return scrapemate.Response{}, err
	}

	return response, nil
}

// Set sets a value to the cache.
func (c *LevelDBCache) Set(_ context.Context, key string, value *scrapemate.Response) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.db.Put([]byte(key), data, nil)
}

// Close closes the LevelDBCache.
func (c *LevelDBCache) Close() error {
	return c.db.Close()
}
