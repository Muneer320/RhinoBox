package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// RoutingRule represents a user-defined routing rule for a file type.
type RoutingRule struct {
	Extension   string   `json:"extension"`    // e.g., ".dwg", ".blend"
	MimeType    string   `json:"mime_type"`    // e.g., "application/x-dwg"
	Category    string   `json:"category"`     // e.g., "cad-files", "3d-models"
	Subcategory string   `json:"subcategory"`  // e.g., "autocad", "blender"
	Description string   `json:"description"`  // user-provided description
	CreatedAt   string   `json:"created_at"`   // timestamp when rule was added
}

// RoutingRulesManager handles persistence and retrieval of custom routing rules.
type RoutingRulesManager struct {
	filePath string
	rules    map[string]RoutingRule // key is extension or mime type
	mu       sync.RWMutex
}

// NewRoutingRulesManager creates a new routing rules manager.
func NewRoutingRulesManager(dataDir string) (*RoutingRulesManager, error) {
	rulesPath := filepath.Join(dataDir, "routing_rules.json")
	mgr := &RoutingRulesManager{
		filePath: rulesPath,
		rules:    make(map[string]RoutingRule),
	}

	// Load existing rules if file exists
	if _, err := os.Stat(rulesPath); err == nil {
		if err := mgr.load(); err != nil {
			return nil, err
		}
	}

	return mgr, nil
}

// AddRule adds or updates a routing rule.
func (m *RoutingRulesManager) AddRule(rule RoutingRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Normalize keys
	if rule.Extension != "" {
		ext := strings.ToLower(rule.Extension)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		m.rules[ext] = rule
	}
	if rule.MimeType != "" {
		m.rules[strings.ToLower(rule.MimeType)] = rule
	}

	return m.save()
}

// GetRuleByExtension retrieves a routing rule by file extension.
func (m *RoutingRulesManager) GetRuleByExtension(ext string) (RoutingRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	rule, ok := m.rules[ext]
	return rule, ok
}

// GetRuleByMimeType retrieves a routing rule by MIME type.
func (m *RoutingRulesManager) GetRuleByMimeType(mimeType string) (RoutingRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[strings.ToLower(mimeType)]
	return rule, ok
}

// ListRules returns all custom routing rules.
func (m *RoutingRulesManager) ListRules() []RoutingRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]RoutingRule, 0, len(m.rules))
	seen := make(map[string]bool) // deduplicate rules with same content

	for _, rule := range m.rules {
		key := rule.Extension + "|" + rule.MimeType + "|" + rule.Category
		if !seen[key] {
			rules = append(rules, rule)
			seen[key] = true
		}
	}

	return rules
}

// DeleteRule removes a routing rule by extension or MIME type.
func (m *RoutingRulesManager) DeleteRule(identifier string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	identifier = strings.ToLower(identifier)
	
	// Try as extension
	if strings.HasPrefix(identifier, ".") {
		delete(m.rules, identifier)
	} else {
		// Try as MIME type
		delete(m.rules, identifier)
		// Also try with dot prefix
		delete(m.rules, "."+identifier)
	}

	return m.save()
}

// load reads rules from the JSON file.
func (m *RoutingRulesManager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var rules []RoutingRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	m.rules = make(map[string]RoutingRule)
	for _, rule := range rules {
		if rule.Extension != "" {
			m.rules[strings.ToLower(rule.Extension)] = rule
		}
		if rule.MimeType != "" {
			m.rules[strings.ToLower(rule.MimeType)] = rule
		}
	}

	return nil
}

// save writes rules to the JSON file.
func (m *RoutingRulesManager) save() error {
	// Deduplicate rules for saving
	seen := make(map[string]RoutingRule)
	for _, rule := range m.rules {
		key := rule.Extension + "|" + rule.MimeType + "|" + rule.Category
		seen[key] = rule
	}

	rules := make([]RoutingRule, 0, len(seen))
	for _, rule := range seen {
		rules = append(rules, rule)
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0o644)
}
