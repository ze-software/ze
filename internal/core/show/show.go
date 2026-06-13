// Design: plan/spec-show-enricher.md -- show enricher registry

package show

import (
	"errors"
	"log/slog"
	"sort"
	"sync"
)

type Enricher struct {
	Detail func(base map[string]any)
	Brief  func(base map[string]any)
}

var errDuplicateKey = errors.New("show.Register: duplicate key")

type entry struct {
	key      string
	enricher Enricher
}

var (
	mu       sync.RWMutex
	registry = map[string][]entry{}
)

func Register(command, key string, e Enricher) error {
	mu.Lock()
	defer mu.Unlock()
	for _, existing := range registry[command] {
		if existing.key == key {
			return errDuplicateKey
		}
	}
	registry[command] = append(registry[command], entry{key: key, enricher: e})
	sort.Slice(registry[command], func(i, j int) bool { return registry[command][i].key < registry[command][j].key })
	return nil
}

func MustRegister(command, key string, e Enricher) {
	if err := Register(command, key, e); err != nil {
		panic("BUG: show.MustRegister")
	}
}

func Enrich(command string, base map[string]any) {
	mu.RLock()
	entries := registry[command]
	mu.RUnlock()

	for _, e := range entries {
		callDetail(e, base)
	}
}

func callDetail(e entry, base map[string]any) {
	if e.enricher.Detail == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("show enricher panicked", "key", e.key, "panic", r)
		}
	}()
	e.enricher.Detail(base)
}

func EnrichBrief(command string, base map[string]any) {
	mu.RLock()
	entries := registry[command]
	mu.RUnlock()

	for _, e := range entries {
		callBrief(e, base)
	}
}

func callBrief(e entry, base map[string]any) {
	if e.enricher.Brief == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("show enricher panicked", "key", e.key, "panic", r)
		}
	}()
	e.enricher.Brief(base)
}

func ResetForTest() {
	mu.Lock()
	registry = map[string][]entry{}
	mu.Unlock()
}
