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
)

func init() {
	Register("ui/le-evidence-answers", uiDriver(leEvidenceAnswers))
}

type fixtureFailure string

func (f fixtureFailure) Error() string { return string(f) }

func uiLeEvidenceAnswersRequire(condition bool, format string, args ...any) {
	if !condition {
		panic(fixtureFailure(fmt.Sprintf(format, args...)))
	}
}

func leEvidenceAnswers(ctx context.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			var failure fixtureFailure
			if errors.As(asError(recovered), &failure) {
				err = failure
				return
			}
			panic(recovered)
		}
	}()
	return runLEEvidenceAnswers(ctx)
}

func asError(value any) error {
	if err, ok := value.(error); ok {
		return err
	}
	return fmt.Errorf("%v", value)
}

type uiLeEvidenceAnswersCommandResult struct {
	stdout string
	stderr string
	code   int
	err    error
}

func uiLeEvidenceAnswersRunCommand(ctx context.Context, dir, program string, args []string, env []string) uiLeEvidenceAnswersCommandResult {
	command := exec.CommandContext(ctx, program, args...)
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
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			code = exitError.ExitCode()
		} else {
			code = -1
		}
	}
	return uiLeEvidenceAnswersCommandResult{stdout: stdout.String(), stderr: stderr.String(), code: code, err: err}
}

func uiLeEvidenceAnswersEnvironment(overrides map[string]string) []string {
	values := make(map[string]string, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if found {
			values[key] = value
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func uiLeEvidenceAnswersTail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}

func writeFile(path, contents string, mode os.FileMode) {
	uiLeEvidenceAnswersRequire(os.WriteFile(path, []byte(contents), mode) == nil, "writing %s failed", path)
}

func uiLeEvidenceAnswersRecorded(path string) []string {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	uiLeEvidenceAnswersRequire(err == nil, "reading recording %s failed: %v", path, err)
	parts := strings.Split(string(contents), "\x1e")
	calls := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			calls = append(calls, strings.ReplaceAll(part, "\x1f", " "))
		}
	}
	return calls
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
	uiLeEvidenceAnswersRequire(root != "", "ZE_REPO_ROOT is not set")

	work, err := os.MkdirTemp("", "le-evidence-answers-")
	uiLeEvidenceAnswersRequire(err == nil, "creating fixture work directory failed: %v", err)
	defer os.RemoveAll(work)

	binary, err := uiLEBinary(root)
	uiLeEvidenceAnswersRequire(err == nil, "%v", err)

	checkout := filepath.Join(work, "fixture")
	uiLeEvidenceAnswersRequire(os.MkdirAll(checkout, 0o755) == nil, "creating fixture checkout failed")
	writeFile(filepath.Join(checkout, "go.mod"), "module example.test/m\n\ngo 1.26\n", 0o644)
	writeFile(filepath.Join(checkout, "feature-gates.txt"),
		"ze_bgp internal/component/bgp\nze_l2tp internal/component/l2tp\n", 0o644)

	self, err := os.Executable()
	uiLeEvidenceAnswersRequire(err == nil, "finding fixture executable failed: %v", err)
	stubs, err := os.MkdirTemp(filepath.Dir(self), "le-evidence-stubs-")
	uiLeEvidenceAnswersRequire(err == nil, "creating stub directory failed: %v", err)
	defer os.RemoveAll(stubs)
	goTool, err := exec.LookPath("go")
	uiLeEvidenceAnswersRequire(err == nil, "finding go for tool stubs failed: %v", err)
	stubPath := filepath.Join(stubs, "docker")
	buildStub := exec.CommandContext(ctx, goTool, "build", "-o", stubPath, "./internal/test/toolstub/cmd")
	buildStub.Dir = root
	buildStub.Env = uiLeEvidenceAnswersEnvironment(map[string]string{"CGO_ENABLED": "0"})
	output, buildErr := buildStub.CombinedOutput()
	uiLeEvidenceAnswersRequire(buildErr == nil, "building tool stub failed: %v\n%s", buildErr, output)
	stubBytes, err := os.ReadFile(stubPath)
	uiLeEvidenceAnswersRequire(err == nil, "reading tool stub failed: %v", err)
	for _, name := range []string{"git", "ip", "ping", "xl2tpd", "pppd", "go"} {
		err := os.WriteFile(filepath.Join(stubs, name), stubBytes, 0o755)
		uiLeEvidenceAnswersRequire(err == nil, "writing native %s stub failed: %v", name, err)
	}

	defaultDockerRecord := filepath.Join(work, "docker-argv")
	ipRecord := filepath.Join(work, "ip-argv")
	baseOverrides := map[string]string{
		"PATH":             stubs + string(os.PathListSeparator) + os.Getenv("PATH"),
		"ZE_REPO_ROOT":     checkout,
		"ZE_GIT_STATUS":    "",
		"ZE_DOCKER_EXIT":   "0",
		"ZE_RECORD_DOCKER": defaultDockerRecord,
		"ZE_RECORD_IP":     ipRecord,
	}

	le := func(args []string, status, exitCode, record string, extra map[string]string) uiLeEvidenceAnswersCommandResult {
		overrides := make(map[string]string, len(baseOverrides)+len(extra))
		for key, value := range baseOverrides {
			overrides[key] = value
		}
		overrides["ZE_GIT_STATUS"] = status
		if exitCode != "" {
			overrides["ZE_DOCKER_EXIT"] = exitCode
		}
		if record != "" {
			overrides["ZE_RECORD_DOCKER"] = record
		}
		for key, value := range extra {
			overrides[key] = value
		}
		return uiLeEvidenceAnswersRunCommand(ctx, work, binary, args, uiLeEvidenceAnswersEnvironment(overrides))
	}

	// Both commands must be present in the composition root used by help.
	usage := le([]string{"--help"}, "", "", "", nil)
	page := usage.stdout + usage.stderr
	for _, name := range []string{"evidence", "deployment"} {
		uiLeEvidenceAnswersRequire(strings.Contains(page, name), "le --help does not list the %s command", name)
	}

	// Bare areas list all actions and identify them as read-only checks.
	listings := []struct{ name, verb string }{
		{"evidence", "release-candidate"},
		{"deployment", "l2tp-test"},
		{"deployment", "l2tp-ppp-test"},
		{"deployment", "gokrazy-l2tp-ppp-test"},
	}
	for _, item := range listings {
		listing := le([]string{item.name}, "", "", "", nil)
		uiLeEvidenceAnswersRequire(listing.code == 0, "le %s exited %d", item.name, listing.code)
		uiLeEvidenceAnswersRequire(strings.Contains(listing.stdout, item.verb),
			"le %s does not list %s: %q", item.name, item.verb, listing.stdout)
		uiLeEvidenceAnswersRequire(strings.Contains(listing.stdout, "checks"),
			"le %s does not mark %s read-only", item.name, item.verb)
	}

	// A clean release-candidate check starts exactly one container.
	rcRecord := filepath.Join(work, "rc-argv")
	answer := le([]string{"evidence", "release-candidate"}, "", "", rcRecord, nil)
	uiLeEvidenceAnswersRequire(answer.code == 0,
		"the release-candidate check exited %d: %s", answer.code, uiLeEvidenceAnswersTail(answer.stderr, 800))
	calls := uiLeEvidenceAnswersRecorded(rcRecord)
	uiLeEvidenceAnswersRequire(len(calls) == 1, "the check started %d containers, want 1: %v", len(calls), calls)
	uiLeEvidenceAnswersRequire(strings.Contains(calls[0], "-v "+checkout+":/host:ro"),
		"the tree is not mounted read-only: %s", calls[0])
	uiLeEvidenceAnswersRequire(strings.Contains(calls[0], "git clone --no-local /host /work/src"),
		"the container program does not clone the mount")

	rendered := le([]string{"evidence", "release-candidate", "|", "json"}, "", "",
		filepath.Join(work, "rc-json"), nil)
	var report map[string]any
	err = json.Unmarshal([]byte(rendered.stdout), &report)
	uiLeEvidenceAnswersRequire(err == nil, "the release-candidate JSON is invalid: %v; output %q", err, rendered.stdout)
	for _, key := range []string{"image", "platform", "tree", "dirty", "passed", "code"} {
		uiLeEvidenceAnswersRequire(hasKey(report, key), "the report answered no %q key: %v", key, uiLeEvidenceAnswersSortedKeys(report))
	}
	uiLeEvidenceAnswersRequire(report["passed"] == true, "a container that exited 0 reports %v", report["passed"])
	uiLeEvidenceAnswersRequire(report["tree"] == checkout, "the report names %q, want the fixture", report["tree"])

	// Preserve the container's exact status rather than flattening it.
	for _, code := range []string{"2", "3", "125"} {
		failed := le([]string{"evidence", "release-candidate"}, "", code,
			filepath.Join(work, "rc-"+code), nil)
		want, _ := strconv.Atoi(code)
		uiLeEvidenceAnswersRequire(failed.code == want,
			"a container that exited %s made the command exit %d", code, failed.code)
	}

	// Dirty trees are rejected before any container is started.
	dirtyRecord := filepath.Join(work, "rc-dirty")
	dirty := le([]string{"evidence", "release-candidate"}, " M internal/a.go\n", "",
		dirtyRecord, nil)
	uiLeEvidenceAnswersRequire(dirty.code == 1, "a dirty tree exited %d, want 1", dirty.code)
	uiLeEvidenceAnswersRequire(len(uiLeEvidenceAnswersRecorded(dirtyRecord)) == 0, "a container was started over a dirty tree")
	uiLeEvidenceAnswersRequire(strings.Contains(dirty.stdout, "internal/a.go"),
		"the refusal does not name the path: %q", dirty.stdout)

	// The L2TP proof builds its daemon, starts the peer, and observes a session.
	l2tpRecord := filepath.Join(work, "l2tp-argv")
	proof := le([]string{"deployment", "l2tp-test", "|", "json"}, "", "", l2tpRecord, nil)
	uiLeEvidenceAnswersRequire(proof.code == 0,
		"the L2TP proof exited %d: %s", proof.code, uiLeEvidenceAnswersTail(proof.stderr, 800))
	var verdict map[string]any
	err = json.Unmarshal([]byte(proof.stdout), &verdict)
	uiLeEvidenceAnswersRequire(err == nil, "the L2TP JSON is invalid: %v; output %q", err, proof.stdout)
	for _, key := range []string{"peer", "image", "container", "established", "log-tail"} {
		uiLeEvidenceAnswersRequire(hasKey(verdict, key), "the proof answered no %q key: %v", key, uiLeEvidenceAnswersSortedKeys(verdict))
	}
	uiLeEvidenceAnswersRequire(verdict["established"] == true, "the proof did not read the session")
	uiLeEvidenceAnswersRequire(verdict["peer"] == "xl2tpd", "the proof names the peer %q", verdict["peer"])

	calls = uiLeEvidenceAnswersRecorded(l2tpRecord)
	uiLeEvidenceAnswersRequire(len(calls) == 6, "the proof made %d docker calls, want 6:\n%s",
		len(calls), strings.Join(calls, "\n"))
	peerStarted := false
	privileged := false
	for _, call := range calls {
		peerStarted = peerStarted || strings.Contains(call, "xl2tpd -D")
		privileged = privileged || strings.Contains(call, "--privileged")
	}
	uiLeEvidenceAnswersRequire(peerStarted, "the proof never started the peer:\n%s", strings.Join(calls, "\n"))
	uiLeEvidenceAnswersRequire(privileged, "the container is not privileged:\n%s", strings.Join(calls, "\n"))

	// Both machine-touching proofs reject the kernel-probe escape before making
	// any namespace, lab, image, or container change.
	escaped := le([]string{"deployment", "l2tp-ppp-test", "|", "json"}, "", "", "",
		map[string]string{"ZE_L2TP_SKIP_KERNEL_PROBE": "true"})
	uiLeEvidenceAnswersRequire(escaped.code == 1,
		"a run carrying the kernel-probe escape exited %d, want 1", escaped.code)
	uiLeEvidenceAnswersRequire(strings.Contains(escaped.stdout+escaped.stderr, "ZE_L2TP_SKIP_KERNEL_PROBE"),
		"the refusal does not name the variable: %q", uiLeEvidenceAnswersTail(escaped.stdout+escaped.stderr, 400))
	uiLeEvidenceAnswersRequire(len(uiLeEvidenceAnswersRecorded(ipRecord)) == 0, "the refused run still made a namespace")
	var refused map[string]any
	err = json.Unmarshal([]byte(escaped.stdout), &refused)
	uiLeEvidenceAnswersRequire(err == nil, "the refused proof JSON is invalid: %v; output %q", err, escaped.stdout)
	for _, key := range []string{"peer", "ze-namespace", "lac-namespace", "proven", "local-address"} {
		uiLeEvidenceAnswersRequire(hasKey(refused, key), "the refusal answered no %q key: %v", key, uiLeEvidenceAnswersSortedKeys(refused))
	}
	uiLeEvidenceAnswersRequire(refused["proven"] == false, "a refused run reports itself proven")

	appliance := le([]string{"deployment", "gokrazy-l2tp-ppp-test", "|", "json"}, "", "", "",
		map[string]string{"ZE_L2TP_SKIP_KERNEL_PROBE": "true"})
	uiLeEvidenceAnswersRequire(appliance.code == 1,
		"the appliance proof exited %d on the escape, want 1", appliance.code)
	uiLeEvidenceAnswersRequire(len(uiLeEvidenceAnswersRecorded(ipRecord)) == 0, "the refused appliance run still made a lab")
	report = nil
	err = json.Unmarshal([]byte(appliance.stdout), &report)
	uiLeEvidenceAnswersRequire(err == nil, "the appliance JSON is invalid: %v; output %q", err, appliance.stdout)
	for _, key := range []string{"arch", "accel", "appliance-address", "appliance-interface", "proven"} {
		uiLeEvidenceAnswersRequire(hasKey(report, key), "the appliance report answered no %q key: %v", key, uiLeEvidenceAnswersSortedKeys(report))
	}
	uiLeEvidenceAnswersRequire(report["proven"] == false, "a refused appliance run reports itself proven")

	// The shared operators render the same command payloads.
	renderings := []struct{ name, verb, operator, needle string }{
		{"evidence", "release-candidate", "yaml", "passed:"},
		{"deployment", "l2tp-test", "yaml", "peer:"},
	}
	for _, item := range renderings {
		out := le([]string{item.name, item.verb, "|", item.operator}, "", "",
			filepath.Join(work, item.name+"-"+item.operator), nil)
		uiLeEvidenceAnswersRequire(strings.Contains(out.stdout, item.needle),
			"`le %s %s | %s` did not render the payload: %q",
			item.name, item.verb, item.operator, uiLeEvidenceAnswersTail(out.stdout, 200))
	}

	// Unknown actions and trailing values remain distinguishable from a gate
	// which ran and failed, and neither rejected invocation performs work.
	missing := le([]string{"evidence", "no-such-action"}, "", "", "", nil)
	uiLeEvidenceAnswersRequire(missing.code == 2, "an unknown action exited %d, want 2", missing.code)
	extra := le([]string{"deployment", "l2tp-test", "somewhere"}, "", "", "", nil)
	uiLeEvidenceAnswersRequire(extra.code == 2, "a value after an action exited %d, want 2", extra.code)
	uiLeEvidenceAnswersRequire(len(uiLeEvidenceAnswersRecorded(defaultDockerRecord)) == 0,
		"a refused invocation still started something")

	fmt.Println("OK")
	return nil
}
