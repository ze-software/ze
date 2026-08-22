// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- RFC 7296 Section 2.9 traffic-selector narrowing
// RFC: rfc/short/rfc7296.md -- Traffic Selector negotiation (Section 2.9)

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// tisPeer is a peer whose operator configured exactly one selector pair: local 10.1.0.0/16
// to remote 10.2.0.0/16. Every test below measures a responder answer against it.
func tisPeer(t *testing.T) ipsec.SiteToSitePeer {
	t.Helper()
	return ipsec.SiteToSitePeer{
		Name: "tis",
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, "10.1.0.0/16"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "10.2.0.0/16"),
			RemotePort:   ipsec.AnyPort(),
		}},
	}
}

// tisInitiator builds an initiator SA that has already sent its proposal, so
// ProposedChildPairs holds whatever proposeChildTSPayloads put on the wire.
func tisInitiator(t *testing.T, peer ipsec.SiteToSitePeer) *SA {
	t.Helper()
	sa := &SA{PeerCfg: peer, IsInitiator: true}
	tsi, tsr := proposeChildTSPayloads(sa)
	if tsi == nil || tsr == nil {
		t.Fatal("proposeChildTSPayloads produced no proposal, so the fixture sends nothing")
	}
	return sa
}

// VALIDATES: an initiator refuses a responder answer that is WIDER than the selectors it
// proposed, and refuses one wider than the operator's configured policy.
// PREVENTS: the policy bypass this check exists to close. RFC 7296 Section 2.9 lets a
// responder narrow and never widen, and the initiator used to install whatever came back.
// A peer answering 0.0.0.0/0 to a proposal of 10.1.0.0/16 had ze program a policy for the
// whole internet, chosen entirely by the far end.
func TestTisInitiatorRefusesAWidenedAnswer(t *testing.T) {
	sa := tisInitiator(t, tisPeer(t))

	// Anti-vacuity: the proposal really carries the operator's /16 pair, so the refusal
	// below is measured against something ze actually sent.
	if len(sa.ProposedChildPairs) != 1 {
		t.Fatalf("the recorded proposal holds %d pairs, want 1; the ceiling is not being recorded",
			len(sa.ProposedChildPairs))
	}
	if got := sa.ProposedChildPairs[0].I.Net.String(); got != "10.1.0.0/16" {
		t.Fatalf("the recorded proposal TSi is %s, want 10.1.0.0/16", got)
	}

	err := recordInitiatorSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0"),
		tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0"), nil)
	if !errors.Is(err, errTSWidened) {
		t.Fatalf("an answer of 0.0.0.0/0 to a proposal of 10.1.0.0/16 returned %v, want errTSWidened", err)
	}
	if sa.NegotiatedPairs != nil {
		t.Error("a widened answer was still recorded, so a Child SA could be installed from it")
	}
	if sa.NegotiatedTSi != nil || sa.NegotiatedTSr != nil {
		t.Error("a widened answer set the negotiated prefixes")
	}

	// One half wider is enough. A responder that narrows TSi honestly and widens TSr still
	// chooses traffic the operator never permitted.
	half := &SA{PeerCfg: sa.PeerCfg, IsInitiator: true}
	proposeChildTSPayloads(half)
	err = recordInitiatorSelectors(half,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.1.0/24"),
		tsPayload(t, wire.PayloadTypeTSr, "10.0.0.0/8"), nil)
	if !errors.Is(err, errTSWidened) {
		t.Errorf("an answer widening only TSr returned %v, want errTSWidened", err)
	}
}

// VALIDATES: the discriminator. A responder answer that is a genuine SUBSET of the proposal
// is accepted and installed, and an answer equal to the proposal is too.
// PREVENTS: the subset check becoming a blanket refusal, which would break every peer that
// narrows -- the behavior RFC 7296 Section 2.9 explicitly permits the responder to use.
func TestTisInitiatorAcceptsANarrowedAnswer(t *testing.T) {
	sa := tisInitiator(t, tisPeer(t))

	if err := recordInitiatorSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.1.0/24"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.1.0/24"), nil); err != nil {
		t.Fatalf("a narrowed answer was refused: %v", err)
	}
	if got := sa.NegotiatedTSi.String(); got != "10.1.1.0/24" {
		t.Errorf("negotiated TSi = %s, want the responder's narrowed 10.1.1.0/24", got)
	}
	if got := sa.NegotiatedTSr.String(); got != "10.2.1.0/24" {
		t.Errorf("negotiated TSr = %s, want the responder's narrowed 10.2.1.0/24", got)
	}

	// An answer equal to the proposal is a subset of it and is accepted unchanged.
	same := tisInitiator(t, tisPeer(t))
	if err := recordInitiatorSelectors(same,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.0.0/16"), nil); err != nil {
		t.Errorf("an answer equal to the proposal was refused: %v", err)
	}
}

// VALIDATES: a peer with no configured traffic-selector list proposes the wildcard and
// accepts any answer, because it constrained nothing.
// PREVENTS: the subset check breaking every configuration written before the list existed.
// Those propose 0.0.0.0/0, so no answer can go past what they asked for. A refusal here is
// one the check invented, and not one the operator wrote.
func TestTisUnconfiguredPeerAcceptsAnyAnswer(t *testing.T) {
	sa := tisInitiator(t, ipsec.SiteToSitePeer{Name: "tis-open"})
	if len(sa.ProposedChildPairs) != 0 {
		t.Fatalf("an unconfigured peer recorded %d proposed pairs, want none; the wildcard "+
			"went on the wire and a recorded ceiling would be one the peer was never told about",
			len(sa.ProposedChildPairs))
	}

	if err := recordInitiatorSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "192.168.5.0/24"),
		tsPayload(t, wire.PayloadTypeTSr, "172.16.9.0/24"), nil); err != nil {
		t.Fatalf("an unconfigured peer refused an answer: %v", err)
	}
	if sa.NegotiatedTSi == nil || sa.NegotiatedTSi.String() != "192.168.5.0/24" {
		t.Error("an unconfigured peer did not install the answer it accepted")
	}
}

// VALIDATES: a transport-mode initiator accepts the answer to the proposal it was REQUIRED
// to send, even when that proposal names addresses the operator's configured prefixes do not
// contain.
// PREVENTS: the subset check breaking transport mode. RFC 7296 Section 2.23.1 requires
// "exactly one IP address in the TSi and TSr payloads", so transportSelectorPairs replaces
// the operator's prefixes with the SA's own endpoints. A check of the answer against the
// configured POLICY as well as the proposal refuses the responder's correct narrowing of
// ze's own correct proposal. Every transport-mode tunnel then fails to establish.
func TestTisTransportModeAnswerIsNotRefusedByPolicyPrefixes(t *testing.T) {
	peer := tisPeer(t)
	peer.LocalAddress = "192.0.2.1"
	peer.RemoteAddress = "192.0.2.2"
	peer.Mode = dataplane.ModeTransport

	sa := tisInitiator(t, peer)

	// Anti-vacuity: the proposal really did move outside the configured 10.1.0.0/16, so an
	// answer inside the proposal is genuinely outside the policy.
	if len(sa.ProposedChildPairs) != 1 {
		t.Fatalf("the transport-mode proposal holds %d pairs, want 1", len(sa.ProposedChildPairs))
	}
	if got := sa.ProposedChildPairs[0].I.Net.String(); got != "192.0.2.1/32" {
		t.Fatalf("the transport-mode proposal TSi is %s, want the SA's own 192.0.2.1/32", got)
	}
	if pairWithinAny(sa.ProposedChildPairs[0], policyPairs(peer, true)) {
		t.Fatal("the fixture's endpoints sit inside the configured prefixes, so this test " +
			"cannot detect a policy check that refuses a pinned transport proposal")
	}

	if err := recordInitiatorSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "192.0.2.1/32"),
		tsPayload(t, wire.PayloadTypeTSr, "192.0.2.2/32"), nil); err != nil {
		t.Fatalf("the answer to ze's own transport-mode proposal was refused: %v", err)
	}

	// The ceiling still binds: an answer outside the pinned proposal is refused.
	other := tisInitiator(t, peer)
	if err := recordInitiatorSelectors(other,
		tsPayload(t, wire.PayloadTypeTSi, "198.51.100.7/32"),
		tsPayload(t, wire.PayloadTypeTSr, "192.0.2.2/32"), nil); !errors.Is(err, errTSWidened) {
		t.Errorf("a transport-mode answer naming a third address returned %v, want errTSWidened", err)
	}
}

// VALIDATES: an answer this node can neither decode nor program is REFUSED, and the refusal
// says which. The SA is not established with no negotiated selector at all.
//
// PREVENTS: the fail-open guard this test exists for. Both exits returned nil, which skipped
// checkAnswerWithin AND left NegotiatedPairs unset, so the caller read success over an
// answer that was neither checked nor adopted (ai/rules/evidence.md).
func TestTisUndecodableAnswerIsRefusedRatherThanIgnored(t *testing.T) {
	sa := tisInitiator(t, tisPeer(t))

	// TS payloads carrying no selector at all: present on the wire, empty after decoding.
	empty := &wire.PayloadTS{}
	err := recordInitiatorSelectors(sa, empty, empty, nil)
	if err == nil {
		t.Fatal("an answer carrying no decodable selector was accepted, so the SA would " +
			"establish with no negotiated traffic selector and nothing would say so")
	}
	if !errors.Is(err, errTSUnusable) {
		t.Errorf("the refusal is %v, want errTSUnusable", err)
	}
	if sa.NegotiatedPairs != nil {
		t.Error("the refused answer still left negotiated pairs on the SA")
	}

	// The discriminator: a decodable, programmable, narrowed answer is adopted through the
	// same function, so the refusal above is about the answer and not unconditional.
	ok := tisInitiator(t, tisPeer(t))
	if err := recordInitiatorSelectors(ok,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.1.0/24"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.1.0/24"), nil); err != nil {
		t.Errorf("a narrowed answer was refused: %v", err)
	} else if len(ok.NegotiatedPairs) == 0 {
		t.Error("a narrowed answer was accepted but recorded no negotiated pair")
	}
}

// VALIDATES: the refusal reaches the production entry point and DELETES the SA, rather than
// being a return value nobody reads.
// PREVENTS: the check existing but not being wired. adoptAuthResponseNegotiation is what
// handleAuthResponse (fsm.go) calls, and a false return is what makes it set StateDead.
func TestTisWidenedAnswerTearsTheSADown(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := tisInitiator(t, tisPeer(t))

	adopted, notify := adoptAuthResponseNegotiation(sa, false,
		tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0"),
		tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0"), log)
	if adopted {
		t.Fatal("a widened answer was adopted, so the initiator would establish an SA " +
			"carrying traffic the responder chose")
	}
	// RFC 7296 Section 2.9 names the notification the peer is owed for an answer this node
	// will not accept. A teardown that reports nothing leaves the peer holding a live SA.
	if notify != wire.NotifyTSUnacceptable {
		t.Errorf("the widened answer tore the SA down with notify %d, want TS_UNACCEPTABLE (%d)",
			notify, wire.NotifyTSUnacceptable)
	}

	// The discriminator: a narrowed answer is adopted through the same entry point, so the
	// teardown above is a decision about that answer.
	ok := tisInitiator(t, tisPeer(t))
	if adopted, _ := adoptAuthResponseNegotiation(ok, false,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.1.0/24"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.1.0/24"), log); !adopted {
		t.Error("a narrowed answer tore the SA down; the check is unconditional")
	}
}
