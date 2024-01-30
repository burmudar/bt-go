package peer

import "sync"

type registration[K comparable, V comparable] struct {
	Key   K
	Value V
}

type registrar[K comparable, V comparable] struct {
	items map[K]Set[V]

	C chan registration[K, V]

	sync.Mutex
}

func newRegistrar[K comparable, V comparable]() *registrar[K, V] {
	return &registrar[K, V]{
		items: map[K]Set[V]{},
		C:     make(chan registration[K, V]),
	}
}

func (rr *registrar[K, V]) listen() {
	for r := range rr.C {
		rr.Add(r.Key, r.Value)
	}
}

func (rr *registrar[K, V]) Get(k K) ([]V, bool) {
	rr.Lock()
	defer rr.Unlock()
	v, ok := rr.items[k]
	if ok {
		return v.All(), ok
	} else {
		return nil, false
	}
}

func (rr *registrar[K, V]) Rem(k K, v V) {
	rr.Lock()
	defer rr.Unlock()
	s, ok := rr.items[k]
	if ok {
		s.Del(v)
	}
}

func (rr *registrar[K, V]) Add(key K, v V) {
	rr.Lock()
	defer rr.Unlock()
	var set Set[V]
	if s, ok := rr.items[key]; ok {
		set = s
	} else {
		set = NewSet[V]()
	}
	set.Put(v)
	rr.items[key] = set
}
