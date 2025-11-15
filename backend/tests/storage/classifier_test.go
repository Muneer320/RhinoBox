package storage_test

import (
    "reflect"
    "testing"

    "github.com/Muneer320/RhinoBox/internal/storage"
)

func TestClassifierMappings(t *testing.T) {
    c := storage.NewClassifier()

    cases := []struct {
        name     string
        mime     string
        filename string
        hint     string
        expected []string
    }{
        {"mimeImage", "image/jpeg", "photo.jpeg", "", []string{"images", "jpg"}},
        {"mimeVideo", "video/mp4", "clip.mov", "", []string{"videos", "mp4"}},
        {"extFallback", "", "archive.zip", "", []string{"archives", "zip"}},
        {"hintAppended", "image/png", "art.png", "vacation pics", []string{"images", "png", "vacation-pics"}},
        {"unknown", "application/octet-stream", "mystery.bin", "", []string{"other", "unknown"}},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            result := c.Classify(tc.mime, tc.filename, tc.hint)
            if !reflect.DeepEqual(result, tc.expected) {
                t.Fatalf("expected %v got %v", tc.expected, result)
            }
        })
    }
}
