package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Muneer320/RhinoBox/internal/queue"
	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// handleAsyncIngest queues a unified ingest job for async processing
func (s *Server) handleAsyncIngest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		httpError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	namespace := r.FormValue("namespace")
	comment := r.FormValue("comment")

	// Create job items from files
	items := []queue.JobItem{}
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, fileHeaders := range r.MultipartForm.File {
			for _, fh := range fileHeaders {
				items = append(items, queue.JobItem{
					ID:   uuid.NewString(),
					Type: "file",
					Name: fh.Filename,
					Size: fh.Size,
					Data: fh,
				})
			}
		}
	}

	if len(items) == 0 {
		httpError(w, http.StatusBadRequest, "no files provided")
		return
	}

	// Create job
	job := &queue.Job{
		ID:        uuid.NewString(),
		Type:      queue.JobTypeBatch,
		Items:     items,
		Namespace: namespace,
		Comment:   comment,
	}

	// Enqueue job
	if err := s.jobQueue.Enqueue(job); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to queue job: %v", err))
		return
	}

	// Return job ID immediately
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":           job.ID,
		"status":           string(job.Status),
		"total_items":      len(items),
		"check_status_url": fmt.Sprintf("/jobs/%s", job.ID),
		"created_at":       job.CreatedAt,
	})
}

// handleMediaIngestAsync queues media files for async processing
func (s *Server) handleMediaIngestAsync(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(512 << 20); err != nil { // 512MB max for large batches
		httpError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	category := r.FormValue("category")
	namespace := r.FormValue("namespace")

	items := []queue.JobItem{}
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, fileHeaders := range r.MultipartForm.File {
			for _, fh := range fileHeaders {
				items = append(items, queue.JobItem{
					ID:   uuid.NewString(),
					Type: "media",
					Name: fh.Filename,
					Size: fh.Size,
					Data: fh,
					Metadata: map[string]interface{}{
						"category": category,
					},
				})
			}
		}
	}

	if len(items) == 0 {
		httpError(w, http.StatusBadRequest, "no files provided")
		return
	}

	job := &queue.Job{
		ID:        uuid.NewString(),
		Type:      queue.JobTypeMedia,
		Items:     items,
		Namespace: namespace,
	}

	if err := s.jobQueue.Enqueue(job); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to queue job: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":           job.ID,
		"status":           string(job.Status),
		"total_items":      len(items),
		"check_status_url": fmt.Sprintf("/jobs/%s", job.ID),
		"created_at":       job.CreatedAt,
	})
}

// handleJSONIngestAsync queues JSON documents for async processing
func (s *Server) handleJSONIngestAsync(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Namespace string                   `json:"namespace"`
		Comment   string                   `json:"comment"`
		Documents []map[string]interface{} `json:"documents"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if len(payload.Documents) == 0 {
		httpError(w, http.StatusBadRequest, "no documents provided")
		return
	}

	items := make([]queue.JobItem, len(payload.Documents))
	for i, doc := range payload.Documents {
		items[i] = queue.JobItem{
			ID:   uuid.NewString(),
			Type: "json",
			Name: fmt.Sprintf("document_%d", i+1),
			Data: doc,
		}
	}

	job := &queue.Job{
		ID:        uuid.NewString(),
		Type:      queue.JobTypeJSON,
		Items:     items,
		Namespace: payload.Namespace,
		Comment:   payload.Comment,
	}

	if err := s.jobQueue.Enqueue(job); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to queue job: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":           job.ID,
		"status":           string(job.Status),
		"total_items":      len(items),
		"check_status_url": fmt.Sprintf("/jobs/%s", job.ID),
		"created_at":       job.CreatedAt,
	})
}

// handleJobStatus returns the current status of a job
func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")

	job, found := s.jobQueue.Get(jobID)
	if !found {
		httpError(w, http.StatusNotFound, "job not found")
		return
	}

	response := map[string]interface{}{
		"job_id":     job.ID,
		"type":       string(job.Type),
		"status":     string(job.Status),
		"progress":   job.Progress,
		"total":      job.Total,
		"created_at": job.CreatedAt,
	}

	if job.StartedAt != nil {
		response["started_at"] = job.StartedAt
	}
	if job.CompletedAt != nil {
		response["completed_at"] = job.CompletedAt
		if job.StartedAt != nil {
			response["duration_ms"] = job.CompletedAt.Sub(*job.StartedAt).Milliseconds()
		}
	}
	if job.Error != "" {
		response["error"] = job.Error
	}

	// Calculate progress percentage
	if job.Total > 0 {
		response["progress_pct"] = float64(job.Progress) / float64(job.Total) * 100
	}

		writeJSON(w, http.StatusOK, response)
}

// handleJobResult returns the detailed result of a completed job
func (s *Server) handleJobResult(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")

	result, found := s.jobQueue.GetResult(jobID)
	if !found {
		// Try getting job status instead
		job, jobFound := s.jobQueue.Get(jobID)
		if !jobFound {
			httpError(w, http.StatusNotFound, "job not found")
			return
		}

		if job.Status != queue.StatusCompleted && job.Status != queue.StatusFailed {
			httpError(w, http.StatusConflict, fmt.Sprintf("job not completed yet (status: %s)", job.Status))
			return
		}

		httpError(w, http.StatusNotFound, "job result not available")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleListJobs returns all active and recent jobs
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	jobs := s.jobQueue.ListJobs()

	// Limit results
	if len(jobs) > limit {
		jobs = jobs[:limit]
	}

	// Transform to response format
	response := make([]map[string]interface{}, len(jobs))
	for i, job := range jobs {
		response[i] = map[string]interface{}{
			"job_id":     job.ID,
			"type":       string(job.Type),
			"status":     string(job.Status),
			"progress":   job.Progress,
			"total":      job.Total,
			"created_at": job.CreatedAt,
		}
		if job.Total > 0 {
			response[i]["progress_pct"] = float64(job.Progress) / float64(job.Total) * 100
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  response,
		"count": len(response),
	})
}

// handleCancelJob attempts to cancel a queued or processing job
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")

	job, found := s.jobQueue.Get(jobID)
	if !found {
		httpError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status == queue.StatusCompleted || job.Status == queue.StatusFailed {
		httpError(w, http.StatusConflict, "cannot cancel completed job")
		return
	}

	// TODO: Implement actual cancellation logic
	// For now, just return the current status
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"job_id":  job.ID,
		"status":  string(job.Status),
		"message": "cancellation not yet implemented",
	})
}

// handleJobStats returns queue statistics
func (s *Server) handleJobStats(w http.ResponseWriter, r *http.Request) {
	stats := s.jobQueue.Stats()
	writeJSON(w, http.StatusOK, stats)
}
