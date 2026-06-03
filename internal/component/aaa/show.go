// Design: docs/architecture/api/commands.md -- show aaa accounting provider

package aaa

import "sync"

var aaaAccountingProvider struct {
	sync.RWMutex
	fn func() map[string]any
}

// RegisterAAAAccountingProvider sets the provider used by show aaa accounting.
func RegisterAAAAccountingProvider(fn func() map[string]any) {
	aaaAccountingProvider.Lock()
	aaaAccountingProvider.fn = fn
	aaaAccountingProvider.Unlock()
}

// AAAAccountingData returns the current accounting data from the registered
// provider. Returns nil if no provider is registered.
func AAAAccountingData() map[string]any {
	aaaAccountingProvider.RLock()
	fn := aaaAccountingProvider.fn
	aaaAccountingProvider.RUnlock()
	if fn == nil {
		return nil
	}
	return fn()
}
