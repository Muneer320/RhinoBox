package stress_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// ListingMetrics holds performance measurements for listing endpoint
type ListingMetrics struct {
	TotalQueries     int
	TotalDuration    time.Duration
	AvgLatency       time.Duration
	MinLatency       time.Duration
	MaxLatency       time.Duration
	P50Latency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	Throughput       float64 // queries/sec
	SuccessCount     int
	ErrorCount       int
	Latencies        []time.Duration
}

// calculatePercentiles calculates P50, P95, P99 from sorted latencies
func calculatePercentiles(latencies []time.Duration) (p50, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	p50Idx := int(float64(len(sorted)) * 0.50)
	p95Idx := int(float64(len(sorted)) * 0.95)
	p99Idx := int(float64(len(sorted)) * 0.99)

	if p50Idx >= len(sorted) {
		p50Idx = len(sorted) - 1
	}
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}

	return sorted[p50Idx], sorted[p95Idx], sorted[p99Idx]
}

// TestListingPerformance measures listing endpoint latency and throughput
func TestListingPerformance(t *testing.T) {
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload test files for listing
	const numFiles = 200
	extensions := []string{"pdf", "jpg", "png", "txt", "doc", "mp4", "mp3"}
	categories := []string{"documents", "images", "videos", "audio"}

	for i := 0; i < numFiles; i++ {
		ext := extensions[i%len(extensions)]
		category := categories[i%len(categories)]
		content := fmt.Sprintf("test file content %d unique data %d", i, time.Now().UnixNano())
		filename := fmt.Sprintf("test_file_%d.%s", i, ext)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(content))
		writer.WriteField("category", category)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}
	}

	t.Logf("Uploaded %d files for performance testing", numFiles)
	time.Sleep(100 * time.Millisecond) // Allow indexing to complete

	// Test single listing query performance
	t.Run("single_query_latency", func(t *testing.T) {
		const iterations = 200
		latencies := make([]time.Duration, 0, iterations)
		var totalDuration time.Duration
		var minDuration, maxDuration time.Duration = time.Hour, 0
		successCount := 0

		for i := 0; i < iterations; i++ {
			start := time.Now()
			req := httptest.NewRequest(http.MethodGet, "/files?limit=50", nil)
			resp := httptest.NewRecorder()
			srv.Router().ServeHTTP(resp, req)

			duration := time.Since(start)
			latencies = append(latencies, duration)
			totalDuration += duration

			if duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}

			if resp.Code == http.StatusOK {
				successCount++
			} else {
				t.Logf("Request failed with status %d: %s", resp.Code, resp.Body.String())
			}
		}

		p50, p95, p99 := calculatePercentiles(latencies)
		avgDuration := totalDuration / iterations
		throughput := float64(iterations) / totalDuration.Seconds()

		metrics := ListingMetrics{
			TotalQueries:  iterations,
			TotalDuration: totalDuration,
			AvgLatency:    avgDuration,
			MinLatency:    minDuration,
			MaxLatency:    maxDuration,
			P50Latency:    p50,
			P95Latency:    p95,
			P99Latency:    p99,
			Throughput:    throughput,
			SuccessCount:  successCount,
			ErrorCount:    iterations - successCount,
			Latencies:     latencies,
		}

		t.Logf("📊 Single Query Performance (%d iterations):", iterations)
		t.Logf("  Average Latency: %v", metrics.AvgLatency)
		t.Logf("  Min Latency: %v", metrics.MinLatency)
		t.Logf("  Max Latency: %v", metrics.MaxLatency)
		t.Logf("  P50 Latency: %v", metrics.P50Latency)
		t.Logf("  P95 Latency: %v", metrics.P95Latency)
		t.Logf("  P99 Latency: %v", metrics.P99Latency)
		t.Logf("  Throughput: %.2f queries/sec", metrics.Throughput)
		t.Logf("  Success Rate: %d/%d (%.1f%%)", metrics.SuccessCount, metrics.TotalQueries,
			float64(metrics.SuccessCount)/float64(metrics.TotalQueries)*100)

		// Performance assertions
		if metrics.AvgLatency > 50*time.Millisecond {
			t.Errorf("average latency too high: %v (expected < 50ms)", metrics.AvgLatency)
		}
		if metrics.P95Latency > 200*time.Millisecond {
			t.Errorf("P95 latency too high: %v (expected < 200ms)", metrics.P95Latency)
		}
		if metrics.SuccessCount < iterations*99/100 {
			t.Errorf("success rate too low: %d/%d (expected >= 99%%)", metrics.SuccessCount, metrics.TotalQueries)
		}
	})

	// Test concurrent listing queries
	t.Run("concurrent_queries", func(t *testing.T) {
		const numConcurrent = 50
		const queriesPerGoroutine = 10

		var wg sync.WaitGroup
		var mu sync.Mutex
		latencies := make([]time.Duration, 0, numConcurrent*queriesPerGoroutine)
		errorCount := 0
		start := time.Now()

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < queriesPerGoroutine; j++ {
					queryStart := time.Now()
					req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files?page=%d&limit=20", j+1), nil)
					resp := httptest.NewRecorder()
					srv.Router().ServeHTTP(resp, req)

					duration := time.Since(queryStart)

					mu.Lock()
					if resp.Code == http.StatusOK {
						latencies = append(latencies, duration)
					} else {
						errorCount++
					}
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()
		totalDuration := time.Since(start)

		p50, p95, p99 := calculatePercentiles(latencies)
		var totalLatency time.Duration
		for _, lat := range latencies {
			totalLatency += lat
		}
		avgLatency := totalLatency / time.Duration(len(latencies))
		totalQueries := numConcurrent * queriesPerGoroutine
		throughput := float64(totalQueries) / totalDuration.Seconds()

		t.Logf("📊 Concurrent Queries Performance:")
		t.Logf("  Concurrent workers: %d", numConcurrent)
		t.Logf("  Queries per worker: %d", queriesPerGoroutine)
		t.Logf("  Total queries: %d", totalQueries)
		t.Logf("  Total duration: %v", totalDuration)
		t.Logf("  Average latency: %v", avgLatency)
		t.Logf("  P50 Latency: %v", p50)
		t.Logf("  P95 Latency: %v", p95)
		t.Logf("  P99 Latency: %v", p99)
		t.Logf("  Throughput: %.2f queries/sec", throughput)
		t.Logf("  Errors: %d", errorCount)
		t.Logf("  Success Rate: %.2f%%", float64(len(latencies))/float64(totalQueries)*100)

		if errorCount > totalQueries/100 {
			t.Errorf("error rate too high: %d errors out of %d queries", errorCount, totalQueries)
		}
		if totalDuration > 2*time.Second {
			t.Errorf("concurrent queries took too long: %v (expected < 2s)", totalDuration)
		}
	})

	// Test different filter combinations performance
	t.Run("filter_combinations", func(t *testing.T) {
		filterTests := []struct {
			name string
			url  string
		}{
			{"no_filters", "/files?limit=50"},
			{"category_filter", "/files?category=images&limit=50"},
			{"type_filter", "/files?type=image&limit=50"},
			{"extension_filter", "/files?extension=pdf&limit=50"},
			{"name_filter", "/files?name=test&limit=50"},
			{"category_and_type", "/files?category=images&type=image&limit=50"},
			{"category_and_extension", "/files?category=documents&extension=pdf&limit=50"},
			{"multiple_filters", "/files?category=images&type=image&extension=jpg&limit=50"},
		}

		for _, ft := range filterTests {
			t.Run(ft.name, func(t *testing.T) {
				const iterations = 50
				latencies := make([]time.Duration, 0, iterations)

				for i := 0; i < iterations; i++ {
					start := time.Now()
					req := httptest.NewRequest(http.MethodGet, ft.url, nil)
					resp := httptest.NewRecorder()
					srv.Router().ServeHTTP(resp, req)

					if resp.Code == http.StatusOK {
						latencies = append(latencies, time.Since(start))
					}
				}

				if len(latencies) == 0 {
					t.Fatalf("no successful requests for %s", ft.name)
				}

				var totalDuration time.Duration
				for _, lat := range latencies {
					totalDuration += lat
				}
				avgDuration := totalDuration / time.Duration(len(latencies))
				throughput := float64(len(latencies)) / totalDuration.Seconds()

				t.Logf("  Filter: %s", ft.name)
				t.Logf("    Average latency: %v", avgDuration)
				t.Logf("    Throughput: %.2f queries/sec", throughput)

				if avgDuration > 100*time.Millisecond {
					t.Errorf("average latency too high for %s: %v (expected < 100ms)", ft.name, avgDuration)
				}
			})
		}
	})

	// Test pagination performance
	t.Run("pagination_performance", func(t *testing.T) {
		pageSizes := []int{10, 25, 50, 100}
		for _, pageSize := range pageSizes {
			t.Run(fmt.Sprintf("page_size_%d", pageSize), func(t *testing.T) {
				const iterations = 30
				latencies := make([]time.Duration, 0, iterations)

				for i := 0; i < iterations; i++ {
					start := time.Now()
					req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files?page=1&limit=%d", pageSize), nil)
					resp := httptest.NewRecorder()
					srv.Router().ServeHTTP(resp, req)

					if resp.Code == http.StatusOK {
						latencies = append(latencies, time.Since(start))
					}
				}

				if len(latencies) == 0 {
					t.Fatalf("no successful requests for page size %d", pageSize)
				}

				var totalDuration time.Duration
				for _, lat := range latencies {
					totalDuration += lat
				}
				avgDuration := totalDuration / time.Duration(len(latencies))
				throughput := float64(len(latencies)) / totalDuration.Seconds()

				t.Logf("  Page size: %d", pageSize)
				t.Logf("    Average latency: %v", avgDuration)
				t.Logf("    Throughput: %.2f queries/sec", throughput)
			})
		}
	})

	// Test sorting performance
	t.Run("sorting_performance", func(t *testing.T) {
		sortOptions := []struct {
			name string
			url  string
		}{
			{"sort_by_name_asc", "/files?sort=name&order=asc&limit=50"},
			{"sort_by_name_desc", "/files?sort=name&order=desc&limit=50"},
			{"sort_by_uploaded_at_asc", "/files?sort=uploaded_at&order=asc&limit=50"},
			{"sort_by_uploaded_at_desc", "/files?sort=uploaded_at&order=desc&limit=50"},
			{"sort_by_size_asc", "/files?sort=size&order=asc&limit=50"},
			{"sort_by_size_desc", "/files?sort=size&order=desc&limit=50"},
			{"sort_by_category_asc", "/files?sort=category&order=asc&limit=50"},
			{"sort_by_mime_type_asc", "/files?sort=mime_type&order=asc&limit=50"},
		}

		for _, so := range sortOptions {
			t.Run(so.name, func(t *testing.T) {
				const iterations = 30
				latencies := make([]time.Duration, 0, iterations)

				for i := 0; i < iterations; i++ {
					start := time.Now()
					req := httptest.NewRequest(http.MethodGet, so.url, nil)
					resp := httptest.NewRecorder()
					srv.Router().ServeHTTP(resp, req)

					if resp.Code == http.StatusOK {
						latencies = append(latencies, time.Since(start))
					}
				}

				if len(latencies) == 0 {
					t.Fatalf("no successful requests for %s", so.name)
				}

				var totalDuration time.Duration
				for _, lat := range latencies {
					totalDuration += lat
				}
				avgDuration := totalDuration / time.Duration(len(latencies))
				throughput := float64(len(latencies)) / totalDuration.Seconds()

				t.Logf("  Sort option: %s", so.name)
				t.Logf("    Average latency: %v", avgDuration)
				t.Logf("    Throughput: %.2f queries/sec", throughput)

				if avgDuration > 100*time.Millisecond {
					t.Errorf("average latency too high for %s: %v (expected < 100ms)", so.name, avgDuration)
				}
			})
		}
	})
}

// TestListingReliability tests listing endpoint reliability under load
func TestListingReliability(t *testing.T) {
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload files
	const numFiles = 100
	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf("reliability test file %d", i)
		filename := fmt.Sprintf("reliability_%d.txt", i)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d", resp.Code)
		}
	}

	time.Sleep(100 * time.Millisecond) // Allow indexing

	// Run reliability test
	const totalQueries = 1000
	const numWorkers = 20
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			queriesPerWorker := totalQueries / numWorkers
			for j := 0; j < queriesPerWorker; j++ {
				req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files?page=%d&limit=20", (j%10)+1), nil)
				resp := httptest.NewRecorder()
				srv.Router().ServeHTTP(resp, req)

				mu.Lock()
				if resp.Code == http.StatusOK {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(start)

	successRate := float64(successCount) / float64(totalQueries) * 100
	t.Logf("📊 Reliability Test Results:")
	t.Logf("  Total queries: %d", totalQueries)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Average QPS: %.2f", float64(totalQueries)/totalDuration.Seconds())

	if successRate < 99.0 {
		t.Errorf("success rate too low: %.2f%% (expected >= 99%%)", successRate)
	}
}

// TestListingScalability tests listing performance with varying dataset sizes
func TestListingScalability(t *testing.T) {
	datasetSizes := []int{10, 50, 100, 200, 500}

	for _, size := range datasetSizes {
		t.Run(fmt.Sprintf("dataset_size_%d", size), func(t *testing.T) {
			cfg := config.Config{
				Addr:           ":0",
				DataDir:        t.TempDir(),
				MaxUploadBytes: 100 * 1024 * 1024,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			srv, err := api.NewServer(cfg, logger)
			if err != nil {
				t.Fatalf("failed to create server: %v", err)
			}

			// Upload files
			for i := 0; i < size; i++ {
				content := fmt.Sprintf("scalability test file %d", i)
				filename := fmt.Sprintf("scalability_%d.txt", i)

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				fileWriter, err := writer.CreateFormFile("file", filename)
				if err != nil {
					t.Fatalf("create form file: %v", err)
				}
				fileWriter.Write([]byte(content))
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				resp := httptest.NewRecorder()
				srv.Router().ServeHTTP(resp, req)

				if resp.Code != http.StatusOK {
					t.Fatalf("upload failed: %d", resp.Code)
				}
			}

			time.Sleep(100 * time.Millisecond) // Allow indexing

			// Measure listing performance
			const iterations = 30
			latencies := make([]time.Duration, 0, iterations)

			for i := 0; i < iterations; i++ {
				start := time.Now()
				req := httptest.NewRequest(http.MethodGet, "/files?limit=50", nil)
				resp := httptest.NewRecorder()
				srv.Router().ServeHTTP(resp, req)

				if resp.Code == http.StatusOK {
					latencies = append(latencies, time.Since(start))
				}
			}

			if len(latencies) == 0 {
				t.Fatalf("no successful requests for dataset size %d", size)
			}

			var totalDuration time.Duration
			for _, lat := range latencies {
				totalDuration += lat
			}
			avgDuration := totalDuration / time.Duration(len(latencies))
			p50, p95, p99 := calculatePercentiles(latencies)
			throughput := float64(len(latencies)) / totalDuration.Seconds()

			t.Logf("📊 Scalability Test Results:")
			t.Logf("  Dataset size: %d files", size)
			t.Logf("  Average latency: %v", avgDuration)
			t.Logf("  P50 Latency: %v", p50)
			t.Logf("  P95 Latency: %v", p95)
			t.Logf("  P99 Latency: %v", p99)
			t.Logf("  Throughput: %.2f queries/sec", throughput)

			// Scalability check: latency should not grow too much with dataset size
			maxExpectedLatency := time.Duration(size/20) * time.Millisecond
			if maxExpectedLatency < 5*time.Millisecond {
				maxExpectedLatency = 5 * time.Millisecond
			}
			if maxExpectedLatency > 150*time.Millisecond {
				maxExpectedLatency = 150 * time.Millisecond
			}

			if avgDuration > maxExpectedLatency {
				t.Logf("  ⚠️  Warning: latency %v exceeds expected %v for dataset size %d", avgDuration, maxExpectedLatency, size)
			} else {
				t.Logf("  ✅ Latency within expected range for dataset size %d", size)
			}
		})
	}
}
