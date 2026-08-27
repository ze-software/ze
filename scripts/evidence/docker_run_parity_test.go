package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	evidencetool "github.com/ze-software/ze/internal/le/evidence"
	"github.com/ze-software/ze/internal/le/leaction"
)

type recordedDockerRunCall struct {
	Program     string            `json:"program"`
	Arguments   []string          `json:"arguments"`
	Environment map[string]string `json:"environment"`
}

type dockerRunHalf struct {
	Calls  []recordedDockerRunCall
	Tree   map[string]string
	Stdout string
	Stderr string
	Code   int
}

// VALIDATES: both producers make every external call with the same payload,
// preserve both streams, leave the same tree, and propagate inner exits.
// PREVENTS: a partial argv comparison hiding a mount, environment, or cleanup difference.
func TestDockerRunScriptAndCommandParity(t *testing.T) {
	for _, inspectCode := range []int{0, 1} {
		for _, innerCode := range []int{0, 3} {
			name := strconv.Itoa(inspectCode) + "-" + strconv.Itoa(innerCode)
			t.Run(name, func(t *testing.T) {
				fixture := newDockerRunParityFixture(t)
				python := fixture.runPython(t, inspectCode, innerCode)
				fixture.resetEffects(t)
				goHalf, report := fixture.runGo(t, inspectCode, innerCode)

				wantCount := 6
				if inspectCode != 0 {
					wantCount = 7
				}
				if len(python.Calls) != wantCount || len(goHalf.Calls) != wantCount {
					t.Fatalf("absolute command counts: Python %d, Go %d, want %d each", len(python.Calls), len(goHalf.Calls), wantCount)
				}
				if !reflect.DeepEqual(python.Calls, goHalf.Calls) {
					t.Fatalf("full command payloads differ:\nPython %#v\nGo %#v", python.Calls, goHalf.Calls)
				}
				if python.Stdout != goHalf.Stdout || python.Stderr != goHalf.Stderr {
					t.Fatalf("streams differ:\nPython stdout=%q stderr=%q\nGo stdout=%q stderr=%q", python.Stdout, python.Stderr, goHalf.Stdout, goHalf.Stderr)
				}
				if !reflect.DeepEqual(python.Tree, goHalf.Tree) {
					t.Fatalf("tree effects differ:\nPython %#v\nGo %#v", python.Tree, goHalf.Tree)
				}
				if python.Code != innerCode || goHalf.Code != innerCode || report.Code != innerCode || report.InnerExitCode != innerCode {
					t.Fatalf("exit codes: Python %d, Go %d, report %#v", python.Code, goHalf.Code, report)
				}
				if len(report.Plan.Commands) != wantCount {
					t.Fatalf("report command count = %d, want %d", len(report.Plan.Commands), wantCount)
				}
			})
		}
	}
}

// VALIDATES: the Python SIGTERM handler and the native action both remove the
// container and answer 128+SIGTERM.
// PREVENTS: a native cancellation becoming exit 1 or leaving the container alive.
func TestDockerRunSignalCodeAndCleanupParity(t *testing.T) {
	fixture := newDockerRunParityFixture(t)
	python := fixture.runPythonSignal(t)
	fixture.resetEffects(t)
	goHalf := fixture.runGoSignal(t)
	if python.Code != 143 || goHalf.Code != 143 {
		t.Fatalf("signal exit codes: Python %d, Go %d, want 143", python.Code, goHalf.Code)
	}
	if len(python.Calls) != 6 || len(goHalf.Calls) != 6 {
		t.Fatalf("signal command counts: Python %d, Go %d, want 6", len(python.Calls), len(goHalf.Calls))
	}
	if !reflect.DeepEqual(python.Calls, goHalf.Calls) {
		t.Fatalf("signal payloads differ:\nPython %#v\nGo %#v", python.Calls, goHalf.Calls)
	}
	if !reflect.DeepEqual(python.Tree, goHalf.Tree) {
		t.Fatalf("signal trees differ:\nPython %#v\nGo %#v", python.Tree, goHalf.Tree)
	}
}

type dockerRunParityFixture struct {
	root   string
	bin    string
	record string
	env    []string
}

func newDockerRunParityFixture(t *testing.T) *dockerRunParityFixture {
	t.Helper()
	base := t.TempDir()
	fixture := &dockerRunParityFixture{
		root: filepath.Join(base, "tree"), bin: filepath.Join(base, "bin"),
		record: filepath.Join(base, "calls.ndjson"),
	}
	if err := os.MkdirAll(filepath.Join(fixture.root, "scripts", "evidence"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixture.bin, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"docker-run.py", "feature_tags.py"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fixture.root, "scripts", "evidence", name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "scripts", "evidence", "proof.py"), []byte("print('unused')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "feature-gates.txt"), []byte("ze_bgp default\nze_l2tp default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"docker": dockerRunDockerStandin, "go": dockerRunGoStandin} {
		if err := os.WriteFile(filepath.Join(fixture.bin, name), []byte(body), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	clean := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && (strings.HasPrefix(key, "ZE_") || strings.HasPrefix(key, "ze.")) {
			continue
		}
		clean = append(clean, item)
	}
	fixture.env = append(clean,
		"PATH="+fixture.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZE_REPO_ROOT="+fixture.root,
		"ZE_DOCKER_EVIDENCE_IMAGE=parity:image",
		"ZE_DOCKER_EVIDENCE_PLATFORM=linux/arm64",
		"ZE_DOCKER_EVIDENCE_GOARCH=arm64",
		"ZE_FIRST=one",
		"ze.dotted=a:b=c",
		"DOCKER_RUN_RECORD="+fixture.record,
		"DOCKER_RUN_ROOT="+fixture.root,
		"GOCACHE="+filepath.Join(base, "cache"),
	)
	return fixture
}

func (fixture *dockerRunParityFixture) runPython(t *testing.T, inspectCode, innerCode int) dockerRunHalf {
	t.Helper()
	fixture.setScenario(inspectCode, innerCode, false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", filepath.Join(fixture.root, "scripts", "evidence", "docker-run.py"),
		"--env", "ZE_LAST=new", "--env", "PUNCT=x:y=z,[]", "scripts/evidence/proof.py", "iproute2", "python3")
	command.Env = slices.Clone(fixture.env)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := dockerRunParityExitCode(t, err)
	return fixture.half(t, code, stdout.String(), stderr.String())
}

func (fixture *dockerRunParityFixture) runGo(t *testing.T, inspectCode, innerCode int) (dockerRunHalf, evidencetool.DockerRunReport) {
	t.Helper()
	fixture.setScenario(inspectCode, innerCode, false)
	fixture.installEnvironment(t)
	args := leaction.Arguments{
		"script": "scripts/evidence/proof.py", "packages": "iproute2 python3",
		"environment": `["ZE_LAST=new","PUNCT=x:y=z,[]"]`,
	}
	options, err := evidencetool.ParseDockerRunArguments(args, fixture.env)
	if err != nil {
		t.Fatalf("parse Go options: %v", err)
	}
	run := evidencetool.NewDockerRun(fixture.root, options)
	run.Environment = slices.Clone(fixture.env)
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	report, err := run.Execute(context.Background())
	if err != nil {
		t.Fatalf("Go docker run: %v", err)
	}
	return fixture.half(t, report.Code, stdout.String(), stderr.String()), report
}

func (fixture *dockerRunParityFixture) runPythonSignal(t *testing.T) dockerRunHalf {
	t.Helper()
	fixture.setScenario(0, 0, true)
	command := exec.Command("python3", filepath.Join(fixture.root, "scripts", "evidence", "docker-run.py"), "scripts/evidence/proof.py")
	command.Env = slices.Clone(fixture.env)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waitForDockerRunFile(t, filepath.Join(fixture.root, "inner-started"))
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := dockerRunParityExitCode(t, command.Wait())
	return fixture.half(t, code, "", "")
}

func (fixture *dockerRunParityFixture) runGoSignal(t *testing.T) dockerRunHalf {
	t.Helper()
	fixture.setScenario(0, 0, true)
	fixture.installEnvironment(t)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	result := make(chan int, 1)
	go func() {
		_, code := evidencetool.Answer([]string{"docker-run", "script", "scripts/evidence/proof.py"})
		result <- code
	}()
	waitForDockerRunFile(t, filepath.Join(fixture.root, "inner-started"))
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-result:
		return fixture.half(t, code, "", "")
	case <-time.After(time.Minute):
		t.Fatal("native docker action did not stop after SIGTERM")
		return dockerRunHalf{}
	}
}
func (fixture *dockerRunParityFixture) installEnvironment(t *testing.T) {
	t.Helper()
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || (!strings.HasPrefix(key, "ZE_") && !strings.HasPrefix(key, "ze.")) {
			continue
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			os.Setenv(key, value) //nolint:errcheck // restore the test process environment
		})
	}
	for _, item := range fixture.env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			t.Setenv(key, value)
		}
	}
}


func (fixture *dockerRunParityFixture) setScenario(inspectCode, innerCode int, block bool) {
	for _, pair := range []string{
		"DOCKER_INSPECT_CODE=" + strconv.Itoa(inspectCode),
		"DOCKER_INNER_CODE=" + strconv.Itoa(innerCode),
		"DOCKER_INNER_BLOCK=" + strconv.FormatBool(block),
	} {
		key, value, _ := strings.Cut(pair, "=")
		fixture.env = replaceDockerRunParityEnv(fixture.env, key, value)
	}
}

func replaceDockerRunParityEnv(environ []string, key, value string) []string {
	prefix := key + "="
	for index, item := range environ {
		if strings.HasPrefix(item, prefix) {
			environ[index] = prefix + value
			return environ
		}
	}
	return append(environ, prefix+value)
}

func (fixture *dockerRunParityFixture) resetEffects(t *testing.T) {
	t.Helper()
	for _, path := range []string{
		fixture.record, filepath.Join(fixture.root, "inner-result.txt"),
		filepath.Join(fixture.root, "inner-started"), filepath.Join(fixture.root, "removed"),
		filepath.Join(fixture.root, "tmp"),
	} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
}

func (fixture *dockerRunParityFixture) half(t *testing.T, code int, stdout, stderr string) dockerRunHalf {
	t.Helper()
	return dockerRunHalf{
		Calls: readDockerRunCalls(t, fixture.record), Tree: dockerRunTree(t, fixture.root),
		Stdout: stdout, Stderr: stderr, Code: code,
	}
}

var dockerRunContainerPattern = regexp.MustCompile(`ze-evidence-[0-9]+`)

func readDockerRunCalls(t *testing.T, path string) []recordedDockerRunCall {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	calls := make([]recordedDockerRunCall, 0, len(lines))
	for _, line := range lines {
		var call recordedDockerRunCall
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatalf("decode call %q: %v", line, err)
		}
		for index, argument := range call.Arguments {
			call.Arguments[index] = dockerRunContainerPattern.ReplaceAllString(argument, "ze-evidence-PID")
		}
		calls = append(calls, call)
	}
	return calls
}

func dockerRunTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, rel := range []string{"inner-result.txt", "removed", "tmp/evidence/bin/ze-linux-arm64"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil {
			result[rel] = string(data)
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	return result
}

func waitForDockerRunFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func dockerRunParityExitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	t.Fatalf("process failed: %v", err)
	return -1
}

const dockerRunGoStandin = `#!/usr/bin/env python3
import json, os, pathlib, sys
record = os.environ["DOCKER_RUN_RECORD"]
env = {key: os.environ[key] for key in ("GOOS", "GOARCH", "CGO_ENABLED", "GOCACHE", "ZE_FIRST", "ze.dotted") if key in os.environ}
with open(record, "a", encoding="utf-8") as stream:
    json.dump({"program":"go", "arguments":sys.argv[1:], "environment":env}, stream, sort_keys=True)
    stream.write("\n")
out = sys.argv[sys.argv.index("-o") + 1]
pathlib.Path(out).parent.mkdir(parents=True, exist_ok=True)
pathlib.Path(out).write_text("ze\n", encoding="utf-8")
`

const dockerRunDockerStandin = `#!/usr/bin/env python3
import json, os, pathlib, sys, time
root = pathlib.Path(os.environ["DOCKER_RUN_ROOT"])
env = {key: os.environ[key] for key in ("GOOS", "GOARCH", "CGO_ENABLED", "GOCACHE", "ZE_FIRST", "ze.dotted") if key in os.environ}
with open(os.environ["DOCKER_RUN_RECORD"], "a", encoding="utf-8") as stream:
    json.dump({"program":"docker", "arguments":sys.argv[1:], "environment":env}, stream, sort_keys=True)
    stream.write("\n")
args = sys.argv[1:]
if args[:2] == ["image", "inspect"]:
    raise SystemExit(int(os.environ["DOCKER_INSPECT_CODE"]))
if args and args[0] == "pull":
    print("pull-out")
    print("pull-err", file=sys.stderr)
    raise SystemExit(0)
if args and args[0] == "run":
    print("container-id")
    raise SystemExit(0)
if args and args[0] == "rm":
    (root / "removed").write_text("removed\n", encoding="utf-8")
    raise SystemExit(0)
if "apk" in args:
    print("apk-out")
    print("apk-err", file=sys.stderr)
    raise SystemExit(0)
if "python3" in args:
    (root / "inner-result.txt").write_text("inner\n", encoding="utf-8")
    if os.environ["DOCKER_INNER_BLOCK"] == "true":
        (root / "inner-started").write_text("started\n", encoding="utf-8")
        while not (root / "removed").exists():
            time.sleep(0.01)
        raise SystemExit(143)
    print("inner-out")
    print("inner-err", file=sys.stderr)
    raise SystemExit(int(os.environ["DOCKER_INNER_CODE"]))
raise SystemExit(99)
`
