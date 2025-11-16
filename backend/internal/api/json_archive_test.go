package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJSONArchiveSupportsFileManagementAndRefreshesCounts(t *testing.T) {
	srv := newTestServer(t)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	// Prime collection statistics before a mutation to exercise cache invalidation.
	get("/collections/json/stats")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, newJSONRequest(t, "/ingest/json", map[string]any{"namespace": "orders", "documents": []map[string]any{{"id": 1}, {"id": 2}}}))
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Hash string `json:"archive_hash"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || len(result.Hash) != 64 {
		t.Fatalf("missing archive identity: %s, %v", w.Body.String(), err)
	}
	var listing struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(get("/files/type/json").Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if listing.Total != 1 || listing.Files[0].ID != result.Hash {
		t.Fatalf("archive was not indexed: %+v", listing)
	}
	if body := get("/files/download?hash=" + result.Hash).Body.String(); strings.Count(body, "\n") != 2 {
		t.Fatalf("expected two NDJSON documents, got %q", body)
	}
	var stats struct {
		Stats struct {
			Count int `json:"file_count"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(get("/collections/json/stats").Body.Bytes(), &stats); err != nil || stats.Stats.Count != 1 {
		t.Fatalf("stale collection count: %+v, %v", stats, err)
	}
	rename := httptest.NewRequest(http.MethodPatch, "/files/rename", strings.NewReader(`{"hash":"`+result.Hash+`","new_name":"orders.ndjson"}`))
	rename.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, rename)
	if w.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/files/"+result.Hash, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(get("/files/type/json").Body.Bytes(), &listing); err != nil || listing.Total != 0 {
		t.Fatalf("deleted archive still listed: %+v", listing)
	}
}

func TestSearchAppliesContentAndIncludesEntireEndDate(t *testing.T) {
	srv := newTestServer(t)
	uploadTestFile(t, srv, "first.txt", "contains needle", "text/plain")
	uploadTestFile(t, srv, "second.txt", "does not match", "text/plain")
	path := "/files/search?content_search=needle&date_to=" + time.Now().UTC().Format("2006-01-02")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("search: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Files []json.RawMessage `json:"results"`
		Count int               `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("content/date filters were not applied: %s", w.Body.String())
	}
}
