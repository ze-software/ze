// Design: docs/architecture/wire/attributes.md -- RFC 9774 Section 3, the send
// guard that writes a set-carrying UPDATE as its withdrawal.
// Related: test/plugin/rfc9774-as-set-api-not-advertised.ci -- the frames this
// announce must produce for one eBGP peer.
// Related: internal/component/bgp/reactor/session_rfc9774.go -- the guard the
// announce reaches, transmittedPathHoldsSet.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rfc9774-as-set-api-not-advertised", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "rfc9774-api-announce", &sdk.Registration{}, ObserverScenario(rfc9774APIAnnounce))
	})
}

// rfc9774APIAnnounce announces one prefix whose AS_PATH holds an AS_SET, then a
// clean fence prefix, over ze's own command dispatcher once the session is up
// and quiet. The .ci beside this file expects the fence and never the set.
func rfc9774APIAnnounce(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitPeers(ctx, plugin, 1, 80) {
		return errors.New("the eBGP session never established")
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}

	//   40010100                         ORIGIN, IGP
	//   40020A 01020000FDF20000FDF3      AS_PATH, one AS_SET {65010 65011}
	//   nhop 0A000001                    10.0.0.1
	//   nlri 18C00002                    192.0.2.0/24
	const setAnnounce = "send bgp * update hex " +
		"attr set 4001010040020A01020000FDF20000FDF3 " +
		"nhop set 0A000001 nlri ipv4/unicast add 18C00002"
	if _, err := plugin01RequireDone(ctx, plugin, setAnnounce); err != nil {
		return err
	}
	// The fence: a clean path, 198.51.100.0/24. It is written after the set
	// announce on the same session, so its arrival bounds the test.
	const fence = "send bgp * update hex " +
		"attr set 40010100 " +
		"nhop set 0A000001 nlri ipv4/unicast add 18C63364"
	if _, err := plugin01RequireDone(ctx, plugin, fence); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: AS_SET announce and clean fence dispatched")
	return nil
}
