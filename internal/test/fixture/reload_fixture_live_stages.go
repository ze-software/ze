// Design: docs/functional-tests.md -- compiled fixtures a .ci drives
// Related: misc_fixture_shellports.go -- reloadSignalDriver, the one-shot reload trigger

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// liveStagesPollAttempts and liveStagesPollDelay bound one `has` or `lacks`
// step: 100 reads 200ms apart, so a step that never holds fails after 20s,
// well inside every reload test's timeout.
const (
	liveStagesPollAttempts = 100
	liveStagesPollDelay    = 200 * time.Millisecond
)

// liveStagesDriver runs a script of reloads and reads against a RUNNING daemon,
// from inside it, as an external plugin. It exists for reload tests that must
// read the daemon's state between two reloads (a BFD session listed after the
// first, gone after the second), which the one-shot reload trigger and a
// peer-driven SIGHUP cannot order.
//
// The script is one step per line; blank lines and lines starting with # are
// skipped:
//
//	await-file <path>            wait for a file in the work directory (the test peer's established-file)
//	reload <source>              copy <source> over ze-bgp.conf, then SIGHUP the daemon (daemon.pid)
//	has <command> => <needle>    poll <command> until its JSON answer contains <needle>
//	lacks <command> => <needle>  poll <command> until its JSON answer does not contain <needle>
//	say <text>                   print <text> to stderr, where a .ci await or expect reads it
//
// The plugin block that runs this fixture MUST be byte-identical in every
// config the script reloads, so the reload keeps the process alive. When the
// last step passes, Observe asks the daemon to shut down cleanly.
func liveStagesDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("reload/live-stages takes one argument: the script file")
	}
	script, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reload/live-stages: read script: %w", err)
	}
	steps := strings.Split(string(script), "\n")
	return Observe(ctx, "live-stages", sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		for number, line := range steps {
			step := strings.TrimSpace(line)
			if step == "" || strings.HasPrefix(step, "#") {
				continue
			}
			if err := runLiveStage(ctx, plugin, step); err != nil {
				return fmt.Errorf("LIVE-STAGES line %d %q: %w", number+1, step, err)
			}
		}
		return nil
	})
}

// runLiveStage runs one script step. The verb is the text before the first space.
func runLiveStage(ctx context.Context, plugin *sdk.Plugin, step string) error {
	verb, rest, _ := strings.Cut(step, " ")
	rest = strings.TrimSpace(rest)
	switch verb {
	case "await-file":
		if !waitForFile(ctx, rest, liveStagesPollAttempts, liveStagesPollDelay) {
			return errors.New("the file never appeared")
		}
		return nil
	case "reload":
		return liveStagesReload(ctx, rest)
	case "has":
		return liveStagesRead(ctx, plugin, rest, true)
	case "lacks":
		return liveStagesRead(ctx, plugin, rest, false)
	case "say":
		fmt.Fprintln(os.Stderr, rest) //nolint:errcheck // the .ci reads this line; a lost write fails its await
		return nil
	}
	return errors.New("unknown step: want await-file, reload, has, lacks or say")
}

// liveStagesReload copies source over the daemon's config file and sends it
// SIGHUP. It returns once the signal is sent: the reload runs asynchronously,
// so the next step that depends on it MUST poll (`has` or `lacks`).
func liveStagesReload(ctx context.Context, source string) error {
	if !waitForFile(ctx, "daemon.pid", liveStagesPollAttempts, liveStagesPollDelay) {
		return errors.New("daemon.pid not found")
	}
	pid, err := readPID("daemon.pid")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source) //nolint:gosec // the source is a tmpfs file the .ci declares
	if err != nil {
		return err
	}
	if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGHUP)
}

// liveStagesRead polls a command until its answer contains the needle (want
// true) or no longer does (want false). The answer is re-encoded as compact
// JSON, so a needle names a key and a value the way the JSON spells them:
// "peer":"127.0.0.1".
func liveStagesRead(ctx context.Context, plugin *sdk.Plugin, spec string, want bool) error {
	command, needle, found := strings.Cut(spec, " => ")
	if !found {
		return errors.New("want `<command> => <needle>`")
	}
	command = strings.TrimSpace(command)
	needle = strings.TrimSpace(needle)
	last := "no answer"
	held := Poll(ctx, liveStagesPollAttempts, liveStagesPollDelay, func() bool {
		status, value, err := dispatchMap(ctx, plugin, command)
		if err != nil {
			last = err.Error()
			return false
		}
		if status != statusDone {
			last = "status " + status
			return false
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			last = err.Error()
			return false
		}
		last = string(encoded)
		return strings.Contains(last, needle) == want
	})
	if held {
		return nil
	}
	if want {
		return fmt.Errorf("the answer never contained %s; last answer: %.600s", needle, last)
	}
	return fmt.Errorf("the answer still contains %s; last answer: %.600s", needle, last)
}
