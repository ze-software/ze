// Design: plan/spec-mpls-10-rsvp-te-reload-completeness.md -- RSVP-TE config reload
// Related: routing_fixture.go -- routingObserver, routingRows, the other rsvp-te scenarios
// Related: misc_fixture_shellports.go -- reloadSignalDriver, the shared rewrite-and-SIGHUP trigger
//
// The two fixtures behind test/reload/rsvpte-reload.ci. Registration lives in this
// file rather than beside the other rsvp-te scenarios so the change touches no file
// another session is editing.
package fixture

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/rsvpte-reload-trigger", reloadSignalDriver(reloadSignalPlan{
		source:       fileConfig2Conf,
		destination:  fileBGPConf,
		hups:         1,
		requireReady: true,
	}))
	Register("reload/rsvpte-reload", routingObserver("rsvpte-reload", rsvpteReloadScenario))
}

// rsvpteInterfaceNames reports the interface names `show rsvp-te interface` lists.
func rsvpteInterfaceNames(ctx context.Context, plugin *sdk.Plugin) ([]string, error) {
	rows, err := routingRows(ctx, plugin, "show rsvp-te interface", false)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, routingString(row, "name"))
	}
	return names, nil
}

// rsvpteReloadScenario proves that an interface a config commit removes stops being
// serviced. The scenario starts before the reload, so it records the interface set the
// daemon booted with, waits for the trigger's SIGHUP to land, and reads the set again:
// the removed interface is gone and the kept one is still there.
//
// Before this, OnConfigApply only ever called setInterface, so an interface the
// operator took out of the config kept its admission entry for the life of the daemon
// and `show rsvp-te interface` kept reporting a link ze no longer accounted for.
func rsvpteReloadScenario(ctx context.Context, plugin *sdk.Plugin) error {
	booted, err := rsvpteInterfaceNames(ctx, plugin)
	if err != nil {
		return err
	}
	for _, want := range []string{"lo", "dummy0"} {
		if !slices.Contains(booted, want) {
			return fmt.Errorf("show rsvp-te interface: booted set %v is missing %s", booted, want)
		}
	}

	// The trigger rewrites the config without dummy0 and sends SIGHUP.
	var latest []string
	removed := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		names, pollErr := rsvpteInterfaceNames(ctx, plugin)
		if pollErr != nil {
			return false
		}
		latest = names
		return !slices.Contains(names, "dummy0")
	})
	if !removed {
		return fmt.Errorf("show rsvp-te interface still lists dummy0 after the reload: %v", latest)
	}
	if !slices.Contains(latest, "lo") {
		return fmt.Errorf("the reload dropped the interface it kept: %v", latest)
	}

	fmt.Fprintln(os.Stderr, "OK: rsvp-te reload stopped servicing the removed interface")
	return nil
}
