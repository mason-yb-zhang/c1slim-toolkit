package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type errorTail struct {
	data []byte
}

func (b *errorTail) Write(p []byte) (int, error) {
	n := len(p)
	const limit = 4096
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
	} else {
		if len(b.data)+len(p) > limit {
			b.data = b.data[len(b.data)+len(p)-limit:]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

type progressWriter struct {
	pending string
	ready   bool
	events  chan playbackEvent
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.pending += string(p)
	for {
		i := strings.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(w.pending[:i])
		w.pending = w.pending[i+1:]
		if !w.ready && strings.HasPrefix(line, "out_time_us=") {
			t, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
			if err == nil && t > 0 {
				w.ready = true
				w.events <- playbackEvent{ready: true}
			}
		}
	}
	if len(w.pending) > 4096 {
		w.pending = ""
	}
	return len(p), nil
}

func processFailure(name string, err error, tail *errorTail) error {
	fmt.Fprintf(os.Stderr, "radio: %s: %v\n%s\n", name, err, bytes.TrimSpace(tail.data))
	code := -1
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	}
	if name != "ffmpeg" {
		return fmt.Errorf("连接失败 (%s %d)", name, code)
	}
	return fmt.Errorf("播放失败 (ffmpeg %d)", code)
}

func startPipeline(download, decoder *exec.Cmd, cancel func(), source string) (*playback, error) {
	r, w, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("启动失败: %w", err)
	}
	events := make(chan playbackEvent, 2)
	downloadLog, decoderLog := &errorTail{}, &errorTail{}
	download.Stdout, download.Stderr = w, downloadLog
	decoder.Stdin, decoder.Stderr = r, decoderLog
	decoder.Stdout = &progressWriter{events: events}
	if err := decoder.Start(); err != nil {
		r.Close()
		w.Close()
		cancel()
		fmt.Fprintln(os.Stderr, "radio: start ffmpeg:", err)
		return nil, fmt.Errorf("启动失败 (ffmpeg)")
	}
	if err := download.Start(); err != nil {
		r.Close()
		w.Close()
		cancel()
		_ = decoder.Wait()
		fmt.Fprintf(os.Stderr, "radio: start %s: %v\n", source, err)
		return nil, fmt.Errorf("启动失败 (%s)", source)
	}
	r.Close()
	w.Close()
	done := make(chan struct{})
	go func() {
		defer cancel()
		downloadDone, decoderDone := make(chan error, 1), make(chan error, 1)
		go func() { downloadDone <- download.Wait() }()
		go func() { decoderDone <- decoder.Wait() }()
		var failure error
		select {
		case err := <-downloadDone:
			if err != nil {
				cancel()
				failure = processFailure(source, err, downloadLog)
			}
			decodeErr := <-decoderDone
			if decodeErr != nil && failure == nil {
				failure = processFailure("ffmpeg", decodeErr, decoderLog)
			}
		case err := <-decoderDone:
			if err != nil {
				cancel()
				_ = <-downloadDone
				failure = processFailure("ffmpeg", err, decoderLog)
			} else {
				// EOF can precede curl's final exit status; do not kill it before collecting the error.
				if err := <-downloadDone; err != nil {
					failure = processFailure(source, err, downloadLog)
				}
			}
		}
		close(done)
		events <- playbackEvent{ended: true, err: failure}
	}()
	return &playback{events: events, stop: func() {
		cancel()
		<-done
	}}, nil
}
