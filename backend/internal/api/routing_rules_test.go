package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/storage"
	"log/slog"
)

func setupTestServer(t *testing.T) *Server {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 512 * 1024 * 1024,
	}

	logger := slog.Default()
	store, err := storage.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage manager: %v", err)
	}

	s, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Replace storage with our test storage
	s.storage = store

	return s
}

func TestHandleSuggestRoutingRule(t *testing.T) {
	s := setupTestServer(t)

	tests := []struct {
		name        string
		requestBody map[string]any
		statusCode  int
		checkFunc   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "valid rule with MIME type",
			requestBody: map[string]any{
				"mime_type":   "application/x-custom",
				"destination": []string{"files", "custom"},
			},
			statusCode: http.StatusOK,
			checkFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp["message"] != "routing rule added successfully" {
					t.Errorf("unexpected message: %v", resp["message"])
				}
			},
		},
		{
			name: "valid rule with extension",
			requestBody: map[string]any{
				"extension":   ".xyz",
				"destination": []string{"files", "xyz"},
			},
			statusCode: http.StatusOK,
			checkFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp["message"] != "routing rule added successfully" {
					t.Errorf("unexpected message: %v", resp["message"])
				}
			},
		},
		{
			name: "missing destination",
			requestBody: map[string]any{
				"mime_type": "application/x-custom",
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "missing mime_type and extension",
			requestBody: map[string]any{
				"destination": []string{"files", "custom"},
			},
			statusCode: http.StatusBadRequest,
		},
		{
			name: "empty destination",
			requestBody: map[string]any{
				"mime_type":   "application/x-custom",
				"destination": []string{},
			},
			statusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			s.handleSuggestRoutingRule(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, w.Code)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, w)
			}
		})
	}
}

func TestHandleGetRoutingRules(t *testing.T) {
	s := setupTestServer(t)

	// Add a rule first
	ruleBody := map[string]any{
		"mime_type":   "application/x-test",
		"destination": []string{"files", "test"},
	}
	body, _ := json.Marshal(ruleBody)
	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSuggestRoutingRule(w, req)

	// Now get all rules
	req = httptest.NewRequest("GET", "/routing-rules", nil)
	w = httptest.NewRecorder()
	s.handleGetRoutingRules(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	rules, ok := resp["rules"].([]any)
	if !ok {
		t.Fatal("response should contain rules array")
	}

	if len(rules) == 0 {
		t.Error("expected at least one rule")
	}
}

func TestHandleUpdateRoutingRule(t *testing.T) {
	s := setupTestServer(t)

	// Add a rule first
	ruleBody := map[string]any{
		"mime_type":   "application/x-update",
		"destination": []string{"files", "original"},
	}
	body, _ := json.Marshal(ruleBody)
	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSuggestRoutingRule(w, req)

	// Update the rule
	updateBody := map[string]any{
		"mime_type":   "application/x-update",
		"destination": []string{"files", "updated"},
	}
	body, _ = json.Marshal(updateBody)
	req = httptest.NewRequest("PUT", "/routing-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.handleUpdateRoutingRule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify the rule was updated
	rulesMgr := s.storage.RoutingRules()
	rule := rulesMgr.FindRule("application/x-update", "")
	if rule == nil {
		t.Fatal("rule should exist")
	}
	if len(rule.Destination) != 2 || rule.Destination[1] != "updated" {
		t.Errorf("rule not updated correctly: %v", rule.Destination)
	}
}

func TestHandleDeleteRoutingRule(t *testing.T) {
	s := setupTestServer(t)

	// Add a rule first
	ruleBody := map[string]any{
		"mime_type":   "application/x-delete",
		"destination": []string{"files", "delete"},
	}
	body, _ := json.Marshal(ruleBody)
	req := httptest.NewRequest("POST", "/routing-rules/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSuggestRoutingRule(w, req)

	// Delete the rule
	deleteBody := map[string]any{
		"mime_type": "application/x-delete",
	}
	body, _ = json.Marshal(deleteBody)
	req = httptest.NewRequest("DELETE", "/routing-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.handleDeleteRoutingRule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify the rule was deleted
	rulesMgr := s.storage.RoutingRules()
	rule := rulesMgr.FindRule("application/x-delete", "")
	if rule != nil {
		t.Error("rule should be deleted")
	}
}

func TestHandleDeleteRoutingRuleNotFound(t *testing.T) {
	s := setupTestServer(t)

	deleteBody := map[string]any{
		"mime_type": "application/x-nonexistent",
	}
	body, _ := json.Marshal(deleteBody)
	req := httptest.NewRequest("DELETE", "/routing-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleDeleteRoutingRule(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

