package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoutingRulesManager(t *testing.T) {
	tmpDir := t.TempDir()
	
	mgr, err := NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create routing rules manager: %v", err)
	}

	// Test adding a rule by MIME type
	err = mgr.AddRule("application/x-custom", "", []string{"files", "custom"})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	// Test finding rule by MIME type
	rule := mgr.FindRule("application/x-custom", "")
	if rule == nil {
		t.Fatal("rule not found by MIME type")
	}
	if len(rule.Destination) != 2 || rule.Destination[0] != "files" || rule.Destination[1] != "custom" {
		t.Errorf("unexpected destination: %v", rule.Destination)
	}

	// Test adding a rule by extension
	err = mgr.AddRule("", ".xyz", []string{"files", "xyz"})
	if err != nil {
		t.Fatalf("failed to add rule by extension: %v", err)
	}

	// Test finding rule by extension
	rule = mgr.FindRule("", ".xyz")
	if rule == nil {
		t.Fatal("rule not found by extension")
	}
	if len(rule.Destination) != 2 || rule.Destination[0] != "files" || rule.Destination[1] != "xyz" {
		t.Errorf("unexpected destination: %v", rule.Destination)
	}

	// Test updating existing rule
	err = mgr.AddRule("application/x-custom", "", []string{"files", "custom", "updated"})
	if err != nil {
		t.Fatalf("failed to update rule: %v", err)
	}

	rule = mgr.FindRule("application/x-custom", "")
	if len(rule.Destination) != 3 || rule.Destination[2] != "updated" {
		t.Errorf("rule not updated correctly: %v", rule.Destination)
	}

	// Test increment usage
	initialCount := rule.UsageCount
	mgr.IncrementUsage("application/x-custom", "")
	rule = mgr.FindRule("application/x-custom", "")
	if rule.UsageCount != initialCount+1 {
		t.Errorf("usage count not incremented: expected %d, got %d", initialCount+1, rule.UsageCount)
	}

	// Test getting all rules
	allRules := mgr.GetAllRules()
	if len(allRules) < 2 {
		t.Errorf("expected at least 2 rules, got %d", len(allRules))
	}

	// Test deleting rule
	err = mgr.DeleteRule("application/x-custom", "")
	if err != nil {
		t.Fatalf("failed to delete rule: %v", err)
	}

	rule = mgr.FindRule("application/x-custom", "")
	if rule != nil {
		t.Error("rule should be deleted but still exists")
	}

	// Test persistence - create new manager and verify rules persist
	mgr2, err := NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second routing rules manager: %v", err)
	}

	rule = mgr2.FindRule("", ".xyz")
	if rule == nil {
		t.Error("rule should persist after manager recreation")
	}
}

func TestRoutingRulesManagerPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create first manager and add rules
	mgr1, err := NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create routing rules manager: %v", err)
	}

	err = mgr1.AddRule("application/test", ".test", []string{"test", "dir"})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	// Create second manager and verify rule persists
	mgr2, err := NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second routing rules manager: %v", err)
	}

	rule := mgr2.FindRule("application/test", ".test")
	if rule == nil {
		t.Fatal("rule should persist after manager recreation")
	}

	if len(rule.Destination) != 2 || rule.Destination[0] != "test" || rule.Destination[1] != "dir" {
		t.Errorf("unexpected destination: %v", rule.Destination)
	}
}

func TestRoutingRulesManagerIsRecognized(t *testing.T) {
	tmpDir := t.TempDir()
	
	mgr, err := NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create routing rules manager: %v", err)
	}

	classifier := NewClassifier()

	// Test unrecognized format
	if mgr.IsRecognized("application/unknown", "unknown.file", classifier) {
		t.Error("unknown format should not be recognized")
	}

	// Test recognized format (built-in)
	if !mgr.IsRecognized("image/jpeg", "test.jpg", classifier) {
		t.Error("JPEG should be recognized by built-in classifier")
	}

	// Add custom rule and test
	err = mgr.AddRule("application/unknown", ".unknown", []string{"files", "unknown"})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	if !mgr.IsRecognized("application/unknown", "test.unknown", classifier) {
		t.Error("unknown format should be recognized after adding custom rule")
	}
}

func TestRoutingRulesManagerEmptyDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create manager with non-existent rules file (should not error)
	mgr, err := NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create routing rules manager: %v", err)
	}

	// Should start with empty rules
	allRules := mgr.GetAllRules()
	if len(allRules) != 0 {
		t.Errorf("expected 0 rules initially, got %d", len(allRules))
	}
}

func TestRoutingRulesManagerInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "routing_rules.json")
	
	// Write invalid JSON
	err := os.WriteFile(rulesPath, []byte("invalid json"), 0o644)
	if err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	// Should return error when loading invalid JSON
	_, err = NewRoutingRulesManager(tmpDir)
	if err == nil {
		t.Error("expected error when loading invalid JSON")
	}
}

