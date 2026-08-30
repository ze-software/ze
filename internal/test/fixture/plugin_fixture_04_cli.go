package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/creack/pty"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func captureInterface04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	for _, check := range []struct {
		command string
		want    []string
	}{
		{"show capture interface nonexistent-iface-xyz-99", []string{"interface not found", "not available"}},
		{"show capture interface", []string{"missing interface", "not available"}},
	} {
		status, value, err := command04(ctx, p, check.command)
		if err != nil {
			return err
		}
		if status != statusError {
			return fmt.Errorf("%s: expected error, got %s", check.command, status)
		}
		text := fmt.Sprint(value)
		if !strings.Contains(text, check.want[0]) && !strings.Contains(text, check.want[1]) {
			return fmt.Errorf("%s: unexpected error message: %v", check.command, value)
		}
		fmt.Fprintf(os.Stderr, "OK: %s rejected: %v\n", check.command, value)
	}
	return nil
}

func clearDNS04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	for _, check := range []struct{ command, action string }{
		{"clear dns cache", "clear-all"},
		{"clear dns cache stats", "reset-stats"},
		{"clear dns cache record localhost", "delete-entry"},
	} {
		data, err := requireDone04(ctx, p, check.command)
		if err != nil {
			return err
		}
		if data["action"] != check.action {
			return fmt.Errorf("%s: action=%v, want %s", check.command, data["action"], check.action)
		}
		fmt.Fprintf(os.Stderr, "OK: %s -> action=%s\n", check.command, check.action)
	}
	return nil
}

func grammarActionFirst04(ctx context.Context, p *sdk.Plugin) error {
	canonical, err := requireDone04(ctx, p, "request commit start canonical-test")
	if err != nil {
		return err
	}
	if _, ok := canonical["deprecated"]; ok {
		return errors.New("canonical grammar should not have deprecated field")
	}
	for _, command := range []string{"request commit show canonical-test", "request commit rollback canonical-test"} {
		if _, err := requireDone04(ctx, p, command); err != nil {
			return err
		}
	}
	deprecated, err := requireDone04(ctx, p, "request commit deprecated-test start")
	if err != nil {
		return err
	}
	if _, ok := deprecated["deprecated"]; !ok {
		return errors.New("deprecated grammar should have deprecated field")
	}
	if _, err := requireDone04(ctx, p, "request commit rollback deprecated-test"); err != nil {
		return err
	}
	list, err := requireDone04(ctx, p, "request commit list")
	if err != nil {
		return err
	}
	if number04(list["count"]) != 0 {
		return fmt.Errorf("expected 0 commits, got %v", list["count"])
	}
	if !Poll(ctx, 100, 200*time.Millisecond, func() bool {
		data, err := requireDone04(ctx, p, "show bgp")
		return err == nil && number04(findPeer04(data)["eor-sent"]) >= 1
	}) {
		return errors.New("ze did not send the End-of-RIB to peer1 before shutdown")
	}
	fmt.Fprintln(os.Stderr, "OK: canonical and deprecated commit grammar verified")
	return nil
}

func logShow04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	data, err := requireDone04(ctx, p, "show log levels")
	if err != nil {
		return err
	}
	levels, ok := data["levels"].(map[string]any)
	if !ok || number04(data["count"]) < 1 {
		return fmt.Errorf("invalid log levels: %v", data)
	}
	fmt.Fprintf(os.Stderr, "OK: log levels returned %d subsystems\n", len(levels))
	return nil
}

func logSet04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	data, err := requireDone04(ctx, p, "show log levels")
	if err != nil {
		return err
	}
	levels, _ := data["levels"].(map[string]any)
	if len(levels) == 0 {
		return errors.New("no subsystems in log levels")
	}
	names := make([]string, 0, len(levels))
	for name := range levels {
		names = append(names, name)
	}
	sort.Strings(names)
	name := names[0]
	if _, err := requireDone04(ctx, p, "request log level "+name+" debug"); err != nil {
		return err
	}
	changed, err := requireDone04(ctx, p, "show log levels")
	if err != nil {
		return err
	}
	newLevels, _ := changed["levels"].(map[string]any)
	if newLevels[name] != logLevelDebug {
		return fmt.Errorf("%s level is %v, expected debug", name, newLevels[name])
	}
	status, _, err := command04(ctx, p, "request log level nonexistent info")
	if err != nil {
		return err
	}
	if status != statusError {
		return errors.New("expected error for unknown subsystem")
	}
	fmt.Fprintf(os.Stderr, "OK: %s level changed to debug and unknown subsystem rejected\n", name)
	return nil
}

func metricsListData04(ctx context.Context, p *sdk.Plugin) (map[string]any, []string, error) {
	data, err := requireDone04(ctx, p, "show metrics list")
	if err != nil {
		return nil, nil, err
	}
	names := stringSlice04(data["names"])
	if _, ok := data["names"].([]any); !ok {
		return data, nil, fmt.Errorf("names not a list: %v", data)
	}
	return data, names, nil
}

func requireNames04(names, required []string) error {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	var missing []string
	for _, name := range required {
		if _, ok := set[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing metrics: %v", missing)
	}
	return nil
}
func waitMetricsNames04(ctx context.Context, p *sdk.Plugin, required []string) ([]string, error) {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return nil, err
	}
	_, value, err := pollCommand04(ctx, p, 40, "show metrics list", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		names := stringSlice04(data["names"])
		return status == statusDone && requireNames04(names, required) == nil
	})
	if err != nil {
		return nil, err
	}
	data, _ := value.(map[string]any)
	names := stringSlice04(data["names"])
	if err := requireNames04(names, required); err != nil {
		return nil, err
	}
	return names, nil
}

func metricsList04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	data, names, err := metricsListData04(ctx, p)
	if err != nil {
		return err
	}
	if int(number04(data["count"])) != len(names) {
		return fmt.Errorf("count mismatch: %v != %d", data["count"], len(names))
	}
	fmt.Fprintf(os.Stderr, "OK: metrics list returned %d names\n", len(names))
	return nil
}

func metricsListDeep04(ctx context.Context, p *sdk.Plugin) error {
	required := []string{
		"ze_peer_sessions_established_total", "ze_peer_session_flaps_total", metricPeerStateTransitions,
		"ze_peer_notifications_sent_total", "ze_peer_notifications_received_total", "ze_peer_session_duration_seconds",
		"ze_bgp_connect_retry_counter", "ze_forward_congestion_events_total", "ze_forward_congestion_resumed_total",
		metricPoolUsedRatio, "ze_bgp_overflow_items", "ze_bgp_overflow_ratio", "ze_config_reloads_total",
		"ze_config_reload_errors_total", "ze_peers_added_total", "ze_peers_removed_total", "ze_wire_bytes_received_total",
		"ze_wire_bytes_sent_total", "ze_wire_read_errors_total", "ze_wire_write_errors_total",
	}
	names, err := waitMetricsNames04(ctx, p, required)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: metrics list contains all %d deep metrics (total: %d)\n", len(required), len(names))
	return nil
}

func metricsPluginHealth04(ctx context.Context, p *sdk.Plugin) error {
	required := []string{"ze_plugin_status", "ze_plugin_restarts_total", "ze_plugin_events_delivered_total"}
	if _, err := waitMetricsNames04(ctx, p, required); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: metrics list contains all 3 plugin health metrics")
	return nil
}

func metricsText04(ctx context.Context, p *sdk.Plugin) (string, error) {
	data, err := requireDone04(ctx, p, "show metrics values")
	if err != nil {
		return "", err
	}
	text, _ := data["metrics"].(string)
	if text == "" {
		return "", fmt.Errorf("empty metrics text: %v", data)
	}
	return text, nil
}

func metricsShow04(ctx context.Context, p *sdk.Plugin) error {
	text, err := metricsText04(ctx, p)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: metrics values returned %d bytes\n", len(text))
	return nil
}

func metricsDeepShow04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	required := []string{
		"ze_peer_sessions_established_total", metricPeerStateTransitions, metricPoolUsedRatio,
		"ze_config_reloads_total", "ze_peers_added_total", "ze_peers_removed_total", "ze_wire_bytes_received_total",
		"ze_wire_bytes_sent_total", "ze_uptime_seconds", "ze_peers_configured",
	}
	_, value, err := pollCommand04(ctx, p, 40, "show metrics values", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		text, _ := data["metrics"].(string)
		if status != statusDone || text == "" {
			return false
		}
		for _, name := range required {
			if !strings.Contains(text, name) {
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	data, _ := value.(map[string]any)
	text, _ := data["metrics"].(string)
	fmt.Fprintf(os.Stderr, "OK: deep instrumentation metrics present in %d bytes\n", len(text))
	return nil
}

func runCommandPeer04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	data, err := requireDone04(ctx, p, "show bgp peer list")
	if err != nil {
		return err
	}
	peers, _ := data["peers"].(map[string]any)
	if _, ok := peers["127.0.0.1"]; !ok {
		return errors.New("127.0.0.1 not in peers")
	}
	fmt.Fprintln(os.Stderr, "OK: peer list via CLI dispatch verified")
	return nil
}

func runCommand04(ctx context.Context, p *sdk.Plugin) error {
	status, value, err := command04(ctx, p, "help")
	if err != nil {
		return err
	}
	if status != statusDone || value == nil {
		return fmt.Errorf("empty help response or status=%s", status)
	}
	encoded, _ := json.Marshal(value)
	if !strings.Contains(string(encoded), "peer") {
		return errors.New(`"peer" not found in help data`)
	}
	fmt.Fprintln(os.Stderr, "OK: help command dispatched with peer commands")
	return nil
}

func summaryShow04(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	data, err := requireDone04(ctx, p, "show bgp")
	if err != nil {
		return err
	}
	if _, ok := data["peers-configured"]; !ok {
		return errors.New("no peers-configured in summary")
	}
	fmt.Fprintf(os.Stderr, "OK: summary via CLI dispatch has %v peer(s)\n", data["peers-configured"])
	return nil
}

func commitRejectPlugin04(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	p, err := newObserver("fixture-cli-reject-04")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // fixture teardown
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root == namespaceBGP && strings.Contains(section.Data, "2.2.2.2") {
				fmt.Fprintln(os.Stderr, "OK: rejecting candidate router-id 2.2.2.2")
				return errors.New("reject router-id 2.2.2.2")
			}
		}
		return nil
	})
	return p.Run(ctx, sdk.Registration{WantsConfig: []string{namespaceBGP}})
}

func cliCommitDriver04(reject bool) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return errors.New("usage: commit driver <ssh-port>")
		}
		clientDir, err := filepath.Abs("client-db")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(clientDir, 0o750); err != nil {
			return err
		}
		env := overrideEnv04(os.Environ(),
			"ZE_CONFIG_DIR="+clientDir,
			"ZE_SSH_PASSWORD=testpass",
			"NO_COLOR=1",
			"TERM=xterm",
		)
		initInput := fmt.Sprintf("admin\ntestpass\n127.0.0.1\n%s\n\n", args[0])
		if _, err := runCommandProcess04(ctx, env, strings.NewReader(initInput), "ze", "init"); err != nil {
			return err
		}
		var last string
		if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
			output, err := runCommandProcess04(ctx, env, nil, "ze", "cli", "-c", "show version")
			last = output
			return err == nil
		}) {
			return fmt.Errorf("daemon did not become reachable: %s", last)
		}
		config := "cli-commit.conf"
		if reject {
			config = "cli-commit-reject.conf"
		}
		transcript, err := driveEditor04(ctx, env, config, reject)
		if err != nil {
			return err
		}
		if reject {
			if !strings.Contains(transcript, "commit failed:") || strings.Contains(transcript, "and reloaded") {
				return fmt.Errorf("config editor did not report transactional commit failure\n%s", transcript)
			}
		} else if !strings.Contains(transcript, "and reloaded") || strings.Contains(transcript, "commit failed:") {
			return fmt.Errorf("config editor did not report transactional commit success\n%s", transcript)
		}
		if _, err := runCommandProcess04(ctx, env, nil, "ze", "cli", "-c", "show version"); err != nil {
			return err
		}
		if reject {
			fmt.Fprintln(os.Stderr, "OK: CLI commit rejected candidate and daemon stayed reachable")
		} else {
			fmt.Fprintln(os.Stderr, "OK: CLI commit promoted candidate and daemon applied router-id")
		}
		return nil
	}
}

func runCommandProcess04(ctx context.Context, env []string, input io.Reader, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	cmd.Stdin = input
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if err != nil {
		return output.String(), fmt.Errorf("%s %v: %w\n%s", name, args, err, output.String())
	}
	return output.String(), nil
}

func overrideEnv04(base []string, values ...string) []string {
	keys := make(map[string]struct{}, len(values))
	for _, value := range values {
		key, _, _ := strings.Cut(value, "=")
		keys[key] = struct{}{}
	}
	env := make([]string, 0, len(base)+len(values))
	for _, value := range base {
		key, _, _ := strings.Cut(value, "=")
		if _, replaced := keys[key]; !replaced {
			env = append(env, value)
		}
	}
	return append(env, values...)
}

func driveEditor04(ctx context.Context, env []string, config string, reject bool) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "config", "edit", "-f", config) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 120})
	if err != nil {
		return "", err
	}
	defer terminal.Close() //nolint:errcheck // fixture teardown
	transcript, err := readPTYUntil04(terminal, nil, 20*time.Second, false, "\x1b[?1049h", "╭")
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return transcript, fmt.Errorf("config editor did not draw its first frame: %w", err)
	}
	send := func(command string, needles ...string) error {
		if _, err := terminal.WriteString(command + "\r"); err != nil {
			return fmt.Errorf("config editor closed before command %q: %w", command, err)
		}
		var chunk string
		chunk, err = readPTYUntil04(terminal, []byte(transcript), 20*time.Second, false, needles...)
		transcript = chunk
		return err
	}
	if err := send("set bgp router-id 2.2.2.2", "2.2.2.2"); err != nil {
		return transcript, err
	}
	if err := send("commit", "Configuration committed", "commit failed:", "commit blocked:"); err != nil {
		return transcript, err
	}
	if reject {
		if !strings.Contains(transcript, "commit failed:") {
			return transcript, errors.New("config editor did not reject the commit")
		}
		if err := send("errors", "reject router-id", "commit failed:"); err != nil {
			return transcript, err
		}
		if err := send("discard all", "discard", "Discard"); err != nil {
			return transcript, err
		}
	} else if strings.Contains(transcript, "commit failed:") || strings.Contains(transcript, "commit blocked:") {
		return transcript, errors.New("config editor rejected the commit")
	}
	if _, err := terminal.WriteString("quit\r"); err != nil {
		return transcript, err
	}
	transcript, _ = readPTYUntil04(terminal, []byte(transcript), 20*time.Second, true)
	if err := cmd.Wait(); err != nil {
		return transcript, fmt.Errorf("config editor exited: %w\n%s", err, transcript)
	}
	return transcript, nil
}

func readPTYUntil04(file *os.File, initial []byte, timeout time.Duration, eofOK bool, needles ...string) (string, error) {
	buf := append([]byte(nil), initial...)
	deadline := time.Now().Add(timeout)
	for {
		for _, needle := range needles {
			if bytes.Contains(buf, []byte(needle)) {
				return string(buf), nil
			}
		}
		if time.Now().After(deadline) {
			return string(buf), errors.New("output deadline expired")
		}
		_ = file.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		chunk := make([]byte, 65536)
		n, err := file.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			}
			if eofOK && (errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed)) {
				return string(buf), nil
			}
			var opErr *os.PathError
			if eofOK && errors.As(err, &opErr) {
				return string(buf), nil
			}
			return string(buf), err
		}
	}
}
