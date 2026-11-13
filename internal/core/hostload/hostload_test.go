package hostload

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// VALIDATES: Load.Contended requires load > CPUs AND a concurrent test process.
// PREVENTS: the verify status tool and the runner drifting on the "contended"
// verdict -- both now read this single definition.
func TestLoadContended(t *testing.T) {
	tests := []struct {
		name     string
		load     Load
		expected bool
	}{
		{"quiet machine", Load{LoadAvg1: 1.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}, false},
		{"loaded but no concurrent processes", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}, false},
		{"loaded with concurrent le test", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, true},
		{"loaded with concurrent go test", Load{LoadAvg1: 10.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 3}, true},
		{"concurrency but quiet (no load gate)", Load{LoadAvg1: 2.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 4}, false},
		{"at CPU boundary with concurrency", Load{LoadAvg1: 8.0, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, false},
		{"just over CPU boundary with concurrency", Load{LoadAvg1: 8.1, CPUs: 8, ZeProcs: 2, GoTestProcs: 0}, true},
		{"zero CPUs defaults to not contended", Load{LoadAvg1: 0, CPUs: 0, ZeProcs: 0, GoTestProcs: 0}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.load.Contended(); got != tc.expected {
				t.Errorf("Contended() = %v, want %v for %+v", got, tc.expected, tc.load)
			}
		})
	}
}

func TestLoadString(t *testing.T) {
	l := Load{LoadAvg1: 3.5, CPUs: 8, ZeProcs: 2, GoTestProcs: 1}
	s := l.String()
	if s == "" {
		t.Fatal("String() returned empty")
	}
	for _, want := range []string{"3.5", "8", "2", "1"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

// VALIDATES: Snapshot samples this machine, and its Go test count is the one
// GoTestProcesses makes (goTestProcessCount).
// PREVENTS: a Snapshot whose Go test count is wired to nothing and stays zero,
// which would read a loaded machine running tests as uncontended.
func TestSnapshot(t *testing.T) {
	l := Snapshot()
	if l.CPUs <= 0 {
		t.Errorf("CPUs = %d, want > 0", l.CPUs)
	}
	if l.LoadAvg1 < 0 {
		t.Errorf("LoadAvg1 = %f, want >= 0", l.LoadAvg1)
	}
	// This test binary is itself a running `.test` process.
	if l.GoTestProcs < 1 {
		t.Errorf("GoTestProcs = %d, want at least this test binary", l.GoTestProcs)
	}
}

// VALIDATES: an argument list counts as the harness exactly when the program
// is le and its first argument is test.
// PREVENTS: the count reading a process name the harness no longer has, which
// made every run look uncontended once the harness became `le test <name>`.
func TestIsHarnessCommand(t *testing.T) {
	tests := []struct {
		args string
		want bool
	}{
		{"/home/u/ze/bin/le test ospf -p 4\n", true},
		{"le test fixture plugin/x 2165", true},
		{"./bin/le test", true},
		{"/home/u/ze/bin/le verify worktree", false},
		{"le", false},
		{"/usr/bin/le-test ospf", false},
		{"/usr/bin/ze test", false},
		{"go test ./internal/...", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isHarnessCommand(tc.args); got != tc.want {
			t.Errorf("isHarnessCommand(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

// VALIDATES: the process sample harnessProcessCount reads carries a live
// `le test` process, and isHarnessCommand counts that entry.
// Method: copy sh to a file named le and run it as `le test`, where `test` is a
// script in its working directory that blocks on stdin. The assertion looks for
// this child's own entry, so other harness runs on the host cannot move it.
func TestHarnessSampleSeesALiveLeTest(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH")
	}
	dir := t.TempDir()
	// A symlink, not a copy: macOS kills a copy of the signed /bin/sh at exec,
	// so a copied shell never appears in the sample. The symlink still puts
	// `<dir>/le` in argv[0], which is what `ps -eo args=` reports.
	le := filepath.Join(dir, "le")
	if err := os.Symlink(shell, le); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test"), []byte("read line\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	child := exec.CommandContext(t.Context(), le, "test")
	child.Dir = dir
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Wait() //nolint:errcheck // the exit status is not the subject

	defer stdin.Close() //nolint:errcheck // closing ends the child's read

	want := le + " test"
	deadline := time.Now().Add(5 * time.Second)
	for {
		lines, err := processArguments()
		if err != nil {
			t.Fatalf("processArguments: %v", err)
		}
		for _, line := range lines {
			if strings.TrimSpace(line) != want {
				continue
			}
			if !isHarnessCommand(line) {
				t.Fatalf("isHarnessCommand(%q) = false for a live `le test`", line)
			}
			_, _ = io.WriteString(stdin, "done\n")
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no process sample entry %q", want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// linuxMeminfo is the head of a /proc/meminfo recorded on a 16 GiB Linux
// machine.
const linuxMeminfo = `MemTotal:       16303428 kB
MemFree:          812340 kB
MemAvailable:    9876544 kB
Buffers:          412008 kB
Cached:          8123456 kB
SwapCached:            0 kB
Active:          5234112 kB
Inactive:        8901234 kB
SwapTotal:       2097148 kB
SwapFree:        2097148 kB
`

// darwinVMStat is `vm_stat` recorded on an Apple Silicon Mac, whose pages are
// 16384 bytes.
const darwinVMStat = `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               48213.
Pages active:                           1204512.
Pages inactive:                         1187340.
Pages speculative:                        12877.
Pages throttled:                              0.
Pages wired down:                        402118.
Pages purgeable:                          31022.
"Translation faults":                3120459871.
Pages copy-on-write:                   98123411.
Pages zero filled:                   1650982133.
Pages reactivated:                     45120988.
Pages purged:                           9120345.
File-backed pages:                       811245.
Anonymous pages:                        1593484.
Pages stored in compressor:             2981123.
Pages occupied by compressor:            901234.
Decompressions:                        61234987.
Compressions:                          88123456.
Pageins:                               31230987.
Pageouts:                                412398.
Swapins:                                1203987.
Swapouts:                               1509812.
`

// VALIDATES: spec-main-thread-schedules-heavy-work AC-16 at the reader: a
// parser handed text it cannot read answers an error.
// PREVENTS: a failed reading that reaches a report as the number 0, which
// reads as an idle machine or an empty one.
//
// Every case also requires the answered value to be the zero value, so no
// caller that ignored the error could be handed a plausible number.
func TestReadingReportsFailure(t *testing.T) {
	loads := map[string]string{
		"an empty file":              "",
		"blank":                      " \n",
		"garbage":                    "not a load average",
		"two figures":                "0.52 0.58",
		"a figure that is text":      "0.52 high 0.59 1/467 12345",
		"a figure that ends in text": "0.52x 0.58 0.59 1/467 12345",
		"an empty brace":             "{ }",
		"a negative load":            "-1.00 0.58 0.59",
		"not a number":               "NaN 0.58 0.59",
		"not finite":                 "0.52 +Inf 0.59",
	}
	for name, text := range loads {
		if got, err := parseLoadAverages(text); err == nil || got != (Averages{}) {
			t.Errorf("parseLoadAverages(%s) = %+v, %v; want the zero value and an error", name, got, err)
		}
	}

	meminfo := map[string]string{
		"an empty file":             "",
		"garbage":                   "not a meminfo file\n",
		"a missing field":           "MemTotal:       16303428 kB\nMemFree:          812340 kB\n",
		"a count with text":         "MemAvailable:    lots kB\n",
		"a count that ends in text": "MemAvailable:    12x kB\n",
		"a negative count":          "MemAvailable:    -5 kB\n",
		"no unit":                   "MemAvailable:    9876544\n",
		"another unit":              "MemAvailable:    9876544 MB\n",
		"a count too large":         "MemAvailable:    18446744073709551615 kB\n",
		"a longer name":             "MemAvailableHuge:    9876544 kB\n",
	}
	for name, text := range meminfo {
		if got, err := parseMeminfo(text, "MemAvailable"); err == nil || got != 0 {
			t.Errorf("parseMeminfo(%s) = %d, %v; want 0 and an error", name, got, err)
		}
	}
	// MemTotal is the field the lint memory ceiling reads: a digit prefix is
	// not a count there either.
	if got, err := parseMeminfo("MemTotal:       12x kB\n", "MemTotal"); err == nil || got != 0 {
		t.Errorf("parseMeminfo(a MemTotal count that ends in text) = %d, %v; want 0 and an error", got, err)
	}

	vmStat := map[string]string{
		"an empty file":       "",
		"garbage":             "not vm_stat output\n",
		"no page size":        "Mach Virtual Memory Statistics:\nPages free: 1.\nPages inactive: 1.\nPages speculative: 1.\n",
		"a page size of zero": "Mach Virtual Memory Statistics: (page size of 0 bytes)\nPages free: 1.\nPages inactive: 1.\nPages speculative: 1.\n",
		"a page size of text": "Mach Virtual Memory Statistics: (page size of big bytes)\nPages free: 1.\nPages inactive: 1.\nPages speculative: 1.\n",
		"no inactive count":   "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages free: 1.\nPages speculative: 1.\n",
		"a count that is text": "Mach Virtual Memory Statistics: (page size of 16384 bytes)\n" +
			"Pages free: many.\nPages inactive: 1.\nPages speculative: 1.\n",
		"a count that ends in text": "Mach Virtual Memory Statistics: (page size of 16384 bytes)\n" +
			"Pages free: 12x.\nPages inactive: 1.\nPages speculative: 1.\n",
		"a page size that ends in text": "Mach Virtual Memory Statistics: (page size of 16384x bytes)\n" +
			"Pages free: 1.\nPages inactive: 1.\nPages speculative: 1.\n",
		"counts too large": "Mach Virtual Memory Statistics: (page size of 16384 bytes)\n" +
			"Pages free: 18446744073709551615.\nPages inactive: 1.\nPages speculative: 1.\n",
	}
	for name, text := range vmStat {
		if got, err := parseVMStatAvailable(text); err == nil || got != 0 {
			t.Errorf("parseVMStatAvailable(%s) = %d, %v; want 0 and an error", name, got, err)
		}
	}

	if lines, err := processLines(" \n"); err == nil || lines != nil {
		t.Errorf("processLines(blank) = %q, %v; want no line and an error", lines, err)
	}
}

// VALIDATES: available and total memory are read out of what each platform
// prints, in bytes.
// PREVENTS: a unit mistake (kB read as bytes, pages read as bytes), and a
// macOS estimate that counts other pages than the three its comment states.
func TestAvailableMemoryIsParsed(t *testing.T) {
	available, err := parseMeminfo(linuxMeminfo, "MemAvailable")
	if err != nil || available != 9876544*1024 {
		t.Errorf("MemAvailable = %d, %v; want %d bytes", available, err, uint64(9876544*1024))
	}
	total, err := parseMeminfo(linuxMeminfo, "MemTotal")
	if err != nil || total != 16303428*1024 {
		t.Errorf("MemTotal = %d, %v; want %d bytes", total, err, uint64(16303428*1024))
	}

	// Free, speculative and inactive pages, and no other line.
	const pages = 48213 + 12877 + 1187340
	estimate, err := parseVMStatAvailable(darwinVMStat)
	if err != nil || estimate != pages*16384 {
		t.Errorf("the vm_stat estimate = %d, %v; want %d bytes", estimate, err, uint64(pages*16384))
	}

	// A measured zero is a value, not a failure.
	empty := "Mach Virtual Memory Statistics: (page size of 4096 bytes)\n" +
		"Pages free: 0.\nPages inactive: 0.\nPages speculative: 0.\n"
	if zero, err := parseVMStatAvailable(empty); err != nil || zero != 0 {
		t.Errorf("a machine with no reclaimable page = %d, %v; want a measured 0", zero, err)
	}
}

// VALIDATES: the three load averages are read out of each platform's format.
// PREVENTS: the five and fifteen minute figures being swapped, and Linux's
// trailing task and pid fields being read as a load.
func TestLoadAveragesAreParsed(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Averages
	}{
		{"linux /proc/loadavg", "0.52 0.58 0.59 1/467 12345\n", Averages{One: 0.52, Five: 0.58, Fifteen: 0.59}},
		{"darwin sysctl vm.loadavg", "{ 2.11 1.98 1.85 }\n", Averages{One: 2.11, Five: 1.98, Fifteen: 1.85}},
		{"an idle machine", "0.00 0.00 0.00 1/100 4321\n", Averages{}},
	}
	for _, tc := range tests {
		got, err := parseLoadAverages(tc.text)
		if err != nil || got != tc.want {
			t.Errorf("parseLoadAverages(%s) = %+v, %v; want %+v", tc.name, got, err, tc.want)
		}
	}
}

// VALIDATES: a compiled Go test binary is counted by its name, and the count
// of a list that holds none is a measured zero.
func TestGoTestProcessesAreCountedByName(t *testing.T) {
	names := []string{"/usr/bin/ps", "/tmp/go-build1/b001/job.test", "reactor.test", "le", "testing"}
	if got := countGoTests(names); got != 2 {
		t.Errorf("countGoTests = %d, want the 2 names that hold .test", got)
	}
	if got := countGoTests([]string{"ps", "le"}); got != 0 {
		t.Errorf("countGoTests = %d, want 0 for a list with no test binary", got)
	}
}

// VALIDATES: on this machine every reading answers a value or an error, and a
// value that was read is plausible.
// PREVENTS: a platform reader wired to the wrong parser or the wrong unit.
// Method: the real readers. A reading the platform cannot provide is logged,
// never failed: the parsers above are what prove the failure path.
func TestReadingsOnThisMachine(t *testing.T) {
	if averages, err := LoadAverages(); err != nil {
		t.Logf("load averages unavailable: %v", err)
	} else if averages.One < 0 || averages.Five < 0 || averages.Fifteen < 0 {
		t.Errorf("LoadAverages = %+v, want no negative load", averages)
	}

	total, totalErr := MemoryTotal()
	if totalErr != nil {
		t.Logf("total memory unavailable: %v", totalErr)
	} else if total < 1<<28 {
		t.Errorf("MemoryTotal = %d bytes, want at least the 256 MiB a development machine has", total)
	}
	available, availableErr := MemoryAvailable()
	if availableErr != nil {
		t.Logf("available memory unavailable: %v", availableErr)
	}
	if totalErr == nil && availableErr == nil && available > total {
		t.Errorf("MemoryAvailable = %d bytes, more than the %d the machine has", available, total)
	}

	// `ps` is on every machine these tests run on, and this test binary is
	// itself a Go test process.
	tests, err := GoTestProcesses()
	if err != nil {
		t.Fatalf("GoTestProcesses: %v", err)
	}
	if tests < 1 {
		t.Errorf("GoTestProcesses = %d, want at least this test binary", tests)
	}
	if _, err := HarnessProcesses(); err != nil {
		t.Errorf("HarnessProcesses: %v", err)
	}
}
