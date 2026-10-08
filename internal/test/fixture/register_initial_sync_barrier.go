// Design: docs/architecture/api/architecture.md -- the initial-sync End-of-RIB barrier
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

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rawUpdateWire is one complete BGP UPDATE announcing 10.0.1.0/24 with ORIGIN
// IGP, an empty AS_PATH, NEXT_HOP 10.0.1.254 and LOCAL_PREF 200. Marker and
// header included, because `send bgp <addr> raw hex <data>` with no type word
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

// initialSyncBarrierRaw starts a raw injection as soon as the peer establishes,
// reports session ready, and waits for the independent initial-sync End-of-RIB.
//
// RFC 4724 Section 4: "The End-of-RIB marker MUST be sent by a BGP speaker to its
// peer once it completes the initial routing update (including the case when
// there is no update to send) for an address family after the BGP session is
// established." The API Sync Protocol in docs/architecture/api/architecture.md
// defines that update as ze's own table; sendInitialRoutes does not wait for this
// process's ready signal.
//
// The observer starts on establishment rather than eor-sent so injection can
// overlap initial sync. Raw messages bypass opQueue and share the session's
// writeMu with the marker. The .ci MUST assert both frames in one unordered
// sequence and MUST keep the peer alive until this observer has read eor-sent.
// This observer MUST retain that counter barrier before it requests shutdown.
func initialSyncBarrierRaw(ctx context.Context, _ []string) error {
	// Keep the reporter declaration and ready signal paired. This exercises
	// session-ready bookkeeping without treating it as a marker-order barrier.
	reg := sdk.Registration{SignalsSessionReady: true}
	return fixtureObserveInitialReplay(ctx, "raw-injector", reg, func(ctx context.Context, plugin *sdk.Plugin, initialReplay uint64) error {
		if _, err := plugin01RequireDone(ctx, plugin, "send bgp 127.0.0.1 raw hex "+rawUpdateWire); err != nil {
			return err
		}

		// Finish the declared reporter's work. This signal does not release or
		// order sendInitialRoutes' independently emitted marker.
		if err := fixture10SessionReady(ctx, plugin, initialReplay); err != nil {
			return err
		}

		if !plugin01WaitCounter(ctx, plugin, "*", "eor-sent", 1, 100) {
			return errors.New("the peer never reported eor-sent, so the marker never reached the wire")
		}
		// The counter proves a successful marker write, not its order against
		// the injection. The .ci separately requires both complete wire frames.
		fmt.Fprintln(os.Stderr, "OK: the raw route was injected and the end-of-rib was counted")
		return nil
	})
}
