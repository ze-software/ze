// Design: docs/architecture/exabgp-bridge.md -- an API route naming a watchdog is the watchdog's
// Overview: server.go -- command dispatch and state management
// Related: pool.go -- the configured routes of each watchdog

package watchdog

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	commandRequestWatchdogGive   = "request bgp watchdog give"
	commandRequestWatchdogForget = "request bgp watchdog forget"

	// givenRoutesMax bounds the routes every owner together can give the
	// watchdogs: each one holds two command texts, and an owner that never
	// exits could otherwise grow the plugin without limit.
	givenRoutesMax = 1 << 20
)

// ErrGivenRoutesFull refuses a route given past givenRoutesMax.
var ErrGivenRoutesFull = errors.New("the watchdogs hold the most routes they can be given; " +
	"withdraw routes from them, or end the process that gave them, before giving more")

// givenRoute is a route a process gave a watchdog at runtime, with
// `request bgp watchdog give`, rather than one the config holds.
//
// ExaBGP's helpers give routes this way (c43f90f79: announce_route gives the
// route to add_to_rib_watchdog with its owner). The route is sent to its
// selector as the process wrote it, so the plugin keeps the two texts and
// the state alone: announce is the update text that sends it, withdrawal the
// one that withdraws it, both for selector.
type givenRoute struct {
	owner      string
	selector   string
	announce   string
	withdrawal string
	announced  bool
}

// givenRequest is one parsed `request bgp watchdog give`.
type givenRequest struct {
	name  string
	held  bool
	route givenRoute
}

// parseGivenRequest reads the arguments of `request bgp watchdog give`:
//
//	<name> owner <owner> selector <selector> [withdraw]
//	    announce <n> <n words of update text> withdrawal <update text>
//
// The announce is counted rather than ended by a keyword, because update
// text has no word that cannot appear in it.
func parseGivenRequest(args []string) (givenRequest, error) {
	const usage = "write request bgp watchdog give <name> owner <owner> selector <selector> [withdraw] " +
		"announce <n> <update text> withdrawal <update text>"
	if len(args) < 8 {
		return givenRequest{}, errors.New("request bgp watchdog give needs a name, an owner, a selector, an announce and a withdrawal; " + usage)
	}
	if args[1] != "owner" {
		return givenRequest{}, fmt.Errorf("request bgp watchdog give read %q where owner belongs; %s", args[1], usage)
	}
	if args[3] != "selector" {
		return givenRequest{}, fmt.Errorf("request bgp watchdog give read %q where selector belongs; %s", args[3], usage)
	}
	request := givenRequest{name: args[0], route: givenRoute{owner: args[2], selector: args[4]}}
	rest := args[5:]
	if rest[0] == "withdraw" {
		request.held = true
		rest = rest[1:]
	}
	if len(rest) < 2 || rest[0] != "announce" {
		return givenRequest{}, errors.New("request bgp watchdog give has no announce; " + usage)
	}
	words, err := strconv.Atoi(rest[1])
	if err != nil {
		return givenRequest{}, fmt.Errorf("request bgp watchdog give read %q as the announce word count: %w; %s", rest[1], err, usage)
	}
	rest = rest[2:]
	if words < 1 {
		return givenRequest{}, errors.New("request bgp watchdog give has an empty announce; " + usage)
	}
	if words+2 > len(rest) {
		return givenRequest{}, errors.New("request bgp watchdog give has fewer announce words than it counts, or no withdrawal; " + usage)
	}
	if rest[words] != "withdrawal" {
		return givenRequest{}, fmt.Errorf("request bgp watchdog give read %q where withdrawal belongs; %s", rest[words], usage)
	}
	request.route.announce = strings.Join(rest[:words], " ")
	request.route.withdrawal = strings.Join(rest[words+1:], " ")
	return request, nil
}

// givenKey is a given route's identity within its watchdog: its withdrawal,
// which names the selector, the family and the NLRI, as ExaBGP keys a
// watchdog's routes on route.index().
func givenKey(route *givenRoute) string { return route.selector + " " + route.withdrawal }

// handleGive gives one route to a watchdog: announced now, or, with
// `withdraw`, held until `request bgp watchdog announce <name>`. The same
// route given again replaces the first, with its new owner, as ExaBGP's
// add_to_rib_watchdog does. A route the engine refuses to send is not kept:
// the previous route under its key comes back, and the refusal is the answer,
// so the process reads an error rather than done for a route nobody sent.
func (s *watchdogServer) handleGive(args []string) (string, any, error) {
	request, err := parseGivenRequest(args)
	if err != nil {
		return statusError, nil, err
	}
	if request.name == medKeyword {
		return statusError, nil, errors.New("'med' is not allowed as a watchdog group name (ambiguous with med <N> argument)")
	}
	route := request.route
	route.announced = !request.held
	key := givenKey(&route)

	s.mu.Lock()
	routes := s.given[request.name]
	previous, replaced := routes[key]
	if !replaced && s.givenCount >= givenRoutesMax {
		s.mu.Unlock()
		return statusError, nil, ErrGivenRoutesFull
	}
	if routes == nil {
		routes = make(map[string]*givenRoute)
		s.given[request.name] = routes
	}
	if !replaced {
		s.givenCount++
	}
	routes[key] = &route
	s.mu.Unlock()

	if route.announced {
		if err := s.send(route.selector, route.announce); err != nil {
			s.ungive(request.name, key, &route, previous)
			return statusError, nil, givenSendError(request.name, route.selector, "announce", err)
		}
	}
	logger().Debug("watchdog route given", "pool", request.name, "owner", route.owner, "held", request.held)
	return statusDone, watchdogPeerResult{Peer: route.selector, Watchdog: request.name, Count: 1}, nil
}

// ungive takes back a route handleGive stored and could not send, putting
// back the route it replaced, if any. A later give of the same key in the
// meantime is left in place.
func (s *watchdogServer) ungive(name, key string, route, previous *givenRoute) {
	s.mu.Lock()
	defer s.mu.Unlock()

	routes := s.given[name]
	if routes[key] != route {
		return
	}
	if previous != nil {
		routes[key] = previous
		return
	}
	delete(routes, key)
	s.givenCount--
	if len(routes) == 0 {
		delete(s.given, name)
	}
}

// givenSendError is the answer to a command whose given route the engine
// refused to send: which watchdog, which selector, and the engine's reason,
// which names the fix (the send permission of the peer's attachment).
func givenSendError(name, selector, action string, err error) error {
	return fmt.Errorf("watchdog %s could not %s a route given for %s, so the route did not reach the peer: %w",
		name, action, selector, err)
}

// givenAction announces (announce=true) or withdraws the routes given to the
// watchdog name, for selector. "*" reaches every given route; any other
// selector reaches the routes given for exactly that selector. It answers how
// many routes the watchdog holds for it, moved or not, and every refused
// send joined: a route the engine refused keeps its former state, so the
// next action retries it.
func (s *watchdogServer) givenAction(name, selector string, announce bool) (int, error) {
	s.mu.Lock()
	var moved []*givenRoute
	held := 0
	for _, route := range s.given[name] {
		if selector != "*" && selector != route.selector {
			continue
		}
		held++
		if route.announced == announce {
			continue
		}
		route.announced = announce
		moved = append(moved, route)
	}
	s.mu.Unlock()

	action := "announce"
	if !announce {
		action = "withdraw"
	}
	var refused []error
	for _, route := range moved {
		cmd := route.announce
		if !announce {
			cmd = route.withdrawal
		}
		if err := s.send(route.selector, cmd); err != nil {
			s.mu.Lock()
			route.announced = !announce
			s.mu.Unlock()
			refused = append(refused, givenSendError(name, route.selector, action, err))
		}
	}
	return held, errors.Join(refused...)
}

// givenHolds reports whether the watchdog name holds a given route.
func (s *watchdogServer) givenHolds(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.given[name]) != 0
}

// handleForget takes every route owner gave a watchdog off it, and withdraws
// those it holds announced, as ExaBGP's _forget_watchdog_owner and
// withdraw_owner do for a helper that exits (rib/outgoing.py at f9bfb853b): a
// later `request bgp watchdog announce` does not bring them back. The
// withdrawal restores a configured route of the same prefix, as an exit's
// withdrawal does (`restore-configured`).
func (s *watchdogServer) handleForget(args []string) (string, any, error) {
	if len(args) != 1 {
		return statusError, nil, errors.New("request bgp watchdog forget needs the one owner whose routes it forgets; " +
			"write request bgp watchdog forget <owner>")
	}
	owner := args[0]

	s.mu.Lock()
	var withdraw []*givenRoute
	forgotten := 0
	for name, routes := range s.given {
		for key, route := range routes {
			if route.owner != owner {
				continue
			}
			if route.announced {
				withdraw = append(withdraw, route)
			}
			delete(routes, key)
			forgotten++
		}
		if len(routes) == 0 {
			delete(s.given, name)
		}
	}
	s.givenCount -= forgotten
	s.mu.Unlock()

	slices.SortFunc(withdraw, func(a, b *givenRoute) int { return strings.Compare(givenKey(a), givenKey(b)) })
	var refused []error
	for _, route := range withdraw {
		if err := s.send(route.selector, restoringWithdrawal(route.withdrawal)); err != nil {
			refused = append(refused, fmt.Errorf("watchdog could not withdraw a route %s gave for %s, so the peer still has it: %w",
				owner, route.selector, err))
		}
	}
	logger().Debug("watchdog owner forgotten", "owner", owner, "routes", forgotten, "withdrawn", len(withdraw)-len(refused))
	if len(refused) != 0 {
		return statusError, nil, errors.Join(refused...)
	}
	return statusDone, watchdogForgetResult{Owner: owner, Forgotten: forgotten, Withdrawn: len(withdraw)}, nil
}

// watchdogForgetResult is the answer to `request bgp watchdog forget`.
type watchdogForgetResult struct {
	Owner     string `json:"owner"`
	Forgotten int    `json:"forgotten"`
	Withdrawn int    `json:"withdrawn"`
}

// restoringWithdrawal is a withdrawal in update text that gives back a
// configured route of the same prefix (`restore-configured`), as the exit of
// the process that announced it asks. A withdrawal in another grammar names
// no configured route and is sent as written.
func restoringWithdrawal(withdrawal string) string {
	tail, cut := strings.CutPrefix(withdrawal, "update text ")
	if !cut {
		return withdrawal
	}
	return "update text restore-configured " + tail
}
