package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestComprehensiveRoutingRulesE2E tests the complete workflow from unrecognized file
// to user suggestion to automatic application
func TestComprehensiveRoutingRulesE2E(t *testing.T) {
	s, tmpDir := setupE2ETestServer(t)

	// Step 1: Upload a file with unrecognized format (.xyz)
	testContent := []byte("test content for .xyz file")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("files", "test.xyz")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(testContent); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var ingestResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ingestResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify file was marked as unrecognized
	results := ingestResp["results"].(map[string]any)
	files := results["files"].([]any)
	if len(files) == 0 {
		t.Fatal("expected at least one file in results")
	}

	fileResult := files[0].(map[string]any)
	if !fileResult["unrecognized"].(bool) {
		t.Error("file should be marked as unrecognized")
	}
	if !fileResult["requires_routing"].(bool) {
		t.Error("file should require routing")
	}

	// Step 2: User suggests routing for .xyz extension
	ruleBody := map[string]any{
		"extension":   ".xyz",
		"destination": []string{"files", "custom", "xyz"},
	}
	bodyBytes, _ := json.Marshal(ruleBody)

	req = httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Step 3: Upload another .xyz file - should now use the learned rule
	testContent2 := []byte("test content for second .xyz file")
	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)
	
	part2, err := writer2.CreateFormFile("files", "test2.xyz")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part2.Write(testContent2); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}
	writer2.Close()

	req2 := httptest.NewRequest("POST", "/ingest", body2)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w2.Code, w2.Body.String())
	}

	var ingestResp2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &ingestResp2); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify second file was NOT marked as unrecognized (rule applied)
	results2 := ingestResp2["results"].(map[string]any)
	files2 := results2["files"].([]any)
	if len(files2) == 0 {
		t.Fatal("expected at least one file in results")
	}

	fileResult2 := files2[0].(map[string]any)
	if fileResult2["unrecognized"].(bool) {
		t.Error("file should NOT be marked as unrecognized after rule is learned")
	}

	// Verify file was stored in the custom location
	storedPath := fileResult2["stored_path"].(string)
	if !contains(storedPath, "custom") || !contains(storedPath, "xyz") {
		t.Errorf("file should be stored in custom/xyz location, got: %s", storedPath)
	}

	// Step 4: Verify rule usage count increased
	req3 := httptest.NewRequest("GET", "/routing-rules", nil)
	w3 := httptest.NewRecorder()
	s.Router().ServeHTTP(w3, req3)

	var rulesResp map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	rules := rulesResp["rules"].([]any)
	found := false
	for _, r := range rules {
		rule := r.(map[string]any)
		if ext, ok := rule["extension"].(string); ok && ext == ".xyz" {
			found = true
			usageCount := int(rule["usage_count"].(float64))
			if usageCount < 1 {
				t.Errorf("expected usage count >= 1, got %d", usageCount)
			}
		}
	}

	if !found {
		t.Error("rule for .xyz extension should exist")
	}

	_ = tmpDir // Suppress unused variable warning
}

// TestRoutingRulesMetrics tests performance and metrics collection
func TestRoutingRulesMetrics(t *testing.T) {
	s, _ := setupE2ETestServer(t)

	// Measure time to add a rule
	start := time.Now()
	ruleBody := map[string]any{
		"mime_type":   "application/x-metrics-test",
		"extension":   ".metrics",
		"destination": []string{"files", "metrics"},
	}
	bodyBytes, _ := json.Marshal(ruleBody)

	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	addRuleDuration := time.Since(start)
	if addRuleDuration > 100*time.Millisecond {
		t.Logf("Warning: Adding rule took %v (expected < 100ms)", addRuleDuration)
	}

	// Measure time to find a rule (via API)
	start = time.Now()
	reqFind := httptest.NewRequest("GET", "/routing-rules", nil)
	wFind := httptest.NewRecorder()
	s.Router().ServeHTTP(wFind, reqFind)
	findRuleDuration := time.Since(start)

	if wFind.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, wFind.Code)
	}

	var findResp map[string]any
	if err := json.Unmarshal(wFind.Body.Bytes(), &findResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	rules := findResp["rules"].([]any)
	found := false
	for _, r := range rules {
		rule := r.(map[string]any)
		if mime, ok := rule["mime_type"].(string); ok && mime == "application/x-metrics-test" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("rule should be found")
	}

	if findRuleDuration > 50*time.Millisecond {
		t.Logf("Warning: Finding rule took %v (expected < 50ms)", findRuleDuration)
	}

	// Measure time to get all rules
	start = time.Now()
	req2 := httptest.NewRequest("GET", "/routing-rules", nil)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)
	getAllRulesDuration := time.Since(start)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w2.Code)
	}

	if getAllRulesDuration > 50*time.Millisecond {
		t.Logf("Warning: Getting all rules took %v (expected < 50ms)", getAllRulesDuration)
	}

	t.Logf("Metrics: AddRule=%v, FindRule=%v, GetAllRules=%v", 
		addRuleDuration, findRuleDuration, getAllRulesDuration)
}

// TestRoutingRulesConcurrency tests concurrent access to routing rules
func TestRoutingRulesConcurrency(t *testing.T) {
	s, _ := setupE2ETestServer(t)

	// Add multiple rules concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			ruleBody := map[string]any{
				"extension":   fmt.Sprintf(".concurrent%d", id),
				"destination": []string{"files", "concurrent", fmt.Sprintf("type%d", id)},
			}
			bodyBytes, _ := json.Marshal(ruleBody)

			req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				done <- true
			} else {
				done <- false
			}
		}(i)
	}

	// Wait for all goroutines
	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	if successCount != 10 {
		t.Errorf("expected 10 successful rule additions, got %d", successCount)
	}

	// Verify all rules exist
	req := httptest.NewRequest("GET", "/routing-rules", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	var rulesResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rulesResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	count := int(rulesResp["count"].(float64))
	if count < 10 {
		t.Errorf("expected at least 10 rules, got %d", count)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}


