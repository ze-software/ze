// Design: docs/plugin-development/handlers.md -- route filters, auto-inserted members
// Related: register_stage_filter.go -- the registration
// Related: plugin_fixture_filter_thirdparty.go -- everyPeerSentEndOfRIB, ribReceivedText

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// stageFilterInserted is the observer of the stage-filter tests
// (spec-filter-wire-6-stage-filters AC-1). Its arguments are the number of
// peers that send routes, then every prefix the RIB must hold once they are
// done. It waits for every peer's End-of-RIB, which a session counts only
// after the import verdict of every UPDATE before it, then checks the RIB
// holds exactly those prefixes: a route the inserted member rejects is
// absent, and a route of a peer the member was not inserted on is present.
func stageFilterInserted(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("stage-filter-inserted takes <peers> <prefix>...: the peer count, then every prefix the RIB must hold")
	}
	peersWant, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("stage-filter-inserted: peer count %q is not a number: %w", args[0], err)
	}
	prefixesWant := args[1:]
	return Observe(ctx, "stage-filter-observer", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return everyPeerSentEndOfRIB(ctx, p, peersWant) }) {
			return fmt.Errorf("not every one of the %d peers' End-of-RIB was received: their routes were never all judged", peersWant)
		}
		var count int
		var readable bool
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			count, readable = routeCount(ctx, p, "show bgp rib received count")
			return readable
		}) {
			return errors.New("show bgp rib received count never answered")
		}
		rib, err := ribReceivedText(ctx, p)
		if err != nil {
			return err
		}
		if count != len(prefixesWant) {
			return fmt.Errorf("the RIB holds %d routes, want %d (%s): an inserted member judged a peer it was not inserted on, or missed one it was: %.600s",
				count, len(prefixesWant), strings.Join(prefixesWant, " "), rib)
		}
		for _, prefix := range prefixesWant {
			if !strings.Contains(rib, `"`+prefix+`"`) {
				return fmt.Errorf("the RIB lacks %s: %.600s", prefix, rib)
			}
		}
		os.Stderr.WriteString("OK: the RIB holds exactly " + strings.Join(prefixesWant, " ") + "\n") //nolint:errcheck // stderr write; the runner reads it
		return nil
	})
}
