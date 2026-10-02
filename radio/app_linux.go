//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"c1device"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--yunting-stream" {
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "radio: invalid streaming arguments")
			os.Exit(2)
		}
		debug.SetMemoryLimit(12 << 20)
		debug.SetGCPercent(50)
		if err := streamYunting(context.Background(), os.Args[2], os.Args[3], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "radio: Yunting:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "radio:", err)
		os.Exit(1)
	}
}

func run() error {
	debug.SetMemoryLimit(12 << 20)
	debug.SetGCPercent(50)

	face, err := c1device.NewBitmapFace(fontData, 16)
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	defer face.Close()

	restoreRefresh, err := saveRefreshMode()
	if err != nil {
		return err
	}
	defer restoreRefresh()
	platform, err := c1device.OpenPlatform()
	if err != nil {
		return fmt.Errorf("open platform: %w", err)
	}
	defer platform.Close()

	originalDAC, err := readDAC()
	if err != nil {
		return fmt.Errorf("read DAC: %w", err)
	}
	defer func() { _ = setDAC(originalDAC) }()
	volume := 32
	if err := setVolume(volume); err != nil {
		return err
	}

	p := newPlayer(startPlayback)
	defer p.stop()
	sel, first := 0, 0
	dirty := true
	last := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	draw := func() {
		frame := render(face, sel, first, volume, &p)
		if err := platform.Draw(frame, true); err != nil {
			fmt.Fprintln(os.Stderr, "radio: draw:", err)
		}
		dirty = false
		last = time.Now()
	}
	draw()

	for {
		select {
		case update := <-p.events():
			p.update(update)
			dirty = true
		case ev, ok := <-platform.Events():
			if !ok {
				return nil
			}
			switch ev.Key {
			case c1device.KeyUp:
				sel = clamp(sel-1, 0, len(stations)-1)
				if sel < first {
					first = sel
				}
			case c1device.KeyDown:
				sel = clamp(sel+1, 0, len(stations)-1)
				if sel >= first+listRows {
					first = sel - listRows + 1
				}
			case c1device.KeyOK:
				if !ev.Repeat {
					p.toggle(sel)
				}
			case c1device.KeyLeft, c1device.KeyVolumeDown:
				volume = clamp(volume-8, 0, 100)
				if err := setVolume(volume); err != nil {
					p.note = "音量设置失败"
				}
			case c1device.KeyRight, c1device.KeyVolumeUp:
				volume = clamp(volume+8, 0, 100)
				if err := setVolume(volume); err != nil {
					p.note = "音量设置失败"
				}
			case c1device.KeyBack:
				return nil
			case c1device.KeyRune:
				if ev.Rune == 'q' || ev.Rune == 'Q' {
					return nil
				}
			}
			dirty = true
		case <-ticker.C:
			if dirty || time.Since(last) > 2*time.Second {
				draw()
			}
		}
		if dirty {
			draw()
		}
	}
}
