package storage

import "sync"

type KV interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
}

type InMemoryKV struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewInMemoryKV() *InMemoryKV {
	return &InMemoryKV{m: map[string]string{}}
}

func (s *InMemoryKV) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

func (s *InMemoryKV) Set(key, value string) {
	s.mu.Lock()
	s.m[key] = value
	s.mu.Unlock()
}

func (s *InMemoryKV) Delete(key string) {
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}
