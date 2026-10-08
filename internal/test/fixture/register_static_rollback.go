// Design: docs/architecture/config/transaction-protocol.md -- which participants a rollback reaches
// Related: netfilter_fixture_static.go -- the kernel route helpers this driver polls with
// Related: register_config_apply_ordering_rollback.go -- the failer that refuses the second reload
//
// The driver behind test/static/static-rollback-keeps-committed.ci.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("static/static-rollback-keeps-committed", observeDriver("static-rollback-driver", staticRollbackKeepsCommitted))
}

// staticRollbackFailer is the plugin block of the participant that refuses
// its section apply, and of this driver, which runs as a plugin so it can read
// `show reload-status`. The failer's name sorts before "static", and section
// applies run in sorted participant order, so it fails before static applies
// anything. The driver declares no config root and is never a participant.
const staticRollbackFailer = `plugin {
	external config-apply-ordering-mixed-rollback-failer {
		run "le test fixture reload/config-apply-ordering-mixed-rollback-failer"
		encoder json
	}
	external static-rollback-driver {
		run "le test fixture static/static-rollback-keeps-committed"
		encoder json
	}
}
`

// staticRollbackRSVP returns the rsvp-te root the failer declares. A changed
// refresh period makes the failer a participant of the reload.
func staticRollbackRSVP(refresh string) string {
	return `rsvp-te {
	router-id 10.0.0.1
	refresh-period ` + refresh + `
	interface lo {
		max-bandwidth 10e9
		max-reservable-bandwidth 8e9
		address 127.0.0.1/8
	}
}
`
}

// staticRollbackCommitted is the first reload: 172.16.0.0/12 is removed and
// the transaction commits.
const staticRollbackCommitted = `static {
	table default {
		route 10.0.0.0/8 {
			next {
				hop 192.168.1.1 { }
			}
		}
	}
}
`

// staticRollbackRefused is the second reload: static changes too, so static is
// a participant, and the failer refuses before static's apply runs.
const staticRollbackRefused = `static {
	table default {
		route 10.0.0.0/8 {
			next {
				hop 192.168.1.1 { }
			}
		}
		route 198.51.100.0/24 {
			next {
				hop 192.168.1.1 { }
			}
		}
	}
}
`

// staticRollbackKeepsCommitted commits a reload that removes a static route,
// then sends a reload that fails before static applies. The daemon's stderr
// carries the verdict, which the .ci asserts: the second reload rolls back, and
// static runs no rollback, because it applied nothing in that transaction.
//
// Each reload is fenced on the `show reload-status` generation, which the hub
// advances only once the whole reload, candidate promotion included, has run
// (Server.MarkReloadProcessed). The kernel route lands before promotion, so
// writing the next config on the route alone met "candidate config already
// staged".
func staticRollbackKeepsCommitted(ctx context.Context, plugin *sdk.Plugin) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var routes string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes = routesOutput(ctx)
		return strings.Contains(routes, "10.0.0.0/8") && strings.Contains(routes, "172.16.0.0/12")
	}) {
		return fmt.Errorf("initial static routes not programmed before reload:\n%s", routes)
	}

	committed := staticRollbackFailer + staticRollbackRSVP("30") + staticRollbackCommitted
	if err := staticRollbackReload(ctx, plugin, pid, committed, "applied"); err != nil {
		return fmt.Errorf("first reload: %w", err)
	}
	routes = routesOutput(ctx)
	if !strings.Contains(routes, "10.0.0.0/8") || strings.Contains(routes, "172.16.0.0/12") {
		return fmt.Errorf("first reload did not remove 172.16.0.0/12 from the kernel:\n%s", routes)
	}

	refused := staticRollbackFailer + staticRollbackRSVP("10") + staticRollbackRefused
	if err := staticRollbackReload(ctx, plugin, pid, refused, "failed"); err != nil {
		return fmt.Errorf("second reload: %w", err)
	}
	// The rollback has run once the generation advanced. The kernel must still
	// hold the set the first reload committed: a replayed first-reload undo
	// re-installs 172.16.0.0/12, the boot set from two commits back.
	routes = routesOutput(ctx)
	if !strings.Contains(routes, "10.0.0.0/8") {
		return fmt.Errorf("after the failed second reload the kernel lost the committed 10.0.0.0/8:\n%s", routes)
	}
	if strings.Contains(routes, "172.16.0.0/12") {
		return fmt.Errorf("after the failed second reload the kernel holds 172.16.0.0/12, which the committed first reload removed:\n%s", routes)
	}
	if strings.Contains(routes, "198.51.100.0/24") {
		return fmt.Errorf("after the failed second reload the kernel holds its refused 198.51.100.0/24:\n%s", routes)
	}
	fmt.Fprintln(os.Stderr, "OK: static kept the committed routes after the refused reload")
	return nil
}

// staticRollbackReload writes config, sends SIGHUP, and waits until the reload
// generation advances, then checks the reload's outcome.
func staticRollbackReload(ctx context.Context, plugin *sdk.Plugin, pid int, config, outcome string) error {
	baseline := reloadGeneration(ctx, plugin)
	if baseline < 0 {
		return errors.New("reload generation unavailable")
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(config), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	var status string
	var result map[string]any
	var err error
	if !Poll(ctx, WaitAttempts(40, 100*time.Millisecond, 100), 100*time.Millisecond, func() bool {
		status, result, err = dispatchMap(ctx, plugin, "show reload-status")
		return err == nil && status == statusDone && int64(number07(result["generation"])) > baseline
	}) {
		return fmt.Errorf("reload did not complete after generation %d: status=%s data=%v error=%v", baseline, status, result, err) //nolint:errorlint // The last poll error may be nil here; it is diagnostic context
	}
	if result["last-outcome"] != outcome {
		return fmt.Errorf("reload outcome %v, want %s", result["last-outcome"], outcome)
	}
	return nil
}
