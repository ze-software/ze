// Design: docs/architecture/show-enricher.md -- show enricher registry

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
	current := registry[command]
	for _, existing := range current {
		if existing.key == key {
			return errDuplicateKey
		}
	}
	// Enrich and EnrichBrief iterate the list they read under the read lock
	// after releasing it, so a published list is never changed in place: an
	// append into spare capacity and an in-place sort would move the entries
	// under that iteration. The new list is a fresh copy, sorted, then
	// published, as Unregister does.
	fresh := make([]entry, 0, len(current)+1)
	fresh = append(fresh, current...)
	fresh = append(fresh, entry{key: key, enricher: e})
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].key < fresh[j].key })
	registry[command] = fresh
	return nil
}

func MustRegister(command, key string, e Enricher) {
	if err := Register(command, key, e); err != nil {
		panic("BUG: show.MustRegister")
	}
}

func Unregister(command, key string) {
	mu.Lock()
	defer mu.Unlock()
	entries, ok := registry[command]
	if !ok {
		return
	}
	for i, e := range entries {
		if e.key != key {
			continue
		}
		fresh := make([]entry, 0, len(entries)-1)
		fresh = append(fresh, entries[:i]...)
		fresh = append(fresh, entries[i+1:]...)
		if len(fresh) == 0 {
			delete(registry, command)
		} else {
			registry[command] = fresh
		}
		return
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
