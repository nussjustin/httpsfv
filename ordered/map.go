package ordered

import (
	"iter"
	"slices"
)

// Map is an ordered map of values.
//
// The zero value of a Map is an empty map ready to use.
type Map[K comparable, V any] struct {
	impl mapIface[K, V]
}

// All returns an iterator over all items in the map in the order they were added.
//
// Modifying the map during iteration can result in some members not being visited.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if m.impl == nil {
			return
		}

		for k, v := range m.impl.All() {
			if !yield(k, v) {
				return
			}
		}
	}
}

// At returns the value at the given position.
func (m *Map[K, V]) At(idx int) (V, bool) {
	if m.impl == nil {
		var zero V
		return zero, false
	}

	return m.impl.At(idx)
}

// Contains returns whether the given key exists in the map.
func (m *Map[K, V]) Contains(key K) bool {
	if m.impl == nil {
		return false
	}

	return m.impl.Contains(key)
}

// Delete deletes the value for the given key.
func (m *Map[K, V]) Delete(key K) bool {
	if m.impl == nil {
		return false
	}

	return m.impl.Delete(key)
}

// Get returns the value with the given key.
func (m *Map[K, V]) Get(key K) (V, bool) {
	if m.impl == nil {
		var zero V
		return zero, false
	}

	return m.impl.Get(key)
}

// Len returns the number of items in the map.
func (m *Map[K, V]) Len() int {
	if m.impl == nil {
		return 0
	}

	return m.impl.Len()
}

// Set sets the value for the given key in the map.
//
// If the key already exists in the map, the value replaces the existing value, keeping the position of the old value.
func (m *Map[K, V]) Set(key K, value V) {
	switch v := m.impl.(type) {
	case *sliceMap[K, V]:
		if len(v.s) == cap(v.s) {
			m.impl = hashMapFrom[K, V](v)
		}
	case nil:
		var x struct {
			m  sliceMap[K, V]
			ms [8]sliceMapEntry[K, V]
		}
		x.m.s = x.ms[:0]
		m.impl = &x.m
	}

	m.impl.Set(key, value)
}

type mapIface[K comparable, V any] interface {
	All() iter.Seq2[K, V]
	At(idx int) (V, bool)
	Contains(key K) bool
	Delete(key K) bool
	Get(key K) (V, bool)
	Len() int
	Set(key K, value V)
}

type sliceMap[K comparable, V any] struct {
	s []sliceMapEntry[K, V]
}

type sliceMapEntry[K comparable, V any] struct {
	key   K
	value V
}

func (f *sliceMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, e := range f.s {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

func (f *sliceMap[K, V]) At(idx int) (V, bool) {
	if idx >= len(f.s) {
		var zero V
		return zero, false
	}
	return f.s[idx].value, true
}

func (f *sliceMap[K, V]) Contains(key K) bool {
	for _, e := range f.s {
		if e.key == key {
			return true
		}
	}

	return false
}

func (f *sliceMap[K, V]) Delete(key K) bool {
	for i, e := range f.s {
		if e.key != key {
			continue
		}

		f.s = slices.Delete(f.s, i, 1)
		return true
	}

	return false
}

func (f *sliceMap[K, V]) Get(key K) (V, bool) {
	for _, e := range f.s {
		if e.key != key {
			continue
		}

		return e.value, true
	}

	var zero V
	return zero, false
}

func (f *sliceMap[K, V]) Len() int {
	return len(f.s)
}

func (f *sliceMap[K, V]) Set(key K, value V) {
	for i, e := range f.s {
		if e.key != key {
			continue
		}

		f.s[i].value = value
		return
	}

	f.s = append(f.s, sliceMapEntry[K, V]{key: key, value: value})
}

type hashMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

func hashMapFrom[K comparable, V any](m mapIface[K, V]) *hashMap[K, V] {
	l := m.Len() + 8

	h := &hashMap[K, V]{keys: make([]K, 0, l), m: make(map[K]V, l)}

	for k, v := range m.All() {
		h.keys = append(h.keys, k)
		h.m[k] = v
	}

	return h
}

func (m *hashMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, key := range m.keys {
			if !yield(key, m.m[key]) {
				return
			}
		}
	}
}

func (m *hashMap[K, V]) At(idx int) (V, bool) {
	if idx >= len(m.keys) {
		var zero V
		return zero, false
	}
	return m.Get(m.keys[idx])
}

func (m *hashMap[K, V]) Contains(key K) bool {
	_, ok := m.m[key]
	return ok
}

func (m *hashMap[K, V]) Delete(key K) bool {
	idx := -1

	for i := range m.keys {
		if m.keys[i] == key {
			idx = i
			break
		}
	}

	if idx == -1 {
		return false
	}

	m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
	clear(m.keys[len(m.keys) : len(m.keys)+1])
	delete(m.m, key)
	return true
}

func (m *hashMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.m[key]
	return v, ok
}

func (m *hashMap[K, V]) Len() int {
	return len(m.keys)
}

func (m *hashMap[K, V]) Set(key K, value V) {
	if m.m == nil {
		m.m = make(map[K]V)
	}

	if _, ok := m.m[key]; !ok {
		m.keys = append(m.keys, key)
	}

	m.m[key] = value
}
