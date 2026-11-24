// Design: docs/architecture/api/process-protocol.md -- owned interface devices
// Related: ../owned_bridge_link.go -- the owned bridges and links each request changes
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
	"log/slog"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
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
	if err := pluginserver.RegisterProcessGenerationCleanup(ownedDevicesCleanup); err != nil {
		panic("BUG: owned interface device cleanup registration failed: " + err.Error())
	}
}

// ownedDeviceCallers maps a compiled registration owner name to the process
// that last made an owned-device request under it, so the generation cleanup
// withdraws an owner's devices only for the process that declared them, never
// for an older generation that ended after its replacement declared. Safe for
// concurrent use.
var ownedDeviceCallers = struct {
	mu      sync.Mutex
	byOwner map[string]*process.Process
}{byOwner: map[string]*process.Process{}}

// noteOwnedDeviceCaller records proc as the current declarer of owner.
func noteOwnedDeviceCaller(owner string, proc *process.Process) {
	ownedDeviceCallers.mu.Lock()
	ownedDeviceCallers.byOwner[owner] = proc
	ownedDeviceCallers.mu.Unlock()
}

// refuseOwnedDeviceCaller verifies proc's compiled registration identity holds
// an owned-device grant, refusing foreign external programs (whose
// RegistrationName is empty) while naming proc's configured endpoint alias in
// the operator-facing refusal.
func refuseOwnedDeviceCaller(proc *process.Process) (string, error) {
	owner := proc.RegistrationName()
	if err := iface.RefuseUngrantedDeviceOwner(owner); err != nil {
		if owner == "" {
			return "", opererr.Wrap(err,
				"Plugin "+proc.Name()+" may not ask for owned interface devices",
				"Only a plugin compiled into this ze binary that granted itself device names may create bridges and links, so no outside process can place a device in any network namespace",
				"Configure a compiled-in plugin with a device-name grant instead of an external program")
		}
		return "", err
	}
	return owner, nil
}

func ownedBridgesReplaceHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	owner, err := refuseOwnedDeviceCaller(proc)
	if err != nil {
		return nil, err
	}
	var input rpc.OwnedBridgesReplaceInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, ownedDeviceSubject); err != nil {
		return nil, err
	}
	noteOwnedDeviceCaller(owner, proc)
	output, err := iface.ReplaceOwnedBridges(owner, &input)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func ownedLinkCreateHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	owner, err := refuseOwnedDeviceCaller(proc)
	if err != nil {
		return nil, err
	}
	var input rpc.OwnedLinkCreateInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, ownedDeviceSubject); err != nil {
		return nil, err
	}
	noteOwnedDeviceCaller(owner, proc)
	output, err := iface.CreateOwnedLink(owner, &input)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func ownedLinkRemoveHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	owner, err := refuseOwnedDeviceCaller(proc)
	if err != nil {
		return nil, err
	}
	var input rpc.OwnedLinkRemoveInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, ownedDeviceSubject); err != nil {
		return nil, err
	}
	noteOwnedDeviceCaller(owner, proc)
	output, err := iface.RemoveOwnedLink(owner, &input)
	if err != nil {
		return nil, err
	}
	return output, nil
}

// ownedDevicesCleanup withdraws every bridge and link of a plugin process that
// is gone, when that process is the one that declared them. It runs after the
// process's admitted requests drained and before a replacement starts.
func ownedDevicesCleanup(_ *pluginserver.Server, proc *process.Process) {
	owner := proc.RegistrationName()
	if owner == "" {
		return
	}
	ownedDeviceCallers.mu.Lock()
	declarer, known := ownedDeviceCallers.byOwner[owner]
	if known && declarer == proc {
		delete(ownedDeviceCallers.byOwner, owner)
	}
	ownedDeviceCallers.mu.Unlock()
	if !known {
		return
	}
	if declarer != proc {
		return
	}
	if err := iface.WithdrawOwnedDevices(owner); err != nil {
		slogutil.Logger("iface").LogAttrs(context.Background(), slog.LevelError,
			"owned interface devices of an ended plugin were not all deleted",
			slog.String("plugin", proc.Name()), slog.String("error", err.Error()),
			slog.String("code", iface.CodeDoctorIfaceOwnedDeviceCleanup),
			slog.String("fix", "ip -d link show lists what remains with alias ze:owned:"+owner+"; the next interface commit deletes it"))
	}
}
