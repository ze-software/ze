// Design: docs/architecture/bgp/structural-forwarding.md -- source policy epoch at the final writer.
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
// Related: rfc8212_sent_owner_epoch_test.go -- the sent-owner fixture and update-route announce rail.
package reactor

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestRFC8212AbortRestore drives an aborted import invalidation through the
// final writer as bgp-rib drives it (docs/architecture/plugin/rib-storage-design.md,
// "Source-policy lifecycle owners"): the source's forwarded route reaches the
// destination under epoch E0; the invalidation moves the source to E1 and the
// owner withdraws its forwarded copy through update-route with the copy's sent
// receipt; the abort moves the source to E2 and the owner re-announces the copy
// with the same receipt, replay meta and E2. The restore is written and the
// destination's Adj-RIB-Out owns the route again under E2. A restore stamped
// with a superseded epoch, for a replaced source Peer, or after the source
// withdrew the route itself, is refused. A later marker settles each refusal.
func TestRFC8212AbortRestore(t *testing.T) {
	t.Run("live-epoch-restored", func(t *testing.T) {
		f, restore, _, e2 := rfc8212AbortFixture(t)
		restore.SourcePolicyEpoch = e2
		rfc8212SentOwnerAnnounce(t, f, restore)
		rfc8212EpochMarker(t, f, "direct", 92)
		rfc8212AbortHistory(t, f, "1 - 1")
		owner, epoch, revision := rfc8212AbortEntry(t, f)
		if owner != f.source {
			t.Fatalf("restored entry owner %p, want the source %p", owner, f.source)
		}
		if epoch != e2 {
			t.Fatalf("restored entry owned under epoch %d, want the abort's %d", epoch, e2)
		}
		if revision != restore.SentOwnerMessage {
			t.Fatalf("restored entry revision %d, want the restore's sent receipt %d", revision, restore.SentOwnerMessage)
		}
		// The owner's later exact removal still names the restored entry.
		restore.SourcePolicyEpoch = 0
		rfc8212AbortWithdraw(t, f, restore)
		rfc8212EpochMarker(t, f, "direct", 93)
		rfc8212AbortHistory(t, f, "1 - 1 -")
	})
	t.Run("superseded-epochs-refused", func(t *testing.T) {
		f, restore, e1, e2 := rfc8212AbortFixture(t)
		for i, stale := range []uint64{restore.SourcePolicyEpoch, e1} {
			restore.SourcePolicyEpoch = stale
			rfc8212SentOwnerAnnounce(t, f, restore)
			rfc8212EpochMarker(t, f, "direct", byte(92+i))
			f.assertHistory(t, []byte{1}, 1)
		}
		// Positive control: the same receipt under the live epoch is written.
		restore.SourcePolicyEpoch = e2
		rfc8212SentOwnerAnnounce(t, f, restore)
		rfc8212EpochMarker(t, f, "direct", 94)
		rfc8212AbortHistory(t, f, "1 - 1")
	})
	t.Run("replaced-source-refused", func(t *testing.T) {
		f, restore, _, e2 := rfc8212AbortFixture(t)
		rfc8212SentOwnerReplaceSource(f)
		restore.SourcePolicyEpoch = e2
		rfc8212SentOwnerAnnounce(t, f, restore)
		restore.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
		rfc8212SentOwnerAnnounce(t, f, restore)
		rfc8212EpochMarker(t, f, "direct", 92)
		f.assertHistory(t, []byte{1}, 1)
	})
	t.Run("source-withdrawal-refused", func(t *testing.T) {
		f, restore, _, e2 := rfc8212AbortFixture(t)
		// RFC 4271 Section 3.1: the source withdraws the route itself, so
		// no restore may bring it back.
		gone := rfc8212EpochAdmit(t, f, ownershipRailBody(family.IPv4Unicast, true, syncOrderPrefixWire, 0))
		rfc8212EpochDispatch(t, f, "direct", gone)
		restore.SourcePolicyEpoch = e2
		rfc8212SentOwnerAnnounce(t, f, restore)
		rfc8212EpochMarker(t, f, "direct", 92)
		f.assertHistory(t, []byte{1}, 1)
	})
}

// rfc8212AbortFixture forwards the source's route under E0, applies the
// invalidation (E1, the owner's withdrawal of the forwarded copy), and aborts
// it (E2). It returns the owner's restore batch, still carrying E0, and E1, E2.
func rfc8212AbortFixture(t *testing.T) (*ownershipRailFixture, bgptypes.NLRIBatch, uint64, uint64) {
	t.Helper()
	f, first, restore := rfc8212SentOwnerFixture(t)
	restore.SourcePolicyEpoch = first.SourcePolicyEpoch
	// The coordinator's apply and abort steps (sourceInvalidation.apply, abort).
	e1 := f.source.openSourcePolicyWindow()
	// bgp-rib's apply-time removal carries the receipt and no epoch.
	withdrawal := restore
	withdrawal.SourcePolicyEpoch = 0
	rfc8212AbortWithdraw(t, f, withdrawal)
	rfc8212EpochMarker(t, f, "direct", 90)
	f.assertHistory(t, []byte{1}, 1)
	e2 := f.source.abortSourcePolicyWindow(e1)
	if e1 == first.SourcePolicyEpoch || e2 == e1 {
		t.Fatalf("epochs did not advance: E0 %d, E1 %d, E2 %d", first.SourcePolicyEpoch, e1, e2)
	}
	return f, restore, e1, e2
}

// rfc8212AbortWithdraw is the update-route rail's withdraw call.
func rfc8212AbortWithdraw(t *testing.T, f *ownershipRailFixture, batch bgptypes.NLRIBatch) {
	t.Helper()
	api := &reactorAPIAdapter{r: f.r}
	sel := selector.ParseDefault(f.destination.Settings().Address.String())
	if err := api.WithdrawNLRIBatch(t.Context(), sel, batch, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
}

// rfc8212AbortEntry reads the destination's Adj-RIB-Out entry for the prefix.
func rfc8212AbortEntry(t *testing.T, f *ownershipRailFixture) (*Peer, uint64, uint64) {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var path fwdPathKey
	if err := fwdPathKeyFor(&path, family.IPv4Unicast, 0, syncOrderPrefixWire, false, scratch[:]); err != nil {
		t.Fatal(err)
	}
	f.destination.adjOut.mu.Lock()
	defer f.destination.adjOut.mu.Unlock()
	entry := f.destination.adjOut.routes[adjOutKey{path: path}]
	if entry == nil {
		t.Fatal("destination's Adj-RIB-Out holds no entry for the restored route")
	}
	return entry.owner.source, entry.owner.policyEpoch, entry.revision
}

// rfc8212AbortHistory compares the recipient's wire history for the prefix,
// in order: an announcement is its ORIGIN marker, a withdrawal is "-".
// RFC 4271 Section 4.3.
func rfc8212AbortHistory(t *testing.T, f *ownershipRailFixture, want string) {
	t.Helper()
	var got []string
	for _, event := range ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()) {
		if event.family != family.IPv4Unicast || event.key != string(syncOrderPrefixWire) {
			continue
		}
		if event.withdraw {
			got = append(got, "-")
			continue
		}
		got = append(got, strconv.Itoa(int(event.origin)))
	}
	if strings.Join(got, " ") != want {
		t.Fatalf("recipient history for the prefix = %q, want %q", strings.Join(got, " "), want)
	}
}

// TestRFC8212AbortRestoreAfterRecovery is the same abort where the RIB elects
// owner withdrawals, as bgp-rib does when it receives the source's UPDATEs:
// the apply-time exact removal is owed, the recovery round finds no survivor
// for the denied path and withdraws it, and the abort's restore, stamped with
// the source's live epoch, is written and re-owns the entry. One stamped with
// the invalidation's superseded epoch is refused first.
func TestRFC8212AbortRestoreAfterRecovery(t *testing.T) {
	rail := newOwedRail(t)
	p := syncOrderPrefixWire
	rail.forward(t, false, p)
	installed, held := adjOutTestEntry(rail.destination)
	if !held || installed.revision == 0 {
		t.Fatal("the forwarded announcement installed no exact receipt")
	}
	e1 := rail.source.openSourcePolicyWindow()
	// RFC 8212 Section 3: apply withdraws the denied path's forwarded copy.
	owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), installed.revision)
	rail.request(t)
	rail.release <- nil // No survivor: the round withdraws P.
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round decided P")
	if _, held := adjOutTestEntry(rail.destination); held {
		t.Fatal("the recovery round left the denied entry in place")
	}
	before := len(recoveryWrittenUpdates(t, rail.conn.written()))
	e2 := rail.source.abortSourcePolicyWindow(e1)
	restore := ownershipRailBody(family.IPv4Unicast, false, p, 1)
	rfc8212AbortRestoreWrite(t, rail.destination, restore, installed.revision, e1)
	if _, held := adjOutTestEntry(rail.destination); held {
		t.Fatal("a restore stamped with the superseded epoch re-installed the entry")
	}
	rfc8212AbortRestoreWrite(t, rail.destination, restore, installed.revision, e2)
	written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
	if len(written) != 1 || len(written[0].NLRI) == 0 {
		t.Fatalf("restore wrote %d UPDATEs, want the one announcement", len(written))
	}
	restored, held := adjOutTestEntry(rail.destination)
	if !held || restored.owner.source != rail.source || restored.owner.policyEpoch != e2 || restored.revision != installed.revision {
		t.Fatalf("restored entry = held %v source %p epoch %d revision %d, want the source %p under %d with revision %d",
			held, restored.owner.source, restored.owner.policyEpoch, restored.revision, rail.source, e2, installed.revision)
	}
}

// rfc8212AbortRestoreWrite writes body as the owner's abort restore: an
// adjOutExpected item naming the sent revision, with replay meta and the
// request's epoch, at the final writer.
func rfc8212AbortRestoreWrite(t *testing.T, target *Peer, body []byte, revision, epoch uint64) {
	t.Helper()
	s := target.currentSession()
	item := fwdItem{peer: target, session: s, authority: adjOutExpected, expectedMessage: revision,
		sourcePolicyEpoch: epoch, meta: map[string]any{"replay": true}}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.sentForward = &item
	s.sentMeta = item.meta
	defer func() { s.sentForward, s.sentMeta = nil, nil }()
	if err := s.writeRawUpdateBody(body); err != nil {
		t.Fatal(err)
	}
	if err := s.flushWrites(); err != nil {
		t.Fatal(err)
	}
}

// TestRFC8212AbortRestoreAfterSurvivor is the abort where the recovery round
// answered the denied path's removal with a survivor: source A's forwarded
// copy of P is denied, the round elects source B's P, and the destination
// (no ADD-PATH) holds B. The abort's restore of A, under the live epoch, then
// meets B's entry. It is owed rather than dropped, so a new round asks the RIB
// to re-elect between A and B; when the RIB answers A, A is written and owns
// P again. A restore stamped with the superseded epoch owes nothing.
// RFC 8212 Section 3 and RFC 4271 Section 9.1.
func TestRFC8212AbortRestoreAfterSurvivor(t *testing.T) {
	rail := newOwedRail(t)
	other := adjOutTestOtherSource(rail.r, rail.source)
	attachRecoveryOwner(t, rail.r, rail.source, other)
	p := syncOrderPrefixWire
	rail.forward(t, false, p)
	installed, held := adjOutTestEntry(rail.destination)
	if !held || installed.revision == 0 {
		t.Fatal("the forwarded announcement installed no exact receipt")
	}
	e1 := rail.source.openSourcePolicyWindow()
	// RFC 8212 Section 3: apply withdraws A's denied forwarded copy, and the
	// round elects B's path.
	owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), installed.revision)
	rail.request(t)
	rail.release <- owedSurvivorMessage(other.Settings().Address, other.sourcePolicyEpoch.Load(), p, 21)
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round elected B")
	survivor, held := adjOutTestEntry(rail.destination)
	if !held || survivor.owner.source != other {
		t.Fatalf("after the round the destination holds %v from %p, want B %p", held, survivor.owner.source, other)
	}
	e2 := rail.source.abortSourcePolicyWindow(e1)
	restore := ownershipRailBody(family.IPv4Unicast, false, p, 1)
	rfc8212AbortRestoreWrite(t, rail.destination, restore, installed.revision, e1)
	if owed := rail.owed(); len(owed) != 0 {
		t.Fatalf("a restore stamped with the superseded epoch owed %d keys", len(owed))
	}
	before := len(recoveryWrittenUpdates(t, rail.conn.written()))
	rfc8212AbortRestoreWrite(t, rail.destination, restore, installed.revision, e2)
	if written := len(recoveryWrittenUpdates(t, rail.conn.written())); written != before {
		t.Fatalf("the restore wrote %d UPDATEs over B's entry, want none before the election", written-before)
	}
	request := rail.request(t)
	if len(request.Keys) != 1 || !bytes.Equal(request.Keys[0].NLRI, p) {
		t.Fatalf("the re-election asked for %v, want P", request.Keys)
	}
	rail.release <- owedSurvivorMessage(rail.source.Settings().Address, e2, p, installed.revision)
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round re-elected A")
	written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
	if len(written) != 1 || len(written[0].NLRI) == 0 {
		t.Fatalf("the re-election wrote %d UPDATEs, want A's announcement", len(written))
	}
	restored, held := adjOutTestEntry(rail.destination)
	if !held || restored.owner.source != rail.source || restored.owner.policyEpoch != e2 {
		t.Fatalf("re-elected entry = held %v source %p epoch %d, want A %p under %d",
			held, restored.owner.source, restored.owner.policyEpoch, rail.source, e2)
	}
}
