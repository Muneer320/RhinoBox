package testfixtures

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Downloads replaces machine-specific personal Downloads inputs.
// Media fixtures exercise storage/MIME/range transport, not codec decoding.
func Downloads(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	canvas := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 160, A: 255})
		}
	}
	var pngBytes, jpgBytes bytes.Buffer
	if err := png.Encode(&pngBytes, canvas); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpgBytes, canvas, nil); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"sample-image.png": pngBytes.Bytes(), "sample-photo.jpg": jpgBytes.Bytes(),
		"sample-document.pdf": []byte("%PDF-1.4\n% synthetic storage transport fixture\n%%EOF\n"),
		"sample-video.mp4":    append([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'}, bytes.Repeat([]byte{0}, 256)...),
		"sample-audio.mp3":    append([]byte("ID3"), bytes.Repeat([]byte{0}, 256)...),
		"sample-data.json":    []byte(`[{"id":1,"name":"Demo One"},{"id":2,"name":"Demo Two"}]`),
		"sample-notes.txt":    []byte("Generated RhinoBox text fixture. A searchable storage example.\n"),
		"sample-table.csv":    []byte("id,name\n1,Demo One\n2,Demo Two\n"),
		"sample-guide.md":     []byte("# Generated demo\nRhinoBox fixture content for search and retrieval.\n"),
		"sample-script.py":    []byte("print('generated fixture')\n"),
		"sample-unknown.xyz":  []byte("generated unknown-format fixture\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
