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
