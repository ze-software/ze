// Design: docs/functional-tests.md -- normal-hub shutdown through the VPP stub.
// Related: misc_fixture_vpp.go -- existing child owner, configuration and API log.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	Register("vpp/vpp-shutdown", vppShutdownDriver)
}

// vppShutdownDriver checks a normal hub's raw exit after its actual ready-file
// barrier. The interface is a live consumer of the external VPP connection, so
// its cleanup must finish before that provider closes.
func vppShutdownDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("VPP shutdown fixture takes no arguments")
	}
	dir, socket, log, err := vppWorkPaths("vpp-shutdown-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // all owned children are joined before the directory cleanup
	stub, err := startVPPStub(ctx, socket, log, 30)
	if err != nil {
		return err
	}
	defer vppShutdownCleanup(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("VPP shutdown stub socket did not appear")
	}
	readyPath := filepath.Join(dir, "ready")
	config := fmt.Sprintf("environment {}\n"+
		"bgp { router-id 10.0.0.1; session { asn { local 65533; } } }\n"+
		"vpp { enabled true; external true; api-socket %s; lcp { enabled false; } }\n"+
		"interface { backend vpp; dummy vpp-shutdown0 { unit 0 { ipv4 { address [ 10.0.0.2/24 ]; } } } }\n", socket)
	daemon, err := startVPPZe(ctx, config, vppShutdownEnvironment(dir, readyPath))
	if err != nil {
		return err
	}
	defer vppShutdownCleanup(daemon, 3*time.Second)
	if err := vppShutdownReady(ctx, daemon, stub, readyPath); err != nil {
		return err
	}
	if err := vppShutdownInterfaceRequest(log, "create_loopback", false); err != nil {
		return err
	}
	if err := vppShutdownInterfaceRequest(log, "sw_interface_add_del_address", true); err != nil {
		return err
	}
	if err := vppShutdownRequirePendingExit(stub, "VPP stub"); err != nil {
		return err
	}
	if err := vppShutdownStopZe(daemon); err != nil {
		return err
	}
	stopFixtureProcess(stub, 2*time.Second)
	if err := stub.exit(<-stub.done); err != nil {
		return fmt.Errorf("VPP stub did not exit cleanly after the daemon: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: VPP daemon and stub exited cleanly after normal hub shutdown")
	return nil
}

// vppShutdownEnvironment owns the diagnostic producers this fixture observes.
// A caller's disabled subsystem or syslog backend must not hide a cleanup
// failure from the captured stderr used by vppShutdownStopZe.
func vppShutdownEnvironment(dir, readyPath string) map[string]string {
	return map[string]string{
		envConfigDir:          dir,
		envReadyFile:          readyPath,
		envLogVPP:             logLevelInfo,
		envLogBGP:             logLevelWarn,
		"ze.log":              logLevelInfo,
		envLogInterface:       logLevelWarn,
		"ze.log.plugin":       logLevelWarn,
		"ze.log.relay":        logLevelWarn,
		"ze.log.plugin.relay": logLevelWarn,
		"ze.log.backend":      "stderr",
	}
}

// vppShutdownReady polls the hub's registered ready-file barrier, which follows
// startup completion and signal registration. A failed child or unreadable
// barrier is an error, never readiness.
func vppShutdownReady(ctx context.Context, daemon, stub *fixtureProcess, path string) error {
	var readinessErr error
	ready := Poll(ctx, 200, 50*time.Millisecond, func() bool {
		if err := vppShutdownRequirePendingExit(daemon, "daemon"); err != nil {
			readinessErr = err
			return true
		}
		if err := vppShutdownRequirePendingExit(stub, "VPP stub"); err != nil {
			readinessErr = err
			return true
		}
		_, err := os.Stat(path) //nolint:gosec // the fixture's own ready-file path
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}
		readinessErr = err
		return true
	})
	if readinessErr != nil {
		return fmt.Errorf("VPP shutdown readiness failed: %w", readinessErr)
	}
	if !ready {
		return errors.New("VPP shutdown daemon did not publish its ready file")
	}
	return nil
}

// vppShutdownInterfaceRequest checks the one interface this fresh stub owns.
// The daemon ready file follows successful configuration acknowledgements.
func vppShutdownInterfaceRequest(path, message string, addAddress bool) error {
	matched, err := vppLogMatches(path, func(entry map[string]any) bool {
		if entry["msg"] != message {
			return false
		}
		fields, ok := entry["fields"].(map[string]any)
		if !ok {
			return false
		}
		if fields["sw_if_index"] != float64(1) {
			return false
		}
		if addAddress {
			return fields["is_add"] == true
		}
		return true
	})
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("VPP shutdown stub did not observe %s for its owned interface 1", message)
	}
	return nil
}

// vppShutdownStopZe MUST receive a successfully started child whose caller
// already deferred vppShutdownCleanup. It observes the existing owner's raw
// wait status, restores it for cleanup, and never calls Cmd.Wait a second time.
func vppShutdownStopZe(daemon *fixtureProcess) error {
	if err := vppShutdownRequirePendingExit(daemon, "daemon"); err != nil {
		return err
	}
	stopFixtureProcess(daemon, 3*time.Second)
	if err := daemon.exit(<-daemon.done); err != nil {
		return fmt.Errorf("daemon did not exit cleanly after SIGTERM: %w", err)
	}
	output := daemon.output.String()
	if !strings.Contains(output, "Ze stopped.") {
		return errors.New("daemon exited without its normal shutdown completion")
	}
	for _, code := range []string{
		diagnostic.CodeDoctorPluginShutdownOrder,
		diagnostic.CodeDoctorPluginStopCleanupLate,
		diagnostic.CodeDoctorPluginStopUnfinished,
		diagnostic.CodeDoctorIfaceOwnedDeviceCleanup,
		diagnostic.CodeDoctorIfaceBackendClose,
	} {
		if strings.Contains(output, code) {
			return fmt.Errorf("daemon shutdown reported %s", code)
		}
	}
	for _, failure := range []string{"warning: shutdown timeout:", "vpp manager stopped with an error"} {
		if strings.Contains(output, failure) {
			return fmt.Errorf("daemon shutdown reported %s", failure)
		}
	}
	return nil
}

// vppShutdownRequirePendingExit refuses a completed raw wait and puts its status
// back. A nil answer means only that this owner's wait has not completed yet.
func vppShutdownRequirePendingExit(process *fixtureProcess, role string) error {
	select {
	case status := <-process.done:
		if err := process.exit(status); err != nil {
			return fmt.Errorf("%s exited before requested shutdown: %w", role, err)
		}
		return fmt.Errorf("%s exited before requested shutdown", role)
	default:
		return nil
	}
}

// vppShutdownCleanup MUST be deferred after each successful start. An already
// reaped child needs no second signal; otherwise the unchanged best-effort stop
// still owns its termination and sole wait on every failed acceptance path.
func vppShutdownCleanup(process *fixtureProcess, grace time.Duration) {
	select {
	case status := <-process.done:
		process.exit(status) //nolint:errcheck // acceptance checks the raw status; cleanup preserves it
		return
	default:
		stopFixtureProcess(process, grace)
	}
}
