// Design: docs/architecture/irr-owner.md -- the IRR owner plugin entry point
// Related: register.go -- the registration, the state grants and the YANG
// Related: register_commands.go -- the daemon forwarders of the irr commands
// Related: owner.go -- the worker that schedules, fetches and stores
// Related: config.go -- system irr and system peeringdb url
//
// Package irr is the one owner of the IRR data Ze enforces. It fetches the
// prefix list of every AS-SET a consumer (bgp-filter-irr, firewall-irr)
// names, schedules each refresh from the time the server last answered, and
// stores each list in the daemon's store, so a restart enforces the lists it
// enforced before. The consumers never fetch: they read what this plugin
// stored and learn of a new copy from its irr/dataset-stored notice.
//
// runIRR wires the owner to the SDK: the config transaction installs
// system irr, the stage-5 subscription delivers interest notices, OnStarted
// starts the worker (state RPCs work only after the handshake, so nothing is
// read or fetched inside it), and the commands, the doctor callback and the
// resolver-cache clear reach the worker.
package irr

import (
	"context"
	"errors"
	"net"

	"github.com/ze-software/ze/internal/core/diagnostic"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var logger = slogutil.LazyLogger("irr")

// pluginName is the registered plugin name, which the consumers name in their
// Dependencies so the engine starts this plugin before them, and
// configRootSystem is the root it reads: system irr, and system peeringdb url
// for the AS-SET of a bare ASN.
const (
	pluginName       = store.OwnerPlugin
	configRootSystem = "system"
	// configPathIRR is the one system container this plugin owns. Auto-load
	// matches ConfigRoots against every container path of the config, so the
	// bare system root would start the plugin for any system block.
	configPathIRR = configRootSystem + "/irr"
)

// The commands the plugin answers, as the dispatcher forwards them
// (register_commands.go) and as the Stage 1 declaration lists them (commandDecls).
const (
	cmdShowIRR        = "show irr"
	cmdUpdateIRRAll   = "update irr all"
	cmdUpdateIRRAsn   = "update irr asn"
	cmdUpdateIRRAsSet = "update irr as-set"
	cmdClearIRRAsn    = "clear irr asn"
	cmdClearIRRAsSet  = "clear irr as-set"
)

const statusError = "error"

// errUnknownCommand answers a command this plugin does not declare.
var errUnknownCommand = errors.New("irr: unknown command")

// commandDecls is the Stage 1 command declaration. registry.Registration and
// the sdk.Registration passed to Run both take it, so the catalog and the
// running daemon agree (./le plugin declarations check).
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: cmdShowIRR, ShortHelp: "Show the stored IRR datasets and their refresh schedule"},
		{Name: cmdUpdateIRRAll, ShortHelp: "Fetch every referenced IRR dataset now"},
		{Name: cmdUpdateIRRAsn, ShortHelp: "Fetch the IRR dataset of an AS number now", Args: []string{"<asn>"}},
		{Name: cmdUpdateIRRAsSet, ShortHelp: "Fetch the IRR dataset of an AS-SET now", Args: []string{"<as-set>"}},
		{Name: cmdClearIRRAsn, ShortHelp: "Remove the stored IRR dataset of an AS number", Args: []string{"<asn>"}},
		{Name: cmdClearIRRAsSet, ShortHelp: "Remove the stored IRR dataset of an AS-SET", Args: []string{"<as-set>"}},
	}
}

func runIRR(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	plug := newOwner(p, p, newStoreResolver)
	sdk.NewConfigTx(defaultOwnerConfig(), parseOwnerConfig, plug.configure).Register(p)

	// A consumer's interest notice tells the owner to read the interest keys
	// again. Subscribed at stage 5, before any event flows, so a notice a
	// consumer sends from its own OnStarted is delivered.
	p.AddStartupSubscriptionsIn(store.NoticeNamespace, []string{store.EventInterestChanged})
	p.OnEvent(func(_ string) error {
		plug.interestChanged()
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	p.OnStarted(func(_ context.Context) error {
		plug.start(ctx)
		return nil
	})
	defer plug.stopped()

	p.OnExecuteCommand(func(_, command string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
		return plug.handleCommand(command, args)
	})
	p.OnDoctorCheck(plug.answerDoctorCheck)
	// Only the owner holds IRR and PeeringDB clients for enforced data, so
	// clear irr cache and clear peeringdb cache reach it.
	p.OnResolverCacheClear(store.ResolverCaches(), plug.clearResolverCache)

	if err := p.Run(ctx, &sdk.Registration{
		Commands:     commandDecls(),
		WantsConfig:  []string{configRootSystem},
		DoctorChecks: []sdk.DoctorCheckDecl{datasetCheckDecl},
		// The stored datasets outlive this process, so a crashed owner comes
		// back and resumes the schedule; the consumers keep enforcing the
		// stored copies meanwhile.
		FailurePolicy: sdk.FailureRestart,
	}); err != nil {
		logger().Error("irr plugin failed", "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		cancel()
		return 1
	}
	cancel()
	return 0
}

// clearResolverCache empties one client cache on the worker, which owns the
// resolver.
func (o *owner) clearResolverCache(input rpc.ResolverCacheClearInput) (int, error) {
	var removed int
	answer := o.do(func(_ context.Context) commandAnswer {
		count, err := o.res.clearCache(input)
		removed = count
		return commandAnswer{err: err}
	})
	return removed, answer.err
}
