package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// PerformanceMetrics tracks performance data for test reporting
type PerformanceMetrics struct {
	Operation          string
	Duration           time.Duration
	Throughput         float64 // ops/sec
	LatencyP50         time.Duration
	LatencyP95         time.Duration
	LatencyP99         time.Duration
	SuccessCount       int
	FailureCount       int
	TotalBytes         int64
	AverageFileSize    int64
	ErrorRate          float64
	ConcurrentOps      int
	Timestamp          time.Time
}

// TestRealWorldEndToEnd performs comprehensive end-to-end testing with real files from Downloads
func TestRealWorldEndToEnd(t *testing.T) {
	downloadsDir := filepath.Join(os.Getenv("HOME"), "Downloads")
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		t.Skip("Downloads directory not found, skipping real-world E2E test")
	}

	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 500 * 1024 * 1024, // 500MB
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Find real-world test files (limit to reasonable sizes for testing)
	testFiles := findRealWorldTestFiles(t, downloadsDir, 50*1024*1024) // 50MB max per file
	if len(testFiles) == 0 {
		t.Skip("No suitable test files found in Downloads directory")
	}

	// Limit to 10 files for comprehensive testing
	maxFiles := 10
	if len(testFiles) > maxFiles {
		testFiles = testFiles[:maxFiles]
	}

	t.Logf("Testing with %d real-world files from Downloads directory", len(testFiles))

	var allMetrics []PerformanceMetrics
	var uploadedHashes []string

	// Phase 1: Upload real-world files and measure performance
	t.Run("Upload_RealWorldFiles", func(t *testing.T) {
		uploadMetrics := testUploadRealWorldFiles(t, srv, testFiles)
		allMetrics = append(allMetrics, uploadMetrics...)
		
		// Collect hashes for subsequent tests
		for _, file := range testFiles {
			hash, _ := uploadRealFile(t, srv, file, filepath.Base(file))
			if hash != "" {
				uploadedHashes = append(uploadedHashes, hash)
			}
		}
	})

	// Phase 2: Metadata retrieval and updates
	t.Run("Metadata_Operations", func(t *testing.T) {
		if len(uploadedHashes) == 0 {
			t.Skip("No files uploaded, skipping metadata tests")
		}
		metadataMetrics := testMetadataOperations(t, srv, uploadedHashes)
		allMetrics = append(allMetrics, metadataMetrics...)
	})

	// Phase 3: File retrieval and streaming
	t.Run("File_Retrieval", func(t *testing.T) {
		if len(uploadedHashes) == 0 {
			t.Skip("No files uploaded, skipping retrieval tests")
		}
		retrievalMetrics := testFileRetrieval(t, srv, uploadedHashes, testFiles)
		allMetrics = append(allMetrics, retrievalMetrics...)
	})

	// Phase 4: Batch operations
	t.Run("Batch_Operations", func(t *testing.T) {
		if len(uploadedHashes) < 3 {
			t.Skip("Need at least 3 files for batch operations")
		}
		batchMetrics := testBatchOperations(t, srv, uploadedHashes[:min(10, len(uploadedHashes))])
		allMetrics = append(allMetrics, batchMetrics...)
	})

	// Print comprehensive metrics report
	printMetricsReport(t, allMetrics)
}

// testUploadRealWorldFiles tests uploading real-world files and collects metrics
func testUploadRealWorldFiles(t *testing.T, srv *api.Server, files []string) []PerformanceMetrics {
	var metrics []PerformanceMetrics
	var latencies []time.Duration
	var totalBytes int64
	successCount := 0
	failureCount := 0

	startTime := time.Now()

	for i, filePath := range files {
		fileStart := time.Now()
		
		// Read file info
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			t.Logf("Skipping file %s: %v", filePath, err)
			failureCount++
			continue
		}

		// Upload file
		hash, _ := uploadRealFile(t, srv, filePath, filepath.Base(filePath))
		if hash == "" {
			failureCount++
			continue
		}

		successCount++
		duration := time.Since(fileStart)
		latencies = append(latencies, duration)
		totalBytes += fileInfo.Size()

		t.Logf("Uploaded [%d/%d] %s (%.2f KB, %v)", 
			i+1, len(files), filepath.Base(filePath), 
			float64(fileInfo.Size())/1024, duration)
	}

	totalDuration := time.Since(startTime)
	throughput := float64(successCount) / totalDuration.Seconds()
	avgFileSize := int64(0)
	if successCount > 0 {
		avgFileSize = totalBytes / int64(successCount)
	}

	// Calculate percentiles
	sortDurations(latencies)
	p50, p95, p99 := calculatePercentiles(latencies)

	metrics = append(metrics, PerformanceMetrics{
		Operation:       "Upload_RealWorldFiles",
		Duration:        totalDuration,
		Throughput:      throughput,
		LatencyP50:      p50,
		LatencyP95:      p95,
		LatencyP99:      p99,
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		TotalBytes:      totalBytes,
		AverageFileSize: avgFileSize,
		ErrorRate:       float64(failureCount) / float64(len(files)) * 100,
		Timestamp:       time.Now(),
	})

	return metrics
}

// testMetadataOperations tests metadata retrieval and updates
func testMetadataOperations(t *testing.T, srv *api.Server, hashes []string) []PerformanceMetrics {
	var metrics []PerformanceMetrics
	var latencies []time.Duration
	successCount := 0
	failureCount := 0

	startTime := time.Now()

	// Test metadata retrieval
	for _, hash := range hashes {
		opStart := time.Now()
		metadata := getFileMetadata(t, srv, hash)
		duration := time.Since(opStart)
		latencies = append(latencies, duration)

		if metadata != nil && metadata["hash"] != nil {
			successCount++
		} else {
			failureCount++
		}
	}

	totalDuration := time.Since(startTime)
	throughput := float64(successCount) / totalDuration.Seconds()
	sortDurations(latencies)
	p50, p95, p99 := calculatePercentiles(latencies)

	metrics = append(metrics, PerformanceMetrics{
		Operation:    "Metadata_Retrieval",
		Duration:     totalDuration,
		Throughput:   throughput,
		LatencyP50:   p50,
		LatencyP95:   p95,
		LatencyP99:   p99,
		SuccessCount: successCount,
		FailureCount: failureCount,
		ErrorRate:    float64(failureCount) / float64(len(hashes)) * 100,
		Timestamp:    time.Now(),
	})

	// Test metadata updates
	updateLatencies := []time.Duration{}
	updateSuccess := 0
	updateFailure := 0
	updateStart := time.Now()

	for i, hash := range hashes[:min(5, len(hashes))] {
		opStart := time.Now()
		req := map[string]interface{}{
			"action": "merge",
			"metadata": map[string]string{
				"test_tag":     fmt.Sprintf("e2e_test_%d", i),
				"test_timestamp": time.Now().Format(time.RFC3339),
				"test_category": "real_world_e2e",
			},
		}
		updateResp := updateMetadata(t, srv, hash, req, http.StatusOK)
		duration := time.Since(opStart)
		updateLatencies = append(updateLatencies, duration)

		if updateResp["new_metadata"] != nil {
			updateSuccess++
		} else {
			updateFailure++
		}
	}

	updateDuration := time.Since(updateStart)
	updateThroughput := float64(updateSuccess) / updateDuration.Seconds()
	sortDurations(updateLatencies)
	upP50, upP95, upP99 := calculatePercentiles(updateLatencies)

	metrics = append(metrics, PerformanceMetrics{
		Operation:    "Metadata_Update",
		Duration:     updateDuration,
		Throughput:   updateThroughput,
		LatencyP50:   upP50,
		LatencyP95:   upP95,
		LatencyP99:   upP99,
		SuccessCount: updateSuccess,
		FailureCount: updateFailure,
		ErrorRate:    float64(updateFailure) / float64(min(5, len(hashes))) * 100,
		Timestamp:    time.Now(),
	})

	return metrics
}

// testFileRetrieval tests file download and streaming
func testFileRetrieval(t *testing.T, srv *api.Server, hashes []string, originalFiles []string) []PerformanceMetrics {
	var metrics []PerformanceMetrics
	var downloadLatencies []time.Duration
	var streamLatencies []time.Duration
	downloadSuccess := 0
	downloadFailure := 0
	streamSuccess := 0
	streamFailure := 0

	// Test downloads
	downloadStart := time.Now()
	for i, hash := range hashes[:min(5, len(hashes))] {
		opStart := time.Now()
		content := downloadFileByHash(t, srv, hash)
		duration := time.Since(opStart)
		downloadLatencies = append(downloadLatencies, duration)

		if len(content) > 0 {
			downloadSuccess++
			t.Logf("Downloaded file %d: %d bytes in %v", i+1, len(content), duration)
		} else {
			downloadFailure++
		}
	}
	downloadDuration := time.Since(downloadStart)
	downloadThroughput := float64(downloadSuccess) / downloadDuration.Seconds()
	sortDurations(downloadLatencies)
	dlP50, dlP95, dlP99 := calculatePercentiles(downloadLatencies)

	metrics = append(metrics, PerformanceMetrics{
		Operation:    "File_Download",
		Duration:     downloadDuration,
		Throughput:   downloadThroughput,
		LatencyP50:   dlP50,
		LatencyP95:   dlP95,
		LatencyP99:   dlP99,
		SuccessCount: downloadSuccess,
		FailureCount: downloadFailure,
		ErrorRate:    float64(downloadFailure) / float64(min(5, len(hashes))) * 100,
		Timestamp:    time.Now(),
	})

	// Test streaming with range requests
	streamStart := time.Now()
	for i, hash := range hashes[:min(3, len(hashes))] {
		opStart := time.Now()
		// Stream first 1KB
		partial := streamFileRange(t, srv, hash, 0, 1023)
		duration := time.Since(opStart)
		streamLatencies = append(streamLatencies, duration)

		if len(partial) > 0 {
			streamSuccess++
			t.Logf("Streamed file %d: %d bytes in %v", i+1, len(partial), duration)
		} else {
			streamFailure++
		}
	}
	streamDuration := time.Since(streamStart)
	streamThroughput := float64(streamSuccess) / streamDuration.Seconds()
	sortDurations(streamLatencies)
	stP50, stP95, stP99 := calculatePercentiles(streamLatencies)

	metrics = append(metrics, PerformanceMetrics{
		Operation:    "File_Stream",
		Duration:     streamDuration,
		Throughput:   streamThroughput,
		LatencyP50:   stP50,
		LatencyP95:   stP95,
		LatencyP99:   stP99,
		SuccessCount: streamSuccess,
		FailureCount: streamFailure,
		ErrorRate:    float64(streamFailure) / float64(min(3, len(hashes))) * 100,
		Timestamp:    time.Now(),
	})

	return metrics
}

// testBatchOperations tests batch metadata updates
func testBatchOperations(t *testing.T, srv *api.Server, hashes []string) []PerformanceMetrics {
	var metrics []PerformanceMetrics

	// Prepare batch update request
	updates := make([]map[string]interface{}, len(hashes))
	for i, hash := range hashes {
		updates[i] = map[string]interface{}{
			"hash":   hash,
			"action": "merge",
			"metadata": map[string]string{
				"batch_operation": "e2e_test",
				"batch_index":     fmt.Sprintf("%d", i),
				"batch_timestamp": time.Now().Format(time.RFC3339),
			},
		}
	}

	batchReq := map[string]interface{}{
		"updates": updates,
	}

	startTime := time.Now()
	body, _ := json.Marshal(batchReq)
	req := httptest.NewRequest("POST", "/files/metadata/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	duration := time.Since(startTime)

	if w.Code != http.StatusOK {
		t.Errorf("Batch update failed: status %d", w.Code)
		return metrics
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("Failed to decode batch response: %v", err)
		return metrics
	}

	successCount := int(resp["success_count"].(float64))
	failureCount := int(resp["failure_count"].(float64))
	throughput := float64(successCount) / duration.Seconds()

	metrics = append(metrics, PerformanceMetrics{
		Operation:    "Batch_Metadata_Update",
		Duration:     duration,
		Throughput:   throughput,
		SuccessCount: successCount,
		FailureCount: failureCount,
		ErrorRate:    float64(failureCount) / float64(len(hashes)) * 100,
		Timestamp:    time.Now(),
	})

	return metrics
}

// Helper functions

func findRealWorldTestFiles(t *testing.T, dir string, maxSize int64) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		// Skip hidden files and system files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Only include files within size limit and with reasonable extensions
		if info.Size() > 0 && info.Size() <= maxSize {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			// Include common file types
			validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".pdf", ".txt", ".doc", ".docx", 
				".mp4", ".mp3", ".wav", ".zip", ".epub", ".odt", ".webp"}
			for _, validExt := range validExts {
				if ext == validExt {
					files = append(files, filepath.Join(dir, entry.Name()))
					break
				}
			}
		}
	}

	return files
}

func sortDurations(durations []time.Duration) {
	for i := 0; i < len(durations)-1; i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
}

func calculatePercentiles(durations []time.Duration) (p50, p95, p99 time.Duration) {
	if len(durations) == 0 {
		return 0, 0, 0
	}
	
	sortDurations(durations)
	
	p50Idx := int(float64(len(durations)) * 0.50)
	p95Idx := int(float64(len(durations)) * 0.95)
	p99Idx := int(float64(len(durations)) * 0.99)
	
	if p50Idx >= len(durations) {
		p50Idx = len(durations) - 1
	}
	if p95Idx >= len(durations) {
		p95Idx = len(durations) - 1
	}
	if p99Idx >= len(durations) {
		p99Idx = len(durations) - 1
	}
	
	return durations[p50Idx], durations[p95Idx], durations[p99Idx]
}

func printMetricsReport(t *testing.T, metrics []PerformanceMetrics) {
	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("COMPREHENSIVE PERFORMANCE METRICS REPORT")
	t.Log(strings.Repeat("=", 80))
	
	for _, m := range metrics {
		t.Logf("\nOperation: %s", m.Operation)
		t.Logf("  Duration: %v", m.Duration)
		t.Logf("  Throughput: %.2f ops/sec", m.Throughput)
		if m.LatencyP50 > 0 {
			t.Logf("  Latency P50: %v", m.LatencyP50)
			t.Logf("  Latency P95: %v", m.LatencyP95)
			t.Logf("  Latency P99: %v", m.LatencyP99)
		}
		t.Logf("  Success: %d, Failure: %d", m.SuccessCount, m.FailureCount)
		t.Logf("  Error Rate: %.2f%%", m.ErrorRate)
		if m.AverageFileSize > 0 {
			t.Logf("  Average File Size: %.2f KB", float64(m.AverageFileSize)/1024)
		}
		if m.TotalBytes > 0 {
			t.Logf("  Total Bytes: %.2f MB", float64(m.TotalBytes)/(1024*1024))
		}
	}
	
	t.Log(strings.Repeat("=", 80))
}

// updateMetadata is a helper to update file metadata
func updateMetadata(t *testing.T, srv *api.Server, hash string, reqBody map[string]interface{}, expectedStatus int) map[string]interface{} {
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request error: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response error: %v", err)
	}

	return resp
}
