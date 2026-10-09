// Design: docs/architecture/core-design.md -- the announce rail groups peers on announceFacts
// Related: test/plugin/local-as-replace-as-announce-partition.ci -- the frames this
// announce must produce for two eBGP peers that differ only in replace-as.
// Related: internal/component/bgp/reactor/reactor_api_batch.go -- announceFacts, the
// group key that is also the builder's argument set.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/local-as-replace-as-announce-partition", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "local-as-announce", sdk.Registration{}, ObserverScenario(localASAnnouncePartition))
	})
}

// localASAnnouncePartition announces one prefix to every established peer, after
// both sessions are up and quiet.
//
// One command, two destinations sharing one local AS. The .ci beside this file
// expects the two-AS prepend on the peer without replace-as and the local AS
// alone on the peer with it, so a group key that merged the two peers sends one
// of them the other's AS_PATH and fails it.
func localASAnnouncePartition(ctx context.Context, plugin *sdk.Plugin) error {
	if !plugin01WaitPeers(ctx, plugin, 2, 80) {
		return errors.New("both eBGP sessions never established")
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}

	// No AS_PATH in the attribute block: the rail synthesizes the prepend, and
	// the prepend is what the .ci compares.
	//
	//   40010100   ORIGIN, IGP
	//   nhop       10.0.0.1
	//   nlri       192.0.2.0/24
	const announce = "send bgp * update hex " +
		"attr set 40010100 nhop set 0A000001 nlri ipv4/unicast add 18C00002"
	if _, err := plugin01RequireDone(ctx, plugin, announce); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: announce dispatched to both local-as peers")
	return nil
}
