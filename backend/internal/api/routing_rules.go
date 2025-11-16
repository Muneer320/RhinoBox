package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

type routingRuleRequest struct {
	MimeType    string   `json:"mime_type"`
	Extension   string   `json:"extension"`
	Destination []string `json:"destination"`
}

func (s *Server) handleSuggestRoutingRule(w http.ResponseWriter, r *http.Request) {
	s.writeRoutingRule(w, r, false)
}

func (s *Server) handleUpdateRoutingRule(w http.ResponseWriter, r *http.Request) {
	s.writeRoutingRule(w, r, true)
}

func (s *Server) writeRoutingRule(w http.ResponseWriter, r *http.Request, update bool) {
	var payload routingRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpError(w, http.StatusBadRequest, "invalid routing rule JSON")
		return
	}
	manager := s.storage.RoutingRules()
	if update && manager.FindRule(payload.MimeType, payload.Extension) == nil {
		httpError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err := manager.AddRule(payload.MimeType, payload.Extension, payload.Destination); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrInvalidRoutingRule) {
			status = http.StatusBadRequest
		}
		httpError(w, status, err.Error())
		return
	}
	message := "routing rule added successfully"
	if update {
		message = "routing rule updated successfully"
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": message})
}

func (s *Server) handleGetRoutingRules(w http.ResponseWriter, r *http.Request) {
	rules := s.storage.RoutingRules().GetAllRules()
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules, "count": len(rules)})
}

func (s *Server) handleDeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	var payload routingRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpError(w, http.StatusBadRequest, "invalid routing rule JSON")
		return
	}
	if payload.MimeType == "" && payload.Extension == "" {
		httpError(w, http.StatusBadRequest, "MIME type or extension is required")
		return
	}
	if err := s.storage.RoutingRules().DeleteRule(payload.MimeType, payload.Extension); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrRoutingRuleNotFound) {
			status = http.StatusNotFound
		}
		httpError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "routing rule deleted successfully"})
}
