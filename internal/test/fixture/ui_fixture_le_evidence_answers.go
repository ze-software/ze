// Design: docs/functional-tests.md -- deployment evidence: the evidence and deployment answers
// Related: register_le_evidence_answers.go -- the registration

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// leEvidenceAnswers drives the built le binary through the evidence and
// deployment commands. Each assertion returns the first failure as the
// fixture's error, so the observer reports it and the run stops there.
func leEvidenceAnswers(ctx context.Context) error {
	return runLEEvidenceAnswers(ctx)
}

type uiLeEvidenceAnswersCommandResult struct {
	stdout string
	stderr string
	code   int
	err    error
}

func uiLeEvidenceAnswersRunCommand(ctx context.Context, dir, program string, args, env []string) uiLeEvidenceAnswersCommandResult {
	command := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	if env != nil {
		command.Env = env
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := 0
	if err != nil {
		if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitError.ExitCode()
		} else {
			code = -1
		}
	}
	return uiLeEvidenceAnswersCommandResult{stdout: stdout.String(), stderr: stderr.String(), code: code, err: err}
}

// uiLeEvidenceAnswersRunAtTerminal runs a whole-tree verb the way Thomas
// starts it: stdin and stdout on a terminal. `verify evidence
// release-candidate` declares leaction.Action.WholeTree, so the dispatcher
// refuses it with exit 2 before its body unless both are terminals
// (human.RequireTerminal); over pipes the fixture would only ever see that
// refusal. stderr stays a separate stream, so the assertions keep reading
// stdout and stderr apart.
func uiLeEvidenceAnswersRunAtTerminal(ctx context.Context, dir, program string, args, env []string) uiLeEvidenceAnswersCommandResult {
	stdout, stderr, code, err := terminalCommandStreams(ctx, dir, env, program, args...)
	return uiLeEvidenceAnswersCommandResult{stdout: stdout, stderr: stderr, code: code, err: err}
}

func uiLeEvidenceAnswersEnvironment(overrides map[string]string) []string {
	return childEnvironment(os.Environ(), overrides)
}

func uiLeEvidenceAnswersTail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}

func uiLeEvidenceAnswersWriteFile(path, contents string, mode os.FileMode) error {
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		return fmt.Errorf("writing %s failed: %w", path, err)
	}
	return nil
}

// uiLeEvidenceAnswersRecorded reads the argv records a stub wrote. A missing
// file is a stub that was never called, so it answers no calls.
func uiLeEvidenceAnswersRecorded(path string) ([]string, error) {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	// refuse-by-default: guards runLEEvidenceAnswers -- a stub nobody called writes no record, so no calls is its answer
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading recording %s failed: %w", path, err)
	}
	parts := strings.Split(string(contents), "\x1e")
	calls := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			calls = append(calls, strings.ReplaceAll(part, "\x1f", " "))
		}
	}
	return calls, nil
}

func hasKey(object map[string]any, key string) bool {
	_, found := object[key]
	return found
}

func uiLeEvidenceAnswersSortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func runLEEvidenceAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}

	work, err := os.MkdirTemp("", "le-evidence-answers-")
	if err != nil {
		return fmt.Errorf("creating fixture work directory failed: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	if err != nil {
		return err
	}

	checkout := filepath.Join(work, "fixture")
	if err := os.MkdirAll(checkout, 0o750); err != nil {
		return fmt.Errorf("creating fixture checkout failed: %w", err)
	}
	if err := uiLeEvidenceAnswersWriteFile(filepath.Join(checkout, "go.mod"), "module example.test/m\n\ngo 1.26\n", 0o644); err != nil {
		return err
	}
	if err := uiLeEvidenceAnswersWriteFile(filepath.Join(checkout, "feature-gates.txt"),
		"ze_bgp internal/component/bgp\nze_l2tp internal/component/l2tp\n", 0o644); err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding fixture executable failed: %w", err)
	}
	stubs, err := os.MkdirTemp(filepath.Dir(self), "le-evidence-stubs-")
	if err != nil {
		return fmt.Errorf("creating stub directory failed: %w", err)
	}
	defer os.RemoveAll(stubs) //nolint:errcheck // fixture cleanup
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("finding go for tool stubs failed: %w", err)
	}
	stubPath := filepath.Join(stubs, "docker")
	buildStub := exec.CommandContext(ctx, goTool, "build", "-o", stubPath, "./internal/test/toolstub/cmd") //nolint:gosec // the fixture chooses the program and its arguments
	buildStub.Dir = root
	buildStub.Env = uiLeEvidenceAnswersEnvironment(map[string]string{envCGOEnabled: "0"})
	output, buildErr := buildStub.CombinedOutput()
	if buildErr != nil {
		return fmt.Errorf("building tool stub failed: %w\n%s", buildErr, output)
	}
	stubBytes, err := os.ReadFile(stubPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("reading tool stub failed: %w", err)
	}
	for _, name := range []string{"git", "ip", "ping", "xl2tpd", "pppd", "go"} {
		err := os.WriteFile(filepath.Join(stubs, name), stubBytes, 0o755) //nolint:gosec // the fixture writes an executable stand-in, so it needs the execute bit
		if err != nil {
			return fmt.Errorf("writing native %s stub failed: %w", name, err)
		}
	}

	// le activates the patched GOROOT of the checkout it runs in, and the
	// activated tree's bin goes first on PATH. The tree is built here over a
	// stand-in GOROOT whose bin/go is the stub, so go stays stubbed after
	// activation. Its src is the real one, which holds the files the patches
	// change, and its version is the real one, which their stamps name. The
	// checkout's own cache/ keeps that tree out of the shared stdpatch cache.
	realGOROOT, versionErr := exec.CommandContext(ctx, goTool, "env", "GOROOT").Output() //nolint:gosec // the fixture chooses the program and its arguments
	if versionErr != nil {
		return fmt.Errorf("reading the real GOROOT failed: %w", versionErr)
	}
	realVersion, versionErr := exec.CommandContext(ctx, goTool, "env", "GOVERSION").Output() //nolint:gosec // the fixture chooses the program and its arguments
	if versionErr != nil {
		return fmt.Errorf("reading the real GOVERSION failed: %w", versionErr)
	}
	standInGOROOT := filepath.Join(work, "goroot")
	if err := os.MkdirAll(filepath.Join(standInGOROOT, "bin"), 0o750); err != nil {
		return fmt.Errorf("creating the stand-in GOROOT failed: %w", err)
	}
	err = os.WriteFile(filepath.Join(standInGOROOT, "bin", "go"), stubBytes, 0o755) //nolint:gosec // the fixture writes an executable stand-in, so it needs the execute bit
	if err != nil {
		return fmt.Errorf("writing the stand-in GOROOT go failed: %w", err)
	}
	err = os.Symlink(filepath.Join(strings.TrimSpace(string(realGOROOT)), "src"), filepath.Join(standInGOROOT, "src"))
	if err != nil {
		return fmt.Errorf("linking the stand-in GOROOT src failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(checkout, "cache"), 0o750); err != nil {
		return fmt.Errorf("creating the checkout cache failed: %w", err)
	}
	if err := leFixtureStdpatch(checkout); err != nil {
		return fmt.Errorf("giving the checkout the standard-library patch set failed: %w", err)
	}

	defaultDockerRecord := filepath.Join(work, "docker-argv")
	ipRecord := filepath.Join(work, "ip-argv")
	baseOverrides := map[string]string{
		envPath:                stubs + string(os.PathListSeparator) + os.Getenv("PATH"),
		envRepoRoot:            checkout,
		"ZE_GIT_STATUS":        "",
		"ZE_STUB_GO_GOROOT":    standInGOROOT,
		"ZE_STUB_GO_GOVERSION": strings.TrimSpace(string(realVersion)),
		"ZE_DOCKER_EXIT":       "0",
		"ZE_RECORD_DOCKER":     defaultDockerRecord,
		"ZE_RECORD_IP":         ipRecord,
	}

	environment := func(status, exitCode, record string, extra map[string]string) []string {
		overrides := make(map[string]string, len(baseOverrides)+len(extra))
		maps.Copy(overrides, baseOverrides)
		overrides["ZE_GIT_STATUS"] = status
		if exitCode != "" {
			overrides["ZE_DOCKER_EXIT"] = exitCode
		}
		if record != "" {
			overrides["ZE_RECORD_DOCKER"] = record
		}
		maps.Copy(overrides, extra)
		return uiLeEvidenceAnswersEnvironment(overrides)
	}
	le := func(args []string, record string, extra map[string]string) uiLeEvidenceAnswersCommandResult {
		return uiLeEvidenceAnswersRunCommand(ctx, work, binary, args, environment("", "", record, extra))
	}
	// leAtTerminal runs a whole-tree verb, which refuses pipes by design.
	leAtTerminal := func(args []string, status, exitCode, record string) uiLeEvidenceAnswersCommandResult {
		return uiLeEvidenceAnswersRunAtTerminal(ctx, work, binary, args, environment(status, exitCode, record, nil))
	}

	// Both commands must be present in the composition root used by help.
	usage := le([]string{flagHelp}, "", nil)
	page := usage.stdout + usage.stderr
	for _, name := range []string{areaEvidence, areaDeployment} {
		if !strings.Contains(page, name) {
			return fmt.Errorf("le --help does not list the %s command", name)
		}
	}

	// Bare areas list all actions and identify them as read-only checks.
	listings := []struct{ name, verb string }{
		{areaEvidence, actionReleaseCandidate},
		{areaDeployment, actionL2TPTest},
		{areaDeployment, "l2tp-ppp"},
		{areaDeployment, "gokrazy-l2tp-ppp"},
	}
	for _, item := range listings {
		listing := le([]string{item.name}, "", nil)
		if listing.code != 0 {
			return fmt.Errorf("le %s exited %d", item.name, listing.code)
		}
		if !strings.Contains(listing.stdout, item.verb) {
			return fmt.Errorf("le %s does not list %s: %q", item.name, item.verb, listing.stdout)
		}
		if !strings.Contains(listing.stdout, "checks") {
			return fmt.Errorf("le %s does not mark %s read-only", item.name, item.verb)
		}
	}

	// A clean release-candidate check starts exactly one container.
	rcRecord := filepath.Join(work, "rc-argv")
	answer := leAtTerminal([]string{areaEvidence, actionReleaseCandidate}, "", "", rcRecord)
	if answer.code != 0 {
		return fmt.Errorf("the release-candidate check exited %d: %s", answer.code, uiLeEvidenceAnswersTail(answer.stderr, 800))
	}
	calls, err := uiLeEvidenceAnswersRecorded(rcRecord)
	if err != nil {
		return err
	}
	if len(calls) != 1 {
		return fmt.Errorf("the check started %d containers, want 1: %v", len(calls), calls)
	}
	if !strings.Contains(calls[0], "-v "+checkout+":/host:ro") {
		return fmt.Errorf("the tree is not mounted read-only: %s", calls[0])
	}
	if !strings.Contains(calls[0], "git clone --no-local /host /work/src") {
		return errors.New("the container program does not clone the mount")
	}

	rendered := leAtTerminal([]string{areaEvidence, actionReleaseCandidate, "|", renderJSON}, "", "",
		filepath.Join(work, "rc-json"))
	var report map[string]any
	if err := json.Unmarshal([]byte(rendered.stdout), &report); err != nil {
		return fmt.Errorf("the release-candidate JSON is invalid: %w; output %q", err, rendered.stdout)
	}
	for _, key := range []string{fieldImage, "platform", fieldTree, "dirty", fieldPassed, "code"} {
		if !hasKey(report, key) {
			return fmt.Errorf("the report answered no %q key: %v", key, uiLeEvidenceAnswersSortedKeys(report))
		}
	}
	if report["passed"] != true {
		return fmt.Errorf("a container that exited 0 reports %v", report["passed"])
	}
	if report[fieldTree] != checkout {
		return fmt.Errorf("the report names %q, want the fixture", report[fieldTree])
	}

	// Preserve the container's exact status rather than flattening it.
	for _, code := range []string{"2", "3", "125"} {
		failed := leAtTerminal([]string{areaEvidence, actionReleaseCandidate}, "", code,
			filepath.Join(work, "rc-"+code))
		want, _ := strconv.Atoi(code)
		if failed.code != want {
			return fmt.Errorf("a container that exited %s made the command exit %d", code, failed.code)
		}
	}

	// Dirty trees are rejected before any container is started.
	dirtyRecord := filepath.Join(work, "rc-dirty")
	dirty := leAtTerminal([]string{areaEvidence, actionReleaseCandidate}, " M internal/a.go\n", "",
		dirtyRecord)
	if dirty.code != 1 {
		return fmt.Errorf("a dirty tree exited %d, want 1", dirty.code)
	}
	calls, err = uiLeEvidenceAnswersRecorded(dirtyRecord)
	if err != nil {
		return err
	}
	if len(calls) != 0 {
		return errors.New("a container was started over a dirty tree")
	}
	if !strings.Contains(dirty.stdout, "internal/a.go") {
		return fmt.Errorf("the refusal does not name the path: %q", dirty.stdout)
	}

	// The L2TP proof builds its daemon, starts the peer, and observes a session.
	l2tpRecord := filepath.Join(work, "l2tp-argv")
	proof := le([]string{areaDeployment, actionL2TPTest, "|", renderJSON}, l2tpRecord, nil)
	if proof.code != 0 {
		return fmt.Errorf("the L2TP proof exited %d: %s", proof.code, uiLeEvidenceAnswersTail(proof.stderr, 800))
	}
	var verdict map[string]any
	if err := json.Unmarshal([]byte(proof.stdout), &verdict); err != nil {
		return fmt.Errorf("the L2TP JSON is invalid: %w; output %q", err, proof.stdout)
	}
	for _, key := range []string{fieldPeer, fieldImage, fieldContainer, stateEstablished, "log-tail"} {
		if !hasKey(verdict, key) {
			return fmt.Errorf("the proof answered no %q key: %v", key, uiLeEvidenceAnswersSortedKeys(verdict))
		}
	}
	if verdict["established"] != true {
		return errors.New("the proof did not read the session")
	}
	if verdict["peer"] != "xl2tpd" {
		return fmt.Errorf("the proof names the peer %q", verdict["peer"])
	}

	calls, err = uiLeEvidenceAnswersRecorded(l2tpRecord)
	if err != nil {
		return err
	}
	if len(calls) != 6 {
		return fmt.Errorf("the proof made %d docker calls, want 6:\n%s", len(calls), strings.Join(calls, "\n"))
	}
	peerStarted := false
	privileged := false
	granted := false
	for _, call := range calls {
		peerStarted = peerStarted || strings.Contains(call, "xl2tpd -D")
		privileged = privileged || strings.Contains(call, "--privileged")
		granted = granted || strings.Contains(call, "--cap-add NET_ADMIN --device /dev/ppp")
	}
	if !peerStarted {
		return fmt.Errorf("the proof never started the peer:\n%s", strings.Join(calls, "\n"))
	}
	if privileged {
		return fmt.Errorf("the container runs --privileged:\n%s", strings.Join(calls, "\n"))
	}
	if !granted {
		return fmt.Errorf("the container lacks NET_ADMIN and /dev/ppp:\n%s", strings.Join(calls, "\n"))
	}

	// Both machine-touching proofs reject the kernel-probe escape before making
	// any namespace, lab, image, or container change. A refused run never
	// reached the proof, so neither answers a verdict (runL2TPPPP and
	// runGokrazyL2TP through notReached): the refusal goes to stderr and the
	// payload is empty, because neither proof starts a container whose removal
	// could still be owed.
	const kernelProbeRefusal = "refusing to run with ZE_L2TP_SKIP_KERNEL_PROBE set; full proof must not skip the kernel probe"
	escaped := le([]string{areaDeployment, "l2tp-ppp", "|", renderJSON}, "",
		map[string]string{"ZE_L2TP_SKIP_KERNEL_PROBE": valueTrue})
	if escaped.code != 1 {
		return fmt.Errorf("a run carrying the kernel-probe escape exited %d, want 1", escaped.code)
	}
	if !strings.Contains(escaped.stderr, kernelProbeRefusal) {
		return fmt.Errorf("the refusal does not name the variable on stderr: %q", uiLeEvidenceAnswersTail(escaped.stderr, 400))
	}
	calls, err = uiLeEvidenceAnswersRecorded(ipRecord)
	if err != nil {
		return err
	}
	if len(calls) != 0 {
		return errors.New("the refused run still made a namespace")
	}
	var refused map[string]any
	if err := json.Unmarshal([]byte(escaped.stdout), &refused); err != nil {
		return fmt.Errorf("the refused proof JSON is invalid: %w; output %q", err, escaped.stdout)
	}
	if refused == nil {
		return fmt.Errorf("the refused proof answered no JSON object: %q", escaped.stdout)
	}
	if len(refused) != 0 {
		return fmt.Errorf("a refused run answered a verdict: %v", uiLeEvidenceAnswersSortedKeys(refused))
	}

	appliance := le([]string{areaDeployment, "gokrazy-l2tp-ppp", "|", renderJSON}, "",
		map[string]string{"ZE_L2TP_SKIP_KERNEL_PROBE": valueTrue})
	if appliance.code != 1 {
		return fmt.Errorf("the appliance proof exited %d on the escape, want 1", appliance.code)
	}
	calls, err = uiLeEvidenceAnswersRecorded(ipRecord)
	if err != nil {
		return err
	}
	if len(calls) != 0 {
		return errors.New("the refused appliance run still made a lab")
	}
	if !strings.Contains(appliance.stderr, kernelProbeRefusal) {
		return fmt.Errorf("the appliance refusal does not name the variable on stderr: %q", uiLeEvidenceAnswersTail(appliance.stderr, 400))
	}
	report = nil
	if err := json.Unmarshal([]byte(appliance.stdout), &report); err != nil {
		return fmt.Errorf("the appliance JSON is invalid: %w; output %q", err, appliance.stdout)
	}
	if report == nil {
		return fmt.Errorf("the appliance answered no JSON object: %q", appliance.stdout)
	}
	if len(report) != 0 {
		return fmt.Errorf("a refused appliance run answered a verdict: %v", uiLeEvidenceAnswersSortedKeys(report))
	}

	// The shared operators render the same command payloads. The whole-tree
	// verb runs at a terminal, the deployment proof over pipes.
	renderings := []struct {
		name, verb, operator, needle string
		wholeTree                    bool
	}{
		{areaEvidence, actionReleaseCandidate, renderYAML, "passed:", true},
		{areaDeployment, actionL2TPTest, renderYAML, "peer:", false},
	}
	for _, item := range renderings {
		args := []string{item.name, item.verb, "|", item.operator}
		record := filepath.Join(work, item.name+"-"+item.operator)
		var out uiLeEvidenceAnswersCommandResult
		if item.wholeTree {
			out = leAtTerminal(args, "", "", record)
		} else {
			out = le(args, record, nil)
		}
		if !strings.Contains(out.stdout, item.needle) {
			return fmt.Errorf("`le %s %s | %s` did not render the payload: %q",
				item.name, item.verb, item.operator, uiLeEvidenceAnswersTail(out.stdout, 200))
		}
	}

	// Unknown actions and trailing values remain distinguishable from a gate
	// which ran and failed, and neither rejected invocation performs work.
	missing := le([]string{areaEvidence, "no-such-action"}, "", nil)
	if missing.code != 2 {
		return fmt.Errorf("an unknown action exited %d, want 2", missing.code)
	}
	extra := le([]string{areaDeployment, actionL2TPTest, "somewhere"}, "", nil)
	if extra.code != 2 {
		return fmt.Errorf("a value after an action exited %d, want 2", extra.code)
	}
	calls, err = uiLeEvidenceAnswersRecorded(defaultDockerRecord)
	if err != nil {
		return err
	}
	if len(calls) != 0 {
		return errors.New("a refused invocation still started something")
	}

	fmt.Println("OK")
	return nil
}
