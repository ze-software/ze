package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/require"
)

// Reproduction of the RFC 7911 Section 2 re-advertisement defect, written before
// the fix (plan/spec-rfc7911-generate-own-path-id.md, phase 1).
//
// Both tests drive buildFwdBody, which is the function that produces the bytes a
// destination peer reads. That level is deliberate: when source and destination
// share an encoding context, buildFwdBody returns the received payload zero-copy
// and never reaches fwdReencodeNLRIs at all (forward_body.go, the sameCtx branch).
// A generator installed only inside fwdReencodeNLRIs would leave that branch, the
// one a route server between like-configured clients takes, still copying the
// received identifier. Asserting on buildFwdBody's output covers both branches.

// TestForwardPathIDsDifferForCollidingSources reproduces the route loss.
//
// VALIDATES: AC-2 -- two peers that both choose Path Identifier 1 for the same
// prefix are re-advertised to a third ADD-PATH peer with DIFFERENT identifiers.
// PREVENTS: a route server relaying two paths as one (prefix, Path Identifier)
// pair, which the receiver reads as a replacement and not as a second path.
// RFC requirement: RFC7911-2-2 positive -- a speaker that re-advertises a route
// generates its own Path Identifier. Path Identifiers are chosen independently by
// each source peer, so two sources choosing 1 is ordinary rather than unlucky. The
// identifier is the only field on the wire that separates the two paths, so equal
// identifiers here are one path at the receiver and a lost route in the network.
//
// The assertion is inequality, not a pinned value: RFC 7911 leaves the choice to
// the speaker, so a test that pinned a number would be asserting an implementation
// rather than the requirement. It cannot pass spuriously by the two frames simply
// differing, because it reads the identifier field out of each destination frame
// and compares those two fields alone.
func TestForwardPathIDsDifferForCollidingSources(t *testing.T) {
	// One context for both sources and the destination: every client negotiated
	// ADD-PATH with the same capabilities, which is what a route server sees and
	// what selects buildFwdBody's zero-copy branch.
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	require.True(t, ctx.AddPath(family.IPv4Unicast), "destination must have ADD-PATH negotiated")
	peer := forwardBodyTestPeer(ctx, ctxID)

	const collidingPathID = 1
	fromAS65001 := pathIDTestBody(t, 65001, collidingPathID)
	fromAS65002 := pathIDTestBody(t, 65002, collidingPathID)

	// Both frames carry a SourceID, which session_read.go sets on every accepted
	// UPDATE. Without it the two read as ONE peer announcing one path twice, which
	// RFC 7911 Section 5 makes a replacement rather than a second path, so a
	// generator that also answers withdraws must give them one identifier.
	firstWire := wireu.NewWireUpdate(fromAS65001, ctxID)
	firstWire.SetSourceID(1)
	secondWire := wireu.NewWireUpdate(fromAS65002, ctxID)
	secondWire.SetSourceID(2)

	firstResult, ok := buildFwdBody(firstWire, message.MaxMsgLen, ctxID, peer, netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
	require.True(t, ok, "first source UPDATE must forward")
	defer ReturnReadBuffer(firstResult.transcodeBuf)
	secondResult, ok := buildFwdBody(secondWire, message.MaxMsgLen, ctxID, peer, netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
	require.True(t, ok, "second source UPDATE must forward")
	defer ReturnReadBuffer(secondResult.transcodeBuf)

	first := forwardedPathID(t, firstResult)
	second := forwardedPathID(t, secondResult)
	require.NotEqual(t, first, second,
		"both paths for 10.0.0.0/24 left with Path Identifier %d, so the destination sees one path where it must see two", first)
}

// TestForwardPathIDStableAcrossUpdates guards the fix rather than the bug.
//
// VALIDATES: AC-1 and AC-3 -- the identifier ze advertises is its own, and it is
// the same one every time that path is re-advertised.
// PREVENTS: a generator that mints per message. The receiver keys its table on
// (prefix, Path Identifier), so a fresh identifier on each refresh accumulates one
// table entry per UPDATE instead of replacing the path.
// RFC requirement: RFC7911-2-2 negative -- the re-advertised identifier is NOT the
// received one. That half fails today: ze copies the ingress value, which is why
// the stability half passes today for the wrong reason. Both halves are needed,
// and neither is sufficient. Copying the source's value is trivially stable, and a
// per-message counter is trivially not the received one.
//
// The received identifier is 0xDEADBEEF rather than a small number so that
// "the emitted value is not the received value" cannot pass by a generator
// happening to mint the same low integer the source chose.
func TestForwardPathIDStableAcrossUpdates(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	require.True(t, ctx.AddPath(family.IPv4Unicast), "destination must have ADD-PATH negotiated")
	peer := forwardBodyTestPeer(ctx, ctxID)

	const receivedPathID = 0xDEADBEEF
	body := pathIDTestBody(t, 65001, receivedPathID)

	firstResult, ok := buildFwdBody(wireu.NewWireUpdate(body, ctxID), message.MaxMsgLen, ctxID, peer, netip.MustParseAddr("192.0.2.11"), &fwdParseCache{})
	require.True(t, ok, "first advertisement must forward")
	defer ReturnReadBuffer(firstResult.transcodeBuf)
	secondResult, ok := buildFwdBody(wireu.NewWireUpdate(body, ctxID), message.MaxMsgLen, ctxID, peer, netip.MustParseAddr("192.0.2.11"), &fwdParseCache{})
	require.True(t, ok, "re-advertisement of the same path must forward")
	defer ReturnReadBuffer(secondResult.transcodeBuf)

	first := forwardedPathID(t, firstResult)
	second := forwardedPathID(t, secondResult)
	require.NotEqual(t, uint32(receivedPathID), first,
		"the re-advertised Path Identifier is the received one, so ze is advertising a value it does not own")
	require.Equal(t, first, second,
		"the same path left with two different Path Identifiers, so a refresh reads as a new path at the receiver")
}

// pathIDTestBody builds an UPDATE body announcing 10.0.0.0/24 with the given
// ADD-PATH Path Identifier, from the given source AS. The AS number is what makes
// two otherwise identical announcements two distinct paths.
func pathIDTestBody(t *testing.T, sourceAS, pathID uint32) []byte {
	t.Helper()
	var pathIDBytes [4]byte
	binary.BigEndian.PutUint32(pathIDBytes[:], pathID)
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nlriBytes := append(pathIDBytes[:], nlri.NewINET(family.IPv4Unicast, prefix, 0).Bytes()...)
	return buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, sourceAS), [][]byte{nlriBytes})
}

// forwardedPathID reads the single Path Identifier out of the destination frame
// buildFwdBody produced, whichever branch produced it: rawBodies for the zero-copy
// same-context forward, updates for the re-encoded one.
func forwardedPathID(t *testing.T, result fwdBodyResult) uint32 {
	t.Helper()
	var nlriBytes []byte
	switch {
	case len(result.rawBodies) > 0:
		require.Len(t, result.rawBodies, 1, "test fixture must produce one destination frame")
		update, err := message.UnpackUpdate(result.rawBodies[0])
		require.NoError(t, err)
		nlriBytes = update.NLRI
	default:
		require.Len(t, result.updates, 1, "test fixture must produce one destination frame")
		nlriBytes = result.updates[0].NLRI
	}

	iter := nlri.NewNLRIIterator(nlriBytes, true)
	_, pathID, ok := iter.Next()
	require.True(t, ok, "destination frame carries no NLRI")
	_, _, more := iter.Next()
	require.False(t, more, "test fixture must announce one prefix")
	require.Zero(t, iter.Remaining(), "destination NLRI is malformed")
	return pathID
}
