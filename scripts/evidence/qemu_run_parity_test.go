package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	qemutool "github.com/ze-software/ze/internal/le/qemu"
)

type recordedQEMUCall struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
	Dotted  []string `json:"dotted"`
}

type qemuHalf struct {
	Calls []recordedQEMUCall
	Tree  map[string]string
	Code  int
}

// VALIDATES: the Python producer and le qemu run make the same host decisions,
// start the same absolute command count, leave the same tree, and map the same
// guest exit status.
// PREVENTS: a partial argv comparison hiding a missing QEMU or SSH effect.
func TestQemuRunScriptAndCommandParity(t *testing.T) {
	for _, guestCode := range []int{0, 3} {
		t.Run(string(rune('0'+guestCode)), func(t *testing.T) {
			fixture := newQEMUParityFixture(t)
			python := fixture.runPython(t, guestCode)
			fixture.resetRunEffects(t)
			goHalf := fixture.runGo(t, guestCode)

			pythonCommands := externalQEMUCommands(python.Calls)
			goCommands := externalQEMUCommands(goHalf.Calls)
			if pythonCommands != 3 || goCommands != 3 {
				t.Fatalf("absolute command counts: Python %d, Go %d, want 3 each", pythonCommands, goCommands)
			}
			if len(python.Calls) != 5 || len(goHalf.Calls) != 5 {
				t.Fatalf("absolute effect counts: Python %d, Go %d, want 5 each", len(python.Calls), len(goHalf.Calls))
			}
			if !reflect.DeepEqual(python.Calls, goHalf.Calls) {
				t.Fatalf("full command payloads differ:\nPython %#v\nGo %#v", python.Calls, goHalf.Calls)
			}
			if !reflect.DeepEqual(python.Tree, goHalf.Tree) {
				t.Fatalf("tree effects differ:\nPython %#v\nGo %#v", python.Tree, goHalf.Tree)
			}
			if python.Code != goHalf.Code || python.Code != guestCode {
				t.Fatalf("exit codes: Python %d, Go %d, guest %d", python.Code, goHalf.Code, guestCode)
			}
		})
	}
}
func externalQEMUCommands(calls []recordedQEMUCall) int {
	count := 0
	for _, call := range calls {
		if call.Program != "serial" {
			count++
		}
	}
	return count
}


type qemuParityFixture struct {
	root   string
	bin    string
	cache  string
	record string
	env    []string
}

func newQEMUParityFixture(t *testing.T) *qemuParityFixture {
	t.Helper()
	base := t.TempDir()
	fixture := &qemuParityFixture{
		root: filepath.Join(base, "tree"), bin: filepath.Join(base, "bin"),
		cache: filepath.Join(base, "cache"), record: filepath.Join(base, "calls.ndjson"),
	}
	if err := os.MkdirAll(filepath.Join(fixture.root, "scripts", "evidence"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fixture.root, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"qemu-run.py", "alpine_iso.py", "homebrew.py"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fixture.root, "scripts", "evidence", name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(fixture.bin, 0o750); err != nil {
		t.Fatal(err)
	}
	fixture.writeStandin(t, qemuParityBinary(), qemuStandin)
	fixture.writeStandin(t, "ssh", sshStandin)
	fixture.seedISO(t)

	fixture.env = append(os.Environ(),
		"PATH="+fixture.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"XDG_CACHE_HOME="+fixture.cache,
		"ZE_QEMU_SSH_PORT=2222", "ZE_QEMU_BOOT_TIMEOUT=30",
		"ZE_QEMU_MEMORY=16384", "ZE_QEMU_CPUS=8",
		"QEMU_RECORD="+fixture.record,
		"ze.l2tp.ncp.ip-timeout=9",
	)
	return fixture
}

func qemuParityBinary() string {
	if runtime.GOARCH == "arm64" {
		return "qemu-system-aarch64"
	}
	return "qemu-system-x86_64"
}

const qemuStandin = `#!/usr/bin/env python3
import json, os, sys
def record(program, args):
    with open(os.environ["QEMU_RECORD"], "a") as f:
        json.dump({"program":program,"args":args,"dotted":sorted(x for x in os.environ if "." in x)}, f)
        f.write("\n")
record("qemu", sys.argv[1:])
print("login:", flush=True)
for line in sys.stdin:
    record("serial", [line.rstrip("\n")])
    if "SSHD_READY" in line:
        print("SSHD_READY", flush=True)
`

const sshStandin = `#!/usr/bin/env python3
import json, os, sys
with open(os.environ["QEMU_RECORD"], "a") as f:
    json.dump({"program":"ssh","args":sys.argv[1:],"dotted":sorted(x for x in os.environ if "." in x)}, f)
    f.write("\n")
if sys.argv[-1] == "true":
    raise SystemExit(0)
raise SystemExit(int(os.environ["QEMU_GUEST_CODE"]))
`

func (f *qemuParityFixture) writeStandin(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.bin, name), []byte(body), 0o750); err != nil {
		t.Fatal(err)
	}
}

func (f *qemuParityFixture) seedISO(t *testing.T) {
	t.Helper()
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	name := "alpine-virt-3.21.3-"+arch+".iso"
	dir := filepath.Join(f.cache, "ze", "alpine-iso")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	payload := []byte("parity iso")
	digest := sha256.Sum256(payload)
	if err := os.WriteFile(filepath.Join(dir, name), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	line := hex.EncodeToString(digest[:])+"  "+name+"\n"
	if err := os.WriteFile(filepath.Join(dir, name+".sha256"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *qemuParityFixture) runPython(t *testing.T, guestCode int) qemuHalf {
	t.Helper()
	if err := os.WriteFile(f.record, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "python3",
		filepath.Join(f.root, "scripts", "evidence", "qemu-run.py"),
		"--run", "printf parity", "--packages", "git bash", "--timeout", "30")
	command.Env = append(append([]string(nil), f.env...), "QEMU_GUEST_CODE="+strconv.Itoa(guestCode))
	output, err := command.CombinedOutput()
	code := exitCode(t, err, output)
	return qemuHalf{Calls: readQEMUCalls(t, f.record), Tree: treeEffect(t, filepath.Join(f.root, "tmp", "qemu")), Code: code}
}

func (f *qemuParityFixture) runGo(t *testing.T, guestCode int) qemuHalf {
	t.Helper()
	if err := os.WriteFile(f.record, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, entry := range append(f.env, "QEMU_GUEST_CODE="+strconv.Itoa(guestCode)) {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			t.Setenv(name, value)
		}
	}
	run := qemutool.NewRun(f.root, qemutool.RunOptions{
		Command: "printf parity", Packages: []string{"git", "bash"},
		Timeout: 30 * time.Second, Boot: 30 * time.Second,
		Memory: "16384", CPUs: "8", SSHPort: 2222,
	})
	report, err := run.Execute(context.Background())
	if err != nil {
		t.Fatalf("Go qemu run: %v", err)
	}
	return qemuHalf{Calls: readQEMUCalls(t, f.record), Tree: treeEffect(t, filepath.Join(f.root, "tmp", "qemu")), Code: reportExit(report)}
}

func reportExit(report qemutool.RunReport) int {
	if report.Verdict == qemutool.RunVerdictPass {
		return 0
	}
	if report.ProofFailure != "" || report.GuestExitCode == 0 {
		return 1
	}
	return report.GuestExitCode
}

func exitCode(t *testing.T, err error, output []byte) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	t.Fatalf("start Python qemu run: %v\n%s", err, output)
	return -1
}

func readQEMUCalls(t *testing.T, name string) []recordedQEMUCall {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	calls := make([]recordedQEMUCall, 0, len(lines))
	for _, line := range lines {
		var call recordedQEMUCall
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatalf("decode recording %q: %v", line, err)
		}
		calls = append(calls, call)
	}
	return calls
}

func treeEffect(t *testing.T, root string) map[string]string {
	t.Helper()
	effect := make(map[string]string)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			effect[filepath.ToSlash(relative)] = "directory"
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		effect[filepath.ToSlash(relative)] = hex.EncodeToString(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return effect
}

func (f *qemuParityFixture) resetRunEffects(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(f.root, "tmp", "qemu")); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: constants imported through alpine_iso.py and homebrew.py stay
// equal to the Go host harness constants.
// PREVENTS: two pins or default wait values drifting while argv still matches.
func TestQemuRunScriptAndCommandShareConstants(t *testing.T) {
	data, err := os.ReadFile("qemu-run.py")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`GO_VERSION = "`+qemutool.GoVersion+`"`,
		`VM_MEMORY = os.environ.get("ZE_QEMU_MEMORY", "`+qemutool.DefaultRunMemory+`")`,
		`VM_CPUS = os.environ.get("ZE_QEMU_CPUS", "`+qemutool.DefaultRunCPUs+`")`,
		`BOOT_TIMEOUT = int(os.environ.get("ZE_QEMU_BOOT_TIMEOUT", "`+
			strconv.FormatInt(int64(qemutool.DefaultBootTimeout/time.Second), 10)+`"))`,
		`DEFAULT_CMD_TIMEOUT = `+strconv.FormatInt(int64(qemutool.DefaultCommandTimeout/time.Second), 10),
		`SCRATCH_MOUNT_TAG = "`+qemutool.ScratchMountTag+`"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("qemu-run.py does not carry %q", want)
		}
	}
	alpine, err := os.ReadFile("alpine_iso.py")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(alpine), `ALPINE_VERSION = "`+qemutool.AlpineVersion+`"`) ||
		!strings.Contains(string(alpine), `ALPINE_MINOR = "`+qemutool.AlpineMinor+`"`) {
		t.Fatal("Alpine version constants differ")
	}
	homebrew, err := os.ReadFile("homebrew.py")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(homebrew), `BREW_DEFAULT_PREFIXES = ("/opt/homebrew", "/usr/local")`) {
		t.Fatal("Homebrew default prefixes differ")
	}
}

