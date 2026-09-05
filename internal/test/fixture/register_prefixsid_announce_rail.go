// Design: docs/architecture/testing/interop.md -- a functional fixture drives the
// product's own dispatcher, so the .ci beside it asserts what an operator gets.
// Related: test/plugin/prefixsid-announce-rail-boundary.ci -- the frames this
// announce must produce for two eBGP peers that differ in one leaf.
// Related: internal/component/bgp/reactor/reactor_api_batch.go -- the announce
// rail the command reaches, buildBatchAnnounceUpdate.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/prefixsid-announce-rail-boundary", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "prefixsid-announce", sdk.Registration{}, ObserverScenario(prefixSIDAnnounceRail))
	})
}

// prefixSIDAnnounceRail announces one prefix carrying a BGP Prefix-SID attribute
// to every established peer, after both sessions are up and quiet.
//
// The attribute is written as raw wire bytes rather than through a typed leaf,
// because RFC 8669 Section 8 governs the ATTRIBUTE and not the route field that
// carried it: a code 40 hand-written by an operator crosses the AS boundary
// under the same condition as a configured one.
//
// One command, two destinations. The .ci beside this file expects the attribute
// on the peer the operator placed inside the SR domain and no attribute 40 at
// all on the other, so a rail that answered the same for both fails it.
func prefixSIDAnnounceRail(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitPeers(ctx, plugin, 2, 80) {
		return errors.New("both eBGP sessions never established")
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}

	// The wire form, because the text form has no token for a raw attribute:
	// `send bgp <selector> update text` accepts origin, med, local-preference,
	// as-path, the three community kinds, next-hop, path-information, rd, label,
	// nlri and watchdog, and none of them writes attribute 40. `update hex` takes
	// the attribute block itself, which is the same base the LLGR readvertise
	// replays onto this rail.
	//
	//   40010100                    ORIGIN, IGP
	//   C0280A 01000700000000000064 flags 0xC0 optional transitive, code 40
	//                               (0x28), length 10, and a Label-Index TLV
	//                               (RFC 8669 Section 3.1: type 1, length 7,
	//                               RESERVED 0, Flags 0, Label Index 100)
	//   nhop 0A000001               10.0.0.1
	//   nlri 18C00002               192.0.2.0/24
	//
	// No AS_PATH: the rail synthesizes one toward an external peer, and supplying
	// one would put the prepend in the frames the .ci compares.
	const announce = "send bgp * update hex " +
		"attr set 40010100C0280A01000700000000000064 " +
		"nhop set 0A000001 nlri ipv4/unicast add 18C00002"
	if _, err := plugin01RequireDone(ctx, plugin, announce); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: announce carrying attribute 40 dispatched to every peer")
	return nil
}
