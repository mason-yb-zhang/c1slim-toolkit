//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"syscall"
	"time"

	"c1device"
)

func (p *player) toggle(idx int) {
	if p.playing == idx {
		p.stop()
		p.note = "已停止"
		return
	}
	p.stop()
	p.playing = idx
	p.note = "连接中…"
	cmd := exec.Command("/bin/sh", "-c",
		"curl -s '"+stations[idx].url+"' | /storage/c1lavax/c1dec")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	log, err := os.OpenFile("/tmp/radio-native.log",
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		cmd.Stderr = log
		defer log.Close()
	}
	if err := cmd.Start(); err != nil {
		p.playing = -1
		p.note = "启动失败"
		return
	}
	p.cmd = cmd
	p.note = ""
	go func() {
		_ = cmd.Wait()
		if p.cmd == cmd {
			p.playing = -1
			p.note = "已停止"
		}
	}()
}

func (p *player) stop() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		_, _ = os.FindProcess(p.cmd.Process.Pid)
	}
	p.cmd = nil
	p.playing = -1
}

func setVolume(v int) {
	vol := v * 190 / 100
	if vol > 190 {
		vol = 190
	}
	_ = exec.Command("/usr/bin/amixer", "sset", "DAC",
		fmt.Sprintf("%d", vol)).Run()
}

func main() {
	debug.SetMemoryLimit(12 << 20)
	debug.SetGCPercent(50)

	face, err := c1device.NewBitmapFace(fontData, 16)
	if err != nil {
		fmt.Fprintln(os.Stderr, "radio: font:", err)
		os.Exit(1)
	}
	defer face.Close()

	platform, err := c1device.OpenPlatform()
	if err != nil {
		fmt.Fprintln(os.Stderr, "radio: open platform:", err)
		os.Exit(1)
	}
	defer platform.Close()

	volume := 32
	setVolume(volume)

	var p player
	sel, first := 0, 0
	dirty := true
	last := time.Now()

	draw := func() {
		frame := render(face, sel, first, volume, &p)
		_ = platform.Draw(frame, true)
		dirty = false
		last = time.Now()
	}
	draw()

	for {
		select {
		case ev, ok := <-platform.Events():
			if !ok {
				p.stop()
				return
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
				p.toggle(sel)
			case c1device.KeyLeft, c1device.KeyVolumeDown:
				volume = clamp(volume-8, 0, 100)
				setVolume(volume)
			case c1device.KeyRight, c1device.KeyVolumeUp:
				volume = clamp(volume+8, 0, 100)
				setVolume(volume)
			case c1device.KeyBack:
				p.stop()
				return
			case c1device.KeyRune:
				if ev.Rune == 'q' || ev.Rune == 'Q' {
					p.stop()
					return
				}
			}
			dirty = true
		case <-time.After(500 * time.Millisecond):
			// 播放状态可能由后台 goroutine 改变；低频刷新。
			if dirty || time.Since(last) > 2*time.Second {
				draw()
			}
		}
		if dirty {
			draw()
		}
	}
}
