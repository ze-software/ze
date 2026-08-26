package main

// This file validates AC-11 for the in-VM run. It compares the COMMAND SEQUENCE
// from scripts/evidence/qemu-all-tests.sh with letools/qemu.AllTests. Stand-in
// binaries record the calls, and both sides have asserted call counts.
//
// The comparison covers commands instead of a passing full run. The script
// boots nothing itself, but its commands start suites that need a Linux kernel.
// They also need root, netlink, and one hour of virtual-machine time. NO ONE HAS
// RUN EITHER HALF END TO END FOR THIS PROOF. This test proves that both halves
// start the same programs in the same order with the same arguments. It also
// states each deliberate difference.
//
// Three differences, all in one direction, all named below: the port also runs
// runner, flow-export and vpp. Those three are declared functional suites that
// the script's hand-written list omits entirely, so they execute in no VM phase
// (plan/journal/gate-excludes-part-of-its-population.md, 2026-08-26).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/qemu"
)

// The stand-in programs record every command from either half. `timeout` starts
// a functional suite, `make` starts the unit phase, and `env` starts the
// integration phase.
var qemuStandIns = []string{"timeout", "make", "env"}

// standInBody logs the program's own name and each argument on a line of its
// own. A single line joined by spaces would lose the boundary inside the
// integration phase's tag list, which is one argument holding several words.
const standInBody = `#!/bin/sh
{
  echo "CMD $(basename "$0")"
  for arg in "$@"; do echo "ARG $arg"; done
  echo "END"
} >> "$ZE_PARITY_LOG"
exit 0
`

// qemuWorkspace builds a stand-in guest. It contains the three binaries checked
// by the run and the feature-gate manifest used for integration tags. It also
// contains each required integration package directory.
func qemuWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()

	writeQemuFile(t, workspace, "feature-gates.txt", "ze_bgp on\nze_ssh on\nze_web on\n")
	for _, bin := range []string{"ze", "ze-stripped", "ze-test"} {
		writeQemuExecutable(t, filepath.Join(workspace, "bin", bin), "#!/bin/sh\nexit 0\n")
	}
	for _, dir := range qemuIntegrationDirs() {
		if err := os.MkdirAll(filepath.Join(workspace, dir), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return workspace
}

// qemuIntegrationDirs lists the package directories named by BOTH halves. The
// list is declared here instead of imported. Thus, a change in either half fails
// the test instead of becoming part of a shared constant.
func qemuIntegrationDirs() []string {
	return []string{
		"internal/component/doctor",
		"internal/component/host",
		"internal/component/iface",
		"internal/component/config/system",
		"internal/core/routewatch",
		"internal/core/network",
		"internal/component/bgp/reactor",
		"internal/plugins/fib/kernel",
		"internal/plugins/firewall/nft",
		"internal/plugins/firewall/vpp",
		"internal/plugins/traffic/netlink",
		"internal/plugins/tftpserver",
		"internal/plugins/dhcpserver",
	}
}

func writeQemuFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func writeQemuExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil { //nolint:gosec // a stand-in this test runs
		t.Fatalf("write %s: %v", path, err)
	}
}

// standInDir builds the directory of stand-in programs that log instead of
// running.
func standInDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range qemuStandIns {
		writeQemuExecutable(t, filepath.Join(dir, name), standInBody)
	}
	return dir
}

// runShellHalf runs scripts/evidence/qemu-all-tests.sh against the stand-in
// workspace and answers the command sequence it emitted.
//
// The script hard-codes `cd /workspace`, which the test cannot create. The test
// SOURCES it under a shell that redirects `cd` to the stand-in workspace. This
// is the only harness substitution, and every other script byte runs unchanged.
// An `exit` in the sourced script ends the `bash -c` shell. Thus, the shell
// reports the script exit status.
func runShellHalf(t *testing.T, workspace, log string) [][]string {
	t.Helper()
	for _, tool := range []string{"bash", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s on PATH", tool)
		}
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	script := filepath.Join(repo, "scripts", "evidence", "qemu-all-tests.sh")

	var driver strings.Builder
	driver.WriteString("cd() { builtin cd ")
	driver.WriteString(shellQuote(workspace))
	driver.WriteString("; }\n. ")
	driver.WriteString(shellQuote(script))
	driver.WriteString("\n")

	cmd := exec.Command("bash", "-c", driver.String()) //nolint:gosec,noctx // the script under comparison, with one documented substitution
	cmd.Dir = workspace
	cmd.Env = qemuEnvironment(t, workspace, log)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		var exit *exec.ExitError
		if ok := asExitError(runErr, &exit); !ok || exit.ExitCode() != 0 {
			t.Fatalf("qemu-all-tests.sh: %v\n%s", runErr, out)
		}
	}
	return readCommandLog(t, log)
}

func asExitError(err error, target **exec.ExitError) bool {
	exit, ok := err.(*exec.ExitError) //nolint:errorlint // the concrete type is what carries the code
	if ok {
		*target = exit
	}
	return ok
}

// shellQuote wraps a path for the driver script. The paths are this test's own
// temporary directories, and single quotes are what a shell reads literally.
func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// qemuEnvironment is what both halves are given: the same binaries, the same
// concurrency, the same caps, the same caches.
func qemuEnvironment(t *testing.T, workspace, log string) []string {
	t.Helper()
	return []string{
		"PATH=" + standInDir(t) + ":" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"ZE_PARITY_LOG=" + log,
		"ZE_BIN=" + filepath.Join(workspace, "bin", "ze"),
		"ZE_STRIPPED_BIN=" + filepath.Join(workspace, "bin", "ze-stripped"),
		"ZE_TEST_BIN=" + filepath.Join(workspace, "bin", "ze-test"),
		"ZE_QEMU_SKIP_SUITES=web",
		"ZE_QEMU_PARALLEL=4",
		"ZE_QEMU_SUITE_TIMEOUT=900s",
		"GOCACHE=" + filepath.Join(workspace, "cache", "go-build"),
		"GOMODCACHE=" + filepath.Join(workspace, "cache", "go-mod"),
	}
}

// readCommandLog parses the stand-ins' log into one argv per command.
func readCommandLog(t *testing.T, path string) [][]string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // this test's own log
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var commands [][]string
	var current []string
	for line := range strings.SplitSeq(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "CMD "):
			current = []string{strings.TrimPrefix(line, "CMD ")}
		case strings.HasPrefix(line, "ARG "):
			current = append(current, strings.TrimPrefix(line, "ARG "))
		case line == "END":
			commands = append(commands, current)
			current = nil
		}
	}
	return commands
}

// runPortHalf runs letools/qemu over the same stand-in workspace and answers
// the command sequence it emitted.
//
// Its binary shim uses a temporary directory instead of guest path
// /tmp/ze-qemu-bin. This avoids changes to symlinks used by an active
// ze-qemu-debug session. The comparison therefore normalizes the shim path, and
// a letools/qemu test pins the constant.
func runPortHalf(t *testing.T, workspace string) ([][]string, string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "shim")

	var commands [][]string
	run := &qemu.AllTests{
		Workspace:   workspace,
		BinDir:      binDir,
		ZeBin:       filepath.Join(workspace, "bin", "ze"),
		StrippedBin: filepath.Join(workspace, "bin", "ze-stripped"),
		TestBin:     filepath.Join(workspace, "bin", "ze-test"),
		Skip:        []string{"web"},
		Parallel:    "4",
		Timeout:     "900s",
		BuildCache:  filepath.Join(workspace, "cache", "go-build"),
		ModuleCache: filepath.Join(workspace, "cache", "go-mod"),
		Note:        func(string) {},
		Run: func(argv, _ []string) int {
			commands = append(commands, argv)
			return 0
		},
	}

	if _, code := run.Execute(); code != 0 {
		t.Fatalf("the port exited %d over a workspace where every child answers 0", code)
	}
	return commands, binDir
}

// normalizeCommands makes two command sequences comparable, and it normalizes
// exactly two things.
//
// The binary shim directory, because each half derives it from a constant of
// its own and this harness gives the port a temporary one.
//
// And a TRAILING SPACE on an argument. The shell builds its integration tag
// list with `tr '\n' ' '`, which leaves one after the last tag: the argument is
// "ze_core integration ze_bgp ze_ssh ze_web ". `go test` reads the two
// identically, and the port does not write the space. That is the second and
// last difference between the halves, and trimming here is what makes it a
// stated one rather than a hidden one.
func normalizeCommands(commands [][]string, from, to string) []string {
	lines := make([]string, 0, len(commands))
	for _, argv := range commands {
		trimmed := make([]string, 0, len(argv))
		for _, arg := range argv {
			if from != "" {
				arg = strings.ReplaceAll(arg, from, to)
			}
			trimmed = append(trimmed, strings.TrimRight(arg, " "))
		}
		lines = append(lines, strings.Join(trimmed, "\x1f"))
	}
	return lines
}

// The two halves emit the same commands, in the same order, except for the
// three suites the script omits.
//
// Both counts are ABSOLUTE. The script lists 25 suites and runs two phases, so
// it emits 27 commands with nothing skipped. The port lists 29, skips web, and
// runs the same two phases, so it emits 30.
func TestBothHalvesDriveTheSameCommandsInTheSameOrder(t *testing.T) {
	const (
		shellCommands = 27
		portCommands  = 30
	)

	workspace := qemuWorkspace(t)
	log := filepath.Join(t.TempDir(), "commands.log")
	shell := runShellHalf(t, workspace, log)
	port, binDir := runPortHalf(t, workspace)

	if len(shell) != shellCommands {
		t.Fatalf("the shell emitted %d commands, want exactly %d:\n%s",
			len(shell), shellCommands, strings.Join(normalizeCommands(shell, "", ""), "\n"))
	}
	if len(port) != portCommands {
		t.Fatalf("the port emitted %d commands, want exactly %d:\n%s",
			len(port), portCommands, strings.Join(normalizeCommands(port, binDir, "/tmp/ze-qemu-bin"), "\n"))
	}

	shellLines := normalizeCommands(shell, "", "")
	portLines := normalizeCommands(port, binDir, "/tmp/ze-qemu-bin")

	// The three the port adds, each named, so this case fails when the
	// difference changes rather than merely when it grows.
	added := map[string]bool{"runner": false, "flow-export": false, "vpp": false}
	var common []string
	for _, line := range portLines {
		if suite, isAdded := addedSuite(line, added); isAdded {
			added[suite] = true
			continue
		}
		common = append(common, line)
	}
	for suite, seen := range added {
		if !seen {
			t.Errorf("the port does not run %q, which is a declared suite the script omits", suite)
		}
	}

	if len(common) != len(shellLines) {
		t.Fatalf("after the three added suites the port emits %d commands and the shell %d",
			len(common), len(shellLines))
	}
	for i := range shellLines {
		if common[i] != shellLines[i] {
			t.Errorf("command %d differs:\n  shell: %s\n   port: %s",
				i, readable(shellLines[i]), readable(common[i]))
		}
	}
}

// addedSuite reports whether a command line is one of the three suites the
// script omits, and which.
func addedSuite(line string, added map[string]bool) (string, bool) {
	for suite := range added {
		if strings.Contains(line, "\x1f"+suite+"\x1f") {
			return suite, true
		}
	}
	return "", false
}

// readable spells a recorded command for a failure message.
func readable(line string) string { return strings.ReplaceAll(line, "\x1f", " ") }

// The script's own hand-written list omits four declared suites, and this pins
// that it does.
//
// This row fails when somebody adds these suites to the script. That failure
// shows that the port completeness guard is no longer the only reason these
// suites run (ai/rules/testing.md).
func TestTheShellOmitsFourDeclaredSuites(t *testing.T) {
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(repo, "scripts", "evidence", "qemu-all-tests.sh")) //nolint:gosec // the script under comparison
	if err != nil {
		t.Fatalf("read the script: %v", err)
	}

	for _, suite := range []string{"runner", "flow-export", "vpp", "web"} {
		if strings.Contains(string(body), "\nfsuite "+suite+" ") {
			t.Errorf("scripts/evidence/qemu-all-tests.sh now runs %q; this case pins that it"+
				" does NOT, so update the parity expectation and delete this suite from the list", suite)
		}
	}
}
