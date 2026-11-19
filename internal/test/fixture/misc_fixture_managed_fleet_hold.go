// Design: docs/architecture/fleet-config.md -- the baseline and the hold of a managed client that runs a configuration its hub did not send
// Related: register_managed_fleet_hold.go -- registers the two drivers
// Related: storage_live_client_fixture.go -- restoreClientDaemon, restoreClientHubConfig, restoreClientConfig, the hub and the device
// Related: misc_fixture_managed_cached_boot.go -- cachedBootStart and cachedBootWait, the device's daemon
// Related: misc_fixture_managed_fleet_status.go -- fleetStatusScenario, the store writes and show fleet status beside the device
// Related: misc_fixture_managed_fleet_doctor.go -- fleetDoctorReport, ze doctor beside the device
// Related: misc_fixture_managed_fleet_leave.go -- the leave and rejoin drivers, which run on this hub and this device

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/test/runner"
)

// The router-ids that tell the configurations of one run apart, beside the
// boot and served ones of storage_live_client_fixture.go, and the texts the
// product writes that the verdicts read.
const (
	// The hub's second configuration, written while the device is stopped.
	fleetHoldChangedRouterID = "3.3.3.3"
	// The configuration given to the device itself, with its daemon stopped.
	fleetHoldLocalRouterID = "9.9.9.9"
	// The hub's configuration written while the device holds.
	fleetHoldFurtherRouterID = "4.4.4.4"
	// The code and the text of the hub's doctor row for a client that refused
	// its configuration (internal/component/plugin/doctor,
	// check_managed_runtime.go, managedRejectedRow), followed by the opening of
	// the device's own refusal (internal/component/managed, client.go,
	// holdRefusalPrefix).
	fleetHoldRejectedCode = "doctor-managed-client-rejected"
	fleetHoldRefusalText  = "refused to apply the config the hub serves it: " +
		"holding: the configuration this device runs differs from the last one this hub sent"
	// The code of the hub's doctor row for a client that holds and whose
	// configuration the hub keeps a copy of (internal/component/managed,
	// doctor.go, diagnoseHeldClient). A device that holds sends its
	// configuration and acknowledges the hold with holding: the hub then keeps
	// the copy and shows the hold under this code, not as a refusal
	// (spec-fleet-7 E-10).
	fleetHoldDivergedCode = "doctor-managed-client-diverged"
	// The parts of that row's message: the client holds at its own version,
	// the hub serves it another, and the hub keeps the copy.
	fleetHoldDivergedHolds  = "the managed client " + caTrustClientName + " holds: it runs version "
	fleetHoldDivergedServes = "this hub serves it version "
	fleetHoldDivergedCopy   = "this hub keeps a copy of its configuration"
	// What every doctor code starts with: a failed `show doctor` that names
	// one printed a report.
	fleetHoldDoctorCodePrefix = "doctor-"
	// The WARN line the device writes for each fetch it holds (client.go,
	// holdFetched). Its attributes carry the active, baseline and offered hashes.
	fleetHoldWarnMessage = "this device holds: the configuration it runs differs from the last one its hub sent"
	// The device applies a push through a reload, and the hub allows its
	// plugins 30 seconds to start (cmd/ze/hub/main.go).
	fleetHoldFollowAttempts = 300
	// Each attempt is a whole `ze cli` process and an SSH session to the hub.
	fleetHoldDoctorAttempts = 150
	// The device logs its hold after it acknowledged it, and the hub pushes a
	// change within its push timeout of 10 seconds.
	fleetHoldLogAttempts = 300
	// The device's active version pointer (pkg/zefs KeyConfigActive): the
	// version stamp of the configuration its daemon boots and runs. Every
	// version the store makes active moves it, so a device that applied
	// nothing still points where its last edit left it.
	fleetHoldActivePointerKey = "meta/config/" + caTrustClientName + ".conf/active"
)

// fleetHoldScenario is one running hub and the store of one managed device,
// with what an operator beside each needs.
type fleetHoldScenario struct {
	hub    *caTrustDaemon
	device *caTrustDaemon
	// hubOperator is the environment of `ze cli` over the hub's SSH, and
	// hubImporter the one of `ze data import` beside the running hub.
	hubOperator []string
	hubImporter []string
	// deviceOperator runs `ze cli`, `ze data write` and `ze doctor` beside the
	// device. With no deviceListener its SSH address is one nothing listens
	// on, so each answers from the device's store. With one it is the running
	// device's own listener.
	deviceOperator fleetStatusScenario
	// deviceListener is the part of the device's configuration that serves
	// SSH, and "" for a device that serves none. It is part of every
	// configuration of the device, on the hub and on the device alike.
	deviceListener string
	// editedStamp is the active version pointer the last edit of the stopped
	// device left: what a device that holds still points at.
	editedStamp string
	managedPort string
	hubAddress  string
}

// managedFleetHoldKeepsLocalEditDriver answers one question: does a managed
// device that runs a configuration its hub did not send keep it, stay
// connected and tell its hub, while a device nobody changed still follows its
// hub across a restart (spec-fleet-6 AC-3, AC-4, AC-5, AC-9, AC-18)?
//
// Method: one real hub and one real device, both `ze start`. The device takes
// the hub's configuration by a push and records its baseline. It restarts
// unchanged after the hub's copy changed, and MUST follow. With its daemon
// stopped and its managed flag true throughout, its active version pointer is
// then written back to the version of the first push with `ze data write`: a
// route the freeze does not cover (spec-fleet-6 D-5), which changes what the
// daemon boots. Started again, it MUST keep that configuration (its active
// pointer is not moved), the hub's `show doctor` MUST list the hold under
// doctor-managed-client-diverged with the device's hash and the one the hub
// serves, and no refusal (spec-fleet-7 E-10), and a further change on the hub
// MUST be refused on the same connection.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetHoldKeepsLocalEditDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-hold-keeps-local-edit", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		// No baseline yet: the device applies what the hub serves, and the push
		// records the baseline (AC-1, AC-7).
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection")
		}); err != nil {
			return err
		}
		// The version the first push made active, which the store keeps after
		// later pushes: the earlier version the stopped edit points back to.
		served, err := s.storeKey(ctx, fleetHoldActivePointerKey)
		if err != nil {
			return err
		}

		// The hub's copy changes while the device is away. A restart alone is
		// not a local change (A-1), so the device MUST follow.
		if err := s.hubImport(ctx, fleetHoldChangedRouterID); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireFollows(ctx, daemon, fleetHoldChangedRouterID, "the restart of a device nobody changed")
		}); err != nil {
			return err
		}
		changed, err := s.storeKey(ctx, fleetHoldActivePointerKey)
		if err != nil {
			return err
		}
		// Without a second version, pointing back to the first changes nothing
		// and the hold below would prove nothing.
		if changed == served {
			return fmt.Errorf("the push of router-id %s left the active pointer on %s, the version of the first push",
				fleetHoldChangedRouterID, served)
		}

		if err := s.pointStopped(ctx, served); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireHoldsAcrossHubChange(ctx, daemon, restoreClientServedRouterID)
		}); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "OK: an unchanged managed device followed its hub across a restart, and the same device "+ //nolint:errcheck // fixture progress line
			"kept a configuration its hub did not send, told the hub, and refused a further change on the same connection, "+
			"active:", s.version(restoreClientServedRouterID), "baseline:", s.version(fleetHoldChangedRouterID))
		return nil
	})
}

// managedFleetHoldClearsOnMatchDriver answers one question: does a device that
// holds stop holding once its hub is given the configuration the device runs
// (spec-fleet-6 AC-6)?
//
// Method: the hub and the device of the driver above. The device takes the
// hub's configuration, is given another one with its daemon stopped, and
// holds. The device's configuration is then imported on the running hub as
// that client's configuration. The device's baseline MUST become its active
// hash, `show fleet status` beside it MUST report it not diverged,
// `ze doctor` beside it MUST stop reporting doctor-managed-diverged, and the
// hub's `show doctor` MUST stop listing the device's hold (spec-fleet-6
// AC-9; spec-fleet-7 E-10: the hold is the doctor-managed-client-diverged
// row), with the device still running its own configuration.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetHoldClearsOnMatchDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-hold-clears-on-match", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection")
		}); err != nil {
			return err
		}
		if err := s.editStopped(ctx, fleetHoldLocalRouterID); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireHoldEndsOnMatch(ctx, daemon)
		}); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "OK: a holding managed device stopped holding once its hub was given the configuration it runs, "+ //nolint:errcheck // fixture progress line
			"and the hub stopped listing its hold, baseline and active:", s.version(fleetHoldLocalRouterID))
		return nil
	})
}

// fleetHoldRun starts the hub, provisions the device and runs scenario. The
// hub serves the device restoreClientServedRouterID, and the device's store
// holds restoreClientBootRouterID and no baseline: it was imported, never
// fetched. With deviceServesSSH every configuration of the device carries an
// SSH listener on the reserved port, so an operator beside the running device
// reaches its daemon; without it nothing binds that port. The hub is stopped
// when scenario returns.
func fleetHoldRun(ctx context.Context, name string, args []string, deviceServesSSH bool,
	scenario func(context.Context, *fleetHoldScenario) error,
) (retErr error) {
	if len(args) != 3 {
		return errors.New("usage: " + name + " <hub-ssh-port> <hub-plugin-port> <hub-managed-port>")
	}
	sshPort, pluginPort, managedPort := args[0], args[1], args[2]
	managed, err := strconv.Atoi(managedPort)
	if err != nil {
		return fmt.Errorf("hub-managed-port %q: %w", managedPort, err)
	}

	// The runner owns the three hub ports. The device's SSH address is a port
	// of its own: reserved, so no sibling test listens on it, and released
	// after both daemons were reaped. Only a device that serves SSH binds it.
	ports, _, err := runner.ReservePorts(managed+1, 1)
	if err != nil {
		return fmt.Errorf("reserve the device's SSH port: %w", err)
	}
	defer ports.Release()
	devicePort := strconv.Itoa(ports.Start)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(cwd, "fleet-hold-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture cleanup

	hub, err := restoreClientDaemon(ctx, base, "hub", false)
	if err != nil {
		return err
	}
	// The leave and rejoin verdicts read the lines the hub's managed listener
	// writes for each session it opens and closes, so their level is stated
	// here and not left to what the runner's environment carries.
	hub.env = childEnvironment(hub.env, map[string]string{"ze.log.hub": logLevelInfo})
	s := &fleetHoldScenario{
		hub:         hub,
		managedPort: managedPort,
		hubAddress:  addrLoopback + ":" + managedPort,
		hubImporter: fleetHoldOperatorEnv(hub.env, sshPort),
	}
	if deviceServesSSH {
		s.deviceListener = restoreClientSSHListener(devicePort)
	}
	if err := caTrustImportConfig(ctx, hub, caTrustHubName+".conf", restoreClientHubConfig(sshPort, pluginPort, managedPort)); err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, hub, s.hubConfigName(), s.config(restoreClientServedRouterID)); err != nil {
		return err
	}
	if err := caTrustStart(ctx, hub); err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(hub.command, hub.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nhub output:\n%s", retErr, hub.output.String())
		}
	}()
	s.hubOperator, err = storageLiveClient(ctx, []string{sshPort})
	if err != nil {
		return err
	}

	device, err := restoreClientDaemon(ctx, base, "device", true)
	if err != nil {
		return err
	}
	// The verdicts read this subsystem's INFO and WARN lines, so its level is
	// stated here and not left to what the runner's environment carries.
	device.env = childEnvironment(device.env, map[string]string{envLogHubManaged: logLevelInfo})
	if err := caTrustImportConfig(ctx, device, caTrustClientName+".conf", s.config(restoreClientBootRouterID)); err != nil {
		return err
	}
	s.device = device
	s.deviceOperator = fleetStatusScenario{
		env: childEnvironment(device.env, map[string]string{
			"ze.ssh.host": addrLoopback,
			"ze.ssh.port": devicePort,
		}),
		dir: device.dir,
	}
	if deviceServesSSH {
		s.deviceOperator.env = fleetHoldOperatorEnv(device.env, devicePort)
	}
	return scenario(ctx, s)
}

// fleetHoldOperatorEnv is base with the SSH address and login of the daemon
// that listens on sshPort: what `ze cli` and `ze data import` beside that
// running daemon dial.
func fleetHoldOperatorEnv(base []string, sshPort string) []string {
	return overrideEnv04(base,
		"ZE_SSH_PASSWORD="+valueTestPassword,
		"ZE_SSH_HOST="+addrLoopback,
		"ZE_SSH_PORT="+sshPort,
		"ZE_SSH_USERNAME="+valueTestUser,
	)
}

// config is the device's configuration with routerID, the same bytes whether
// the hub or the device is given it.
func (s *fleetHoldScenario) config(routerID string) string {
	return s.deviceListener + restoreClientConfig(s.managedPort, routerID)
}

// version is the version hash of config(routerID): what the baseline and the
// refusal carry when the store keeps the bytes it was given.
func (s *fleetHoldScenario) version(routerID string) string {
	return managedVersionHash([]byte(s.config(routerID)))
}

// hubConfigName is the name the hub stores the device's configuration under.
func (s *fleetHoldScenario) hubConfigName() string {
	return "client-" + caTrustClientName + ".conf"
}

// withDevice runs `ze start` on the device's store, runs during, and stops the
// daemon before it returns. A failure carries the daemon's output.
func (s *fleetHoldScenario) withDevice(ctx context.Context, during func(*cachedBootDaemon) error) (retErr error) {
	daemon, err := cachedBootStart(ctx, s.device.dir, s.device.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\ndevice output:\n%s", retErr, daemon.output.String())
		}
	}()
	return during(daemon)
}

// hubImport gives the running hub config(routerID) as the device's
// configuration, with `ze data import` beside the hub, the command the guide
// names. The hub may write it whether or not the device is connected.
func (s *fleetHoldScenario) hubImport(ctx context.Context, routerID string) error {
	if err := s.hubImportBytes(ctx, []byte(s.config(routerID))); err != nil {
		return fmt.Errorf("give the running hub router-id %s for %s: %w", routerID, caTrustClientName, err)
	}
	return nil
}

// hubImportBytes gives the running hub content as the device's configuration,
// with `ze data import` beside the hub.
func (s *fleetHoldScenario) hubImportBytes(ctx context.Context, content []byte) error {
	return fleetHoldImportBeside(ctx, s.hubImporter, s.hub, s.hubConfigName(), content)
}

// fleetHoldImportBeside gives the running daemon content as the configuration
// called name, with `ze data import` in operator, the environment of an
// operator beside that daemon. The daemon owns the store, so the command sends
// the import to it.
func fleetHoldImportBeside(ctx context.Context, operator []string, daemon *caTrustDaemon, name string, content []byte) error {
	path := filepath.Join(daemon.dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return err
	}
	_, err := runCommandProcess04(ctx, operator, nil, "data", "--path", daemon.dbPath, "import", path)
	return err
}

// deviceStoredConfig prints the device's stored configuration with
// `ze data cat` beside the device, from stdout alone: the copy the guide tells
// an operator to carry to the hub (docs/guide/fleet-config.md, "When a Client
// Holds").
func (s *fleetHoldScenario) deviceStoredConfig(ctx context.Context) ([]byte, error) {
	key := "file/active/" + caTrustClientName + ".conf"
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.device.env,
		"ze", "data", "--path", s.device.dbPath, "cat", key)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("ze data cat %s exited %d: %s", key, code, stderr)
	}
	return []byte(stdout), nil
}

// editStopped gives the stopped device config(routerID) as its own
// configuration: the managed flag is set to false with a raw key write, the
// configuration is imported, which records a new active version and so changes
// what the daemon boots, and the flag is set back to true. A raw key write is
// a route the freeze does not cover (spec-fleet-6 Known Limitations), so the
// edit reaches the store whether or not the device is frozen. The active
// pointer the import left is recorded as the edit's.
func (s *fleetHoldScenario) editStopped(ctx context.Context, routerID string) error {
	return s.editStoppedTo(ctx, s.config(routerID))
}

// editStoppedTo is editStopped with the whole configuration the edit gives
// the device.
func (s *fleetHoldScenario) editStoppedTo(ctx context.Context, content string) error {
	if err := s.deviceOperator.writeKey(ctx, fleetStatusKeyManaged, "false"); err != nil {
		return err
	}
	if err := caTrustImportConfig(ctx, s.device, caTrustClientName+".conf", content); err != nil {
		return err
	}
	stamp, err := s.storeKey(ctx, fleetHoldActivePointerKey)
	if err != nil {
		return err
	}
	s.editedStamp = stamp
	return s.deviceOperator.writeKey(ctx, fleetStatusKeyManaged, valueTrue)
}

// pointStopped writes the stopped device's active version pointer back to
// stamp, an earlier version its store holds, with a raw `ze data write`. The
// managed flag is true before the write and is never touched, so the device
// is managed throughout: a raw key write is a route the freeze does not cover
// (spec-fleet-6 D-5, AC-18), and only the hold can keep what it wrote. The
// daemon reads its stored configuration through the active pointer, so the
// write changes what it boots; the plain file/active mirror keeps the bytes
// of the last push.
func (s *fleetHoldScenario) pointStopped(ctx context.Context, stamp string) error {
	flag, err := s.storeKey(ctx, fleetStatusKeyManaged)
	if err != nil {
		return err
	}
	if flag != valueTrue {
		return fmt.Errorf("the device's %s is %q before the pointer write, want %q: the edit is made on a managed device",
			fleetStatusKeyManaged, flag, valueTrue)
	}
	if err := s.deviceOperator.writeKey(ctx, fleetHoldActivePointerKey, stamp+"\n"); err != nil {
		return fmt.Errorf("write the active pointer of the stopped managed device back to %s: %w", stamp, err)
	}
	s.editedStamp = stamp
	return nil
}

// baseline reads meta/managed/base-version from the device's store.
func (s *fleetHoldScenario) baseline(ctx context.Context) (string, error) {
	return s.storeKey(ctx, fleetStatusKeyBaseline)
}

// storeKey reads one key of the device's store with `ze data cat`, from stdout
// alone: a warning the command logs on stderr is not the key.
func (s *fleetHoldScenario) storeKey(ctx context.Context, key string) (string, error) {
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.device.env,
		"ze", "data", "--path", s.device.dbPath, "cat", key)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("ze data cat %s exited %d: %s", key, code, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// requireFollows waits until the running device's active configuration is
// config(routerID) and its baseline is that configuration's hash. The baseline
// is written after the reload that made the push active, so both are polled.
func (s *fleetHoldScenario) requireFollows(ctx context.Context, daemon *cachedBootDaemon, routerID, state string) error {
	want := s.version(routerID)
	var active, baseline string
	var readErr error
	followed := cachedBootWait(ctx, daemon, fleetHoldFollowAttempts, func() bool {
		active, readErr = caTrustActiveConfig(ctx, s.device)
		if readErr != nil {
			return false
		}
		if !strings.Contains(active, "router-id "+routerID) {
			return false
		}
		baseline, readErr = s.baseline(ctx)
		if readErr != nil {
			return false
		}
		return baseline == want
	})
	if followed {
		return nil
	}
	return fmt.Errorf("%s: the device did not take router-id %s from its hub and record baseline %s, or its daemon stopped.\n"+
		"baseline: %q\nlast read error: %v\nactive config:\n%s", state, routerID, want, baseline, readErr, active)
}

// requireHold requires the state of a device that holds: the hub's
// `show doctor` lists its hold under doctor-managed-client-diverged, with the
// device's hash, that of config(local), which the device computes from the
// version its active pointer names and sends with its copy, and the hash of
// config(served), what the hub serves it now; the same report lists no
// refusal for it, because a holding acknowledgement clears the refusal row
// once the hub keeps the copy (spec-fleet-7 E-10). The device's active
// pointer is still the one the stopped edit left, so nothing was made active
// over it; and its baseline is still the hash of config(sent), the last
// configuration the hub made active on it. The pointer, and not the plain
// file/active mirror, is what the daemon runs: an edit of the pointer alone
// leaves the mirror on the last push. The hub keeps the copy when the device
// has finished the fetch, so the store is read after it.
func (s *fleetHoldScenario) requireHold(ctx context.Context, daemon *cachedBootDaemon, local, sent, served, state string) error {
	wants := []string{
		fleetHoldDivergedCode,
		fleetHoldDivergedHolds + s.version(local),
		fleetHoldDivergedServes + s.version(served),
		fleetHoldDivergedCopy,
	}
	var answer string
	listed := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		answer, _ = storageLiveRequest(ctx, s.hubOperator, "show doctor") //nolint:errcheck // a report that carries a warning may exit non-zero; the answer is what is judged
		for _, want := range wants {
			if !strings.Contains(answer, want) {
				return false
			}
		}
		return !strings.Contains(answer, fleetHoldRefusalText)
	})
	if !listed {
		return fmt.Errorf("%s: show doctor on the hub never listed %s for %s with %q and no refusal %q, or the device stopped; the last answer:\n%s",
			state, fleetHoldDivergedCode, caTrustClientName, wants[1:], fleetHoldRefusalText, answer)
	}

	pointed, err := s.storeKey(ctx, fleetHoldActivePointerKey)
	if err != nil {
		return err
	}
	if pointed != s.editedStamp {
		return fmt.Errorf("%s: the device's active pointer names %q, and its own configuration (router-id %s) is %q: "+
			"a configuration was made active over it", state, pointed, local, s.editedStamp)
	}
	baseline, err := s.baseline(ctx)
	if err != nil {
		return err
	}
	if baseline != s.version(sent) {
		return fmt.Errorf("%s: the baseline is %q, want %q, the hash of the last configuration the hub made active", state, baseline, s.version(sent))
	}
	return nil
}

// requireHoldsAcrossHubChange is the device started on its own configuration,
// config(local), while the hub holds config(fleetHoldChangedRouterID): it
// MUST hold on connecting, and hold again on the same connection when the
// hub's copy changes once more.
func (s *fleetHoldScenario) requireHoldsAcrossHubChange(ctx context.Context, daemon *cachedBootDaemon, local string) error {
	if err := s.requireHold(ctx, daemon, local, fleetHoldChangedRouterID, fleetHoldChangedRouterID, "on connecting"); err != nil {
		return err
	}
	// The device logs its hold after it acknowledged the hold the hub now shows.
	// The connections are counted once that line is written, so a connection
	// the hub refused earlier (a name it had not yet released) is in the count.
	warned := cachedBootWait(ctx, daemon, fleetHoldLogAttempts, func() bool {
		return fleetHoldLogLines(daemon.output.String(), fleetHoldWarnMessage, s.version(local)) > 0
	})
	if !warned {
		return fmt.Errorf("the device held and logged no warning %q, or its daemon stopped", fleetHoldWarnMessage)
	}
	connections := fleetHoldLogLines(daemon.output.String(), firstStartConnectedMessage, s.hubAddress)
	if connections == 0 {
		return fmt.Errorf("the device held and logged no %q for %s, so its connections cannot be counted", firstStartConnectedMessage, s.hubAddress)
	}

	if err := s.hubImport(ctx, fleetHoldFurtherRouterID); err != nil {
		return err
	}
	// The hold of the further change is the one whose offered hash is that
	// change's: no earlier line carries it.
	again := cachedBootWait(ctx, daemon, fleetHoldLogAttempts, func() bool {
		return fleetHoldLogLines(daemon.output.String(), fleetHoldWarnMessage, s.version(fleetHoldFurtherRouterID)) > 0
	})
	if !again {
		return fmt.Errorf("the device did not hold against the hub's further change (offered %s), or its daemon stopped",
			s.version(fleetHoldFurtherRouterID))
	}
	after := fleetHoldLogLines(daemon.output.String(), firstStartConnectedMessage, s.hubAddress)
	if after != connections {
		return fmt.Errorf("the device connected to its hub %d times before the further change and %d after it: "+
			"a hold MUST keep the connection", connections, after)
	}
	// The baseline stays the last configuration the hub made active, and the
	// hub now serves the further change.
	return s.requireHold(ctx, daemon, local, fleetHoldChangedRouterID, fleetHoldFurtherRouterID, "after the hub's further change")
}

// requireHoldEndsOnMatch is the device started on its own configuration while
// the hub holds another: it MUST hold, and stop holding once the hub is given
// the device's configuration.
func (s *fleetHoldScenario) requireHoldEndsOnMatch(ctx context.Context, daemon *cachedBootDaemon) error {
	if err := s.requireHold(ctx, daemon, fleetHoldLocalRouterID, restoreClientServedRouterID, restoreClientServedRouterID, "on connecting"); err != nil {
		return err
	}
	holding, err := s.deviceOperator.ask(ctx)
	if err != nil {
		return err
	}
	if !holding.Diverged {
		return fmt.Errorf("show fleet status beside a holding device does not report it diverged: %+v", holding)
	}
	// Without this row before the import, its absence after it proves nothing.
	if err := fleetDoctorRequireDiverged(ctx, s.deviceOperator.env, "a holding device", caTrustClientName,
		s.version(restoreClientServedRouterID), s.version(fleetHoldLocalRouterID)); err != nil {
		return err
	}

	// The hub is given what `ze data cat` prints beside the device, not the
	// file the fixture wrote: the copy an operator has. A copy that is not the
	// device's configuration byte for byte hashes differently, and the hold
	// would not end.
	stored, err := s.deviceStoredConfig(ctx)
	if err != nil {
		return err
	}
	if err := s.hubImportBytes(ctx, stored); err != nil {
		return fmt.Errorf("give the running hub the configuration the device runs: %w", err)
	}
	want := s.version(fleetHoldLocalRouterID)
	var status fleetStatusAnswer
	var askErr error
	settled := cachedBootWait(ctx, daemon, fleetHoldFollowAttempts, func() bool {
		status, askErr = s.deviceOperator.ask(ctx)
		if askErr != nil {
			return false
		}
		if status.Diverged {
			return false
		}
		return status.BaselineHash == want
	})
	if !settled {
		return fmt.Errorf("the device kept holding after its hub was given its configuration (version %s), or its daemon stopped.\n"+
			"last show fleet status: %+v\nlast error: %v", want, status, askErr)
	}
	if !status.Managed {
		return fmt.Errorf("the device stopped holding and is no longer managed: %+v", status)
	}
	if status.ActiveHash != status.BaselineHash {
		return fmt.Errorf("the device stopped holding with active hash %q and baseline %q, want them equal: %+v",
			status.ActiveHash, status.BaselineHash, status)
	}
	active, err := caTrustActiveConfig(ctx, s.device)
	if err != nil {
		return err
	}
	if !strings.Contains(active, "router-id "+fleetHoldLocalRouterID) {
		return fmt.Errorf("the device stopped holding and no longer runs its own configuration (router-id %s):\n%s",
			fleetHoldLocalRouterID, active)
	}
	// No control row: the same report carried the diverged row before the import.
	if err := fleetDoctorRequireSilent(ctx, s.deviceOperator.env, "a device whose hub holds its configuration", ""); err != nil {
		return err
	}
	return s.requireHubDroppedHold(ctx, daemon)
}

// requireHubDroppedHold waits until `show doctor` on the hub no longer lists
// the device's hold: no doctor-managed-client-diverged row, the code of the
// hold whose copy the hub keeps (spec-fleet-7 E-10), and no refusal text
// either. requireHold read the row in the same report before the hold ended,
// so its absence is a change and not a report that never had it. The hub
// drops its copy when the device's fetch matches what it serves, after the
// baseline the caller waited for, so the report is polled. An answer that is
// no doctor report (the command failed and printed no doctor code) proves
// nothing and is "not yet".
func (s *fleetHoldScenario) requireHubDroppedHold(ctx context.Context, daemon *cachedBootDaemon) error {
	var answer string
	var askErr error
	dropped := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		answer, askErr = storageLiveRequest(ctx, s.hubOperator, "show doctor")
		if askErr != nil {
			// A report that carries a warning may exit non-zero: it is a
			// report when it names a doctor code.
			if !strings.Contains(answer, fleetHoldDoctorCodePrefix) {
				return false
			}
		}
		if strings.Contains(answer, fleetHoldDivergedCode) {
			return false
		}
		return !strings.Contains(answer, fleetHoldRefusalText)
	})
	if dropped {
		return nil
	}
	return fmt.Errorf("show doctor on the hub kept listing the hold of %s after the hold ended, or the device stopped.\n"+
		"last error: %v\nthe last answer:\n%s", caTrustClientName, askErr, answer)
}

// fleetHoldLogLines counts the lines of the device's output that its managed
// client wrote and that hold every one of wants.
func fleetHoldLogLines(output string, wants ...string) int {
	count := 0
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, cachedBootSubsystem) {
			continue
		}
		if fleetHoldLineHolds(line, wants) {
			count++
		}
	}
	return count
}

// fleetHoldLineHolds reports whether line holds every one of wants.
func fleetHoldLineHolds(line string, wants []string) bool {
	for _, want := range wants {
		if !strings.Contains(line, want) {
			return false
		}
	}
	return true
}
