// Design: docs/architecture/bfd.md -- the BFD surface the test/bfd suite drives
// Overview: bfd_fixture.go -- the scenarios this file names

package fixture

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("bfd/bfd-detection-interval",
		bfdObserver("bfd-detection-interval-test", bfdDetectionIntervalScenario))
}

// bfdObserver builds the driver a test/bfd `.ci` names in its
// `plugin { external <name> }` block. The name MUST be that block's name: it is
// what the daemon calls the plugin back on.
func bfdObserver(name string, scenario ObserverScenario) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, sdk.Registration{}, scenario)
	}
}
