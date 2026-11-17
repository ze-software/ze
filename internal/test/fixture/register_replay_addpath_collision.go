// Design: docs/functional-tests.md -- readiness markers between fixture processes.
// Related: plugin_fixture_01_adj_rib.go -- the original ADD-PATH replay scenario.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	replayAddPathCollisionRelease   = "addpath-collision-release"
	replayAddPathCollisionAPDone    = "addpath-collision-ap-done"
	replayAddPathCollisionPlainDone = "addpath-collision-plain-done"
)

func init() {
	Register("plugin/adj-rib-in-replay-addpath-collision", replayAddPathCollisionDriver, StopsDaemon)
}

// replayAddPathCollisionDriver keeps the original observer identity while
// selecting the collision scenario through a separate registry alias.
func replayAddPathCollisionDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("the ADD-PATH collision fixture takes no arguments; run le test fixture plugin/adj-rib-in-replay-addpath-collision")
	}
	return Observe(ctx, "replay-addpath", &sdk.Registration{}, replayAddPathCollision)
}

// replayAddPathCollision completes the original live/replay checks before
// releasing the fourth source. Each destination writes its own done marker
// only after matching the new route, so Observe cannot shut the daemon down
// while either wire assertion is still in flight. All markers are relative
// to the runner's fresh working directory.
func replayAddPathCollision(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01ReplayAddPath(ctx, plugin); err != nil {
		return err
	}
	if err := os.WriteFile(replayAddPathCollisionRelease, []byte("ready\n"), 0o600); err != nil {
		return fmt.Errorf("release the ADD-PATH collision source with %s: %w", replayAddPathCollisionRelease, err)
	}
	if err := waitForMarker02(ctx, replayAddPathCollisionAPDone); err != nil {
		return err
	}
	return waitForMarker02(ctx, replayAddPathCollisionPlainDone)
}
