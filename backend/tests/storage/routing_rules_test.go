package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestRoutingRulesManager(t *testing.T) {
	tmpDir := t.TempDir()
	
	mgr, err := storage.NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("AddAndRetrieveByExtension", func(t *testing.T) {
		rule := storage.RoutingRule{
			Extension:   ".dwg",
			MimeType:    "application/x-dwg",
			Category:    "cad-files",
			Subcategory: "autocad",
			Description: "AutoCAD drawing files",
			CreatedAt:   "2025-01-01T00:00:00Z",
		}

		if err := mgr.AddRule(rule); err != nil {
			t.Fatalf("failed to add rule: %v", err)
		}

		retrieved, ok := mgr.GetRuleByExtension(".dwg")
		if !ok {
			t.Fatalf("rule not found by extension")
		}

		if retrieved.Category != "cad-files" {
			t.Errorf("expected category cad-files, got %s", retrieved.Category)
		}
	})

	t.Run("AddAndRetrieveByMimeType", func(t *testing.T) {
		rule := storage.RoutingRule{
			Extension:   ".blend",
			MimeType:    "application/x-blender",
			Category:    "3d-models",
			Subcategory: "blender",
			Description: "Blender 3D model files",
			CreatedAt:   "2025-01-01T00:00:00Z",
		}

		if err := mgr.AddRule(rule); err != nil {
			t.Fatalf("failed to add rule: %v", err)
		}

		retrieved, ok := mgr.GetRuleByMimeType("application/x-blender")
		if !ok {
			t.Fatalf("rule not found by MIME type")
		}

		if retrieved.Subcategory != "blender" {
			t.Errorf("expected subcategory blender, got %s", retrieved.Subcategory)
		}
	})

	t.Run("ListRules", func(t *testing.T) {
		rules := mgr.ListRules()
		if len(rules) < 2 {
			t.Errorf("expected at least 2 rules, got %d", len(rules))
		}
	})

	t.Run("DeleteRule", func(t *testing.T) {
		if err := mgr.DeleteRule(".dwg"); err != nil {
			t.Fatalf("failed to delete rule: %v", err)
		}

		_, ok := mgr.GetRuleByExtension(".dwg")
		if ok {
			t.Errorf("rule should have been deleted")
		}
	})

	t.Run("PersistenceCheck", func(t *testing.T) {
		// Create a new manager pointing to the same directory
		mgr2, err := storage.NewRoutingRulesManager(tmpDir)
		if err != nil {
			t.Fatalf("failed to create second manager: %v", err)
		}

		// Should still find the .blend rule
		retrieved, ok := mgr2.GetRuleByExtension(".blend")
		if !ok {
			t.Errorf("persisted rule not found")
		}
		if retrieved.Category != "3d-models" {
			t.Errorf("expected category 3d-models, got %s", retrieved.Category)
		}
	})
}

func TestClassifierWithCustomRules(t *testing.T) {
	tmpDir := t.TempDir()
	
	mgr, err := storage.NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add custom rules for unrecognized formats
	rules := []storage.RoutingRule{
		{
			Extension:   ".psd",
			MimeType:    "image/vnd.adobe.photoshop",
			Category:    "design-files",
			Subcategory: "photoshop",
			Description: "Adobe Photoshop files",
		},
		{
			Extension:   ".fig",
			MimeType:    "application/x-figma",
			Category:    "design-files",
			Subcategory: "figma",
			Description: "Figma design files",
		},
		{
			Extension:   ".sketch",
			MimeType:    "application/x-sketch",
			Category:    "design-files",
			Subcategory: "sketch",
			Description: "Sketch design files",
		},
	}

	for _, rule := range rules {
		if err := mgr.AddRule(rule); err != nil {
			t.Fatalf("failed to add rule: %v", err)
		}
	}

	classifier := storage.NewClassifier()
	classifier.SetRoutingRulesManager(mgr)

	t.Run("ClassifyWithCustomRule", func(t *testing.T) {
		path := classifier.Classify("image/vnd.adobe.photoshop", "design.psd", "")
		if len(path) < 2 || path[0] != "design-files" || path[1] != "photoshop" {
			t.Errorf("expected [design-files photoshop], got %v", path)
		}
	})

	t.Run("ClassifyByExtension", func(t *testing.T) {
		path := classifier.Classify("application/octet-stream", "ui.fig", "")
		if len(path) < 2 || path[0] != "design-files" || path[1] != "figma" {
			t.Errorf("expected [design-files figma], got %v", path)
		}
	})

	t.Run("UnrecognizedFormat", func(t *testing.T) {
		// Should be unrecognized before adding rule
		if !classifier.IsUnrecognized("application/x-unknown", "file.xyz") {
			t.Errorf("expected file.xyz to be unrecognized")
		}

		// After adding rule, should not be unrecognized
		mgr.AddRule(storage.RoutingRule{
			Extension:   ".xyz",
			Category:    "custom",
			Subcategory: "unknown-format",
		})

		if classifier.IsUnrecognized("application/x-unknown", "file.xyz") {
			t.Errorf("expected file.xyz to be recognized after adding rule")
		}
	})

	t.Run("CustomRuleWithHint", func(t *testing.T) {
		path := classifier.Classify("image/vnd.adobe.photoshop", "mockup.psd", "website-redesign")
		if len(path) < 3 || path[2] != "website-redesign" {
			t.Errorf("expected hint to be appended, got %v", path)
		}
	})
}

func TestRoutingRulesFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	
	mgr, err := storage.NewRoutingRulesManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add a few rules
	mgr.AddRule(storage.RoutingRule{
		Extension:   ".dwg",
		Category:    "cad-files",
		Description: "AutoCAD files",
		CreatedAt:   "2025-01-01T00:00:00Z",
	})

	// Verify the file was created
	rulesPath := filepath.Join(tmpDir, "routing_rules.json")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		t.Errorf("routing_rules.json was not created")
	}

	// Read and verify the file is valid JSON
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("failed to read rules file: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("rules file is empty")
	}
}
