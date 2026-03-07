package session

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type IsolationMode string

const (
	ModeMain               IsolationMode = "main"
	ModePerPeer            IsolationMode = "per-peer"
	ModePerChannelPeer     IsolationMode = "per-channel-peer"
	ModePerAccountChanPeer IsolationMode = "per-account-channel-peer"
)

type KeyInput struct {
	AgentID   string
	AccountID string
	ChannelID string
	PeerID    string
	ThreadID  string
	Mode      IsolationMode
}

func NormalizeKey(in KeyInput) string {
	agent := clean(in.AgentID)
	account := clean(in.AccountID)
	channel := clean(in.ChannelID)
	peer := clean(in.PeerID)
	thread := clean(in.ThreadID)
	parts := []string{"agent", agent}
	switch in.Mode {
	case ModePerPeer:
		parts = append(parts, "peer", peer)
	case ModePerChannelPeer:
		parts = append(parts, "channel", channel, "peer", peer)
	case ModePerAccountChanPeer:
		parts = append(parts, "account", account, "channel", channel, "peer", peer)
	default:
		parts = append(parts, "main")
	}
	if thread != "" {
		parts = append(parts, "thread", thread)
	}
	return strings.Join(parts, ":")
}

func clean(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	if s == "" {
		return "unknown"
	}
	return s
}

type MaintenanceMode string

const (
	WarnMode    MaintenanceMode = "warn"
	EnforceMode MaintenanceMode = "enforce"
)

type MaintenancePolicy struct {
	Mode       MaintenanceMode
	PruneAfter time.Duration
	MaxEntries int
	MaxDisk    int64
}

type MaintenanceResult struct {
	Warnings []string
	Actions  []string
}

func Evaluate(policy MaintenancePolicy, entryCount int, diskBytes int64) MaintenanceResult {
	res := MaintenanceResult{}
	if policy.MaxEntries > 0 && entryCount > policy.MaxEntries {
		msg := fmt.Sprintf("max_entries exceeded: %d > %d", entryCount, policy.MaxEntries)
		if policy.Mode == WarnMode {
			res.Warnings = append(res.Warnings, msg)
		} else {
			res.Actions = append(res.Actions, "prune_entries")
		}
	}
	if policy.MaxDisk > 0 && diskBytes > policy.MaxDisk {
		msg := fmt.Sprintf("max_disk exceeded: %d > %d", diskBytes, policy.MaxDisk)
		if policy.Mode == WarnMode {
			res.Warnings = append(res.Warnings, msg)
		} else {
			res.Actions = append(res.Actions, "rotate_archive")
		}
	}
	return res
}

type TranscriptEntry struct {
	At      time.Time      `json:"at"`
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload,omitempty"`
}

type TranscriptStore struct {
	mu    sync.RWMutex
	byKey map[string][]TranscriptEntry
}

func NewTranscriptStore() *TranscriptStore {
	return &TranscriptStore{byKey: map[string][]TranscriptEntry{}}
}

func (s *TranscriptStore) Append(key string, e TranscriptEntry) {
	s.mu.Lock()
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	s.byKey[key] = append(s.byKey[key], e)
	s.mu.Unlock()
}

func (s *TranscriptStore) List(key string) []TranscriptEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	in := s.byKey[key]
	out := make([]TranscriptEntry, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func (s *TranscriptStore) RemoveOlderThan(key string, deadline time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	in := s.byKey[key]
	out := in[:0]
	removed := 0
	for _, e := range in {
		if e.At.Before(deadline) {
			removed++
			continue
		}
		out = append(out, e)
	}
	s.byKey[key] = out
	return removed
}
