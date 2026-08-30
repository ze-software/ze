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
	"reflect"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin15FreePort() (int, error) {
	// The listener closes on return, so there is nothing to cancel.
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("a tcp listener answered %T, want *net.TCPAddr", listener.Addr())
	}
	return address.Port, nil
}

func plugin15RunCommand(ctx context.Context, env []string, stdin string, args ...string) (int, string, string, error) {
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = env
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String(), nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode(), stdout.String(), stderr.String(), nil
	}
	return -1, stdout.String(), stderr.String(), err
}

func plugin15Environment(updates map[string]string) []string {
	env := os.Environ()
	result := make([]string, 0, len(env)+len(updates))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := updates[key]; !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range updates {
		result = append(result, key+"="+value)
	}
	return result
}

func plugin15RPKIPipeSummary(ctx context.Context, _ []string) error {
	for _, path := range []string{"ssh.addr", fieldReady} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale %s: %w", path, err)
		}
	}
	cachePort, err := plugin15FreePort()
	if err != nil {
		return err
	}
	bgpPort, err := plugin15FreePort()
	if err != nil {
		return err
	}
	code, passwordHash, passwordErr, err := plugin15RunCommand(ctx, os.Environ(), "secret\n", "passwd")
	if err != nil || code != 0 {
		return fmt.Errorf("ze passwd: exit=%d error=%w stderr=%s", code, err, passwordErr)
	}
	config := fmt.Sprintf(`plugin {
    internal rpki {
        use bgp-rpki
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
}

bgp {
    rpki {
        cache-server 127.0.0.1 {
            port %d
        }
    }
}

system {
    authentication {
        user ci {
            password %q
            profile [ admin ]
        }
    }
}
`, cachePort, strings.TrimSpace(passwordHash))
	if err := os.WriteFile("rpki-pipe.conf", []byte(config), 0o600); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	daemonEnv := plugin15Environment(map[string]string{
		envSSHEphemeral: cwd + "/ssh.addr",
		envReadyFile:    cwd + "/ready",
		envConfigDir:    cwd,
		envTestBGPPort:  fmt.Sprint(bgpPort),
	})
	daemon := exec.CommandContext(ctx, "ze", "-f", "rpki-pipe.conf")
	daemon.Env = daemonEnv
	var daemonOut, daemonErr bytes.Buffer
	daemon.Stdout = &daemonOut
	daemon.Stderr = &daemonErr
	if err := daemon.Start(); err != nil {
		return err
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- daemon.Wait() }()
	exited := false
	defer func() {
		if exited {
			return
		}
		_ = daemon.Process.Signal(syscall.SIGTERM)
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
			_ = daemon.Process.Kill()
			<-waitCh
		}
	}()

	ready := false
	var host, port string
	for range 300 {
		select {
		case waitErr := <-waitCh:
			exited = true
			return fmt.Errorf("daemon exited early: %w\nstdout:\n%s\nstderr:\n%s", waitErr, daemonOut.String(), daemonErr.String())
		default:
		}
		addressBytes, sshErr := os.ReadFile("ssh.addr")
		_, readyErr := os.Stat("ready")
		if sshErr == nil && readyErr == nil {
			host, port, err = net.SplitHostPort(strings.TrimSpace(string(addressBytes)))
			if err == nil && host != "" && port != "" {
				ready = true
				break
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		return fmt.Errorf("daemon did not become ready")
	}
	cliEnv := plugin15Environment(map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   cwd,
	})
	cli := func(command string) (int, string, string, error) {
		return plugin15RunCommand(ctx, cliEnv, "", "cli", "-c", command)
	}
	answered := false
	for range 200 {
		code, out, stderr, runErr := cli("show bgp rpki | json")
		if runErr == nil && code == 0 && strings.Contains(out, "vrp-count") {
			answered = true
			break
		}
		_ = stderr
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !answered {
		return fmt.Errorf("`show bgp rpki` never answered")
	}
	cliJSON := func(command string) (map[string]any, error) {
		code, out, stderr, runErr := cli(command)
		if runErr != nil || code != 0 {
			return nil, fmt.Errorf("%s exit=%d: %w %s%s", command, code, runErr, out, stderr)
		}
		value := map[string]any{}
		if err := json.Unmarshal([]byte(out), &value); err != nil {
			return nil, fmt.Errorf("%s: %w", command, err)
		}
		return value, nil
	}
	whole, err := cliJSON("show bgp rpki | json")
	if err != nil {
		return err
	}
	summaryFields := []string{fieldVRPCount, "validation-enabled", "sessions-total", "sessions-established", "sessions-synced", "aspa-enabled", "aspa-records"}
	for _, field := range summaryFields {
		if _, ok := whole[field]; !ok {
			return fmt.Errorf("the bare answer is missing %s: %v", field, whole)
		}
	}
	rows, ok := whole["cache-servers"].([]any)
	if !ok || len(rows) != 1 {
		return fmt.Errorf("the bare answer carries no cache server row: %v", whole)
	}
	row, _ := rows[0].(map[string]any)
	if row["address"] != addrLoopback {
		return fmt.Errorf("the cache server row names another address: %v", row)
	}
	half, err := cliJSON("show bgp rpki | summary | json")
	if err != nil {
		return err
	}
	gotFields := make([]string, 0, len(half))
	for field := range half {
		gotFields = append(gotFields, field)
	}
	sortStrings := func(values []string) []string {
		copyValues := append([]string(nil), values...)
		slicesSort(copyValues)
		return copyValues
	}
	if !reflect.DeepEqual(sortStrings(gotFields), sortStrings(summaryFields)) {
		return fmt.Errorf("`| summary` answered another field set: %v", half)
	}
	if _, ok := half["cache-servers"]; ok {
		return fmt.Errorf("a cache server row survived `| summary`: %v", half)
	}
	subcommand, err := cliJSON("show bgp rpki summary | json")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(subcommand, half) {
		return fmt.Errorf("`show bgp rpki summary` and `show bgp rpki | summary` disagree: %v vs %v", subcommand, half)
	}
	negativeCode, negativeOut, negativeErr, negativeRunErr := cli("show bgp rpki status | summary | json")
	if negativeRunErr != nil {
		return negativeRunErr
	}
	if !strings.Contains(negativeOut+negativeErr, "unknown pipe operator: summary") {
		return fmt.Errorf("`| summary` reached a command below the one it sits on: exit=%d %q %q", negativeCode, negativeOut, negativeErr)
	}
	for _, command := range []string{"show bgp rpki", "show bgp rpki | summary", "show bgp rpki summary"} {
		code, out, stderr, runErr := cli(command)
		if runErr != nil || code != 0 {
			return fmt.Errorf("%s exit=%d: %w %s%s", command, code, runErr, out, stderr)
		}
		fmt.Printf("--- %s\n%s", command, out)
	}
	fmt.Println("OK")
	return nil
}

func slicesSort(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func plugin15StreamAnswerTable(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("plugin/stream-answer-renders-table takes no arguments")
	}
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return err
	}
	const config = `plugin {
	external record-plugin {
		run "ze-test record-plugin"
		encoder json
	}
}

bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.2
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 65533
				remote 65533
			}
		}
	}
}

system {
	authentication {
		user admin {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ admin ]
		}
	}
	authorization {
		profile admin {
			run {
				default-action allow
			}
			edit {
				default-action allow
			}
		}
	}
}

environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1;
			port 0;
		}
	}
}
`
	if err := os.WriteFile("stream-answer-renders-table.conf", []byte(config), 0o600); err != nil {
		return err
	}
	bgpPort, err := plugin15FreePort()
	if err != nil {
		return err
	}
	logFile, err := os.Create("daemon.log")
	if err != nil {
		return err
	}
	defer logFile.Close() //nolint:errcheck // the fixture is exiting, so a close failure changes no assertion
	daemon := exec.CommandContext(ctx, "ze", "start", "stream-answer-renders-table.conf")
	daemon.Env = plugin15Environment(map[string]string{envTestBGPPort: fmt.Sprint(bgpPort)})
	daemon.Stdout = os.Stdout
	daemon.Stderr = logFile
	if err := daemon.Start(); err != nil {
		return err
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- daemon.Wait() }()
	exited := false
	defer func() {
		if exited {
			return
		}
		_ = daemon.Process.Signal(syscall.SIGTERM)
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
			_ = daemon.Process.Kill()
			<-waitCh
		}
	}()
	readLog := func() string {
		_ = logFile.Sync()
		data, _ := os.ReadFile("daemon.log")
		return string(data)
	}
	addressPattern := regexp.MustCompile(`127\.0\.0\.1:\d+`)
	var sshAddress string
	for range 50 {
		select {
		case waitErr := <-waitCh:
			exited = true
			return fmt.Errorf("daemon exited before SSH started: %w\n%s", waitErr, readLog())
		default:
		}
		sshAddress = addressPattern.FindString(readLog())
		if sshAddress != "" {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if sshAddress == "" {
		return fmt.Errorf("SSH server did not start (no address in daemon.log):\n%s", readLog())
	}
	host, port, err := net.SplitHostPort(sshAddress)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", port)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}
	adminDir, err := os.MkdirTemp("", "ze-stream-answer-admin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(adminDir) //nolint:errcheck // scratch cleanup on exit, so a removal failure changes no assertion
	initEnv := plugin15Environment(map[string]string{envConfigDir: adminDir})
	initInput := strings.Join([]string{"admin", valueTestPassword, host, port, ""}, "\n")
	code, _, initErr, runErr := plugin15RunCommand(ctx, initEnv, initInput, "init")
	if runErr != nil || code != 0 {
		return fmt.Errorf("ze init exit=%d: %w %s", code, runErr, initErr)
	}
	cliEnv := plugin15Environment(map[string]string{
		envConfigDir:   adminDir,
		envSSHPassword: valueTestPassword,
	})
	cli := func(command string) (int, string, string, error) {
		return plugin15RunCommand(ctx, cliEnv, "", "cli", "-c", command)
	}
	registered := false
	for range 50 {
		code, out, _, runErr := cli("system command list | raw")
		if runErr == nil && code == 0 && strings.Contains(out, "show test records table") {
			registered = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if !registered {
		return fmt.Errorf("the plugin never registered its schema-declaring command:\n%s", readLog())
	}
	runCLI := func(command string) (string, error) {
		code, out, stderr, runErr := cli(command)
		if runErr != nil || code != 0 {
			return "", fmt.Errorf("%s exit=%d: %w %s", command, code, runErr, stderr)
		}
		return out, nil
	}
	tableJSON, err := runCLI("show test records table | first 100 | raw")
	if err != nil {
		return fmt.Errorf("show test records table did not answer: %w", err)
	}
	objectJSON, err := runCLI("show test records object | first 100 | raw")
	if err != nil {
		return fmt.Errorf("show test records object did not answer: %w", err)
	}
	if tableJSON != objectJSON {
		return fmt.Errorf("the declared schema answered a different document from its schema-less twin: %.400s", tableJSON)
	}
	rendered, err := runCLI("show test records table | first 5 | table")
	if err != nil {
		return fmt.Errorf("the table rendering did not answer: %w", err)
	}
	if err := plugin15ValidateStreamAnswer([]byte(tableJSON), rendered); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the declared schema and its schema-less twin answered one document")
	return nil
}

func plugin15ValidateStreamAnswer(answerBytes []byte, rendered string) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(answerBytes, &envelope); err != nil {
		return err
	}
	rowsRaw, ok := envelope["rows"]
	if !ok {
		return fmt.Errorf("the answer is %.200s, want the rows envelope", answerBytes)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(rowsRaw, &rows); err != nil {
		return fmt.Errorf("the answer rows are not an array: %w", err)
	}
	if len(rows) != 100 {
		return fmt.Errorf("the answer carries %d rows, want the 100 the chain asked for", len(rows))
	}
	for number, raw := range rows {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		token, err := decoder.Token()
		if err != nil || token != json.Delim('{') {
			return fmt.Errorf("row %d reached the operator as a non-object", number)
		}
		keys := make([]string, 0, 2)
		values := make(map[string]any, 2)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("row %d carries a non-string key", number)
			}
			keys = append(keys, key)
			var value any
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			values[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return err
		}
		if !reflect.DeepEqual(keys, []string{"index", "fill"}) {
			return fmt.Errorf("row %d carries %v, want [index fill] in that order", number, keys)
		}
		if plugin15Number(values["index"]) != float64(number) {
			return fmt.Errorf("row %d carries index %v; the values landed under the wrong names", number, values["index"])
		}
	}
	fmt.Fprintf(os.Stderr, "OK: %d positional rows reached the operator under the names the head declared\n", len(rows))
	for _, column := range []string{"index", "fill"} {
		if !strings.Contains(rendered, column) {
			return fmt.Errorf("the table has no %s column:\n%.400s", column, rendered)
		}
	}
	for number := range 5 {
		if !strings.Contains(rendered, fmt.Sprint(number)) {
			return fmt.Errorf("the table has no row %d:\n%.600s", number, rendered)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: the streamed answer rendered as a table under its declared columns")
	return nil
}

func plugin15SummaryFormat(ctx context.Context, _ []string) error {
	p, err := newObserver("summary-check")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
	p.SetStartupSubscriptions([]string{eventUpdate}, nil, "summary")
	var validated sync.Once
	validatedCh := make(chan struct{})
	p.OnEvent(func(event string) error {
		var root map[string]any
		if json.Unmarshal([]byte(event), &root) != nil {
			return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
		}
		bgp, _ := root["bgp"].(map[string]any)
		message, _ := bgp["message"].(map[string]any)
		if message["type"] != eventUpdate {
			return nil
		}
		nlri, ok := bgp["nlri"].(map[string]any)
		if !ok {
			fmt.Fprintf(os.Stderr, "FAIL: bgp.nlri not a dict: %T\n", bgp["nlri"])
			return nil
		}
		for _, key := range []string{"announce", "withdrawn", "mp-reach", "mp-unreach"} {
			if _, ok := nlri[key]; !ok {
				fmt.Fprintf(os.Stderr, "FAIL: missing key %s in %v\n", key, nlri)
				return nil
			}
		}
		if _, ok := nlri["announce"].(bool); !ok {
			fmt.Fprintf(os.Stderr, "FAIL: announce not bool: %T\n", nlri["announce"])
			return nil
		}
		if _, ok := nlri["withdrawn"].(bool); !ok {
			fmt.Fprintf(os.Stderr, "FAIL: withdrawn not bool: %T\n", nlri["withdrawn"])
			return nil
		}
		if _, ok := nlri["mp-reach"].(string); !ok {
			fmt.Fprintf(os.Stderr, "FAIL: mp-reach not str: %T\n", nlri["mp-reach"])
			return nil
		}
		if _, ok := nlri["mp-unreach"].(string); !ok {
			fmt.Fprintf(os.Stderr, "FAIL: mp-unreach not str: %T\n", nlri["mp-unreach"])
			return nil
		}
		if nlri["announce"] != true {
			fmt.Fprintln(os.Stderr, "FAIL: announce should be true for default route")
			return nil
		}
		if _, ok := message["id"]; !ok {
			fmt.Fprintln(os.Stderr, "FAIL: message.id missing")
			return nil
		}
		var updateErr error
		validated.Do(func() {
			fmt.Fprintf(os.Stderr, "OK: summary format validated: %v\n", nlri)
			close(validatedCh)
			updateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, _, updateErr = p.UpdateRoute(updateCtx, "127.0.0.1", "update text origin igp local-preference 100 nhop 5.6.7.8 nlri ipv4/unicast add 1.2.3.4/32")
		})
		return updateErr
	})
	timedOut := make(chan struct{})
	var timer *time.Timer
	p.OnStarted(func(context.Context) error {
		timer = time.AfterFunc(8*time.Second, func() {
			select {
			case <-validatedCh:
				return
			default:
			}
			fmt.Fprintln(os.Stderr, "FAIL: timeout waiting for summary event")
			close(timedOut)
			_ = p.Close()
		})
		return nil
	})
	runErr := p.Run(ctx, sdk.Registration{})
	if timer != nil {
		timer.Stop()
	}
	select {
	case <-timedOut:
		return fmt.Errorf("timeout waiting for summary event")
	default:
		return runErr
	}
}
