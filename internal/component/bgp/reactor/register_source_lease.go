// Design: docs/architecture/api/process-protocol.md -- original source lease lifetime.
// Related: source_lease.go -- shared authenticated typed and JSON evaluator.
package reactor

import (
	"context"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const codeSourceLeaseCleanup = "doctor-bgp-source-lease-cleanup"

func init() {
	if err := pluginserver.RegisterEngineOp(pluginserver.EngineOperation{
		Method:            rpc.MethodSourceLease,
		Handler:           sourceLeaseJSON,
		Native:            sourceLeaseNative,
		Startup:           pluginserver.EngineOperationRuntimeOnly,
		BusExemptReason:   "caller's own source-receipt leases, gated by SelectedOwner and peer dissemination receive grant",
		CancellationScope: sourceLeaseCancellation,
	}); err != nil {
		panic("BUG: source lease operation registration failed: " + err.Error())
	}
	if err := pluginserver.RegisterProcessGenerationCleanup(sourceGenerationCleanup); err != nil {
		panic("BUG: source lease cleanup registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{Code: codeSourceLeaseCleanup,
		Title:       "BGP selected output cleanup did not finish",
		Description: "A stopped process, removed Peer or reactor still has selected source or origin work which did not join within its bounded cleanup deadline. Its authority is revoked. Restart Ze before relying on selected output from a replacement process.",
		Examples:    []string{"ze request restart"},
	}); err != nil {
		panic("BUG: source lease diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeSourceLeaseCleanup}); err != nil {
		panic("BUG: source lease event registration failed: " + err.Error())
	}
}

// sourceGenerationCleanup joins source writes before retiring the ended
// incarnation's output. The server is captured by dispatch teardown, and its
// reactor attachment cannot detach until that reactor's sessions Stop/Wait.
func sourceGenerationCleanup(server *pluginserver.Server, proc *process.Process) {
	sourceLeaseCleanup(proc)
	owner, captured := proc.SelectedOwnerForCleanup()
	if !captured {
		return
	}
	keep := proc.Config().OnExit == plugin.OnExitKeep
	if origins, ok := server.Reactor().(interface {
		EndOriginOwnerWithPolicy(context.Context, plugin.SelectedOwner, bool) error
	}); ok {
		ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
		err := origins.EndOriginOwnerWithPolicy(ctx, owner, keep)
		cancel()
		if err != nil {
			message := report.FitMessage("Origin cleanup for stopped process " + proc.Name() + " failed: " + err.Error())
			report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, proc.Name(), message, "ze show bgp peers")
			peerLogger().Error(message, "code", codeSourceLeaseCleanup, "plugin", proc.Name(), "fix", "ze show bgp peers")
		}
	} else if origins, ok := server.Reactor().(interface {
		EndOriginOwner(context.Context, plugin.SelectedOwner) error
	}); ok {
		ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
		err := origins.EndOriginOwner(ctx, owner)
		cancel()
		if err != nil {
			message := report.FitMessage("Origin cleanup for stopped process " + proc.Name() + " failed: " + err.Error())
			report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, proc.Name(), message, "ze show bgp peers")
			peerLogger().Error(message, "code", codeSourceLeaseCleanup, "plugin", proc.Name(), "fix", "ze show bgp peers")
		}
	}
	retirement, supported := server.Reactor().(interface {
		RetireSelectedOwner(context.Context, plugin.SelectedOwner) error
	})
	if !supported {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	if err := retirement.RetireSelectedOwner(ctx, owner); err != nil {
		message := report.FitMessage("Selected output for stopped process " + proc.Name() + " could not be withdrawn; affected destination sessions were retired: " + err.Error())
		report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, proc.Name(), message, "ze show bgp peers")
		peerLogger().Error(message, "code", codeSourceLeaseCleanup, "plugin", proc.Name(), "fix", "ze show bgp peers")
	}
}

// sourceLeaseCleanup uses the captured ended token, not a live constructor or
// name lookup. The existing process-generation cleanup invokes it after request
// dispatch drains and before publishing a replacement incarnation.
func sourceLeaseCleanup(proc *process.Process) {
	owner, captured := proc.SelectedOwnerForCleanup()
	if !captured {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	if err := sourceReceipts.revokeOwner(ctx, owner); err != nil {
		message := report.FitMessage("Source leases for stopped process " + proc.Name() + " were revoked, but their final writes did not finish: " + err.Error())
		report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, proc.Name(), message, "ze request restart")
		peerLogger().Error(message, "code", codeSourceLeaseCleanup, "plugin", proc.Name(), "fix", "ze request restart")
	}
}

// completeSourceLeaseRemoval is a terminal barrier after Cease/stop and outside
// reactor/peer locks. A context-free stop cannot return an error; after its
// bounded attempt fails it continues joining the already-revoked guards and
// reports the unresolved peer names rather than claiming early completion.
func completeSourceLeaseRemoval(peers ...*Peer) {
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	err := sourceReceipts.revokePeers(ctx, peers...)
	for _, peer := range peers {
		if joinErr := peer.joinSentObservations(ctx); joinErr != nil && err == nil {
			err = joinErr
		}
	}
	cancel()
	if err == nil {
		return
	}
	message := report.FitMessage("Removed BGP peers have revoked authority, but final writes or SENT publications missed their join deadline: " + err.Error())
	report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, "source-removal", message, "ze request restart")
	peerLogger().Error(message, "code", codeSourceLeaseCleanup, "fix", "ze request restart")
	// All rights were revoked before the timed wait. The caller has no error
	// channel and MUST keep its barrier closed until their final cleanup ends.
	if err := sourceReceipts.revokePeers(context.Background(), peers...); err != nil {
		panic("BUG: noncancelable source removal join failed: " + err.Error())
	}
	for _, peer := range peers {
		if err := peer.joinSentObservations(context.Background()); err != nil {
			panic("BUG: noncancelable SENT publication join failed: " + err.Error())
		}
	}
}
