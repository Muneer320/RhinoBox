package cache

import (
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

// Cache implements a multi-level caching strategy:
// L1: In-memory LRU cache (fast, limited size)
// L2: Bloom filter for negative lookups (memory-efficient)
// L3: On-disk persistent cache using BadgerDB
type Cache struct {
	// L1 cache: In-memory LRU with expiration
	l1 *expirable.LRU[string, []byte]

	// L2 cache: Bloom filter for fast negative lookups
	bloom *bloom.BloomFilter
	mu    sync.RWMutex

	// L3 cache: Persistent on-disk storage
	l3 *badger.DB

	// Metrics
	hits   uint64
	misses uint64
}

// Config holds cache configuration parameters
type Config struct {
	L1Size      int           // Number of items in L1 cache
	L1TTL       time.Duration // Time-to-live for L1 entries
	BloomSize   uint          // Expected number of items in bloom filter
	BloomFPRate float64       // False positive rate (e.g., 0.01 for 1%)
	L3Path      string        // Path for BadgerDB storage
}

// DefaultConfig returns sensible defaults for the cache
func DefaultConfig() Config {
	return Config{
		L1Size:      10000,             // 10K items in memory
		L1TTL:       5 * time.Minute,   // 5 minute TTL
		BloomSize:   1000000,            // 1M expected items
		BloomFPRate: 0.01,               // 1% false positive rate
		L3Path:      "./data/cache",     // Default path
	}
}

// New creates a new multi-level cache
func New(cfg Config) (*Cache, error) {
	// Initialize L1 cache with LRU and expiration
	l1Cache := expirable.NewLRU[string, []byte](cfg.L1Size, nil, cfg.L1TTL)

	// Initialize L2 bloom filter
	bloomFilter := bloom.NewWithEstimates(cfg.BloomSize, cfg.BloomFPRate)

	// Initialize L3 BadgerDB
	opts := badger.DefaultOptions(cfg.L3Path)
	opts.Logger = nil // Disable verbose logging
	opts.SyncWrites = false // Async writes for better performance
	opts.NumVersionsToKeep = 1 // Keep only latest version

	l3DB, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &Cache{
		l1:    l1Cache,
		bloom: bloomFilter,
		l3:    l3DB,
	}, nil
}

// Get retrieves a value from the cache (checks L1 → L2 → L3)
func (c *Cache) Get(key string) ([]byte, bool) {
	// Try L1 cache first (fastest)
	if value, ok := c.l1.Get(key); ok {
		c.recordHit()
		return value, true
	}

	// Check L2 bloom filter for negative lookup
	c.mu.RLock()
	inBloom := c.bloom.Test([]byte(key))
	c.mu.RUnlock()

	if !inBloom {
		c.recordMiss()
		return nil, false // Definitely not in cache
	}

	// Try L3 persistent cache
	var value []byte
	err := c.l3.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})

	if err == nil {
		// Found in L3, promote to L1
		c.l1.Add(key, value)
		c.recordHit()
		return value, true
	}

	c.recordMiss()
	return nil, false
}

// Set stores a value in all cache levels
func (c *Cache) Set(key string, value []byte) error {
	// Add to L1 cache
	c.l1.Add(key, value)

	// Add to L2 bloom filter
	c.mu.Lock()
	c.bloom.Add([]byte(key))
	c.mu.Unlock()

	// Add to L3 persistent storage
	err := c.l3.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})

	return err
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) error {
	// Remove from L1
	c.l1.Remove(key)

	// Note: Bloom filter doesn't support deletion (by design)
	// It will eventually expire when filter is rebuilt

	// Remove from L3
	return c.l3.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Close closes the cache and flushes pending writes
func (c *Cache) Close() error {
	return c.l3.Close()
}

// Stats returns cache statistics
func (c *Cache) Stats() CacheStats {
	totalReqs := c.hits + c.misses
	hitRate := float64(0)
	if totalReqs > 0 {
		hitRate = float64(c.hits) / float64(totalReqs)
	}

	return CacheStats{
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: hitRate,
		L1Size:  c.l1.Len(),
	}
}

// CacheStats holds cache performance metrics
type CacheStats struct {
	Hits    uint64
	Misses  uint64
	HitRate float64
	L1Size  int
}

func (c *Cache) recordHit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
}

func (c *Cache) recordMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

// Clear removes all entries from L1 and resets bloom filter
func (c *Cache) Clear() error {
	c.l1.Purge()

	c.mu.Lock()
	c.bloom.ClearAll()
	c.mu.Unlock()

	// Drop all data from L3
	return c.l3.DropAll()
}
