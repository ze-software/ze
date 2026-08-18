package peer

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireMessage builds a BGP frame whose body is the given hex, so a test can
// place a needle at a known byte offset.
func wireMessage(t *testing.T, bodyHex string) *Message {
	t.Helper()
	body, err := hex.DecodeString(bodyHex)
	require.NoError(t, err)
	header := make([]byte, HeaderLen)
	copy(header, Marker)
	header[16] = byte((HeaderLen + len(body)) >> 8)
	header[17] = byte((HeaderLen + len(body)) & 0xFF)
	header[18] = MsgUPDATE
	return &Message{Header: header, Body: body}
}

// TestSplitRejectRules pins what a reject=bgp: rule must carry.
//
// VALIDATES: AC-4 — a rejection that could never match is a parse error, not a
// rule that silently passes.
func TestSplitRejectRules(t *testing.T) {
	expect, rejects, err := splitRejectRules([]string{
		"expect=bgp:conn=1:seq=1:contains=18C00002",
		"reject=bgp:conn=1:pattern=180a0100",
		"reject=bgp:conn=2:pattern=180A0200",
		"reject=bgp:conn=1:pattern=180A0300",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"expect=bgp:conn=1:seq=1:contains=18C00002"}, expect)
	assert.Equal(t, map[int][]string{
		1: {"180A0100", "180A0300"},
		2: {"180A0200"},
	}, rejects, "patterns are upper-cased and grouped by connection")

	for name, rule := range map[string]string{
		"missing conn":  "reject=bgp:pattern=180A0100",
		"conn zero":     "reject=bgp:conn=0:pattern=180A0100",
		"conn not a nu": "reject=bgp:conn=x:pattern=180A0100",
		"missing patte": "reject=bgp:conn=1",
		"odd hex":       "reject=bgp:conn=1:pattern=180A010",
		"not hex":       "reject=bgp:conn=1:pattern=zzzz",
		// pattern= swallows the rest of the line in ci.ParseKVPairs, so this
		// spelling leaves conn unset. The runner's guard reads the line through
		// this same function, so both refuse it and neither is left believing
		// the other saw a conn.
		"conn after pattern": "reject=bgp:pattern=180A0100:conn=1",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := splitRejectRules([]string{rule})
			assert.Error(t, err, "a rejection that can never match must be refused at parse time")
		})
	}
}

// TestCheckerRejectedIsByteAligned proves the needle is matched on wire bytes,
// not on hex characters.
//
// VALIDATES: AC-4 — a rejection fires on the bytes it names and on no others.
func TestCheckerRejectedIsByteAligned(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:contains=18C00002",
		"reject=bgp:conn=1:pattern=180A0100",
	})
	require.NoError(t, err)
	c.Init()

	needle, found := c.rejection(wireMessage(t, "0000001B180A0100"))
	assert.True(t, found, "the forbidden bytes are on the wire")
	assert.Equal(t, "180A0100", needle)

	// The same hex characters, shifted one nibble: 0x81 0x0A 0x01 0x00 8 holds
	// the needle's characters at an odd offset, which is not a wire byte.
	_, found = c.rejection(wireMessage(t, "0000001B8180A010008"[:18]))
	assert.False(t, found, "an odd-offset character match is not a byte match")

	_, found = c.rejection(wireMessage(t, "0000001B18C00002"))
	assert.False(t, found, "a frame carrying other routes is accepted")
}

// TestCheckerRejectedIsPerConnection proves a rejection is scoped to the
// connection it names.
//
// VALIDATES: AC-4 — conn= selects which of a multi-connection peer's sessions
// the rejection governs.
func TestCheckerRejectedIsPerConnection(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:contains=18C00002",
		"expect=bgp:conn=2:seq=1:contains=18C00002",
		"reject=bgp:conn=2:pattern=180A0100",
	})
	require.NoError(t, err)

	c.Init()
	_, found := c.rejection(wireMessage(t, "0000001B180A0100"))
	assert.False(t, found, "connection 1 carries no rejection")

	// Consume connection 1's expectation so the checker advances to connection 2.
	require.True(t, c.Expected(wireMessage(t, "0000001B18C00002")))
	c.Init()
	needle, found := c.rejection(wireMessage(t, "0000001B180A0100"))
	assert.True(t, found, "connection 2 carries the rejection")
	assert.Equal(t, "180A0100", needle)
}

// TestCheckerRejectedNeedsNoRejection proves the common path costs nothing.
//
// VALIDATES: a peer block with no reject= behaves exactly as before.
func TestCheckerRejectedNeedsNoRejection(t *testing.T) {
	c, err := newChecker([]string{"expect=bgp:conn=1:seq=1:contains=18C00002"})
	require.NoError(t, err)
	c.Init()
	_, found := c.rejection(wireMessage(t, "0000001B180A0100"))
	assert.False(t, found)
}

// TestNewRefusesRejectOutsideCheckMode proves ze-peer fails closed rather than
// attributing a rejection to a connection it cannot identify.
//
// VALIDATES: AC-4 — sink and echo run every accepted connection concurrently
// against one Checker, so conn= cannot select a session there.
func TestNewRefusesRejectOutsideCheckMode(t *testing.T) {
	rules := []string{
		"expect=bgp:conn=1:seq=1:contains=18C00002",
		"reject=bgp:conn=1:pattern=180A0100",
	}
	_, err := New(&Config{Mode: ModeCheck, Expect: rules})
	require.NoError(t, err, "check mode is the one mode that reads connections in turn")

	for _, mode := range []Mode{ModeSink, ModeEcho} {
		_, err := New(&Config{Mode: mode, Expect: rules})
		require.Error(t, err, "mode %v must refuse a rejection it cannot attribute", mode)
		assert.Contains(t, err.Error(), "check mode")
	}

	_, err = New(&Config{Mode: ModeSink, Expect: []string{"expect=bgp:conn=1:seq=1:contains=18C00002"}})
	require.NoError(t, err, "a block with no rejection is untouched by the refusal")
}

// TestParseRejectRulePassesOtherLinesThrough proves the exported parser answers
// only for its own directive, which is what lets the runner call it on every
// line of a peer block.
func TestParseRejectRulePassesOtherLinesThrough(t *testing.T) {
	for _, line := range []string{
		"expect=bgp:conn=1:seq=1:contains=18C00002",
		"reject=stderr:pattern=level=DEBUG",
		"option=linger:value=true",
		"# a comment",
		"",
	} {
		conn, pattern, isReject, err := ParseRejectRule(line)
		require.NoError(t, err, line)
		assert.False(t, isReject, line)
		assert.Zero(t, conn)
		assert.Empty(t, pattern)
	}
}

// TestRejectedPrintsTheRejectionMarker pins the peer half of the linger verdict.
//
// VALIDATES: a rejection writes RejectionMarker to the peer's own output, which
// is the only channel the runner's verdict reads once the peer has already
// announced success.
// PREVENTS: the marker being dropped from rejected() while
// failedCheckPeers (internal/test/runner/peer_contract.go) still looks for it.
// The two halves share only this constant, so nothing else would go red: every
// negative assertion held open by option=linger would quietly become vacuous
// again, which is the exact defect this pair was written to close.
func TestRejectedPrintsTheRejectionMarker(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:contains=18C00002",
		"reject=bgp:conn=1:pattern=180A0100",
	})
	require.NoError(t, err)
	c.Init()

	var out strings.Builder
	p := &Peer{config: &Config{}, checker: c, output: &out}

	res, rejected := p.rejected(wireMessage(t, "0000001B180A0100"))
	assert.True(t, rejected, "the forbidden bytes are on the wire")
	assert.False(t, res.Success, "a rejection fails the peer")
	assert.Contains(t, out.String(), RejectionMarker,
		"the rejection must be visible in the peer's output, not only in the returned Result")

	var clean strings.Builder
	p2 := &Peer{config: &Config{}, checker: c, output: &clean}
	_, rejected = p2.rejected(wireMessage(t, "0000001B18C00002"))
	assert.False(t, rejected, "a frame carrying other routes is accepted")
	assert.NotContains(t, clean.String(), RejectionMarker,
		"a clean frame must not print the marker; it would fail every lingering peer")
}
