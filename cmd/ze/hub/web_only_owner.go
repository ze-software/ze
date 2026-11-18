//go:build ze_web

// Design: docs/architecture/web-interface.md -- standalone web cleanup owns its commands.
package hub

import (
	"context"
	"errors"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// bindWebOnlyRecovery borrows the launcher's store and existing stop channel.
// The caller MUST join web requests and call shutdownWebOnly before releasing
// either resource. No configuration reload or confirmed window is installed.
func bindWebOnlyRecovery(store storage.Storage, commands *pluginserver.Server, stop chan<- os.Signal) {
	stopRestartCause.Store(nil)
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: store, ConfigPath: storage.SourcePath(store)})
	commands.SetRestartFunc(func() {
		requestRestartStop(rpc.DrainRestartInPlace)
		select {
		case stop <- requestedStop{}:
		default:
			// A full channel already has a stop for the same existing loop.
		}
	})
}

func webOnlyExitStatus(cleanupError error) int {
	if cleanupError != nil {
		return 1
	}
	if cause := stopRestartCause.Load(); cause != nil {
		if *cause == rpc.DrainRestartInPlace {
			return rpc.DaemonRestartExitStatus
		}
	}
	return 0
}

// shutdownWebOnly MUST finish before the caller releases its borrowed store.
// Web requests and history retain the command owner until their join finishes.
func shutdownWebOnly(ctx context.Context, service *webService, commands *pluginserver.Server) error {
	var webError error
	if service != nil {
		webError = service.Shutdown(ctx)
	}
	storagecli.InstallDataRPC(nil)
	commands.SetRestartFunc(nil)
	commands.Stop()
	if commandError := commands.StopFailure(); commandError != nil {
		return errors.Join(webError, commandError)
	}
	return webError
}
