// Design: docs/architecture/exabgp-bridge.md -- a script that exits has its routes withdrawn, or kept
// Related: bridge_lines.go -- LineRunner.Dispatched notes each route command ze accepted
// Related: bridge.go -- the standalone runner settles its child's exit here

package bridge

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// OnExit is ExaBGP's `on-exit` leaf: what becomes of the routes a script
// announced when it exits.
type OnExit uint8

const (
	// OnExitByVersion is the leaf left unset, and it is a decision, not an
	// absence: a script held to API 4 keeps its routes, as 4.x and 5.x did,
	// and any other has them withdrawn (exitPolicy).
	OnExitByVersion OnExit = iota
	// OnExitKeep is `on-exit keep`: the routes stay whatever the version.
	OnExitKeep
	// OnExitWithdraw is `on-exit withdraw`: the routes are withdrawn whatever
	// the version.
	OnExitWithdraw
)

// exitPolicy is ExaBGP's Processes._on_exit (src/exabgp/reactor/api/processes.py
// at the pin): whether an exited script's routes are withdrawn, and the reason
// the exit log names, `on-exit`, `API 4`, `API 6` or `API undecided`.
func exitPolicy(onExit OnExit, version APIVersion) (withdraw bool, by string) {
	switch onExit {
	case OnExitKeep:
		return false, "on-exit"
	case OnExitWithdraw:
		return true, "on-exit"
	case OnExitByVersion:
	}
	switch version {
	case APIV4:
		return false, "API 4"
	case APIV6:
		return true, "API 6"
	case APIVersionUnspecified, APIVersionAuto:
	}
	return true, "API undecided"
}

// ExitRunner is how an exit reaches ze: the withdrawal of each owned route,
// then a flush of each selector they addressed. Every LineRunner is one.
type ExitRunner interface {
	// Dispatch hands ze one command and answers its refusal.
	Dispatch(ctx context.Context, command string) error
	// Flush drains the forward pool of the peers selector names.
	Flush(ctx context.Context, selector string)
}

// OwnerLedger records which script owns each route the bridge announced, so
// the routes of a script that exits can be withdrawn and no other's.
//
// It is ExaBGP's owner rule (rib/outgoing.py, withdraw_owner): the last
// announcer of a route owns it, and a withdraw from anywhere clears it. A
// route is one selector and one RouteKey identity, as the runner dispatched
// them, so `neighbor 127.0.0.1` and `peer *` name two entries even where they
// reach one session.
//
// Its size is bounded by the routes the scripts currently hold announced: a
// withdraw removes its entry, and an exit removes every entry of the script.
// Safe for concurrent use: every script's batch goroutine notes into it.
type OwnerLedger struct {
	mu     sync.Mutex
	routes map[ownedKey]ownedRoute
}

// ownedKey is one route on the selector it was sent to.
type ownedKey struct {
	selector string
	family   string
	nlri     string
}

// ownedRoute is who announced a route last, and the command that withdraws it.
type ownedRoute struct {
	script     string
	selector   string
	withdrawal string
}

// NewOwnerLedger answers an empty ledger. A runner holds one for every script
// it runs, because ownership passes between scripts.
func NewOwnerLedger() *OwnerLedger {
	return &OwnerLedger{routes: make(map[ownedKey]ownedRoute)}
}

// Note records one command the runner dispatched for script on selector. An
// announce makes script the route's owner; a withdrawal clears the route,
// whoever owned it. A command that carries no route changes nothing.
func (l *OwnerLedger) Note(script, selector string, command Command) {
	if !command.Key.Route() {
		return
	}
	key := ownedKey{selector: selector, family: command.Key.Family, nlri: command.Key.NLRI}

	l.mu.Lock()
	defer l.mu.Unlock()

	if command.Key.Withdraw {
		delete(l.routes, key)
		return
	}
	if command.Withdrawal == "" {
		// An announce whose builder wrote no withdrawal cannot be withdrawn
		// on exit, so it is not recorded as owned: an entry with nothing to
		// send would be a withdrawal the exit silently skips.
		return
	}
	l.routes[key] = ownedRoute{script: script, selector: selector, withdrawal: restoringWithdrawal(command.Withdrawal)}
}

// updateTextWords is where every route builder's withdrawal names its grammar.
const updateTextWords = " update text "

// restoringWithdrawal is a route's withdrawal as an exit sends it: with
// `restore-configured`, so a route that replaced one the peer's config holds
// gives the configured route back rather than leaving the prefix unannounced
// (ExaBGP's withdraw_owner, rib/outgoing.py). A withdrawal written in another
// grammar is sent as written: no configured route is named by it.
func restoringWithdrawal(withdrawal string) string {
	head, tail, found := strings.Cut(withdrawal, updateTextWords)
	if !found {
		return withdrawal
	}
	return head + updateTextWords + "restore-configured " + tail
}

// release removes every route script owns and answers them, ordered by their
// withdrawal command, so the exit dispatches them in a stable order.
func (l *OwnerLedger) release(script string) []ownedRoute {
	l.mu.Lock()
	defer l.mu.Unlock()

	var owned []ownedRoute
	for key, route := range l.routes {
		if route.script != script {
			continue
		}
		owned = append(owned, route)
		delete(l.routes, key)
	}
	slices.SortFunc(owned, func(a, b ownedRoute) int { return strings.Compare(a.withdrawal, b.withdrawal) })
	return owned
}

// Exited settles the routes of a script whose child has exited, as ExaBGP's
// Processes._queue_exit does: the `on-exit` leaf, or the API version the
// child was held to, decides whether they are withdrawn (exitPolicy), the
// decision is logged with its reason, and a withdrawal sends each route the
// script owns its own withdrawal command through runner, then flushes each
// selector they addressed.
//
// The caller MUST call it after the child's last batch is dispatched and
// before another child of the script starts, with the version that child was
// held to.
func (l *OwnerLedger) Exited(ctx context.Context, log *slog.Logger, script string, onExit OnExit, version APIVersion, runner ExitRunner) {
	withdraw, by := exitPolicy(onExit, version)
	routes := "keep"
	if withdraw {
		routes = "withdraw"
	}
	log.Info("exabgp-bridge process exited", "process", script, "routes", routes, "by", by)
	if !withdraw {
		return
	}

	var selectors []string
	for _, route := range l.release(script) {
		if err := runner.Dispatch(ctx, route.withdrawal); err != nil {
			log.Warn("exabgp-bridge exit withdrawal failed",
				"script", script, "error", err, "cmd", route.withdrawal)
			continue
		}
		if !slices.Contains(selectors, route.selector) {
			selectors = append(selectors, route.selector)
		}
	}
	for _, selector := range selectors {
		runner.Flush(ctx, selector)
	}
}
