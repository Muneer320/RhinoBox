package cache

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func setupTestCache(b *testing.B) *Cache {
	b.Helper()
	cfg := Config{
		L1Size:      1000,
		L1TTL:       5 * time.Minute,
		BloomSize:   10000,
		BloomFPRate: 0.01,
		L3Path:      b.TempDir(),
	}
	c, err := New(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	b.Cleanup(func() { c.Close() })
	return c
}

// BenchmarkCacheSet measures cache write performance
func BenchmarkCacheSet(b *testing.B) {
	c := setupTestCache(b)
	data := make([]byte, 1024) // 1KB
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		if err := c.Set(key, data); err != nil {
			b.Fatalf("set failed: %v", err)
		}
	}
}

// BenchmarkCacheGet measures cache read performance
func BenchmarkCacheGet(b *testing.B) {
	c := setupTestCache(b)
	data := make([]byte, 1024) // 1KB
	rand.Read(data)

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		c.Set(key, data)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		_, _ = c.Get(key)
	}
}

// BenchmarkCacheGetMiss measures performance on cache misses
func BenchmarkCacheGetMiss(b *testing.B) {
	c := setupTestCache(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("nonexistent_%d", i)
		_, _ = c.Get(key)
	}
}

// BenchmarkCacheMixed measures realistic mixed workload
func BenchmarkCacheMixed(b *testing.B) {
	c := setupTestCache(b)
	data := make([]byte, 1024)
	rand.Read(data)

	// 80% reads, 20% writes
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", rand.Intn(10000))
		if rand.Float64() < 0.8 {
			c.Get(key)
		} else {
			c.Set(key, data)
		}
	}
}

// BenchmarkHashCompute measures hash computation performance
func BenchmarkHashCompute(b *testing.B) {
	sizes := []int{1024, 10240, 102400, 1048576} // 1KB, 10KB, 100KB, 1MB

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%dKB", size/1024), func(b *testing.B) {
			data := make([]byte, size)
			rand.Read(data)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = ComputeHash(data)
			}
		})
	}
}

// BenchmarkDeduplication measures deduplication performance
func BenchmarkDeduplication(b *testing.B) {
	c := setupTestCache(b)
	h := NewHashIndex(c)

	data := make([]byte, 10240) // 10KB
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, _ = h.GetOrCompute(data)
	}
}

// TestCacheHitRate verifies cache hit rate meets target (95%+)
func TestCacheHitRate(t *testing.T) {
	cfg := Config{
		L1Size:      100,
		L1TTL:       5 * time.Minute,
		BloomSize:   1000,
		BloomFPRate: 0.01,
		L3Path:      t.TempDir(),
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	// Populate cache with 100 items
	data := make([]byte, 1024)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		rand.Read(data)
		if err := c.Set(key, data); err != nil {
			t.Fatalf("set failed: %v", err)
		}
	}

	// Simulate realistic access pattern (Zipf distribution)
	// 80% of requests hit 20% of keys
	requests := 10000
	hotKeys := 20 // 20% of 100

	for i := 0; i < requests; i++ {
		var key string
		if rand.Float64() < 0.8 {
			// Hot key (80% of traffic)
			key = fmt.Sprintf("key_%d", rand.Intn(hotKeys))
		} else {
			// Cold key (20% of traffic)
			key = fmt.Sprintf("key_%d", hotKeys+rand.Intn(80))
		}
		c.Get(key)
	}

	stats := c.Stats()
	t.Logf("Cache Stats: Hits=%d, Misses=%d, HitRate=%.2f%%", 
		stats.Hits, stats.Misses, stats.HitRate*100)

	if stats.HitRate < 0.95 {
		t.Errorf("Cache hit rate %.2f%% below target 95%%", stats.HitRate*100)
	}
}

// TestCacheConcurrency verifies thread-safe operation
func TestCacheConcurrency(t *testing.T) {
	cfg := Config{
		L1Size:      1000,
		L1TTL:       5 * time.Minute,
		BloomSize:   10000,
		BloomFPRate: 0.01,
		L3Path:      t.TempDir(),
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	// Spawn 10 concurrent goroutines
	done := make(chan bool)
	for g := 0; g < 10; g++ {
		go func(id int) {
			data := make([]byte, 1024)
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key_%d_%d", id, i)
				rand.Read(data)
				c.Set(key, data)
				c.Get(key)
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines
	for g := 0; g < 10; g++ {
		<-done
	}

	stats := c.Stats()
	t.Logf("Concurrent ops: Hits=%d, Misses=%d, HitRate=%.2f%%",
		stats.Hits, stats.Misses, stats.HitRate*100)
}

// TestBloomFilterAccuracy verifies false positive rate
func TestBloomFilterAccuracy(t *testing.T) {
	cfg := Config{
		L1Size:      10,
		L1TTL:       5 * time.Minute,
		BloomSize:   1000,
		BloomFPRate: 0.01,
		L3Path:      t.TempDir(),
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	// Insert 1000 items
	data := make([]byte, 100)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		rand.Read(data)
		c.Set(key, data)
	}

	// Test for false positives with non-existent keys
	falsePositives := 0
	tests := 10000
	for i := 0; i < tests; i++ {
		key := fmt.Sprintf("nonexistent_%d", i)
		_, found := c.Get(key)
		if found {
			falsePositives++
		}
	}

	fpRate := float64(falsePositives) / float64(tests)
	t.Logf("False positive rate: %.4f%% (target: 1%%)", fpRate*100)

	if fpRate > 0.02 { // Allow 2% tolerance
		t.Errorf("False positive rate %.4f%% exceeds 2%% threshold", fpRate*100)
	}
}

// BenchmarkCacheThroughput measures operations per second
func BenchmarkCacheThroughput(b *testing.B) {
	c := setupTestCache(b)
	data := make([]byte, 1024)
	rand.Read(data)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		c.Set(key, data)
	}

	b.ResetTimer()

	start := time.Now()
	ops := 0

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", rand.Intn(1000))
		c.Get(key)
		ops++
	}

	elapsed := time.Since(start)
	opsPerSec := float64(ops) / elapsed.Seconds()

	b.ReportMetric(opsPerSec, "ops/sec")
	
	if opsPerSec < 1000000 { // Target: 1M ops/sec
		b.Logf("WARNING: Throughput %.0f ops/sec below 1M target", opsPerSec)
	}
}

// Example usage demonstrating cache API
func ExampleCache() {
	// Create temporary directory for test
	tmpDir, _ := os.MkdirTemp("", "cache_example")
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		L1Size:      100,
		L1TTL:       5 * time.Minute,
		BloomSize:   1000,
		BloomFPRate: 0.01,
		L3Path:      tmpDir,
	}

	c, _ := New(cfg)
	defer c.Close()

	// Store data
	c.Set("user:123", []byte(`{"name":"Alice"}`))

	// Retrieve data
	if data, found := c.Get("user:123"); found {
		fmt.Printf("Found: %s\n", string(data))
	}

	// Check stats
	stats := c.Stats()
	fmt.Printf("Hit rate: %.0f%%\n", stats.HitRate*100)

	// Output:
	// Found: {"name":"Alice"}
	// Hit rate: 100%
}
