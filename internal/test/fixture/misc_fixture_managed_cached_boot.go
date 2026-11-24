// Design: docs/architecture/fleet-config.md -- a managed client starts from its cached config when its hub is unreachable
// Related: misc_fixture_managed.go -- managedRunCommand and stopManagedProcess, which this driver reuses
// Related: misc_fixture_managed_ca_trust.go -- caTrustReap, the one Wait a launched daemon needs

package fixture

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// What the cached config declares, and what the daemon logs when its managed
// client cannot reach the hub that config names.
const (
	cachedBootClientName   = "edge-01"
	cachedBootClientSecret = "cached-boot-client-secret-at-least-32ch" // #nosec G101 -- a fixture secret for a hub that never answers.
	cachedBootPluginSecret = "cached-boot-plugin-secret-at-least-32ch" // #nosec G101 -- a fixture secret for the client's own plugin transport.
	cachedBootRouterID     = "1.1.1.1"
	cachedBootReadyName    = "daemon-started.ready"
	// The WARN line RunManagedClient (internal/component/managed/client.go)
	// writes after each failed connection, and the subsystem its logger carries.
	cachedBootReconnectMessage = "connection lost, reconnecting"
	cachedBootSubsystem        = "hub.managed"
	// Two failed connections: the first proves the client tried its hub, the
	// second proves it tried again after the backoff, which is what "in the
	// background" promises.
	cachedBootAttemptsWanted = 2
	cachedBootPollDelay      = 100 * time.Millisecond
	// The hub allows its plugins 30 seconds to complete startup before it
	// refuses the start (cmd/ze/hub/main.go, WaitForStartupComplete).
	cachedBootReadyAttempts = 300
	// The second attempt follows the first by the initial backoff of one
	// second. Fifteen seconds leaves a loaded machine room.
	cachedBootRetryAttempts = 150
)

// cachedBootDaemon is the one running `ze start` of this scenario and its
// combined output, which the fixture reads while the daemon still writes it.
type cachedBootDaemon struct {
	command *exec.Cmd
	done    chan error
	output  *synchronizedBuffer
}

// managedCachedBootHubDownDriver answers one question: does a managed client
// whose hub is unreachable start from its cached config and keep trying the
// hub in the background (docs/architecture/fleet-config.md, "Startup Matrix",
// hub unreachable with a cached config)?
//
// Method: `ze init --managed`, import a config whose hub client block names a
// loopback port nothing listens on, then `ze start`. The daemon MUST publish
// its ready file, which follows the startup of everything the cached config
// declares, and its managed client MUST then report two failed connections to
// that hub address while the daemon keeps running.
func managedCachedBootHubDownDriver(ctx context.Context, args []string) (retErr error) {
	flags := flag.NewFlagSet("managed/client-cached-boot-hub-down", flag.ContinueOnError)
	pluginPort := flags.Int("plugin-port", 0, "the client's own plugin transport port")
	hubPort := flags.Int("hub-port", 0, "the hub port the cached config names, which nothing listens on")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pluginPort == 0 {
		return errors.New("plugin-port is required")
	}
	if *hubPort == 0 {
		return errors.New("hub-port is required")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp(cwd, "managed-cached-boot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	readyPath := filepath.Join(dir, cachedBootReadyName)

	// The hub address MUST come from the cached config alone. The three
	// troubleshooting overrides replace it when set (hubClientEndpoint,
	// cmd/ze/ze_core_start.go), so they leave the environment under every
	// spelling. The store stays on disk because the daemon starts on what
	// `ze init --managed` and the import seeded.
	base := withoutEnv(os.Environ(), "ze.managed.server", "ze.managed.name", "ze.managed.token")
	env := childEnvironment(daemonEnvironment(base, dir, daemonStoreDisk), map[string]string{
		envNoColor:     "1",
		envReadyFile:   readyPath,
		envTestBGPPort: strconv.Itoa(*pluginPort + 1000),
		// One second bounds each attempt, so a dial that hangs still fails
		// inside the retry budget. A refused loopback dial fails at once.
		envManagedConnectTimeout: "1s",
		// The assertion reads this subsystem's WARN line, so its level is
		// stated here and not left to what the runner's environment carries.
		envLogHubManaged: "warn",
	})

	initInput := "admin\nsecret123\n127.0.0.1\n2222\n" + cachedBootClientName + "\n"
	if _, err := managedRunCommand(ctx, env, dir, initInput, "init", "--managed"); err != nil {
		return err
	}
	configPath := filepath.Join(dir, cachedBootClientName+".conf")
	if err := os.WriteFile(configPath, []byte(cachedBootConfig(*pluginPort, *hubPort)), 0o600); err != nil {
		return err
	}
	dbPath := filepath.Join(dir, "database")
	if _, err := managedRunCommand(ctx, env, dir, "", "data", "--path", dbPath, "import", configPath); err != nil {
		return err
	}

	daemon, err := cachedBootStart(ctx, dir, env)
	if err != nil {
		return err
	}
	defer func() {
		stopManagedProcess(daemon.command, daemon.done)
		if retErr != nil {
			retErr = fmt.Errorf("%w\nze output:\n%s", retErr, daemon.output.String())
		}
	}()

	started := cachedBootWait(ctx, daemon, cachedBootReadyAttempts, func() bool {
		_, statErr := os.Stat(readyPath)
		return statErr == nil
	})
	if !started {
		return errors.New("the managed client did not start from its cached config while its hub was unreachable: it published no ready file")
	}

	hubAddress := "127.0.0.1:" + strconv.Itoa(*hubPort)
	retried := cachedBootWait(ctx, daemon, cachedBootRetryAttempts, func() bool {
		return cachedBootFailedConnections(daemon.output.String(), hubAddress) >= cachedBootAttemptsWanted
	})
	seen := cachedBootFailedConnections(daemon.output.String(), hubAddress)
	if !retried {
		return fmt.Errorf("the managed client reported %d failed connections to its hub at %s, want %d, "+
			"or the daemon stopped while its hub was unreachable", seen, hubAddress, cachedBootAttemptsWanted)
	}

	fmt.Fprintln(os.Stderr, "OK: the managed client started from its cached config with its hub unreachable and retried the hub in the background, failed connections seen:", seen)
	return nil
}

// cachedBootStart launches `ze start` on the seeded store and collects its
// output. The caller MUST stop the daemon with stopManagedProcess.
func cachedBootStart(ctx context.Context, dir string, env []string) (*cachedBootDaemon, error) {
	daemon := &cachedBootDaemon{output: &synchronizedBuffer{}, done: make(chan error, 1)}
	command := exec.CommandContext(ctx, "ze", "start") //nolint:gosec // the fixture chooses the program
	command.Dir = dir
	command.Env = env
	command.Stdout = daemon.output
	command.Stderr = daemon.output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start ze in %s: %w", dir, err)
	}
	daemon.command = command
	go caTrustReap(command, daemon.done)
	return daemon, nil
}

// cachedBootWait polls reached within the attempt budget and reports whether
// it held while the daemon was still running. A daemon that exited ends the
// wait at once and answers false: nothing it would have done can still happen.
// The reap result goes back on the channel, because stopManagedProcess reads
// the one value caTrustReap sends.
func cachedBootWait(ctx context.Context, daemon *cachedBootDaemon, attempts int, reached func() bool) bool {
	exited := false
	held := Poll(ctx, attempts, cachedBootPollDelay, func() bool {
		select {
		case reaped := <-daemon.done:
			daemon.done <- reaped
			exited = true
			return true
		default:
		}
		return reached()
	})
	if exited {
		return false
	}
	return held
}

// cachedBootFailedConnections counts the lines of output in which the managed
// client reports a failed connection to hubAddress. The address is part of the
// match because it proves the client dialed the hub its cached config names.
func cachedBootFailedConnections(output, hubAddress string) int {
	count := 0
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, cachedBootReconnectMessage) {
			continue
		}
		if !strings.Contains(line, cachedBootSubsystem) {
			continue
		}
		if !strings.Contains(line, hubAddress) {
			continue
		}
		count++
	}
	return count
}

// cachedBootConfig is the config a managed client holds after an earlier fetch:
// its own plugin transport, the hub it follows, and a BGP section with no peer.
func cachedBootConfig(pluginPort, hubPort int) string {
	var b textbuf.Buffer
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(strconv.Itoa(pluginPort)).
		Str("; secret \"").Str(cachedBootPluginSecret).Str("\"; }\n")
	b.Str("        client ").Str(cachedBootClientName).Str(" { host 127.0.0.1; port ").
		Str(strconv.Itoa(hubPort)).Str("; secret \"").Str(cachedBootClientSecret).Str("\"; }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    router-id ").Str(cachedBootRouterID).Str("\n    session { asn { local 65000; } }\n}\n")
	return b.String()
}
