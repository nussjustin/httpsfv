package ordered

import (
	"iter"
)

// Map is an ordered map of values.
//
// The zero value of a Map is an empty map ready to use.
type Map[K comparable, V any] struct {
	// putting the actual map behind a pointer allows safely copying the Map
	internal *internalMap[K, V]
}

type internalMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

// All returns an iterator over all items in the map in the order they were added.
//
// Modifying the map during iteration can result in some members not being visited.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if m.internal == nil {
			return
		}

		for _, key := range m.internal.keys {
			if !yield(key, m.internal.m[key]) {
				return
			}
		}
	}
}

// At returns the value at the given position.
func (m *Map[K, V]) At(idx int) (V, bool) {
	if m.internal == nil || idx >= len(m.internal.keys) {
		var zero V
		return zero, false
	}
	return m.Get(m.internal.keys[idx])
}

// Contains returns whether the given key exists in the map.
func (m *Map[K, V]) Contains(key K) bool {
	if m.internal == nil {
		return false
	}
	_, ok := m.internal.m[key]
	return ok
}

// Delete deletes the value for the given key.
func (m *Map[K, V]) Delete(key K) bool {
	if m.internal == nil {
		return false
	}

	idx := -1

	for i := range m.internal.keys {
		if m.internal.keys[i] == key {
			idx = i
			break
		}
	}

	if idx == -1 {
		return false
	}

	m.internal.keys = append(m.internal.keys[:idx], m.internal.keys[idx+1:]...)
	clear(m.internal.keys[len(m.internal.keys) : len(m.internal.keys)+1])
	delete(m.internal.m, key)
	return true
}

// Get returns the value with the given key.
func (m *Map[K, V]) Get(key K) (V, bool) {
	if m.internal == nil {
		var zero V
		return zero, false
	}

	v, ok := m.internal.m[key]
	return v, ok
}

// Len returns the number of items in the map.
func (m *Map[K, V]) Len() int {
	if m.internal == nil {
		return 0
	}

	return len(m.internal.keys)
}

// Set sets the value for the given key in the map.
//
// If the key already exists in the map, the value replaces the existing value, keeping the position of the old value.
func (m *Map[K, V]) Set(key K, value V) {
	// Pre-allocate a little space to avoid some allocations in the common case
	const initialCap = 4
	if m.internal == nil {
		m.internal = &internalMap[K, V]{m: make(map[K]V, initialCap), keys: make([]K, 0, initialCap)}
	}
	if _, ok := m.internal.m[key]; !ok {
		m.internal.keys = append(m.internal.keys, key)
	}
	m.internal.m[key] = value
}
