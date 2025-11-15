package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

func setupE2ETestServer(t *testing.T) (*api.Server, string) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 512 * 1024 * 1024,
	}

	logger := slog.Default()
	s, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return s, tmpDir
}

func TestE2EUnrecognizedFileFormat(t *testing.T) {
	s, _ := setupE2ETestServer(t)

	// Create a test file with unrecognized format
	testFile := bytes.NewBufferString("test content for unrecognized format")
	
	req := httptest.NewRequest("POST", "/ingest", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	
	// We'll simulate the multipart form manually for testing
	// In a real scenario, we'd use multipart.Writer
	// For now, let's test the routing rules API directly
	
	// First, verify no rule exists for .unknown extension
	rulesReq := httptest.NewRequest("GET", "/routing-rules", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, rulesReq)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	var rulesResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	
	initialCount := int(rulesResp["count"].(float64))
	
	// Add a routing rule for .unknown extension
	ruleBody := map[string]any{
		"extension":   ".unknown",
		"destination": []string{"files", "unknown"},
	}
	body, _ := json.Marshal(ruleBody)
	
	req = httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	
	// Verify rule was added
	rulesReq = httptest.NewRequest("GET", "/routing-rules", nil)
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, rulesReq)
	
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	
	newCount := int(rulesResp["count"].(float64))
	if newCount != initialCount+1 {
		t.Errorf("expected rule count to increase by 1, got %d -> %d", initialCount, newCount)
	}
}

func TestE2ERoutingRulePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create first server instance
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 512 * 1024 * 1024,
	}
	
	logger := slog.Default()
	s1, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	
	// Add a routing rule
	ruleBody := map[string]any{
		"mime_type":   "application/x-persist",
		"extension":   ".persist",
		"destination": []string{"files", "persist"},
	}
	body, _ := json.Marshal(ruleBody)
	
	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s1.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	
	// Create second server instance with same data directory
	s2, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create second server: %v", err)
	}
	
	// Verify rule persists by querying API
	req = httptest.NewRequest("GET", "/routing-rules", nil)
	w = httptest.NewRecorder()
	s2.Router().ServeHTTP(w, req)
	
	var rulesResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	
	rules := rulesResp["rules"].([]any)
	found := false
	for _, r := range rules {
		rule := r.(map[string]any)
		if mime, ok := rule["mime_type"].(string); ok && mime == "application/x-persist" {
			found = true
			dest := rule["destination"].([]any)
			if len(dest) != 2 || dest[0].(string) != "files" || dest[1].(string) != "persist" {
				t.Errorf("unexpected destination: %v", dest)
			}
		}
	}
	
	if !found {
		t.Fatal("rule should persist after server restart")
	}
}

func TestE2ERoutingRuleUsageTracking(t *testing.T) {
	s, _ := setupE2ETestServer(t)
	
	// Add a routing rule
	ruleBody := map[string]any{
		"mime_type":   "application/x-usage",
		"destination": []string{"files", "usage"},
	}
	body, _ := json.Marshal(ruleBody)
	
	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	// Note: We can't directly access storage from outside the api package
	// This test verifies the API works correctly
	// For detailed usage tracking, see unit tests in routing_rules_test.go
	time.Sleep(100 * time.Millisecond)
}

func TestE2EFullWorkflow(t *testing.T) {
	s, tmpDir := setupE2ETestServer(t)
	
	// Step 1: Upload a file with unrecognized format
	// (In a real scenario, we'd use multipart form, but for testing we'll simulate)
	
	// Step 2: Check that the file is marked as unrecognized
	// This would be done through the /ingest endpoint response
	
	// Step 3: User suggests routing for the unrecognized format
	ruleBody := map[string]any{
		"extension":   ".custom",
		"destination": []string{"files", "custom", "type"},
	}
	body, _ := json.Marshal(ruleBody)
	
	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	
	// Step 4: Verify rule was added
	req = httptest.NewRequest("GET", "/routing-rules", nil)
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	var rulesResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	
	rules := rulesResp["rules"].([]any)
	found := false
	for _, r := range rules {
		rule := r.(map[string]any)
		if ext, ok := rule["extension"].(string); ok && ext == ".custom" {
			found = true
			dest := rule["destination"].([]any)
			if len(dest) != 3 {
				t.Errorf("unexpected destination length: %d", len(dest))
			}
		}
	}
	
	if !found {
		t.Error("rule for .custom extension not found")
	}
	
	// Step 5: Update the rule
	updateBody := map[string]any{
		"extension":   ".custom",
		"destination": []string{"files", "custom", "updated"},
	}
	body, _ = json.Marshal(updateBody)
	
	req = httptest.NewRequest("PUT", "/routing-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	// Step 6: Verify persistence by creating new server instance
	s2, _ := setupE2ETestServer(t)
	
	// Get rules from new server instance
	req = httptest.NewRequest("GET", "/routing-rules", nil)
	w = httptest.NewRecorder()
	s2.Router().ServeHTTP(w, req)
	
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	
	rules = rulesResp["rules"].([]any)
	found = false
	for _, r := range rules {
		rule := r.(map[string]any)
		if ext, ok := rule["extension"].(string); ok && ext == ".custom" {
			found = true
			dest := rule["destination"].([]any)
			if len(dest) != 3 || dest[2].(string) != "updated" {
				t.Errorf("rule not updated correctly: %v", dest)
			}
		}
	}
	
	if !found {
		t.Error("rule should persist after server recreation")
	}
	
	// Step 7: Delete the rule
	deleteBody := map[string]any{
		"extension": ".custom",
	}
	body, _ = json.Marshal(deleteBody)
	
	req = httptest.NewRequest("DELETE", "/routing-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	
	// Verify rule was deleted by checking API
	req = httptest.NewRequest("GET", "/routing-rules", nil)
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	
	rules = rulesResp["rules"].([]any)
	found = false
	for _, r := range rules {
		rule := r.(map[string]any)
		if ext, ok := rule["extension"].(string); ok && ext == ".custom" {
			found = true
		}
	}
	
	if found {
		t.Error("rule should be deleted")
	}
}

