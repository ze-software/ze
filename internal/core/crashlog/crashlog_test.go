package crashlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(strings.ReplaceAll(key, ".", "_"), value)
	env.ResetCache()
}

func TestBuildCrashReport(t *testing.T) {
	startTime = time.Now().Add(-3 * time.Hour)
	report := buildCrashReport("test panic", []byte("goroutine 1 [running]:\nmain.main()\n"))

	if !strings.Contains(report, "=== Ze Crash Report ===") {
		t.Error("missing report header")
	}
	if !strings.Contains(report, "=== Panic ===") {
		t.Error("missing panic section")
	}
	if !strings.Contains(report, "test panic") {
		t.Error("missing panic value")
	}
	if !strings.Contains(report, "=== Stack Trace ===") {
		t.Error("missing stack trace section")
	}
	if !strings.Contains(report, "goroutine 1 [running]:") {
		t.Error("missing stack trace content")
	}
	if !strings.Contains(report, "Uptime: 3h0m0s") {
		t.Error("missing uptime")
	}
}

func TestBuildCrashReportErrorValue(t *testing.T) {
	report := buildCrashReport(os.ErrNotExist, nil)
	if !strings.Contains(report, "file does not exist") {
		t.Error("error value not formatted")
	}
}

func TestBuildCrashReportUnknownValue(t *testing.T) {
	report := buildCrashReport(42, nil)
	if !strings.Contains(report, "<unknown panic value>") {
		t.Error("unknown value not handled")
	}
}

func TestWriteCrashFile(t *testing.T) {
	dir := t.TempDir()
	writeCrashFile(dir, 5, "test crash report content")

	files := listCrashFileNames(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 crash file, got %d", len(files))
	}

	data, err := os.ReadFile(filepath.Join(dir, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test crash report content" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestCrashFileRotation(t *testing.T) {
	dir := t.TempDir()

	for i := range 7 {
		name := "crash-20260518-00000" + string(rune('0'+i)) + ".log"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("crash"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	rotateCrashFiles(dir, 3)

	files := listCrashFileNames(dir)
	if len(files) != 3 {
		t.Fatalf("expected 3 files after rotation, got %d", len(files))
	}
	if files[0] != "crash-20260518-000004.log" {
		t.Errorf("expected oldest kept file to be crash-20260518-000004.log, got %s", files[0])
	}
}

func TestPanicPatternDetection(t *testing.T) {
	tests := []struct {
		line  string
		match bool
	}{
		{"goroutine 1 [running]:", true},
		{"goroutine 42 [running]:", true},
		{"goroutine 1234567 [running]:", true},
		{"normal log output", false},
		{"goroutine dump requested", false},
		{"", false},
	}

	for _, tt := range tests {
		got := panicPattern.MatchString(tt.line)
		if got != tt.match {
			t.Errorf("panicPattern.MatchString(%q) = %v, want %v", tt.line, got, tt.match)
		}
	}
}

func TestAutodetectExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, "ze.crash.dir", dir)

	result := resolveCrashDir()
	if result != dir {
		t.Errorf("expected %s, got %s", dir, result)
	}
}

func TestAutodetectExplicitFallsThrough(t *testing.T) {
	setEnv(t, "ze.crash.dir", "/nonexistent/path/that/cannot/exist")

	result := resolveCrashDir()
	if result == "/nonexistent/path/that/cannot/exist" {
		t.Error("should have fallen through from non-writable explicit path")
	}
}

// VALIDATES: with no ze.crash.dir, crash logs land under the pinned ze.config.dir.
// PREVENTS: crash reports splitting away from the config directory the operator
// actually pinned, landing in a binary-derived tree they never chose and do not
// collect. Deliberate: on systemd and gokrazy the pinned dir and the binary-derived
// dir agree, so this only takes effect where they diverge.
func TestAutodetectFollowsConfigDir(t *testing.T) {
	setEnv(t, "ze.crash.dir", "")
	dir := t.TempDir()
	setEnv(t, "ze.config.dir", dir)

	want := filepath.Join(dir, "crash")
	if result := resolveCrashDir(); result != want {
		t.Errorf("expected %s, got %s", want, result)
	}
}

// VALIDATES: ze.crash.dir still outranks ze.config.dir.
// PREVENTS: the config-dir fallback silently overriding an explicit crash-dir choice.
func TestAutodetectCrashDirBeatsConfigDir(t *testing.T) {
	crashDir := t.TempDir()
	setEnv(t, "ze.crash.dir", crashDir)
	setEnv(t, "ze.config.dir", t.TempDir())

	if result := resolveCrashDir(); result != crashDir {
		t.Errorf("expected %s, got %s", crashDir, result)
	}
}

func TestParseCrashKeepDefault(t *testing.T) {
	setEnv(t, "ze.crash.keep", "")
	if n := parseCrashKeep(); n != defaultKeep {
		t.Errorf("expected %d, got %d", defaultKeep, n)
	}
}

func TestParseCrashKeepClampLow(t *testing.T) {
	setEnv(t, "ze.crash.keep", "0")
	if n := parseCrashKeep(); n != minKeep {
		t.Errorf("expected %d, got %d", minKeep, n)
	}
}

func TestParseCrashKeepClampHigh(t *testing.T) {
	setEnv(t, "ze.crash.keep", "200")
	if n := parseCrashKeep(); n != maxKeep {
		t.Errorf("expected %d, got %d", maxKeep, n)
	}
}

func TestParseSyslogAddr(t *testing.T) {
	tests := []struct {
		addr    string
		network string
		raddr   string
	}{
		{"", "", ""},
		{"/dev/log", "unix", "/dev/log"},
		{"localhost:514", "udp", "localhost:514"},
	}

	for _, tt := range tests {
		network, raddr := parseSyslogAddr(tt.addr)
		if network != tt.network || raddr != tt.raddr {
			t.Errorf("parseSyslogAddr(%q) = (%q, %q), want (%q, %q)",
				tt.addr, network, raddr, tt.network, tt.raddr)
		}
	}
}

func TestHandleCaughtPanic(t *testing.T) {
	dir := t.TempDir()
	crashDir = dir
	crashKeep = 5
	startTime = time.Now()

	HandleCaughtPanic(os.ErrPermission)

	files := listCrashFileNames(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 crash file, got %d", len(files))
	}

	data, err := os.ReadFile(filepath.Join(dir, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "=== Ze Crash Report ===") {
		t.Error("crash file missing metadata header")
	}
	if !strings.Contains(content, "permission denied") {
		t.Error("crash file missing error value")
	}
	if strings.Contains(content, "=== Stack Trace ===") {
		t.Error("caught panic should not include stack trace")
	}
}

func TestHandlePanic(t *testing.T) {
	dir := t.TempDir()
	crashDir = dir
	crashKeep = 5
	origStderr = os.Stderr
	startTime = time.Now()

	HandlePanic("test panic value")

	files := listCrashFileNames(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 crash file, got %d", len(files))
	}

	data, err := os.ReadFile(filepath.Join(dir, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "test panic value") {
		t.Error("crash file missing panic value")
	}
	if !strings.Contains(content, "=== Stack Trace ===") {
		t.Error("crash file missing stack trace")
	}
}
