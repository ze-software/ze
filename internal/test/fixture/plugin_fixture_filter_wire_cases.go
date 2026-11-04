// Design: docs/plugin-development/protocol.md -- The Route Fields and Sets of filter-update.
// Related: test/plugin/filter-wire-new-fields.ci -- the two field chains.

package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// End-of-RIB is counted after every earlier UPDATE's import verdict. The
// observer MUST wait for both peers before asking Observe to stop the daemon.
func filterWireCasesFence(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("filter-wire-cases-fence takes no arguments")
	}
	return Observe(ctx, "filter-wire-cases-fence", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 80, 100*time.Millisecond, func() bool { return everyPeerSentEndOfRIB(ctx, p, 2) }) {
			return fmt.Errorf("both peers' post-filter End-of-RIB fences were not observed")
		}
		_, err := os.Stderr.WriteString("OK: both field chains reached End-of-RIB\n")
		return err
	})
}
