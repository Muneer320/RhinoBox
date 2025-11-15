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
	case strings.Contains(mime, "pdf"), 
		 strings.Contains(mime, "msword"),
		 strings.Contains(mime, "wordprocessingml"),
		 strings.Contains(mime, "spreadsheet"),
		 strings.Contains(mime, "presentation"),
		 strings.Contains(mime, "opendocument"):
		return "documents"
	case strings.Contains(mime, "zip"),
		 strings.Contains(mime, "compressed"),
		 strings.Contains(mime, "archive"),
		 mime == "application/x-rar-compressed",
		 mime == "application/x-7z-compressed",
		 mime == "application/x-tar",
		 mime == "application/gzip",
		 mime == "application/x-bzip2":
		return "archives"
	default:
		return inferFromExt(mime)
	}
}

func inferFromExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	// Image formats
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".tif", 
		 ".heic", ".heif", ".avif", ".jfif", ".pjpeg", ".pjp", ".apng", ".raw", ".cr2", 
		 ".nef", ".orf", ".sr2", ".dng":
		return "images"
	// Video formats
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv", ".m4v", ".mpg", ".mpeg", 
		 ".3gp", ".3g2", ".ogv", ".ts", ".mts", ".m2ts", ".vob", ".divx", ".xvid", ".f4v":
		return "videos"
	// Audio formats
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".oga", ".opus", ".m4a", ".wma", ".aiff", 
		 ".aif", ".ape", ".alac", ".amr", ".mid", ".midi", ".ra", ".rm":
		return "audio"
	// Document formats
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp",
		 ".rtf", ".tex", ".txt", ".md", ".csv", ".tsv", ".epub", ".mobi":
		return "documents"
	// Archive formats
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".iso", ".dmg", ".pkg":
		return "archives"
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
