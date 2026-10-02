package main

type playbackEvent struct {
	ready bool
	ended bool
	err   error
}

type playback struct {
	events <-chan playbackEvent
	stop   func()
}

type player struct {
	playing int
	note    string
	active  *playback
	start   func(station) (*playback, error)
}

func newPlayer(start func(station) (*playback, error)) player {
	return player{playing: -1, note: "已停止", start: start}
}

func (p *player) toggle(idx int) {
	if p.active != nil && p.playing == idx {
		p.stop()
		return
	}
	p.stop()
	p.playing = idx
	p.note = "连接中…"
	active, err := p.start(stations[idx])
	if err != nil {
		p.playing = -1
		p.note = err.Error()
		return
	}
	p.active = active
}

func (p *player) stop() {
	if p.active != nil {
		p.active.stop()
	}
	p.active = nil
	p.playing = -1
	p.note = "已停止"
}

func (p *player) events() <-chan playbackEvent {
	if p.active == nil {
		return nil
	}
	return p.active.events
}

func (p *player) update(ev playbackEvent) {
	if ev.ready {
		p.note = ""
	}
	if ev.ended {
		p.stop()
		if ev.err != nil {
			p.note = ev.err.Error()
		}
	}
}
