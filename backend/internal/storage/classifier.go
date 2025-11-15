package storage

import (
    "path/filepath"
    "strings"
)

// Classifier groups files into the directory structure defined for RhinoBox storage.
type Classifier struct {
    mimeMap map[string][]string
    extMap  map[string][]string
}

func NewClassifier() *Classifier {
    c := &Classifier{
        mimeMap: map[string][]string{},
        extMap:  map[string][]string{},
    }

    // MIME mappings
    c.mimeMap = map[string][]string{
        // Images
        "image/jpeg":    {"images", "jpg"},
        "image/png":     {"images", "png"},
        "image/gif":     {"images", "gif"},
        "image/svg+xml": {"images", "svg"},
        "image/webp":    {"images", "webp"},
        "image/bmp":     {"images", "bmp"},

        // Videos
        "video/mp4":       {"videos", "mp4"},
        "video/avi":       {"videos", "avi"},
        "video/quicktime": {"videos", "mov"},
        "video/x-matroska": {"videos", "mkv"},
        "video/webm":      {"videos", "webm"},
        "video/x-flv":     {"videos", "flv"},

        // Audio
        "audio/mpeg": {"audio", "mp3"},
        "audio/wav":  {"audio", "wav"},
        "audio/flac": {"audio", "flac"},
        "audio/ogg":  {"audio", "ogg"},

        // Documents
        "application/pdf":                                                    {"documents", "pdf"},
        "application/msword":                                                {"documents", "doc"},
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document": {"documents", "docx"},
        "text/plain":                                                         {"documents", "txt"},
        "text/rtf":                                                           {"documents", "rtf"},
        "text/markdown":                                                      {"documents", "md"},
        "application/epub+zip":                                               {"documents", "epub"},
        "application/x-mobipocket-ebook":                                     {"documents", "mobi"},

        // Spreadsheets
        "application/vnd.ms-excel":                                      {"spreadsheets", "xls"},
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {"spreadsheets", "xlsx"},
        "text/csv":                                                     {"spreadsheets", "csv"},

        // Presentations
        "application/vnd.ms-powerpoint":                               {"presentations", "ppt"},
        "application/vnd.openxmlformats-officedocument.presentationml.presentation": {"presentations", "pptx"},

        // Archives
        "application/zip":   {"archives", "zip"},
        "application/x-tar": {"archives", "tar"},
        "application/gzip":  {"archives", "gz"},
        "application/x-rar-compressed": {"archives", "rar"},

        // Code
        "text/x-go":       {"code", "go"},
        "text/x-python":   {"code", "py"},
        "text/javascript": {"code", "js"},
        "text/x-java-source": {"code", "java"},
        "text/x-c++src":      {"code", "cpp"},
    }

    c.extMap = map[string][]string{
        // Images
        ".jpg":  {"images", "jpg"},
        ".jpeg": {"images", "jpg"},
        ".png":  {"images", "png"},
        ".gif":  {"images", "gif"},
        ".svg":  {"images", "svg"},
        ".bmp":  {"images", "bmp"},
        ".webp": {"images", "webp"},

        // Videos
        ".mp4":  {"videos", "mp4"},
        ".avi":  {"videos", "avi"},
        ".mov":  {"videos", "mov"},
        ".mkv":  {"videos", "mkv"},
        ".webm": {"videos", "webm"},
        ".flv":  {"videos", "flv"},

        // Audio
        ".mp3": {"audio", "mp3"},
        ".wav": {"audio", "wav"},
        ".flac": {"audio", "flac"},
        ".ogg": {"audio", "ogg"},

        // Documents
        ".pdf":  {"documents", "pdf"},
        ".doc":  {"documents", "doc"},
        ".docx": {"documents", "docx"},
        ".txt":  {"documents", "txt"},
        ".rtf":  {"documents", "rtf"},
        ".md":   {"documents", "md"},
        ".epub": {"documents", "epub"},
        ".mobi": {"documents", "mobi"},

        // Spreadsheets
        ".xls":  {"spreadsheets", "xls"},
        ".xlsx": {"spreadsheets", "xlsx"},
        ".csv":  {"spreadsheets", "csv"},

        // Presentations
        ".ppt":  {"presentations", "ppt"},
        ".pptx": {"presentations", "pptx"},

        // Archives
        ".zip": {"archives", "zip"},
        ".tar": {"archives", "tar"},
        ".gz":  {"archives", "gz"},
        ".rar": {"archives", "rar"},

        // Code
        ".py":   {"code", "py"},
        ".js":   {"code", "js"},
        ".go":   {"code", "go"},
        ".java": {"code", "java"},
        ".cpp":  {"code", "cpp"},
    }

    return c
}

func (c *Classifier) Classify(mimeType, filename, hint string) []string {
    if path, ok := c.mimeMap[mimeType]; ok {
        return appendPathWithHint(path, hint)
    }

    ext := strings.ToLower(filepath.Ext(filename))
    if path, ok := c.extMap[ext]; ok {
        return appendPathWithHint(path, hint)
    }

    return appendPathWithHint([]string{"other", "unknown"}, hint)
}

func appendPathWithHint(path []string, hint string) []string {
    sanitized := sanitize(hint)
    if sanitized == "" {
        return path
    }
    cp := make([]string, len(path), len(path)+1)
    copy(cp, path)
    cp = append(cp, sanitized)
    return cp
}
