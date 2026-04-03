package traffic

import (
	"context"
	"time"

	"proxy-center/internal/store"
)

type eventType int

const (
	eventDomain eventType = iota + 1
)

type event struct {
	kind   eventType
	userID int64
	domain string
	at     time.Time
}

type Recorder struct {
	store *store.Store
	ch    chan event
	stop  chan struct{}
}

func NewRecorder(st *store.Store, buffer int) *Recorder {
	if buffer <= 0 {
		buffer = 512
	}
	r := &Recorder{
		store: st,
		ch:    make(chan event, buffer),
		stop:  make(chan struct{}),
	}
	go r.loop()
	return r
}

func (r *Recorder) Close() {
	close(r.stop)
}

func (r *Recorder) RecordUsage(userID int64, bytes int64, at time.Time) {
	if userID <= 0 || bytes <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = r.store.AddUsage(ctx, userID, bytes, at)
}

func (r *Recorder) RecordDomain(userID int64, domain string, at time.Time) {
	if userID <= 0 || domain == "" {
		return
	}
	r.enqueue(event{kind: eventDomain, userID: userID, domain: domain, at: at})
}

func (r *Recorder) enqueue(ev event) {
	select {
	case r.ch <- ev:
	default:
		// Keep proxy path non-blocking under burst; dropped events are acceptable for v1.
	}
}

func (r *Recorder) loop() {
	for {
		select {
		case <-r.stop:
			return
		case ev := <-r.ch:
			r.handle(ev)
		}
	}
}

func (r *Recorder) handle(ev event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch ev.kind {
	case eventDomain:
		_ = r.store.LogDomain(ctx, ev.userID, ev.domain, ev.at)
	}
}
