package api

import (
	"net/http"
)

// AppConfig represents the application configuration response
type AppConfig struct {
	AuthEnabled bool               `json:"auth_enabled"`
	Version     string             `json:"version"`
	Features    map[string]bool    `json:"features"`
}

// handleConfig handles GET /api/config
// Returns the application configuration including auth status and feature flags.
// This endpoint is public (no auth required) to allow frontend to determine
// which features to display.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	config := AppConfig{
		AuthEnabled: s.cfg.AuthEnabled,
		Version:     "1.0.0", // TODO: Get from build info or version file
		Features: map[string]bool{
			"authentication":  s.cfg.AuthEnabled,
			"multi_tenant":    true, // Always enabled
			"async_ingestion": false, // TODO: Read from config when implemented
			"deduplication":   true,  // Always enabled
		},
	}

	writeJSON(w, http.StatusOK, config)
}


