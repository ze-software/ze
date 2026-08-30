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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	Register("ui/show-bgp-plugin-shapes", uiDriver(showBGPPluginShapes))
}

type pluginShapesResult struct {
	code   int
	stdout string
	stderr string
}

type pluginShapesProcess struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}

	mu      sync.Mutex
	waitErr error
}

// Observe runs an actual compiled product binary and records all three parts of
// its observable result: status, standard output, and standard error.
type pluginShapesFixture struct{}

func (f *pluginShapesFixture) Observe(ctx context.Context, dir string, env []string, stdin string, argv ...string) (pluginShapesResult, error) {
	if len(argv) == 0 {
		return pluginShapesResult{}, errors.New("Observe: empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := pluginShapesResult{
		code:   0,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

// Dispatch starts the long-running compiled product binary without involving a
// command interpreter.
func (f *pluginShapesFixture) Dispatch(ctx context.Context, dir string, env []string, argv ...string) (*pluginShapesProcess, error) {
	if len(argv) == 0 {
		return nil, errors.New("Dispatch: empty command")
	}
	p := &pluginShapesProcess{done: make(chan struct{})}
	p.cmd = exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	p.cmd.Dir = dir
	p.cmd.Env = env
	p.cmd.Stdout = &p.stdout
	p.cmd.Stderr = &p.stderr
	if err := p.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := p.cmd.Wait()
		p.mu.Lock()
		p.waitErr = err
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

// Poll evaluates observe immediately and then at the requested interval. An
// observation error aborts the poll rather than being mistaken for a timeout.
func (f *pluginShapesFixture) Poll(ctx context.Context, attempts int, delay time.Duration, observe func() (bool, error)) (bool, error) {
	for attempt := range attempts {
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

func showBGPPluginShapes(ctx context.Context) error {
	f := &pluginShapesFixture{}
	dir, err := os.MkdirTemp("", "ze-show-bgp-plugin-shapes-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup

	baseEnv := setPluginShapesEnv(os.Environ(), "PWD", dir)
	passwd, err := f.Observe(ctx, dir, baseEnv, "secret\n", "ze", "passwd")
	if err != nil {
		return fmt.Errorf("ze passwd: %w", err)
	}
	if passwd.code != 0 {
		return fmt.Errorf("ze passwd exit=%d: %s%s", passwd.code, passwd.stdout, passwd.stderr)
	}
	passwordHash := strings.TrimSpace(passwd.stdout)

	config := `plugin {
    internal rpki {
        use bgp-rpki
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
    internal rs {
        use bgp-rs
    }
    internal watchdog {
        use bgp-watchdog
    }
    internal healthcheck {
        use bgp-healthcheck
    }
}

bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
    rpki {
        cache-server 192.0.2.200 {
            port 3323
            preference 10
        }
        cache-server 198.51.100.7 {
            port 3324
            preference 20
        }
    }
}

system {
    authentication {
        user ci {
            password "` + passwordHash + `"
            profile [ admin ]
        }
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "shapes.conf"), []byte(config), 0o600); err != nil {
		return err
	}

	sshAddrPath := filepath.Join(dir, "ssh.addr")
	readyPath := filepath.Join(dir, "ready")
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := setPluginShapesEnv(baseEnv,
		"ZE_SSH_EPHEMERAL", sshAddrPath,
		"ZE_READY_FILE", readyPath,
		"ZE_CONFIG_DIR", dir,
		// Leave port 179 alone. The suite runs unprivileged, and a bind
		// failure there would stop the daemon before it writes ready.
		"ze_test_bgp_port", strconv.Itoa(bgpPort),
	)
	daemon, err := f.Dispatch(ctx, dir, daemonEnv, "ze", "-f", "shapes.conf")
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer stopPluginShapesDaemon(daemon)

	ready, err := f.Poll(ctx, 300, 100*time.Millisecond, func() (bool, error) {
		select {
		case <-daemon.done:
			daemon.mu.Lock()
			waitErr := daemon.waitErr
			daemon.mu.Unlock()
			return false, fmt.Errorf("daemon exited early (%w)\nstdout:\n%s\nstderr:\n%s", waitErr, daemon.stdout.String(), daemon.stderr.String())
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
		return errors.New("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddrPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH listener address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	cliEnv := setPluginShapesEnv(baseEnv,
		"ZE_SSH_HOST", host,
		"ZE_SSH_PORT", port,
		"ZE_SSH_USERNAME", "ci",
		"ZE_SSH_PASSWORD", "secret",
		"ZE_CONFIG_DIR", dir,
	)

	cli := func(command string) (string, error) {
		result, err := f.Observe(ctx, dir, cliEnv, "", "ze", "cli", "-c", command)
		if err != nil {
			return "", fmt.Errorf("%s: %w", command, err)
		}
		if result.code != 0 {
			return "", fmt.Errorf("%s exit=%d: %s%s", command, result.code, result.stdout, result.stderr)
		}
		return result.stdout, nil
	}
	refusal := func(command string) (string, error) {
		result, err := f.Observe(ctx, dir, cliEnv, "", "ze", "cli", "-c", command)
		if err != nil {
			return "", fmt.Errorf("%s: %w", command, err)
		}
		return result.stdout + result.stderr, nil
	}
	namesThe := func(text, command, operator, reason string) error {
		// "cannot apply here" is emitted only by the declaration-based
		// pre-dispatch validation. The operator's process has not run.
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

	// The RTR sessions are created while configuration is applied. Registration
	// and configuration happen on the plugin goroutine, so observe the rows
	// rather than assuming they exist as soon as the daemon's ready file does.
	var lastCacheRows []map[string]any
	cacheReady, err := f.Poll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		result, observeErr := f.Observe(ctx, dir, cliEnv, "", "ze", "cli", "-c", "show bgp rpki cache | json")
		if observeErr != nil || result.code != 0 {
			lastCacheRows = nil
			return false, nil //nolint:nilerr // the daemon has not published the cache yet, so the poll continues
		}
		rows, rowsErr := pluginShapesRowsOf(result.stdout, "cache-servers")
		if rowsErr != nil {
			lastCacheRows = nil
			return false, nil //nolint:nilerr // the rows are not shaped yet, so the poll continues
		}
		lastCacheRows = rows
		return len(rows) == 2, nil
	})
	if err != nil {
		return err
	}
	if !cacheReady {
		return fmt.Errorf("`show bgp rpki cache` never listed the two configured cache servers: %v", lastCacheRows)
	}

	// AC-9: display narrows every cache row to exactly the two selected fields.
	answer, err := cli("show bgp rpki cache | display address state | json")
	if err != nil {
		return err
	}
	displayed, err := pluginShapesRowsOf(answer, "cache-servers")
	if err != nil {
		return err
	}
	if len(displayed) != 2 {
		return fmt.Errorf("`| display address state` answered %d rows, want 2", len(displayed))
	}
	for _, row := range displayed {
		keys := pluginShapesSortedKeys(row)
		if len(keys) != 2 || keys[0] != columnAddress || keys[1] != columnState {
			return fmt.Errorf("`| display address state` answered fields %v", keys)
		}
	}

	// AC-10: resolve acts on address even when TEST-NET reverse lookup has no answer.
	answer, err = cli("show bgp rpki cache | resolve | json compact")
	if err != nil {
		return err
	}
	resolved, err := pluginShapesRowsOf(answer, "cache-servers")
	if err != nil {
		return err
	}
	if len(resolved) != 2 {
		return fmt.Errorf("`| resolve` answered %d rows, want 2", len(resolved))
	}
	for _, row := range resolved {
		if _, ok := row["address-name"]; !ok {
			return fmt.Errorf("`| resolve` did not decorate \"address\": %v", row)
		}
	}

	// AC-13: both row-shaped commands accept count, and an empty set answers zero.
	answer, err = cli("show bgp rs peers | count")
	if err != nil {
		return err
	}
	count, decoded, err := pluginShapesCount(answer)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("`show bgp rs peers | count` = %v, want 0", decoded)
	}
	answer, err = cli("show bgp healthcheck | count")
	if err != nil {
		return err
	}
	count, decoded, err = pluginShapesCount(answer)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("`show bgp healthcheck | count` = %v, want 0", decoded)
	}

	refusals := []struct {
		command  string
		operator string
		reason   string
	}{
		// AC-11 and AC-12, followed by the other document-shaped answers.
		{"show bgp rpki summary | first 2", pipeFirst, shapeOneDocument},
		{"show bgp rpki status | count", pipeCount, shapeOneDocument},
		{"show bgp rs status | count", pipeCount, shapeOneDocument},
		{"show bgp adj-rib-in | first 1", pipeFirst, shapeOneDocument},
		{"show bgp adj-rib-in | count", pipeCount, shapeOneDocument},
		{"show bgp adj-rib-in status | count", pipeCount, shapeOneDocument},
		// AC-10b: address-field requirements are declared per child command.
		{"show bgp rpki summary | resolve", pipeResolve, shapeIPAddress},
		{"show bgp rpki aspa | origin", pipeOrigin, shapeIPAddress},
		{"show bgp rs status | resolve", pipeResolve, shapeIPAddress},
		{"show bgp adj-rib-in | resolve", pipeResolve, shapeIPAddress},
		{"show bgp healthcheck | resolve", pipeResolve, shapeIPAddress},
	}
	for _, refusalCase := range refusals {
		text, err := refusal(refusalCase.command)
		if err != nil {
			return err
		}
		if err := namesThe(text, refusalCase.command, refusalCase.operator, refusalCase.reason); err != nil {
			return err
		}
	}

	fmt.Println("OK")
	return nil
}

func pluginShapesRowsOf(answer, key string) ([]map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(answer))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	if object, ok := decoded.(map[string]any); ok {
		var present bool
		decoded, present = object[key]
		if !present {
			return []map[string]any{}, nil
		}
	}
	items, ok := decoded.([]any)
	if !ok {
		return nil, fmt.Errorf("the answer holds no row list under %q: %v", key, decoded)
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("the answer holds a non-object row under %q: %v", key, item)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func pluginShapesCount(answer string) (int64, any, error) {
	decoder := json.NewDecoder(strings.NewReader(answer))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return 0, nil, err
	}
	value := decoded
	if object, ok := decoded.(map[string]any); ok {
		var present bool
		value, present = object["count"]
		if !present {
			return 0, decoded, fmt.Errorf("count answer has no count: %v", decoded)
		}
	}
	switch number := value.(type) {
	case json.Number:
		count, err := number.Int64()
		return count, decoded, err
	case float64:
		return int64(number), decoded, nil
	case string:
		count, err := strconv.ParseInt(number, 10, 64)
		return count, decoded, err
	default:
		return 0, decoded, fmt.Errorf("count answer is not numeric: %v", decoded)
	}
}

func pluginShapesSortedKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func setPluginShapesEnv(env []string, pairs ...string) []string {
	values := make(map[string]string, len(env)+len(pairs)/2)
	order := make([]string, 0, len(env)+len(pairs)/2)
	for _, entry := range env {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}
	for i := 0; i < len(pairs); i += 2 {
		key, value := pairs[i], pairs[i+1]
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, key+"="+values[key])
	}
	return result
}

func stopPluginShapesDaemon(p *pluginShapesProcess) {
	select {
	case <-p.done:
		return
	default:
	}
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}
	if pluginShapesWaitForProcess(p.done, 5*time.Second) {
		return
	}
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = pluginShapesWaitForProcess(p.done, 5*time.Second)
}

func pluginShapesWaitForProcess(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

var _ io.Reader = strings.NewReader("")
