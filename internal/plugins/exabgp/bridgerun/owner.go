// Design: docs/architecture/exabgp-bridge.md -- a script that exits has its routes withdrawn, or kept
// Related: script.go -- script.batch notes each dispatched route, script.exited withdraws them
// Related: fleet.go -- the Fleet holds the one ledger every script shares

package bridgerun

import (
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/exabgp/bridge"
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
func exitPolicy(onExit OnExit, version bridge.APIVersion) (withdraw bool, by string) {
	switch onExit {
	case OnExitKeep:
		return false, "on-exit"
	case OnExitWithdraw:
		return true, "on-exit"
	case OnExitByVersion:
	}
	switch version {
	case bridge.APIV4:
		return false, "API 4"
	case bridge.APIV6:
		return true, "API 6"
	case bridge.APIVersionUnspecified, bridge.APIVersionAuto:
	}
	return true, "API undecided"
}

// ownerLedger records which script owns each route the bridge announced, so
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
type ownerLedger struct {
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

func newOwnerLedger() *ownerLedger {
	return &ownerLedger{routes: make(map[ownedKey]ownedRoute)}
}

// note records one command the runner dispatched for script on selector. An
// announce makes script the route's owner; a withdrawal clears the route,
// whoever owned it. A command that carries no route changes nothing.
func (l *ownerLedger) note(script, selector string, command bridge.Command) {
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
	l.routes[key] = ownedRoute{script: script, selector: selector, withdrawal: command.Withdrawal}
}

// release removes every route script owns and answers them, ordered by their
// withdrawal command, so the exit dispatches them in a stable order.
func (l *ownerLedger) release(script string) []ownedRoute {
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
