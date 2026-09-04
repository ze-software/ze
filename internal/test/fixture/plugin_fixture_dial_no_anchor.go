// Design: docs/architecture/api/process-protocol.md -- the plugin trust anchor

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// dialNoAnchorPlugin is the name the .ci declares for this process, and the name
// BOTH connection attempts use. The token, the address and the name are held
// constant so the trust anchor is the only thing that differs between the
// attempt that must be refused and the one that must succeed.
const dialNoAnchorPlugin = "dial-no-anchor-test"

// pluginDialNoAnchor proves spec-local-ca AC-4 from the entry point an external
// plugin uses: sdk.NewFromTLSEnv, the call every plugin process makes to reach
// its engine.
//
// It runs the refused attempt FIRST, while this is still an unregistered
// process, then registers normally. The registration is what makes the refusal
// mean something: the same environment with one variable removed is the
// difference between a plugin the engine knows about and no connection at all.
func pluginDialNoAnchor(ctx context.Context, _ []string) error {
	if probeErr := dialNoAnchorProbe(); probeErr != nil {
		// Reported HERE rather than carried into the scenario below. A blind
		// dial that SUCCEEDS has already registered this name with the engine,
		// so registering a second time under it leaves the engine reading a
		// closed mux and the .ci fails on a timeout instead of on the sentence
		// that says what went wrong. This process is still the engine's child,
		// so the engine relays this line (process.go, relayStderrFrom).
		ReportFailure(probeErr)
		return probeErr
	}
	return Observe(ctx, dialNoAnchorPlugin, sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		// The peer is held until ze sends its End-of-RIB, for the reason
		// fixture10PKIExport states: a shutdown requested before the marker
		// reaches the peer replaces it with a Cease notification, and the .ci
		// asserts on the marker.
		if err := fixture10WaitEOR(ctx, plugin, "*", 100); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: a plugin with no trust anchor did not register, and the same plugin with the engine root did")
		return nil
	})
}

// dialNoAnchorProbe removes the trust anchor from this process's environment and
// requires the dial to fail.
//
// The assertion is on the RETURNED PLUGIN, not on a log line: a non-nil *Plugin
// is a completed TLS handshake plus an accepted auth frame, which is what
// registration is. The pin this replaced answered an empty anchor with
// InsecureSkipVerify and no comparison, so this same call returned a live,
// registered plugin over a connection nothing had verified.
func dialNoAnchorProbe() error {
	anchor := env.Get("ze.plugin.ca.pem")
	if anchor == "" {
		return errors.New("dial-no-anchor: the engine passed no ze.plugin.ca.pem, so removing it proves nothing")
	}

	if err := env.Set("ze.plugin.ca.pem", ""); err != nil {
		return fmt.Errorf("dial-no-anchor: clear the trust anchor: %w", err)
	}
	blind, dialErr := sdk.NewFromTLSEnv(dialNoAnchorPlugin)
	if restoreErr := env.Set("ze.plugin.ca.pem", anchor); restoreErr != nil {
		return fmt.Errorf("dial-no-anchor: restore the trust anchor: %w", restoreErr)
	}

	if dialErr == nil {
		blind.Close() //nolint:errcheck // the failure is that this connection exists at all
		return errors.New("dial-no-anchor: a plugin with no trust anchor REGISTERED with the engine")
	}
	if !strings.Contains(dialErr.Error(), "trust anchor") {
		return fmt.Errorf("dial-no-anchor: refused for the wrong reason: %w", dialErr)
	}
	return nil
}
