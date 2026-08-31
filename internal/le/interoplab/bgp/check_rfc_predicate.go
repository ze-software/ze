// Design: docs/architecture/testing/interop.md -- pure decisions over foreign daemon output.
// Related: check_rfc.go -- the tagged checker bodies that do the lab I/O and call these.
// Overview: every function here takes text or decoded JSON and holds no lab handle, so
// TestBespokeCheckerBranches exercises both polarities of each one with no container.
package bgp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// addPathReceiveNegotiated reports whether FRR's own neighbor JSON says the
// ADD-PATH receive direction was advertised by FRR and received from ze. The needle
// keeps its opening quote so the send direction, which FRR spells
// "txAdvertisedAndReceived", can never satisfy it: ze sending Path Identifiers that
// FRR never agreed to receive is one of the states this predicate exists to reject.
// FRR pretty-prints the document, so the spaces come out before the match. The
// rendered token is matched rather than a decoded field because the nesting under
// neighborCapabilities moves between FRR releases while the token does not, and the
// deleted Python checker matched this same token against FRR 10.3.1.
func addPathReceiveNegotiated(neighborJSON string) bool {
	const negotiated = `"rxAdvertisedAndReceived":true`
	return strings.Contains(strings.ReplaceAll(neighborJSON, " ", ""), negotiated)
}

type addPathRoute struct {
	ASPath struct {
		String string `json:"string"`
	} `json:"aspath"`
	AddPathRxID uint64 `json:"addpathRxId"`
}

type addPathDocument struct {
	Routes map[string][]addPathRoute `json:"routes"`
}

func parseAddPathState(output string) (map[string]uint64, error) {
	var document addPathDocument
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, fmt.Errorf("decode FRR ADD-PATH JSON: %w", err)
	}
	paths := document.Routes[peerPrefixFirst]
	if len(paths) < 2 {
		return nil, fmt.Errorf("FRR holds %d paths for %s, want 2", len(paths), peerPrefixFirst)
	}
	state := make(map[string]uint64, len(paths))
	for _, path := range paths {
		origin := strings.TrimSpace(path.ASPath.String)
		if origin != "65003" && origin != "65004" {
			continue
		}
		state[origin] = path.AddPathRxID
	}
	if len(state) != 2 {
		return nil, fmt.Errorf("FRR paths have origins %v, want 65003 and 65004", state)
	}
	if state["65003"] == state["65004"] {
		return nil, fmt.Errorf("FRR received both paths under Path Identifier %d", state["65003"])
	}
	return state, nil
}

func samePathIdentifiers(left, right map[string]uint64) bool {
	return len(left) == len(right) && left["65003"] == right["65003"] && left["65004"] == right["65004"]
}

func parseOTCValue(output string) (uint64, error) {
	index := strings.Index(strings.ToUpper(output), "OTC")
	if index < 0 {
		return 0, errors.New("FRR reported no OTC Attribute")
	}
	tail := output[index+len("OTC"):]
	for tail != "" {
		switch tail[0] {
		case ' ', '\t', ':', '=', '"':
			tail = tail[1:]
		default:
			goto digits
		}
	}
digits:
	end := 0
	for end < len(tail) && tail[end] >= '0' && tail[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, errors.New("FRR OTC Attribute has no numeric value")
	}
	value, err := strconv.ParseUint(tail[:end], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse FRR OTC value: %w", err)
	}
	return value, nil
}

type nextHop struct {
	Scope string `json:"scope"`
	IP    string `json:"ip"`
}

// frrPath is one path of FRR's `show bgp <family> <prefix> json`, cut to the two
// fields the restored checkers read: the AS_PATH FRR attributes to the route, and
// the next-hop addresses it decoded.
type frrPath struct {
	ASPath struct {
		String string `json:"string"`
	} `json:"aspath"`
	NextHops []nextHop `json:"nexthops"`
}

type frrPrefixDocument struct {
	Paths []frrPath `json:"paths"`
}

// frrSinglePath decodes FRR's answer about one prefix and returns the one path it
// holds. Two paths are an error rather than a choice: every caller asserts about
// the route ze advertised over the single session the scenario runs, so picking
// one of two would be a guess. An unparsed document is an error rather than an
// empty path list, because a query that answered nothing is not a table with no
// route in it.
func frrSinglePath(output string) (frrPath, error) {
	var document frrPrefixDocument
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return frrPath{}, fmt.Errorf("decode FRR route JSON: %w", err)
	}
	if len(document.Paths) != 1 {
		return frrPath{}, fmt.Errorf("FRR route has %d paths, want 1", len(document.Paths))
	}
	return document.Paths[0], nil
}

func parseFRRNextHops(output string) ([]nextHop, error) {
	path, err := frrSinglePath(output)
	if err != nil {
		return nil, err
	}
	if len(path.NextHops) == 0 {
		return nil, errors.New("FRR route carries no next-hop entries")
	}
	return path.NextHops, nil
}

// requireFRRASPath reports whether FRR attributes exactly the wanted AS_PATH to
// the route it decoded. The whole path is compared rather than searched for a
// member AS, so a path that carries the wanted one BESIDE another AS fails: the
// requirement is what the receiver decoded, and an AS the relay added or dropped
// changes that answer. FRR's own spelling is normalized on whitespace alone, so
// the comparison is against the AS numbers in their order.
func requireFRRASPath(routeJSON, want string) error {
	path, err := frrSinglePath(routeJSON)
	if err != nil {
		return err
	}
	decoded := strings.Join(strings.Fields(path.ASPath.String), " ")
	if decoded != want {
		return fmt.Errorf("FRR attributes AS_PATH %q to the route, want %q", decoded, want)
	}
	return nil
}

// requireSoleNextHop reports whether FRR attributes exactly one next hop to the
// route, and that it is want. The count is asserted with the address, because a
// route carrying the wanted next hop BESIDE another one is not the route ze
// relayed. The address is parsed rather than compared as text, so one spelling of
// an address cannot pass as a different one.
func requireSoleNextHop(entries []nextHop, want netip.Addr) error {
	if len(entries) != 1 {
		return fmt.Errorf("FRR decoded %d next-hop addresses, want 1", len(entries))
	}
	address, err := netip.ParseAddr(entries[0].IP)
	if err != nil {
		return fmt.Errorf("invalid FRR next hop %q: %w", entries[0].IP, err)
	}
	if address != want {
		return fmt.Errorf("FRR decoded next hop %s, want %s", address, want)
	}
	return nil
}

func requireNextHopShape(entries []nextHop, global, linkLocal netip.Addr) error {
	want := 1
	if linkLocal.IsValid() {
		want = 2
	}
	if len(entries) != want {
		return fmt.Errorf("FRR decoded %d next-hop addresses, want %d", len(entries), want)
	}
	seenGlobal := false
	seenLinkLocal := false
	for _, entry := range entries {
		address, err := netip.ParseAddr(entry.IP)
		if err != nil {
			return fmt.Errorf("invalid FRR next hop %q: %w", entry.IP, err)
		}
		switch entry.Scope {
		case nextHopScopeGlobal:
			seenGlobal = address == global
		case "link-local":
			seenLinkLocal = linkLocal.IsValid() && address == linkLocal
		}
	}
	if !seenGlobal {
		return fmt.Errorf("global next hop %s not decoded", global)
	}
	if seenLinkLocal != linkLocal.IsValid() {
		return fmt.Errorf("link-local next-hop presence=%t, want %t", seenLinkLocal, linkLocal.IsValid())
	}
	return nil
}

// requireRouteInstalledVia reports whether a daemon's route listing installs prefix
// through nextHop. The prefix and the next hop MUST appear on ONE line: FRR writes
// each installed route as `B>* <prefix> [20/0] via <next hop>, eth0`, so a next hop
// found on any other line belongs to a different route, and accepting it would let
// a route installed through its global next hop alone pass as installed through the
// link-local one. An empty listing is a failure rather than an absence, because a
// query that answered nothing is not a table with no route in it.
func requireRouteInstalledVia(routes, prefix, nextHop string) error {
	installed := false
	for line := range strings.SplitSeq(routes, "\n") {
		if !strings.Contains(line, prefix) {
			continue
		}
		installed = true
		if strings.Contains(line, nextHop) {
			return nil
		}
	}
	if !installed {
		return fmt.Errorf("daemon installed no route for %s", prefix)
	}
	return fmt.Errorf("daemon installed %s, but not via next hop %s", prefix, nextHop)
}

// requireNoExternalLSA reports whether FRR's OSPF database holds no AS-external
// LSA, which is what a not-so-stubby area owes: an NSSA carries Type 7 and never
// Type 5. FRR prints the database heading for every `show ip ospf database`
// command, whether or not the area holds the requested LSA type, and prints an
// LS age line only for an LSA it actually holds. So the heading is the positive
// proof that the query ran and the age line is the leak. An unanswered query
// carries neither and fails.
func requireNoExternalLSA(database string) error {
	const (
		heading = "OSPF Router with ID"
		lsaAge  = "LS age"
	)
	return requireAbsentWithProof(database, []string{lsaAge}, []string{heading})
}

// endOfRIBDecoded reports whether FRR's log holds its own decode of the IPv4-unicast
// End-of-RIB marker received from peer. FRR writes each decode as
// `<peer> rcvd End-of-RIB for IPv4 Unicast from <peer>` (bgp_packet.c), and both
// halves are required on ONE line: a "rcvd" phrase on one line and the peer address
// on another belong to two different events, and the marker of a THIRD peer would
// otherwise pass. The family half is matched case-folded because FRR renders it
// through get_afi_safi_str, whose spelling moves between releases, while the peer
// half is matched exactly because an address has one spelling. The "rcvd" verb keeps
// the send direction, which FRR spells "sending End-of-RIB", from passing as a
// receive decode.
func endOfRIBDecoded(log, peer string) bool {
	needle := strings.ToLower("rcvd End-of-RIB for IPv4 Unicast from")
	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(strings.ToLower(line), needle) {
			continue
		}
		if strings.Contains(line, peer) {
			return true
		}
	}
	return false
}

// frrNeighborCapabilities is FRR's `show bgp neighbor <peer> json`, cut to the one
// capability the no-family scenario reads. FRR reports each address family under
// exactly one of the three keys: both directions, its own direction, or the peer's.
type frrNeighborCapabilities struct {
	NeighborCapabilities struct {
		MultiprotocolExtensions struct {
			IPv4Unicast struct {
				Advertised            bool `json:"advertised"`
				Received              bool `json:"received"`
				AdvertisedAndReceived bool `json:"advertisedAndReceived"`
			} `json:"ipv4Unicast"`
		} `json:"multiprotocolExtensions"`
	} `json:"neighborCapabilities"`
}

// requireMultiprotocolAdvertisedOnly reports whether FRR advertised the IPv4-unicast
// Multiprotocol capability itself and received none from peer. Both halves are
// required. The advertised half is the positive evidence that the query ran and
// answered about a live capability block, so an unparsed, empty, or foreign document
// fails rather than reading as "no capability from ze". The received half is the
// assertion: a peer configured with no `family` block MUST advertise no
// Multiprotocol capability, which pins the End-of-RIB fix in capability.Negotiate
// rather than in the OPEN builder and keeps the wire byte-identical for that peer.
func requireMultiprotocolAdvertisedOnly(neighborJSON, peer string) error {
	var document map[string]frrNeighborCapabilities
	if err := json.Unmarshal([]byte(neighborJSON), &document); err != nil {
		return fmt.Errorf("decode FRR neighbor JSON: %w", err)
	}
	neighbor, ok := document[peer]
	if !ok {
		return fmt.Errorf("FRR neighbor JSON holds no entry for %s", peer)
	}
	family := neighbor.NeighborCapabilities.MultiprotocolExtensions.IPv4Unicast
	if !family.Advertised && !family.AdvertisedAndReceived {
		return fmt.Errorf("FRR advertised no IPv4-unicast Multiprotocol capability to %s", peer)
	}
	if family.Received || family.AdvertisedAndReceived {
		return fmt.Errorf("FRR received an IPv4-unicast Multiprotocol capability from %s, which a peer with no family block advertises never", peer)
	}
	return nil
}

// isisAdjacencyUp reports whether FRR holds at least one IS-IS adjacency in the Up
// state. The state is matched as a whole FIELD of the row, never as a substring of
// it, because a neighbor whose rendered name carries those two letters would
// otherwise report an adjacency that is Init or Down as Up.
func isisAdjacencyUp(neighbors string) bool {
	const stateUp = "Up"
	for line := range strings.SplitSeq(neighbors, "\n") {
		if slices.Contains(strings.Fields(line), stateUp) {
			return true
		}
	}
	return false
}

// isisDatabaseNamesZe reports whether FRR's IS-IS database renders an LSP of ze's
// dynamic hostname rather than of a raw system ID. FRR writes each LSP ID as
// `<name>.<pseudonode>-<fragment>` and prints a name there only after decoding
// TLV 137, so the name is required to be the WHOLE identity half of that field: a
// second router called `ze-p2p-2` announces an LSP ID that carries ze's name as a
// prefix, and reading it as ze's would credit ze with another router's dynamic
// hostname.
func isisDatabaseNamesZe(database string) bool {
	// The name isis-p2p-frr/ze.conf configures under `hostname`.
	const hostname = "ze-p2p"
	for line := range strings.SplitSeq(database, "\n") {
		for field := range strings.FieldsSeq(line) {
			identity, _, separated := strings.Cut(field, ".")
			if separated && identity == hostname {
				return true
			}
		}
	}
	return false
}

// FRR's own per-UPDATE decode, as `debug bgp updates in` writes it. The receive
// verb and the withdrawn marker are whole FIELDS of the line, never substrings of
// it: FRR spells the other direction `send`, and a decode of an announcement and a
// decode of a withdrawal differ by this one word.
const (
	frrReceiveVerb     = "rcvd"
	frrWithdrawnMarker = "withdrawn"
)

// frrDecodeFields returns the fields of line when line is FRR's own decode of an
// UPDATE that carried prefix from peer, and nil when it is not. FRR 10.3.x writes
// `<peer> rcvd <prefix> IPv4 unicast` for an announcement and `<peer> rcvd UPDATE
// about <prefix> IPv4 unicast -- withdrawn` for a withdrawal, so all three tokens
// are required on ONE line: a peer address on one line and a prefix on another
// belong to two different events. Each token is matched as a whole field, so
// 110.11.0.0/24 cannot pass as 10.11.0.0/24 and neighbor 172.30.0.22 cannot pass
// as 172.30.0.2.
func frrDecodeFields(line, peer, prefix string) []string {
	fields := strings.Fields(line)
	if !slices.Contains(fields, peer) {
		return nil
	}
	if !slices.Contains(fields, frrReceiveVerb) {
		return nil
	}
	if !slices.Contains(fields, prefix) {
		return nil
	}
	return fields
}

// frrDecodedPrefix reports whether FRR's log holds its own decode of an UPDATE
// that carried prefix from peer, announced or withdrawn. The question it answers
// is whether the prefix reached FRR at all, which no routing table can answer:
// FRR applies RFC 4271 Section 6.3(a) itself, so a route naming FRR's own address
// as NEXT_HOP is absent from its table whether ze withheld it or sent it.
func frrDecodedPrefix(log, peer, prefix string) bool {
	for line := range strings.SplitSeq(log, "\n") {
		if frrDecodeFields(line, peer, prefix) != nil {
			return true
		}
	}
	return false
}

// frrDecodedWithdrawal reports whether FRR's log holds its own decode of an UPDATE
// that WITHDREW prefix from peer. The marker is required on the same line as the
// prefix and the receive verb, so a decode of the announcement of that prefix,
// which the same session carries first, cannot pass as a decode of its withdrawal.
func frrDecodedWithdrawal(log, peer, prefix string) bool {
	for line := range strings.SplitSeq(log, "\n") {
		if slices.Contains(frrDecodeFields(line, peer, prefix), frrWithdrawnMarker) {
			return true
		}
	}
	return false
}

// requireNoAttributeError reports whether FRR accepted the withdrawal of prefix
// from peer with no verdict against the UPDATE's attributes. The decode of that
// withdrawal is required FIRST and is the positive proof that the mechanism ran:
// a log with no such line, an unanswered query among them, holds no attribute
// error either, and reading that as acceptance is how an absence assertion passes
// vacuously. The verdicts are FRR's own words from bgp_attr_parse and
// bgp_update_receive, and the whole log is scanned rather than one line, because
// FRR states the refusal and the route it withdrew on separate lines.
func requireNoAttributeError(log, peer, prefix string) error {
	if !frrDecodedWithdrawal(log, peer, prefix) {
		return fmt.Errorf("FRR logged no decode of a withdrawal for %s from %s, so an absent attribute error proves nothing", prefix, peer)
	}
	verdicts := []string{"Missing well-known attribute", "rcvd UPDATE with errors in attr"}
	for line := range strings.SplitSeq(log, "\n") {
		for _, verdict := range verdicts {
			if strings.Contains(line, verdict) {
				return fmt.Errorf("FRR refused the attributes of an UPDATE ze relayed: %s", strings.TrimSpace(line))
			}
		}
	}
	return nil
}

// The Linux FIB, as `ip -4 route show` prints it inside the ze container. A
// discard route opens its line with the verb and a forwarded one carries the
// gateway keyword, so those two words separate a prefix ze blackholed from one
// it programmed for forwarding. Reading the address alone cannot tell them apart.
const (
	kernelDiscardVerb = "blackhole"
	kernelViaKeyword  = "via"
)

// kernelRoute names what the Linux FIB does with one host destination. The zero
// value is the answer for a destination the table does not name at all, and a
// route in neither shape gets its own value rather than reading as absent: a
// caller asserting "forwarded" must fail on an on-link route instead of being
// told there is no route.
type kernelRoute uint8

const (
	kernelRouteUnspecified kernelRoute = iota
	kernelRouteDiscard
	kernelRouteForwarded
	kernelRouteOther
)

func (route kernelRoute) String() string {
	switch route {
	case kernelRouteUnspecified:
		return "absent from the FIB"
	case kernelRouteDiscard:
		return "a discard route"
	case kernelRouteForwarded:
		return "forwarded through a gateway"
	case kernelRouteOther:
		return "a route in neither the discard nor the forwarded shape"
	}
	return "an unspecified route"
}

// kernelRouteDestination returns the host address one `ip route show` field
// names, and reports whether the field names one. A prefix shorter than a host
// route is rejected rather than reduced to its first address: a covering
// `blackhole 10.100.0.0/24` is not evidence that the announced 10.100.0.1/32 was
// programmed, and reading it as such would credit ze with honoring an
// announcement it never installed. busybox prints a host route without its /32
// and iproute2 with it, so both spellings resolve to the same address.
func kernelRouteDestination(field string) (netip.Addr, bool) {
	if prefix, err := netip.ParsePrefix(field); err == nil {
		if prefix.Bits() != prefix.Addr().BitLen() {
			return netip.Addr{}, false
		}
		return prefix.Addr(), true
	}
	address, err := netip.ParseAddr(field)
	if err != nil {
		return netip.Addr{}, false
	}
	return address, true
}

// kernelRouteFor returns what the Linux FIB does with destination. The verb and
// the destination are read from ONE line, so a discard route for another address
// cannot lend its verb to this one, and a longer address that carries this one as
// a text prefix cannot answer for it either: every field is parsed to an address
// and compared as an address. The first line naming the destination decides,
// because the FIB holds one route per destination and `ip route show` prints one
// line for it.
func kernelRouteFor(table string, destination netip.Addr) kernelRoute {
	for line := range strings.SplitSeq(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		named := fields[0]
		discard := named == kernelDiscardVerb
		if discard {
			if len(fields) < 2 {
				continue
			}
			named = fields[1]
		}
		address, ok := kernelRouteDestination(named)
		if !ok || address != destination {
			continue
		}
		if discard {
			return kernelRouteDiscard
		}
		if slices.Contains(fields, kernelViaKeyword) {
			return kernelRouteForwarded
		}
		return kernelRouteOther
	}
	return kernelRouteUnspecified
}

// BIRD's own decode of one route, as `show route for <prefix> all` prints it. The
// attribute name is the whole first field of its line, so a value that carries the
// same text cannot pass as the attribute.
const birdASPathAttribute = "BGP.as_path:"

// requireBIRDASPath reports whether BIRD attributes exactly want to the route it
// holds for prefix. Three conditions carry the decision. A route line naming
// prefix as a whole FIELD is required first, because `show route for` answers with
// the LONGEST MATCH: without it a covering route's AS_PATH would pass as this
// prefix's, and an unanswered query would pass as anything. Exactly one AS_PATH
// line is required next, because two paths for one prefix leave the assertion
// picking one of them, which is a guess. The whole path is then compared, so an AS
// the relay added ANYWHERE fails: RFC 7947 Section 2.2.2.1 asks a route server not
// to prepend its own AS "nor modify the AS_PATH segment in any other way", and a
// search for the relay's own AS would prove only the first half of that.
func requireBIRDASPath(routeText, prefix, want string) error {
	named := false
	paths := 0
	decoded := ""
	for line := range strings.SplitSeq(routeText, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if slices.Contains(fields, prefix) {
			named = true
		}
		if fields[0] != birdASPathAttribute {
			continue
		}
		paths++
		if paths == 1 {
			decoded = strings.Join(fields[1:], " ")
		}
	}
	if !named {
		return fmt.Errorf("BIRD's answer names no route for %s", prefix)
	}
	if paths != 1 {
		return fmt.Errorf("BIRD holds %d AS_PATH attributes for %s, want 1", paths, prefix)
	}
	if decoded != want {
		return fmt.Errorf("BIRD decoded AS_PATH %q for %s, want %q", decoded, prefix, want)
	}
	return nil
}

// frrTableDocument is FRR's `show bgp ipv4 unicast json`, cut to the one member the
// egress assertions read. The map is keyed by the prefix exactly as FRR renders it,
// so a key is compared whole and 110.10.0.0/24 can never answer for 10.10.0.0/24.
type frrTableDocument struct {
	Routes map[string]json.RawMessage `json:"routes"`
}

// requireRouteWithheld reports whether FRR's table holds control and does NOT hold
// withheld. The control half is the positive proof that the query ran and answered
// about a live table carrying routes ze relayed over this session: a table with
// neither prefix in it holds no withheld route either, and reading that as
// compliance is how an absence assertion passes vacuously. An unparsed answer is an
// error rather than an empty table, because a query that answered nothing is not a
// table with no route in it.
func requireRouteWithheld(tableJSON, withheld, control string) error {
	var document frrTableDocument
	if err := json.Unmarshal([]byte(tableJSON), &document); err != nil {
		return fmt.Errorf("decode FRR table JSON: %w", err)
	}
	if _, ok := document.Routes[control]; !ok {
		return fmt.Errorf("FRR holds no route for the control prefix %s, so the absence of %s proves nothing", control, withheld)
	}
	if _, ok := document.Routes[withheld]; ok {
		return fmt.Errorf("FRR learned %s, which ze must withhold from this peer", withheld)
	}
	return nil
}

// GoBGP's own decode of one route, as `gobgp global rib -a ipv4 <prefix>` prints
// it. Every path attribute that has no column of its own is rendered into the
// Attrs field as `{Name: value}`, so an attribute is PRESENT exactly when its
// brace item is, and the value beside the name is what GoBGP read off the wire.
// The text form is read rather than the JSON because it prints the RECEIVED
// attribute list verbatim, which is the question these scenarios ask, and because
// the deleted Python checkers measured this rendering against this daemon.
//
// The name is anchored on its opening brace and its colon, so an attribute whose
// value carries another attribute's name can never answer for it.
var gobgpAttributeItem = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*): ?([^{}]*)\}`)

// The two attribute names the egress-attribute scenarios read. GoBGP spells
// MULTI_EXIT_DISC `Med` and LOCAL_PREF `LocalPref`.
const (
	gobgpMEDAttribute       = "Med"
	gobgpLocalPrefAttribute = "LocalPref"
)

// gobgpRoute is the one route line GoBGP printed for a prefix: its whole fields,
// which carry the columns GoBGP gives an attribute of its own, and the attribute
// items it rendered into the Attrs field.
type gobgpRoute struct {
	fields     []string
	attributes map[string]string
}

// gobgpRouteFor returns the single route GoBGP holds for prefix. Three conditions
// carry the lookup. A line naming prefix as a whole FIELD is required, so
// 110.54.0.0/24 can never answer for 10.54.0.0/24 and an unanswered query can
// never answer at all. Exactly one such line is required, because two paths for
// one prefix leave an attribute assertion picking one of them, which is a guess.
// The line must then carry at least one attribute item: ORIGIN is well-known
// mandatory and GoBGP renders it there for every route, so a line with none is a
// message that named the prefix rather than a decode of it.
func gobgpRouteFor(table, prefix string) (gobgpRoute, error) {
	var route gobgpRoute
	lines := 0
	for line := range strings.SplitSeq(table, "\n") {
		fields := strings.Fields(line)
		if !slices.Contains(fields, prefix) {
			continue
		}
		lines++
		if lines > 1 {
			continue
		}
		items := gobgpAttributeItem.FindAllStringSubmatch(line, -1)
		route.fields = fields
		route.attributes = make(map[string]string, len(items))
		for _, item := range items {
			route.attributes[item[1]] = item[2]
		}
	}
	if lines == 0 {
		return gobgpRoute{}, fmt.Errorf("GoBGP's answer names no route for %s", prefix)
	}
	if lines != 1 {
		return gobgpRoute{}, fmt.Errorf("GoBGP holds %d route lines for %s, want 1", lines, prefix)
	}
	if len(route.attributes) == 0 {
		return gobgpRoute{}, fmt.Errorf("GoBGP's line for %s decodes no path attribute, so it is not a route", prefix)
	}
	return route, nil
}

// requireGoBGPSourceBlock reports whether the route GoBGP holds for prefix is the
// one the source put on the wire. The AS and the next hop are matched as whole
// FIELDS of that route's line, so an AS that is a text prefix of another and a
// longer address carrying the wanted one both fail. Every attribute assertion is
// read against this: a ze that forwarded nothing, or that rebuilt the route from
// something other than the received block, would satisfy "GoBGP carries no
// LOCAL_PREF" for a reason that has nothing to do with the requirement.
func requireGoBGPSourceBlock(table, prefix, sourceAS, nextHop string) error {
	route, err := gobgpRouteFor(table, prefix)
	if err != nil {
		return err
	}
	if !slices.Contains(route.fields, sourceAS) {
		return fmt.Errorf("GoBGP route %s carries no AS %s: %s", prefix, sourceAS, strings.Join(route.fields, " "))
	}
	if !slices.Contains(route.fields, nextHop) {
		return fmt.Errorf("GoBGP route %s carries no next hop %s: %s", prefix, nextHop, strings.Join(route.fields, " "))
	}
	return nil
}

// requireGoBGPAttributeAbsent reports whether GoBGP decoded no attribute named
// name on the route it holds for prefix. An attribute carrying zero is PRESENT and
// GoBGP renders it as `{name: 0}`, so this separates a stripped attribute from one
// set to zero. The two are different outcomes, and every scenario that calls this
// turns on exactly that difference.
func requireGoBGPAttributeAbsent(table, prefix, name string) error {
	route, err := gobgpRouteFor(table, prefix)
	if err != nil {
		return err
	}
	value, carried := route.attributes[name]
	if carried {
		return fmt.Errorf("GoBGP route %s still carries %s %s", prefix, name, value)
	}
	return nil
}

// requireGoBGPAttributeValue reports whether GoBGP decoded name as exactly want on
// the route it holds for prefix. The value is compared whole, so 1000 never
// satisfies an assertion about 100, and an ABSENT attribute fails rather than
// reading as any value at all.
func requireGoBGPAttributeValue(table, prefix, name, want string) error {
	route, err := gobgpRouteFor(table, prefix)
	if err != nil {
		return err
	}
	value, carried := route.attributes[name]
	if !carried {
		return fmt.Errorf("GoBGP route %s carries no %s at all, want %s", prefix, name, want)
	}
	if value != want {
		return fmt.Errorf("GoBGP decoded %s %s for %s, want %s", name, value, prefix, want)
	}
	return nil
}

// The native speaker's end-of-run report (runSpeakerHelper, speaker.go). Two
// shapes carry it: `<key>: <value>` at the top level for the verdict and the
// oracle, and `note: <key>: <value>` for each measurement. A field is therefore
// the WHOLE first token of its line, or the whole second token behind `note:`,
// which is what separates `established:` from a line naming `not-established:`.
const (
	speakerFieldResult       = "result"
	speakerFieldPlugin       = "plugin"
	speakerFieldRouteUpdates = "route-bearing-updates"
	speakerFieldEVPNNLRI     = "evpn-nlri"
	speakerNoteMarker        = "note:"
	speakerFailMarker        = "fail:"
	speakerResultPass        = "PASS"
)

// speakerReportField returns the value the speaker printed for key, and reports
// whether it printed the key at all. An absent key is an absent answer rather than
// an empty value: the caller decides what a missing measurement means.
func speakerReportField(report, key string) (string, bool) {
	token := key + ":"
	for line := range strings.SplitSeq(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == token {
			return strings.Join(fields[1:], " "), true
		}
		if len(fields) > 1 && fields[0] == speakerNoteMarker && fields[1] == token {
			return strings.Join(fields[2:], " "), true
		}
	}
	return "", false
}

// speakerReportPrinted reports whether the speaker has finished its run and
// printed its verdict line. A body waits on this and never on what the verdict
// says, so a wrong verdict reads as a wrong verdict rather than as a timeout.
func speakerReportPrinted(report string) bool {
	_, printed := speakerReportField(report, speakerFieldResult)
	return printed
}

// speakerReportFailures returns every finding the speaker's oracle recorded, so a
// FAIL verdict is reported with the reason the speaker gave for it.
func speakerReportFailures(report string) []string {
	var failures []string
	for line := range strings.SplitSeq(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == speakerFailMarker {
			failures = append(failures, strings.Join(fields[1:], " "))
		}
	}
	return failures
}

// speakerReportCount returns the count the speaker printed for key. A key it never
// printed, and a value that is not a count, are both errors: a measurement that
// was never taken must not read as zero, because zero is a meaningful answer here.
func speakerReportCount(report, key string) (int, error) {
	value, printed := speakerReportField(report, key)
	if !printed {
		return 0, fmt.Errorf("the speaker printed no %s note", key)
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("the speaker %s note %q is not a count: %w", key, value, err)
	}
	return count, nil
}

// requireSpeakerEVPNDiscard reports whether the independent speaker's own verdict
// says the assigned EVPN route type reached it and the unassigned one did not.
// Five facts carry the decision, and the first four are what stop the fifth from
// being vacuous. The verdict line is required, because an unfinished run says
// nothing. The oracle name is required, because a report from another scenario's
// speaker would otherwise answer here. Established is required, because a session
// that never came up relayed nothing. One route-bearing UPDATE and one decoded
// EVPN NLRI are required, because the behavior under test REMOVES a route: a
// verdict that only reported no unassigned type would read the same on a relay
// that was broken outright. PASS is then the discard itself, because
// applySpeakerOracle fails the run for every EVPN route type outside 1 to 5.
func requireSpeakerEVPNDiscard(report string) error {
	result, printed := speakerReportField(report, speakerFieldResult)
	if !printed {
		return errors.New("the speaker printed no verdict line, so nothing was read")
	}
	plugin, printed := speakerReportField(report, speakerFieldPlugin)
	if !printed {
		return errors.New("the speaker named no oracle, so the verdict belongs to no requirement")
	}
	if plugin != speakerOracleNoUnrecognizedEVPNType {
		return fmt.Errorf("the speaker ran oracle %q, want %q", plugin, speakerOracleNoUnrecognizedEVPNType)
	}
	established, printed := speakerReportField(report, fieldEstablished)
	if !printed || established != logValueYes {
		return fmt.Errorf("the speaker reports established %q, want %q", established, logValueYes)
	}
	updates, err := speakerReportCount(report, speakerFieldRouteUpdates)
	if err != nil {
		return err
	}
	if updates < 1 {
		return errors.New("the speaker received no route-bearing UPDATE, so ze relayed nothing and the discard is unproven")
	}
	routes, err := speakerReportCount(report, speakerFieldEVPNNLRI)
	if err != nil {
		return err
	}
	if routes < 1 {
		return errors.New("the speaker decoded no EVPN NLRI, so the assigned route type never arrived and the discard is unproven")
	}
	if result != speakerResultPass {
		return fmt.Errorf("the speaker verdict is %s: %s", result, strings.Join(speakerReportFailures(report), "; "))
	}
	return nil
}
