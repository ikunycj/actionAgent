package memory

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestFallbackToFTSWhenVectorUnavailable(t *testing.T) {
	e := Engine{
		Vector: StaticRetriever{Err: errors.New("vector down")},
		FTS:    StaticRetriever{Results: []Result{{Source: "fts", Score: 0.9}}},
	}
	res, err := e.Query("hello", 3)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(res) != 1 || res[0].Source != "fts" {
		t.Fatalf("expected fts fallback result, got %+v", res)
	}
}

func TestMissingMemoryFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")
	text, err := LoadMarkdown(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if text != "" {
		t.Fatalf("expected empty text for missing file")
	}
}
