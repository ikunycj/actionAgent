package session

import (
	"strings"
	"testing"
	"time"
)

func TestTranscriptStoreEnforcePolicy(t *testing.T) {
	store := NewTranscriptStoreWithPolicy(MaintenancePolicy{
		Mode:       EnforceMode,
		PruneAfter: 24 * time.Hour,
		MaxEntries: 2,
		MaxDisk:    256,
	})
	key := "s1"
	now := time.Now().UTC()
	store.Append(key, TranscriptEntry{At: now.Add(-48 * time.Hour), Event: "old", Payload: map[string]any{"text": strings.Repeat("x", 32)}})
	store.Append(key, TranscriptEntry{At: now.Add(-2 * time.Hour), Event: "mid", Payload: map[string]any{"text": strings.Repeat("y", 32)}})
	store.Append(key, TranscriptEntry{At: now.Add(-1 * time.Hour), Event: "new", Payload: map[string]any{"text": strings.Repeat("z", 32)}})

	stats := store.Stats(key)
	if stats.Entries > 2 {
		t.Fatalf("expected entry cap enforcement, got %d", stats.Entries)
	}
	if stats.DiskBytes > 256 {
		t.Fatalf("expected disk cap enforcement, got %d", stats.DiskBytes)
	}
	list := store.List(key)
	for _, e := range list {
		if e.Event == "old" {
			t.Fatal("expected prune_by_age to remove old entry")
		}
	}
}

func TestTranscriptStoreWarnPolicy(t *testing.T) {
	store := NewTranscriptStoreWithPolicy(MaintenancePolicy{
		Mode:       WarnMode,
		PruneAfter: time.Hour,
		MaxEntries: 1,
		MaxDisk:    64,
	})
	key := "s2"
	now := time.Now().UTC()
	store.Append(key, TranscriptEntry{At: now.Add(-2 * time.Hour), Event: "e1", Payload: map[string]any{"text": strings.Repeat("a", 32)}})
	store.Append(key, TranscriptEntry{At: now, Event: "e2", Payload: map[string]any{"text": strings.Repeat("b", 32)}})

	res := store.LastMaintenance(key)
	if len(res.Warnings) == 0 {
		t.Fatal("expected warnings in warn mode")
	}
	stats := store.Stats(key)
	if stats.Entries != 2 {
		t.Fatalf("warn mode should not prune automatically, got %d entries", stats.Entries)
	}
}
