package ordered_test

import (
	"slices"
	"testing"

	"github.com/nussjustin/httpsfv/ordered"
)

func TestMap(t *testing.T) {
	var m ordered.Map[string, int]

	if got, want := collect(&m), []pair[string, int]{}; !slices.Equal(got, want) {
		t.Errorf("m.All() = %v, want %v", got, want)
	}

	if got, gotOk := m.At(0); got != 0 || gotOk {
		t.Errorf("m.At(0) = (%d, %t); want (%d, %t)", got, gotOk, 0, false)
	}

	if got, want := m.Contains("key1"), false; got != want {
		t.Errorf("m.Contains(%q) = (%t); want (%t)", "key1", got, want)
	}

	if got, want := m.Delete("key1"), false; got != want {
		t.Errorf("m.Delete(%q) = (%t); want (%t)", "key1", got, want)
	}

	if got, gotOk := m.Get("key1"); got != 0 || gotOk {
		t.Errorf("m.Got(%q) = (%d, %t); want (%d, %t)", "key1", got, gotOk, 0, false)
	}

	if got, want := m.Len(), 0; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}

	for i, key := range []string{"key1", "key3", "key2"} {
		m.Set(key, i+1)
	}

	if got, want := collect(&m), []pair[string, int]{{"key1", 1}, {"key3", 2}, {"key2", 3}}; !slices.Equal(got, want) {
		t.Errorf("m.All() = %v, want %v", got, want)
	}

	if got, gotOk := m.At(0); got != 1 || !gotOk {
		t.Errorf("m.At(0) = (%d, %t); want (%d, %t)", got, gotOk, 0, true)
	}

	if got, gotOk := m.At(1); got != 2 || !gotOk {
		t.Errorf("m.At(1) = (%d, %t); want (%d, %t)", got, gotOk, 0, true)
	}

	if got, gotOk := m.At(3); got != 0 || gotOk {
		t.Errorf("m.At(3) = (%d, %t); want (%d, %t)", got, gotOk, 0, false)
	}

	if got, want := m.Contains("key1"), true; got != want {
		t.Errorf("m.Contains(%q) = (%t); want (%t)", "key1", got, want)
	}

	if got, want := m.Contains("key2"), true; got != want {
		t.Errorf("m.Contains(%q) = (%t); want (%t)", "key2", got, want)
	}

	if got, want := m.Contains("key4"), false; got != want {
		t.Errorf("m.Contains(%q) = (%t); want (%t)", "key4", got, want)
	}

	if got, gotOk := m.Get("key1"); got != 1 || !gotOk {
		t.Errorf("m.Got(%q) = (%d, %t); want (%d, %t)", "key1", got, gotOk, 1, true)
	}

	if got, gotOk := m.Get("key2"); got != 3 || !gotOk {
		t.Errorf("m.Got(%q) = (%d, %t); want (%d, %t)", "key2", got, gotOk, 3, true)
	}

	if got, want := m.Len(), 3; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}

	m.Set("key1", -1)

	if got, gotOk := m.Get("key1"); got != -1 || !gotOk {
		t.Errorf("m.Got(%q) = (%d, %t); want (%d, %t)", "key1", got, gotOk, -1, true)
	}

	if got, want := collect(&m), []pair[string, int]{{"key1", -1}, {"key3", 2}, {"key2", 3}}; !slices.Equal(got, want) {
		t.Errorf("m.All() = %v, want %v", got, want)
	}

	if got, want := m.Delete("key1"), true; got != want {
		t.Errorf("m.Delete(%q) = (%t); want (%t)", "key1", got, want)
	}

	if got, want := m.Delete("key1"), false; got != want {
		t.Errorf("m.Delete(%q) = (%t); want (%t)", "key1", got, want)
	}

	if got, want := collect(&m), []pair[string, int]{{"key3", 2}, {"key2", 3}}; !slices.Equal(got, want) {
		t.Errorf("m.All() = %v, want %v", got, want)
	}

	if got, want := m.Contains("key1"), false; got != want {
		t.Errorf("m.Contains(%q) = (%t); want (%t)", "key1", got, want)
	}

	if got, gotOk := m.Get("key1"); got != 0 || gotOk {
		t.Errorf("m.Got(%q) = (%d, %t); want (%d, %t)", "key1", got, gotOk, 0, false)
	}

	if got, want := m.Len(), 2; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}

	m.Set("key1", 1)

	if got, gotOk := m.Get("key1"); got != 1 || !gotOk {
		t.Errorf("m.Got(%q) = (%d, %t); want (%d, %t)", "key1", got, gotOk, 1, true)
	}

	if got, want := collect(&m), []pair[string, int]{{"key3", 2}, {"key2", 3}, {"key1", 1}}; !slices.Equal(got, want) {
		t.Errorf("m.All() = %v, want %v", got, want)
	}
}

type pair[K any, V any] struct {
	key   K
	value V
}

func collect[K comparable, V any](m *ordered.Map[K, V]) []pair[K, V] {
	var s []pair[K, V]
	for k, v := range m.All() {
		s = append(s, pair[K, V]{k, v})
	}
	return s
}
