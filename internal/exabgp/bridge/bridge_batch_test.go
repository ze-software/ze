// Design: docs/architecture/exabgp-bridge.md -- the batch these tests drive
// Related: bridge_batch.go -- the reader, the netting and the answer pass
//
// An operator migrating an ExaBGP script writes several API commands in ONE
// write. ExaBGP holds the commands of one read in its outgoing RIB and lets
// them cancel each other before anything is encoded, so the wire carries the
// NET of that write. These tests drive the same unit for ze: what one read
// holds, what the netting removes, in what order the survivors go out, and that
// every line is still answered.
package bridge

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeReader answers one recorded write per Read call, which is what a pipe
// does for a writer whose writes are under PIPE_BUF.
type writeReader struct {
	writes []string
}

func (r *writeReader) Read(p []byte) (int, error) {
	if len(r.writes) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.writes[0])
	if n < len(r.writes[0]) {
		r.writes[0] = r.writes[0][n:]
		return n, nil
	}
	r.writes = r.writes[1:]
	return n, nil
}

// batchOf translates the lines of one batch, as both runners do before netting.
func batchOf(t *testing.T, lines ...string) []BatchLine {
	t.Helper()
	batch := make([]BatchLine, 0, len(lines))
	// The version moves as ScriptVersion.Detect moves it for a fresh script:
	// the first deciding line holds the rest.
	version := APIVersionAuto
	for _, line := range lines {
		if version == APIVersionAuto {
			version = CommandAPIVersion(line)
		}
		translation, err := Translator{Families: []string{"ipv4/unicast"}}.Line(line)
		require.NoError(t, err, line)
		batch = append(batch, BatchLine{Text: line, Version: version, Translation: translation})
	}
	return batch
}

// dispatchedText is the command text of a netted batch, in dispatch order.
func dispatchedText(netted []BatchDispatch) []string {
	texts := make([]string, 0, len(netted))
	for _, dispatch := range netted {
		texts = append(texts, dispatch.Command.Text)
	}
	return texts
}

// TestBridgeBatchCutsOneReadIntoOneBatch is the unit the netting acts on.
//
// VALIDATES: the four lines of one write are answered as ONE batch, and a
// trailing line with no newline is carried rather than dispatched half-written.
// PREVENTS: netting over a boundary the script did not write, and a truncated
// command reaching ze's dispatcher as a shorter command that parses.
func TestBridgeBatchCutsOneReadIntoOneBatch(t *testing.T) {
	reader := NewBatchReader(&writeReader{writes: []string{
		"announce route 1.1.0.0/24 next-hop 101.1.101.1\n" +
			"announce route 1.1.0.0/25 next-hop 101.1.101.1\n" +
			"withdraw route 1.1.0.0/24 next-hop 101.1.101.1\n" +
			"announce route 1.1.0.0/25 next-hop 101.1.101.1\n",
		"announce route 2.2.0.0/25 next-hop 101.1.101.1\nannounce route ",
		"2.2.0.0/24 next-hop 101.1.101.1\n",
	}})

	first, err := reader.Next()
	require.NoError(t, err)
	assert.Len(t, first, 4, "one write of four lines is one batch")

	second, err := reader.Next()
	require.NoError(t, err)
	require.Len(t, second, 1, "the partial line is carried, not dispatched")
	assert.Equal(t, "announce route 2.2.0.0/25 next-hop 101.1.101.1", second[0])

	third, err := reader.Next()
	require.NoError(t, err)
	require.Len(t, third, 1, "the carried line completes in the next batch")
	assert.Equal(t, "announce route 2.2.0.0/24 next-hop 101.1.101.1", third[0])
}

// TestBridgeBatchSecondReadIsSecondBatch pins the rule that keeps ze's batches
// no coarser than ExaBGP's.
//
// VALIDATES: the reader does not wait for more input to grow a batch, so two
// writes are two batches and nothing in the second cancels anything in the
// first.
// PREVENTS: an announce and a later withdrawal the script wrote in separate
// writes canceling each other, which would remove a frame the ExaBGP fixtures
// list (api-add-remove, api-flow).
func TestBridgeBatchSecondReadIsSecondBatch(t *testing.T) {
	reader := NewBatchReader(&writeReader{writes: []string{
		"announce route 1.1.0.0/24 next-hop 101.1.101.1\n",
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1\n",
	}})

	first, err := reader.Next()
	require.NoError(t, err)
	require.Len(t, first, 1)

	second, err := reader.Next()
	require.NoError(t, err)
	require.Len(t, second, 1)

	assert.Len(t, Net(batchOf(t, first...)), 1, "the announce stands: nothing in this batch withdraws it")
	assert.Len(t, Net(batchOf(t, second...)), 1, "the withdrawal stands: it cancels nothing in its own batch")
}

// TestBridgeBatchRefusesAnOverlongLine drives the read buffer's bound.
//
// VALIDATES: a line past batchLineMax is refused whole and its bytes never
// become a line, while the line after it is read normally.
// PREVENTS: a truncated command reaching ze's dispatcher, where a cut
// `announce route 10.0.0.0/8 next-hop ...` would parse as a shorter command
// that means something else.
func TestBridgeBatchRefusesAnOverlongLine(t *testing.T) {
	long := "announce route 1.1.0.0/24 next-hop 101.1.101.1 community [" +
		strings.Repeat("1:1 ", batchLineMax/4) + "]"
	reader := NewBatchReader(&writeReader{writes: []string{
		long + "\nannounce route 2.2.0.0/24 next-hop 101.1.101.1\n",
	}})

	var lines []string
	for {
		batch, err := reader.Next()
		lines = append(lines, batch...)
		if err != nil {
			break
		}
	}

	require.Len(t, lines, 1, "the overlong line is refused, the next one is read")
	assert.Equal(t, "announce route 2.2.0.0/24 next-hop 101.1.101.1", lines[0])
}

// TestBridgeBatchNetsWithdrawOverEarlierAnnounce is api-fast batch 1's rule.
//
// VALIDATES: a withdrawal cancels an announce of the same route earlier in the
// batch, and neither is dispatched.
// PREVENTS: an announce and its own withdrawal both reaching the wire, which is
// two frames where ExaBGP puts none.
func TestBridgeBatchNetsWithdrawOverEarlierAnnounce(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
	))

	require.Len(t, netted, 1, "the announce is canceled, the withdrawal stands")
	assert.Contains(t, netted[0].Command.Text, "del 1.1.0.0/24")
}

// TestBridgeBatchAnnounceDoesNotCancelWithdraw is the other direction, and it
// is the one that decides api-fast batch 2.
//
// Upstream states it where it drains: "announce does NOT cancel pending
// withdraw. This allows withdraw+announce sequences to both be sent"
// (src/exabgp/rib/outgoing.py).
//
// VALIDATES: `withdraw X` then `announce X` dispatches BOTH, the withdrawal
// first.
// PREVENTS: a symmetric cancellation rule, which would drop the withdrawal of
// 2.2.0.0/25 that api-fast records on the wire.
func TestBridgeBatchAnnounceDoesNotCancelWithdraw(t *testing.T) {
	netted := Net(batchOf(t,
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
	))

	require.Len(t, netted, 2, "both are dispatched")
	assert.Contains(t, netted[0].Command.Text, "del 1.1.0.0/24")
	assert.Contains(t, netted[1].Command.Text, "add 1.1.0.0/24")
}

// TestBridgeBatchWithdrawalsDispatchFirst pins the order inside a batch.
//
// Upstream: "Generate Updates for pending withdraws before announces (preserves
// semantic ordering)" (src/exabgp/rib/outgoing.py). api-fast batch 2 writes the
// announce of 2.2.0.0/24 BEFORE the withdrawal of 2.2.0.0/25 and records the
// withdrawal on the wire first.
//
// VALIDATES: a batch holding an announce of one route and a withdrawal of
// another dispatches the withdrawal first, and both go out.
// PREVENTS: write order reaching the wire, which is the frame order api-fast
// refuses.
func TestBridgeBatchWithdrawalsDispatchFirst(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 2.2.0.0/24 next-hop 101.1.101.1",
		"withdraw route 3.3.0.0/25 next-hop 101.1.101.1",
	))

	require.Len(t, netted, 2)
	assert.Contains(t, netted[0].Command.Text, "del 3.3.0.0/25")
	assert.Contains(t, netted[1].Command.Text, "add 2.2.0.0/24")
}

// TestBridgeBatchKeyMatchesAcrossSpellings drives the two ways one script can
// name one route.
//
// ExaBGP writes `announce route <prefix>`, which states no family, and
// `announce ipv4 unicast <prefix>`, which states it. Both reach one ze command
// through one builder, so both must carry one key.
//
// VALIDATES: a withdrawal written in either spelling cancels an announce
// written in the other.
// PREVENTS: a cancellation that fires for one spelling and not the other, which
// would leave the frames a script gets depending on how it spelled the family.
func TestBridgeBatchKeyMatchesAcrossSpellings(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw ipv4 unicast 1.1.0.0/24 next-hop 101.1.101.1",
	))
	assert.Len(t, netted, 1, "the two spellings name one route")

	netted = Net(batchOf(t,
		"announce ipv4 unicast 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
	))
	assert.Len(t, netted, 1, "and they name it in either order of spelling")
}

// TestBridgeBatchEndOfRIBNeitherCancelsNorIsCancelled guards the zero key.
//
// RFC 4724 Section 2 makes the marker an UPDATE with no reachable NLRI and no
// withdrawn routes, so it names no route.
//
// VALIDATES: an End-of-RIB line survives a batch that cancels a route, and it
// dispatches after the routes.
// PREVENTS: two commands that name nothing canceling each other because their
// keys are both the zero value (ai/rules/principles.md).
func TestBridgeBatchEndOfRIBNeitherCancelsNorIsCancelled(t *testing.T) {
	netted := Net(batchOf(t,
		"announce eor ipv4 unicast",
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
		"announce eor ipv4 unicast",
	))

	texts := dispatchedText(netted)
	require.Len(t, texts, 3, "the announce is canceled; both markers stand")
	assert.Contains(t, texts[0], "del 1.1.0.0/24")
	assert.Contains(t, texts[1], "eor")
	assert.Contains(t, texts[2], "eor")
}

// TestApiFastBatchOneProducesOneAnnounce drives the four lines
// test/exabgp-compat/etc/run/api-fast.run writes in its first flush.
//
// ExaBGP put ONE frame on the wire for them: `announce route 1.1.0.0/25`. The
// netting accounts for two of the four lines; the duplicate announce is the
// Adj-RIB-Out's (adj_rib_out.go) and the withheld withdrawal is the reactor's
// (withdrawBatchFromPeers).
//
// VALIDATES: the batch dispatches the withdrawal of 1.1.0.0/24 first, with its
// own announce canceled, then the two announces of 1.1.0.0/25.
// PREVENTS: the announce of 1.1.0.0/24 reaching the wire, which is the frame
// api-fast does not list.
func TestApiFastBatchOneProducesOneAnnounce(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"announce route 1.1.0.0/25 next-hop 101.1.101.1",
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
		"announce route 1.1.0.0/25 next-hop 101.1.101.1",
	))

	texts := dispatchedText(netted)
	require.Len(t, texts, 3)
	assert.Contains(t, texts[0], "del 1.1.0.0/24")
	assert.Contains(t, texts[1], "add 1.1.0.0/25")
	assert.Contains(t, texts[2], "add 1.1.0.0/25")
	for _, text := range texts {
		assert.NotContains(t, text, "add 1.1.0.0/24", "the canceled announce never dispatches")
	}
}

// TestBridgeBatchDispatchOrderAndAcks drives the contract both runners consume.
//
// VALIDATES: one flush per distinct selector the batch reached, one answer per
// line in write order, a failed line answered `error` on its own, and the rest
// of the batch answered `done`.
// PREVENTS: one line's dispatch failure swallowing the batch's remaining acks,
// which blocks a script for an answer that never comes, and a batch that
// reached two neighbors flushing only one of them.
func TestBridgeBatchDispatchOrderAndAcks(t *testing.T) {
	batch := batchOf(t,
		"neighbor 10.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"neighbor 10.0.0.2 announce route 2.2.0.0/24 next-hop 101.1.101.1",
		"neighbor 10.0.0.1 withdraw route 3.3.0.0/24 next-hop 101.1.101.1",
		"# a comment carries no command",
	)
	netted := Net(batch)

	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, BatchSelectors(batch, netted),
		"one flush per selector, in the order the batch first reached them")

	answers := make([]BatchAnswer, len(batch))
	answers[1] = BatchAnswer{Failed: true, Error: "no peers match selector"}

	var written strings.Builder
	ack := NewAckMode()
	AnswerBatch(&written, &ack, batch, answers)

	assert.Equal(t, "done\nerror: no peers match selector\nerror\ndone\n", written.String())
}

// TestBridgeBatchAckControlAppliesInWriteOrder pins the second reason the
// answers run in write order rather than in dispatch order.
//
// `disable-ack` is itself acked and silences what FOLLOWS it, so a batch that
// answered its lines in dispatch order would silence the wrong ones.
//
// VALIDATES: a batch holding a route, `disable-ack`, and a second route answers
// the first two and not the third.
// PREVENTS: api-ack-control and api-silence-ack losing their meaning inside a
// batch.
func TestBridgeBatchAckControlAppliesInWriteOrder(t *testing.T) {
	batch := batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"disable-ack",
		"announce route 2.2.0.0/24 next-hop 101.1.101.1",
	)

	var written strings.Builder
	ack := NewAckMode()
	AnswerBatch(&written, &ack, batch, make([]BatchAnswer, len(batch)))

	assert.Equal(t, "done\ndone\n", written.String(),
		"the route before disable-ack and disable-ack itself are acked; the route after it is not")
}

// TestBridgeBatchWithdrawsOneRouteOnce is api-flow's own batch.
//
// Upstream queues a withdrawal in a dict keyed by the NLRI index, so a second
// `withdraw` of the same route replaces the first rather than adding to it
// (_del_from_rib_impl, src/exabgp/rib/outgoing.py). api-flow's script writes the
// same flow withdrawal twice, around an announce of that route.
//
// VALIDATES: two withdrawals of one route in one batch dispatch ONE withdrawal,
// the announce between them is still canceled, and the announce after them
// still survives.
// PREVENTS: the second withdrawal api-flow read as an unexpected frame, and a
// dedupe that also swallowed the announce the script wrote last.
func TestBridgeBatchWithdrawsOneRouteOnce(t *testing.T) {
	netted := Net(batchOf(t,
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
	))

	require.Len(t, netted, 2, "one withdrawal and the announce that follows it")
	assert.Contains(t, netted[0].Command.Text, "del 1.1.0.0/24")
	assert.Contains(t, netted[1].Command.Text, "add 1.1.0.0/24")
	assert.Equal(t, 0, netted[0].Line, "the withdrawal answers the line that first wrote it")
	assert.Equal(t, 3, netted[1].Line, "and the announce answers the line the script wrote last")
}

// TestBridgeBatchWithdrawsEachRouteOnce keeps the dedupe on the route it names.
//
// VALIDATES: two withdrawals of DIFFERENT routes in one batch both dispatch.
// PREVENTS: a dedupe keyed on the family or the verb rather than the route,
// which would drop every withdrawal after the first in any batch.
func TestBridgeBatchWithdrawsEachRouteOnce(t *testing.T) {
	netted := Net(batchOf(t,
		"withdraw route 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw route 2.2.0.0/24 next-hop 101.1.101.1",
	))

	require.Len(t, netted, 2, "both routes are withdrawn")
	assert.Contains(t, netted[0].Command.Text, "del 1.1.0.0/24")
	assert.Contains(t, netted[1].Command.Text, "del 2.2.0.0/24")
}

// TestAnswerBatchRefusalInTheLineVersion pins where AnswerBatch takes the form
// of a refusal's reason (X-13, X-14): from the version the script was held to
// when it wrote THAT line.
//
// VALIDATES: an unknown command (ErrUnknownCommand) and a line the translator
// refused are both answered with the reason then `error`; an undecided line
// keeps the text form even when a later line of the same batch holds the
// script to API 6, because ExaBGP answers each line before it reads the next.
// PREVENTS: a translator refusal answered with nothing, which left a script
// blocked for an answer ExaBGP gives ("Could not parse route").
func TestAnswerBatchRefusalInTheLineVersion(t *testing.T) {
	unknown := errors.Join(ErrUnknownCommand, errors.New("v4 command from an API 6 helper"))
	unread := errors.New("cannot read route")
	lines := []BatchLine{
		{Text: "bogus", Version: APIVersionAuto, Err: unread},
		{Text: "announce route 1.1.1.1/32 next-hop 2.2.2.2", Version: APIV6, Err: unknown},
		{Text: "peer * bogus", Version: APIV6, Err: unread},
	}
	answers := make([]BatchAnswer, len(lines))

	var written strings.Builder
	ack := NewAckMode()
	AnswerBatch(&written, &ack, lines, answers)

	want := "error: cannot read route\nerror\n" +
		"{\"error\": \"unknown command v4 command from an API 6 helper\"}\nerror\n" +
		"{\"error\": \"cannot read route\"}\nerror\n"
	assert.Equal(t, want, written.String())
}

// TestSelectorMatchingNoPeerRefused is X-24 (AC-30).
//
// VALIDATES: a selector naming only addresses outside Translator.Peers, in the
// API 6 `peer` and the API 4 `neighbor` spellings, is refused and carries no
// command, and the batch answers it with the error marker; a selector naming
// one served and one unserved address reaches the served one only.
// PREVENTS: the old `done` for a line that reached nothing (Translation.Unmatched),
// and a selector widened to every peer.
func TestSelectorMatchingNoPeerRefused(t *testing.T) {
	translator := Translator{Families: []string{"ipv4/unicast"}, Peers: []string{"192.0.2.1"}}
	for _, line := range []string{
		"peer 192.0.2.99 announce route 10.0.0.0/24 next-hop 1.1.1.1",
		"neighbor 192.0.2.99 announce route 10.0.0.0/24 next-hop 1.1.1.1",
	} {
		t.Run(line, func(t *testing.T) {
			translation, err := translator.Line(line)
			require.ErrorIs(t, err, errSelectorMatchesNoPeer)
			assert.Empty(t, translation.Commands)

			lines := []BatchLine{{Text: line, Version: CommandAPIVersion(line), Translation: translation, Err: err}}
			var written strings.Builder
			ack := NewAckMode()
			AnswerBatch(&written, &ack, lines, make([]BatchAnswer, len(lines)))
			assert.True(t, strings.HasSuffix(written.String(), "error\n"), written.String())
			assert.Contains(t, written.String(), "the selector matches no peer")
		})
	}

	translation, err := translator.Line("neighbor 192.0.2.1, neighbor 192.0.2.99 announce route 10.0.0.0/24 next-hop 1.1.1.1")
	require.NoError(t, err)
	require.Len(t, translation.Commands, 1)
	assert.Equal(t, "send bgp 192.0.2.1 update text nhop 1.1.1.1 nlri add 10.0.0.0/24", translation.Commands[0].Text)
}
