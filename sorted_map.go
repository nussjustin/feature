package feature

import (
	"maps"
	"slices"
)

type sortedMap[T any] struct {
	m    map[string]T
	keys []string
}

func (s sortedMap[T]) add(key string, val T) sortedMap[T] {
	return s.addMany(map[string]T{key: val})
}

func (s sortedMap[T]) addMany(m map[string]T) sortedMap[T] {
	s2 := sortedMap[T]{m: maps.Clone(s.m), keys: slices.Clone(s.keys)}

	if s2.m == nil {
		s2.m = make(map[string]T, 1)
	}

	for key, val := range m {
		if _, ok := s.m[key]; !ok {
			s2.keys = append(s2.keys, key)
		}
		s2.m[key] = val
	}

	if len(s.keys) != len(s2.keys) {
		slices.Sort(s2.keys)
	}

	return s2
}
