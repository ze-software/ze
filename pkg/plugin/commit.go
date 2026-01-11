package plugin

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// Commit action constants.
const (
	actionEnd = "end"
	actionEOR = "eor"
)

// RegisterCommitHandlers registers commit-related command handlers.
func RegisterCommitHandlers(d *Dispatcher) {
	d.Register("commit", handleCommit, "Named commit operations (commit <name> start|end|eor|rollback|show, commit list)")
}

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
func handleCommit(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) == 0 {
		return &Response{
			Status: "error",
			Data:   "usage: commit <name> <start|end|eor|rollback|show> or commit list",
		}, fmt.Errorf("missing commit arguments")
	}

	// Special case: commit list (no name)
	if args[0] == "list" {
		return handleCommitList(ctx)
	}

	if len(args) < 2 {
		return &Response{
			Status: "error",
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
			return &Response{
				Status: "error",
				Data:   "usage: commit <name> withdraw route <prefix>",
			}, fmt.Errorf("missing withdraw arguments")
		}
		return handleNamedCommitWithdraw(ctx, name, args[2:])
	default:
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("unknown commit action: %s", action),
		}, fmt.Errorf("unknown commit action: %s", action)
	}
}

// handleCommitList returns all active commit names.
func handleCommitList(ctx *CommandContext) (*Response, error) {
	names := ctx.CommitManager.List()
	return &Response{
		Status: "done",
		Data: map[string]any{
			"commits": names,
			"count":   len(names),
		},
	}, nil
}

// handleNamedCommitStart begins a new named commit.
func handleNamedCommitStart(ctx *CommandContext, name string) (*Response, error) {
	peerSelector := ctx.PeerSelector()

	if err := ctx.CommitManager.Start(name, peerSelector); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to start commit: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"commit":  name,
			"peer":    peerSelector,
			"message": "commit started",
		},
	}, nil
}

// handleNamedCommitEnd flushes the named commit.
// If sendEOR is true, sends EOR for affected families after routes.
func handleNamedCommitEnd(ctx *CommandContext, name string, sendEOR bool) (*Response, error) {
	tx, err := ctx.CommitManager.End(name)
	if err != nil {
		return &Response{
			Status: "error",
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
		return &Response{
			Status: "done",
			Data: map[string]any{
				"commit":  name,
				"action":  action,
				"queued":  0,
				"message": "commit empty, nothing sent",
			},
		}, nil
	}

	// Send routes to matching peers via Reactor
	result, err := ctx.Reactor.SendRoutes(tx.PeerSelector(), routes, withdrawals, sendEOR)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to send routes: %v", err),
		}, err
	}

	action := actionEnd
	if sendEOR {
		action = actionEOR
	}

	return &Response{
		Status: "done",
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
func handleNamedCommitRollback(ctx *CommandContext, name string) (*Response, error) {
	discarded, err := ctx.CommitManager.Rollback(name)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("rollback failed: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"commit":           name,
			"routes_discarded": discarded,
			"message":          "commit rolled back",
		},
	}, nil
}

// handleNamedCommitShow returns info about a pending commit.
func handleNamedCommitShow(ctx *CommandContext, name string) (*Response, error) {
	tx, err := ctx.CommitManager.Get(name)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	families := tx.Families()
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = fmt.Sprintf("%d/%d", f.AFI, f.SAFI)
	}

	return &Response{
		Status: "done",
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
func handleNamedCommitWithdraw(ctx *CommandContext, name string, args []string) (*Response, error) {
	tx, err := ctx.CommitManager.Get(name)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	// args[0] should be "route"
	if len(args) < 1 || !strings.EqualFold(args[0], "route") {
		return &Response{
			Status: "error",
			Data:   "usage: commit <name> withdraw route <prefix>",
		}, fmt.Errorf("expected 'route' keyword")
	}

	if len(args) < 2 {
		return &Response{
			Status: "error",
			Data:   "usage: commit <name> withdraw route <prefix>",
		}, fmt.Errorf("missing prefix")
	}

	// Parse prefix
	prefix, err := netip.ParsePrefix(args[1])
	if err != nil {
		return &Response{
			Status: "error",
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

	return &Response{
		Status: "done",
		Data: map[string]any{
			"commit":      name,
			"prefix":      prefix.String(),
			"withdrawals": tx.WithdrawalCount(),
		},
	}, nil
}
