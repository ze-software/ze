//go:build !race

// Design: docs/architecture/update-building.md -- one final ownership and duplicate boundary
// Related: forward_rs.go -- tryDirectWriteNoFlush, the rs fast path these tests drive
// Related: session_ownership.go -- adjOutWrite.filter and record, the cost under test
// Related: session_ownership_bench_race_test.go -- the same test name under -race, skipped

package reactor

import (
	"bufio"
	"net/netip"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// forwardedWriteChunkKeys is the number of Adj-RIB-Out entries one allocation
// of the entry store may hold. The allocation guard lets a write of N fresh
// keys allocate at most one chunk per forwardedWriteChunkKeys keys beyond the
// write's fixed cost. The entry store MUST allocate in chunks at least this
// large, so this value bounds that store's chunk size from below.
const forwardedWriteChunkKeys = 256

// forwardedWriteBatchKeys is the `le perf send` auto batch: as many IPv4 /24
// NLRIs as fit one 4096-byte UPDATE beside ORIGIN, AS_PATH and NEXT_HOP.
const forwardedWriteBatchKeys = 1000

// forwardedWriteFixture is one Established route-server destination and the
// EBGP source its UPDATEs are forwarded from. The destination discards its
// bytes, so the wire write costs nothing beyond bufio.
type forwardedWriteFixture struct {
	target *Peer
	source *Peer
	item   fwdItem
	bodies [1][]byte // backs item.rawBodies, so a write allocates no slice header
}

// newForwardedWriteFixture builds the destination as newGroupUpdatesPeer and
// ownershipWriterPeer do (NewPeerSettings, Established FSM, the session bound
// to the peer's own Adj-RIB-Out, an ASN4 send context), and the source with
// makeForwardSourcePeer. It takes testing.TB so the benchmark shares it.
func newForwardedWriteFixture(tb testing.TB) *forwardedWriteFixture {
	tb.Helper()
	ip := netip.MustParseAddr("10.0.0.2")
	settings := NewPeerSettings(ip, 65000, 65001, 0x01020302)
	target := NewPeer(settings)
	target.state.Store(int32(PeerStateEstablished))
	target.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{family.IPv4Unicast: true},
	})
	session := NewSession(settings)
	for _, ev := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
		if err := session.fsm.Event(ev); err != nil {
			tb.Fatal(err)
		}
	}
	conn := &discardConn{}
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.adjOut = &target.adjOut
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	session.sendCtxID = ctxID
	target.session = session
	return &forwardedWriteFixture{target: target, source: makeForwardSourcePeer(tb, ctx, ctxID)}
}

// write forwards body through tryDirectWriteNoFlush with the item
// reactorForwardRSSection builds (source-bound, adjOutForwarded, the source's
// generation and policy epoch), then flushes as the fast path's batch flush
// does. The item and its body slice live in the fixture, so every allocation
// counted is the writer's.
func (f *forwardedWriteFixture) write(body []byte) error {
	f.item = fwdItem{peer: f.target, session: f.target.session, authority: adjOutForwarded}
	f.item.receivedPeer = f.source
	f.item.receivedGeneration = f.source.forwardGeneration.Load()
	f.item.sourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
	f.bodies[0] = body
	f.item.rawBodies = f.bodies[:]
	handled, delivered, dst := tryDirectWriteNoFlush(&f.item)
	if !handled {
		return errAdjOutSession
	}
	if !delivered {
		return errAdjOutProvenance
	}
	dst.writeMu.Lock()
	defer dst.writeMu.Unlock()
	return dst.flushWrites()
}

// heldKeys reports how many keys the destination's Adj-RIB-Out holds.
func (f *forwardedWriteFixture) heldKeys() int {
	f.target.adjOut.mu.Lock()
	defer f.target.adjOut.mu.Unlock()
	return len(f.target.adjOut.routes)
}

// forwardedAnnounceBody announces n distinct IPv4 /24 prefixes 10.h.l.0/24
// under ORIGIN IGP, AS_PATH [65001] and NEXT_HOP 10.0.0.1: the attributes of
// syncOrderAnnounceBody.
// RFC 4271 Section 4.3: "An UPDATE message is used to advertise feasible routes
// that share common path attributes to a peer, or to withdraw multiple
// unfeasible routes from service (see 3.1)".
func forwardedAnnounceBody(n int) []byte {
	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN = IGP
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, // AS_PATH = AS_SEQUENCE [65001]
		0x40, 3, 4, 10, 0, 0, 1, // NEXT_HOP = 10.0.0.1
	}
	body := make([]byte, 0, 4+len(attrs)+4*n)
	body = append(body, 0, 0, 0, byte(len(attrs)))
	body = append(body, attrs...)
	return appendForwardedPrefixes(body, n)
}

// forwardedWithdrawBody withdraws the n prefixes forwardedAnnounceBody announces.
func forwardedWithdrawBody(n int) []byte {
	length := 4 * n
	body := make([]byte, 0, 4+length)
	body = append(body, byte(length>>8), byte(length))
	body = appendForwardedPrefixes(body, n)
	return append(body, 0, 0)
}

func appendForwardedPrefixes(body []byte, n int) []byte {
	for i := range n {
		body = append(body, 24, 10, byte(i>>8), byte(i))
	}
	return body
}

// TestAdjOutForwardedWriteAllocations guards the per-NLRI cost of the final
// writer's ownership bookkeeping on the rs fast path.
//
// Method: a source-bound forwarded UPDATE of N fresh keys, then its
// withdrawal, is written through tryDirectWriteNoFlush and writeUpdateBody for
// N = 10 and N = 1,000, after two warm-up cycles so the table's maps already
// hold the capacity a steady stream reuses. The cycle's allocations at 1,000
// keys MUST NOT exceed those at 10 keys by more than one entry-store chunk per
// forwardedWriteChunkKeys keys: a per-key heap object fails it. A same-owner
// re-announcement of 1,000 held keys MUST allocate nothing at all.
func TestAdjOutForwardedWriteAllocations(t *testing.T) {
	cycle := func(t *testing.T, n int) float64 {
		t.Helper()
		f := newForwardedWriteFixture(t)
		announce, withdraw := forwardedAnnounceBody(n), forwardedWithdrawBody(n)
		var cause error
		run := func() {
			if err := f.write(announce); err != nil {
				cause = err
				return
			}
			if got := f.heldKeys(); got != n {
				cause = errAdjOutProvenance
				t.Errorf("announce of %d fresh keys left %d held", n, got)
				return
			}
			if err := f.write(withdraw); err != nil {
				cause = err
				return
			}
			if got := f.heldKeys(); got != 0 {
				cause = errAdjOutProvenance
				t.Errorf("withdrawal of %d keys left %d held", n, got)
			}
		}
		run()
		run()
		allocs := testing.AllocsPerRun(20, run)
		if cause != nil {
			t.Fatalf("forwarded write of %d keys: %v", n, cause)
		}
		return allocs
	}

	small := cycle(t, 10)
	large := cycle(t, forwardedWriteBatchKeys)
	chunks := float64((forwardedWriteBatchKeys + forwardedWriteChunkKeys - 1) / forwardedWriteChunkKeys)
	t.Logf("announce+withdraw allocations per cycle: 10 keys %.1f, %d keys %.1f", small, forwardedWriteBatchKeys, large)
	if large > small+chunks {
		t.Errorf("a %d-key forwarded cycle allocates %.1f, a 10-key one %.1f: growth %.1f exceeds %.0f chunk(s) of %d keys",
			forwardedWriteBatchKeys, large, small, large-small, chunks, forwardedWriteChunkKeys)
	}

	f := newForwardedWriteFixture(t)
	announce := forwardedAnnounceBody(forwardedWriteBatchKeys)
	if err := f.write(announce); err != nil {
		t.Fatal(err)
	}
	var cause error
	reannounce := testing.AllocsPerRun(20, func() {
		if err := f.write(announce); err != nil {
			cause = err
		}
	})
	if cause != nil {
		t.Fatalf("re-announcement: %v", cause)
	}
	if got := f.heldKeys(); got != forwardedWriteBatchKeys {
		t.Fatalf("re-announcement left %d keys held, want %d", got, forwardedWriteBatchKeys)
	}
	t.Logf("same-owner re-announcement of %d held keys: %.1f allocations per write", forwardedWriteBatchKeys, reannounce)
	if reannounce != 0 {
		t.Errorf("a same-owner re-announcement of %d held keys allocates %.1f per write, want 0",
			forwardedWriteBatchKeys, reannounce)
	}
}

// BenchmarkAdjOutForwardedWrite measures one rs fast-path destination write
// through tryDirectWriteNoFlush and writeUpdateBody. "announce-withdraw" is a
// forwarded UPDATE of N fresh keys into a table that holds none of them, then
// its withdrawal, as one op; "reannounce" is the same owner re-announcing N
// held keys. Perf numbers are taken on the owner's Mac only.
func BenchmarkAdjOutForwardedWrite(b *testing.B) {
	for _, n := range []int{10, forwardedWriteBatchKeys} {
		announce, withdraw := forwardedAnnounceBody(n), forwardedWithdrawBody(n)
		b.Run(benchKeysName("announce-withdraw", n), func(b *testing.B) {
			f := newForwardedWriteFixture(b)
			b.ReportAllocs()
			for b.Loop() {
				if err := f.write(announce); err != nil {
					b.Fatal(err)
				}
				if err := f.write(withdraw); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(benchKeysName("reannounce", n), func(b *testing.B) {
			f := newForwardedWriteFixture(b)
			if err := f.write(announce); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := f.write(announce); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchKeysName(kind string, n int) string {
	return kind + "-" + strconv.Itoa(n)
}
