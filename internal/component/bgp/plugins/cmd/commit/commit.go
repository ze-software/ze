// Design: docs/architecture/api/commands.md — BGP commit workflow handlers

package commit

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingCommitArguments   = errors.New("missing commit arguments")
	errMissingWithdrawArguments = errors.New("missing withdraw arguments")
	errExpectedRouteKeyword     = errors.New("expected 'route' keyword")
	errMissingPrefix            = errors.New("missing prefix")
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
			Error:  "commit manager not available",
		}, ErrCommitManagerNotAvailable
	}
	typed, ok := cm.(*transaction.CommitManager)
	if !ok {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "commit manager not available",
		}, ErrCommitManagerTypeAssertionFailed
	}
	return typed, nil, nil
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:commit", Handler: handleCommit},
	)
}

// Commit action constants.
const (
	actionEnd      = "end"
	actionEOR      = "eor"
	actionStart    = "start"
	actionRollback = "rollback"
	actionShow     = "show"
	actionWithdraw = "withdraw"
	actionList     = "list"
)

// commitActionKeywords is the set of valid commit action keywords.
var commitActionKeywords = map[string]bool{
	actionList: true, actionStart: true, actionEnd: true, actionEOR: true,
	actionRollback: true, actionShow: true, actionWithdraw: true,
}

// handleCommit dispatches commit subcommands.
//
// Canonical grammar (action before identifier):
//
//	commit list                          - list active commits
//	commit start <name>                  - start named commit
//	commit end <name>                    - flush without EOR
//	commit eor <name>                    - flush with EOR
//	commit rollback <name>               - discard queued routes
//	commit show <name>                   - show queued count
//	commit withdraw <name> route <pfx>   - withdraw a route
//
// Deprecated grammar (accepted with deprecation warning):
//
//	commit <name> start|end|eor|rollback|show|withdraw ...
func handleCommit(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: commit start|end|eor|rollback|show|withdraw <name> or commit list",
		}, errMissingCommitArguments
	}

	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	switch args[0] {
	case actionList:
		return handleCommitList(ctx)
	case actionStart, actionEnd, actionEOR, actionRollback, actionShow:
		if len(args) < 2 {
			var tb textbuf.Buffer
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  tb.Str("usage: commit ").Str(args[0]).Str(" <name>").String(),
			}, fmt.Errorf("missing name for commit %s", args[0])
		}
		return dispatchCommitAction(ctx, args[0], args[1], args[2:], false)
	case actionWithdraw:
		if len(args) < 2 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "usage: commit withdraw <name> route <prefix>",
			}, errMissingWithdrawArguments
		}
		return dispatchCommitAction(ctx, actionWithdraw, args[1], args[2:], false)
	}

	// Deprecated: commit <name> <action> [args...]
	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: commit start|end|eor|rollback|show|withdraw <name>",
		}, fmt.Errorf("missing action for commit %q", args[0])
	}
	if !commitActionKeywords[args[1]] {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("unknown commit action: ").Str(args[1]).String(),
		}, fmt.Errorf("unknown commit action: %s", args[1])
	}
	return dispatchCommitAction(ctx, args[1], args[0], args[2:], true)
}

// dispatchCommitAction routes a commit action to the right handler.
func dispatchCommitAction(ctx *pluginserver.CommandContext, action, name string, extraArgs []string, deprecated bool) (*plugin.Response, error) {
	withDeprecation := func(resp *plugin.Response, err error) (*plugin.Response, error) {
		if !deprecated || resp == nil || resp.Status != plugin.StatusDone {
			return resp, err
		}
		var tb textbuf.Buffer
		newForm := tb.Str("commit ").Str(action).Byte(' ').Str(name).String()
		if data, ok := resp.Data.(plugin.Map); ok {
			data["deprecated"] = tb.Reset().Str("use: ").Str(newForm).String()
		}
		return resp, err
	}

	switch action {
	case actionStart:
		return withDeprecation(handleNamedCommitStart(ctx, name))
	case actionEnd:
		return withDeprecation(handleNamedCommitEnd(ctx, name, false))
	case actionEOR:
		return withDeprecation(handleNamedCommitEnd(ctx, name, true))
	case actionRollback:
		return withDeprecation(handleNamedCommitRollback(ctx, name))
	case actionShow:
		return withDeprecation(handleNamedCommitShow(ctx, name))
	case actionWithdraw:
		if len(extraArgs) == 0 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "usage: commit withdraw <name> route <prefix>",
			}, errMissingWithdrawArguments
		}
		return withDeprecation(handleNamedCommitWithdraw(ctx, name, extraArgs))
	default:
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("unknown commit action: ").Str(action).String(),
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
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("failed to start commit: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("commit failed: %v", err),
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
			Data: plugin.Map{
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
	result, err := bgpReactor.SendRoutes(selector.ParseDefault(tx.PeerSelector()), routes, withdrawals, sendEOR)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("failed to send routes: %v", err),
		}, err
	}

	action := actionEnd
	if sendEOR {
		action = actionEOR
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("rollback failed: %v", err),
		}, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	families := tx.Families()
	familyStrs := make([]string, len(families))
	for i, f := range families {
		var b textbuf.Buffer
		familyStrs[i] = b.Reset().Int(int64(f.AFI)).Byte('/').Int(int64(f.SAFI)).String()
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
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
			Error:  fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	// args[0] should be "route"
	if len(args) < 1 || !strings.EqualFold(args[0], "route") {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: commit <name> withdraw route <prefix>",
		}, errExpectedRouteKeyword
	}

	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: commit <name> withdraw route <prefix>",
		}, errMissingPrefix
	}

	// Parse prefix
	prefix, err := netip.ParsePrefix(args[1])
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("invalid prefix: ").Str(args[1]).String(),
		}, err
	}

	// Build NLRI
	var n nlri.NLRI
	if prefix.Addr().Is4() {
		n = nlri.NewINET(family.IPv4Unicast, prefix, 0)
	} else {
		n = nlri.NewINET(family.IPv6Unicast, prefix, 0)
	}

	// Queue withdrawal
	tx.QueueWithdraw(n)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"commit":      name,
			"prefix":      prefix.String(),
			"withdrawals": tx.WithdrawalCount(),
		},
	}, nil
}
