package main

import (
	"errors"
	"testing"
)

func TestPlayerFirstOKStartsAndSecondOKStops(t *testing.T) {
	for _, ready := range []bool{false, true} {
		name := "connecting"
		if ready {
			name = "ready"
		}
		t.Run(name, func(t *testing.T) {
			events := make(chan playbackEvent, 2)
			starts, stops := 0, 0
			active := &playback{events: events, stop: func() { stops++ }}
			var p player
			p = newPlayer(func(s station) (*playback, error) {
				starts++
				if s != stations[0] {
					t.Fatalf("started station = %v, want %v", s, stations[0])
				}
				if p.playing != 0 || p.note != "连接中…" {
					t.Fatalf("state during start = (%d, %q)", p.playing, p.note)
				}
				return active, nil
			})
			if p.playing != -1 || p.note != "已停止" || p.active != nil || p.events() != nil {
				t.Fatalf("initial state = %+v", p)
			}

			p.toggle(0)
			if starts != 1 || stops != 0 || p.active != active || p.events() != events {
				t.Fatalf("first OK did not start playback: starts=%d stops=%d state=%+v", starts, stops, p)
			}
			if p.playing != 0 || p.note != "连接中…" {
				t.Fatalf("state before ready = (%d, %q)", p.playing, p.note)
			}
			select {
			case ev := <-p.events():
				t.Fatalf("unexpected event before ready: %+v", ev)
			default:
			}
			if ready {
				events <- playbackEvent{ready: true}
				p.update(<-p.events())
				if p.playing != 0 || p.note != "" || p.active != active || stops != 0 {
					t.Fatalf("state after ready = %+v, stops=%d", p, stops)
				}
			}

			p.toggle(0)
			if starts != 1 || stops != 1 || p.active != nil || p.events() != nil {
				t.Fatalf("second OK did not stop playback: starts=%d stops=%d state=%+v", starts, stops, p)
			}
			if p.playing != -1 || p.note != "已停止" {
				t.Fatalf("state after stop = (%d, %q)", p.playing, p.note)
			}
			p.stop()
			if stops != 1 {
				t.Fatalf("idle stop called runner again: stops=%d", stops)
			}
		})
	}
}

func TestPlayerEndedReclaimsPlayback(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ready bool
		err   error
		note  string
	}{
		{"EOF before ready", false, nil, "已停止"},
		{"EOF after ready", true, nil, "已停止"},
		{"failure before ready", false, errors.New("连接失败"), "连接失败"},
		{"failure after ready", true, errors.New("解码失败"), "解码失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := make(chan playbackEvent, 2)
			stops := 0
			var p player
			active := &playback{events: events, stop: func() {
				stops++
				wantNote := "连接中…"
				if tc.ready {
					wantNote = ""
				}
				if p.active == nil || p.playing != 0 || p.note != wantNote {
					t.Fatalf("state changed before resource cleanup: %+v", p)
				}
			}}
			p = newPlayer(func(station) (*playback, error) { return active, nil })
			p.toggle(0)
			if tc.ready {
				events <- playbackEvent{ready: true}
				p.update(<-p.events())
			}
			events <- playbackEvent{ended: true, err: tc.err}
			p.update(<-p.events())
			if stops != 1 || p.active != nil || p.events() != nil || p.playing != -1 || p.note != tc.note {
				t.Fatalf("ended state = %+v, stops=%d, want note %q", p, stops, tc.note)
			}
		})
	}
}

func TestPlayerSwitchWaitsForStopAndIgnoresOldEvents(t *testing.T) {
	oldEvents := make(chan playbackEvent, 2)
	newEvents := make(chan playbackEvent, 2)
	stopStarted := make(chan struct{})
	cleanupDone := make(chan struct{})
	go func() {
		<-stopStarted
		close(cleanupDone)
	}()
	oldStops, newStops, starts := 0, 0, 0
	old := &playback{events: oldEvents, stop: func() {
		oldStops++
		close(stopStarted)
		<-cleanupDone
	}}
	next := &playback{events: newEvents, stop: func() { newStops++ }}
	p := newPlayer(func(s station) (*playback, error) {
		starts++
		if starts == 1 {
			return old, nil
		}
		select {
		case <-cleanupDone:
		default:
			t.Fatal("new playback started before old resource cleanup completed")
		}
		if oldStops != 1 || s != stations[1] {
			t.Fatalf("switch start: oldStops=%d station=%v", oldStops, s)
		}
		return next, nil
	})
	p.toggle(0)
	p.toggle(1)
	if starts != 2 || p.active != next || p.events() != newEvents || p.playing != 1 || p.note != "连接中…" {
		t.Fatalf("state after switch = %+v, starts=%d", p, starts)
	}

	oldEvents <- playbackEvent{ready: true}
	oldEvents <- playbackEvent{ended: true, err: errors.New("旧台失败")}
	select {
	case ev := <-p.events():
		t.Fatalf("selected an old event after switching: %+v", ev)
	default:
	}
	if p.playing != 1 || p.note != "连接中…" || p.active != next {
		t.Fatalf("old events changed new playback state: %+v", p)
	}
	newEvents <- playbackEvent{ready: true}
	p.update(<-p.events())
	if p.playing != 1 || p.note != "" || p.active != next || len(oldEvents) != 2 {
		t.Fatalf("new ready state = %+v, pending old events=%d", p, len(oldEvents))
	}
	p.stop()
	if oldStops != 1 || newStops != 1 {
		t.Fatalf("cleanup counts: old=%d new=%d", oldStops, newStops)
	}
}

func TestPlayerStartFailure(t *testing.T) {
	starts := 0
	p := newPlayer(func(station) (*playback, error) {
		starts++
		return nil, errors.New("启动失败")
	})
	for attempt := 1; attempt <= 2; attempt++ {
		p.toggle(0)
		if starts != attempt || p.playing != -1 || p.note != "启动失败" || p.active != nil || p.events() != nil {
			t.Fatalf("failed start attempt %d: starts=%d state=%+v", attempt, starts, p)
		}
	}
	p.stop()
	if p.note != "已停止" || p.playing != -1 {
		t.Fatalf("state after clearing failure = %+v", p)
	}
}
