// Package omap is JavaScript's Map and Set: collections that iterate in insertion order,
// where Go's maps iterate in no fixed order.
package omap

import "iter"

type entry[K comparable, V any] struct {
	key     K
	value   V
	deleted bool
}

// Map is a JS Map. The zero value is empty and ready to use. It is not safe for concurrent
// use.
type Map[K comparable, V any] struct {
	entries []entry[K, V]
	index   map[K]int
	live    int
}

// Get returns the value for key and whether the map has it.
func (m *Map[K, V]) Get(key K) (V, bool) {
	if i, ok := m.index[key]; ok {
		return m.entries[i].value, true
	}
	var zero V
	return zero, false
}

// Has reports whether the map has key.
func (m *Map[K, V]) Has(key K) bool {
	_, ok := m.index[key]
	return ok
}

// Set adds key at the end, or replaces its value in place.
func (m *Map[K, V]) Set(key K, value V) {
	if i, ok := m.index[key]; ok {
		m.entries[i].value = value
		return
	}
	if m.index == nil {
		m.index = map[K]int{}
	}
	m.index[key] = len(m.entries)
	m.entries = append(m.entries, entry[K, V]{key: key, value: value})
	m.live++
}

// Delete removes key. Setting it again adds it at the end.
func (m *Map[K, V]) Delete(key K) bool {
	i, ok := m.index[key]
	if !ok {
		return false
	}
	delete(m.index, key)
	var zero V
	m.entries[i] = entry[K, V]{deleted: true, value: zero}
	m.live--
	return true
}

// Len returns the number of keys.
func (m *Map[K, V]) Len() int { return m.live }

// All iterates in insertion order. As with a JS Map iterator, it reaches entries added
// while it runs and skips ones deleted before it gets to them.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := 0; i < len(m.entries); i++ {
			e := m.entries[i]
			if e.deleted {
				continue
			}
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

// Keys iterates over the keys in insertion order, as All does.
func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range m.All() {
			if !yield(k) {
				return
			}
		}
	}
}

// Set is a JS Set.
type Set[K comparable] struct {
	m Map[K, struct{}]
}

// Add adds key at the end if it is not already there.
func (s *Set[K]) Add(key K) { s.m.Set(key, struct{}{}) }

// Has reports whether the set has key.
func (s *Set[K]) Has(key K) bool { return s.m.Has(key) }

// Delete removes key.
func (s *Set[K]) Delete(key K) bool { return s.m.Delete(key) }

// Len returns the number of keys.
func (s *Set[K]) Len() int { return s.m.Len() }

// All iterates over the keys in insertion order.
func (s *Set[K]) All() iter.Seq[K] { return s.m.Keys() }
