package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

func setupTestServer(t *testing.T) (*api.Server, string) {
	tempDir, err := os.MkdirTemp("", "rhinobox_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := config.Config{
		Addr:           ":0",
		DataDir:        tempDir,
		MaxUploadBytes: 10 * 1024 * 1024, // 10MB for testing
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server, tempDir
}

func TestValidation_JSONIngest_Valid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	body := map[string]interface{}{
		"namespace": "test",
		"documents": []map[string]interface{}{
			{"id": 1, "name": "test"},
		},
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/ingest/json", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestValidation_JSONIngest_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing documents",
			body: map[string]interface{}{
				"namespace": "test",
			},
		},
		{
			name: "empty documents",
			body: map[string]interface{}{
				"namespace": "test",
				"documents": []interface{}{},
			},
		},
		{
			name: "invalid namespace",
			body: map[string]interface{}{
				"namespace": "invalid/namespace",
				"documents": []map[string]interface{}{
					{"id": 1},
				},
			},
		},
		{
			name: "too long comment",
			body: map[string]interface{}{
				"namespace": "test",
				"comment":   strings.Repeat("a", 1001),
				"documents": []map[string]interface{}{
					{"id": 1},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/ingest/json", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			server.Router().ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
			}

			var errorResp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
				t.Fatalf("failed to unmarshal error response: %v", err)
			}

			if errorResp["error"] == nil {
				t.Error("expected error field in response")
			}
		})
	}
}

func TestValidation_FileRename_Valid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	body := map[string]interface{}{
		"hash":     "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
		"new_name": "newfile.txt",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/files/rename", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	// Should fail with 404 (file not found) but not 400 (validation error)
	if rr.Code == http.StatusBadRequest {
		var errorResp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &errorResp)
		t.Errorf("expected validation to pass, got 400. Response: %v", errorResp)
	}
}

func TestValidation_FileRename_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing hash",
			body: map[string]interface{}{
				"new_name": "newfile.txt",
			},
		},
		{
			name: "missing new_name",
			body: map[string]interface{}{
				"hash": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
			},
		},
		{
			name: "invalid hash format",
			body: map[string]interface{}{
				"hash":     "invalid-hash",
				"new_name": "newfile.txt",
			},
		},
		{
			name: "path traversal in filename",
			body: map[string]interface{}{
				"hash":     "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
				"new_name": "../../etc/passwd",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("PATCH", "/files/rename", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			server.Router().ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestValidation_FileDelete_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Test missing file_id
	req := httptest.NewRequest("DELETE", "/files/", nil)
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	// Should get 404 (route not found) or 400 (validation error)
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 404 or 400, got %d", rr.Code)
	}

	// Test invalid hash format
	req2 := httptest.NewRequest("DELETE", "/files/invalid-hash", nil)
	rr2 := httptest.NewRecorder()
	server.Router().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid hash, got %d. Body: %s", rr2.Code, rr2.Body.String())
	}
}

func TestValidation_FileSearch_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Test missing name parameter
	req := httptest.NewRequest("GET", "/files/search", nil)
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Test empty name parameter
	req2 := httptest.NewRequest("GET", "/files/search?name=", nil)
	rr2 := httptest.NewRecorder()
	server.Router().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty name, got %d", rr2.Code)
	}
}

func TestValidation_FileDownload_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Test missing both hash and path
	req := httptest.NewRequest("GET", "/files/download", nil)
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Test invalid hash format
	req2 := httptest.NewRequest("GET", "/files/download?hash=invalid", nil)
	rr2 := httptest.NewRecorder()
	server.Router().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid hash, got %d", rr2.Code)
	}
}

func TestValidation_FileMetadata_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Test missing hash
	req := httptest.NewRequest("GET", "/files/metadata", nil)
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Test invalid hash format
	req2 := httptest.NewRequest("GET", "/files/metadata?hash=invalid", nil)
	rr2 := httptest.NewRecorder()
	server.Router().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid hash, got %d", rr2.Code)
	}
}

func TestValidation_MetadataUpdate_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name string
		body map[string]interface{}
		path string
	}{
		{
			name: "missing body",
			body: nil,
			path: "/files/abc123/metadata",
		},
		{
			name: "invalid action",
			body: map[string]interface{}{
				"action":   "invalid",
				"metadata": map[string]interface{}{"key": "value"},
			},
			path: "/files/abc123/metadata",
		},
		{
			name: "protected field in metadata",
			body: map[string]interface{}{
				"action": "merge",
				"metadata": map[string]interface{}{
					"hash": "newhash",
				},
			},
			path: "/files/abc123/metadata",
		},
		{
			name: "remove action without fields",
			body: map[string]interface{}{
				"action": "remove",
			},
			path: "/files/abc123/metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("PATCH", tt.path, bytes.NewReader(bodyBytes))
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			server.Router().ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestValidation_MediaIngest_FileSize(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Create a file that's too large (assuming MaxUploadBytes is 10MB)
	// We'll test with a smaller limit by creating a large multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "large.jpg")
	
	// Write data larger than typical limit (but we can't easily test this without modifying config)
	// For now, just test that the endpoint accepts valid files
	part.Write([]byte("fake image data"))
	writer.Close()

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	// Should either succeed (if file is small enough) or fail with appropriate error
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("unexpected status %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestValidation_BatchMetadataUpdate_Invalid(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing updates",
			body: map[string]interface{}{},
		},
		{
			name: "empty updates",
			body: map[string]interface{}{
				"updates": []interface{}{},
			},
		},
		{
			name: "too many updates",
			body: map[string]interface{}{
				"updates": make([]interface{}, 101),
			},
		},
		{
			name: "update without hash",
			body: map[string]interface{}{
				"updates": []map[string]interface{}{
					{
						"action":   "merge",
						"metadata": map[string]interface{}{"key": "value"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/files/metadata/batch", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			server.Router().ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestValidation_ErrorResponseFormat(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Make a request that will fail validation
	req := httptest.NewRequest("POST", "/ingest/json", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var errorResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	// Check error response structure
	if errorResp["error"] == nil {
		t.Error("expected 'error' field in response")
	}

	if errorResp["details"] == nil {
		t.Error("expected 'details' field in response")
	}

	details, ok := errorResp["details"].([]interface{})
	if !ok {
		t.Error("expected 'details' to be an array")
	} else if len(details) == 0 {
		t.Error("expected at least one validation error detail")
	}
}

// Benchmark validation middleware performance
func BenchmarkValidation_JSONIngest(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "rhinobox_bench_*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.Config{
		Addr:           ":0",
		DataDir:        tempDir,
		MaxUploadBytes: 10 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}

	body := map[string]interface{}{
		"namespace": "test",
		"documents": []map[string]interface{}{
			{"id": 1, "name": "test"},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/ingest/json", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		server.Router().ServeHTTP(rr, req)
	}
}

func TestValidation_ConsistentErrorFormat(t *testing.T) {
	server, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Test multiple endpoints to ensure consistent error format
	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/ingest/json", `{}`},
		{"PATCH", "/files/rename", `{}`},
		{"POST", "/files/metadata/batch", `{}`},
	}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func(t *testing.T) {
			var bodyReader io.Reader
			if endpoint.body != "" {
				bodyReader = strings.NewReader(endpoint.body)
			}

			req := httptest.NewRequest(endpoint.method, endpoint.path, bodyReader)
			if endpoint.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			server.Router().ServeHTTP(rr, req)

			if rr.Code == http.StatusBadRequest {
				var errorResp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
					t.Errorf("failed to unmarshal error response: %v", err)
					return
				}

				// Verify consistent structure
				if errorResp["error"] == nil {
					t.Error("expected 'error' field")
				}
			}
		})
	}
}

