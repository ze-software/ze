// Design: plan/spec-fixit-peer-pending-sync-settles-too-early.md -- the initial-sync End-of-RIB barrier
// Related: plugin_fixture_09.go -- announceWithdraw09, the announce/withdraw observers for the same peer shape
// Related: plugin_fixture_01.go -- plugin01PeerCounter, plugin01RequireDone, the dispatch helpers used here
//
// The fixture behind test/plugin/initial-sync-barrier-raw.ci. Registration lives
// in this file rather than beside the other bgp scenarios so the change touches
// no file another session is editing.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rawUpdateWire is one complete BGP UPDATE announcing 10.0.1.0/24 with ORIGIN
// IGP, an empty AS_PATH, NEXT_HOP 10.0.1.254 and LOCAL_PREF 200. Marker and
// header included, because `peer <addr> raw hex <data>` with no type word
// carries a whole packet the caller built
// (docs/architecture/api/update-syntax.md).
//
// Byte-identical to the frame ze builds for the same route in
// test/plugin/mup-ipv4-announce.ci, so the .ci expectation reads as an ordinary
// announce rather than as a shape only this fixture can produce.
const rawUpdateWire = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" +
	"00300200000015400101004002004003040A0001FE400504000000C8180A0001"

func init() {
	Register("plugin/initial-sync-barrier-raw", initialSyncBarrierRaw)
}

// initialSyncBarrierRaw pushes one route through the RAW rail while the peer's
// initial-sync End-of-RIB is still owed, then reports the session ready.
//
// The order it produces on the wire is the whole assertion: the injected route,
// then the marker. RFC 4724 Section 4 owes the marker once the initial routing
// update completes, and the owner ruled on 2026-08-30 that a plugin-injected
// route belongs to that update, so the marker MUST wait for this process. The
// binding in the .ci grants `send [ raw ]` and nothing else, so the only reason
// the daemon counts this process at all is the raw rail.
//
// It waits for ESTABLISHMENT rather than for eor-sent, which is the opposite of
// what the announce fixtures beside it do. Those take the marker as their
// go-ahead, so they can never observe a marker that failed to wait; this one
// races the marker on purpose, and the barrier is what gives that race one
// outcome.
//
// The poll is deliberately tight. The barrier releases at the sync timeout
// whether or not this process reports ready, so a slow poll would let the
// timeout rather than the barrier decide the order, and the test would then pass
// against a daemon that never waited.
func initialSyncBarrierRaw(ctx context.Context, _ []string) error {
	return Observe(ctx, "raw-injector", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		established := Poll(ctx, 400, 5*time.Millisecond, func() bool {
			return plugin01PeerCounter(ctx, plugin, "*", "connections-established") >= 1
		})
		if !established {
			return errors.New("no peer reached established, so nothing was ever injected")
		}

		if _, err := plugin01RequireDone(ctx, plugin, "peer 127.0.0.1 raw hex "+rawUpdateWire); err != nil {
			return err
		}

		// The signal the barrier waits for. Without it the peer's marker leaves
		// at the sync timeout instead, which is late rather than wrong, and
		// which would leave this fixture proving nothing about the barrier.
		if _, err := plugin01RequireDone(ctx, plugin, "request peer 127.0.0.1 plugin session ready"); err != nil {
			return err
		}

		if !plugin01WaitCounter(ctx, plugin, "*", "eor-sent", 1, 100) {
			return errors.New("the peer never reported eor-sent, so the marker never reached the wire")
		}
		fmt.Fprintln(os.Stderr, "OK: the raw route reached the wire before the end-of-rib")
		return nil
	})
}
