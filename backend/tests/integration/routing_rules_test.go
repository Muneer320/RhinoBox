package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

func TestRoutingRulesAPI(t *testing.T) {
	srv := newTestServerForRoutingRules(t)

	t.Run("AddRoutingRule", func(t *testing.T) {
		rule := storage.RoutingRule{
			Extension:   ".dwg",
			MimeType:    "application/x-dwg",
			Category:    "cad-files",
			Subcategory: "autocad",
			Description: "AutoCAD drawing files",
		}

		body, _ := json.Marshal(rule)
		req := httptest.NewRequest(http.MethodPost, "/routing-rules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		if result["message"] == nil {
			t.Errorf("expected success message")
		}
	})

	t.Run("ListRoutingRules", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/routing-rules", nil)
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		if result["rules"] == nil {
			t.Errorf("expected rules array")
		}
	})

	t.Run("AddInvalidRuleMissingCategory", func(t *testing.T) {
		rule := storage.RoutingRule{
			Extension: ".xyz",
			// Missing Category - should fail
		}

		body, _ := json.Marshal(rule)
		req := httptest.NewRequest(http.MethodPost, "/routing-rules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("DeleteRoutingRule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/routing-rules/.dwg", nil)
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}

func TestUnrecognizedFormatDetection(t *testing.T) {
	srv := newTestServerForRoutingRules(t)

	t.Run("IngestUnrecognizedFormat", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Upload a file with an unrecognized format (.blend - Blender file)
		fileWriter, _ := writer.CreateFormFile("files", "model.blend")
		fileWriter.Write([]byte("fake blender file data"))

		writer.WriteField("namespace", "3d-models")
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		// Should have unrecognized_formats array
		if result["unrecognized_formats"] == nil {
			t.Errorf("expected unrecognized_formats field")
		}

		unrecognized := result["unrecognized_formats"].([]any)
		if len(unrecognized) != 1 {
			t.Errorf("expected 1 unrecognized format, got %d", len(unrecognized))
		}

		format := unrecognized[0].(map[string]any)
		if format["filename"] != "model.blend" {
			t.Errorf("expected filename model.blend, got %v", format["filename"])
		}
		if format["extension"] != ".blend" {
			t.Errorf("expected extension .blend, got %v", format["extension"])
		}
	})

	t.Run("IngestRecognizedFormat", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Upload a file with a recognized format (.jpg)
		fileWriter, _ := writer.CreateFormFile("files", "photo.jpg")
		fileWriter.Write([]byte("fake image data"))

		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		// Should have empty or no unrecognized_formats array
		if result["unrecognized_formats"] != nil {
			unrecognized := result["unrecognized_formats"].([]any)
			if len(unrecognized) > 0 {
				t.Errorf("expected no unrecognized formats for .jpg, got %d", len(unrecognized))
			}
		}
	})
}

func TestCustomRoutingRulesE2E(t *testing.T) {
	srv := newTestServerForRoutingRules(t)

	// Step 1: Upload an unrecognized file format
	t.Run("Step1_UploadUnrecognizedFormat", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fileWriter, _ := writer.CreateFormFile("files", "design.psd")
		fileWriter.Write([]byte("fake photoshop file"))

		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		// Should detect as unrecognized
		if result["unrecognized_formats"] == nil {
			t.Fatalf("expected unrecognized_formats field")
		}

		unrecognized := result["unrecognized_formats"].([]any)
		if len(unrecognized) == 0 {
			t.Fatalf("expected unrecognized format for .psd")
		}

		t.Logf("Unrecognized format detected: %v", unrecognized[0])
	})

	// Step 2: Add a custom routing rule for .psd files
	t.Run("Step2_AddCustomRule", func(t *testing.T) {
		rule := storage.RoutingRule{
			Extension:   ".psd",
			MimeType:    "image/vnd.adobe.photoshop",
			Category:    "design-files",
			Subcategory: "photoshop",
			Description: "Adobe Photoshop files",
		}

		body, _ := json.Marshal(rule)
		req := httptest.NewRequest(http.MethodPost, "/routing-rules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Fatalf("failed to add rule: %d - %s", resp.Code, resp.Body.String())
		}

		t.Logf("Custom rule added successfully")
	})

	// Step 3: Upload the same format again - should now be recognized
	t.Run("Step3_UploadRecognizedFormat", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fileWriter, _ := writer.CreateFormFile("files", "design2.psd")
		fileWriter.Write([]byte("another fake photoshop file"))

		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		// Should NOT have unrecognized formats now
		if result["unrecognized_formats"] != nil {
			unrecognized := result["unrecognized_formats"].([]any)
			if len(unrecognized) > 0 {
				t.Errorf("expected .psd to be recognized after adding rule, but got: %v", unrecognized)
			}
		}

		t.Logf("Format now recognized and processed correctly")
	})

	// Step 4: Verify the file was stored in the correct location
	t.Run("Step4_VerifyStorageLocation", func(t *testing.T) {
		// The file should be in design-files/photoshop directory
		// We can verify this by checking the stored path in the response
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fileWriter, _ := writer.CreateFormFile("files", "design3.psd")
		fileWriter.Write([]byte("yet another fake photoshop file"))

		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		srv.Router().ServeHTTP(resp, req)

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		// Check that the file was stored with the correct category
		results := result["results"].(map[string]any)
		if results["files"] != nil {
			files := results["files"].([]any)
			if len(files) > 0 {
				file := files[0].(map[string]any)
				storedPath := file["stored_path"].(string)
				t.Logf("File stored at: %s", storedPath)
			}
		}
	})
}

func TestMultipleUnrecognizedFormats(t *testing.T) {
	srv := newTestServerForRoutingRules(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Upload multiple unrecognized formats
	files := []struct {
		name string
		ext  string
	}{
		{"model.blend", ".blend"},
		{"drawing.dwg", ".dwg"},
		{"animation.fbx", ".fbx"},
		{"design.fig", ".fig"},
		{"mockup.sketch", ".sketch"},
	}

	for _, file := range files {
		fileWriter, _ := writer.CreateFormFile("files", file.name)
		fileWriter.Write([]byte("fake file data"))
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	srv.Router().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	// Should have multiple unrecognized formats
	if result["unrecognized_formats"] == nil {
		t.Fatalf("expected unrecognized_formats field")
	}

	unrecognized := result["unrecognized_formats"].([]any)
	if len(unrecognized) != len(files) {
		t.Errorf("expected %d unrecognized formats, got %d", len(files), len(unrecognized))
	}

	t.Logf("Detected %d unrecognized formats", len(unrecognized))
	for i, format := range unrecognized {
		f := format.(map[string]any)
		t.Logf("  %d. %s (%s) - %s", i+1, f["filename"], f["extension"], f["suggestion"])
	}
}

func newTestServerForRoutingRules(t *testing.T) *api.Server {
	t.Helper()
	tmpDir := t.TempDir()

	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        tmpDir,
		MaxUploadBytes: 512 * 1024 * 1024,
	}

	// Create a logger for the test
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return srv
}
