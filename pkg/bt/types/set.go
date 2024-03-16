package types

import "sync"

var _sentinel = struct{}{}

type Set[K comparable] interface {
	All() []K
	Len() int
	Put(key K)
	PutAll(all []K)
	Has(key K) bool
	Del(key K)
}

var _1 Set[string] = &SyncSet[string]{}
var _2 Set[string] = &BasicSet[string]{}

type SyncSet[K comparable] struct {
	BasicSet[K]

	sync.Mutex
}

func (s *SyncSet[K]) All() []K {
	s.Lock()
	defer s.Unlock()
	return s.BasicSet.All()
}

func (s *SyncSet[K]) Put(v K) {
	s.Lock()
	defer s.Unlock()
	s.BasicSet.Put(v)
}

func (s *SyncSet[K]) PutAll(v []K) {
	s.Lock()
	defer s.Unlock()
	s.BasicSet.PutAll(v)
}

func (s *SyncSet[K]) Has(v K) bool {
	s.Lock()
	defer s.Unlock()
	return s.BasicSet.Has(v)
}

func (s *SyncSet[K]) Del(v K) {
	s.Lock()
	defer s.Unlock()
	s.BasicSet.Del(v)
}

type BasicSet[K comparable] struct {
	items map[K]struct{}
}

func NewSet[K comparable]() Set[K] {
	return &BasicSet[K]{
		items: make(map[K]struct{}),
	}
}

func NewSyncSet[K comparable]() Set[K] {
	return &SyncSet[K]{}
}

func (s *BasicSet[K]) All() []K {
	all := []K{}
	for k := range s.items {
		all = append(all, k)
	}
	return all
}

func (s *BasicSet[K]) Len() int {
	return len(s.items)
}

func (s *BasicSet[K]) Put(v K) {
	s.items[v] = _sentinel
}

func (s *BasicSet[K]) PutAll(all []K) {
	for _, item := range all {
		s.items[item] = _sentinel
	}
}

func (s *BasicSet[K]) Has(v K) bool {
	if _, ok := s.items[v]; ok {
		return true
	}
	return false
}

func (s *BasicSet[K]) Del(v K) {
	delete(s.items, v)
}
