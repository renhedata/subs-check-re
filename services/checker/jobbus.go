package checker

import (
	"context"
	"sync"
)

// JobBus routes per-job progress events to subscribers and tracks cancellation
// hooks so any caller can stop a running job by its jobID.
//
// The default adapter is in-process ([inProcessJobBus]) — a map of channels +
// a map of cancel funcs guarded by a mutex. That assumes a single backend
// instance: in a multi-replica deployment the SSE subscriber and the running
// job goroutine must live on the same process for events to flow. To support
// horizontal scaling, swap in a Redis-backed adapter (publish to a Pub/Sub
// channel keyed by jobID, cancel via a distributed lock or signal channel).
type JobBus interface {
	Subscribe(jobID string) chan progressUpdate
	Unsubscribe(jobID string, ch chan progressUpdate)
	Publish(jobID string, update progressUpdate)
	Close(jobID string)
	StoreCancel(jobID string, cancel context.CancelFunc)
	TriggerCancel(jobID string)
	RemoveCancel(jobID string)
}

type inProcessJobBus struct {
	progressMu sync.Mutex
	channels   map[string][]chan progressUpdate

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

func newInProcessJobBus() *inProcessJobBus {
	return &inProcessJobBus{
		channels: make(map[string][]chan progressUpdate),
		cancels:  make(map[string]context.CancelFunc),
	}
}

func (b *inProcessJobBus) Subscribe(jobID string) chan progressUpdate {
	ch := make(chan progressUpdate, 100)
	b.progressMu.Lock()
	b.channels[jobID] = append(b.channels[jobID], ch)
	b.progressMu.Unlock()
	return ch
}

func (b *inProcessJobBus) Unsubscribe(jobID string, ch chan progressUpdate) {
	b.progressMu.Lock()
	defer b.progressMu.Unlock()
	channels := b.channels[jobID]
	for i, c := range channels {
		if c == ch {
			b.channels[jobID] = append(channels[:i], channels[i+1:]...)
			return
		}
	}
}

func (b *inProcessJobBus) Publish(jobID string, update progressUpdate) {
	b.progressMu.Lock()
	defer b.progressMu.Unlock()
	for _, ch := range b.channels[jobID] {
		select {
		case ch <- update:
		default:
		}
	}
}

func (b *inProcessJobBus) Close(jobID string) {
	b.progressMu.Lock()
	defer b.progressMu.Unlock()
	for _, ch := range b.channels[jobID] {
		close(ch)
	}
	delete(b.channels, jobID)
}

func (b *inProcessJobBus) StoreCancel(jobID string, cancel context.CancelFunc) {
	b.cancelMu.Lock()
	b.cancels[jobID] = cancel
	b.cancelMu.Unlock()
}

func (b *inProcessJobBus) TriggerCancel(jobID string) {
	b.cancelMu.Lock()
	if fn, ok := b.cancels[jobID]; ok {
		fn()
		delete(b.cancels, jobID)
	}
	b.cancelMu.Unlock()
}

func (b *inProcessJobBus) RemoveCancel(jobID string) {
	b.cancelMu.Lock()
	delete(b.cancels, jobID)
	b.cancelMu.Unlock()
}

// defaultJobBus is the package-level bus used by runJob, GetProgress and CancelCheck.
// Tests can swap this for a stub bus to assert progress emission without spawning real jobs.
var defaultJobBus JobBus = newInProcessJobBus()
