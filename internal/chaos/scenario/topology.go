// Design: docs/architecture/chaos-web-dashboard.md — simulated network topology
// Detail: topology_generate.go — a topology generated from a seed
// Related: generator.go — the peer profiles a chaos run is built from
// Related: profile.go — the identity each speaker of a topology fills

package scenario

import (
	"cmp"
	"errors"
	"fmt"
	"net/netip"
	"slices"
)

// Limits of a topology. A hand-written or generated network past these is
// refused, so every loop below is bounded by a number stated here.
const (
	topologyRoutersMax    = 32
	topologyNeighboursMax = 128
	// topologyRoutesMax bounds the prefixes one speaker originates or announces.
	topologyRoutesMax = 256
)

// SessionKind says which kind of BGP session a speaker holds with the collector.
type SessionKind uint8

const (
	// SessionKindUnspecified is the zero value and never a valid session.
	SessionKindUnspecified SessionKind = iota
	// SessionKindInternal is an iBGP session: the speaker is a router of the AS.
	SessionKindInternal
	// SessionKindExternal is an eBGP session: the speaker is an external neighbour.
	SessionKindExternal
)

// NextHopPolicy says what a border router writes as next hop on a route it
// learned from an external neighbour, toward the other routers and the collector.
type NextHopPolicy uint8

const (
	// NextHopPolicyUnspecified is the zero value. NewTopology refuses it.
	NextHopPolicyUnspecified NextHopPolicy = iota
	// NextHopPolicySelf rewrites the next hop to the border router's own loopback.
	NextHopPolicySelf
	// NextHopPolicyUnchanged keeps the external neighbour's address.
	NextHopPolicyUnchanged
)

// TraceEnd says how a trace leaves the routers of the AS.
type TraceEnd uint8

const (
	// TraceEndUnspecified is the zero value and never the end of a trace.
	TraceEndUnspecified TraceEnd = iota
	// TraceEndExitOwnAddress: the last router announces its own address as next
	// hop on a route learned outside. It is the exit.
	TraceEndExitOwnAddress
	// TraceEndExitExternalAddress: the last router's next hop is the address of
	// no router of the AS, and its route has an AS path. The trace leaves the
	// AS there, toward that address.
	TraceEndExitExternalAddress
	// TraceEndOriginInside: the last router's route has an empty AS path. The
	// prefix originates inside the AS, at that router, whatever next hop the
	// route carries: an empty AS path is asked before the next hop (traceEnd).
	TraceEndOriginInside
)

// ChoiceReason says why a router chose its route over the next best candidate.
type ChoiceReason uint8

const (
	// ChoiceReasonUnspecified is the zero value and never a stated reason.
	ChoiceReasonUnspecified ChoiceReason = iota
	// ChoiceReasonOnlyRoute: the router holds one route for the prefix.
	ChoiceReasonOnlyRoute
	// ChoiceReasonShorterASPath: the chosen route has fewer AS numbers in its path.
	ChoiceReasonShorterASPath
	// ChoiceReasonOriginatedHere: the router originates the prefix itself.
	ChoiceReasonOriginatedHere
	// ChoiceReasonExternalOverInternal: equal path lengths, and the chosen route was
	// learned from an external neighbour while the other came from inside.
	ChoiceReasonExternalOverInternal
	// ChoiceReasonLowerPeerAddress: the two best routes rank equal, and the chosen one
	// was learned from the speaker with the lower address.
	ChoiceReasonLowerPeerAddress
)

// TopologySpec describes a network: the routers of one AS and the external
// neighbours attached to them. It is the input of NewTopology, written by hand
// or produced by GenerateTopology.
//
// The iBGP sessions follow from the routers: a client holds one session, with
// its reflector, and every router that is no client holds a session with every
// other such router.
type TopologySpec struct {
	// LocalAS is the AS of the routers and of the collector.
	LocalAS    uint32
	Routers    []TopologyRouter
	Neighbours []TopologyNeighbour
}

// TopologyRouter is one router of the AS.
type TopologyRouter struct {
	// Loopback is the address the router writes as next hop. It is also its
	// session address with the collector and its BGP identifier.
	Loopback netip.Addr
	// Clients holds the loopbacks of the routers this router reflects routes
	// for. A router with clients is a route reflector, and it rewrites the next
	// hop to its own loopback on every route it reflects.
	Clients []netip.Addr
	// Originates holds the prefixes this router originates inside the AS.
	Originates []netip.Prefix
}

// TopologyNeighbour is one external neighbour, on an eBGP session with one
// border router and on an eBGP session of its own with the collector.
type TopologyNeighbour struct {
	// Address is the neighbour's address on the link to its border router. It
	// is also its session address with the collector and its BGP identifier.
	Address netip.Addr
	AS      uint32
	// Border is the loopback of the router the neighbour is attached to.
	Border netip.Addr
	// BorderNextHop is what Border writes on the routes this neighbour sends.
	BorderNextHop NextHopPolicy
	Announces     []NeighbourRoute
}

// NeighbourRoute is one prefix an external neighbour announces.
type NeighbourRoute struct {
	Prefix netip.Prefix
	// ASPathBeyond holds the AS numbers after the neighbour's own, toward the
	// origin. It is empty when the neighbour originates the prefix. The path
	// on the wire is the neighbour's AS followed by these.
	ASPathBeyond []uint32
}

// Announcement is one route a speaker sends to the collector.
type Announcement struct {
	Prefix  netip.Prefix
	NextHop netip.Addr
	// ASPath is the whole AS path on the wire, the neighbour's own AS first on
	// an external session. It is empty for a prefix that originates inside the
	// AS. A sender MUST NOT prepend anything to it.
	ASPath []uint32
}

// Speaker is one simulated BGP speaker and everything it sends to the collector.
type Speaker struct {
	// Address is the speaker's session address with the collector and its BGP
	// identifier: a router's loopback, or a neighbour's link address.
	Address netip.Addr
	// AS is the speaker's own AS.
	AS      uint32
	Session SessionKind
	// Announcements is ordered by prefix. Its length is the route count the
	// collector holds from this speaker.
	Announcements []Announcement
}

// Trace is the expected walk of one prefix from one router: the routers of the
// AS its BGP next hops cross, then the AS path outside.
type Trace struct {
	// Routers holds the loopbacks crossed, the starting router first and the
	// router the trace leaves the AS at last.
	Routers []netip.Addr
	End     TraceEnd
	// NextHop is the next hop the last router announces: its own loopback when
	// End is TraceEndExitOwnAddress, the external address when End is
	// TraceEndExitExternalAddress. When End is TraceEndOriginInside it is the
	// next hop of the originated route, which is the router's own loopback in
	// every network NewTopology builds.
	NextHop netip.Addr
	// ASPath is the AS path of the last router's route.
	ASPath []uint32
	// Reason is why the starting router chose this route.
	Reason ChoiceReason
}

// Topology is a computed network: what every speaker announces to the
// collector, and the answers a looking glass over those sessions owes.
//
// Only NewTopology and GenerateTopology return one. It is immutable, so it is
// safe for concurrent use, and a caller MUST NOT modify a slice it returns.
type Topology struct {
	localAS    uint32
	speakers   []Speaker
	prefixes   []netip.Prefix
	announcers map[netip.Prefix][]netip.Addr
	traces     map[traceKey]Trace
}

type traceKey struct {
	router netip.Addr
	prefix netip.Prefix
}

// LocalAS returns the AS of the routers and of the collector.
func (t *Topology) LocalAS() uint32 { return t.localAS }

// Speakers returns every speaker: the routers in the order of the spec, then
// the external neighbours in the order of the spec.
func (t *Topology) Speakers() []Speaker { return t.speakers }

// Prefixes returns every prefix of the network, ordered by address then length.
func (t *Topology) Prefixes() []netip.Prefix { return t.prefixes }

// Announcers returns the addresses of the external neighbours announcing
// prefix, in address order. The list is empty for a prefix that only
// originates inside the AS, and a prefix the network does not hold is an error.
func (t *Topology) Announcers(prefix netip.Prefix) ([]netip.Addr, error) {
	announcers, known := t.announcers[prefix]
	if !known {
		return nil, fmt.Errorf("topology: no speaker announces prefix %s: ask for one of Prefixes()", prefix)
	}
	return announcers, nil
}

// Trace returns the expected walk of prefix starting at the router whose
// loopback is router. An address that is no router's loopback, or a prefix the
// network does not hold, is an error.
func (t *Topology) Trace(router netip.Addr, prefix netip.Prefix) (Trace, error) {
	trace, known := t.traces[traceKey{router: router, prefix: prefix}]
	if !known {
		return Trace{}, fmt.Errorf(
			"topology: no trace from %s for prefix %s: the address must be a router's loopback and the prefix one of Prefixes()",
			router, prefix)
	}
	return trace, nil
}

// crossesIntermediateRouter reports whether some trace crosses a router
// between its starting router and the router it leaves the AS at.
func (t *Topology) crossesIntermediateRouter() bool {
	for _, trace := range t.traces {
		if len(trace.Routers) >= 3 {
			return true
		}
	}
	return false
}

// NewTopology validates spec, runs the model's own best-path computation for
// every router and prefix, and returns what each speaker announces and the
// trace of each prefix from each router.
//
// The computation is written here on purpose. It shares no code with the RIB
// or the looking glass, so a test that compares them cannot agree with itself.
func NewTopology(spec TopologySpec) (*Topology, error) {
	model, err := newTopologyModel(spec)
	if err != nil {
		return nil, err
	}
	learned := learnedRoutes{
		candidates: map[netip.Prefix][][]modelRoute{},
		announcers: map[netip.Prefix][]netip.Addr{},
	}
	if err := learned.addOriginated(model, spec); err != nil {
		return nil, err
	}
	neighbours, err := learned.addNeighbours(model, spec)
	if err != nil {
		return nil, err
	}

	topology := &Topology{
		localAS:    spec.LocalAS,
		announcers: learned.announcers,
		traces:     map[traceKey]Trace{},
	}
	for prefix := range learned.candidates {
		topology.prefixes = append(topology.prefixes, prefix)
	}
	slices.SortFunc(topology.prefixes, comparePrefixes)
	for _, addresses := range topology.announcers {
		slices.SortFunc(addresses, netip.Addr.Compare)
	}

	routers := make([]Speaker, len(model.loopbacks))
	for router, loopback := range model.loopbacks {
		routers[router] = Speaker{Address: loopback, AS: spec.LocalAS, Session: SessionKindInternal}
	}
	for _, prefix := range topology.prefixes {
		chosen, err := model.converge(prefix, learned.candidates[prefix])
		if err != nil {
			return nil, err
		}
		for router, loopback := range model.loopbacks {
			route := chosen[router].route
			routers[router].Announcements = append(routers[router].Announcements,
				Announcement{Prefix: prefix, NextHop: route.nextHop, ASPath: route.asPath})
			trace, err := model.trace(router, prefix, chosen)
			if err != nil {
				return nil, err
			}
			topology.traces[traceKey{router: loopback, prefix: prefix}] = trace
		}
	}
	topology.speakers = slices.Concat(routers, neighbours)
	return topology, nil
}

// routeSource says how a router came to hold a route. The order of the values
// is the order of preference between routes of equal AS path length.
type routeSource uint8

const (
	sourceUnspecified routeSource = iota
	sourceOriginated
	sourceExternal
	sourceInternal
)

// modelRoute is one route a router holds for one prefix.
type modelRoute struct {
	asPath []uint32
	// nextHop is the next hop the router writes when it announces the route
	// as its own choice: its loopback on a route it originates, its loopback
	// or the neighbour's address on a route learned outside, by the border's
	// policy, and the next hop it received on a route learned inside.
	nextHop netip.Addr
	source  routeSource
	// from is the address of the speaker the route was learned from, and the
	// router's own loopback on a route it originates.
	from netip.Addr
}

// selection is a router's chosen route for one prefix. found is the guard:
// route and reason mean nothing while it is false.
type selection struct {
	route  modelRoute
	reason ChoiceReason
	found  bool
}

// reflectorNone marks a router that is the client of no reflector.
const reflectorNone = -1

// topologyModel holds the routers of a validated spec and their iBGP sessions.
type topologyModel struct {
	loopbacks []netip.Addr
	routerOf  map[netip.Addr]int
	// reflectorOf holds, for each router, the index of the reflector it is a
	// client of, or reflectorNone.
	reflectorOf []int
}

func newTopologyModel(spec TopologySpec) (*topologyModel, error) {
	if spec.LocalAS == 0 {
		return nil, errors.New("topology: the local AS is 0: set LocalAS to the AS of the routers")
	}
	if len(spec.Routers) == 0 {
		return nil, errors.New("topology: no router: add at least one router to Routers")
	}
	if len(spec.Routers) > topologyRoutersMax {
		return nil, fmt.Errorf("topology: %d routers, and the model holds at most %d: remove routers",
			len(spec.Routers), topologyRoutersMax)
	}
	model := &topologyModel{routerOf: map[netip.Addr]int{}}
	for index, router := range spec.Routers {
		if !router.Loopback.Is4() {
			return nil, fmt.Errorf("topology: router %d has loopback %q, which is no IPv4 address: set Loopback to one",
				index, router.Loopback)
		}
		if other, taken := model.routerOf[router.Loopback]; taken {
			return nil, fmt.Errorf("topology: routers %d and %d share loopback %s: give each router its own",
				other, index, router.Loopback)
		}
		model.routerOf[router.Loopback] = index
		model.loopbacks = append(model.loopbacks, router.Loopback)
		model.reflectorOf = append(model.reflectorOf, reflectorNone)
	}
	for reflector, router := range spec.Routers {
		for _, client := range router.Clients {
			if err := model.addClient(reflector, client); err != nil {
				return nil, err
			}
		}
	}
	for index, router := range spec.Routers {
		if len(router.Clients) == 0 {
			continue
		}
		if model.reflectorOf[index] != reflectorNone {
			return nil, fmt.Errorf(
				"topology: reflector %s is itself a client of %s, and the model holds one level of reflection: remove it from that client list",
				router.Loopback, model.loopbacks[model.reflectorOf[index]])
		}
	}
	return model, nil
}

func (m *topologyModel) addClient(reflector int, client netip.Addr) error {
	index, known := m.routerOf[client]
	if !known {
		return fmt.Errorf("topology: reflector %s lists client %s, which is no router's loopback: list a router of Routers",
			m.loopbacks[reflector], client)
	}
	if index == reflector {
		return fmt.Errorf("topology: reflector %s lists itself as a client: remove it from its own list", client)
	}
	if m.reflectorOf[index] != reflectorNone {
		return fmt.Errorf("topology: router %s is listed as a client of %s and of %s: keep it in one client list",
			client, m.loopbacks[m.reflectorOf[index]], m.loopbacks[reflector])
	}
	m.reflectorOf[index] = reflector
	return nil
}

// learnedRoutes holds what the routers learn before any iBGP exchange.
type learnedRoutes struct {
	// candidates holds, per prefix and per router, the routes the router
	// originates or learns from its external neighbours.
	candidates map[netip.Prefix][][]modelRoute
	// announcers holds, per prefix, the external neighbours announcing it. A
	// prefix that only originates inside the AS has a key and no address.
	announcers map[netip.Prefix][]netip.Addr
}

func (l *learnedRoutes) add(routers int, prefix netip.Prefix, router int, route modelRoute) {
	if l.candidates[prefix] == nil {
		l.candidates[prefix] = make([][]modelRoute, routers)
		l.announcers[prefix] = nil
	}
	l.candidates[prefix][router] = append(l.candidates[prefix][router], route)
}

func (l *learnedRoutes) addOriginated(model *topologyModel, spec TopologySpec) error {
	for index, router := range spec.Routers {
		if err := validateTopologyPrefixes(router.Loopback, router.Originates); err != nil {
			return err
		}
		for _, prefix := range router.Originates {
			l.add(len(model.loopbacks), prefix, index, modelRoute{
				nextHop: router.Loopback,
				source:  sourceOriginated,
				from:    router.Loopback,
			})
		}
	}
	return nil
}

// addNeighbours validates the external neighbours, hands each border router
// the routes its neighbours send, and returns the neighbours as speakers.
func (l *learnedRoutes) addNeighbours(model *topologyModel, spec TopologySpec) ([]Speaker, error) {
	if len(spec.Neighbours) > topologyNeighboursMax {
		return nil, fmt.Errorf("topology: %d external neighbours, and the model holds at most %d: remove neighbours",
			len(spec.Neighbours), topologyNeighboursMax)
	}
	speakers := make([]Speaker, 0, len(spec.Neighbours))
	addresses := map[netip.Addr]bool{}
	for _, neighbour := range spec.Neighbours {
		border, err := model.validateNeighbour(spec.LocalAS, neighbour)
		if err != nil {
			return nil, err
		}
		if addresses[neighbour.Address] {
			return nil, fmt.Errorf("topology: two external neighbours share address %s: give each its own", neighbour.Address)
		}
		addresses[neighbour.Address] = true

		nextHop := neighbour.Address
		if neighbour.BorderNextHop == NextHopPolicySelf {
			nextHop = neighbour.Border
		}
		speaker := Speaker{Address: neighbour.Address, AS: neighbour.AS, Session: SessionKindExternal}
		for _, route := range neighbour.Announces {
			asPath := append([]uint32{neighbour.AS}, route.ASPathBeyond...)
			l.add(len(model.loopbacks), route.Prefix, border, modelRoute{
				asPath:  asPath,
				nextHop: nextHop,
				source:  sourceExternal,
				from:    neighbour.Address,
			})
			l.announcers[route.Prefix] = append(l.announcers[route.Prefix], neighbour.Address)
			// The neighbour announces on its own session with its own address:
			// the rewrite is the border router's, never the neighbour's.
			speaker.Announcements = append(speaker.Announcements,
				Announcement{Prefix: route.Prefix, NextHop: neighbour.Address, ASPath: asPath})
		}
		slices.SortFunc(speaker.Announcements, func(a, b Announcement) int {
			return comparePrefixes(a.Prefix, b.Prefix)
		})
		speakers = append(speakers, speaker)
	}
	return speakers, nil
}

// validateNeighbour checks one external neighbour and returns the index of
// its border router.
func (m *topologyModel) validateNeighbour(localAS uint32, neighbour TopologyNeighbour) (int, error) {
	if !neighbour.Address.Is4() {
		return 0, fmt.Errorf("topology: an external neighbour has address %q, which is no IPv4 address: set Address to one",
			neighbour.Address)
	}
	if _, isRouter := m.routerOf[neighbour.Address]; isRouter {
		return 0, fmt.Errorf("topology: external neighbour %s has the address of a router of the AS: give it its own",
			neighbour.Address)
	}
	if neighbour.AS == 0 {
		return 0, fmt.Errorf("topology: external neighbour %s has AS 0: set AS to its AS number", neighbour.Address)
	}
	if neighbour.AS == localAS {
		return 0, fmt.Errorf("topology: external neighbour %s has the local AS %d: an external neighbour is in another AS",
			neighbour.Address, localAS)
	}
	border, known := m.routerOf[neighbour.Border]
	if !known {
		return 0, fmt.Errorf("topology: external neighbour %s is attached to %q, which is no router's loopback: set Border to one",
			neighbour.Address, neighbour.Border)
	}
	switch neighbour.BorderNextHop {
	case NextHopPolicySelf, NextHopPolicyUnchanged:
	case NextHopPolicyUnspecified:
		return 0, fmt.Errorf(
			"topology: external neighbour %s does not say what its border router writes as next hop: set BorderNextHop to NextHopPolicySelf or NextHopPolicyUnchanged",
			neighbour.Address)
	default:
		// The spec is caller input, so a value outside the named set is refused.
		return 0, fmt.Errorf("topology: external neighbour %s has BorderNextHop %d, which is no known policy: use NextHopPolicySelf or NextHopPolicyUnchanged",
			neighbour.Address, neighbour.BorderNextHop)
	}
	prefixes := make([]netip.Prefix, 0, len(neighbour.Announces))
	for _, route := range neighbour.Announces {
		prefixes = append(prefixes, route.Prefix)
		for _, as := range route.ASPathBeyond {
			if as == 0 {
				return 0, fmt.Errorf("topology: external neighbour %s announces %s with AS 0 in its path: remove it",
					neighbour.Address, route.Prefix)
			}
			if as == localAS {
				return 0, fmt.Errorf(
					"topology: external neighbour %s announces %s with the local AS %d in its path, which a border router refuses as a loop: remove it",
					neighbour.Address, route.Prefix, localAS)
			}
		}
	}
	if err := validateTopologyPrefixes(neighbour.Address, prefixes); err != nil {
		return 0, err
	}
	return border, nil
}

// validateTopologyPrefixes checks the prefixes one speaker originates or announces.
func validateTopologyPrefixes(speaker netip.Addr, prefixes []netip.Prefix) error {
	if len(prefixes) > topologyRoutesMax {
		return fmt.Errorf("topology: speaker %s has %d prefixes, and the model holds at most %d per speaker: remove prefixes",
			speaker, len(prefixes), topologyRoutesMax)
	}
	seen := map[netip.Prefix]bool{}
	for _, prefix := range prefixes {
		if !prefix.Addr().Is4() {
			return fmt.Errorf("topology: speaker %s has prefix %q, which is no IPv4 prefix: the model covers IPv4 unicast",
				speaker, prefix)
		}
		if prefix != prefix.Masked() {
			return fmt.Errorf("topology: speaker %s has prefix %s with host bits set: write it as %s",
				speaker, prefix, prefix.Masked())
		}
		if seen[prefix] {
			return fmt.Errorf("topology: speaker %s has prefix %s twice: keep one", speaker, prefix)
		}
		seen[prefix] = true
	}
	return nil
}

func comparePrefixes(a, b netip.Prefix) int {
	return cmp.Or(a.Addr().Compare(b.Addr()), cmp.Compare(a.Bits(), b.Bits()))
}

// converge computes every router's chosen route for one prefix. Each round
// lets every router choose among what it learned itself and what its iBGP
// peers announced from the round before, until a round changes nothing.
//
// A route travels at most one session per round, and the longest way through
// one level of reflection is client, reflector, reflector, client. The round
// bound leaves room beyond that, and a network that has not settled by then is
// an error, never a guess.
func (m *topologyModel) converge(prefix netip.Prefix, learned [][]modelRoute) ([]selection, error) {
	current := make([]selection, len(m.loopbacks))
	for range 2*len(m.loopbacks) + 4 {
		next := make([]selection, len(m.loopbacks))
		for router := range next {
			candidates := slices.Clone(learned[router])
			for peer := range current {
				peerRoute, sent := m.announcementTo(peer, router, current[peer])
				if sent {
					candidates = append(candidates, peerRoute)
				}
			}
			next[router] = selectBest(candidates)
		}
		if !slices.EqualFunc(next, current, selectionsEqual) {
			current = next
			continue
		}
		for router, chosen := range current {
			if !chosen.found {
				return nil, fmt.Errorf("topology: router %s learns no route for prefix %s: no iBGP session reaches it",
					m.loopbacks[router], prefix)
			}
		}
		return current, nil
	}
	return nil, fmt.Errorf("topology: the routers do not settle on a route for prefix %s: the model cannot state what they announce",
		prefix)
}

func selectionsEqual(a, b selection) bool {
	if a.found != b.found {
		return false
	}
	if a.reason != b.reason {
		return false
	}
	if a.route.nextHop != b.route.nextHop {
		return false
	}
	if a.route.source != b.route.source {
		return false
	}
	if a.route.from != b.route.from {
		return false
	}
	return slices.Equal(a.route.asPath, b.route.asPath)
}

// peered reports whether two routers hold an iBGP session: a client with its
// reflector, and any two routers that are clients of no reflector.
func (m *topologyModel) peered(a, b int) bool {
	if m.reflectorOf[a] == b {
		return true
	}
	if m.reflectorOf[b] == a {
		return true
	}
	if m.reflectorOf[a] != reflectorNone {
		return false
	}
	return m.reflectorOf[b] == reflectorNone
}

// announcementTo returns the route router `from` sends to its iBGP peer `to`
// when chosen is its choice, and false when it sends none.
func (m *topologyModel) announcementTo(from, to int, chosen selection) (modelRoute, bool) {
	if from == to {
		return modelRoute{}, false
	}
	if !chosen.found {
		return modelRoute{}, false
	}
	if !m.peered(from, to) {
		return modelRoute{}, false
	}
	nextHop := chosen.route.nextHop
	switch chosen.route.source {
	case sourceOriginated, sourceExternal:
		// A route the router did not learn from iBGP goes to every iBGP peer.
	case sourceInternal:
		if !m.reflects(from, to, chosen.route) {
			return modelRoute{}, false
		}
		// RFC 4456 Section 10: "when a RR reflects a route, it SHOULD NOT modify
		// the following path attributes: NEXT_HOP, AS_PATH, LOCAL_PREF, and MED."
		// The simulated reflector does rewrite NEXT_HOP, to its own loopback.
		// That is the configuration the network under test is defined to run,
		// so a client's trace crosses its reflector. AS_PATH stays as received.
		nextHop = m.loopbacks[from]
	case sourceUnspecified:
		panic("BUG: a chosen route has no source")
	}
	return modelRoute{
		asPath:  chosen.route.asPath,
		nextHop: nextHop,
		source:  sourceInternal,
		from:    m.loopbacks[from],
	}, true
}

// reflects reports whether router `from` passes a route it learned from one
// iBGP peer on to its iBGP peer `to`. Only a reflector does.
//
// RFC 4456 Section 6: "A route from a Non-Client IBGP peer: Reflect to all the
// Clients." and "A route from a Client peer: Reflect to all the Non-Client
// peers and also to the Client peers."
func (m *topologyModel) reflects(from, to int, route modelRoute) bool {
	source := m.routerOf[route.from]
	if source == to {
		return false
	}
	if m.reflectorOf[source] == from {
		return true
	}
	return m.reflectorOf[to] == from
}

// selectBest returns the best of candidates and why it beat the runner-up. It
// reorders candidates. No candidate gives a selection with found false.
func selectBest(candidates []modelRoute) selection {
	if len(candidates) == 0 {
		return selection{}
	}
	slices.SortFunc(candidates, compareRoutes)
	reason := ChoiceReasonOnlyRoute
	if len(candidates) > 1 {
		reason = choiceReason(candidates[0], candidates[1])
	}
	return selection{route: candidates[0], reason: reason, found: true}
}

// compareRoutes orders routes best first. The model has no policy and no IGP,
// so local preference, origin, MED and interior cost (steps b, c and e) are
// equal for every route, and three steps remain.
//
// RFC 4271 Section 9.1.2.2: "a) Remove from consideration all routes that are
// not tied for having the smallest number of AS numbers present in their
// AS_PATH attributes."
//
// RFC 4271 Section 9.1.2.2: "d) If at least one of the candidate routes was
// received via EBGP, remove from consideration all routes that were received
// via IBGP."
//
// RFC 4271 Section 9.1.2.2: "g) Prefer the route received from the lowest peer
// address." A speaker's address is also its BGP identifier here, so step f
// gives the same answer.
func compareRoutes(a, b modelRoute) int {
	return cmp.Or(
		cmp.Compare(len(a.asPath), len(b.asPath)),
		cmp.Compare(a.source, b.source),
		a.from.Compare(b.from),
	)
}

// choiceReason names the first step of compareRoutes that put best ahead of
// second. best MUST sort before second.
func choiceReason(best, second modelRoute) ChoiceReason {
	if len(best.asPath) < len(second.asPath) {
		return ChoiceReasonShorterASPath
	}
	if best.source == second.source {
		return ChoiceReasonLowerPeerAddress
	}
	switch best.source {
	case sourceOriginated:
		return ChoiceReasonOriginatedHere
	case sourceExternal:
		return ChoiceReasonExternalOverInternal
	case sourceInternal, sourceUnspecified:
		// Neither sorts ahead of another source: the BUG below.
	}
	panic("BUG: the best route does not rank ahead of the runner-up")
}

// traceEnd names how a trace ends at the router whose route is route. exit is
// the end the next hop alone gives: the router's own address, or an address
// outside the AS.
//
// An empty AS path decides first, whatever the next hop is: the prefix then
// originates inside the AS at this router, and there is no exit to name. The
// looking glass draws its trace by the same rule, so a route with an empty AS
// path and a next hop outside the AS ends origin-inside on both sides.
func traceEnd(route modelRoute, exit TraceEnd) TraceEnd {
	if len(route.asPath) == 0 {
		return TraceEndOriginInside
	}
	return exit
}

// trace follows the chosen routes of one prefix from router start: each
// router's next hop names the next router, until it names the router
// itself or no router of the AS. The visited guard bounds the walk by the
// number of routers.
func (m *topologyModel) trace(start int, prefix netip.Prefix, chosen []selection) (Trace, error) {
	visited := make([]bool, len(m.loopbacks))
	crossed := make([]netip.Addr, 0, len(m.loopbacks))
	router := start
	for range m.loopbacks {
		visited[router] = true
		crossed = append(crossed, m.loopbacks[router])
		route := chosen[router].route
		trace := Trace{Routers: crossed, NextHop: route.nextHop, ASPath: route.asPath, Reason: chosen[start].reason}

		next, isRouter := m.routerOf[route.nextHop]
		if !isRouter {
			trace.End = traceEnd(route, TraceEndExitExternalAddress)
			return trace, nil
		}
		if next != router {
			if visited[next] {
				return Trace{}, fmt.Errorf("topology: the trace of prefix %s from %s returns to %s: the model cannot state where it leaves the AS",
					prefix, m.loopbacks[start], m.loopbacks[next])
			}
			router = next
			continue
		}
		trace.End = traceEnd(route, TraceEndExitOwnAddress)
		return trace, nil
	}
	return Trace{}, fmt.Errorf("topology: the trace of prefix %s from %s does not end within %d routers",
		prefix, m.loopbacks[start], len(m.loopbacks))
}
