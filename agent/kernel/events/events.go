package events

import (
	"context"
	"sync"
	"time"
)

type Event struct {
	Domain       string         `json:"domain"`
	Type         string         `json:"type"`
	RunID        string         `json:"run_id,omitempty"`
	TaskID       string         `json:"task_id,omitempty"`
	RequestID    string         `json:"request_id,omitempty"`
	ConnectionID string         `json:"connection_id,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	Seq          uint64         `json:"seq"`
	Timestamp    time.Time      `json:"timestamp"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type Sink interface {
	Append(context.Context, Event) error
	List() []Event
}

type InMemorySink struct {
	mu     sync.RWMutex
	events []Event
}

func NewInMemorySink() *InMemorySink { return &InMemorySink{} }

func (s *InMemorySink) Append(_ context.Context, e Event) error {
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
	return nil
}

func (s *InMemorySink) List() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

type Bus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
	seq  map[string]uint64
	sink Sink
}

func NewBus(sink Sink) *Bus {
	if sink == nil {
		sink = NewInMemorySink()
	}
	return &Bus{subs: map[chan Event]struct{}{}, seq: map[string]uint64{}, sink: sink}
}

func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan Event, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	cancel := func() {
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}
	return ch, cancel
}

func (b *Bus) Publish(ctx context.Context, e Event) error {
	b.mu.Lock()
	if e.RunID != "" {
		b.seq[e.RunID]++
		e.Seq = b.seq[e.RunID]
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	subs := make([]chan Event, 0, len(b.subs))
	for ch := range b.subs {
		subs = append(subs, ch)
	}
	b.mu.Unlock()

	if err := b.sink.Append(ctx, e); err != nil {
		return err
	}
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
	return nil
}

func (b *Bus) Stored() []Event { return b.sink.List() }
