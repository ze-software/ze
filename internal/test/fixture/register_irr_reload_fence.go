// Design: docs/architecture/irr-owner.md -- a reload's new AS-SET is enforced before the peer's UPDATE is judged
// Related: irr_owner_fixture.go -- irrBGPEntry, irrListedUpdateAccepted
// Related: fixture.go -- observeConfigured owns startup, assertion reporting and shutdown
package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/irr-listed-fence", irrReloadFence, StopsDaemon)
}

// irrReloadFenceRoute is the route the observer announces to the test peer
// once the reload's AS-TEST is enforced. The peer expects it (NLRI 18C0A802,
// 192.168.2.0/24) before it sends the UPDATE the filter judges.
const irrReloadFenceRoute = "update text origin igp nhop 10.0.0.1 nlri add 192.168.2.0/24"

// irrReloadFencePeer selects the test peer the fence is announced to.
const irrReloadFencePeer = "127.0.0.1"

// irrReloadFence observes the reload tests that give a peer the IRR as-set
// AS-TEST by SIGHUP and then judge the peer's UPDATE
// (test/reload/irr-bgp-commit-new-set.ci, bgp-filter-irr-added-live.ci,
// bgp-filter-irr-first-add-live.ci).
//
// Goal: the peer's UPDATE is judged only once the filter enforces AS-TEST.
// The reload's sentinel route proves the reload applied, and no more: AS-TEST
// is stored only after the commit, when the irr plugin fetches the set the new
// interest names, and the filter publishes it on the owner's notice. An UPDATE
// that arrives between the two is rejected "no-prefix-list" and held, by
// design (docs/architecture/bgp/filter-irr.md), and these peers negotiate no
// route-refresh, so they are never asked for it again.
//
// Method: poll `show bgp irr` until AS-TEST is reported enforced (status ok),
// then announce irrReloadFenceRoute to the peer, which sends its UPDATE only
// after it received that route. Then wait for the peer's UPDATE announcing
// 10.0.0.0/24 to reach this observer: the reactor delivers a received UPDATE
// only after the import chain accepted it (reactor_notify.go), so its arrival
// is the filter's accept, and returning earlier would stop the daemon before
// the filter decided. Each failure returns an error naming what was not seen.
// irrReloadSentinelPrefix is the static route config2.conf adds on SIGHUP.
// Waiting for Ze to send it before announcing irrReloadFenceRoute ensures the
// peer receives 192.168.1.0/24 before 192.168.2.0/24 even when AS-TEST settles
// before the reload's static route is written to the wire.
const irrReloadSentinelPrefix = "192.168.1.0/24"

func irrReloadFence(ctx context.Context, _ []string) error {
	const name = "reload/irr-listed-fence"
	updates := make(chan string, 4)
	sentinelSent := make(chan struct{}, 1)
	setup := func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			var decoded map[string]any
			if json.Unmarshal([]byte(event), &decoded) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp, _ := decoded["bgp"].(map[string]any)
			message, _ := bgp["message"].(map[string]any)
			update, _ := bgp["update"].(map[string]any)
			if _, hasNLRI := update["nlri"]; !hasNLRI {
				return nil
			}
			rendered := text07(update)
			switch message["direction"] {
			case directionSent:
				if strings.Contains(rendered, irrReloadSentinelPrefix) {
					select {
					case sentinelSent <- struct{}{}:
					default:
					}
				}
			case directionReceived:
				select {
				case updates <- rendered:
				default:
				}
			}
			return nil
		})
		p.SetEventFormat("parsed")
		return nil
	}
	scenario := func(ctx context.Context, p *sdk.Plugin) error {
		const delay = 200 * time.Millisecond
		attempts := max(int(WaitBudget(60, 18*time.Second)/delay), 1)
		listed := until07(ctx, p, "show bgp irr", attempts, delay, func(r commandResult07) bool {
			entry := irrBGPEntry(r)
			return entry != nil && entry["status"] == "ok"
		})
		if entry := irrBGPEntry(listed); entry == nil || entry["status"] != "ok" {
			return fmt.Errorf("show bgp irr never reported %s enforced after the reload: status=%s error=%w data=%s",
				irrTestASSet, listed.status, listed.err, text07(listed.data))
		}
		select {
		case <-sentinelSent:
		case <-time.After(WaitBudget(50, 10*time.Second)):
			return fmt.Errorf("reload sentinel %s was never sent to %s before announcing fence", irrReloadSentinelPrefix, irrReloadFencePeer)
		case <-ctx.Done():
			return ctx.Err()
		}
		if _, _, err := p.UpdateRoute(ctx, irrReloadFencePeer, irrReloadFenceRoute); err != nil {
			return fmt.Errorf("announce the fence route to %s: %w", irrReloadFencePeer, err)
		}
		return irrListedUpdateAccepted(ctx, updates)
	}
	reg := &sdk.Registration{
		Receives: bgpReceiveDecls([]string{eventUpdate}, nil),
	}
	return observeConfigured(ctx, name, reg, setup, scenario)
}
