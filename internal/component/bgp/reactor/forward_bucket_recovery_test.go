// Design: docs/architecture/bgp/structural-forwarding.md -- outbound attribute bucket grouping
// Related: forward_bucket.go -- recoveryBucketMerge
// Related: relay_recovery.go -- the admission whose sections it packs
package reactor

import (
	"bytes"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// recoveryPackBodyOf builds an UPDATE body with no withdrawn routes, attrs and
// one NLRI field.
// RFC 4271 Section 4.3:
//
//	Byte offset: | 0..1: Withdrawn Routes Length | 2..3: Total Path Attribute Length | 4..: Path Attributes | NLRI |
func recoveryPackBodyOf(attrs, nlri []byte) []byte {
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	return append(body, nlri...)
}

// TestRecoveryBucketMergePacksOneSourceMessage proves that the sections a
// recovery admission writes are packed: adjacent announcements of one
// candidate source's same received message with identical attributes share
// one UPDATE, a key admit did not decide to write is left out, and every other
// section stays alone, mapped back to the item it came from. fwdBucketMerge
// never packs these owned items; the recovery pack has its own join rule.
//
// VALIDATES: recoveryBucketMerge packs only sections that say the same thing,
// within the message size, and staged names the keys each written section
// carries.
// PREVENTS: one UPDATE per owed key, which turns a 900-prefix election into
// 900 messages, and a pack that moves a route onto another source's or
// another revision's attributes.
func TestRecoveryBucketMergePacksOneSourceMessage(t *testing.T) {
	source, other := new(Peer), new(Peer)
	attrs := []byte{0x40, 0x01, 0x01, 0x00}                                // RFC 4271 Section 4.3: ORIGIN IGP.
	attrs2 := []byte{0x40, 0x01, 0x01, 0x02, 0x80, 0x04, 0x04, 0, 0, 0, 1} // ORIGIN INCOMPLETE, MULTI_EXIT_DISC 1.
	p1, p2, p3 := []byte{24, 10, 0, 1}, []byte{24, 10, 0, 2}, []byte{24, 10, 0, 3}
	p4, p5, p6 := []byte{24, 10, 0, 4}, []byte{24, 10, 0, 5}, []byte{24, 10, 0, 6}
	announce := func(peer *Peer, id uint64, a, nlri []byte) fwdItem {
		return fwdItem{authority: adjOutRecovery, receivedPeer: peer, receivedGeneration: 1,
			sourceMessageID: id, rawBodies: [][]byte{recoveryPackBodyOf(a, nlri)}}
	}
	reencoded := announce(source, 7, attrs, p2)
	reencoded.rawBodies = nil
	reencoded.updates = []*message.Update{{PathAttributes: attrs, NLRI: p2}}
	withdrawal := fwdItem{authority: adjOutRecovery, rawBodies: [][]byte{{0, 4, 24, 10, 0, 9, 0, 0}}}
	items := []fwdItem{
		announce(source, 7, attrs, p1), // 0: opens the pack.
		reencoded,                      // 1: joins, as a re-encoded update.
		announce(source, 7, attrs, p6), // 2: held, never written.
		announce(source, 7, attrs, p3), // 3: joins across the held key.
		announce(source, 8, attrs, p4), // 4: another received message.
		announce(other, 8, attrs, p5),  // 5: another source.
		withdrawal,                     // 6: a withdrawal stays alone.
		announce(other, 8, attrs, p6),  // 7: no join across the withdrawal.
		announce(other, 8, attrs2, p1), // 8: other attributes.
		announce(other, 8, attrs2, p2), // 9: joins 8.
		announce(other, 8, attrs2, p3), // 10: over the size limit, opens a new pack.
		announce(other, 8, attrs2, p4), // 11: joins 10.
	}
	admission := &recoveryAdmission{items: items}
	for k := range items {
		outcome := recoveryKeyWrite
		if k == 2 {
			outcome = recoveryKeyHeld
		}
		admission.keys = append(admission.keys, recoveryAdmitKey{outcome: outcome})
		admission.itemKeys = append(admission.itemKeys, k)
	}

	// Room for attrs2 and two NLRI tuples of 4 octets each, or attrs and
	// three.
	maxBodySize := 4 + len(attrs2) + 2*4
	sections, staged := recoveryBucketMerge(admission, maxBodySize)

	wantStaged := [][]int{{0, 1, 3}, {4}, {5}, {6}, {7}, {8, 9}, {10, 11}}
	if !slices.EqualFunc(staged, wantStaged, slices.Equal[[]int]) {
		t.Fatalf("staged = %v, want %v", staged, wantStaged)
	}
	wantBodies := [][]byte{
		recoveryPackBodyOf(attrs, slices.Concat(p1, p2, p3)),
		items[4].rawBodies[0],
		items[5].rawBodies[0],
		items[6].rawBodies[0],
		items[7].rawBodies[0],
		recoveryPackBodyOf(attrs2, slices.Concat(p1, p2)),
		recoveryPackBodyOf(attrs2, slices.Concat(p3, p4)),
	}
	for k := range sections {
		section := &sections[k]
		if len(section.updates) != 0 {
			t.Errorf("section %d keeps %d parsed updates beside its body", k, len(section.updates))
		}
		if len(section.rawBodies) != 1 {
			t.Fatalf("section %d has %d bodies, want 1", k, len(section.rawBodies))
		}
		if !bytes.Equal(section.rawBodies[0], wantBodies[k]) {
			t.Errorf("section %d body = %x, want %x", k, section.rawBodies[0], wantBodies[k])
		}
		first := &items[staged[k][0]]
		if section.receivedPeer != first.receivedPeer || section.sourceMessageID != first.sourceMessageID ||
			section.authority != adjOutRecovery {
			t.Errorf("section %d does not carry its first item's owner and authority", k)
		}
	}
	// A packed body is a copy: the staged item the admission releases still
	// holds its own bytes.
	if !bytes.Equal(items[0].rawBodies[0], recoveryPackBodyOf(attrs, p1)) {
		t.Errorf("the pack rewrote staged item 0's body: %x", items[0].rawBodies[0])
	}
}

// TestRecoveryPackReowesEveryKeyWhenSourceEnds proves that a packed recovery
// section whose source session ends between admit and the write re-owes
// EVERY key the pack carries, not only its first. Method: one admission stages
// a withdrawal (no source) ahead of two announcements of one source with
// identical attributes, which recoveryBucketMerge packs into one section. The
// destination's final egress check, which runs on each section's write,
// replaces the source's session when the withdrawal is written, so the packed
// section meets its fence after admit decided to write it.
//
// VALIDATES: fwdBatchHandler re-owes each staged key of a fenced section with
// the source excluded, and writes nothing of it.
// PREVENTS: a pack whose first key is re-owed while the others are settled as
// written, so the destination loses routes it was never sent.
func TestRecoveryPackReowesEveryKeyWhenSourceEnds(t *testing.T) {
	r, source, destination, conn, _ := newSyncOrderRail(t)
	destination.sendingInitialRoutes.Store(0)
	session := destination.currentSession()
	// The table already belongs to this session, as after its first write:
	// beginAdjOut would otherwise bind it on the withdrawal's write and clear
	// the owed set with it.
	destination.adjOut.mu.Lock()
	destination.adjOut.session = session
	destination.adjOut.mu.Unlock()
	attrs := syncOrderAnnounceBody[4:24] // ORIGIN, AS_PATH, NEXT_HOP.
	withdrawn := []byte{24, 198, 51, 100}
	prefixes := [][]byte{{24, 192, 0, 2}, {24, 192, 0, 3}}
	generation := source.forwardGeneration.Load()
	admission := &recoveryAdmission{peer: destination, session: session, ctx: t.Context(),
		done:    make(chan struct{}),
		sources: map[*Peer]recoverySourceReceipt{source: {generation: generation, cut: source.receiveCut()}}}
	key, record := recoveryOweForTest(t, destination, session, withdrawn)
	admission.keys = append(admission.keys, recoveryAdmitKey{key: key, record: record, epoch: record.epoch})
	// RFC 4271 Section 4.3: withdrawn routes length 4, the prefix, no attributes.
	admission.items = append(admission.items, fwdItem{peer: destination, session: session, authority: adjOutRecovery,
		rawBodies: [][]byte{slices.Concat([]byte{0, 4}, withdrawn, []byte{0, 0})}})
	admission.mark(0)
	for _, prefix := range prefixes {
		key, record := recoveryOweForTest(t, destination, session, prefix)
		admission.keys = append(admission.keys, recoveryAdmitKey{key: key, record: record, epoch: record.epoch,
			candidate: adjOutPath{source: source}})
		admission.items = append(admission.items, fwdItem{peer: destination, session: session, authority: adjOutRecovery,
			rawBodies:    [][]byte{recoveryPackBodyOf(attrs, prefix)},
			receivedPeer: source, receivedGeneration: generation})
		admission.mark(len(admission.keys) - 1)
	}
	// The source's session is replaced while the withdrawal is written.
	checks := 0
	session.egressFinalAccepts = func() bool {
		checks++
		source.advanceForwardGeneration()
		return true
	}

	r.fwdPool.safeBatchHandle(fwdKey{peerAddr: destination.Settings().PeerKey()},
		[]fwdItem{{peer: destination, recovery: admission}})
	<-admission.done

	if checks == 0 {
		t.Fatal("the withdrawal section never reached the final egress check, so the source never ended")
	}
	for _, update := range recoveryWrittenUpdates(t, conn.written()) {
		if len(update.NLRI) != 0 {
			t.Errorf("the fenced pack was written: NLRI %x", update.NLRI)
		}
	}
	if got := admission.keys[0].outcome; got != recoveryKeyWrite {
		t.Errorf("withdrawal key outcome = %d, want recoveryKeyWrite", got)
	}
	for k := 1; k < len(admission.keys); k++ {
		if got := admission.keys[k].outcome; got != recoveryKeyReowed {
			t.Errorf("packed key %d outcome = %d, want recoveryKeyReowed", k, got)
		}
		// settle applied the re-owe to the record the round read; the woken
		// recovery worker may already have decided it since.
		table := &destination.adjOut
		table.mu.Lock()
		excluded := exclusionSources(admission.keys[k].record.excluded)
		table.mu.Unlock()
		if !slices.Contains(excluded, source.Settings().Address) {
			t.Errorf("packed key %d is owed again with excluded %v, want the ended source excluded", k, excluded)
		}
	}
}

// TestRecoveryPackLiveDepartureOneUpdatePerFit proves the recovery pack
// through the real entry point: bgp-rs and bgp-rib on a live router, and
// sessions over the remote socket. A owns 300 prefixes at client C; B
// announced the same 300 in one UPDATE with a longer AS_PATH, so C still
// holds A's route. A's session ends with no NOTIFICATION: bgp-rs runs
// departure recovery in one batch (withdrawalBatchSize is 500), bgp-rib
// elects B for every key, and the recovery worker writes B's routes.
//
// Every elected route comes from B's one received message with identical
// attributes, so C MUST receive them in ceil(300/fit) UPDATEs, where fit is
// how many 4-octet /24 tuples share one UPDATE body with those attributes
// under the 4096-octet maximum. fit exceeds 300 here, so that is one UPDATE.
//
// VALIDATES: recovery packs the owed keys of one source message, end to end.
// PREVENTS: an election over one received UPDATE reaching the client as one
// UPDATE per prefix.
func TestRecoveryPackLiveDepartureOneUpdatePerFit(t *testing.T) {
	const keys = 300
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	prefixes := make([]netip.Prefix, keys)
	for i := range prefixes {
		prefixes[i] = netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 40 + byte(i/256), byte(i % 256), 0}), 24)
	}
	ownershipLiveSend(t, a, nil, attrsA, prefixes...) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefixes[keys-1], attrsA)
	ownershipLiveSend(t, b, nil, attrsB, prefixes...) // RFC 4271 Section 4.3.
	lowEventually(t, func() bool {
		return adjOutOwnedBy(c.peer, a.peer) == keys && adjOutOwedCount(c.peer) == 0
	}, "A's shorter AS_PATH stays C's route")

	c.mu.Lock()
	setup := len(c.frames)
	c.mu.Unlock()
	_ = a.remote.Close() // A's session ends with no NOTIFICATION: a departure.
	lowEventually(t, func() bool {
		return adjOutOwnedBy(c.peer, b.peer) == keys && adjOutOwedCount(c.peer) == 0
	}, "B's routes replace A's at C")

	// RFC 4271 Section 4: "The maximum message size is 4096 octets." The
	// body is that less the 19-octet header; 4 octets are the two length
	// fields around the attributes, and each /24 tuple is 4 octets.
	fit := (4096 - message.HeaderLen - 4 - len(attrsB)) / 4
	want := (keys + fit - 1) / fit // Rounds up: a partial UPDATE still counts.
	updates, carried := 0, 0
	c.mu.Lock()
	frames := slices.Clone(c.frames[setup:])
	c.mu.Unlock()
	for _, frame := range frames {
		if frame[18] != byte(msgtype.TypeUPDATE) {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:]) // RFC 4271 Section 4.3.
		if err != nil {
			t.Fatalf("decode C's UPDATE: %v", err)
		}
		if len(update.NLRI) == 0 || !bytes.Equal(update.PathAttributes, attrsB) {
			continue
		}
		updates++
		carried += len(ownershipLivePrefixes(t, update.NLRI))
	}
	if carried != keys {
		t.Errorf("C received %d of B's %d elected prefixes", carried, keys)
	}
	if updates != want {
		t.Errorf("C received B's %d elected prefixes in %d UPDATEs, want %d (fit %d per UPDATE)",
			keys, updates, want, fit)
	}
}
