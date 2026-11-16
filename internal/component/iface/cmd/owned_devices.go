// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: ../device_owner.go -- the device name grant each request checks
// Related: ../../../../pkg/plugin/rpc/owned_devices.go -- the request values
//
// The owned-device requests are engine operations in the interface namespace,
// so a plugin reaches them over the direct path internal and over RPC forked,
// and the owner of every device is the authenticated calling process. They
// register here, not in package iface, because plugin/server imports iface.

package cmd

import (
	"context"
	"encoding/json"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ownedDeviceSubject names the requests in a decode refusal.
const ownedDeviceSubject = "Owned interface device request"

func init() {
	// ConfigureSafe: an owner may declare its bridges and start what needs a
	// link from its configure stage, and every handler answers within ctx.
	for _, operation := range []pluginserver.EngineOperation{
		{Method: rpc.MethodOwnedBridgesReplace, Handler: ownedBridgesReplaceHandler, Startup: pluginserver.EngineOperationConfigureSafe},
		{Method: rpc.MethodOwnedLinkCreate, Handler: ownedLinkCreateHandler, Startup: pluginserver.EngineOperationConfigureSafe},
		{Method: rpc.MethodOwnedLinkRemove, Handler: ownedLinkRemoveHandler, Startup: pluginserver.EngineOperationConfigureSafe},
	} {
		if err := pluginserver.RegisterEngineOp(operation); err != nil {
			panic("BUG: owned interface device registration failed: " + err.Error())
		}
	}
}

func ownedBridgesReplaceHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	if err := iface.RefuseUngrantedDeviceOwner(proc.Name()); err != nil {
		return nil, err
	}
	var input rpc.OwnedBridgesReplaceInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, ownedDeviceSubject); err != nil {
		return nil, err
	}
	return nil, ownedDevicesNotBuilt()
}

func ownedLinkCreateHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	if err := iface.RefuseUngrantedDeviceOwner(proc.Name()); err != nil {
		return nil, err
	}
	var input rpc.OwnedLinkCreateInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, ownedDeviceSubject); err != nil {
		return nil, err
	}
	return nil, ownedDevicesNotBuilt()
}

func ownedLinkRemoveHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	if err := iface.RefuseUngrantedDeviceOwner(proc.Name()); err != nil {
		return nil, err
	}
	var input rpc.OwnedLinkRemoveInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, ownedDeviceSubject); err != nil {
		return nil, err
	}
	return nil, ownedDevicesNotBuilt()
}

// ownedDevicesNotBuilt is the refusal every owned-device request answers until
// the interface component creates owned bridges and links. Nothing is
// recorded while every request is refused, so a process that ends holds
// nothing and no generation cleanup is registered yet.
func ownedDevicesNotBuilt() error {
	return opererr.New(
		"Owned interface devices are not implemented in this build",
		"The interface component declares the request and creates no bridge or link yet, so nothing was changed",
		"Run a ze build that implements owned interface devices; ze show version names the running build",
	)
}
