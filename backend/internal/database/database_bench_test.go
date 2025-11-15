package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// Test helpers
func getTestPostgresURL() string {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		url = "postgres://postgres:test@localhost:5432/rhinobox_test?sslmode=disable"
	}
	return url
}

func getTestMongoURL() string {
	url := os.Getenv("TEST_MONGO_URL")
	if url == "" {
		url = "mongodb://localhost:27017"
	}
	return url
}

// BenchmarkPostgresInsert measures raw INSERT performance
// Target: 100K inserts/sec
func BenchmarkPostgresInsert(b *testing.B) {
	url := getTestPostgresURL()
	ctx := context.Background()
	
	db, err := NewPostgresDB(ctx, url)
	if err != nil {
		b.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()
	
	// Create test table
	table := fmt.Sprintf("bench_insert_%d", time.Now().Unix())
	schema := map[string]string{
		"id":    "BIGINT",
		"name":  "TEXT",
		"value": "DOUBLE PRECISION",
	}
	
	if err := db.CreateTableFromSchema(ctx, table, schema); err != nil {
		b.Fatalf("create table: %v", err)
	}
	
	// Prepare test data
	doc := map[string]any{
		"id":    int64(1),
		"name":  "test",
		"value": 123.45,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		doc["id"] = int64(i + 1)
		docs := []map[string]any{doc}
		if err := db.multiInsert(ctx, table, docs); err != nil {
			b.Fatalf("insert failed: %v", err)
		}
	}
	
	b.StopTimer()
	
	// Cleanup
	_ = db.ExecuteSQL(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %q", table))
}

// BenchmarkPostgresBatchInsert measures batch INSERT performance with various batch sizes
func BenchmarkPostgresBatchInsert(b *testing.B) {
	url := getTestPostgresURL()
	ctx := context.Background()
	
	db, err := NewPostgresDB(ctx, url)
	if err != nil {
		b.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()
	
	batchSizes := []int{10, 100, 1000}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", batchSize), func(b *testing.B) {
			table := fmt.Sprintf("bench_batch_%d_%d", batchSize, time.Now().Unix())
			schema := map[string]string{
				"id":    "BIGINT",
				"name":  "TEXT",
				"value": "DOUBLE PRECISION",
			}
			
			if err := db.CreateTableFromSchema(ctx, table, schema); err != nil {
				b.Fatalf("create table: %v", err)
			}
			
			// Prepare batch
			docs := make([]map[string]any, batchSize)
			for i := 0; i < batchSize; i++ {
				docs[i] = map[string]any{
					"id":    int64(i + 1),
					"name":  fmt.Sprintf("test_%d", i),
					"value": float64(i) * 123.45,
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				if err := db.multiInsert(ctx, table, docs); err != nil {
					b.Fatalf("batch insert failed: %v", err)
				}
			}
			
			b.StopTimer()
			
			// Report throughput
			insertsPerOp := int64(batchSize)
			totalInserts := int64(b.N) * insertsPerOp
			elapsedSeconds := b.Elapsed().Seconds()
			insertsPerSecond := float64(totalInserts) / elapsedSeconds
			
			b.ReportMetric(insertsPerSecond, "inserts/sec")
			
			// Cleanup
			_ = db.ExecuteSQL(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %q", table))
		})
	}
}

// BenchmarkPostgresCopyInsert measures COPY protocol performance (fastest method)
// This is the key to achieving 100K+ inserts/sec
func BenchmarkPostgresCopyInsert(b *testing.B) {
	url := getTestPostgresURL()
	ctx := context.Background()
	
	db, err := NewPostgresDB(ctx, url)
	if err != nil {
		b.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()
	
	batchSizes := []int{100, 1000, 10000}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("copy_%d", batchSize), func(b *testing.B) {
			table := fmt.Sprintf("bench_copy_%d_%d", batchSize, time.Now().Unix())
			schema := map[string]string{
				"id":    "BIGINT",
				"name":  "TEXT",
				"value": "DOUBLE PRECISION",
			}
			
			if err := db.CreateTableFromSchema(ctx, table, schema); err != nil {
				b.Fatalf("create table: %v", err)
			}
			
			// Prepare batch
			docs := make([]map[string]any, batchSize)
			for i := 0; i < batchSize; i++ {
				docs[i] = map[string]any{
					"id":    int64(i + 1),
					"name":  fmt.Sprintf("test_%d", i),
					"value": float64(i) * 123.45,
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				if err := db.copyInsert(ctx, table, docs); err != nil {
					b.Fatalf("copy insert failed: %v", err)
				}
			}
			
			b.StopTimer()
			
			// Report throughput
			insertsPerOp := int64(batchSize)
			totalInserts := int64(b.N) * insertsPerOp
			elapsedSeconds := b.Elapsed().Seconds()
			insertsPerSecond := float64(totalInserts) / elapsedSeconds
			
			b.ReportMetric(insertsPerSecond, "inserts/sec")
			
			// Cleanup
			_ = db.ExecuteSQL(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %q", table))
		})
	}
}

// BenchmarkMongoInsert measures MongoDB single insert performance
func BenchmarkMongoInsert(b *testing.B) {
	url := getTestMongoURL()
	ctx := context.Background()
	
	db, err := NewMongoDB(ctx, url)
	if err != nil {
		b.Skipf("MongoDB not available: %v", err)
	}
	defer db.Close(ctx)
	
	dbName := "rhinobox_test"
	collection := fmt.Sprintf("bench_insert_%d", time.Now().Unix())
	
	doc := map[string]any{
		"id":    1,
		"name":  "test",
		"value": 123.45,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		doc["id"] = i + 1
		if err := db.InsertOne(ctx, dbName, collection, doc); err != nil {
			b.Fatalf("insert failed: %v", err)
		}
	}
	
	b.StopTimer()
	
	// Cleanup
	_ = db.DropCollection(ctx, dbName, collection)
}

// BenchmarkMongoBulkInsert measures MongoDB bulk insert performance
// Target: 200K inserts/sec
func BenchmarkMongoBulkInsert(b *testing.B) {
	url := getTestMongoURL()
	ctx := context.Background()
	
	db, err := NewMongoDB(ctx, url)
	if err != nil {
		b.Skipf("MongoDB not available: %v", err)
	}
	defer db.Close(ctx)
	
	dbName := "rhinobox_test"
	batchSizes := []int{100, 1000, 10000}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("bulk_%d", batchSize), func(b *testing.B) {
			collection := fmt.Sprintf("bench_bulk_%d_%d", batchSize, time.Now().Unix())
			
			// Prepare batch
			docs := make([]map[string]any, batchSize)
			for i := 0; i < batchSize; i++ {
				docs[i] = map[string]any{
					"id":    i + 1,
					"name":  fmt.Sprintf("test_%d", i),
					"value": float64(i) * 123.45,
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				if err := db.BulkInsert(ctx, dbName, collection, docs); err != nil {
					b.Fatalf("bulk insert failed: %v", err)
				}
			}
			
			b.StopTimer()
			
			// Report throughput
			insertsPerOp := int64(batchSize)
			totalInserts := int64(b.N) * insertsPerOp
			elapsedSeconds := b.Elapsed().Seconds()
			insertsPerSecond := float64(totalInserts) / elapsedSeconds
			
			b.ReportMetric(insertsPerSecond, "inserts/sec")
			
			// Cleanup
			_ = db.DropCollection(ctx, dbName, collection)
		})
	}
}

// BenchmarkPostgresConnectionAcquisition measures connection pool acquisition time
// Target: <1ms
func BenchmarkPostgresConnectionAcquisition(b *testing.B) {
	url := getTestPostgresURL()
	ctx := context.Background()
	
	db, err := NewPostgresDB(ctx, url)
	if err != nil {
		b.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		conn, err := db.pool.Acquire(ctx)
		if err != nil {
			b.Fatalf("acquire failed: %v", err)
		}
		conn.Release()
	}
}

// BenchmarkPostgresParallelInserts measures concurrent insert performance
func BenchmarkPostgresParallelInserts(b *testing.B) {
	url := getTestPostgresURL()
	ctx := context.Background()
	
	db, err := NewPostgresDB(ctx, url)
	if err != nil {
		b.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()
	
	table := fmt.Sprintf("bench_parallel_%d", time.Now().Unix())
	schema := map[string]string{
		"id":    "BIGINT",
		"name":  "TEXT",
		"value": "DOUBLE PRECISION",
	}
	
	if err := db.CreateTableFromSchema(ctx, table, schema); err != nil {
		b.Fatalf("create table: %v", err)
	}
	
	docs := make([]map[string]any, 100)
	for i := 0; i < 100; i++ {
		docs[i] = map[string]any{
			"id":    int64(i + 1),
			"name":  fmt.Sprintf("test_%d", i),
			"value": float64(i) * 123.45,
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := db.multiInsert(ctx, table, docs); err != nil {
				b.Fatalf("parallel insert failed: %v", err)
			}
		}
	})
	
	b.StopTimer()
	
	// Cleanup
	_ = db.ExecuteSQL(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %q", table))
}

// BenchmarkMongoParallelInserts measures concurrent MongoDB insert performance
func BenchmarkMongoParallelInserts(b *testing.B) {
	url := getTestMongoURL()
	ctx := context.Background()
	
	db, err := NewMongoDB(ctx, url)
	if err != nil {
		b.Skipf("MongoDB not available: %v", err)
	}
	defer db.Close(ctx)
	
	dbName := "rhinobox_test"
	collection := fmt.Sprintf("bench_parallel_%d", time.Now().Unix())
	
	docs := make([]map[string]any, 100)
	for i := 0; i < 100; i++ {
		docs[i] = map[string]any{
			"id":    i + 1,
			"name":  fmt.Sprintf("test_%d", i),
			"value": float64(i) * 123.45,
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := db.BulkInsert(ctx, dbName, collection, docs); err != nil {
				b.Fatalf("parallel insert failed: %v", err)
			}
		}
	})
	
	b.StopTimer()
	
	// Cleanup
	_ = db.DropCollection(ctx, dbName, collection)
}
