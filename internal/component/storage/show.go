// Design: docs/architecture/storage/smart-health.md -- SMART disk health management
// Related: manager.go — storage manager lifecycle

package storage

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

var storageManagerPtr atomic.Pointer[Manager]

// SetStorageManager installs the storage manager for the show RPC.
func SetStorageManager(m *Manager) {
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
