// Design: docs/architecture/traffic/cos-plugin.md -- a profile edit reaches the interfaces using it
// Related: misc_fixture_shellports.go -- reloadSignalDriver, the shared rewrite-and-SIGHUP trigger
// Related: misc_fixture_drivers.go -- commandOutput
//
// The driver behind test/reload/cos-profile-reload.ci. It reads the VLAN QoS
// maps the kernel holds for zcos0.100 before and after a reload that edits
// only the class-of-service profile the VLAN's parent names, so the evidence
// is what the kernel was programmed with, not a log line.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

func init() {
	Register("reload/cos-profile-reload-setup", cosProfileReloadSetup)
	Register("reload/cos-profile-reload", cosProfileReloadDriver)
}

// cosProfileReloadSetup removes a zcos0 an earlier run left behind. The
// daemon programs an existing VLAN to its config at boot, so a leftover
// would not make the test pass wrongly, but it would make a red run start
// from the previous run's maps instead of from nothing.
func cosProfileReloadSetup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("reload/cos-profile-reload-setup takes no arguments")
	}
	// Only iproute2's own "does not exist" answer means absent; any other
	// failure of `ip` is reported, never read as a clean start.
	output, err := commandOutput(ctx, "", nil, "", "ip", "link", "show", "dev", cosReloadParent)
	if err != nil {
		if strings.Contains(string(output), "does not exist") {
			fmt.Fprintln(os.Stderr, "OK: no leftover", cosReloadParent)
			return nil
		}
		return fmt.Errorf("look for a leftover %s: %w", cosReloadParent, err)
	}
	if _, err := commandOutput(ctx, "", nil, "", "ip", "link", "del", "dev", cosReloadParent); err != nil {
		return fmt.Errorf("remove the leftover %s: %w", cosReloadParent, err)
	}
	fmt.Fprintln(os.Stderr, "OK: removed a leftover", cosReloadParent)
	return nil
}

// The VLAN and the maps test/reload/cos-profile-reload.ci configures, in the
// form `ip -d link show` prints them. The edit moves ingress PCP 5 to PCP 2 and
// drops egress priority 5, so the after-state also proves the maps were
// replaced rather than merged: a merge leaves `5:5` in both.
const (
	cosReloadParent        = "zcos0"
	cosReloadVLAN          = "zcos0.100"
	cosReloadBootIngress   = "ingress-qos-map { 5:5 }"
	cosReloadBootEgress    = "egress-qos-map { 3:3 5:5 }"
	cosReloadEditedIngress = "ingress-qos-map { 2:2 }"
	cosReloadEditedEgress  = "egress-qos-map { 3:6 }"
	cosReloadPollDelay     = 100 * time.Millisecond
)

var cosProfileReloadTrigger = reloadSignalDriver(reloadSignalPlan{
	source:       fileConfig2Conf,
	destination:  fileBGPConf,
	hups:         1,
	requireReady: true,
})

// cosProfileReloadDriver waits for the boot maps, triggers the reload, then
// waits for the edited maps. Each wait is bounded by the runner's budget and
// fails naming the line the kernel last printed.
func cosProfileReloadDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("reload/cos-profile-reload takes no arguments")
	}
	if !waitForFile(ctx, "daemon.ready", 300, cosReloadPollDelay) {
		return errors.New("daemon.ready not found: the daemon did not finish starting")
	}
	if err := awaitVLANQoS(ctx, cosReloadBootIngress, cosReloadBootEgress); err != nil {
		return fmt.Errorf("boot maps: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: boot programmed", cosReloadVLAN, "with", cosReloadBootIngress, cosReloadBootEgress)

	if err := cosProfileReloadTrigger(ctx, nil); err != nil {
		return err
	}
	if err := awaitVLANQoS(ctx, cosReloadEditedIngress, cosReloadEditedEgress); err != nil {
		return fmt.Errorf("after the profile-only reload: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: the profile edit reprogrammed", cosReloadVLAN, "with", cosReloadEditedIngress, cosReloadEditedEgress)
	return nil
}

// awaitVLANQoS polls `ip -d link show` until the VLAN carries both map
// strings. An `ip` failure is kept and reported, never read as absence.
func awaitVLANQoS(ctx context.Context, ingress, egress string) error {
	var last string
	var lastErr error
	held := Poll(ctx, WaitAttempts(50, cosReloadPollDelay, 100), cosReloadPollDelay, func() bool {
		output, err := commandOutput(ctx, "", nil, "", "ip", "-d", "link", "show", "dev", cosReloadVLAN)
		last, lastErr = string(output), err
		if err != nil {
			return false
		}
		return strings.Contains(last, ingress) && strings.Contains(last, egress)
	})
	if held {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("want %q and %q on %s; reading it failed: %w", ingress, egress, cosReloadVLAN, lastErr)
	}
	return fmt.Errorf("want %q and %q on %s; the kernel shows:\n%s", ingress, egress, cosReloadVLAN, last)
}
