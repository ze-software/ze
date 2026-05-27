// Design: plan/spec-smart-management.md — SMART disk health management
// Related: show.go — show verb RPC registration pattern

package show

import (
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/storage"
)

var storageManagerPtr atomic.Pointer[storage.Manager]

// SetStorageManager installs the storage manager for the show RPC.
func SetStorageManager(m *storage.Manager) {
	storageManagerPtr.Store(m)
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:storage-smart",
			Handler:    handleShowStorageSmart,
		},
	)
}

func handleShowStorageSmart(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	m := storageManagerPtr.Load()
	if m == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "storage SMART management not configured",
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"devices": m.Status()},
	}, nil
}
