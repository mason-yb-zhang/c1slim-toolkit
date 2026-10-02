//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func startPlayback(s station) (*playback, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("启动失败 (路径)")
	}
	ca := filepath.Join(filepath.Dir(exe), "..", "assets", "cacert.pem")
	if _, err := os.Stat(ca); err != nil {
		fmt.Fprintln(os.Stderr, "radio: CA bundle:", err)
		return nil, fmt.Errorf("连接失败 (CA)")
	}
	redirectProtocols := "=http,https"
	if strings.HasPrefix(s.url, "https://") {
		redirectProtocols = "=https"
	}
	ctx, cancel := context.WithCancel(context.Background())
	source := "curl"
	var download *exec.Cmd
	if s.broadcastID != "" {
		source = "云听"
		download = exec.CommandContext(ctx, exe, "--yunting-stream", s.broadcastID, ca)
	} else {
		download = exec.CommandContext(ctx, "/usr/bin/curl",
			"--fail", "--silent", "--show-error", "--location",
			"--proto", "=http,https", "--proto-redir", redirectProtocols,
			"--connect-timeout", "10", "--speed-limit", "1", "--speed-time", "15",
			"--max-redirs", "3", "--cacert", ca, "--", s.url)
	}
	decoder := exec.CommandContext(ctx, "/usr/bin/ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error", "-nostats",
		"-progress", "pipe:1", "-stats_period", "0.5",
		"-analyzeduration", "1000000", "-probesize", "32768",
		"-protocol_whitelist", "pipe", "-i", "pipe:0",
		"-vn", "-ac", "2", "-ar", "44100", "-f", "alsa", "default")
	return startPipeline(download, decoder, cancel, source)
}

func readDAC() (int, error) {
	out, err := exec.Command("/usr/bin/amixer", "sget", "DAC").Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "Mono:" && fields[1] == "Playback" {
			return strconv.Atoi(fields[2])
		}
	}
	return 0, fmt.Errorf("missing DAC playback level")
}

func setDAC(v int) error {
	out, err := exec.Command("/usr/bin/amixer", "sset", "DAC", strconv.Itoa(v)).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "radio: DAC: %v: %s\n", err, out)
	}
	return err
}

func setVolume(v int) error {
	return setDAC(dacLevel(v))
}

func saveRefreshMode() (func(), error) {
	const path = "/sys/devices/platform/e0266a128/epaper/fast_refresh_only"
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read refresh mode: %w", err)
	}
	return func() {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err == nil {
			_, err = f.Write(value)
			_ = f.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "radio: restore refresh mode:", err)
		}
	}, nil
}
