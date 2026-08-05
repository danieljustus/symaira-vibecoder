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

// logRingCapacity bounds the in-memory log replay buffer. It matches
// ActivityStore.maxLines and the board's 500-line cap so a reconnecting client
// can always backfill to exactly what its own store would have kept.
const logRingCapacity = 500

// logRing is a bounded ring buffer holding the most recent log/error events
// published on the Bus. It exists purely for replay: reconnecting clients
// fetch it once (GET /api/logs) and merge by ts. It is memory-only — the run
// ledger stays the durable record and raw logs are never persisted.
type logRing struct {
	mu      sync.Mutex
	entries [logRingCapacity]Event
	head    int // index of the oldest entry; 0 while not full
	len     int
}

// push appends an event, evicting the oldest entry once the ring is full.
func (r *logRing) push(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.len == logRingCapacity {
		r.entries[r.head] = ev
		r.head = (r.head + 1) % logRingCapacity
		return
	}
	r.entries[(r.head+r.len)%logRingCapacity] = ev
	r.len++
}

// snapshot returns a copy of the buffered events in publish order (oldest
// first). The returned slice is detached from the ring.
func (r *logRing) snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, r.len)
	for i := 0; i < r.len; i++ {
		out[i] = r.entries[(r.head+i)%logRingCapacity]
	}
	return out
}

// Bus is a tiny in-process pub/sub. Each SSE client gets one subscription; slow
// subscribers drop events rather than blocking the engine. Alongside the
// subscriber fan-out it keeps a bounded replay buffer of log/error events.
type Bus struct {
	mu   sync.Mutex
	subs map[int]chan Event
	next int
	logs logRing
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

// Publish fans an event out to all subscribers (non-blocking per subscriber)
// and keeps a bounded replay of log/error events for reconnecting clients.
func (b *Bus) Publish(ev Event) {
	if ev.TS == 0 {
		ev.TS = nowMs()
	}
	// Only log/error events are worth replaying: run_state is re-primed on
	// every SSE connect and step_status is re-derived from the cycle.
	if ev.Type == "log" || ev.Type == "error" {
		b.logs.push(ev)
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

// Logs returns a copy of the bounded log/error replay buffer, oldest first.
// Reconnecting clients fetch this once (GET /api/logs) and merge by ts.
func (b *Bus) Logs() []Event { return b.logs.snapshot() }
