// Design: docs/architecture/irr-owner.md -- the IRR owner plugin entry point
// Related: register.go -- the registration, the state grants and the YANG
// Related: register_commands.go -- the daemon forwarders of the irr commands
//
// Package irr is the one owner of the IRR data Ze enforces. It fetches the
// prefix list of every AS-SET a consumer (bgp-filter-irr, firewall-irr)
// names, schedules each refresh from the time the server last answered, and
// stores each list in the daemon's store, so a restart enforces the lists it
// enforced before. The consumers never fetch: they read what this plugin
// stored and learn of a new copy from its irr/dataset-stored notice.
//
// The registration, the YANG (`system irr`, `show irr`, `update irr`,
// `clear irr asn|as-set`), the state grants and the startup subscription are
// in place. Reading `system irr`, the schedule, the fetch and the store are
// not: every irr command answers that it is not available yet, and the
// consumers still fetch for themselves.
package irr

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/core/diagnostic"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
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

	// The config is delivered and not read yet: nothing here parses system irr,
	// so a reload has nothing for this plugin to apply.
	p.OnConfigure(func(_ []sdk.ConfigSection) error { return nil })
	p.DeclareAppliesNothing("the irr plugin does not read system irr yet, so a reload has nothing for it to apply")

	// A consumer's interest notice tells the owner what to fetch. Subscribed at
	// stage 5, before any event flows, so a notice a consumer sends from its
	// own OnStarted is delivered.
	p.AddStartupSubscriptionsIn(store.NoticeNamespace, []string{store.EventInterestChanged})
	p.OnEvent(func(event string) error {
		logger().Debug("irr: interest notice received, not acted on yet", "event", event)
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		return handleCommand(command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		Commands:    commandDecls(),
		WantsConfig: []string{configRootSystem},
		// The stored datasets outlive this process, so a crashed owner comes
		// back and resumes the schedule; the consumers keep enforcing the
		// stored copies meanwhile.
		FailurePolicy: sdk.FailureRestart,
	}); err != nil {
		logger().Error("irr plugin failed", "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		return 1
	}
	return 0
}

// handleCommand answers every declared command with an operator error: the
// registration is wired and the fetch, store and removal behind it are not.
func handleCommand(command string) (string, any, error) {
	switch command {
	case cmdShowIRR, cmdUpdateIRRAll, cmdUpdateIRRAsn, cmdUpdateIRRAsSet, cmdClearIRRAsn, cmdClearIRRAsSet:
		return statusError, nil, errNotAvailable(command)
	}
	return statusError, nil, errUnknownCommand
}

// errNotAvailable is the refusal of a command whose behavior the irr plugin
// does not hold yet. The consumers still fetch for themselves, so the fix
// names their own commands.
func errNotAvailable(command string) *opererr.Error {
	return opererr.New(command+" is not available yet",
		"The irr plugin is registered, but it does not fetch, store or remove IRR datasets yet; bgp-filter-irr and firewall-irr still fetch for themselves.",
		"update bgp irr as-set <name>, or update firewall irr as-set <name>")
}
