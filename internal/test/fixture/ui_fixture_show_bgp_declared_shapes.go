package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/show-bgp-declared-shapes", uiDriver(showBGPDeclaredShapes))
}

type showBGPDeclaredShapesResult struct {
	code   int
	stdout string
	stderr string
}

type showBGPDeclaredShapesDaemon struct {
	cmd     *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	done    chan error
	exited  bool
	waitErr error
}

func showBGPDeclaredShapes(ctx context.Context) (retErr error) {
	workDir, err := os.MkdirTemp("", "ze-ui-show-bgp-declared-shapes-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	passwd, err := showBGPDeclaredShapesRun(ctx, workDir, os.Environ(), "secret\n", "ze", "passwd")
	if err != nil {
		return fmt.Errorf("run ze passwd: %w", err)
	}
	if passwd.code != 0 {
		return fmt.Errorf("ze passwd exit=%d: %s%s", passwd.code, passwd.stdout, passwd.stderr)
	}
	passwordHash := strings.TrimSpace(passwd.stdout)

	config := fmt.Sprintf(`bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
    peer peer2 {
        connection {
            remote {
                ip 192.0.2.2
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                remote 65002
            }
        }
    }
}

system {
    authentication {
        user ci {
            password "%s"
            profile [ admin ]
        }
    }
}
`, passwordHash)
	if err := os.WriteFile(filepath.Join(workDir, "shapes.conf"), []byte(config), 0o666); err != nil {
		return fmt.Errorf("write shapes.conf: %w", err)
	}

	sshAddrPath := filepath.Join(workDir, "ssh.addr")
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	readyPath := filepath.Join(workDir, "ready")
	daemonEnv := showBGPDeclaredShapesSetEnv(os.Environ(),
		"ZE_SSH_EPHEMERAL", sshAddrPath,
		"ZE_READY_FILE", readyPath,
		"ZE_CONFIG_DIR", workDir,
		"ze_test_bgp_port", strconv.Itoa(bgpPort),
	)

	daemon, err := showBGPDeclaredShapesStartDaemon(ctx, workDir, daemonEnv)
	if err != nil {
		return err
	}
	cleanupNeeded := true
	defer func() {
		if !cleanupNeeded {
			return
		}
		if cleanupErr := daemon.stop(); cleanupErr != nil {
			if retErr == nil {
				retErr = cleanupErr
			} else {
				retErr = fmt.Errorf("%v; cleanup failed: %w", retErr, cleanupErr)
			}
		}
	}()

	ready, err := showBGPDeclaredShapesPoll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		select {
		case waitErr := <-daemon.done:
			daemon.exited = true
			daemon.waitErr = waitErr
			return false, fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		default:
		}
		_, addrErr := os.Stat(sshAddrPath)
		_, readyErr := os.Stat(readyPath)
		return addrErr == nil && readyErr == nil, nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddrPath)
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]
	cliEnv := showBGPDeclaredShapesSetEnv(os.Environ(),
		"ZE_SSH_HOST", host,
		"ZE_SSH_PORT", port,
		"ZE_SSH_USERNAME", "ci",
		"ZE_SSH_PASSWORD", "secret",
		"ZE_CONFIG_DIR", workDir,
	)

	cli := func(command string) (string, error) {
		result, runErr := showBGPDeclaredShapesRun(ctx, workDir, cliEnv, "", "ze", "cli", "-c", command)
		if runErr != nil {
			return "", fmt.Errorf("%s: %w", command, runErr)
		}
		if result.code != 0 {
			return "", fmt.Errorf("%s exit=%d: %s%s", command, result.code, result.stdout, result.stderr)
		}
		return result.stdout, nil
	}
	refusal := func(command string) (string, error) {
		result, runErr := showBGPDeclaredShapesRun(ctx, workDir, cliEnv, "", "ze", "cli", "-c", command)
		if runErr != nil {
			return "", fmt.Errorf("%s: %w", command, runErr)
		}
		return result.stdout + result.stderr, nil
	}
	namesThe := func(text, command, operator, reason string) error {
		if !strings.Contains(text, "cannot apply here") {
			return fmt.Errorf("%s: refused AFTER dispatch, or not refused: the declaration is not what answered: %q", command, text)
		}
		if !strings.Contains(text, operator) {
			return fmt.Errorf("%s: the refusal does not name `%s`: %q", command, operator, text)
		}
		if !strings.Contains(text, reason) {
			return fmt.Errorf("%s: the refusal does not say %q: %q", command, reason, text)
		}
		return nil
	}

	// AC-11: health is a row answer, and both row operators must run.
	countOutput, err := cli("show bgp health | count")
	if err != nil {
		return err
	}
	counted, err := showBGPDeclaredShapesDecode(countOutput)
	if err != nil {
		return fmt.Errorf("decode `show bgp health | count`: %w", err)
	}
	count, err := showBGPDeclaredShapesCount(counted)
	if err != nil {
		return fmt.Errorf("decode `show bgp health | count` count: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("`show bgp health | count` = %#v, want 2", counted)
	}

	matched, err := cli("show bgp health | match 192.0.2.1 | count")
	if err != nil {
		return err
	}
	matchedValue, err := showBGPDeclaredShapesDecode(matched)
	if err != nil {
		return fmt.Errorf("decode `show bgp health | match 192.0.2.1 | count`: %w", err)
	}
	matchedCount, err := showBGPDeclaredShapesCount(matchedValue)
	if err != nil {
		return fmt.Errorf("decode matched count: %w", err)
	}
	if matchedCount != 1 {
		return fmt.Errorf("`show bgp health | match 192.0.2.1 | count` = %q, want 1", matched)
	}

	// AC-12: resolve must decorate every returned peer address.
	resolvedOutput, err := cli("show bgp health | resolve | json compact")
	if err != nil {
		return err
	}
	resolved, err := showBGPDeclaredShapesDecode(resolvedOutput)
	if err != nil {
		return fmt.Errorf("decode `show bgp health | resolve`: %w", err)
	}
	var rows []any
	switch value := resolved.(type) {
	case map[string]any:
		rows, _ = value["peers"].([]any)
	case []any:
		rows = value
	}
	if len(rows) == 0 {
		return fmt.Errorf("`show bgp health | resolve` answered no rows: %#v", resolved)
	}
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("`| resolve` returned a non-object row: %#v", item)
		}
		if _, ok := row["peer-name"]; !ok {
			return fmt.Errorf("`| resolve` did not decorate \"peer\": %#v", row)
		}
	}

	// AC-16: these commands declare one document and reject row operators
	// before dispatch, regardless of the current peer state.
	refusals := []struct {
		command  string
		operator string
		reason   string
	}{
		{"show bgp rib status | count", "count", "one document"},
		{"show bgp rib status | first 2", "first", "one document"},
		{"show bgp rib best status | count", "count", "one document"},
		{"show bgp rib rpf | count", "count", "one document"},
		{"show bgp irr check | first 1", "first", "one document"},
		{"show bgp peer list | resolve", "resolve", "IP address"},
		{"show bgp peer history | resolve", "resolve", "IP address"},
		{"show bgp irr | resolve", "resolve", "IP address"},
	}
	for _, tc := range refusals {
		text, refusalErr := refusal(tc.command)
		if refusalErr != nil {
			return refusalErr
		}
		if err := namesThe(text, tc.command, tc.operator, tc.reason); err != nil {
			return err
		}
	}

	published := func(command string) (string, map[string]struct{}, error) {
		result, runErr := showBGPDeclaredShapesRun(ctx, workDir, cliEnv, "", "ze", "help", "command", command, "--json")
		if runErr != nil {
			return "", nil, fmt.Errorf("ze help command %q: %w", command, runErr)
		}
		if result.code != 0 {
			return "", nil, fmt.Errorf("ze help command %q exit=%d: %s%s", command, result.code, result.stdout, result.stderr)
		}
		parsed, decodeErr := showBGPDeclaredShapesDecode(result.stdout)
		if decodeErr != nil {
			return "", nil, fmt.Errorf("decode ze help command %q: %w", command, decodeErr)
		}
		var entry map[string]any
		switch value := parsed.(type) {
		case []any:
			if len(value) == 0 {
				return "", nil, fmt.Errorf("ze help command %q returned an empty catalog", command)
			}
			entry, _ = value[0].(map[string]any)
		case map[string]any:
			entry = value
		}
		if entry == nil {
			return "", nil, fmt.Errorf("ze help command %q returned no catalog entry: %#v", command, parsed)
		}
		shape, _ := entry["answer-shape"].(string)
		always := make(map[string]struct{})
		if operators, ok := entry["operators"].([]any); ok {
			for _, rawOperator := range operators {
				operator, ok := rawOperator.(map[string]any)
				if !ok || operator["available"] != "always" {
					continue
				}
				if name, ok := operator["name"].(string); ok {
					always[name] = struct{}{}
				}
			}
		}
		return shape, always, nil
	}

	// AC-19: the independently read command catalog must publish each declared
	// shape and only the operators that shape always supports.
	shape, always, err := published("show bgp rib best")
	if err != nil {
		return err
	}
	if shape != "tab" {
		return fmt.Errorf("`show bgp rib best` publishes shape %q, want tab", shape)
	}
	for _, operator := range []string{"count", "first", "display", "origin", "resolve"} {
		if _, ok := always[operator]; !ok {
			return fmt.Errorf("`show bgp rib best` does not publish `%s`: %v", operator, showBGPDeclaredShapesSortedSet(always))
		}
	}

	shape, always, err = published("show bgp rib status")
	if err != nil {
		return err
	}
	if shape != "doc" {
		return fmt.Errorf("`show bgp rib status` publishes shape %q, want doc", shape)
	}
	for _, operator := range []string{"count", "first", "display", "origin", "resolve"} {
		if _, ok := always[operator]; ok {
			return fmt.Errorf("`show bgp rib status` publishes `%s` over an answer with no rows: %v", operator, showBGPDeclaredShapesSortedSet(always))
		}
	}

	shape, always, err = published("show bgp irr prefix")
	if err != nil {
		return err
	}
	if shape != "map" {
		return fmt.Errorf("`show bgp irr prefix` publishes shape %q, want map", shape)
	}
	if _, ok := always["count"]; !ok {
		return fmt.Errorf("`show bgp irr prefix` does not publish `count`: %v", showBGPDeclaredShapesSortedSet(always))
	}
	if _, ok := always["fill"]; ok {
		return fmt.Errorf("`show bgp irr prefix` publishes `fill` over rows with no columns")
	}

	cleanupErr := daemon.stop()
	cleanupNeeded = false
	if cleanupErr != nil {
		return cleanupErr
	}
	fmt.Println("OK")
	return nil
}

func showBGPDeclaredShapesRun(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (showBGPDeclaredShapesResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := showBGPDeclaredShapesResult{code: 0, stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func showBGPDeclaredShapesStartDaemon(ctx context.Context, dir string, env []string) (*showBGPDeclaredShapesDaemon, error) {
	daemon := &showBGPDeclaredShapesDaemon{done: make(chan error, 1)}
	daemon.cmd = exec.CommandContext(ctx, "ze", "-f", "shapes.conf")
	daemon.cmd.Dir = dir
	daemon.cmd.Env = env
	daemon.cmd.Stdout = &daemon.stdout
	daemon.cmd.Stderr = &daemon.stderr
	if err := daemon.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}
	go func() {
		daemon.done <- daemon.cmd.Wait()
	}()
	return daemon, nil
}

func (daemon *showBGPDeclaredShapesDaemon) stop() error {
	if daemon.exited {
		return nil
	}
	_ = daemon.cmd.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case daemon.waitErr = <-daemon.done:
		daemon.exited = true
		return nil
	case <-timer.C:
	}

	_ = daemon.cmd.Process.Kill()
	timer.Reset(5 * time.Second)
	select {
	case daemon.waitErr = <-daemon.done:
		daemon.exited = true
		return nil
	case <-timer.C:
		return fmt.Errorf("daemon did not exit within 5 seconds after kill")
	}
}

func showBGPDeclaredShapesPoll(ctx context.Context, attempts int, delay time.Duration, observe func() (bool, error)) (bool, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		ready, err := observe()
		if err != nil || ready {
			return ready, err
		}
		if attempt+1 == attempts {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return false, nil
}

func showBGPDeclaredShapesSetEnv(base []string, keyValues ...string) []string {
	replacements := make(map[string]string, len(keyValues)/2)
	order := make([]string, 0, len(keyValues)/2)
	for i := 0; i < len(keyValues); i += 2 {
		replacements[keyValues[i]] = keyValues[i+1]
		order = append(order, keyValues[i])
	}
	result := make([]string, 0, len(base)+len(order))
	for _, item := range base {
		key := item
		if equal := strings.IndexByte(item, '='); equal >= 0 {
			key = item[:equal]
		}
		if _, replaced := replacements[key]; !replaced {
			result = append(result, item)
		}
	}
	for _, key := range order {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func showBGPDeclaredShapesDecode(text string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func showBGPDeclaredShapesCount(value any) (int, error) {
	if object, ok := value.(map[string]any); ok {
		value = object["count"]
	}
	switch number := value.(type) {
	case float64:
		integer := int(number)
		if float64(integer) != number {
			return 0, fmt.Errorf("count is not an integer: %#v", value)
		}
		return integer, nil
	case string:
		integer, err := strconv.Atoi(number)
		if err != nil {
			return 0, fmt.Errorf("invalid count %q: %w", number, err)
		}
		return integer, nil
	default:
		return 0, fmt.Errorf("invalid count value %#v", value)
	}
}

func showBGPDeclaredShapesSortedSet(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}
