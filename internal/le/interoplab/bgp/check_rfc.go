// Design: docs/architecture/testing/interop.md -- RFC evidence executed by the native lab action.
// Related: check_special.go -- registry that dispatches these scenario-specific checkers.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const addPathEvidencePrefix = "10.99.0.0/24"

// RFC requirement: RFC7911-2-2 positive -- a BGP speaker that re-advertises a route
// generates its own Path Identifier and does not relay the received one. Asserted at
// FRR, a foreign daemon holding the RIB Ze filled, because the loss this requirement
// prevents happens at a RECEIVER: RFC 7911 Section 5 keys replacement on (prefix,
// Path Identifier), so two paths sharing one identifier collapse into one route, and
// nothing in Ze's own view of what it sent would show it.
func checkAddPathReadvertiseCollision(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkScenario(ctx, check, "bgp-addpath-readvertise-collision-frr"); err != nil {
		return err
	}
	zeAddress := networkHostAddress(check.Network, 2)
	before, err := queryFRREstablishedEpoch(ctx, check.Lab, zeAddress)
	if err != nil {
		return err
	}
	live, err := waitAddPathState(ctx, check.Lab)
	if err != nil {
		return err
	}
	if _, err := check.Lab.Exec(ctx, "frr", []string{"vtysh", "-c", "clear bgp " + zeAddress}, nil); err != nil {
		return err
	}
	if err := waitFRRNewEpoch(ctx, check.Lab, zeAddress, before); err != nil {
		return err
	}
	replayed, err := waitAddPathState(ctx, check.Lab)
	if err != nil {
		return err
	}
	if !samePathIdentifiers(live, replayed) {
		return fmt.Errorf("replayed Path Identifiers differ from live identifiers: live=%v replayed=%v", live, replayed)
	}
	return nil
}

// RFC requirement: RFC4271-5.1.5-2 positive -- "A BGP speaker MUST NOT include
// this attribute in UPDATE messages it sends to external peers, except in the
// case of BGP Confederations [RFC3065]." Ze has no confederation configuration
// surface, so the exception cannot apply to either session here.
func checkLocalPrefStrip(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-local-pref-strip-gobgp")
}

// RFC requirement: RFC4271-5.1.4-1 positive -- "The MULTI_EXIT_DISC attribute
// received from a neighboring AS MUST NOT be propagated to other neighboring
// ASes" (Section 5.1.4). AS 65004's metric reaches neither AS 65002 nor AS
// 65003.
// RFC requirement: RFC4271-5.1.4-1 negative -- the MUST NOT covers a RECEIVED
// value and nothing else. Ze's own metric of 42 arrives at AS 65003 intact,
// judged by that daemon's RIB.
func checkMEDAcrossAS(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-med-across-as-gobgp")
}

// RFC requirement: RFC4271-5.1.4-4 positive -- "A BGP speaker MUST implement a
// mechanism (based on local configuration) that allows the MULTI_EXIT_DISC
// attribute to be removed from a route." The configured prefix reaches GoBGP
// carrying the injected AS_PATH and NEXT_HOP but no MED.
// RFC requirement: RFC4271-5.1.4-4 negative -- the mechanism is what an operator
// selects, not an unconditional strip. The control prefix from the same peer,
// outside the configured match, reaches GoBGP with MED 100 intact.
func checkMEDRemovalConfiguration(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-med-remove-configured-gobgp")
}

// RFC requirement: RFC4271-5.1.2-3 positive -- an independent conforming receiver (FRR 10.3.1) reports AS_PATH "65001 65004" on a route ze relays to it as an ordinary external peer, so the local AS is prepended when a route IS advertised.
// RFC requirement: RFC4271-5.1.2-3 negative -- the same receiver accepts ze's withdrawal of that route with no attribute error, because the clause's condition ("advertises the route") is not met and no AS_PATH is created. RFC 4271 Section 6.3 makes the opposite a Missing Well-known Attribute error.
func checkRelayWithdrawalShape(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-relay-withdraw-shape-frr")
}

// RFC requirement: RFC2545-3-2 positive -- an independent conforming receiver (FRR
// 10.3.1) decodes a Length of Next Hop Network Address octet of 32 on the on-link
// route, reporting one global-scope and one link-local-scope next hop, and an octet
// of 16 on the off-link route, reporting the global-scope entry alone. The two
// routes cross the same session, so the length octet is the only thing that can
// differ between the two decodes.
// RFC requirement: RFC2545-3-3 positive -- the link-local address IS included when
// the speaker shares a common subnet with BOTH the entity named by the global next
// hop and the peer the route is advertised to. FRR reports fe80::be:ef:2 as a
// second, link-local-scope next hop for 2001:db8:5601::/48 and installs the route
// via it (`B>* ... via fe80::be:ef:2, eth0`), so the receiver both parsed and used
// the second address.
// RFC requirement: RFC2545-3-3 negative -- the link-local address is NOT included
// when the speaker shares no subnet with the entity named by the global next hop,
// even though the peer half of the condition holds and the same `link-local` leaf
// is configured. FRR reports 2001:db8:ffff::1 as the sole next hop of
// 2001:db8:5602::/48, with no link-local-scope entry. A leaf that decided inclusion
// by itself would put fe80::be:ef:2 on this route too, and this assertion is what
// fails when it does.
func checkRFC2545NextHops(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkScenario(ctx, check, "bgp-rfc2545-linklocal-nexthop-frr"); err != nil {
		return err
	}
	onLink, err := queryFRRNextHops(ctx, check.Lab, "2001:db8:5601::/48")
	if err != nil {
		return err
	}
	offLink, err := queryFRRNextHops(ctx, check.Lab, "2001:db8:5602::/48")
	if err != nil {
		return err
	}
	if !check.Network.IPv6.IsValid() {
		return errors.New("RFC 2545 scenario has no selected IPv6 network")
	}
	expectedGlobal := check.Network.IPv6.Addr()
	for range 2 {
		expectedGlobal = expectedGlobal.Next()
	}
	if err := requireNextHopShape(onLink, expectedGlobal, netip.MustParseAddr("fe80::be:ef:2")); err != nil {
		return fmt.Errorf("on-link route: %w", err)
	}
	if err := requireNextHopShape(offLink, netip.MustParseAddr("2001:db8:ffff::1"), netip.Addr{}); err != nil {
		return fmt.Errorf("off-link route: %w", err)
	}
	return nil
}

// RFC requirement: RFC7606-5.1-3 positive -- ONE UPDATE mixing Withdrawn Routes with NLRI is
// ACCEPTED on receive, relayed, and installed by a real FRR. Section 5.1's second bullet forbids
// any conforming SENDER to produce that shape, so a raw injector is the only carrier that can
// drive the third bullet's "MUST still be prepared to receive these fields in any position or
// combination" clause against a foreign peer. The existing unit binding
// (message/rfc7606_test.go) proves the "any position" clause only; its own audit note says so.
func checkRFC7606MixedUpdate(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-rfc7606-relay-shape-frr")
}

// RFC requirement: RFC7606-5.4-1 positive -- an independent peer receives the assigned EVPN route type and never the unassigned one ze was sent in the same attribute.
func checkRFC7606TypedNLRIDiscard(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-rfc7606-typed-nlri-discard")
}

// RFC requirement: RFC7999-3.3-1 positive -- "The announced prefix is covered by an equal or shorter prefix that the neighboring network is authorized to advertise" (RFC 7999 Section 3.3, first condition). FRR 10.3.1 announces 10.100.0.1/32 carrying 65535:666, inside the 10.100.0.0/24 that peer is authorized for, and the Linux FIB in the ze container holds `blackhole 10.100.0.1`. The condition holds and the announcement is honored, asserted on kernel state rather than on a Ze table.
// RFC requirement: RFC7999-3.3-1 negative -- the same FRR session announces 198.51.100.1/32 with the same community, outside every entry of that peer's blackhole `prefixes`. The kernel holds an ordinary `via` route for it and no discard route, so the first condition failing withholds honoring. Without this polarity the check passes equally when the community alone grants a discard.
// RFC requirement: RFC7999-3.3-2 positive -- "The receiving party agreed to honor the BLACKHOLE community on the particular BGP session" (RFC 7999 Section 3.3, second condition). The FRR session names that community in its blackhole `communities`, which is that agreement, and the same 10.100.0.1/32 reaches the kernel as a discard route. Both conditions of the one MUST sentence hold on this session, which is why one outcome is positive evidence for both.
// RFC requirement: RFC7999-3.3-2 negative -- BIRD announces 10.200.0.1/32 carrying 65535:666, inside the 10.200.0.0/24 that peer IS authorized for, on a session whose blackhole `communities` names 65001:666 alone. The kernel forwards it. The authorization is present and the session agreed to a DIFFERENT community, so this isolates the second condition rather than testing an absent config block, and it also pins that a stated community list is taken exactly: the well-known value is never added to it.
func checkRFC7999Blackhole(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-rfc7999-blackhole-frr")
}

// RFC requirement: RFC9234-5-4 positive -- an independent conforming receiver (FRR 10.3.1) reports the OTC Attribute carrying ze's local AS on a route ze advertises to it as a Customer.
// RFC requirement: RFC9234-5-4 negative -- the same receiver raises no attribute error over ze's withdrawal of that route and keeps the session up. RFC 7606 Section 5.2 makes the opposite a session-reset hazard, so this is the polarity a peer can actually punish. Stamping a withdrawal produces the error (measured, see the docstring); the emitted bytes themselves are pinned by test/plugin/role-otc-fwd-withdraw.ci.
func checkOTCWithdrawal(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name   = "bgp-role-otc-withdraw-frr"
		prefix = "10.10.0.0/24"
	)
	positive := []operation{
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: prefix, timeout: 60 * time.Second},
	}
	for index := range positive {
		if err := runOperation(ctx, check.Network, check.Lab, &positive[index]); err != nil {
			return checkerFailure(ctx, check.Lab, name, index+1, err)
		}
	}
	output, err := check.Lab.Query(ctx, "frr", []string{"vtysh", "-c", "show bgp ipv4 unicast " + prefix}, nil)
	if err != nil {
		return checkerFailure(ctx, check.Lab, name, 3, err)
	}
	value, err := parseOTCValue(output)
	if err != nil {
		return checkerFailure(ctx, check.Lab, name, 3, err)
	}
	if value != 65001 {
		return checkerFailure(ctx, check.Lab, name, 3, fmt.Errorf("FRR reports OTC %d, want local AS 65001", value))
	}
	negative := []operation{
		{kind: opFRRRouteAbsent, argument: prefix, timeout: 90 * time.Second},
		{kind: opFRRSession, argument: "172.30.0.2"},
	}
	negative = append(negative, relayWithdrawalExtras(prefix)...)
	negative = append(negative, operation{kind: opFRRSession, argument: "172.30.0.2"})
	for index := range negative {
		if err := runOperation(ctx, check.Network, check.Lab, &negative[index]); err != nil {
			return checkerFailure(ctx, check.Lab, name, index+4, err)
		}
	}
	return nil
}

// RFC requirement: RFC7947-x-1 positive -- a route server does not prepend its own AS to a
// relayed route. Asserted at BIRD, a foreign daemon parsing the wire Ze emitted, rather than
// from Ze's own RIB view: an AS-path transparency claim read back out of the speaker that
// built the path proves the least interesting half of it.
func checkRouteServerASPath(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-route-server-frr")
}

// RFC requirement: RFC4271-5.1.3-1 positive -- FRR, a conforming implementation, never decodes an UPDATE carrying 10.11.0.0/24, whose NEXT_HOP 172.30.0.3 is FRR's own address. The assertion is on FRR's own per-UPDATE log rather than on its table, because FRR applies Section 6.3(a) itself and would drop such a route either way.
// RFC requirement: RFC4271-5.1.3-1 negative -- the same session, one message later, receives 10.12.0.0/24 with the third-party NEXT_HOP 172.30.0.9 and installs it with that address. Section 5.1.3 case 2 permits a third-party next hop, so a relay that withheld everything would be a different violation, and this half is what stops the absence above from passing vacuously.
func checkSelfNextHopWithheld(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-self-nexthop-withheld-frr")
}

// RFC requirement: RFC1997-Well-1 positive -- "All routes received carrying a communities attribute containing this value [NO_EXPORT] MUST NOT be advertised outside a BGP confederation boundary" (RFC 1997, Well-known Communities). An independent conforming receiver (FRR) never learns 10.10.0.0/24, while it does learn 10.11.0.0/24 relayed by the same Ze over the same session in the same run.
// RFC requirement: RFC1997-Well-1 negative -- the clause's condition is "outside a BGP confederation boundary", and Ze runs a stand-alone AS, which RFC 1997 says to consider a confederation itself. A second independent receiver INSIDE that boundary (BIRD, AS 65001) learns the same 10.10.0.0/24, so the prohibition is scoped rather than a blanket refusal.
func checkNoExportBoundary(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "bgp-wellknown-noexport-frr")
}

// RFC requirement: RFC5301-3-4 positive -- "The Dynamic hostname TLV is
// defined here as TLV type 137" (RFC 5301 Section 3). FRR resolves the name
// only by decoding type 137, so a different type would leave a raw system ID
// in its database.
// RFC requirement: RFC5301-3-6 positive -- "Value - a string of 1 to 255
// bytes" (RFC 5301 Section 3). FRR reads the whole configured name, so the
// length octet framed the value FRR then rendered.
func checkISISDynamicHostname(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "isis-p2p-frr")
}

// RFC requirement: RFC4724-4-1 positive -- an independent conforming receiver
// (FRR 10.3.1) decodes ze's End-of-RIB marker for IPv4 unicast on a session where
// neither speaker advertised a Multiprotocol capability. Removing the per-side
// implicit family in capability.Negotiate leaves FRR receiving the route and no
// marker, which is the state this scenario was written against (measured
// 2026-08-17).
func checkNoFamilyEndOfRIB(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "no-family-peer-eor-frr")
}

// RFC requirement: RFC3101-2.4-5 positive -- the NSSA border router
// originates a default into every directly attached NSSA without a config gate.
func checkNSSADefault(ctx context.Context, check *interoplab.CheckContext) error {
	return checkScenario(ctx, check, "ospf-stub-nssa-frr")
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
	paths := document.Routes[addPathEvidencePrefix]
	if len(paths) < 2 {
		return nil, fmt.Errorf("FRR holds %d paths for %s, want 2", len(paths), addPathEvidencePrefix)
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

func parseOTCValue(output string) (uint64, error) {
	index := strings.Index(strings.ToUpper(output), "OTC")
	if index < 0 {
		return 0, errors.New("FRR reported no OTC Attribute")
	}
	tail := output[index+len("OTC"):]
	for len(tail) > 0 {
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
func waitAddPathState(ctx context.Context, lab interoplab.CheckerLab) (map[string]uint64, error) {
	state, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: 2 * time.Second, Description: "two re-advertised paths with distinct Path Identifiers"}, func(probeCtx context.Context) (map[string]uint64, error) {
		output, err := lab.Query(probeCtx, "frr", []string{"vtysh", "-c", "show bgp ipv4 unicast detail json"}, nil)
		if err != nil {
			return nil, err
		}
		return parseAddPathState(output)
	}, func(state map[string]uint64) bool { return len(state) == 2 })
	return state, err
}

func queryFRREstablishedEpoch(ctx context.Context, lab interoplab.CheckerLab, neighbor string) (uint64, error) {
	output, err := lab.Query(ctx, "frr", []string{"vtysh", "-c", "show bgp neighbor " + neighbor + " json"}, nil)
	if err != nil {
		return 0, err
	}
	var peers map[string]struct {
		Epoch uint64 `json:"bgpTimerUpEstablishedEpoch"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return 0, fmt.Errorf("decode FRR neighbor JSON: %w", err)
	}
	if peers[neighbor].Epoch == 0 {
		return 0, errors.New("FRR neighbor has no established epoch")
	}
	return peers[neighbor].Epoch, nil
}

func waitFRRNewEpoch(ctx context.Context, lab interoplab.CheckerLab, neighbor string, previous uint64) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 90 * time.Second, Interval: 2 * time.Second, Description: "FRR session re-establishment"}, func(probeCtx context.Context) (uint64, error) {
		return queryFRREstablishedEpoch(probeCtx, lab, neighbor)
	}, func(epoch uint64) bool { return epoch != 0 && epoch != previous })
	return err
}

func samePathIdentifiers(left, right map[string]uint64) bool {
	return len(left) == len(right) && left["65003"] == right["65003"] && left["65004"] == right["65004"]
}

type nextHop struct {
	Scope string `json:"scope"`
	IP    string `json:"ip"`
}

type frrPrefixDocument struct {
	Paths []struct {
		NextHops []nextHop `json:"nexthops"`
	} `json:"paths"`
}

func parseFRRNextHops(output string) ([]nextHop, error) {
	var document frrPrefixDocument
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, fmt.Errorf("decode FRR route JSON: %w", err)
	}
	if len(document.Paths) != 1 {
		return nil, fmt.Errorf("FRR route has %d paths, want 1", len(document.Paths))
	}
	if len(document.Paths[0].NextHops) == 0 {
		return nil, errors.New("FRR route carries no next-hop entries")
	}
	return document.Paths[0].NextHops, nil
}

func queryFRRNextHops(ctx context.Context, lab interoplab.CheckerLab, prefix string) ([]nextHop, error) {
	output, err := lab.Query(ctx, "frr", []string{"vtysh", "-c", "show bgp ipv6 unicast " + prefix + " json"}, nil)
	if err != nil {
		return nil, err
	}
	return parseFRRNextHops(output)
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
		case "global":
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
