package window

import "time"

const DefaultTargetFPS = 30

type framePacer struct {
	nextFrame time.Time
	targetFPS int
	interval  time.Duration
	started   bool
}

func newFramePacer() framePacer {
	var pacer framePacer
	pacer.setTargetFPS(DefaultTargetFPS)
	return pacer
}

func (p *framePacer) setTargetFPS(fps int) {
	if fps < 0 {
		fps = 0
	}
	p.targetFPS = fps
	p.started = false
	p.nextFrame = time.Time{}
	if fps == 0 {
		p.interval = 0
		return
	}
	p.interval = time.Second / time.Duration(fps)
}

func (p *framePacer) reset() {
	if p == nil {
		return
	}
	p.started = false
	p.nextFrame = time.Time{}
}

func (p *framePacer) wait() {
	if p == nil || p.interval <= 0 {
		return
	}
	now := time.Now()
	if !p.started {
		p.started = true
		p.nextFrame = now.Add(p.interval)
		return
	}
	if sleep := time.Until(p.nextFrame); sleep > 0 {
		time.Sleep(sleep)
		now = time.Now()
	} else {
		now = time.Now()
	}
	for !p.nextFrame.After(now) {
		p.nextFrame = p.nextFrame.Add(p.interval)
	}
}

func (p *framePacer) ready(now time.Time) bool {
	if p == nil || p.interval <= 0 {
		return true
	}
	if !p.started {
		p.started = true
		p.nextFrame = now.Add(p.interval)
		return true
	}
	if now.Before(p.nextFrame) {
		return false
	}
	for !p.nextFrame.After(now) {
		p.nextFrame = p.nextFrame.Add(p.interval)
	}
	return true
}
