// Design: docs/architecture/testing/interop.md -- RFC evidence executed by the native lab action.
// Related: check_special.go -- registry that dispatches these scenario-specific checkers.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// RFC requirement: RFC7911-2-2 positive -- a BGP speaker that re-advertises a route
// generates its own Path Identifier and does not relay the received one. Asserted at
// FRR, a foreign daemon holding the RIB Ze filled, because the loss this requirement
// prevents happens at a RECEIVER: RFC 7911 Section 5 keys replacement on (prefix,
// Path Identifier), so two paths sharing one identifier collapse into one route, and
// nothing in Ze's own view of what it sent would show it.
func checkAddPathReadvertiseCollision(ctx context.Context, check *interoplab.CheckContext) error {
	const name = "bgp-addpath-readvertise-collision-frr"
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("ADD-PATH collision scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	sessions := []operation{
		{kind: opFRRSession, argument: zeAddress},
		{kind: opGoBGPSession, argument: zeAddress},
	}
	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+1, err)
		}
	}

	// Assertion 3. Without ADD-PATH on the ze to FRR session, FRR keeps one path for
	// a reason that has nothing to do with the Path Identifier, and every assertion
	// below reads a table whose shape the capability decided.
	neighbor, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}, nil)
	if err != nil {
		return fail(3, err)
	}
	if !addPathReceiveNegotiated(neighbor) {
		return fail(3, errors.New("FRR did not negotiate ADD-PATH receive with ze"))
	}

	// Assertion 4. BIRD announces the same prefix when its session comes up, so
	// injecting GoBGP's path here makes the two paths reach FRR by different rails:
	// BIRD's through the peer-up replay, GoBGP's through the live forward.
	if _, err := check.Lab.Exec(ctx, peerGoBGP, []string{
		cmdGoBGP, gobgpGlobal, gobgpRIB, gobgpAdd, peerPrefixFirst, "-a", gobgpFamilyIPv4,
		gobgpNextHop, networkHostAddress(check.Network, 5),
	}, nil); err != nil {
		return fail(4, err)
	}
	live, err := waitAddPathState(ctx, check.Lab)
	if err != nil {
		return fail(5, err)
	}

	// Assertions 6 to 9. The identifier belongs to the path, not to the delivery, so
	// the replay after a reset must repeat it. The reset is watched by the epoch FRR
	// reports rather than by the prefix going away: ze reconnects in about a second,
	// so polling for the absence loses the race and reports a reset that plainly
	// happened as one that never did.
	before, err := queryFRREstablishedEpoch(ctx, check.Lab, zeAddress)
	if err != nil {
		return fail(6, err)
	}
	if _, err := check.Lab.Exec(ctx, peerFRR, []string{cmdVtysh, "-c", "clear bgp " + zeAddress}, nil); err != nil {
		return fail(7, err)
	}
	if err := waitFRRNewEpoch(ctx, check.Lab, zeAddress, before); err != nil {
		return fail(8, err)
	}
	replayed, err := waitAddPathState(ctx, check.Lab)
	if err != nil {
		return fail(9, err)
	}
	if !samePathIdentifiers(live, replayed) {
		return fail(10, fmt.Errorf("replayed Path Identifiers differ from live identifiers: live=%v replayed=%v", live, replayed))
	}
	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+11, err)
		}
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
	const (
		name             = "bgp-rfc2545-linklocal-nexthop-frr"
		onLinkPrefix     = "2001:db8:5601::/48"
		offLinkPrefix    = "2001:db8:5602::/48"
		offLinkNextHop   = "2001:db8:ffff::1"
		linkLocalNextHop = "fe80::be:ef:2"
	)
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("RFC 2545 scenario has no selected IPv4 network"))
	}
	if !check.Network.IPv6.IsValid() {
		return fail(1, errors.New("RFC 2545 scenario has no selected IPv6 network"))
	}
	// Every assertion below reads its prefix back out of the operation that waited
	// for the route, so the wait and the assertion can never name two routes.
	session := operation{kind: opFRRSession, argument: networkHostAddress(check.Network, 2)}
	onLinkRoute := operation{kind: opFRRRoute, argument: onLinkPrefix, family: frrFamilyIPv6Unicast, timeout: 60 * time.Second}
	offLinkRoute := operation{kind: opFRRRoute, argument: offLinkPrefix, family: frrFamilyIPv6Unicast, timeout: 60 * time.Second}
	for index, step := range []*operation{&session, &onLinkRoute, &offLinkRoute} {
		if err := runOperation(ctx, check.Network, check.Lab, step); err != nil {
			return fail(index+1, err)
		}
	}

	// Assertion 4. ze.conf names fd00:1e:0::2 as the on-link global next hop, which
	// the harness rewrites onto the selected IPv6 network: host 2 on that /64.
	expectedGlobal := check.Network.IPv6.Addr().Next().Next()
	// One parse, so the shape assertion and the installed-route assertion below can
	// never name two addresses, and FRR's listing is matched against the canonical
	// rendering rather than against a second spelling of the same address.
	linkLocal := netip.MustParseAddr(linkLocalNextHop)
	onLink, err := queryFRRNextHops(ctx, check.Lab, onLinkRoute.argument)
	if err != nil {
		return fail(4, err)
	}
	if err := requireNextHopShape(onLink, expectedGlobal, linkLocal); err != nil {
		return fail(4, fmt.Errorf("on-link route: %w", err))
	}

	// Assertion 5. FRR forwards through the link-local address, so the second next
	// hop was not merely parsed but used. Without this half the RFC2545-3-3 positive
	// tag claims an installation that nothing reads.
	routes, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show ipv6 route bgp"}, nil)
	if err != nil {
		return fail(5, err)
	}
	if err := requireRouteInstalledVia(routes, onLinkRoute.argument, linkLocal.String()); err != nil {
		return fail(5, err)
	}

	// Assertion 6. The same session, the same `link-local` leaf, and a global next
	// hop on no locally connected prefix: the link-local address is absent.
	offLink, err := queryFRRNextHops(ctx, check.Lab, offLinkRoute.argument)
	if err != nil {
		return fail(6, err)
	}
	if err := requireNextHopShape(offLink, netip.MustParseAddr(offLinkNextHop), netip.Addr{}); err != nil {
		return fail(6, fmt.Errorf("off-link route: %w", err))
	}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(7, err)
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
		prefix = injectPrefixFirst
	)
	positive := []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opFRRRoute, argument: prefix, timeout: 60 * time.Second},
	}
	for index := range positive {
		if err := runOperation(ctx, check.Network, check.Lab, &positive[index]); err != nil {
			return checkerFailure(ctx, check.Lab, name, index+1, err)
		}
	}
	output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix}, nil)
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
	extras := relayWithdrawalExtras(prefix)
	negative := make([]operation, 0, len(extras)+3)
	negative = append(negative,
		operation{kind: opFRRRouteAbsent, argument: prefix, timeout: 90 * time.Second},
		operation{kind: opFRRSession, argument: zeLabAddress},
	)
	negative = append(negative, extras...)
	negative = append(negative, operation{kind: opFRRSession, argument: zeLabAddress})
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
	const name = "isis-p2p-frr"
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	neighbors := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", frrShowISISNeighbor}, nil)
	}

	// Assertion 1. The point-to-point adjacency reaches Up, which takes the RFC 5303
	// three-way handshake completing on both sides.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR IS-IS adjacency Up",
	}, neighbors, isisAdjacencyUp); err != nil {
		return fail(1, err)
	}

	// Assertion 2. FRR renders ze's LSP by the NAME ze advertises rather than by its
	// system ID, and it prints a name there only after decoding TLV 137. This is an
	// independent implementation reading ze's Dynamic Hostname off the wire. The
	// 7-bit ASCII rule (RFC5301-3-7) is NOT provable here, because a conforming peer
	// accepts the octets it is given; it is enforced and proven at the config
	// boundary instead (test/isis/isis-hostname-ascii.ci).
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    2 * time.Second,
		Description: "ze dynamic hostname in the FRR IS-IS database",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", frrShowISISDatabase}, nil)
	}, isisDatabaseNamesZe); err != nil {
		return fail(2, err)
	}

	// Assertion 3. The adjacency is still Up after a settle, so the name above was
	// read from a stable adjacency rather than from one that flapped.
	settle := time.NewTimer(5 * time.Second)
	defer settle.Stop()
	select {
	case <-ctx.Done():
		return fail(3, ctx.Err())
	case <-settle.C:
	}
	adjacencies, err := neighbors(ctx)
	if err != nil {
		return fail(3, err)
	}
	if !isisAdjacencyUp(adjacencies) {
		return fail(3, errors.New("the point-to-point IS-IS adjacency did not stay Up"))
	}
	return nil
}

// RFC requirement: RFC4724-4-1 positive -- an independent conforming receiver
// (FRR 10.3.1) decodes ze's End-of-RIB marker for IPv4 unicast on a session where
// neither speaker advertised a Multiprotocol capability. Removing the per-side
// implicit family in capability.Negotiate leaves FRR receiving the route and no
// marker, which is the state this scenario was written against (measured
// 2026-08-17).
func checkNoFamilyEndOfRIB(ctx context.Context, check *interoplab.CheckContext) error {
	const name = "no-family-peer-eor-frr"
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("no-family End-of-RIB scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	session := operation{kind: opFRRSession, argument: zeAddress}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(1, err)
	}

	// Assertion 2. IPv4 unicast is exchanged in this state, which RFC 4271 carries
	// with no capability at all. Without this assertion the End-of-RIB below would
	// be a barrier over an empty conversation.
	route := operation{kind: opFRRRoute, argument: injectPrefixFirst, timeout: 60 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &route); err != nil {
		return fail(2, err)
	}

	// Assertion 3. The marker is owed because the family is IMPLICIT, not because ze
	// named it: FRR reports the IPv4-unicast capability as advertised by itself and
	// received from nobody. That pins the fix in capability.Negotiate rather than in
	// the OPEN builder, which is what keeps the wire byte-identical for every peer
	// configured this way.
	neighbor, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}, nil)
	if err != nil {
		return fail(3, err)
	}
	if err := requireMultiprotocolAdvertisedOnly(neighbor, zeAddress); err != nil {
		return fail(3, err)
	}

	// Assertion 4. The marker is the last frame of the initial update, so it can land
	// after the route: poll FRR's own per-UPDATE decode. Neither a routing table nor
	// `show bgp neighbor json` answers this question here, because FRR fills
	// gracefulRestartInfo.endOfRibRecv only for the families a peer named in a
	// Graceful Restart capability, and this peer advertises neither that capability
	// nor a Multiprotocol one.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    time.Second,
		Description: "FRR decode of the End-of-RIB marker",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	}, func(log string) bool {
		return endOfRIBDecoded(log, zeAddress)
	}); err != nil {
		return fail(4, err)
	}

	// Assertion 5. The session is still established after the exchange, so the marker
	// above was decoded on a live session rather than on one that was failing.
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return fail(5, err)
	}
	return nil
}

// RFC requirement: RFC3101-2.4-5 positive -- the NSSA border router
// originates a default into every directly attached NSSA without a config gate.
func checkNSSADefault(ctx context.Context, check *interoplab.CheckContext) error {
	const (
		name         = "ospf-stub-nssa-frr"
		defaultRoute = "0.0.0.0/0"
		// The two vtysh commands this scenario alone runs. FRR takes one after
		// each -c flag, as the shared commands in names.go do.
		showOSPFRoute   = "show ip route ospf"
		showExternalLSA = "show ip ospf database external"
	)
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("NSSA default scenario has no selected IPv4 network"))
	}

	// Assertion 1. The adjacency forms only when the two N-bit options agree, so
	// reaching Full is itself the option-match assertion.
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, 90*time.Second, ospfStateFull); err != nil {
		return fail(1, err)
	}

	// Assertion 2. FRR is NSSA internal, so the default it installs is the Type 7 one
	// the NSSA border router originates. ze.conf carries no `default-originate` leaf,
	// so nothing gates that origination on an operator. The border router is required
	// on the SAME line as the prefix: a default reaching FRR from anywhere else would
	// otherwise satisfy the assertion.
	borderRouter := networkHostAddress(check.Network, 2)
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     60 * time.Second,
		Interval:    2 * time.Second,
		Description: "NSSA default route from the border router",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", showOSPFRoute}, nil)
	}, func(routes string) bool {
		return requireRouteInstalledVia(routes, defaultRoute, borderRouter) == nil
	}); err != nil {
		return fail(2, err)
	}

	// Assertion 3. No Type 5 AS-external LSA is flooded into the NSSA. FRR prints the
	// database heading whether or not the area holds one, so the heading is the
	// positive proof that the query ran and an `LS age` entry is the leak.
	database, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", showExternalLSA}, nil)
	if err != nil {
		return fail(3, err)
	}
	if err := requireNoExternalLSA(database); err != nil {
		return fail(3, err)
	}
	return nil
}

func waitAddPathState(ctx context.Context, lab interoplab.CheckerLab) (map[string]uint64, error) {
	state, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 60 * time.Second, Interval: 2 * time.Second, Description: "two re-advertised paths with distinct Path Identifiers"}, func(probeCtx context.Context) (map[string]uint64, error) {
		output, err := lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast detail json"}, nil)
		if err != nil {
			return nil, err
		}
		return parseAddPathState(output)
	}, func(state map[string]uint64) bool { return len(state) == 2 })
	return state, err
}

func queryFRREstablishedEpoch(ctx context.Context, lab interoplab.CheckerLab, neighbor string) (uint64, error) {
	output, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}, nil)
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

func queryFRRNextHops(ctx context.Context, lab interoplab.CheckerLab, prefix string) ([]nextHop, error) {
	output, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv6 unicast " + prefix + " json"}, nil)
	if err != nil {
		return nil, err
	}
	return parseFRRNextHops(output)
}
