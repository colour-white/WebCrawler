package main

import (
	"sync"
)

type URLSet struct {
	visited map[string]struct{}
	mu      sync.Mutex
}

func NewURLSet() *URLSet {
	return &URLSet{
		visited: make(map[string]struct{}),
	}
}

func (s *URLSet) Add(rawURL string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.visited[rawURL]; exists {
		return false
	}

	s.visited[rawURL] = struct{}{}
	return true
}

type ParsedPage struct {
	URL     string
	Links   []string
	Content string
	Title   string
}
