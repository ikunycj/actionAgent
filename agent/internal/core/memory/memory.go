package memory

import (
	"errors"
	"os"
)

type Result struct {
	Source string         `json:"source"`
	Score  float64        `json:"score"`
	Fields map[string]any `json:"fields,omitempty"`
}

type VectorRetriever interface {
	Search(query string, topK int) ([]Result, error)
}

type FTSRetriever interface {
	Search(query string, topK int) ([]Result, error)
}

type Engine struct {
	Vector VectorRetriever
	FTS    FTSRetriever
}

func (e Engine) Query(query string, topK int) ([]Result, error) {
	if topK < 1 {
		topK = 5
	}
	if e.Vector != nil {
		vec, err := e.Vector.Search(query, topK)
		if err == nil {
			fts, ferr := e.fts(query, topK)
			if ferr != nil {
				return vec, nil
			}
			return append(vec, fts...), nil
		}
	}
	return e.fts(query, topK)
}

func (e Engine) fts(query string, topK int) ([]Result, error) {
	if e.FTS == nil {
		return []Result{}, nil
	}
	return e.FTS.Search(query, topK)
}

func LoadMarkdown(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

type StaticRetriever struct {
	Results []Result
	Err     error
}

func (s StaticRetriever) Search(_ string, _ int) ([]Result, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Results, nil
}
