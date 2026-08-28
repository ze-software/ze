package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const leL2TPDiagnosticsAnswersName = "ui/le-l2tp-diagnostics-answers"

func init() {
	Register(leL2TPDiagnosticsAnswersName, uiDriver(runLEL2TPDiagnosticsAnswers))
}

type uiLeL2tpDiagnosticsAnswersCommandResult struct {
	stdout string
	stderr string
	code   int
}

type diagnosticReport struct {
	Diagnostic string `json:"diagnostic"`
	Verdict    string `json:"verdict"`
	Retained   []struct {
		Kind string `json:"kind"`
	} `json:"retained"`
}

type diagnosticCall struct {
	Operation string `json:"operation"`
	Payload   string `json:"payload"`
}

func runLEL2TPDiagnosticsAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return uiLeL2tpDiagnosticsAnswersFailf("ZE_REPO_ROOT is not set")
	}

	work, err := os.MkdirTemp("", "ze-test-le-l2tp-diagnostics-answers-")
	if err != nil {
		return uiLeL2tpDiagnosticsAnswersFailf("creating fixture work directory: %v", err)
	}
	defer os.RemoveAll(work)

	binary := filepath.Join(work, "le")
	tags, err := uiLEFeatureTags(root, "zetest")
	if err != nil {
		return err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return uiLeL2tpDiagnosticsAnswersFailf("finding go: %v", err)
	}
	build := exec.CommandContext(ctx, goTool, "build", "-tags", strings.Join(tags, ","), "-o", binary, "./cmd/ze")
	build.Dir = root
	build.Env = uiLeL2tpDiagnosticsAnswersEnvironmentWith("CGO_ENABLED", "0")
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		return uiLeL2tpDiagnosticsAnswersFailf("building le with the diagnostic seam: %v\n%s", err, buildOutput.String())
	}

	invalidRecord := filepath.Join(work, "invalid.ndjson")
	invalid := runLE(
		ctx,
		binary,
		work,
		"invalid.ndjson",
		"l2tp-tunnel-diag", "local", "not-an-address",
	)
	if invalid.code != 1 {
		return uiLeL2tpDiagnosticsAnswersFailf("invalid address exited %d", invalid.code)
	}
	if _, err := os.Stat(invalidRecord); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return uiLeL2tpDiagnosticsAnswersFailf("invalid input reached the Linux seam")
		}
		return uiLeL2tpDiagnosticsAnswersFailf("checking invalid-input recording: %v", err)
	}

	pppoxArgs := []string{
		"l2tp-pppox-diag",
		"local", "0.0.0.0", "remote", "127.0.0.1",
		"source-port", "1701", "destination-port", "1701",
		"tunnel-id", "1", "peer-tunnel-id", "100",
		"session-id", "1", "peer-session-id", "100",
	}
	pppox := runLE(ctx, binary, work, "pppox.ndjson", append(pppoxArgs, "|", "json")...)
	if pppox.code != 0 {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX diagnostic failed: %s", pppox.stderr)
	}

	var pppoxReport diagnosticReport
	if err := json.Unmarshal([]byte(pppox.stdout), &pppoxReport); err != nil {
		return uiLeL2tpDiagnosticsAnswersFailf("decoding PPPoX JSON report: %v; output: %s", err, pppox.stdout)
	}
	if pppoxReport.Diagnostic != "l2tp-pppox-diag" {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX diagnostic changed: %s", reportJSON(pppoxReport))
	}
	if pppoxReport.Verdict != "working" {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX verdict changed: %s", reportJSON(pppoxReport))
	}
	if got := retainedKinds(pppoxReport); !uiLeL2tpDiagnosticsAnswersEqualStrings(got, []string{"tunnel", "session"}) {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX retained objects changed: %s", reportJSON(pppoxReport))
	}

	pppoxCalls, err := readDiagnosticCalls(filepath.Join(work, "pppox.ndjson"))
	if err != nil {
		return err
	}
	wantPPPoXOperations := []string{
		"socket inet",
		"setsockopt reuseport",
		"bind udp",
		"resolve family",
		"tunnel-create",
		"tunnel-dump",
		"session-create",
		"session-dump",
		"socket pppox",
		"connect pppox",
		"ioctl get channel",
		"open /dev/ppp",
		"ioctl attach channel",
		"open /dev/ppp",
		"ioctl new unit",
		"ioctl connect unit",
		"link ppp0",
	}
	if got := callOperations(pppoxCalls); !uiLeL2tpDiagnosticsAnswersEqualStrings(got, wantPPPoXOperations) {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX operation order changed: %s", callsJSON(pppoxCalls))
	}

	pppoxByName := callsByOperation(pppoxCalls)
	if got := pppoxByName["tunnel-create"].Payload; got != "01010000080009000100000008000a0064000000050007000200000006000200000000000800170003000000" {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX tunnel-create payload changed: %s", callJSON(pppoxByName["tunnel-create"]))
	}
	if got := pppoxByName["session-create"].Payload; got != "05010000080009000100000008000b000100000008000c00640000000600010007000000" {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX session-create payload changed: %s", callJSON(pppoxByName["session-create"]))
	}
	if got := pppoxByName["connect pppox"].Payload; got != "1800010000000000000003000000020006a57f00000100000000000000000100010064006400" {
		return uiLeL2tpDiagnosticsAnswersFailf("PPPoX connect payload changed: %s", callJSON(pppoxByName["connect pppox"]))
	}

	tunnelArgs := []string{
		"l2tp-tunnel-diag",
		"local", "172.30.0.1", "remote", "172.30.0.2",
		"source-port", "1701", "destination-port", "1702",
		"tunnel-id", "1", "peer-tunnel-id", "100",
	}
	tunnel := runLE(ctx, binary, work, "tunnel.ndjson", append(tunnelArgs, "|", "json")...)
	if tunnel.code != 0 {
		return uiLeL2tpDiagnosticsAnswersFailf("tunnel diagnostic failed: %s", tunnel.stderr)
	}

	var tunnelReport diagnosticReport
	if err := json.Unmarshal([]byte(tunnel.stdout), &tunnelReport); err != nil {
		return uiLeL2tpDiagnosticsAnswersFailf("decoding tunnel JSON report: %v; output: %s", err, tunnel.stdout)
	}
	if tunnelReport.Verdict != "working" {
		return uiLeL2tpDiagnosticsAnswersFailf("tunnel verdict changed: %s", reportJSON(tunnelReport))
	}
	if len(tunnelReport.Retained) != 1 || tunnelReport.Retained[0].Kind != "tunnel" {
		return uiLeL2tpDiagnosticsAnswersFailf("tunnel retained objects changed: %s", reportJSON(tunnelReport))
	}

	tunnelCalls, err := readDiagnosticCalls(filepath.Join(work, "tunnel.ndjson"))
	if err != nil {
		return err
	}
	if got := callOperations(tunnelCalls); !uiLeL2tpDiagnosticsAnswersEqualStrings(got, []string{"resolve family", "tunnel-create", "tunnel-dump"}) {
		return uiLeL2tpDiagnosticsAnswersFailf("tunnel operation order changed: %s", callsJSON(tunnelCalls))
	}
	if got := tunnelCalls[1].Payload; got != "01010000080009000100000008000a00640000000500070003000000060002000000000006001a00a506000006001b00a606000008001800ac1e000108001900ac1e0002" {
		return uiLeL2tpDiagnosticsAnswersFailf("tunnel-create payload changed: %s", callJSON(tunnelCalls[1]))
	}

	yamlResult := runLE(ctx, binary, work, "yaml.ndjson", append(tunnelArgs, "|", "yaml")...)
	if yamlResult.code != 0 || !strings.Contains(yamlResult.stdout, "diagnostic: l2tp-tunnel-diag") {
		return uiLeL2tpDiagnosticsAnswersFailf("YAML rendering failed: %s%s", yamlResult.stdout, yamlResult.stderr)
	}

	tableResult := runLE(ctx, binary, work, "table.ndjson", append(pppoxArgs, "|", "table")...)
	if tableResult.code != 0 || !strings.Contains(tableResult.stdout, "l2tp-pppox-diag") {
		return uiLeL2tpDiagnosticsAnswersFailf("table rendering failed: %s%s", tableResult.stdout, tableResult.stderr)
	}

	fmt.Println("LE_L2TP_DIAGNOSTICS_ANSWERS_OK")
	return nil
}

func runLE(ctx context.Context, binary, work, record string, args ...string) uiLeL2tpDiagnosticsAnswersCommandResult {
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, "deployment")
	argv = append(argv, args...)
	return uiLeL2tpDiagnosticsAnswersRunCommand(
		ctx,
		work,
		uiLeL2tpDiagnosticsAnswersEnvironmentWith("ZE_TEST_L2TP_DIAGNOSTIC_RECORD", filepath.Join(work, record)),
		binary,
		argv...,
	)
}

func uiLeL2tpDiagnosticsAnswersRunCommand(ctx context.Context, dir string, env []string, name string, args ...string) uiLeL2tpDiagnosticsAnswersCommandResult {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			code = exitError.ExitCode()
		} else {
			code = -1
			if stderr.Len() == 0 {
				stderr.WriteString(err.Error())
			}
		}
	}

	return uiLeL2tpDiagnosticsAnswersCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   code,
	}
}

func uiLeL2tpDiagnosticsAnswersEnvironmentWith(keyValues ...string) []string {
	replacements := make(map[string]string, len(keyValues)/2)
	for i := 0; i < len(keyValues); i += 2 {
		replacements[keyValues[i]] = keyValues[i+1]
	}

	env := make([]string, 0, len(os.Environ())+len(replacements))
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if _, replaced := replacements[key]; ok && replaced {
			continue
		}
		env = append(env, item)
	}
	for key, value := range replacements {
		env = append(env, key+"="+value)
	}
	return env
}

func readDiagnosticCalls(path string) ([]diagnosticCall, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, uiLeL2tpDiagnosticsAnswersFailf("opening diagnostic recording %s: %v", path, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var calls []diagnosticCall
	lineNumber := 0
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) != 0 {
			lineNumber++
			if strings.TrimSpace(line) != "" {
				var call diagnosticCall
				if err := json.Unmarshal([]byte(line), &call); err != nil {
					return nil, uiLeL2tpDiagnosticsAnswersFailf("decoding %s line %d: %v", path, lineNumber, err)
				}
				calls = append(calls, call)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, uiLeL2tpDiagnosticsAnswersFailf("reading diagnostic recording %s: %v", path, readErr)
		}
	}
	return calls, nil
}

func retainedKinds(report diagnosticReport) []string {
	kinds := make([]string, len(report.Retained))
	for i := range report.Retained {
		kinds[i] = report.Retained[i].Kind
	}
	return kinds
}

func callOperations(calls []diagnosticCall) []string {
	operations := make([]string, len(calls))
	for i := range calls {
		operations[i] = calls[i].Operation
	}
	return operations
}

func callsByOperation(calls []diagnosticCall) map[string]diagnosticCall {
	byOperation := make(map[string]diagnosticCall, len(calls))
	for _, call := range calls {
		byOperation[call.Operation] = call
	}
	return byOperation
}

func uiLeL2tpDiagnosticsAnswersEqualStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func reportJSON(report diagnosticReport) string {
	encoded, _ := json.Marshal(report)
	return string(encoded)
}

func callsJSON(calls []diagnosticCall) string {
	encoded, _ := json.Marshal(calls)
	return string(encoded)
}

func callJSON(call diagnosticCall) string {
	encoded, _ := json.Marshal(call)
	return string(encoded)
}

func uiLeL2tpDiagnosticsAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
