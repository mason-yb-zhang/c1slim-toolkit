package main

import (
	"os"
	"testing"

	"c1device"
)

// TestPreview renders the UI to preview-*.frame for PC-side PNG conversion.
func TestPreview(t *testing.T) {
	face, err := c1device.NewBitmapFace(fontData, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()

	cases := []struct {
		name    string
		sel     int
		first   int
		volume  int
		playing int
		note    string
	}{
		{"idle", 0, 0, 32, -1, ""},
		{"playing", 1, 0, 64, 1, ""},
		{"scrolled", 7, 2, 80, 6, "连接中…"},
	}
	for _, tc := range cases {
		p := &player{playing: tc.playing, note: tc.note}
		frame := render(face, tc.sel, tc.first, tc.volume, p)
		if err := os.WriteFile("preview-"+tc.name+".frame", frame[:], 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
