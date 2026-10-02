package main

import (
	"os"
	"path/filepath"
	"testing"

	"c1device"
)

// TestPreview renders the UI to temporary preview-*.frame files.
func TestPreview(t *testing.T) {
	face, err := c1device.NewBitmapFace(fontData, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()

	dir := t.TempDir()
	cases := []struct {
		name   string
		sel    int
		first  int
		volume int
		player player
	}{
		{"initial", 0, 0, 32, newPlayer(nil)},
		{"idle", 0, 0, 32, player{playing: -1}},
		{"playing", 1, 0, 64, player{playing: 1}},
		{"scrolled", 7, 2, 80, player{playing: 6, note: "连接中…"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame := render(face, tc.sel, tc.first, tc.volume, &tc.player)
			path := filepath.Join(dir, "preview-"+tc.name+".frame")
			if err := os.WriteFile(path, frame[:], 0o644); err != nil {
				t.Fatal(err)
			}
		})
	}
}
