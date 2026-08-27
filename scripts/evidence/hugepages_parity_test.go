package main

// AC-11 for the QEMU hugepage gate: effective-vpp-hugepages-qemu.py and
// `le qemu vpp-hugepages-test` do the same thing.
//
// A Python script and a Go command share no process, so one test cannot call
// both directly. This proof instead compares their effects. These effects
// include the host ze build, two appliance commands, appliance configuration,
// virtual machine, and two appliance CLI commands over SSH. Recording stand-ins
// replace go, qemu, and sshpass over a fixture checkout. The test compares their
// argv and the configuration that they leave on disk.
//
// THE STAND-INS PLAY THE APPLIANCE. The go stand-in leaves an executable at the
// requested host ze path. That executable creates the appliance and its image.
// The sshpass stand-in answers each query with JSON from a booted appliance.
// Without these effects, both halves stop at the build. Comparing those stopped
// runs proves nothing.
//
// This file lives beside the script rather than beside the port, so that step 14
// deletes the script and its parity proof in one commit.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/qemu"
)

// hugepagesTimeout bounds one run of the Python original. The stand-ins answer
// at once, so a minute means the script is stuck rather than slow.
const hugepagesTimeout = time.Minute

// hugepagesPort is the forwarded SSH port both halves are given.
//
// The port is FIXED instead of selected because each half would otherwise use a
// different port. The two QEMU argv would then differ. Nothing listens on this
// port because qemu binds nothing and sshpass connects to nothing.
const hugepagesPort = "34122"

// The kernel command line the appliance answers with, and the one that carries
// only the default page-size argument.
//
// The script substring match cannot distinguish the second argument from the
// first. `hugepagesz=2M` is a substring of `default_hugepagesz=2M`. Thus, the
// second script assertion cannot fail by itself.
const (
	fullCmdline    = "console=ttyS0 root=/dev/sda2 default_hugepagesz=2M hugepagesz=2M hugepages=64"
	partialCmdline = "console=ttyS0 root=/dev/sda2 default_hugepagesz=2M hugepages=64"
)

// hugepagesGoStub records the build and writes the host ze the run then drives.
//
// The remaining binary is the appliance stand-in. It records its argv, creates
// the appliance directory, and adds a configuration to patch. It also writes
// the image that the build expects. These effects carry the comparison from the
// build into the boot.
const hugepagesGoStub = `#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_GO"
prev=""
out=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then out="$a"; fi
  prev="$a"
done
[ -z "$out" ] && exit 0
mkdir -p "$(dirname "$out")"
cat > "$out" <<'ZEHOST'
#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_ZE"
case "$1 $2" in
  "appliance init")
    mkdir -p "$ZE_APPLIANCE_DIR/$3"
    printf '%s' '{"name":"'"$3"'","image":{"arch":"amd64"}}' > "$ZE_APPLIANCE_DIR/$3/appliance.json"
    ;;
  "appliance build")
    : > "$ZE_APPLIANCE_DIR/$3/ze-hugepages.img"
    ;;
esac
exit 0
ZEHOST
chmod +x "$out"
exit 0
`

// hugepagesQemuStub records the VM and stays up until it is stopped.
const hugepagesQemuStub = `#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_QEMU"
echo "appliance booting"
sleep 30
exit 0
`

// hugepagesSSHStub records the query and answers it as a booted appliance would.
const hugepagesSSHStub = `#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_SSH"
for a in "$@"; do
  case "$a" in
    "show host kernel | json")
      printf '{"cmdline":"%s"}\n' "${ZE_HP_CMDLINE}" ; exit 0 ;;
    "show host memory | json")
      printf '{"hugepages-total":%s}\n' "${ZE_HP_TOTAL:-64}" ; exit 0 ;;
  esac
done
exit 0
`

// hugepagesToolStub stands in for a prerequisite the run only tests the
// presence of.
const hugepagesToolStub = "#!/bin/sh\nexit 0\n"

// hugepagesRun is what one half of the comparison did.
type hugepagesRun struct {
	code      int
	build     []string
	appliance []string
	vm        []string
	ssh       []string
	config    map[string]any
}

// hugepagesFixture builds a checkout each half can be pointed at.
//
// The fixture gets both ZE_REPO_ROOT and a COPIED script. The script searches
// upward from its own file when the variable is unset. A run from this checkout
// would write a multi-gigabyte image here.
func hugepagesFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/m\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatalf("write the fixture go.mod: %v", err)
	}

	dir := filepath.Join(root, "scripts", "evidence")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create the fixture script directory: %v", err)
	}
	name := "effective-vpp-hugepages-qemu.py"
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
		t.Fatalf("copy %s into the fixture: %v", name, err)
	}

	return root
}

// hugepagesStubs writes every recording program and answers their directory.
func hugepagesStubs(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	programs := map[string]string{
		"go":                     hugepagesGoStub,
		qemu.QemuBinary("amd64"): hugepagesQemuStub,
		qemu.QemuBinary("arm64"): hugepagesQemuStub,
		"sshpass":                hugepagesSSHStub,
		"mkfs.ext4":              hugepagesToolStub,
		"debugfs":                hugepagesToolStub,
	}
	for name, body := range programs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
			t.Fatalf("write the %s stub: %v", name, err)
		}
	}
	return dir
}

// hugepagesEnv is the environment both halves are run under.
//
// KEEP is on because the run removes its work directory otherwise, and the
// appliance configuration this comparison reads lives inside it.
func hugepagesEnv(root, stubDir, record, cmdline string, extra ...string) []string {
	base := []string{
		"PATH=" + stubDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"ZE_REPO_ROOT=" + root,
		"ZE_VPP_HP_SSH_PORT=" + hugepagesPort,
		"ZE_VPP_HP_KEEP=1",
		"ZE_HP_CMDLINE=" + cmdline,
		"ZE_RECORD_GO=" + filepath.Join(record, "go"),
		"ZE_RECORD_ZE=" + filepath.Join(record, "ze"),
		"ZE_RECORD_QEMU=" + filepath.Join(record, "qemu"),
		"ZE_RECORD_SSH=" + filepath.Join(record, "ssh"),
	}
	return append(base, extra...)
}

// hugepagesConfig reads the appliance configuration the run patched.
//
// The test compares the parsed documents instead of their bytes. Python
// preserves the input key order, but Go sorts map keys. Thus, equal documents
// have different bytes. The appliance build consumes the document, so the test
// compares that document.
func hugepagesConfig(t *testing.T, root string) map[string]any {
	t.Helper()

	pattern := filepath.Join(root, "tmp", "vpp-hugepages-qemu", "run-*", "appliances", qemu.ApplianceName, "appliance.json")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) != 1 {
		t.Fatalf("the run wrote %d appliance configurations under %s, want 1 (%v)", len(matches), root, err)
	}

	body, err := os.ReadFile(matches[0]) //nolint:gosec // a path this test's own run wrote
	if err != nil {
		t.Fatalf("read the appliance configuration: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("parse the appliance configuration: %v", err)
	}
	return parsed
}

// hugepagesNormalize removes what cannot match between two runs: the tree each
// was pointed at, and the work directory each made under it.
func hugepagesNormalize(lines []string, root string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(line, root, "<root>")
		out = append(out, replaceRunDir(line))
	}
	return out
}

// replaceRunDir folds the random suffix of one run's work directory away.
func replaceRunDir(line string) string {
	const marker = "vpp-hugepages-qemu/run-"
	at := strings.Index(line, marker)
	if at < 0 {
		return line
	}
	end := at + len(marker)
	for end < len(line) && line[end] != '/' && line[end] != ' ' {
		end++
	}
	return line[:at] + marker + "<run>" + line[end:]
}

// runHugepagesScript runs the Python original over its own fixture.
func runHugepagesScript(t *testing.T, cmdline string, extra ...string) hugepagesRun {
	t.Helper()

	root := hugepagesFixture(t)
	stubDir := hugepagesStubs(t)
	record := t.TempDir()

	ctx, cancel := context.WithTimeout(t.Context(), hugepagesTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", //nolint:gosec // a path this test built
		filepath.Join(root, "scripts", "evidence", "effective-vpp-hugepages-qemu.py"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), hugepagesEnv(root, stubDir, record, cmdline, extra...)...)
	out, err := cmd.CombinedOutput()

	code := 0
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			t.Fatalf("run the script: %v\n%s", err, out)
		}
		code = exit.ExitCode()
	}

	return hugepagesRun{
		code:      code,
		build:     hugepagesNormalize(calls(t, filepath.Join(record, "go")), root),
		appliance: hugepagesNormalize(calls(t, filepath.Join(record, "ze")), root),
		vm:        hugepagesNormalize(calls(t, filepath.Join(record, "qemu")), root),
		ssh:       hugepagesNormalize(calls(t, filepath.Join(record, "ssh")), root),
		config:    hugepagesConfig(t, root),
	}
}

// runHugepagesCommand runs the ported command over its own fixture, through the
// same stand-ins and the same production code path the binary runs.
func runHugepagesCommand(t *testing.T, cmdline string, extra ...string) hugepagesRun {
	t.Helper()

	root := hugepagesFixture(t)
	stubDir := hugepagesStubs(t)
	record := t.TempDir()

	for _, entry := range hugepagesEnv(root, stubDir, record, cmdline, extra...) {
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}
	// env.Get uses a cache built ONCE from os.Environ(). A variable set after the
	// first read is invisible to it. The command half uses this reader for all
	// configuration. Thus, this test rebuilds the cache now and at cleanup. The
	// cleanup prevents this fixture configuration from reaching the next case.
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	run := qemu.NewHugepages(root)
	run.Progress = io.Discard
	// The stand-ins answer at once, so the real three-minute deadline would only
	// slow a failure down.
	run.Deadline = 5 * time.Second

	report, err := run.Run()
	code := 0
	switch {
	case err != nil:
		code = 1
	case report.Verdict == qemu.VerdictFail, report.Verdict == qemu.VerdictUnspecified:
		code = 1
	}

	return hugepagesRun{
		code:      code,
		build:     hugepagesNormalize(calls(t, filepath.Join(record, "go")), root),
		appliance: hugepagesNormalize(calls(t, filepath.Join(record, "ze")), root),
		vm:        hugepagesNormalize(calls(t, filepath.Join(record, "qemu")), root),
		ssh:       hugepagesNormalize(calls(t, filepath.Join(record, "ssh")), root),
		config:    hugepagesConfig(t, root),
	}
}

// VALIDATES: both halves reach the same verdict, build the same host ze, run the
// same two appliance commands, write the same appliance configuration, start the
// same virtual machine and ask the booted appliance the same two questions.
// PREVENTS: every way this port could change what the proof does, none of which
// the one PASS line on stdout would show.
func TestScriptAndCommandProveHugepagesTheSameWay(t *testing.T) {
	script := runHugepagesScript(t, fullCmdline)
	command := runHugepagesCommand(t, fullCmdline)

	if script.code != 0 || command.code != 0 {
		t.Fatalf("the run answered script=%d command=%d, want 0 and 0", script.code, command.code)
	}

	// The test STATES the counts instead of only comparing them, so two inactive
	// halves cannot pass. It expects one host build, two appliance commands, one
	// virtual machine, and two appliance questions.
	for _, one := range []struct {
		half string
		run  hugepagesRun
	}{{"script", script}, {"command", command}} {
		if len(one.run.build) != 1 || len(one.run.appliance) != 2 ||
			len(one.run.vm) != 1 || len(one.run.ssh) != 2 {
			t.Fatalf("the %s made %d go, %d appliance, %d qemu and %d ssh calls, want 1, 2, 1 and 2",
				one.half, len(one.run.build), len(one.run.appliance), len(one.run.vm), len(one.run.ssh))
		}
	}

	sameCalls(t, "go", script.build, command.build)
	sameCalls(t, "appliance", script.appliance, command.appliance)
	sameCalls(t, "qemu", script.vm, command.vm)
	sameCalls(t, "ssh", script.ssh, command.ssh)

	if !reflect.DeepEqual(script.config, command.config) {
		t.Errorf("the appliance configuration differs:\nscript:  %v\ncommand: %v", script.config, command.config)
	}
}

// VALIDATES: the appliance configuration carries the reservation the proof is
// about, in both halves.
// PREVENTS: a comparison that passes because neither half wrote the hugepage
// block at all. The whole gate exists to drive the derived kernel-argument path,
// and that path is reached only through this configuration.
func TestBothHalvesAskTheApplianceToReserveHugepages(t *testing.T) {
	script := runHugepagesScript(t, fullCmdline)
	command := runHugepagesCommand(t, fullCmdline)

	for _, one := range []struct {
		half   string
		config map[string]any
	}{{"script", script.config}, {"command", command.config}} {
		image, ok := one.config["image"].(map[string]any)
		if !ok {
			t.Fatalf("the %s wrote no image section: %v", one.half, one.config)
		}
		pages, ok := image["hugepages"].(map[string]any)
		if !ok {
			t.Fatalf("the %s wrote no hugepages block: %v", one.half, image)
		}
		if pages["size"] != qemu.DefaultReservation || pages["page-size"] != qemu.DefaultPageSize {
			t.Errorf("the %s asked for %v, want size %q and page-size %q",
				one.half, pages, qemu.DefaultReservation, qemu.DefaultPageSize)
		}
		if image["memory"] != qemu.DefaultMemory {
			t.Errorf("the %s gave the appliance %v, want %q", one.half, image["memory"], qemu.DefaultMemory)
		}
	}
}

// VALIDATES: both halves ask the booted appliance BOTH questions. Each question
// uses the operator interface instead of a shell command.
// PREVENTS: an empty comparison. The kernel command line proves only that the
// REQUEST reached the boot arguments. The memory query reports what the kernel
// did. Two halves that ask only the first question still compare equal.
func TestBothHalvesAskTheApplianceBothQuestions(t *testing.T) {
	script := runHugepagesScript(t, fullCmdline)
	command := runHugepagesCommand(t, fullCmdline)

	for _, one := range []struct {
		half string
		ssh  []string
	}{{"script", script.ssh}, {"command", command.ssh}} {
		for _, query := range []string{qemu.KernelQuery, qemu.MemoryQuery} {
			if !anyCall(one.ssh, query) {
				t.Errorf("the %s never asked %q:\n  %s", one.half, query, strings.Join(one.ssh, "\n  "))
			}
		}
	}
}

// VALIDATES: the port FAILS but the script PASSES a kernel command line with
// `default_hugepagesz=2M` and no separate `hugepagesz=2M`.
// PREVENTS: loss of this difference. The script searches for each argument as a
// SUBSTRING. `hugepagesz=2M` is part of `default_hugepagesz=2M`, so that
// assertion cannot fail alone. A path with only the default argument would pass.
// The 2026-08-26 row in
// plan/journal/green-that-could-not-have-been-red.md records this defect. Only
// the port fixes it. When somebody fixes the script, this test fails and must
// be deleted with the script.
func TestThePortRefusesTheCmdlineTheScriptCannotJudge(t *testing.T) {
	script := runHugepagesScript(t, partialCmdline)
	if script.code != 0 {
		t.Fatalf("the script now refuses a cmdline with no standalone hugepagesz (exit %d); delete this test with the script", script.code)
	}

	command := runHugepagesCommand(t, partialCmdline)
	if command.code == 0 {
		t.Error("the port passed a cmdline that never carried hugepagesz as its own argument")
	}
}

// VALIDATES: both halves fail for a kernel that reserved NO pages.
// PREVENTS: loss of the most important assertion. The command line records the
// request, but hugepages-total records the kernel result. That result can fail
// on a machine without enough contiguous memory.
func TestBothHalvesFailWhenTheKernelReservedNothing(t *testing.T) {
	script := runHugepagesScript(t, fullCmdline, "ZE_HP_TOTAL=0")
	command := runHugepagesCommand(t, fullCmdline, "ZE_HP_TOTAL=0")

	if script.code == 0 {
		t.Error("the script passed a kernel that reserved no hugepages")
	}
	if command.code == 0 {
		t.Error("the port passed a kernel that reserved no hugepages")
	}
}
