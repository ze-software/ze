// Design: docs/architecture/api/commands.md -- show fleet status answers with no daemon and through a running one
// Related: misc_fixture_managed.go -- managedRunCommand and stopManagedProcess, which this driver reuses
// Related: misc_fixture_managed_cached_boot.go -- cachedBootStart and cachedBootWait, the daemon this driver starts

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// What the scenario seeds and what `show fleet status` says of it.
const (
	fleetStatusClientName   = "edge-01"
	fleetStatusClientSecret = "fleet-status-client-secret-at-least-32ch" // #nosec G101 -- a fixture secret for a hub that never answers.
	fleetStatusPluginSecret = "fleet-status-plugin-secret-at-least-32ch" // #nosec G101 -- a fixture secret for the device's own plugin transport.
	fleetStatusReadyName    = "daemon-started.ready"
	fleetStatusCommand      = "show fleet status | json compact"
	// The store keys the scenario writes with `ze data write` while the daemon
	// is stopped: the route an operator has to both.
	fleetStatusKeyManaged  = "meta/instance/managed"
	fleetStatusKeyBaseline = "meta/managed/base-version"
	// A baseline no configuration hashes to, so the device reads as diverged.
	fleetStatusOtherBaseline = "0000000000000000"
	// The words of the client, connection and holding fields
	// (internal/component/managed, cmd_fleet.go). No daemon answered: all three
	// say so. A daemon that started managed owns a hub client, which runs or is
	// stopped; one that started unmanaged started none.
	fleetStatusNoDaemon      = "no-daemon"
	fleetStatusClientRunning = "running"
	fleetStatusClientStopped = "stopped"
	fleetStatusClientNone    = "not-started"
	fleetStatusConnected     = "connected"
	fleetStatusNotConnected  = "not-connected"
	fleetStatusNotHolding    = "not-holding"
	fleetStatusPollDelay     = 250 * time.Millisecond
	// The hub allows its plugins 30 seconds to complete startup
	// (cmd/ze/hub/main.go, WaitForStartupComplete).
	fleetStatusReadyAttempts = 300
	// Each attempt is a whole `ze cli` process and an SSH session.
	fleetStatusAnswerAttempts = 60
)

// fleetStatusAnswer is the `show fleet status | json` document.
type fleetStatusAnswer struct {
	Managed          bool   `json:"managed"`
	Frozen           bool   `json:"frozen"`
	HubDeclared      bool   `json:"hub-declared"`
	HubName          string `json:"hub-name"`
	HubAddress       string `json:"hub-address"`
	BaselineRecorded bool   `json:"baseline-recorded"`
	BaselineHash     string `json:"baseline-hash"`
	ActiveRecorded   bool   `json:"active-recorded"`
	ActiveHash       string `json:"active-hash"`
	Diverged         bool   `json:"diverged"`
	DaemonAnswered   bool   `json:"daemon-answered"`
	Client           string `json:"client"`
	Connection       string `json:"connection"`
	Holding          string `json:"holding"`
}

// managedFleetStatusDriver answers one question: does `show fleet status`
// report where a device stands with its hub, from the store when no daemon
// runs and through the daemon when one does, for a managed device and for one
// that is not, and does a running daemon say what its hub client is doing?
//
// Method: `ze init --managed` and an imported config that declares a hub
// nothing listens on. The command runs as `ze cli -c`, the operator's own
// process, in four states: daemon stopped and managed, daemon running and
// managed, daemon stopped and not managed, daemon running and not managed.
// Between the first two a baseline is written that no configuration hashes
// to, so the divergence fields are read from a store that holds one. Each
// answer MUST carry the same store facts, and say whether a daemon gave it.
// The managed daemon MUST report a hub client that runs with no connection and
// holds nothing, because its hub never answers, and the unmanaged daemon MUST
// report that it started none.
func managedFleetStatusDriver(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("managed/fleet-status", flag.ContinueOnError)
	pluginPort := flags.Int("plugin-port", 0, "the device's own plugin transport port")
	hubPort := flags.Int("hub-port", 0, "the hub port the config names, which nothing listens on")
	sshPort := flags.Int("ssh-port", 0, "the port the daemon's SSH listener takes, which `ze cli` dials")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pluginPort == 0 {
		return errors.New("plugin-port is required")
	}
	if *hubPort == 0 {
		return errors.New("hub-port is required")
	}
	if *sshPort == 0 {
		return errors.New("ssh-port is required")
	}

	scenario, err := fleetStatusSeed(ctx, "managed-fleet-status-", *pluginPort, *hubPort, *sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(scenario.dir) //nolint:errcheck // fixture cleanup

	// Daemon stopped, managed, no baseline: the store alone answers.
	stopped, err := scenario.ask(ctx)
	if err != nil {
		return err
	}
	if err := scenario.requireStoreFacts(stopped, "daemon stopped, managed", true); err != nil {
		return err
	}
	if err := fleetStatusRequireNoDaemon(stopped, "daemon stopped, managed"); err != nil {
		return err
	}
	if stopped.BaselineRecorded || stopped.Diverged {
		return fmt.Errorf("daemon stopped, managed: a store with no baseline reads as holding one or as diverged: %+v", stopped)
	}

	// A baseline that differs from the active hash: the device is diverged.
	if err := scenario.writeKey(ctx, fleetStatusKeyBaseline, fleetStatusOtherBaseline); err != nil {
		return err
	}
	if err := scenario.withDaemon(ctx, func() error {
		// The daemon serves SSH before it starts its hub client, so an answer
		// given in between reports no client yet: that is "not yet".
		running, err := scenario.askDaemonUntil(ctx, "a running hub client", func(answer fleetStatusAnswer) bool {
			return answer.Client == fleetStatusClientRunning
		})
		if err != nil {
			return err
		}
		if err := scenario.requireStoreFacts(running, "daemon running, managed", true); err != nil {
			return err
		}
		// Nothing listens on the hub's port: the client retries and never
		// connects, and a client no fetch was answered for holds nothing,
		// although the store reads as diverged.
		if err := fleetStatusRequireClient(running, "daemon running, managed",
			fleetStatusClientRunning, fleetStatusNotConnected, fleetStatusNotHolding); err != nil {
			return err
		}
		if !running.BaselineRecorded || running.BaselineHash != fleetStatusOtherBaseline || !running.Diverged {
			return fmt.Errorf("daemon running, managed: a baseline other than the active hash is not reported as diverged: %+v", running)
		}
		if running.ActiveHash != stopped.ActiveHash {
			return fmt.Errorf("daemon running, managed: the daemon answers active hash %q, the store alone answered %q", running.ActiveHash, stopped.ActiveHash)
		}
		return nil
	}); err != nil {
		return err
	}

	// The same device taken out of the fleet: stopped, then running.
	if err := scenario.writeKey(ctx, fleetStatusKeyManaged, "false"); err != nil {
		return err
	}
	unmanaged, err := scenario.ask(ctx)
	if err != nil {
		return err
	}
	if err := scenario.requireStoreFacts(unmanaged, "daemon stopped, not managed", false); err != nil {
		return err
	}
	if err := fleetStatusRequireNoDaemon(unmanaged, "daemon stopped, not managed"); err != nil {
		return err
	}
	if err := scenario.withDaemon(ctx, func() error {
		running, err := scenario.askDaemon(ctx)
		if err != nil {
			return err
		}
		if err := scenario.requireStoreFacts(running, "daemon running, not managed", false); err != nil {
			return err
		}
		// A daemon that started unmanaged owns no hub client at any moment of
		// its life, so its first answer already says so.
		return fleetStatusRequireClient(running, "daemon running, not managed",
			fleetStatusClientNone, fleetStatusNotConnected, fleetStatusNotHolding)
	}); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "OK: show fleet status answered the store facts with the daemon stopped and running, managed and not, "+
		"and each running daemon reported its hub client, hub:",
		scenario.hubAddress, "active hash:", stopped.ActiveHash)
	return nil
}

// fleetStatusScenario is the seeded device of one run: the environment every
// `ze` child gets, its folder, and the hub address its config names.
type fleetStatusScenario struct {
	env        []string
	dir        string
	readyPath  string
	hubAddress string
}

// fleetStatusSeed provisions the device of one run in a new folder under the
// working directory, named after prefix: `ze init --managed`, then an imported
// config that declares a hub on hubPort, which nothing listens on, and an SSH
// listener on sshPort, the address `ze init` stores. No daemon is started. The
// caller MUST remove the dir of the scenario it answers.
func fleetStatusSeed(ctx context.Context, prefix string, pluginPort, hubPort, sshPort int) (fleetStatusScenario, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return fleetStatusScenario{}, err
	}
	dir, err := os.MkdirTemp(cwd, prefix)
	if err != nil {
		return fleetStatusScenario{}, err
	}
	readyPath := filepath.Join(dir, fleetStatusReadyName)

	// The hub the answer names MUST be the config's, and the SSH address and
	// credential MUST be the ones `ze init` stored, so every override of
	// either leaves the environment under every spelling.
	base := withoutEnv(os.Environ(), "ze.managed.server", "ze.managed.name", "ze.managed.token",
		"ze.ssh.host", "ze.ssh.port", "ze.ssh.password")
	scenario := fleetStatusScenario{
		env: childEnvironment(daemonEnvironment(base, dir, daemonStoreDisk), map[string]string{
			envNoColor:     "1",
			envReadyFile:   readyPath,
			envTestBGPPort: strconv.Itoa(pluginPort + 1000),
			// A refused loopback dial fails at once; one second bounds one that hangs.
			"ze.managed.connect.timeout": "1s",
		}),
		dir:        dir,
		readyPath:  readyPath,
		hubAddress: "127.0.0.1:" + strconv.Itoa(hubPort),
	}
	if err := scenario.seedStore(ctx, fleetStatusConfig(pluginPort, hubPort, sshPort), sshPort); err != nil {
		os.RemoveAll(dir) //nolint:errcheck,gosec // fixture cleanup of a seed that failed
		return fleetStatusScenario{}, err
	}
	return scenario, nil
}

// seedStore runs `ze init --managed` in the scenario's folder and imports
// config as the device's own.
func (s fleetStatusScenario) seedStore(ctx context.Context, config string, sshPort int) error {
	initInput := "admin\nsecret123\n127.0.0.1\n" + strconv.Itoa(sshPort) + "\n" + fleetStatusClientName + "\n"
	if _, err := managedRunCommand(ctx, s.env, s.dir, initInput, "init", "--managed"); err != nil {
		return err
	}
	configPath := filepath.Join(s.dir, fleetStatusClientName+".conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return err
	}
	_, err := managedRunCommand(ctx, s.env, s.dir, "", "data", "--path", filepath.Join(s.dir, "database"), "import", configPath)
	return err
}

// ask runs `ze cli -c "show fleet status | json compact"` once and decodes
// the document. An exit code other than zero is an error naming the output.
func (s fleetStatusScenario) ask(ctx context.Context) (fleetStatusAnswer, error) {
	code, stdout, stderr, err := runCaptured(ctx, s.env, "", "ze", "cli", "-c", fleetStatusCommand)
	if err != nil {
		return fleetStatusAnswer{}, fmt.Errorf("run ze cli -c %q: %w", fleetStatusCommand, err)
	}
	if code != 0 {
		return fleetStatusAnswer{}, fmt.Errorf("ze cli -c %q exited %d:\n%s%s", fleetStatusCommand, code, stdout, stderr)
	}
	var answer fleetStatusAnswer
	if err := json.Unmarshal([]byte(stdout), &answer); err != nil {
		return fleetStatusAnswer{}, fmt.Errorf("ze cli -c %q did not answer the status document: %w:\n%s", fleetStatusCommand, err, stdout)
	}
	return answer, nil
}

// askDaemon asks until the answer is the running daemon's. The ready file
// does not order the SSH listener, so an answer the store gave alone is "not
// yet", within a bounded number of attempts.
func (s fleetStatusScenario) askDaemon(ctx context.Context) (fleetStatusAnswer, error) {
	return s.askDaemonUntil(ctx, "any answer of its own", func(fleetStatusAnswer) bool { return true })
}

// askDaemonUntil asks until the running daemon gives an answer reached
// accepts, within a bounded number of attempts. wanted says what reached waits
// for, in the words of a failure.
func (s fleetStatusScenario) askDaemonUntil(ctx context.Context, wanted string, reached func(fleetStatusAnswer) bool) (fleetStatusAnswer, error) {
	var last fleetStatusAnswer
	var lastErr error
	answered := Poll(ctx, fleetStatusAnswerAttempts, fleetStatusPollDelay, func() bool {
		last, lastErr = s.ask(ctx)
		if lastErr != nil {
			return false
		}
		if !last.DaemonAnswered {
			return false
		}
		return reached(last)
	})
	if lastErr != nil {
		return fleetStatusAnswer{}, lastErr
	}
	if !answered {
		return fleetStatusAnswer{}, fmt.Errorf("the running daemon never answered show fleet status with %s; the last answer: %+v", wanted, last)
	}
	return last, nil
}

// fleetStatusRequireNoDaemon requires an answer the store gave alone: it says
// that no daemon answered, in each field only a running daemon fills.
func fleetStatusRequireNoDaemon(answer fleetStatusAnswer, state string) error {
	if answer.DaemonAnswered {
		return fmt.Errorf("%s: the answer says a daemon gave it: %+v", state, answer)
	}
	return fleetStatusRequireClient(answer, state, fleetStatusNoDaemon, fleetStatusNoDaemon, fleetStatusNoDaemon)
}

// fleetStatusRequireClient requires the three fields that describe the
// daemon's hub client to carry the given words.
func fleetStatusRequireClient(answer fleetStatusAnswer, state, client, connection, holding string) error {
	if answer.Client != client {
		return fmt.Errorf("%s: client is %q, want %q: %+v", state, answer.Client, client, answer)
	}
	if answer.Connection != connection {
		return fmt.Errorf("%s: connection is %q, want %q: %+v", state, answer.Connection, connection, answer)
	}
	if answer.Holding != holding {
		return fmt.Errorf("%s: holding is %q, want %q: %+v", state, answer.Holding, holding, answer)
	}
	return nil
}

// requireStoreFacts checks the fields every state of the scenario shares: the
// flag, the freeze that follows it, the hub the config declares and a recorded
// active hash.
func (s fleetStatusScenario) requireStoreFacts(answer fleetStatusAnswer, state string, managed bool) error {
	if answer.Managed != managed || answer.Frozen != managed {
		return fmt.Errorf("%s: managed=%t frozen=%t, want both %t: %+v", state, answer.Managed, answer.Frozen, managed, answer)
	}
	if !answer.HubDeclared || answer.HubName != fleetStatusClientName || answer.HubAddress != s.hubAddress {
		return fmt.Errorf("%s: the hub is not the one the config declares (%s at %s): %+v", state, fleetStatusClientName, s.hubAddress, answer)
	}
	if !answer.ActiveRecorded || answer.ActiveHash == "" {
		return fmt.Errorf("%s: the stored configuration has no active hash: %+v", state, answer)
	}
	return nil
}

// writeKey writes one store key with `ze data write` while no daemon runs.
func (s fleetStatusScenario) writeKey(ctx context.Context, key, value string) error {
	source := filepath.Join(s.dir, "key-value")
	if err := os.WriteFile(source, []byte(value), 0o600); err != nil {
		return err
	}
	_, err := managedRunCommand(ctx, s.env, s.dir, "", "data", "--path", filepath.Join(s.dir, "database"), "write", key, source)
	return err
}

// withDaemon runs `ze start` on the seeded store, waits for its ready file,
// runs during, and stops the daemon before it returns. A failure carries the
// daemon's output.
func (s fleetStatusScenario) withDaemon(ctx context.Context, during func() error) (retErr error) {
	if err := os.Remove(s.readyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	daemon, err := cachedBootStart(ctx, s.dir, s.env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nze output:\n%s", retErr, daemon.output.String())
		}
	}()
	started := cachedBootWait(ctx, daemon, fleetStatusReadyAttempts, func() bool {
		_, statErr := os.Stat(s.readyPath)
		return statErr == nil
	})
	if !started {
		return errors.New("the daemon published no ready file")
	}
	return during()
}

// fleetStatusConfig is the device's config: an SSH listener `ze cli` reaches,
// its own plugin transport, the hub it follows, and a BGP section with no peer.
func fleetStatusConfig(pluginPort, hubPort, sshPort int) string {
	var b textbuf.Buffer
	b.Str("environment {\n    ssh {\n        enabled true\n        server main {\n")
	b.Str("            ip 127.0.0.1;\n            port ").Str(strconv.Itoa(sshPort)).Str(";\n        }\n    }\n}\n")
	b.Str("plugin {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(strconv.Itoa(pluginPort)).
		Str("; secret \"").Str(fleetStatusPluginSecret).Str("\"; }\n")
	b.Str("        client ").Str(fleetStatusClientName).Str(" { host 127.0.0.1; port ").
		Str(strconv.Itoa(hubPort)).Str("; secret \"").Str(fleetStatusClientSecret).Str("\"; }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    router-id 1.1.1.1\n    session { asn { local 65000; } }\n}\n")
	return b.String()
}
