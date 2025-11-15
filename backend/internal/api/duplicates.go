package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// handleDuplicateScan initiates a scan for duplicate files
func (s *Server) handleDuplicateScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req storage.DuplicateScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty, use defaults
		if err.Error() != "EOF" {
			httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
		req = storage.DuplicateScanRequest{
			DeepScan:       false,
			IncludeMetadata: true,
		}
	}

	result, err := s.storage.ScanForDuplicates(req)
	if err != nil {
		if errors.Is(err, storage.ErrScanInProgress) {
			httpError(w, http.StatusConflict, err.Error())
			return
		}
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
		return
	}

	s.logger.Info("duplicate scan completed",
		"scan_id", result.ScanID,
		"total_files", result.TotalFiles,
		"duplicates_found", result.DuplicatesFound,
		"storage_wasted", result.StorageWasted,
	)

	writeJSON(w, http.StatusOK, result)
}

// handleGetDuplicates returns the current duplicate report
func (s *Server) handleGetDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	groups, err := s.storage.GetDuplicateReport()
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get duplicates: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"duplicate_groups": groups,
		"count":            len(groups),
	})
}

// handleVerifyDuplicates verifies the deduplication system integrity
func (s *Server) handleVerifyDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result, err := s.storage.VerifyDeduplicationSystem()
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("verification failed: %v", err))
		return
	}

	s.logger.Info("duplicate verification completed",
		"metadata_count", result.MetadataIndexCount,
		"physical_count", result.PhysicalFilesCount,
		"orphaned", result.OrphanedFiles,
		"missing", result.MissingFiles,
		"hash_mismatches", result.HashMismatches,
	)

	writeJSON(w, http.StatusOK, result)
}

// handleMergeDuplicates merges duplicate files
func (s *Server) handleMergeDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req storage.MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	result, err := s.storage.MergeDuplicates(req)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidMergeRequest):
			httpError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, storage.ErrDuplicateNotFound):
			httpError(w, http.StatusNotFound, err.Error())
		default:
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("merge failed: %v", err))
		}
		return
	}

	s.logger.Info("duplicates merged",
		"hash", result.Hash,
		"kept", result.Kept,
		"removed_count", len(result.Removed),
		"space_reclaimed", result.SpaceReclaimed,
	)

	writeJSON(w, http.StatusOK, result)
}

// handleDuplicateStatistics returns statistics about duplicates
func (s *Server) handleDuplicateStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats, err := s.storage.GetDuplicateStatistics()
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get statistics: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

