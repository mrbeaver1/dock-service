package middleware

import "sync"

type typedSyncMap[K comparable, V any] struct {
	m sync.Map
}

func (s *typedSyncMap[K, V]) Load(key K) (V, bool) {
	v, ok := s.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}

	return v.(V), true
}

func (s *typedSyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	a, loaded := s.m.LoadOrStore(key, value)
	return a.(V), loaded
}
