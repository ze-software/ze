// Design: docs/architecture/diagnostics/production-diagnostics.md -- diag component RPC registration

package cmd

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:tcp-check", Handler: HandleTCPCheck},
		pluginserver.RPCRegistration{WireMethod: "ze-show:capture", Handler: HandleShowCapture},
		pluginserver.RPCRegistration{WireMethod: "ze-show:capture-raw", Handler: HandleCaptureRaw},
		pluginserver.RPCRegistration{WireMethod: "ze-show:capture-interface", Handler: HandleCaptureInterface},
	)
}
