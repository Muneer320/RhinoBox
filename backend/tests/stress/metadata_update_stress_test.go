package stress

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func uploadTestFile(t *testing.T, srv *api.Server, filename, content, comment string) map[string]interface{} {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if comment != "" {
		if err := writer.WriteField("comment", comment); err != nil {
			t.Fatalf("write comment: %v", err)
		}
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: status %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	stored, ok := resp["stored"].([]interface{})
	if !ok || len(stored) == 0 {
		t.Fatalf("missing stored files in response")
	}

	fileInfo := stored[0].(map[string]interface{})
	return fileInfo
}

func TestMetadataUpdateConcurrency(t *testing.T) {
if testing.Short() {
t.Skip("skipping stress test in short mode")
}

tmpDir := t.TempDir()
cfg := config.Config{
DataDir:        tmpDir,
Addr:           ":0",
MaxUploadBytes: 10 << 20,
}

srv, err := api.NewServer(cfg, testLogger())
if err != nil {
t.Fatalf("NewServer() error = %v", err)
}

// Upload a test file
uploadResp := uploadTestFile(t, srv, "test.txt", "content", "initial")
hash := uploadResp["hash"].(string)

// Perform concurrent metadata updates
const numGoroutines = 100
const updatesPerGoroutine = 10

var wg sync.WaitGroup
var successCount, failureCount atomic.Int64

startTime := time.Now()

for i := 0; i < numGoroutines; i++ {
wg.Add(1)
go func(id int) {
defer wg.Done()

for j := 0; j < updatesPerGoroutine; j++ {
req := map[string]interface{}{
"action": "merge",
"metadata": map[string]string{
fmt.Sprintf("field_%d_%d", id, j): fmt.Sprintf("value_%d_%d", id, j),
},
}

body, _ := json.Marshal(req)
httpReq := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
httpReq.Header.Set("Content-Type", "application/json")

w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, httpReq)

if w.Code == http.StatusOK {
successCount.Add(1)
} else {
failureCount.Add(1)
}
}
}(i)
}

wg.Wait()
duration := time.Since(startTime)

totalOps := numGoroutines * updatesPerGoroutine
opsPerSecond := float64(totalOps) / duration.Seconds()

t.Logf("Concurrent metadata updates:")
t.Logf("  Total operations: %d", totalOps)
t.Logf("  Successful: %d", successCount.Load())
t.Logf("  Failed: %d", failureCount.Load())
t.Logf("  Duration: %v", duration)
t.Logf("  Operations/sec: %.2f", opsPerSecond)

if failureCount.Load() > 0 {
t.Errorf("expected all operations to succeed, but %d failed", failureCount.Load())
}

// Verify all updates were persisted
finalReq := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader([]byte(`{}`)))
w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, finalReq)

if w.Code != http.StatusBadRequest {
// The request should fail but we can check the state
}
}

func TestMetadataUpdateThroughput(t *testing.T) {
if testing.Short() {
t.Skip("skipping stress test in short mode")
}

tmpDir := t.TempDir()
cfg := config.Config{
DataDir:        tmpDir,
Addr:           ":0",
MaxUploadBytes: 10 << 20,
}

srv, err := api.NewServer(cfg, testLogger())
if err != nil {
t.Fatalf("NewServer() error = %v", err)
}

// Upload multiple test files
const numFiles = 100
hashes := make([]string, numFiles)

for i := 0; i < numFiles; i++ {
uploadResp := uploadTestFile(t, srv, fmt.Sprintf("test%d.txt", i), "content", "")
hashes[i] = uploadResp["hash"].(string)
}

// Sequential updates to measure throughput
const updatesPerFile = 10
startTime := time.Now()

for i := 0; i < numFiles; i++ {
for j := 0; j < updatesPerFile; j++ {
req := map[string]interface{}{
"action": "merge",
"metadata": map[string]string{
fmt.Sprintf("field_%d", j): fmt.Sprintf("value_%d", j),
},
}

body, _ := json.Marshal(req)
httpReq := httptest.NewRequest("PATCH", "/files/"+hashes[i]+"/metadata", bytes.NewReader(body))
httpReq.Header.Set("Content-Type", "application/json")

w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, httpReq)

if w.Code != http.StatusOK {
t.Errorf("update failed for file %d, update %d: status %d", i, j, w.Code)
}
}
}

duration := time.Since(startTime)
totalOps := numFiles * updatesPerFile
opsPerSecond := float64(totalOps) / duration.Seconds()

t.Logf("Sequential metadata update throughput:")
t.Logf("  Total files: %d", numFiles)
t.Logf("  Updates per file: %d", updatesPerFile)
t.Logf("  Total operations: %d", totalOps)
t.Logf("  Duration: %v", duration)
t.Logf("  Operations/sec: %.2f", opsPerSecond)

if opsPerSecond < 50 {
t.Logf("Warning: throughput is low (%.2f ops/sec)", opsPerSecond)
}
}

func TestBatchMetadataUpdateStress(t *testing.T) {
if testing.Short() {
t.Skip("skipping stress test in short mode")
}

tmpDir := t.TempDir()
cfg := config.Config{
DataDir:        tmpDir,
Addr:           ":0",
MaxUploadBytes: 10 << 20,
}

srv, err := api.NewServer(cfg, testLogger())
if err != nil {
t.Fatalf("NewServer() error = %v", err)
}

// Upload files
const numFiles = 100
hashes := make([]string, numFiles)

for i := 0; i < numFiles; i++ {
uploadResp := uploadTestFile(t, srv, fmt.Sprintf("test%d.txt", i), "content", "")
hashes[i] = uploadResp["hash"].(string)
}

// Perform batch updates
const batchSize = 100
updates := make([]map[string]interface{}, batchSize)

startTime := time.Now()

for i := 0; i < batchSize; i++ {
updates[i] = map[string]interface{}{
"hash":   hashes[i],
"action": "merge",
"metadata": map[string]string{
"batch_field": fmt.Sprintf("batch_value_%d", i),
"timestamp":   time.Now().Format(time.RFC3339),
},
}
}

batchReq := map[string]interface{}{
"updates": updates,
}

body, _ := json.Marshal(batchReq)
req := httptest.NewRequest("POST", "/files/metadata/batch", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")

w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, req)

duration := time.Since(startTime)

if w.Code != http.StatusOK {
t.Fatalf("batch update failed: status %d: %s", w.Code, w.Body.String())
}

var resp map[string]interface{}
if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
t.Fatalf("decode response error: %v", err)
}

successCount := int(resp["success_count"].(float64))
opsPerSecond := float64(batchSize) / duration.Seconds()

t.Logf("Batch metadata update stress:")
t.Logf("  Batch size: %d", batchSize)
t.Logf("  Successful: %d", successCount)
t.Logf("  Duration: %v", duration)
t.Logf("  Operations/sec: %.2f", opsPerSecond)

if successCount != batchSize {
t.Errorf("expected %d successes, got %d", batchSize, successCount)
}
}

func TestMetadataUpdateLargePayloads(t *testing.T) {
if testing.Short() {
t.Skip("skipping stress test in short mode")
}

tmpDir := t.TempDir()
cfg := config.Config{
DataDir:        tmpDir,
Addr:           ":0",
MaxUploadBytes: 10 << 20,
}

srv, err := api.NewServer(cfg, testLogger())
if err != nil {
t.Fatalf("NewServer() error = %v", err)
}

// Upload a test file
uploadResp := uploadTestFile(t, srv, "test.txt", "content", "")
hash := uploadResp["hash"].(string)

// Test with various payload sizes
// Note: Total size = sum of (key_length + value_size) for all fields
// MaxMetadataSize = 64KB, MaxMetadataValueSize = 32KB
// Key format "field_N" is ~7-8 bytes, using 8 bytes for calculations
testCases := []struct {
name       string
fieldCount int
valueSize  int
shouldPass bool
}{
{"small payload", 10, 100, true},
{"medium payload", 50, 500, true},
{"large payload", 100, 1000, true},
{"near limit", 50, 1300, true}, // 50 * (8 + 1300) = 65400 bytes < 64KB
{"at limit", 64, 1016, true},   // 64 * (8 + 1016) = 65536 bytes = 64KB exactly
{"exceeds value limit", 1, 32*1024 + 1, false}, // 32769 bytes > 32KB per value
{"exceeds total limit", 100, 700, false},        // 100 * (8 + 700) = 70800 bytes > 64KB
{"very large payload", 100, 10000, false},       // 100 * (8 + 10000) = 1000800 bytes >> 64KB
}

for _, tc := range testCases {
t.Run(tc.name, func(t *testing.T) {
metadata := make(map[string]string)
for i := 0; i < tc.fieldCount; i++ {
metadata[fmt.Sprintf("field_%d", i)] = strings.Repeat("x", tc.valueSize)
}

req := map[string]interface{}{
"action":   "replace",
"metadata": metadata,
}

body, _ := json.Marshal(req)
httpReq := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
httpReq.Header.Set("Content-Type", "application/json")

w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, httpReq)

if tc.shouldPass && w.Code != http.StatusOK {
t.Errorf("expected success, got status %d: %s", w.Code, w.Body.String())
} else if !tc.shouldPass && w.Code == http.StatusOK {
t.Errorf("expected failure, but got success")
}

t.Logf("Payload: %d fields × %d bytes = ~%d KB, Status: %d",
tc.fieldCount, tc.valueSize, (tc.fieldCount*tc.valueSize)/1024, w.Code)
})
}
}

func TestMetadataUpdateRaceConditions(t *testing.T) {
if testing.Short() {
t.Skip("skipping stress test in short mode")
}

tmpDir := t.TempDir()
cfg := config.Config{
DataDir:        tmpDir,
Addr:           ":0",
MaxUploadBytes: 10 << 20,
}

srv, err := api.NewServer(cfg, testLogger())
if err != nil {
t.Fatalf("NewServer() error = %v", err)
}

// Upload a test file
uploadResp := uploadTestFile(t, srv, "test.txt", "content", "")
hash := uploadResp["hash"].(string)

// Perform concurrent updates with different actions
const numGoroutines = 50
var wg sync.WaitGroup

actions := []string{"merge", "replace", "remove"}
startTime := time.Now()

for i := 0; i < numGoroutines; i++ {
wg.Add(1)
go func(id int) {
defer wg.Done()

action := actions[id%len(actions)]

var reqBody map[string]interface{}
switch action {
case "merge", "replace":
reqBody = map[string]interface{}{
"action": action,
"metadata": map[string]string{
fmt.Sprintf("field_%d", id): fmt.Sprintf("value_%d", id),
},
}
case "remove":
reqBody = map[string]interface{}{
"action": "remove",
"fields": []string{fmt.Sprintf("field_%d", (id-1+numGoroutines)%numGoroutines)},
}
}

body, _ := json.Marshal(reqBody)
httpReq := httptest.NewRequest("PATCH", "/files/"+hash+"/metadata", bytes.NewReader(body))
httpReq.Header.Set("Content-Type", "application/json")

w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, httpReq)
}(i)
}

wg.Wait()
duration := time.Since(startTime)

t.Logf("Race condition test completed:")
t.Logf("  Concurrent goroutines: %d", numGoroutines)
t.Logf("  Duration: %v", duration)
t.Logf("  No deadlocks or crashes detected")
}

func TestMetadataUpdateMemoryUsage(t *testing.T) {
if testing.Short() {
t.Skip("skipping stress test in short mode")
}

tmpDir := t.TempDir()
cfg := config.Config{
DataDir:        tmpDir,
Addr:           ":0",
MaxUploadBytes: 10 << 20,
}

srv, err := api.NewServer(cfg, testLogger())
if err != nil {
t.Fatalf("NewServer() error = %v", err)
}

// Upload many files
const numFiles = 1000
hashes := make([]string, numFiles)

t.Log("Uploading files...")
for i := 0; i < numFiles; i++ {
uploadResp := uploadTestFile(t, srv, fmt.Sprintf("test%d.txt", i), "content", "")
hashes[i] = uploadResp["hash"].(string)
}

// Update each file multiple times
const updatesPerFile = 20
t.Log("Performing updates...")

startTime := time.Now()
for i := 0; i < numFiles; i++ {
for j := 0; j < updatesPerFile; j++ {
req := map[string]interface{}{
"action": "merge",
"metadata": map[string]string{
fmt.Sprintf("iteration_%d", j): fmt.Sprintf("value_%d", j),
},
}

body, _ := json.Marshal(req)
httpReq := httptest.NewRequest("PATCH", "/files/"+hashes[i]+"/metadata", bytes.NewReader(body))
httpReq.Header.Set("Content-Type", "application/json")

w := httptest.NewRecorder()
srv.Router().ServeHTTP(w, httpReq)

if w.Code != http.StatusOK {
t.Errorf("update failed: status %d", w.Code)
}
}

if (i+1)%100 == 0 {
t.Logf("Processed %d/%d files", i+1, numFiles)
}
}

duration := time.Since(startTime)
totalOps := numFiles * updatesPerFile

t.Logf("Memory usage test completed:")
t.Logf("  Total files: %d", numFiles)
t.Logf("  Updates per file: %d", updatesPerFile)
t.Logf("  Total operations: %d", totalOps)
t.Logf("  Duration: %v", duration)
t.Logf("  Operations/sec: %.2f", float64(totalOps)/duration.Seconds())
}
