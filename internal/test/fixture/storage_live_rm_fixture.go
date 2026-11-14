// Design: docs/architecture/zefs-format.md -- the ze data CLI and its daemon forms
// Related: storage_live_fixture.go -- storageLiveClient, storageLiveHolds, storageLiveStaleKey
// Related: register_storage_live.go -- registers storage/data-rm-refused-live

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// storageLiveRunningOwner is the plugin data-rm-refused-live.ci runs, and
// storageLiveRunningOwnerKey the key it holds the grant for: the ddos-flowspec
// record (internal/plugins/ddos/flowspec/register.go, announcementKey),
// granted to the process name ddos-flowspec alone. It is the cheapest granted
// plugin to run: `ddos flowspec` at `response-level alert` needs no BGP peer,
// no detection and no kernel feature.
const (
	storageLiveRunningOwner    = "ddos-flowspec"
	storageLiveRunningOwnerKey = "meta/ddos-flowspec/announcement"
)

// storageLiveRemoveLocally proves AC-3's local half, AC-4 and AC-5 through the
// local `ze data rm` on the daemon's host while the daemon owns the store: a
// configuration key is refused naming why and stays as it was; the key of the
// running ddos-flowspec plugin is refused naming the plugin's restart and stays
// absent; the seeded runtime key is removed with exactly the local lines and
// exit 0, and a fresh reader handle no longer finds it; removing it again is
// an error, exit 2. The daemon still answers afterwards.
func storageLiveRemoveLocally(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	// The daemon's own store, in the runner's config folder, with the login
	// the client store above holds: the environment an operator's shell on
	// the daemon's host has.
	dataEnv := overrideEnv04(os.Environ(),
		"ZE_SSH_PASSWORD=testpass",
		"ZE_SSH_HOST=127.0.0.1",
		"ZE_SSH_PORT="+args[0],
		"ZE_SSH_USERNAME=admin",
		"NO_COLOR=1",
	)
	if err := storageLiveKeyHeld(true); err != nil {
		return err
	}
	if err := storageLiveRmConfigurationKey(ctx, dataEnv); err != nil {
		return err
	}
	if err := storageLiveRmRunningOwnerKey(ctx, dataEnv); err != nil {
		return err
	}
	if err := storageLiveRmRuntimeKey(ctx, dataEnv); err != nil {
		return err
	}
	if _, err := storageLiveRequest(ctx, env, "show version"); err != nil {
		return err
	}
	return storageLiveOK("data-rm-refused-live")
}

// storageLiveRmConfigurationKey requires the local removal of a configuration
// key to be refused by the daemon, naming why, and the key to be as it was.
func storageLiveRmConfigurationKey(ctx context.Context, env []string) error {
	key := zefs.KeyFileActive.Key("data-rm-refused-live.conf")
	before, err := storageLiveHolds(key)
	if err != nil {
		return err
	}
	output, code, err := storageLiveDataRm(ctx, env, key)
	if err != nil {
		return err
	}
	if code != 2 {
		return fmt.Errorf("ze data rm %s exited %d, want 2:\n%s", key, code, output)
	}
	if !strings.Contains(output, "is configuration, not runtime state") {
		return fmt.Errorf("ze data rm %s refused without saying it is configuration:\n%s", key, output)
	}
	after, err := storageLiveHolds(key)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("the refused removal changed %s: held %t before, %t after", key, before, after)
	}
	return nil
}

// storageLiveRmRunningOwnerKey requires the local removal of the running
// plugin's key to be refused, naming the plugin's restart. Until the daemon
// has started its plugins it refuses saying it cannot tell yet, so the
// removal is retried until the plugin runs; a removal that is never refused
// for the running owner fails. The key was never written, so it stays absent.
func storageLiveRmRunningOwnerKey(ctx context.Context, env []string) error {
	key := storageLiveRunningOwnerKey
	restart := "request plugin restart " + storageLiveRunningOwner
	var last string
	var runErr error
	refused := Poll(ctx, 50, 200*time.Millisecond, func() bool {
		output, code, err := storageLiveDataRm(ctx, env, key)
		last, runErr = output, err
		if err != nil {
			return true
		}
		return code == 2 && strings.Contains(output, restart)
	})
	if runErr != nil {
		return runErr
	}
	if !refused {
		return fmt.Errorf("ze data rm %s was never refused naming %q while %s runs:\n%s", key, restart, storageLiveRunningOwner, last)
	}
	held, err := storageLiveHolds(key)
	if err != nil {
		return err
	}
	if held {
		return fmt.Errorf("%s is in the store after its refused removal", key)
	}
	return nil
}

// storageLiveRmRuntimeKey requires the local removal of the seeded runtime key
// to print exactly the local route's lines and exit 0, a fresh reader handle
// to no longer find the key, and a second removal to be an error, exit 2.
func storageLiveRmRuntimeKey(ctx context.Context, env []string) error {
	key := storageLiveStaleKey
	output, code, err := storageLiveDataRm(ctx, env, key)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze data rm %s exited %d, want 0:\n%s", key, code, output)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 2 {
		return fmt.Errorf("ze data rm %s printed %d lines, want the local route's two:\n%s", key, len(lines), output)
	}
	if lines[0] != "removed "+key {
		return fmt.Errorf("ze data rm %s first line = %q, want %q", key, lines[0], "removed "+key)
	}
	if !strings.HasPrefix(lines[1], "1 entries removed from ") {
		return fmt.Errorf("ze data rm %s second line = %q, want the count of the local route", key, lines[1])
	}
	if err := storageLiveKeyHeld(false); err != nil {
		return err
	}
	output, code, err = storageLiveDataRm(ctx, env, key)
	if err != nil {
		return err
	}
	if code != 2 {
		return fmt.Errorf("ze data rm of the removed %s exited %d, want 2:\n%s", key, code, output)
	}
	for _, want := range []string{"the store holds no key " + key, "0 entries removed from "} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("ze data rm of the removed %s lacks %q:\n%s", key, want, output)
		}
	}
	return nil
}

// storageLiveDataRm runs the local `ze data rm <key>` and answers its output
// (stdout and stderr together) and its exit code. An error is a run that did
// not reach an exit code, never a refusal.
func storageLiveDataRm(ctx context.Context, env []string, key string) (string, int, error) {
	output, err := runCommandProcess04(ctx, env, nil, "data", "rm", key)
	if err == nil {
		return output, 0, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return output, exitErr.ExitCode(), nil
	}
	return output, 0, err
}
