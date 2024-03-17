package types

import "sync"

type Queue[K any] interface {
	IsEmpty() bool
	Size() int
	Head() (K, bool)
	Tail() (K, bool)
	Add(v K)
	AddFirst(v K)
	AddAll(v ...K)
	Pop() (K, bool)
}

type syncSliceQueue[K any] struct {
	sync.Mutex
	queue Queue[K]
}

// Add implements Queue.
func (s *syncSliceQueue[K]) Add(v K) {
	s.Lock()
	defer s.Unlock()
	s.queue.Add(v)
}

// Add implements Queue.
func (s *syncSliceQueue[K]) AddFirst(v K) {
	s.Lock()
	defer s.Unlock()
	s.queue.AddFirst(v)
}

// AddAll implements Queue.
func (s *syncSliceQueue[K]) AddAll(v ...K) {
	s.Lock()
	defer s.Unlock()
	s.queue.AddAll(v...)
}

// Head implements Queue.
func (s *syncSliceQueue[K]) Head() (K, bool) {
	s.Lock()
	defer s.Unlock()
	return s.queue.Head()
}

// Pop implements Queue.
func (s *syncSliceQueue[K]) Pop() (K, bool) {
	s.Lock()
	defer s.Unlock()
	return s.queue.Pop()
}

// Size implements Queue.
func (s *syncSliceQueue[K]) Size() int {
	s.Lock()
	defer s.Unlock()
	return s.queue.Size()
}

// IsEmpty implements Queue.
func (s *syncSliceQueue[K]) IsEmpty() bool {
	s.Lock()
	defer s.Unlock()
	return s.queue.IsEmpty()
}

// Tail implements Queue.
func (s *syncSliceQueue[K]) Tail() (K, bool) {
	s.Lock()
	defer s.Unlock()
	return s.queue.Tail()
}

type sliceQueue[K any] struct {
	items []K
}

// Add implements Queue.
func (s *sliceQueue[K]) Add(v K) {
	s.items = append(s.items, v)
}

// AddFirst implements Queue.
func (s *sliceQueue[K]) AddFirst(v K) {
	items := []K{v}
	s.items = append(items, s.items...)
}

// AddAll implements Queue.
func (s *sliceQueue[K]) AddAll(v ...K) {
	s.items = append(s.items, v...)
}

// Head implements Queue.
func (s *sliceQueue[K]) Head() (K, bool) {
	if s.IsEmpty() {
		var empty K
		return empty, false
	}
	return s.items[0], true
}

// Pop implements Queue.
func (s *sliceQueue[K]) Pop() (K, bool) {
	if s.IsEmpty() {
		var empty K
		return empty, false
	}
	value := s.items[0]
	s.items = s.items[1:]

	return value, true
}

// Size implements Queue.
func (s *sliceQueue[K]) Size() int {
	return len(s.items)
}

func (s *sliceQueue[K]) IsEmpty() bool {
	return len(s.items) == 0
}

// Tail implements Queue.
func (s *sliceQueue[K]) Tail() (K, bool) {
	last := len(s.items) - 1

	if last < 0 {
		var empty K
		return empty, false
	}

	return s.items[last], true
}

func NewSliceQueue[K any]() Queue[K] {
	return &sliceQueue[K]{
		items: []K{},
	}
}

func NewSyncQueue[K any]() Queue[K] {
	return &syncSliceQueue[K]{
		queue: NewSliceQueue[K](),
	}
}
