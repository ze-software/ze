// Design: docs/guide/graceful-restart.md -- received-route GR and LLGR lifecycle.
package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	llgrScenarioTransition = "transition"
	llgrScenarioTimer      = "timer"
)

// llgrLifecycle observes only routes received from TCP peers. Markers release
// peer actions after their prerequisite RIB state, never after a guessed delay.
func llgrLifecycle(scenario string) Driver {
	return func(ctx context.Context, _ []string) error {
		return runPlugin13(ctx, "observer", nil, func(ctx context.Context, p *sdk.Plugin) (scenarioErr error) {
			defer func() { ReportFailure(scenarioErr) }()
			count := func(filter string, want int) bool {
				r := command13(ctx, p, "show bgp rib received "+filter+" count")
				got, ok := countFrom13(r)
				return done13(r) && ok && got == want
			}
			wait := func(phase string, predicate func() bool) error {
				fmt.Fprintln(os.Stderr, "observing LLGR: "+phase)
				if !Poll(ctx, 150, 100*time.Millisecond, predicate) {
					fmt.Fprintln(os.Stderr, "failed LLGR observation: "+phase+": "+command13(ctx, p, "show bgp rib received").text())
					return fmt.Errorf("%s: %s", phase, command13(ctx, p, "show bgp rib received").text())
				}
				return nil
			}
			marker := func(peer string) error {
				r := command13(ctx, p, "send bgp "+peer+" update text origin igp local-preference 100 nhop 1.1.1.1 nlri ipv4/unicast add 198.51.100.0/24")
				if !done13(r) {
					return fmt.Errorf("release %s: %s", peer, r.text())
				}
				return nil
			}
			if scenario != llgrScenarioTransition {
				destination := "control"
				if scenario == llgrScenarioTimer {
					destination = "receiver"
				}
				if err := fixture10WaitEOR(ctx, p, destination, 60); err != nil {
					return err
				}
				if err := fixture10WaitEOR(ctx, p, "source", 60); err != nil {
					return err
				}
				if scenario == llgrScenarioTimer {
					if err := fixture10WaitEOR(ctx, p, "control", 60); err != nil {
						return err
					}
					r := command13(ctx, p, "send bgp control update text origin igp local-preference 100 nhop 1.1.1.1 nlri ipv4/unicast add 198.51.99.0/24")
					if !done13(r) {
						return fmt.Errorf("release long LLST source: %s", r.text())
					}
					if err := wait("long LLST control received and acknowledged on wire", func() bool {
						return count("prefix 10.0.3.0/24", 1) && count("prefix 203.0.110.0/24", 1)
					}); err != nil {
						return err
					}
					if err := marker("control"); err != nil {
						return err
					}
					// RFC 9494 Section 4.2. The timer fixture MUST acknowledge
					// control's LLGR_STALE advertisement before source can send.
					// Each LLGR entry replays the whole destination family; this
					// ordering makes both replay populations deterministic.
					if err := wait("long LLST control is stale and acknowledged on wire", func() bool {
						return count("prefix 10.0.3.0/24 community 65535:6", 1) &&
							llgrRouteLevel(ctx, p, "10.0.3.0/24", 2) &&
							count("prefix 203.0.111.0/24", 1)
					}); err != nil {
						return err
					}
				}
				r := command13(ctx, p, "send bgp source update text origin igp local-preference 100 nhop 1.1.1.1 nlri ipv4/unicast add 198.51.99.0/24")
				if !done13(r) {
					return fmt.Errorf("release source after destination readiness: %s", r.text())
				}
			}
			if err := wait("initial received routes", func() bool {
				return count("prefix 10.0.0.0/24", 1) && count("prefix 10.0.1.0/24 community 65535:7", 1) && count("prefix 10.0.2.0/24", 1)
			}); err != nil {
				return err
			}
			if scenario != llgrScenarioTransition {
				if err := wait("destination acknowledges initial wire delivery", func() bool { return count("prefix 203.0.112.0/24", 1) }); err != nil {
					return err
				}
			}
			if err := marker("source"); err != nil {
				return err
			}
			if err := wait("GR retains received NO_LLGR and plain routes before LLGR", func() bool {
				return llgrRouteLevel(ctx, p, "10.0.0.0/24", 1) && count("prefix 10.0.1.0/24 community 65535:7", 1)
			}); err != nil {
				return err
			}
			if err := wait("LLGR deletes NO_LLGR and marks retained routes", func() bool {
				return count("prefix 10.0.1.0/24", 0) && count("prefix 10.0.0.0/24 community 65535:6", 1) && llgrRouteLevel(ctx, p, "10.0.0.0/24", 2) && llgrRouteLevel(ctx, p, "10.0.2.0/24", 2)
			}); err != nil {
				return err
			}
			if scenario != llgrScenarioTransition {
				if err := wait("destination acknowledges NO_LLGR withdrawal and stale wire advertisement", func() bool { return count("prefix 203.0.113.0/24", 1) }); err != nil {
					return err
				}
			}
			switch scenario {
			case llgrScenarioTimer:
				if err := wait("remote short LLST expires while long LLST control survives", func() bool {
					return count("prefix 10.0.0.0/24", 0) && count("prefix 10.0.2.0/24", 0) && count("prefix 10.0.3.0/24 community 65535:6", 1) && llgrRouteLevel(ctx, p, "10.0.3.0/24", 2)
				}); err != nil {
					return err
				}
				if err := wait("destination acknowledges expiry withdrawals", func() bool { return count("prefix 203.0.114.0/24", 1) }); err != nil {
					return err
				}
			case "eor":
				if err := wait("reconnect refresh before EOR retains the unrefreshed route", func() bool {
					return llgrRouteLevel(ctx, p, "10.0.0.0/24", 0) && count("prefix 10.0.0.0/24 community 65535:6", 0) && llgrRouteLevel(ctx, p, "10.0.2.0/24", 2)
				}); err != nil {
					return err
				}
				if err := marker("source"); err != nil {
					return err
				}
				if err := wait("EOR purges only the unrefreshed stale route", func() bool {
					return count("prefix 10.0.2.0/24", 0) && llgrRouteLevel(ctx, p, "10.0.0.0/24", 0)
				}); err != nil {
					return err
				}
				if err := wait("destination acknowledges EOR withdrawal", func() bool { return count("prefix 203.0.114.0/24", 1) }); err != nil {
					return err
				}
			case "wire":
				if err := wait("nonretained family is removed", func() bool { return count("prefix fc00:1::/64", 0) }); err != nil {
					return err
				}
			}
			// The .ci stderr fence MUST own successful shutdown; this observer
			// MUST NOT race it with a second shutdown request.
			fmt.Fprintln(os.Stderr, "OK: received LLGR lifecycle "+scenario)
			return nil
		}, false)
	}
}

func llgrRouteLevel(ctx context.Context, p *sdk.Plugin, prefix string, want int) bool {
	r := command13(ctx, p, "show bgp rib received prefix "+prefix)
	if !done13(r) {
		return false
	}
	rows, ok := r.object()["routes"].([]any)
	if !ok {
		return false
	}
	if len(rows) != 1 {
		return false
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		return false
	}
	level, present := row["stale-level"]
	if want == 0 {
		return !present
	}
	return present && number13(level) == want
}
