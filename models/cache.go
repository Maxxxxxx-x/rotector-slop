package models

import "sync"

type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	store map[K]V
}

func NewCache[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{
		store: make(map[K]V),
	}
}

func (cache *Cache[K, V]) Get(key K) (V, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	val, exists := cache.store[key]
	return val, exists
}

func (cache *Cache[K, V]) Set(key K, val V) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.store[key] = val
}
