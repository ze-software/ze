// Design: docs/architecture/api/commands.md — BGP commit workflow handlers
// Overview: doc.go — bgp-cmd-commit plugin registration

package commit

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// Sentinel errors for commit handlers.
var (
	// ErrCommitManagerNotAvailable is returned when the commit manager is nil.
	ErrCommitManagerNotAvailable = errors.New("commit manager not available")

	// ErrCommitManagerTypeAssertionFailed is returned when the commit manager
	// cannot be type-asserted to *transaction.CommitManager.
	ErrCommitManagerTypeAssertionFailed = errors.New("commit manager type assertion failed")
)

// requireCommitManager returns the commit manager or an error response.
// Type-asserts from the opaque any stored in plugin.Server.
func requireCommitManager(ctx *pluginserver.CommandContext) (*transaction.CommitManager, *plugin.Response, error) {
	cm := ctx.CommitManager()
	if cm == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "commit manager not available",
		}, ErrCommitManagerNotAvailable
	}
	typed, ok := cm.(*transaction.CommitManager)
	if !ok {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "commit manager not available",
		}, ErrCommitManagerTypeAssertionFailed
	}
	return typed, nil, nil
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:commit", CLICommand: "bgp commit", Handler: handleCommit, Help: "Named commit operations"},
	)
}

// Commit action constants.
const (
	actionEnd = "end"
	actionEOR = "eor"
)

// handleCommit dispatches commit subcommands.
//
// Syntax:
//
//	commit list                     - list active commits
//	commit <name> start             - start named commit
//	commit <name> end               - flush without EOR
//	commit <name> eor               - flush with EOR
//	commit <name> rollback          - discard queued routes
//	commit <name> show              - show queued count
func handleCommit(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: commit <name> <start|end|eor|rollback|show> or commit list",
		}, fmt.Errorf("missing commit arguments")
	}

	// Guard reactor access
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Special case: commit list (no name)
	if args[0] == "list" {
		return handleCommitList(ctx)
	}

	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: commit <name> <start|end|eor|rollback|show>",
		}, fmt.Errorf("missing action for commit %q", args[0])
	}

	name := args[0]
	action := args[1]

	switch action {
	case "start":
		return handleNamedCommitStart(ctx, name)
	case actionEnd:
		return handleNamedCommitEnd(ctx, name, false)
	case actionEOR:
		return handleNamedCommitEnd(ctx, name, true)
	case "rollback":
		return handleNamedCommitRollback(ctx, name)
	case "show":
		return handleNamedCommitShow(ctx, name)
	case "withdraw":
		// commit <name> withdraw route <prefix>
		if len(args) < 3 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   "usage: commit <name> withdraw route <prefix>",
			}, fmt.Errorf("missing withdraw arguments")
		}
		return handleNamedCommitWithdraw(ctx, name, args[2:])
	default: // unknown commit action — return explicit error
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("unknown commit action: %s", action),
		}, fmt.Errorf("unknown commit action: %s", action)
	}
}

// handleCommitList returns all active commit names.
func handleCommitList(ctx *pluginserver.CommandContext) (*plugin.Response, error) {
	cm, errResp, err := requireCommitManager(ctx)
	if err != nil {
		return errResp, err
	}
	names := cm.List()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commits": names,
			"count":   len(names),
		},
	}, nil
}

// handleNamedCommitStart begins a new named commit.
func handleNamedCommitStart(ctx *pluginserver.CommandContext, name string) (*plugin.Response, error) {
	cm, errResp, err := requireCommitManager(ctx)
	if err != nil {
		return errResp, err
	}
	peerSelector := ctx.PeerSelector()

	if err := cm.Start(name, peerSelector); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to start commit: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commit":  name,
			"peer":    peerSelector,
			"message": "commit started",
		},
	}, nil
}

// handleNamedCommitEnd flushes the named commit.
// If sendEOR is true, sends EOR for affected families after routes.
func handleNamedCommitEnd(ctx *pluginserver.CommandContext, name string, sendEOR bool) (*plugin.Response, error) {
	cm, errResp, cmErr := requireCommitManager(ctx)
	if cmErr != nil {
		return errResp, cmErr
	}
	tx, err := cm.End(name)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("commit failed: %v", err),
		}, err
	}

	// Get routes and withdrawals from transaction
	routes := tx.Routes()
	withdrawals := tx.Withdrawals()

	if len(routes) == 0 && len(withdrawals) == 0 {
		// No routes to send
		action := actionEnd
		if sendEOR {
			action = actionEOR
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"commit":  name,
				"action":  action,
				"queued":  0,
				"message": "commit empty, nothing sent",
			},
		}, nil
	}

	// Send routes to matching peers via BGP Reactor
	bgpReactor, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}
	result, err := bgpReactor.SendRoutes(tx.PeerSelector(), routes, withdrawals, sendEOR)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to send routes: %v", err),
		}, err
	}

	action := actionEnd
	if sendEOR {
		action = actionEOR
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commit":           name,
			"action":           action,
			"peer":             tx.PeerSelector(),
			"routes_announced": result.RoutesAnnounced,
			"routes_withdrawn": result.RoutesWithdrawn,
			"updates_sent":     result.UpdatesSent,
			"families":         result.Families,
			"eor_sent":         sendEOR,
		},
	}, nil
}

// handleNamedCommitRollback discards all queued routes in the commit.
func handleNamedCommitRollback(ctx *pluginserver.CommandContext, name string) (*plugin.Response, error) {
	cm, errResp, cmErr := requireCommitManager(ctx)
	if cmErr != nil {
		return errResp, cmErr
	}
	discarded, err := cm.Rollback(name)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("rollback failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commit":           name,
			"routes_discarded": discarded,
			"message":          "commit rolled back",
		},
	}, nil
}

// handleNamedCommitShow returns info about a pending commit.
func handleNamedCommitShow(ctx *pluginserver.CommandContext, name string) (*plugin.Response, error) {
	cm, errResp, cmErr := requireCommitManager(ctx)
	if cmErr != nil {
		return errResp, cmErr
	}
	tx, err := cm.Get(name)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	families := tx.Families()
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = fmt.Sprintf("%d/%d", f.AFI, f.SAFI)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commit":      name,
			"peer":        tx.PeerSelector(),
			"queued":      tx.Count(),
			"withdrawals": tx.WithdrawalCount(),
			"families":    familyStrs,
		},
	}, nil
}

// handleNamedCommitWithdraw queues a route withdrawal to a named commit.
// Syntax: commit <name> withdraw route <prefix>.
func handleNamedCommitWithdraw(ctx *pluginserver.CommandContext, name string, args []string) (*plugin.Response, error) {
	cm, errResp, cmErr := requireCommitManager(ctx)
	if cmErr != nil {
		return errResp, cmErr
	}
	tx, err := cm.Get(name)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	// args[0] should be "route"
	if len(args) < 1 || !strings.EqualFold(args[0], "route") {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: commit <name> withdraw route <prefix>",
		}, fmt.Errorf("expected 'route' keyword")
	}

	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: commit <name> withdraw route <prefix>",
		}, fmt.Errorf("missing prefix")
	}

	// Parse prefix
	prefix, err := netip.ParsePrefix(args[1])
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("invalid prefix: %s", args[1]),
		}, err
	}

	// Build NLRI
	var n nlri.NLRI
	if prefix.Addr().Is4() {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	} else {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
	}

	// Queue withdrawal
	tx.QueueWithdraw(n)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commit":      name,
			"prefix":      prefix.String(),
			"withdrawals": tx.WithdrawalCount(),
		},
	}, nil
}
