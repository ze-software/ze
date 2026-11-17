// Design: docs/architecture/config/yang-config-design.md -- publishing the modules external plugins declare
// Related: ui_fixture_pipe_alias_completion.go -- the ssh terminal and the screen read this reuses
// Related: ui_fixture_local_command_ssh.go -- localCommandLogin, and the SSH exec the daemon dispatches
// Related: plugin_fixture_11_alias.go -- startFixtureDaemon

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

func init() {
	registerFixture("ui/external-yang-session-editor", externalYANGSessionEditor)
	registerFixture("ui/external-yang-daemon-schema", externalYANGDaemonSchema)
}

// externalYANGSessionConfig is a daemon with one SSH account, the external
// plugin of the external-yang fixture in its valid mode, and a section under
// the root that plugin's module defines (internal/test/externalyang).
const externalYANGSessionConfig = `system {
    authentication {
        user ci {
            password "$PASSWORD_HASH"
            profile [ admin ]
        }
    }
    authorization {
        profile admin { run { default-action allow; } edit { default-action allow; } }
    }
}
plugin {
    external acme-monitor {
        run "le test fixture plugin/external-yang valid"
        encoder json
    }
}
acme-monitor {
    target probe-a
    interval 5
}
`

// The prompts of the daemon's session model (internal/component/cli
// model_render.go): the configuration editor and the operational prompt.
const (
	sessionEditPrompt    = "ze# "
	sessionCommandPrompt = "ze> "
)

// The texts the refusal of a value outside a leaf's range holds
// (config.ValidateLeafValue, configyang.RangeRefusal): the problem line and
// the range the module declares for interval (internal/test/externalyang).
const (
	externalYANGRangeProblem = "is outside the accepted range"
	externalYANGRange        = "1..10"
)

// externalYANGIntervalText is the description the fixture plugin's module
// gives its leaf interval (internal/test/externalyang).
const externalYANGIntervalText = "Seconds between two probes."

// externalYANGSessionEditor proves the configuration editor the DAEMON hosts
// works under the root of an external plugin, from the module that plugin
// declared when ze started. A plain ssh client under a pseudo-terminal reaches
// that editor; the headless editor runner holds the compiled-in modules alone.
//
// Five facts, each read from the screen the editor drew:
//   - after `set ` and Tab the plugin's root is offered;
//   - after `set acme-monitor ` and Tab its leaves are offered. The offer holds
//     names alone (Model.warningText adds the ze:help summary of the selected
//     name, and this module declares none);
//   - `?` after that Tab draws the description the module gives the leaf the
//     offer singles out, interval (Model.revealCandidateExplanation);
//   - `set acme-monitor interval 99` is refused, and the refusal says the value
//     is outside the accepted range and names the range 1..10;
//   - `set acme-monitor interval 7` is not refused, so the refusal above is of
//     the value and not of the path.
func externalYANGSessionEditor(ctx context.Context, args []string) error {
	daemon, cliEnv, err := startFixtureDaemon(ctx, args, externalYANGSessionConfig)
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	account, err := localCommandLogin(cliEnv)
	if err != nil {
		return err
	}
	edit := func(text, echo, keys string) (string, error) {
		return sessionScreenAfterKeys(ctx, account, sshclient.CLIModeEdit, sessionEditPrompt, text, echo, keys)
	}

	roots, err := edit("set ", "set", "\t")
	if err != nil {
		return err
	}
	if !strings.Contains(roots, "acme-monitor") {
		return fmt.Errorf("the daemon's editor did not offer the external plugin's root after 'set ':\n%s", roots)
	}

	leaves, err := edit("set acme-monitor ", "acme-monitor", "\t")
	if err != nil {
		return err
	}
	for _, want := range []string{"interval", "target"} {
		if !strings.Contains(leaves, want) {
			return fmt.Errorf("the daemon's editor did not offer %q after 'set acme-monitor ':\n%s", want, leaves)
		}
	}

	explained, err := edit("set acme-monitor ", "acme-monitor", "\t?")
	if err != nil {
		return err
	}
	if !strings.Contains(explained, externalYANGIntervalText) {
		return fmt.Errorf("the daemon's editor did not show the description of interval, %q, after Tab and ? on 'set acme-monitor ':\n%s",
			externalYANGIntervalText, explained)
	}

	refused, err := edit("set acme-monitor interval 99", "99", "\r")
	if err != nil {
		return err
	}
	for _, want := range []string{externalYANGRangeProblem, externalYANGRange} {
		if !strings.Contains(refused, want) {
			return fmt.Errorf("the daemon's editor did not refuse interval 99 with %q:\n%s", want, refused)
		}
	}

	accepted, err := edit("set acme-monitor interval 7", "7", "\r")
	if err != nil {
		return err
	}
	if strings.Contains(accepted, externalYANGRangeProblem) {
		return fmt.Errorf("the daemon's editor refused interval 7, a value inside the range:\n%s", accepted)
	}

	fmt.Println("OK: the daemon's editor offered acme-monitor and its leaves, explained interval, refused 99 with the range and took 7")
	return nil
}

// externalYANGDaemonSchema proves the schema commands the daemon serves answer
// the module an external plugin declared when ze started.
//
// Two routes reach the daemon's dispatcher. An SSH exec is one. The other is a
// line typed at the operational prompt of the daemon's session model, which is
// what a `ze cli` session is (sshclient.RunInteractive asks for command mode):
// that is the route the `ze schema` warning tells an operator to take.
// `ze cli -c` is neither: it answers these local-data commands in its own
// process, from the compiled-in modules.
//
// The daemon has no command that prints one module's YANG text (`ze schema
// show <module>` exists only as the local command). What it shows of a module
// is its name in `show schema list` and its nodes in `show yang tree config`.
func externalYANGDaemonSchema(ctx context.Context, args []string) error {
	daemon, cliEnv, err := startFixtureDaemon(ctx, args, externalYANGSessionConfig)
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	account, err := localCommandLogin(cliEnv)
	if err != nil {
		return err
	}
	creds := sshclient.Credentials{Host: "127.0.0.1", Port: account.port, Username: account.user, Auth: account.password}

	listed, err := sshclient.ExecCommand(creds, "show schema list")
	if err != nil {
		return fmt.Errorf("ssh exec of show schema list: %w", err)
	}
	if !strings.Contains(listed, "acme-monitor-conf") {
		return fmt.Errorf("show schema list in the daemon does not list the external plugin's module acme-monitor-conf:\n%s", listed)
	}

	tree, err := sshclient.ExecCommand(creds, "show yang tree config | json")
	if err != nil {
		return fmt.Errorf("ssh exec of show yang tree config: %w", err)
	}
	if err := externalYANGTreeHoldsTheModule(tree); err != nil {
		return err
	}

	// The match word is shorter than the module name, so the name on the
	// screen is the daemon's answer and never the echo of the typed line.
	typed, err := sessionScreenAfterKeys(ctx, account, sshclient.CLIModeCommand, sessionCommandPrompt,
		"show schema list | match acme-mon", "acme-mon", "\r")
	if err != nil {
		return err
	}
	if !strings.Contains(typed, "acme-monitor-conf") {
		return fmt.Errorf("show schema list typed in the daemon's session does not list acme-monitor-conf:\n%s", typed)
	}

	running, err := sshclient.ExecCommand(creds, "show config running | json")
	if err != nil {
		return fmt.Errorf("ssh exec of show config running: %w", err)
	}
	for _, want := range []string{"acme-monitor", "probe-a"} {
		if !strings.Contains(running, want) {
			return fmt.Errorf("show config running in the daemon lacks %q:\n%s", want, running)
		}
	}

	fmt.Println("OK: the daemon listed acme-monitor-conf, showed its nodes, and its running config holds the acme-monitor section")
	return nil
}

// externalYANGTreeHoldsTheModule reads the JSON answer of `show yang tree
// config` and judges that it holds the root the fixture plugin's module
// defines, with its two leaves and the description the module gives interval.
//
// The answer is searched for the node rather than read at a fixed place: the
// rows are the top-level nodes, and the envelope around them belongs to the
// pipe layer.
func externalYANGTreeHoldsTheModule(answer string) error {
	var decoded any
	if err := json.Unmarshal([]byte(answer), &decoded); err != nil {
		return fmt.Errorf("show yang tree config in the daemon did not answer JSON: %w\n%s", err, answer)
	}
	root := externalYANGTreeNode(decoded, "acme-monitor")
	if root == nil {
		return errors.New("show yang tree config in the daemon holds no node acme-monitor, the root the external plugin's module defines")
	}
	if externalYANGTreeNode(root["children"], "target") == nil {
		return fmt.Errorf("show yang tree config in the daemon holds no leaf target under acme-monitor: %v", root)
	}
	interval := externalYANGTreeNode(root["children"], "interval")
	if interval == nil {
		return fmt.Errorf("show yang tree config in the daemon holds no leaf interval under acme-monitor: %v", root)
	}
	if interval["description"] != externalYANGIntervalText {
		return fmt.Errorf("show yang tree config in the daemon gives interval the description %v, want %q",
			interval["description"], externalYANGIntervalText)
	}
	return nil
}

// externalYANGTreeNode answers the first object named name in a decoded JSON
// value, searched depth first, or nil when none is. The depth is the depth of
// the daemon's own answer, which the config schema bounds.
func externalYANGTreeNode(value any, name string) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if typed["name"] == name {
			return typed
		}
		for _, member := range typed {
			if found := externalYANGTreeNode(member, name); found != nil {
				return found
			}
		}
	case []any:
		for _, member := range typed {
			if found := externalYANGTreeNode(member, name); found != nil {
				return found
			}
		}
	}
	return nil
}
