// Design: docs/architecture/api/commands.md — `update bgp config`
// Overview: plugin_fixture_01.go — the plugin observer fixtures and their registry

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// savedPeerName is the name `update bgp config` writes a peer created at
// 192.0.2.7 under. A peer created by a command carries no name of its own, so
// the name is derived from its address (peerConfigNameFor,
// internal/component/bgp/reactor/reactor_api.go), and it cannot be the address
// itself because a peer name cannot start with a digit.
const savedPeerName = "peer-192.0.2.7"

// plugin01APIPeerSave drives `update bgp config` over the whole path an
// operator and a plugin share: the command dispatcher, the handler, the config
// editor and the file on disk.
//
// The file is read back rather than the answer trusted, because a handler that
// reports a save it never performed is the failure this test exists to catch.
func plugin01APIPeerSave(ctx context.Context, plugin *sdk.Plugin) error {
	if err := plugin01SaveCreatesAPeer(ctx, plugin); err != nil {
		return err
	}
	if err := plugin01SaveDeletesAPeer(ctx, plugin); err != nil {
		return err
	}
	if err := plugin01SaveSurvivesAReload(ctx, plugin); err != nil {
		return err
	}
	return plugin01SaveRefusesASelector(ctx, plugin)
}

// plugin01SaveCreatesAPeer holds AC-10: a peer created at runtime reaches the
// configuration file with the values the create command stated.
func plugin01SaveCreatesAPeer(ctx context.Context, plugin *sdk.Plugin) error {
	// accept false, so the peer neither listens nor shares the harness port.
	// It dials an address in TEST-NET-3 that answers nothing, which is enough:
	// the command under test reads the peer's configuration, not its session.
	//
	// local-as is stated because the configuration declares none at the bgp
	// level, and a peer with no local AS at either level is refused. It is also
	// a second leaf for the save to carry: the file must name it for the peer
	// to come back up.
	if _, err := plugin01RequireDone(ctx, plugin,
		"create bgp peer 192.0.2.7 asn 65002 local-as 65001 accept false"); err != nil {
		return err
	}

	data, err := plugin01RequireDone(ctx, plugin, "update bgp config")
	if err != nil {
		return err
	}
	if added := plugin01Strings(data["added"]); len(added) != 1 || added[0] != savedPeerName {
		return fmt.Errorf("update bgp config added %v, want [%s]", added, savedPeerName)
	}

	config, err := plugin01ReadBGPConfig()
	if err != nil {
		return err
	}
	if !strings.Contains(config, "peer "+savedPeerName) {
		return fmt.Errorf("the config file names no %s:\n%s", savedPeerName, config)
	}
	if !strings.Contains(config, "65002") {
		return fmt.Errorf("the config file carries no remote AS for %s:\n%s", savedPeerName, config)
	}
	if !strings.Contains(config, "65001") {
		return fmt.Errorf("the config file carries no local AS for %s:\n%s", savedPeerName, config)
	}
	fmt.Fprintln(os.Stderr, "OK: the created peer is in the file")
	return nil
}

// plugin01SaveDeletesAPeer holds AC-11: a peer the file declares that was
// deleted at runtime leaves the file, and the saved peer stays in it.
func plugin01SaveDeletesAPeer(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := plugin01RequireDone(ctx, plugin, "delete bgp peer 127.0.0.1"); err != nil {
		return err
	}
	if !Poll(ctx, 40, plugin01PollDelay, func() bool {
		current, status, err := plugin01DispatchMap(ctx, plugin, "show bgp peer list")
		if err != nil || status != rpc.StatusDone {
			return false
		}
		_, exists := plugin01PeerRows(current)["127.0.0.1"]
		return !exists
	}) {
		return errors.New("127.0.0.1 is still a peer after delete bgp peer")
	}

	data, err := plugin01RequireDone(ctx, plugin, "update bgp config")
	if err != nil {
		return err
	}
	if removed := plugin01Strings(data["removed"]); len(removed) != 1 || removed[0] != "peer1" {
		return fmt.Errorf("update bgp config removed %v, want [peer1]", removed)
	}

	config, err := plugin01ReadBGPConfig()
	if err != nil {
		return err
	}
	if strings.Contains(config, "peer peer1") {
		return fmt.Errorf("the config file still names peer1:\n%s", config)
	}
	if !strings.Contains(config, "peer "+savedPeerName) {
		return fmt.Errorf("the config file lost %s:\n%s", savedPeerName, config)
	}
	fmt.Fprintln(os.Stderr, "OK: the deleted peer is out of the file")
	return nil
}

// plugin01SaveSurvivesAReload holds AC-10's second half and AC-12: the file the
// save wrote is a file the daemon reads, and the running configuration and the
// file agree, so a reload keeps the created peer and does not bring the deleted
// one back.
//
// SIGHUP is the reload an operator's `config commit` performs on the peer set:
// both reconcile the running peers against the configuration.
func plugin01SaveSurvivesAReload(ctx context.Context, plugin *sdk.Plugin) error {
	// The running configuration and the file agree now, so a second save has
	// nothing to do. A peer named here would be one the two sides disagree
	// about, and the reload below would act on that disagreement.
	again, err := plugin01RequireDone(ctx, plugin, "update bgp config")
	if err != nil {
		return err
	}
	if added, removed := plugin01Strings(again["added"]), plugin01Strings(again["removed"]); len(added)+len(removed) > 0 {
		return fmt.Errorf("the running configuration and the file still disagree: added=%v removed=%v", added, removed)
	}

	// An unrelated leaf, which is AC-12's own condition: the operator commits
	// something that has nothing to do with peers. The reload re-reads the file
	// and reconciles the running peers against it, so a peer the save failed to
	// write would be torn down here.
	if err := plugin01AppendUnrelatedLeaf(); err != nil {
		return err
	}
	baseline := int64(-1)
	if !Poll(ctx, 40, plugin01PollDelay, func() bool {
		baseline = reloadGeneration(ctx, plugin)
		return baseline >= 0
	}) {
		return errors.New("before the reload: show reload-status returned no generation")
	}
	if err := plugin01SignalDaemonReload(); err != nil {
		return err
	}

	// The reload generation is the fence: it advances only after every reload
	// step ran, the store promotion included. The peer name below becomes
	// visible part-way through the reload, so a poll on the name alone lets the
	// scenario end and the daemon stop while the reload still runs, and the
	// daemon then never reports "sighup reload complete".
	generation := int64(-1)
	if !Poll(ctx, 120, plugin01PollDelay, func() bool {
		generation = reloadGeneration(ctx, plugin)
		return generation > baseline
	}) {
		return fmt.Errorf("the reload generation never advanced past %d: got %d", baseline, generation)
	}
	outcome, status, err := plugin01DispatchMap(ctx, plugin, "show reload-status")
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("show reload-status after the reload: status=%s: %w", status, err)
	}
	if outcome["last-outcome"] != "applied" {
		return fmt.Errorf("the reload of the saved file was not applied: %v", outcome)
	}

	// The NAME is what proves the reload read the file. Before it, the created
	// peer has none: a command builds a peer that carries no name of its own,
	// and `show bgp peer list` keys its rows by address either way. The name
	// the save wrote reaches the running peer only by being loaded from the
	// file, so this poll cannot pass on the state that was already there.
	if !Poll(ctx, 40, plugin01PollDelay, func() bool {
		current, status, err := plugin01DispatchMap(ctx, plugin, "show bgp peer list")
		if err != nil || status != rpc.StatusDone {
			return false
		}
		rows := plugin01PeerRows(current)
		if _, deleted := rows["127.0.0.1"]; deleted {
			return false
		}
		return plugin01Object(rows["192.0.2.7"])["name"] == savedPeerName
	}) {
		return fmt.Errorf("after the reload no peer 192.0.2.7 named %s is running, and the deleted peer must stay gone",
			savedPeerName)
	}
	fmt.Fprintln(os.Stderr, "OK: the saved file survives a reload")
	return nil
}

// plugin01AppendUnrelatedLeaf adds a system leaf to the configuration file the
// save has just written. It names no peer, so the peer set the reload
// reconciles against is the one the save wrote and nothing else.
func plugin01AppendUnrelatedLeaf() error {
	config, err := plugin01ReadBGPConfig()
	if err != nil {
		return err
	}
	config += "\nsystem {\n\tpeeringdb {\n\t\tmargin 20\n\t}\n}\n"
	if err := os.WriteFile(fileBGPConf, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", fileBGPConf, err)
	}
	return nil
}

// plugin01SignalDaemonReload sends SIGHUP to the daemon under test, whose pid
// the runner writes beside the configuration file.
func plugin01SignalDaemonReload() error {
	rawPID, err := os.ReadFile("daemon.pid")
	if err != nil {
		return fmt.Errorf("read daemon pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		return fmt.Errorf("parse daemon pid: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon process: %w", err)
	}
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("signal daemon reload: %w", err)
	}
	return nil
}

// plugin01SaveRefusesASelector holds AC-13: a word after the command is
// refused, and the refusal says why.
func plugin01SaveRefusesASelector(ctx context.Context, plugin *sdk.Plugin) error {
	data, status, err := plugin01DispatchMap(ctx, plugin, "update bgp config 192.0.2.7")
	if err != nil {
		return fmt.Errorf("update bgp config with a selector: %w", err)
	}
	if status != rpc.StatusError {
		return fmt.Errorf("update bgp config 192.0.2.7 answered %s, want an error: %v", status, data)
	}
	if refusal := fmt.Sprint(data); !strings.Contains(refusal, "no selector") {
		return fmt.Errorf("the refusal does not say the command takes no selector: %s", refusal)
	}
	fmt.Fprintln(os.Stderr, "OK: a selector is refused")
	return nil
}

// plugin01ReadBGPConfig reads the configuration file the daemon under test was
// started on, which the runner writes beside the fixture's working directory.
func plugin01ReadBGPConfig() (string, error) {
	content, err := os.ReadFile(fileBGPConf) //nolint:gosec // the path is the test runner's own config file
	if err != nil {
		return "", fmt.Errorf("read %s: %w", fileBGPConf, err)
	}
	return string(content), nil
}

// plugin01Strings reads a JSON array of strings out of a command's answer.
func plugin01Strings(value any) []string {
	array := plugin01Array(value)
	strs := make([]string, 0, len(array))
	for _, item := range array {
		text, ok := item.(string)
		if !ok {
			return nil
		}
		strs = append(strs, text)
	}
	return strs
}
