package show

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

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

func handleShowAAAAccounting(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	aaaAccountingProvider.RLock()
	fn := aaaAccountingProvider.fn
	aaaAccountingProvider.RUnlock()
	if fn == nil {
		return &plugin.Response{Status: plugin.StatusDone, Data: map[string]any{"dropped-records": uint64(0)}}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: fn()}, nil
}
