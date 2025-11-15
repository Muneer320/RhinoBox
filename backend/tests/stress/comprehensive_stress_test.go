package stress

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// StressTestMetrics captures comprehensive metrics for stress testing
type StressTestMetrics struct {
	TestName           string
	TotalOperations    int64
	SuccessfulOps      int64
	FailedOps          int64
	TotalDuration      time.Duration
	Throughput         float64 // ops/sec
	AverageLatency     time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	P50Latency         time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	ErrorRate          float64
	ConcurrentWorkers  int
	TotalBytes         int64
	MemoryPeak         int64 // bytes (if available)
	Timestamp          time.Time
	LatencyHistogram   map[string]int // latency buckets
}

// TestComprehensiveStress performs a full stress test suite with real-world scenarios
func TestComprehensiveStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping comprehensive stress test in short mode")
	}

	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		Addr:           ":0",
		MaxUploadBytes: 100 << 20, // 100MB
	}

	srv, err := api.NewServer(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	var allMetrics []StressTestMetrics

	// Test 1: High-concurrency uploads
	t.Run("HighConcurrencyUploads", func(t *testing.T) {
		metrics := testHighConcurrencyUploads(t, srv)
		allMetrics = append(allMetrics, metrics)
	})

	// Test 2: Sustained load metadata updates
	t.Run("SustainedLoadMetadataUpdates", func(t *testing.T) {
		metrics := testSustainedLoadMetadataUpdates(t, srv)
		allMetrics = append(allMetrics, metrics)
	})

	// Test 3: Mixed workload (upload + metadata + retrieval)
	t.Run("MixedWorkload", func(t *testing.T) {
		metrics := testMixedWorkload(t, srv)
		allMetrics = append(allMetrics, metrics)
	})

	// Test 4: Large batch operations
	t.Run("LargeBatchOperations", func(t *testing.T) {
		metrics := testLargeBatchOperations(t, srv)
		allMetrics = append(allMetrics, metrics)
	})

	// Test 5: Real-world file stress test
	t.Run("RealWorldFileStress", func(t *testing.T) {
		metrics := testRealWorldFileStress(t, srv)
		if metrics.TestName != "" {
			allMetrics = append(allMetrics, metrics)
		}
	})

	// Print comprehensive stress test report
	printStressTestReport(t, allMetrics)
}

// testHighConcurrencyUploads tests concurrent file uploads
func testHighConcurrencyUploads(t *testing.T, srv *api.Server) StressTestMetrics {
	const numWorkers = 50
	const filesPerWorker = 10
	const totalOps = numWorkers * filesPerWorker

	var latencies []time.Duration
	var latenciesMu sync.Mutex
	var successCount, failureCount atomic.Int64
	var totalBytes atomic.Int64
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < filesPerWorker; j++ {
				opStart := time.Now()
				
				content := []byte(fmt.Sprintf("stress test file from worker %d, file %d\n%s", 
					workerID, j, strings.Repeat("x", 1024))) // 1KB files
				
				hash, _ := uploadTestFileContent(t, srv, fmt.Sprintf("stress_%d_%d.txt", workerID, j), content)
				
				latency := time.Since(opStart)
				latenciesMu.Lock()
				latencies = append(latencies, latency)
				latenciesMu.Unlock()
				
				if hash != "" {
					successCount.Add(1)
					totalBytes.Add(int64(len(content)))
				} else {
					failureCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	metrics := calculateStressMetrics("HighConcurrencyUploads", totalOps, int64(totalOps), 
		successCount.Load(), failureCount.Load(), totalDuration, latencies, totalBytes.Load(), numWorkers)

	return metrics
}

// testSustainedLoadMetadataUpdates tests sustained metadata update load
func testSustainedLoadMetadataUpdates(t *testing.T, srv *api.Server) StressTestMetrics {
	// Upload initial files
	const numFiles = 20
	hashes := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		uploadResp := uploadTestFile(t, srv, fmt.Sprintf("sustained_%d.txt", i), "content", "")
		hashes[i] = uploadResp["hash"].(string)
	}

	const numWorkers = 30
	const updatesPerWorker = 50
	const totalOps = int64(numWorkers * updatesPerWorker)

	var latencies []time.Duration
	var successCount, failureCount atomic.Int64
	var wg sync.WaitGroup
	var mu sync.Mutex

	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < updatesPerWorker; j++ {
				opStart := time.Now()
				
				hash := hashes[workerID%numFiles]
				req := map[string]interface{}{
					"action": "merge",
					"metadata": map[string]string{
						fmt.Sprintf("worker_%d_update_%d", workerID, j): fmt.Sprintf("value_%d_%d", workerID, j),
					},
				}

				body, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				srv.Router().ServeHTTP(w, httpReq)

				latency := time.Since(opStart)
				
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				if w.Code == http.StatusOK {
					successCount.Add(1)
				} else {
					failureCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	metrics := calculateStressMetrics("SustainedLoadMetadataUpdates", totalOps, totalOps,
		successCount.Load(), failureCount.Load(), totalDuration, latencies, 0, numWorkers)

	return metrics
}

// testMixedWorkload tests a mixed workload of uploads, metadata updates, and retrievals
func testMixedWorkload(t *testing.T, srv *api.Server) StressTestMetrics {
	const numWorkers = 40
	const opsPerWorker = 20
	const totalOps = int64(numWorkers * opsPerWorker)

	var latencies []time.Duration
	var successCount, failureCount atomic.Int64
	var totalBytes atomic.Int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	hashes := make([]string, 0, 100)
	var hashesMu sync.Mutex

	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < opsPerWorker; j++ {
				opStart := time.Now()
				var success bool

				switch j % 3 {
				case 0: // Upload
					content := []byte(fmt.Sprintf("mixed workload file %d_%d", workerID, j))
					hash, _ := uploadTestFileContent(t, srv, fmt.Sprintf("mixed_%d_%d.txt", workerID, j), content)
					if hash != "" {
						hashesMu.Lock()
						hashes = append(hashes, hash)
						hashesMu.Unlock()
						success = true
						totalBytes.Add(int64(len(content)))
					}

				case 1: // Metadata update
					hashesMu.Lock()
					if len(hashes) > 0 {
						hash := hashes[workerID%len(hashes)]
						hashesMu.Unlock()
						
						req := map[string]interface{}{
							"action": "merge",
							"metadata": map[string]string{
								"mixed_workload": fmt.Sprintf("update_%d_%d", workerID, j),
							},
						}
						body, _ := json.Marshal(req)
						httpReq := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
						httpReq.Header.Set("Content-Type", "application/json")
						w := httptest.NewRecorder()
						srv.Router().ServeHTTP(w, httpReq)
						success = w.Code == http.StatusOK
					} else {
						hashesMu.Unlock()
					}

				case 2: // Retrieval
					hashesMu.Lock()
					if len(hashes) > 0 {
						hash := hashes[workerID%len(hashes)]
						hashesMu.Unlock()
						
						req := httptest.NewRequest("GET", fmt.Sprintf("/files/metadata?hash=%s", hash), nil)
						w := httptest.NewRecorder()
						srv.Router().ServeHTTP(w, req)
						success = w.Code == http.StatusOK
					} else {
						hashesMu.Unlock()
					}
				}

				latency := time.Since(opStart)
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				if success {
					successCount.Add(1)
				} else {
					failureCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	metrics := calculateStressMetrics("MixedWorkload", totalOps, totalOps,
		successCount.Load(), failureCount.Load(), totalDuration, latencies, totalBytes.Load(), numWorkers)

	return metrics
}

// testLargeBatchOperations tests large batch metadata updates
func testLargeBatchOperations(t *testing.T, srv *api.Server) StressTestMetrics {
	// Upload files for batch operations
	const numFiles = 100
	hashes := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		uploadResp := uploadTestFile(t, srv, fmt.Sprintf("batch_%d.txt", i), "content", "")
		hashes[i] = uploadResp["hash"].(string)
	}

	const batchSize = 100
	const numBatches = 10
	const totalOps = int64(numBatches)

	var latencies []time.Duration
	var successCount, failureCount atomic.Int64

	startTime := time.Now()

	for batch := 0; batch < numBatches; batch++ {
		opStart := time.Now()

		updates := make([]map[string]interface{}, batchSize)
		for i := 0; i < batchSize; i++ {
			updates[i] = map[string]interface{}{
				"hash":   hashes[i],
				"action": "merge",
				"metadata": map[string]string{
					"batch_id":  fmt.Sprintf("batch_%d", batch),
					"item_id":   fmt.Sprintf("%d", i),
					"timestamp": time.Now().Format(time.RFC3339),
				},
			}
		}

		batchReq := map[string]interface{}{"updates": updates}
		body, _ := json.Marshal(batchReq)
		req := httptest.NewRequest("POST", "/files/metadata/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		latency := time.Since(opStart)
		latencies = append(latencies, latency)

		if w.Code == http.StatusOK {
			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
				success := int(resp["success_count"].(float64))
				successCount.Add(int64(success))
				failureCount.Add(int64(batchSize - success))
			}
		} else {
			failureCount.Add(batchSize)
		}
	}

	totalDuration := time.Since(startTime)

	metrics := calculateStressMetrics("LargeBatchOperations", totalOps, int64(numBatches*batchSize),
		successCount.Load(), failureCount.Load(), totalDuration, latencies, 0, 1)

	return metrics
}

// testRealWorldFileStress tests with actual files from Downloads if available
func testRealWorldFileStress(t *testing.T, srv *api.Server) StressTestMetrics {
	downloadsDir := filepath.Join(os.Getenv("HOME"), "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skip("Downloads directory not found")
		return StressTestMetrics{}
	}

	// Find real files (limit to smaller files for stress testing)
	entries, err := os.ReadDir(downloadsDir)
	if err != nil {
		t.Skipf("Cannot read Downloads: %v", err)
		return StressTestMetrics{}
	}

	var testFiles []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// Limit to files under 5MB for stress testing
		if info.Size() > 0 && info.Size() < 5*1024*1024 {
			testFiles = append(testFiles, filepath.Join(downloadsDir, entry.Name()))
			if len(testFiles) >= 20 { // Limit to 20 files
				break
			}
		}
	}

	if len(testFiles) == 0 {
		t.Skip("No suitable test files found")
		return StressTestMetrics{}
	}

	const numWorkers = 10
	const filesPerWorker = 2
	const totalOps = int64(numWorkers * filesPerWorker)

	var latencies []time.Duration
	var successCount, failureCount atomic.Int64
	var totalBytes atomic.Int64
	var wg sync.WaitGroup
	var mu sync.Mutex

	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < filesPerWorker; j++ {
				fileIdx := (workerID*filesPerWorker + j) % len(testFiles)
				filePath := testFiles[fileIdx]

				opStart := time.Now()
				
				file, err := os.Open(filePath)
				if err != nil {
					failureCount.Add(1)
					continue
				}

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", filepath.Base(filePath))
				if err != nil {
					file.Close()
					failureCount.Add(1)
					continue
				}

				if _, err := io.Copy(part, file); err != nil {
					file.Close()
					writer.Close()
					failureCount.Add(1)
					continue
				}
				file.Close()
				writer.Close()

				req := httptest.NewRequest("POST", "/ingest/media", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				w := httptest.NewRecorder()
				srv.Router().ServeHTTP(w, req)

				latency := time.Since(opStart)
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				if w.Code == http.StatusOK {
					successCount.Add(1)
					if info, err := os.Stat(filePath); err == nil {
						totalBytes.Add(info.Size())
					}
				} else {
					failureCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	metrics := calculateStressMetrics("RealWorldFileStress", totalOps, totalOps,
		successCount.Load(), failureCount.Load(), totalDuration, latencies, totalBytes.Load(), numWorkers)

	return metrics
}

// Helper functions

func uploadTestFileContent(t *testing.T, srv *api.Server, filename string, content []byte) (string, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", ""
	}
	if _, err := part.Write(content); err != nil {
		return "", ""
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return "", ""
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		return "", ""
	}

	stored, ok := response["stored"].([]interface{})
	if !ok || len(stored) == 0 {
		return "", ""
	}

	fileInfo, ok := stored[0].(map[string]interface{})
	if !ok {
		return "", ""
	}

	hash, ok := fileInfo["hash"].(string)
	if !ok {
		return "", ""
	}

	path, ok := fileInfo["path"].(string)
	if !ok {
		return "", ""
	}

	return hash, path
}

func calculateStressMetrics(testName string, totalOps, expectedOps int64, success, failure int64,
	duration time.Duration, latencies []time.Duration, totalBytes int64, workers int) StressTestMetrics {

	throughput := float64(success) / duration.Seconds()
	errorRate := float64(failure) / float64(expectedOps) * 100

	var avgLatency, minLatency, maxLatency time.Duration
	var p50, p95, p99 time.Duration

	if len(latencies) > 0 {
		// Sort latencies
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})

		minLatency = sorted[0]
		maxLatency = sorted[len(sorted)-1]

		var sum time.Duration
		for _, l := range sorted {
			sum += l
		}
		avgLatency = sum / time.Duration(len(sorted))

		// Calculate percentiles
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

		p50 = sorted[p50Idx]
		p95 = sorted[p95Idx]
		p99 = sorted[p99Idx]
	}

	return StressTestMetrics{
		TestName:          testName,
		TotalOperations:   totalOps,
		SuccessfulOps:     success,
		FailedOps:         failure,
		TotalDuration:     duration,
		Throughput:        throughput,
		AverageLatency:    avgLatency,
		MinLatency:        minLatency,
		MaxLatency:        maxLatency,
		P50Latency:        p50,
		P95Latency:        p95,
		P99Latency:        p99,
		ErrorRate:         errorRate,
		ConcurrentWorkers: workers,
		TotalBytes:        totalBytes,
		Timestamp:          time.Now(),
	}
}

func printStressTestReport(t *testing.T, metrics []StressTestMetrics) {
	t.Log("\n" + strings.Repeat("=", 100))
	t.Log("COMPREHENSIVE STRESS TEST REPORT")
	t.Log(strings.Repeat("=", 100))

	for _, m := range metrics {
		t.Logf("\n%s", strings.Repeat("-", 100))
		t.Logf("Test: %s", m.TestName)
		t.Logf("  Total Operations: %d", m.TotalOperations)
		t.Logf("  Successful: %d, Failed: %d", m.SuccessfulOps, m.FailedOps)
		t.Logf("  Duration: %v", m.TotalDuration)
		t.Logf("  Throughput: %.2f ops/sec", m.Throughput)
		t.Logf("  Error Rate: %.2f%%", m.ErrorRate)
		t.Logf("  Concurrent Workers: %d", m.ConcurrentWorkers)
		if m.TotalBytes > 0 {
			t.Logf("  Total Bytes: %.2f MB", float64(m.TotalBytes)/(1024*1024))
		}
		if m.AverageLatency > 0 {
			t.Logf("  Latency - Min: %v, Avg: %v, Max: %v", m.MinLatency, m.AverageLatency, m.MaxLatency)
			t.Logf("  Latency - P50: %v, P95: %v, P99: %v", m.P50Latency, m.P95Latency, m.P99Latency)
		}
	}

	t.Log(strings.Repeat("=", 100))
}
