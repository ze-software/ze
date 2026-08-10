package peer

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The three frames of the test/plugin/mup4.ci and test/plugin/ipv4.ci shape: a
// plugin's announce, ze's own initial-sync End-of-RIB, and the plugin's
// withdraw, declared as three consecutive seq groups.
const (
	orderAnnounceHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00300200000015400101004002004003040A0001FE400504000000C8180A0001"
	orderEORHex      = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"
	orderWithdrawHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A00010000"
)

func mkFrame(t *testing.T, frameHex string) *Message {
	t.Helper()
	raw, err := hex.DecodeString(frameHex)
	require.NoError(t, err)
	require.Greater(t, len(raw), HeaderLen)
	return &Message{Header: raw[:HeaderLen], Body: raw[HeaderLen:]}
}

func mkOrderedChecker(t *testing.T) *Checker {
	t.Helper()
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:hex=" + orderAnnounceHex,
		"expect=bgp:conn=1:seq=2:hex=" + orderEORHex,
		"expect=bgp:conn=1:seq=3:hex=" + orderWithdrawHex,
	})
	require.NoError(t, err)
	c.Init()
	return c
}

// TestCheckerEORExpectedInALaterGroupIsNamedByTheFailure pins the one thing the
// silent-accept tolerance must never do: destroy a frame the fixture is still
// asking for and leave no trace of it.
//
// VALIDATES: an End-of-RIB that matches an expectation in a LATER seq group is
// accepted (a daemon emits markers on its own schedule and a second identical
// one CAN still fill the declared slot) and RECORDED, so the failure that
// follows names the frame that actually arrived out of order.
// PREVENTS: the tolerance cannibalizing a later expectation in silence.
// consumeMatches searches the CURRENT group only, so the seq-2 marker below is
// invisible while seq 1 is unsatisfied; accepting it with no record destroyed
// it, seq 1 was then satisfied by the announce, and the seq-2 marker slot was
// offered the WITHDRAW. The reported failure was "Expected UPDATE (len=23),
// Received UPDATE (len=27) WITHDRAWN" -- two frames neither of which is the one
// that went wrong -- and the peer log showed two `msg recv` lines for three
// received frames.
//
// Refusing the marker on arrival was the first answer and it was wrong: the two
// meanings of an identical marker are told apart only by what comes AFTER it, so
// the refusal red test/plugin/role-otc-rs-withdraw-eor.ci, where ze's own
// establishment marker precedes the relayed one the fixture declares at seq 3
// (checker_relay_probe_test.go).
func TestCheckerEORExpectedInALaterGroupIsNamedByTheFailure(t *testing.T) {
	c := mkOrderedChecker(t)

	matched, silent := c.ExpectedOrKeepalive(mkFrame(t, orderEORHex))
	assert.False(t, matched, "the marker does not satisfy the seq-1 announce expectation")
	assert.True(t, silent, "the marker is accepted: a second identical one can still fill seq 2")

	note := c.takeMisorderNote()
	assert.Contains(t, note, orderEORHex,
		"the marker must be recorded where it landed, so a run that ends in a timeout still names it")

	matched, _ = c.ExpectedOrKeepalive(mkFrame(t, orderAnnounceHex))
	assert.True(t, matched, "the announce satisfies seq 1")

	// seq 2 owes a marker, and the only frame left is the withdraw.
	matched, silent = c.ExpectedOrKeepalive(mkFrame(t, orderWithdrawHex))
	assert.False(t, matched, "the withdraw does not satisfy the seq-2 marker expectation")
	assert.False(t, silent, "a withdraw is no marker and is never accepted in silence")

	expected, received := c.lastMismatch()
	assert.Equal(t, orderEORHex, expected)
	assert.Equal(t, orderWithdrawHex, received)
	assert.Contains(t, c.misorderNotes(), orderEORHex,
		"the report must name the frame that arrived out of order beside the two innocent ones")
}

// TestCheckerEORUnexpectedIsStillSwallowed keeps the tolerance doing its job.
//
// VALIDATES: an End-of-RIB no remaining expectation matches is still accepted in
// silence -- a marker for a second negotiated family, or for a peer whose
// fixture asserts routes only.
// PREVENTS: over-tightening the rule above into "every End-of-RIB must be
// declared", which would red every fixture that negotiates a family it does not
// assert on.
func TestCheckerEORUnexpectedIsStillSwallowed(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:hex=" + orderAnnounceHex,
		"expect=bgp:conn=1:seq=2:hex=" + orderWithdrawHex,
	})
	require.NoError(t, err)
	c.Init()

	matched, silent := c.ExpectedOrKeepalive(mkFrame(t, orderEORHex))

	assert.False(t, matched)
	assert.True(t, silent, "a marker no expectation asks for is noise and stays silent")
	assert.Empty(t, c.takeMisorderNote(), "noise is not an out-of-order arrival and earns no note")
}

// TestCheckerEORInOrderMatchesAcrossGroups proves the refusal is about ORDER and
// nothing else: the same three frames delivered in the declared order all match.
//
// VALIDATES: the tolerance change leaves the passing path untouched -- announce
// at seq 1, marker at seq 2, withdraw at seq 3, all consumed by hex match.
// PREVENTS: a fix that reds the correct wire order along with the wrong one.
func TestCheckerEORInOrderMatchesAcrossGroups(t *testing.T) {
	c := mkOrderedChecker(t)

	for _, frameHex := range []string{orderAnnounceHex, orderEORHex, orderWithdrawHex} {
		matched, silent := c.ExpectedOrKeepalive(mkFrame(t, frameHex))
		assert.True(t, matched, "frame %s must match its own expectation", frameHex)
		assert.False(t, silent, "a matched frame is never a silent accept")
	}
	assert.True(t, c.Completed(), "all three expectations must be satisfied")
	assert.Empty(t, c.misorderNotes(), "the declared order records nothing out of order")
}
