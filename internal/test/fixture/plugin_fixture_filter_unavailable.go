// Design: docs/plugin-development/handlers.md -- route filters, "unavailable" on a stopped plugin
// Related: register_filter_unavailable.go -- the registration

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// filterUnavailableRoutesWant is how many routes the RIB holds once both
// peers are done: the one the live fakefilter accepted. The route the second
// peer sends after the plugin stopped meets the slot's "unavailable" and the
// member's on-error reject, so it is not installed.
const filterUnavailableRoutesWant = 1

// filterWirePluginUnavailable waits for the second peer's End-of-RIB, which a
// session counts only after the import verdict of every UPDATE before it, so
// the route sent after the stop has been judged when the RIB is read. It then
// checks the RIB holds exactly the route judged before the stop.
func filterWirePluginUnavailable(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("filter-wire-plugin-unavailable takes no arguments")
	}
	return Observe(ctx, "filter-unavailable-observer", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		eorReceived := func(value any) bool {
			switch v := value.(type) {
			case bool:
				return v
			case float64:
				return v >= 1
			}
			return false
		}
		if !peerDetailReady(ctx, p, 60, "eor-received", eorReceived) {
			return errors.New("no peer's End-of-RIB was received: the route sent after the stop was never judged")
		}
		var count int
		var readable bool
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			count, readable = routeCount(ctx, p, "show bgp rib received count")
			return readable
		}) {
			return errors.New("show bgp rib received count never answered")
		}
		if count != filterUnavailableRoutesWant {
			return fmt.Errorf("the RIB holds %d routes, want %d: the route judged while fakefilter was stopped did not take on-error reject",
				count, filterUnavailableRoutesWant)
		}
		os.Stderr.WriteString("OK: the RIB holds only the route the live filter accepted\n") //nolint:errcheck // stderr write; the runner reads it
		return nil
	})
}
