package media

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Categorizer implements lightweight heuristics to group media by type and label.
// Thread-safe for concurrent use in worker pools.
type Categorizer struct{
	mu sync.RWMutex
}

func NewCategorizer() *Categorizer {
	return &Categorizer{}
}

// Classify returns the top-level media type directory and the inferred category label.
// Thread-safe for concurrent access.
func (c *Categorizer) Classify(mimeType, filename, hint string) (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	mediaType := classifyByMime(mimeType)
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	category := firstNonEmpty(sanitize(hint), sanitize(base), mediaType)
	return mediaType, category
}

func classifyByMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "images"
	case strings.HasPrefix(mime, "video/"):
		return "videos"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case strings.HasPrefix(mime, "text/"):
		return "documents"
	default:
		return inferFromExt(mime)
	}
}

func inferFromExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return "images"
	case ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return "videos"
	case ".mp3", ".wav", ".flac", ".aac":
		return "audio"
	default:
		return "other"
	}
}

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitize(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}
	lower = invalidChars.ReplaceAllString(lower, "-")
	return strings.Trim(lower, "-")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return "general"
}
