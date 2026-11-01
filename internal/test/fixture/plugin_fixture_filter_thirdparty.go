// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call
// Related: register_filter_thirdparty.go -- the registration

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// filterThirdpartyPeersWant is how many peers send routes: one judged by
	// fakefilter running internal, one by the forked copy.
	filterThirdpartyPeersWant = 2
	// filterThirdpartyRoutesWant is how many routes the RIB holds once both
	// peers are done: each peer sends one route fakefilter accepts and one it
	// rejects.
	filterThirdpartyRoutesWant = 2
)

// filterWireThirdpartyCompiledIn waits for every peer's End-of-RIB, which a
// session counts only after the import verdict of every UPDATE before it, so
// each route has been judged when the RIB is read. It then checks the RIB
// holds exactly the routes fakefilter accepted, one per transport.
func filterWireThirdpartyCompiledIn(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("filter-wire-thirdparty-compiled-in takes no arguments")
	}
	return Observe(ctx, "filter-thirdparty-observer", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 60, 250*time.Millisecond, func() bool { return everyPeerSentEndOfRIB(ctx, p, filterThirdpartyPeersWant) }) {
			return fmt.Errorf("not every one of the %d peers' End-of-RIB was received: their routes were never all judged",
				filterThirdpartyPeersWant)
		}
		var count int
		var readable bool
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			count, readable = routeCount(ctx, p, "show bgp rib received count")
			return readable
		}) {
			return errors.New("show bgp rib received count never answered")
		}
		if count != filterThirdpartyRoutesWant {
			return fmt.Errorf("the RIB holds %d routes, want %d: a fakefilter verdict was not applied on one transport",
				count, filterThirdpartyRoutesWant)
		}
		os.Stderr.WriteString("OK: the RIB holds the 2 routes fakefilter accepted, none it rejected\n") //nolint:errcheck // stderr write; the runner reads it
		return nil
	})
}

// everyPeerSentEndOfRIB answers whether want peers are listed and each has
// received an End-of-RIB. A command that does not answer is not yet ready.
func everyPeerSentEndOfRIB(ctx context.Context, p *sdk.Plugin, want int) bool {
	status, data, err := dispatchMap11(ctx, p, "show bgp peer * detail")
	if err != nil || status != statusDone || data == nil {
		return false
	}
	peers, _ := data["peers"].(map[string]any)
	if len(peers) != want {
		return false
	}
	for _, raw := range peers {
		row, _ := raw.(map[string]any)
		switch v := row["eor-received"].(type) {
		case bool:
			if !v {
				return false
			}
		case float64:
			if v < 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
