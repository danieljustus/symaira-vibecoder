package engine

import (
	"sync"
	"time"
)

// Event is a server->board push message. Type drives the frontend reducer:
//
//	run_state   — the overall run changed (idle|running|paused); State set.
//	step_status — one step's status changed; StepID + Status set.
//	log         — a streamed runner line; StepID + Kind + Line set.
//	board       — the cycle structure changed (edit); board should refetch.
//	error       — a transport/engine error; Line set.
type Event struct {
	Type   string `json:"type"`
	RunID  string `json:"run_id,omitempty"`
	StepID string `json:"step_id,omitempty"`
	Status string `json:"status,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Line   string `json:"line,omitempty"`
	State  string `json:"state,omitempty"`
	TS     int64  `json:"ts"`
}

func nowMs() int64 { return time.Now().UnixMilli() }

// Bus is a tiny in-process pub/sub. Each SSE client gets one subscription; slow
// subscribers drop events rather than blocking the engine.
type Bus struct {
	mu   sync.Mutex
	subs map[int]chan Event
	next int
}

func NewBus() *Bus { return &Bus{subs: make(map[int]chan Event)} }

// Subscribe returns a subscription id and its receive channel.
func (b *Bus) Subscribe() (int, <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, 256)
	b.subs[id] = ch
	return id, ch
}

// Unsubscribe removes and closes a subscription.
func (b *Bus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		close(ch)
		delete(b.subs, id)
	}
}

// Publish fans an event out to all subscribers (non-blocking per subscriber).
func (b *Bus) Publish(ev Event) {
	if ev.TS == 0 {
		ev.TS = nowMs()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default: // drop for a slow/full subscriber; status is also persisted on disk
		}
	}
}
