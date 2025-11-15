package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Muneer320/RhinoBox/internal/duplicates"
)

// handleDuplicatesScan scans for duplicate files in the storage.
func (s *Server) handleDuplicatesScan(w http.ResponseWriter, r *http.Request) {
	var opts duplicates.ScanOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		// Use default options if body is empty or invalid
		opts = duplicates.ScanOptions{
			DeepScan:        false,
			IncludeMetadata: true,
		}
	}

	scanner := duplicates.NewScanner(s.storage)
	result, err := scanner.Scan(opts)
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleDuplicatesList returns a report of all duplicate groups.
func (s *Server) handleDuplicatesList(w http.ResponseWriter, r *http.Request) {
	scanner := duplicates.NewScanner(s.storage)
	groups, err := scanner.GetDuplicateGroups()
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("get duplicates failed: %v", err))
		return
	}

	response := map[string]any{
		"duplicate_groups": groups,
		"total_groups":     len(groups),
	}

	// Calculate totals
	totalDuplicates := 0
	totalWasted := int64(0)
	for _, group := range groups {
		totalDuplicates += group.Count - 1
		totalWasted += group.TotalWasted
	}
	response["total_duplicates"] = totalDuplicates
	response["storage_wasted"] = totalWasted

	writeJSON(w, http.StatusOK, response)
}

// handleDuplicatesVerify verifies the integrity of the storage system.
func (s *Server) handleDuplicatesVerify(w http.ResponseWriter, r *http.Request) {
	scanner := duplicates.NewScanner(s.storage)
	result, err := scanner.Verify()
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("verify failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleDuplicatesMerge merges duplicate files keeping only one copy.
func (s *Server) handleDuplicatesMerge(w http.ResponseWriter, r *http.Request) {
	var req duplicates.MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	if req.Hash == "" {
		httpError(w, http.StatusBadRequest, "hash is required")
		return
	}
	if req.Keep == "" {
		httpError(w, http.StatusBadRequest, "keep path is required")
		return
	}

	scanner := duplicates.NewScanner(s.storage)
	result, err := scanner.Merge(req)
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("merge failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}
