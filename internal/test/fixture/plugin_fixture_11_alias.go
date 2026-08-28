package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/pipe-alias", aliasDriver(aliasCaseBasic))
	Register("plugin/pipe-alias/provider", aliasProvider(aliasCaseBasic))
	Register("plugin/pipe-alias-help", aliasDriver(aliasCaseHelp))
	Register("plugin/pipe-alias-help/provider", aliasProvider(aliasCaseHelp))
	Register("plugin/pipe-alias-namespaced", aliasDriver(aliasCaseNamespaced))
	Register("plugin/pipe-alias-namespaced/provider", aliasProvider(aliasCaseNamespaced))
	Register("plugin/pipe-alias-collision", aliasDriver(aliasCaseCollision))
	Register("plugin/pipe-alias-collision/provider", aliasProvider(aliasCaseCollision))
	Register("plugin/shape-declaration-refused", aliasDriver(aliasCaseShape))
	Register("plugin/shape-declaration-refused/provider", aliasProvider(aliasCaseShape))
}

type aliasCase int

const (
	aliasCaseBasic aliasCase = iota
	aliasCaseHelp
	aliasCaseNamespaced
	aliasCaseCollision
	aliasCaseShape
)

func aliasProvider(which aliasCase) Driver {
	return func(ctx context.Context, _ []string) error {
		name := map[aliasCase]string{
			aliasCaseBasic:      "pipe-alias-provider",
			aliasCaseHelp:       "pipe-help-provider",
			aliasCaseNamespaced: "pipe-alias-scope-provider",
			aliasCaseCollision:  "pipe-alias-thief",
			aliasCaseShape:      "shape-typo",
		}[which]
		plugin, err := newObserver(name)
		if err != nil {
			return err
		}
		defer plugin.Close() //nolint:errcheck
		var registration sdk.Registration
		switch which {
		case aliasCaseBasic:
			registration = sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: "show pipealias counters"}},
				Pipes: []sdk.PipeDecl{{
					Command:     "show pipealias counters",
					Name:        "totals",
					Expansion:   "display kind vrp-count",
					Description: "The counters alone",
				}},
			}
		case aliasCaseHelp:
			registration = sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: "show pipehelp counters"}},
				Pipes: []sdk.PipeDecl{{
					Command:     "show pipehelp counters",
					Name:        "totals",
					Expansion:   "display kind vrp-count",
					Description: "The counters alone",
				}},
			}
		case aliasCaseNamespaced:
			registration = sdk.Registration{
				Commands: []sdk.CommandDecl{
					{Name: "show nsalias counters"},
					{Name: "show nsalias counters rows"},
				},
				Pipes: []sdk.PipeDecl{{
					Command:     "show nsalias counters",
					Name:        "totals",
					Expansion:   "display kind vrp-count",
					Description: "The counters alone",
				}},
			}
		case aliasCaseCollision:
			registration = sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: "show bgp"}},
				Pipes:    []sdk.PipeDecl{{Command: "show bgp", Name: "summary", Expansion: "display vrp-count", Description: "The name show bgp already answers to"}},
			}
		case aliasCaseShape:
			registration = sdk.Registration{Commands: []sdk.CommandDecl{{Name: "show shape typo", Shape: "table", Columns: []string{"address", "state"}}}}
		}
		plugin.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
			switch command {
			case "show pipealias counters":
				return "done", pipeAnswer("pipe-alias-probe", "servers"), nil
			case "show pipehelp counters":
				return "done", map[string]any{
					"kind":      "pipe-help-probe",
					"vrp-count": 7,
					"servers": []map[string]any{
						{"address": "192.0.2.101", "state": "established"},
					},
				}, nil
			case "show nsalias counters":
				return "done", pipeAnswer("pipe-alias-scope-probe", "rows"), nil
			case "show nsalias counters rows":
				return "done", map[string]any{"rows": pipeRows()}, nil
			default:
				return "error", map[string]any{"error": "unknown: " + command}, nil
			}
		})
		return plugin.Run(ctx, registration)
	}
}

func pipeRows() []map[string]any {
	return []map[string]any{
		{"address": "192.0.2.101", "state": "established"},
		{"address": "192.0.2.102", "state": "connecting"},
	}
}

func pipeAnswer(kind, rowsKey string) map[string]any {
	return map[string]any{"kind": kind, "vrp-count": 7, rowsKey: pipeRows()}
}

type fixtureDaemon struct {
	command *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	done    chan struct{}
	workdir string
}

func startFixtureDaemon(ctx context.Context, config string) (*fixtureDaemon, []string, error) {
	code, hash, passwordErr, err := runCaptured(ctx, os.Environ(), "secret\n", "ze", "passwd")
	if err != nil || code != 0 {
		return nil, nil, fmt.Errorf("ze passwd exit=%d: %v %s", code, err, passwordErr)
	}
	config = strings.ReplaceAll(config, "$PASSWORD_HASH", strings.TrimSpace(hash))
	workdir, err := os.MkdirTemp("", "ze-plugin-alias-")
	if err != nil {
		return nil, nil, err
	}
	keepWorkdir := false
	defer func() {
		if !keepWorkdir {
			_ = os.RemoveAll(workdir)
		}
	}()
	configPath := filepath.Join(workdir, "ze.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return nil, nil, err
	}
	sshAddr := filepath.Join(workdir, "ssh.addr")
	ready := filepath.Join(workdir, "ready")
	env := append(os.Environ(),
		"ZE_SSH_EPHEMERAL="+sshAddr,
		"ZE_READY_FILE="+ready,
		"ZE_CONFIG_DIR="+workdir,
		"ze_test_bgp_port="+strconv.Itoa(12000+os.Getpid()%40000),
	)
	daemon := &fixtureDaemon{
		command: exec.CommandContext(ctx, "ze", "-f", configPath),
		done:    make(chan struct{}),
		workdir: workdir,
	}
	daemon.command.Env = env
	daemon.command.Stdout = &daemon.stdout
	daemon.command.Stderr = &daemon.stderr
	if err := daemon.command.Start(); err != nil {
		return nil, nil, err
	}
	go func() {
		_ = daemon.command.Wait()
		close(daemon.done)
	}()
	readyOK := false
	for range 300 {
		_, addressErr := os.Stat(sshAddr)
		_, readyErr := os.Stat(ready)
		if addressErr == nil && readyErr == nil {
			readyOK = true
			break
		}
		select {
		case <-daemon.done:
			return nil, nil, fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		case <-ctx.Done():
			_ = daemon.stop()
			return nil, nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !readyOK {
		_ = daemon.stop()
		return nil, nil, fmt.Errorf("daemon did not become ready\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
	}
	address, err := os.ReadFile(sshAddr)
	if err != nil {
		_ = daemon.stop()
		return nil, nil, err
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(string(address)))
	if err != nil {
		_ = daemon.stop()
		return nil, nil, fmt.Errorf("bad SSH address %q: %w", address, err)
	}
	cliEnv := append(os.Environ(), "ZE_SSH_HOST="+host, "ZE_SSH_PORT="+port, "ZE_SSH_USERNAME=ci", "ZE_SSH_PASSWORD=secret", "ZE_CONFIG_DIR="+workdir)
	keepWorkdir = true
	return daemon, cliEnv, nil
}

func (daemon *fixtureDaemon) stop() error {
	defer os.RemoveAll(daemon.workdir) //nolint:errcheck // isolated fixture state is best-effort cleanup
	if daemon.command.Process == nil {
		return nil
	}
	_ = daemon.command.Process.Signal(syscall.SIGTERM)
	select {
	case <-daemon.done:
		return nil
	case <-time.After(5 * time.Second):
		_ = daemon.command.Process.Kill()
		<-daemon.done
		return nil
	}
}

func cli11(ctx context.Context, env []string, command string) (int, string, string, error) {
	return runCaptured(ctx, env, "", "ze", "cli", "-c", command)
}

func require11(condition bool, format string, args ...any) error {
	if condition {
		return nil
	}
	return fmt.Errorf(format, args...)
}

func aliasDriver(which aliasCase) Driver {
	return func(ctx context.Context, _ []string) error {
		config := aliasConfig(which)
		daemon, cliEnv, err := startFixtureDaemon(ctx, config)
		if err != nil {
			return err
		}
		defer daemon.stop() //nolint:errcheck

		run := func(command string) (int, string, string, error) { return cli11(ctx, cliEnv, command) }
		cli := func(command string) (string, error) {
			code, out, stderr, runErr := run(command)
			if runErr != nil || code != 0 {
				return "", fmt.Errorf("%s exit=%d: %v %s%s", command, code, runErr, out, stderr)
			}
			return out, nil
		}

		switch which {
		case aliasCaseBasic:
			return runBasicAlias(ctx, cli, run)
		case aliasCaseHelp:
			return runAliasHelp(ctx, cli)
		case aliasCaseNamespaced:
			return runNamespacedAlias(ctx, cli, run)
		case aliasCaseCollision:
			if err := runCollisionAlias(ctx, cli); err != nil {
				return err
			}
			if err := daemon.stop(); err != nil {
				return err
			}
			for _, word := range []string{"pipe alias refused", "pipe-alias-thief", "summary", "show bgp", "already registered"} {
				if !strings.Contains(daemon.stderr.String(), word) {
					return fmt.Errorf("daemon log does not name %q: %s", word, daemon.stderr.String())
				}
			}
			fmt.Println("OK")
			return nil
		case aliasCaseShape:
			code, out, stderr, runErr := run("show bgp | json")
			if err := require11(runErr == nil && code == 0 && strings.Contains(out, "10.255.255.254"), "daemon stopped answering after refusal: %v %s%s", runErr, out, stderr); err != nil {
				return err
			}
			code, out, stderr, runErr = run("show shape typo")
			if err := require11(runErr == nil && code != 0, "refused plugin still serves command: %s%s", out, stderr); err != nil {
				return err
			}
			if err := daemon.stop(); err != nil {
				return err
			}
			for _, word := range []string{"answer shape", "shape-typo", "show shape typo", "table", "doc, map or tab"} {
				if !strings.Contains(daemon.stderr.String(), word) {
					return fmt.Errorf("daemon log does not name %q: %s", word, daemon.stderr.String())
				}
			}
			fmt.Println("OK")
			return nil
		default:
			return errors.New("unknown alias fixture")
		}
	}
}

func runBasicAlias(ctx context.Context, cli func(string) (string, error), run func(string) (int, string, string, error)) error {
	var whole string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		var err error
		whole, err = cli("show pipealias counters | json")
		return err == nil && strings.Contains(whole, "vrp-count")
	}) {
		return errors.New("plugin command never answered")
	}
	if !strings.Contains(whole, "192.0.2.101") {
		return fmt.Errorf("rows missing from whole answer: %q", whole)
	}
	only, err := cli("show pipealias counters | totals | json")
	if err != nil {
		return err
	}
	if !strings.Contains(only, "vrp-count") || !strings.Contains(only, "pipe-alias-probe") || strings.Contains(only, "192.0.2.101") || strings.Contains(only, "192.0.2.102") {
		return fmt.Errorf("bad totals answer: %q", only)
	}
	_, out, stderr, _ := run("system command list | totals | json")
	if !strings.Contains(out+stderr, "unknown pipe operator: totals") {
		return fmt.Errorf("totals reached unrelated command: %q %q", out, stderr)
	}
	fmt.Println("OK")
	return nil
}

func runAliasHelp(ctx context.Context, cli func(string) (string, error)) error {
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		out, err := cli("show pipehelp counters | json")
		return err == nil && strings.Contains(out, "vrp-count")
	}) {
		return errors.New("plugin command never answered")
	}
	helpFor := func(name string) (map[string]any, error) {
		out, err := cli("show command help \"" + name + "\" | json")
		if err != nil {
			return nil, err
		}
		var doc map[string]any
		err = json.Unmarshal([]byte(out), &doc)
		return doc, err
	}
	declared, err := helpFor("show pipehelp counters")
	if err != nil {
		return err
	}
	if declared["command"] != "show pipehelp counters" {
		return fmt.Errorf("help does not name plugin command: %v", declared)
	}
	aliases := aliasMap(declared)
	totals := aliases["totals"]
	if totals == nil || totals["description"] != "The counters alone" || totals["expansion"] != "display kind vrp-count" {
		return fmt.Errorf("bad declared alias help: %v", declared)
	}
	builtin, err := helpFor("show bgp")
	if err != nil {
		return err
	}
	carried := aliasMap(builtin)
	if carried["summary"] == nil || carried["peers"] == nil || carried["summary"]["expansion"] == "" {
		return fmt.Errorf("bad built-in alias help: %v", builtin)
	}
	bare, err := helpFor("show command list")
	if err != nil {
		return err
	}
	if _, exists := bare["pipe-aliases"]; exists {
		return fmt.Errorf("bare command reports aliases: %v", bare)
	}
	fmt.Println("OK")
	return nil
}

func aliasMap(doc map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any)
	rows, _ := doc["pipe-aliases"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		name, _ := row["name"].(string)
		result[name] = row
	}
	return result
}

func runNamespacedAlias(ctx context.Context, cli func(string) (string, error), run func(string) (int, string, string, error)) error {
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		out, err := cli("show nsalias counters | json")
		return err == nil && strings.Contains(out, "vrp-count")
	}) {
		return errors.New("plugin parent command never answered")
	}
	leaf, err := cli("show nsalias counters rows | json")
	if err != nil || !strings.Contains(leaf, "192.0.2.101") {
		return fmt.Errorf("leaf command answers no rows: %q: %w", leaf, err)
	}
	only, err := cli("show nsalias counters | totals | json")
	if err != nil || !strings.Contains(only, "vrp-count") || strings.Contains(only, "192.0.2.101") {
		return fmt.Errorf("bad parent totals: %q: %w", only, err)
	}
	for _, command := range []string{"show nsalias counters rows | totals | json", "system command list | totals | json"} {
		_, out, stderr, _ := run(command)
		if !strings.Contains(out+stderr, "unknown pipe operator: totals") {
			return fmt.Errorf("totals reached %q: %q %q", command, out, stderr)
		}
	}
	fmt.Println("OK")
	return nil
}

func runCollisionAlias(_ context.Context, cli func(string) (string, error)) error {
	whole, err := cli("show bgp | json")
	if err != nil {
		return err
	}
	if !strings.Contains(whole, "192.0.2.1") || !strings.Contains(whole, "10.255.255.254") {
		return fmt.Errorf("bad whole BGP answer: %q", whole)
	}
	only, err := cli("show bgp | summary | json")
	if err != nil || !strings.Contains(only, "10.255.255.254") || strings.Contains(only, "192.0.2.1") {
		return fmt.Errorf("bad summary answer: %q: %w", only, err)
	}
	rows, err := cli("show bgp | peers | json")
	if err != nil || !strings.Contains(rows, "192.0.2.1") || strings.Contains(rows, "10.255.255.254") {
		return fmt.Errorf("bad peers answer: %q: %w", rows, err)
	}
	return nil
}

func aliasConfig(which aliasCase) string {
	provider := map[aliasCase]string{
		aliasCaseBasic:      "plugin/pipe-alias/provider",
		aliasCaseHelp:       "plugin/pipe-alias-help/provider",
		aliasCaseNamespaced: "plugin/pipe-alias-namespaced/provider",
		aliasCaseCollision:  "plugin/pipe-alias-collision/provider",
		aliasCaseShape:      "plugin/shape-declaration-refused/provider",
	}[which]
	pluginName := map[aliasCase]string{aliasCaseBasic: "pipe-alias-provider", aliasCaseHelp: "pipe-help-provider", aliasCaseNamespaced: "pipe-alias-scope-provider", aliasCaseCollision: "pipe-alias-thief", aliasCaseShape: "shape-typo"}[which]
	bgp := ""
	if which == aliasCaseHelp {
		bgp = "bgp { router-id 192.0.2.254; session { asn { local 65000; } } }\n"
	}
	if which == aliasCaseCollision || which == aliasCaseShape {
		bgp = `bgp {
    router-id 10.255.255.254
    session { asn { local 65000 } }
    group transit {
        peer peer1 {
            connection { remote { ip 192.0.2.1 } local { ip 127.0.0.1 } }
            session { asn { remote 65001 } }
        }
    }
}
`
	}
	return bgp + `plugin {
    external ` + pluginName + ` {
        run "ze-test fixture ` + provider + `"
        encoder json
    }
}
system {
    authentication {
        user ci {
            password "$PASSWORD_HASH"
            profile [ admin ]
        }
    }
}
`
}
