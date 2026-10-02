package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

const pipelineTestTimeout = 5 * time.Second

func TestPipelineHelper(t *testing.T) {
	mode := os.Getenv("C1RADIO_PIPELINE_TEST_HELPER")
	if mode == "" {
		return
	}
	switch mode {
	case "download":
		fmt.Fprint(os.Stdout, "audio bytes")
	case "download-fail":
		fmt.Fprintln(os.Stderr, "unable to get local issuer certificate")
		os.Exit(60)
	case "download-fail-after-eof":
		fmt.Fprintln(os.Stderr, "unable to get local issuer certificate")
		os.Stdout.Close()
		time.Sleep(200 * time.Millisecond)
		os.Exit(60)
	case "decoder":
		io.Copy(io.Discard, os.Stdin)
		fmt.Fprintln(os.Stdout, "out_time_us=100")
	case "decoder-zero":
		io.Copy(io.Discard, os.Stdin)
		fmt.Fprintln(os.Stdout, "out_time_us=0")
	case "decoder-drain":
		io.Copy(io.Discard, os.Stdin)
	case "decoder-fail":
		time.Sleep(50 * time.Millisecond)
		fmt.Fprintln(os.Stderr, "decoder rejected input")
		os.Exit(23)
	case "wait":
		for {
			time.Sleep(time.Second)
		}
	default:
		os.Exit(99)
	}
	os.Exit(0)
}

func pipelineHelperCommand(ctx context.Context, mode string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPipelineHelper$")
	cmd.Env = append(os.Environ(), "C1RADIO_PIPELINE_TEST_HELPER="+mode)
	return cmd
}

func testPipeline(t *testing.T, downloadMode, decoderMode string) (*playback, *exec.Cmd, *exec.Cmd) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pipelineTestTimeout)
	t.Cleanup(cancel)
	download := pipelineHelperCommand(ctx, downloadMode)
	decoder := pipelineHelperCommand(ctx, decoderMode)
	p, err := startPipeline(download, decoder, cancel, "curl")
	if err != nil {
		t.Fatalf("startPipeline: %v", err)
	}
	t.Cleanup(func() { stopPipelineWithin(t, p) })
	return p, download, decoder
}

func stopPipelineWithin(t *testing.T, p *playback) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		p.stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(pipelineTestTimeout):
		t.Fatal("stop did not complete within timeout")
	}
}

func pipelineEventsUntilEnd(t *testing.T, p *playback) []playbackEvent {
	t.Helper()
	deadline := time.NewTimer(pipelineTestTimeout)
	defer deadline.Stop()
	var events []playbackEvent
	for {
		select {
		case ev, ok := <-p.events:
			if !ok {
				t.Fatal("events closed without ended event")
			}
			events = append(events, ev)
			if ev.ended {
				return events
			}
		case <-deadline.C:
			t.Fatal("pipeline did not end within timeout")
		}
	}
}

func assertPipelineWaited(t *testing.T, download, decoder *exec.Cmd) {
	t.Helper()
	for name, cmd := range map[string]*exec.Cmd{"download": download, "decoder": decoder} {
		if cmd.ProcessState == nil {
			t.Errorf("%s has not completed Wait", name)
		}
	}
}

func TestPipelineDownloadFailure(t *testing.T) {
	for _, mode := range []string{"download-fail", "download-fail-after-eof"} {
		t.Run(mode, func(t *testing.T) {
			p, download, decoder := testPipeline(t, mode, "decoder-drain")
			events := pipelineEventsUntilEnd(t, p)
			stopPipelineWithin(t, p)
			assertPipelineWaited(t, download, decoder)
			last := events[len(events)-1]
			if last.err == nil || !strings.Contains(last.err.Error(), "curl 60") {
				t.Fatalf("expected curl 60, got %+v", last)
			}
			for _, ev := range events {
				if ev.ready {
					t.Error("failed download must not become ready")
				}
			}
		})
	}
}

func TestPipelineDecoderFailureStopsDownload(t *testing.T) {
	started := time.Now()
	p, download, decoder := testPipeline(t, "wait", "decoder-fail")
	events := pipelineEventsUntilEnd(t, p)
	stopPipelineWithin(t, p)
	assertPipelineWaited(t, download, decoder)
	last := events[len(events)-1]
	if last.err == nil || !strings.Contains(last.err.Error(), "ffmpeg 23") {
		t.Fatalf("expected decoder error 23, got %+v", last)
	}
	if time.Since(started) >= 4*time.Second {
		t.Error("decoder failure relied on context deadline to stop download")
	}
}

func TestPipelineReadyThenEnded(t *testing.T) {
	p, download, decoder := testPipeline(t, "download", "decoder")
	events := pipelineEventsUntilEnd(t, p)
	stopPipelineWithin(t, p)
	assertPipelineWaited(t, download, decoder)
	if len(events) != 2 || !events[0].ready || events[0].ended || !events[1].ended || events[1].ready || events[1].err != nil {
		t.Fatalf("want ready then successful ended, got %+v", events)
	}
}

func TestPipelineZeroProgressNeverReady(t *testing.T) {
	p, download, decoder := testPipeline(t, "download", "decoder-zero")
	events := pipelineEventsUntilEnd(t, p)
	stopPipelineWithin(t, p)
	assertPipelineWaited(t, download, decoder)
	if len(events) != 1 || !events[0].ended || events[0].ready {
		t.Fatalf("zero progress must produce only ended, got %+v", events)
	}
}

func TestPipelineStopWaitsAndIsIdempotent(t *testing.T) {
	for i := 0; i < 4; i++ {
		p, download, decoder := testPipeline(t, "wait", "wait")
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.stop()
			}()
		}
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(pipelineTestTimeout):
			t.Fatal("concurrent stop calls deadlocked")
		}
		assertPipelineWaited(t, download, decoder)
		stopPipelineWithin(t, p)
		pipelineEventsUntilEnd(t, p)
	}
}

func TestErrorTailRetainsLast4096Bytes(t *testing.T) {
	var tail errorTail
	var all []byte
	for _, chunk := range [][]byte{
		[]byte("first"), bytes.Repeat([]byte("a"), 4090), []byte("boundary"),
		bytes.Repeat([]byte("bc"), 5000), []byte("last"), nil,
	} {
		n, err := tail.Write(chunk)
		if err != nil || n != len(chunk) {
			t.Fatalf("Write returned %d, %v", n, err)
		}
		all = append(all, chunk...)
		want := all
		if len(want) > 4096 {
			want = want[len(want)-4096:]
		}
		if !bytes.Equal(tail.data, want) {
			t.Fatalf("tail mismatch: got %d bytes, want %d", len(tail.data), len(want))
		}
	}
}

func TestProgressWriterSplitAndInvalidValues(t *testing.T) {
	events := make(chan playbackEvent, 4)
	w := &progressWriter{events: events}
	for _, chunk := range []string{
		"out_time_us=0\nout_time_us=-1\nout_time_us=N/A\n",
		"out_time_us=9223372036854775808\nout_time_us=1.5\n",
		"other_out_time_us=100\nout_ti", "me_us=1", "00",
	} {
		n, err := w.Write([]byte(chunk))
		if err != nil || n != len(chunk) {
			t.Fatalf("Write returned %d, %v", n, err)
		}
		select {
		case ev := <-events:
			t.Fatalf("premature ready from invalid or incomplete line: %+v", ev)
		default:
		}
	}
	w.Write([]byte("\r\n"))
	select {
	case ev := <-events:
		if !ev.ready || ev.ended || ev.err != nil {
			t.Fatalf("unexpected event: %+v", ev)
		}
	default:
		t.Fatal("complete positive progress did not produce ready")
	}
	w.Write([]byte("out_time_us=200\nout_time_us=300\n"))
	select {
	case ev := <-events:
		t.Fatalf("duplicate progress event: %+v", ev)
	default:
	}
}

func TestProgressWriterBoundsIncompleteLine(t *testing.T) {
	events := make(chan playbackEvent, 2)
	w := &progressWriter{events: events}
	w.Write(bytes.Repeat([]byte("x"), 4097))
	if len(w.pending) > 4096 {
		t.Fatalf("pending line grew to %d bytes", len(w.pending))
	}
	w.Write([]byte("\nout_time_us=100\n"))
	select {
	case ev := <-events:
		if !ev.ready {
			t.Fatalf("unexpected event after oversized line: %+v", ev)
		}
	default:
		t.Fatal("parser did not recover after oversized line")
	}
}
