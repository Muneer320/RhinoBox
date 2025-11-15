package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RoutingRule represents a user-defined routing rule for unrecognized file formats.
type RoutingRule struct {
	MimeType    string   `json:"mime_type"`
	Extension   string   `json:"extension"`
	Destination []string `json:"destination"` // e.g., ["files", "custom", "type"]
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	UsageCount  int      `json:"usage_count"`
}

// RoutingRulesManager manages persistent storage of custom routing rules.
type RoutingRulesManager struct {
	rulesPath string
	rules     map[string]*RoutingRule // Key: mimeType or extension
	mu        sync.RWMutex
}

// NewRoutingRulesManager creates a new routing rules manager.
func NewRoutingRulesManager(dataDir string) (*RoutingRulesManager, error) {
	rulesPath := filepath.Join(dataDir, "routing_rules.json")
	
	manager := &RoutingRulesManager{
		rulesPath: rulesPath,
		rules:     make(map[string]*RoutingRule),
	}
	
	// Load existing rules
	if err := manager.load(); err != nil {
		// If file doesn't exist, that's okay - start with empty rules
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load routing rules: %w", err)
		}
	}
	
	return manager, nil
}

// load reads routing rules from the JSON file.
func (m *RoutingRulesManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	data, err := os.ReadFile(m.rulesPath)
	if err != nil {
		return err
	}
	
	var rules []RoutingRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("unmarshal routing rules: %w", err)
	}
	
	m.rules = make(map[string]*RoutingRule, len(rules))
	for i := range rules {
		rule := &rules[i]
		// Index by both MIME type and extension
		if rule.MimeType != "" {
			m.rules[rule.MimeType] = rule
		}
		if rule.Extension != "" {
			key := "ext:" + strings.ToLower(rule.Extension)
			m.rules[key] = rule
		}
	}
	
	return nil
}

// save writes routing rules to the JSON file.
func (m *RoutingRulesManager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Deduplicate rules (same rule might be indexed by both MIME and extension)
	ruleMap := make(map[string]*RoutingRule)
	for _, rule := range m.rules {
		key := rule.MimeType + "|" + rule.Extension
		if _, exists := ruleMap[key]; !exists {
			ruleMap[key] = rule
		}
	}
	
	rules := make([]RoutingRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, *rule)
	}
	
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal routing rules: %w", err)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.rulesPath), 0o755); err != nil {
		return fmt.Errorf("create rules directory: %w", err)
	}
	
	if err := os.WriteFile(m.rulesPath, data, 0o644); err != nil {
		return fmt.Errorf("write routing rules: %w", err)
	}
	
	return nil
}

// FindRule looks up a routing rule by MIME type or extension.
func (m *RoutingRulesManager) FindRule(mimeType, extension string) *RoutingRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Try MIME type first
	if mimeType != "" && mimeType != "application/octet-stream" {
		if rule, ok := m.rules[mimeType]; ok {
			return rule
		}
	}
	
	// Try extension
	if extension != "" {
		key := "ext:" + strings.ToLower(extension)
		if rule, ok := m.rules[key]; ok {
			return rule
		}
	}
	
	return nil
}

// AddRule adds or updates a routing rule.
func (m *RoutingRulesManager) AddRule(mimeType, extension string, destination []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now().UTC().Format(time.RFC3339)
	
	// Check if rule already exists
	var rule *RoutingRule
	if mimeType != "" {
		rule = m.rules[mimeType]
	}
	if rule == nil && extension != "" {
		key := "ext:" + strings.ToLower(extension)
		rule = m.rules[key]
	}
	
	if rule != nil {
		// Update existing rule
		rule.Destination = destination
		rule.UpdatedAt = now
	} else {
		// Create new rule
		rule = &RoutingRule{
			MimeType:    mimeType,
			Extension:   extension,
			Destination: destination,
			CreatedAt:   now,
			UpdatedAt:   now,
			UsageCount:  0,
		}
	}
	
	// Index by both MIME type and extension
	if mimeType != "" {
		m.rules[mimeType] = rule
	}
	if extension != "" {
		key := "ext:" + strings.ToLower(extension)
		m.rules[key] = rule
	}
	
	// Save to disk
	return m.save()
}

// IncrementUsage increments the usage count for a rule.
func (m *RoutingRulesManager) IncrementUsage(mimeType, extension string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var rule *RoutingRule
	if mimeType != "" {
		rule = m.rules[mimeType]
	}
	if rule == nil && extension != "" {
		key := "ext:" + strings.ToLower(extension)
		rule = m.rules[key]
	}
	
	if rule != nil {
		rule.UsageCount++
		rule.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		// Save periodically (every 10 uses) to avoid excessive writes
		if rule.UsageCount%10 == 0 {
			go m.save()
		}
	}
}

// GetAllRules returns all routing rules.
func (m *RoutingRulesManager) GetAllRules() []RoutingRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Deduplicate rules
	ruleMap := make(map[string]*RoutingRule)
	for _, rule := range m.rules {
		key := rule.MimeType + "|" + rule.Extension
		if _, exists := ruleMap[key]; !exists {
			ruleMap[key] = rule
		}
	}
	
	rules := make([]RoutingRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, *rule)
	}
	
	return rules
}

// DeleteRule removes a routing rule by MIME type or extension.
func (m *RoutingRulesManager) DeleteRule(mimeType, extension string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var rule *RoutingRule
	if mimeType != "" {
		rule = m.rules[mimeType]
		delete(m.rules, mimeType)
	}
	if extension != "" {
		key := "ext:" + strings.ToLower(extension)
		if rule == nil {
			rule = m.rules[key]
		}
		delete(m.rules, key)
	}
	
	if rule == nil {
		return fmt.Errorf("rule not found")
	}
	
	// Also delete the other index if it exists
	if mimeType != "" && extension != "" {
		key := "ext:" + strings.ToLower(extension)
		delete(m.rules, key)
	}
	if extension != "" && mimeType != "" {
		delete(m.rules, mimeType)
	}
	
	return m.save()
}

// IsRecognized checks if a file format is recognized (either by classifier or custom rules).
func (m *RoutingRulesManager) IsRecognized(mimeType, extension string, classifier *Classifier) bool {
	// Check if classifier recognizes it
	if classifier != nil {
		path := classifier.Classify(mimeType, "test"+extension, "")
		// If path is not "other/unknown", it's recognized
		if len(path) >= 2 && !(path[0] == "other" && path[1] == "unknown") {
			return true
		}
	}
	
	// Check custom rules
	return m.FindRule(mimeType, extension) != nil
}

