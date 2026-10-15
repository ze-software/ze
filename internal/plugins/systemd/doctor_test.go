// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Detail: doctor.go -- checkServiceInstall and its registration
//
// These cases came with the check from internal/component/doctor/doctor_test.go,
// one for one, when the check moved onto the registry.

package systemd

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// withServiceUnit stands in a host whose unit file holds data, for the life of
// one test.
func withServiceUnit(t *testing.T, data string) {
	t.Helper()
	previous := readServiceUnitFile
	readServiceUnitFile = func(string) ([]byte, error) { return []byte(data), nil }
	t.Cleanup(func() { readServiceUnitFile = previous })
	withServiceStop(t, "TimeoutStopUSec=1min 30s\nKillMode=mixed\n", nil)
}

// withServiceStop stands in the answer `systemctl show` gives for the unit's
// stop settings, for the life of one test.
func withServiceStop(t *testing.T, output string, err error) {
	t.Helper()
	previous := showServiceStop
	showServiceStop = func() ([]byte, error) { return []byte(output), err }
	t.Cleanup(func() { showServiceStop = previous })
}

// TestDoctorUnitStopTimeoutBelowCeiling drives the stop-settings check over
// the effective values systemd reports, drop-ins and distro defaults included.
//
// VALIDATES: spec-daemon-drain-before-stop AC-15: a stop timeout below the
// drain ceiling plus the teardown margin, or a kill mode other than mixed,
// is a warning under doctor-service-stop; the derived values, a longer
// timeout and infinity are silent; an unreadable answer is a warning, never
// a silent pass.
// PREVENTS: an installed unit, or a drop-in over it, that lets systemd SIGKILL
// ze mid-drain or SIGTERM its external plugins before ze drains them.
func TestDoctorUnitStopTimeoutBelowCeiling(t *testing.T) {
	cases := []struct {
		name   string
		output string
		err    error
		want   int
	}{
		{"derived values", "TimeoutStopUSec=1min 30s\nKillMode=mixed\n", nil, 0},
		{"longer timeout", "TimeoutStopUSec=2min\nKillMode=mixed\n", nil, 0},
		{"no timeout", "TimeoutStopUSec=infinity\nKillMode=mixed\n", nil, 0},
		{"below ceiling plus margin", "TimeoutStopUSec=1min 29s\nKillMode=mixed\n", nil, 1},
		// Printed by systemd 257 for TimeoutStopSec=89.999s (spec A-4 row).
		{"fractional below", "TimeoutStopUSec=1min 29.999000s\nKillMode=mixed\n", nil, 1},
		{"generated unit under systemd 257", "TimeoutStopUSec=1min 30s\nKillMode=mixed\n", nil, 0},
		{"months", "TimeoutStopUSec=3month\nKillMode=mixed\n", nil, 0},
		{"distro default kill mode", "TimeoutStopUSec=1min 30s\nKillMode=control-group\n", nil, 1},
		{"both wrong", "TimeoutStopUSec=10s\nKillMode=process\n", nil, 2},
		{"systemctl fails", "", errors.New("exit status 1"), 1},
		{"timeout missing", "KillMode=mixed\n", nil, 1},
		{"timeout unparsable", "TimeoutStopUSec=soon\nKillMode=mixed\n", nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withServiceStop(t, tc.output, tc.err)
			diags := checkServiceStop("/etc/systemd/system/ze.service")
			if len(diags) != tc.want {
				t.Fatalf("diagnostics = %d, want %d: %v", len(diags), tc.want, diags)
			}
			for i := range diags {
				if diags[i].Code != diagnostic.CodeDoctorServiceStop {
					t.Errorf("code = %q, want %q", diags[i].Code, diagnostic.CodeDoctorServiceStop)
				}
				if diags[i].Severity != diagnostic.SeverityWarning {
					t.Errorf("severity = %v, want warning", diags[i].Severity)
				}
				if !strings.Contains(diags[i].Message, "ze install systemd --force") &&
					!strings.Contains(diags[i].Message, "systemctl show") {
					t.Errorf("message names no fix: %q", diags[i].Message)
				}
			}
		})
	}
}

// TestParseSystemdTimespan reads the spans systemd's format_timespan writes.
//
// VALIDATES: every unit format_timespan prints (y, month, w, d, h, min, s, ms,
// us), the strings systemd 257 printed for real units (spec A-4 row), and the
// refusals: an unknown unit, a bare number, a negative, an empty value.
// PREVENTS: a unit matched by another's tail ("3month" read as "3mont" hours),
// and a value doctor cannot read passing as a zero.
func TestParseSystemdTimespan(t *testing.T) {
	cases := []struct {
		value string
		want  time.Duration
		bad   bool
	}{
		// Printed by `systemctl show -p TimeoutStopUSec` under systemd 257.
		{"1min 30s", 90 * time.Second, false},
		{"1min 29.999000s", 89999 * time.Millisecond, false},
		{"2s", 2 * time.Second, false},
		{"500ms", 500 * time.Millisecond, false},
		{"3month", 3 * 2629800 * time.Second, false},
		{"1y", 31557600 * time.Second, false},
		{"1h 30min", 90 * time.Minute, false},
		{"1d 1h 1min 1.500000s", 90061500 * time.Millisecond, false},
		// The remaining units format_timespan writes.
		{"2w", 14 * 24 * time.Hour, false},
		{"250us", 250 * time.Microsecond, false},
		// Refusals.
		{"", 0, true},
		{"90", 0, true},
		{"3mont", 0, true},
		{"5M", 0, true},
		{"-5s", 0, true},
		{"1.2.3s", 0, true},
		{"soon", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			got, unlimited, err := parseSystemdTimespan(tc.value)
			if tc.bad {
				if err == nil {
					t.Fatalf("parseSystemdTimespan(%q) = %v, want an error", tc.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSystemdTimespan(%q): %v", tc.value, err)
			}
			if unlimited {
				t.Fatalf("parseSystemdTimespan(%q) answered unlimited", tc.value)
			}
			if got != tc.want {
				t.Fatalf("parseSystemdTimespan(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
	if _, unlimited, err := parseSystemdTimespan("infinity"); err != nil || !unlimited {
		t.Fatalf("infinity: unlimited=%v err=%v, want unlimited", unlimited, err)
	}
}

// TestStopQueryFailureNamesTheCause drives the warning for a stop query that
// gave no answer.
//
// VALIDATES: a host with no systemctl is told it does not run systemd, with a
// fix that works there (remove the unit), and never the systemctl command it
// lacks; any other failure carries systemctl's own stderr.
// PREVENTS: "exit status 1" with no reason, and a fix that cannot run.
func TestStopQueryFailureNamesTheCause(t *testing.T) {
	const unitPath = "/etc/systemd/system/ze.service"
	missing := stopQueryFailure(unitPath, &exec.Error{Name: "systemctl", Err: exec.ErrNotFound})
	if !strings.Contains(missing.Message, "systemctl not found: this host does not run systemd") {
		t.Errorf("message = %q, want the missing-systemctl cause", missing.Message)
	}
	if !strings.Contains(missing.Message, "sudo rm "+unitPath) {
		t.Errorf("message = %q, want the removal fix", missing.Message)
	}
	if strings.Contains(missing.Message, "systemctl show") {
		t.Errorf("message = %q tells a host without systemctl to run it", missing.Message)
	}

	bus := stopQueryFailure(unitPath, &exec.ExitError{
		Stderr: []byte("Failed to connect to system scope bus via local transport: No such file or directory\n"),
	})
	if !strings.Contains(bus.Message, "Failed to connect to system scope bus") {
		t.Errorf("message = %q, want systemctl's stderr", bus.Message)
	}
}

// TestQueryServiceStopFollowsTheTestSeam proves a run pointed at a test unit
// never asks the host's systemd about the host's ze.service.
//
// VALIDATES: with ze.test.doctor.service-stop set, the answer is that file;
// with only ze.test.doctor.service-unit set, the query refuses.
// PREVENTS: a functional test whose verdict depends on the machine it runs on.
func TestQueryServiceStopFollowsTheTestSeam(t *testing.T) {
	t.Cleanup(env.ResetCache)
	stopPath := filepath.Join(t.TempDir(), "stop")
	if err := os.WriteFile(stopPath, []byte("TimeoutStopUSec=1min 30s\nKillMode=mixed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(doctorServiceUnitEnv, "ze.service")
	t.Setenv(doctorServiceStopEnv, stopPath)
	env.ResetCache()
	output, err := queryServiceStop()
	if err != nil {
		t.Fatalf("queryServiceStop: %v", err)
	}
	if string(output) != "TimeoutStopUSec=1min 30s\nKillMode=mixed\n" {
		t.Fatalf("queryServiceStop = %q, want the seam file", output)
	}

	t.Setenv(doctorServiceStopEnv, "")
	env.ResetCache()
	if _, err := queryServiceStop(); !errors.Is(err, errStopSeamUnset) {
		t.Fatalf("queryServiceStop with no stop seam: err = %v, want %v", err, errStopSeamUnset)
	}
}

// TestStopDrainLiteralsMatchTheConstants checks the catalog entry and the
// operator page, which write the drain budget as numbers, against the
// constants the daemon runs on.
//
// VALIDATES: the doctor-service-stop description and docs/guide/health-checks.md
// name the drain ceiling, the teardown margin and their sum as rpc declares
// them.
// PREVENTS: a budget change that leaves the operator reading the old numbers.
func TestStopDrainLiteralsMatchTheConstants(t *testing.T) {
	ceiling := strconv.Itoa(rpc.DrainCeilingSeconds) + " s"
	margin := strconv.Itoa(rpc.DrainTeardownMarginSeconds) + " s"
	total := strconv.Itoa(stopTimeoutSeconds) + " s"

	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup(diagnostic.CodeDoctorServiceStop)
	if meta == nil {
		t.Fatalf("%s is not in the catalog", diagnostic.CodeDoctorServiceStop)
	}
	for _, want := range []string{"up to " + ceiling, "up to " + margin, "at least " + total, "below " + total} {
		if !strings.Contains(meta.Description, want) {
			t.Errorf("catalog description lacks %q: %s", want, meta.Description)
		}
	}

	page, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "guide", "health-checks.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "at least " + total + " (" + strconv.Itoa(rpc.DrainCeilingSeconds) + " s drain ceiling plus " + margin + " teardown)"
	if !strings.Contains(string(page), want) {
		t.Errorf("docs/guide/health-checks.md lacks %q", want)
	}
}

// TestCheckServiceInstallReportsServiceStop proves the stop-settings check is
// reached from the registered check over an installed unit, not only when
// called directly.
//
// VALIDATES: AC-15 through checkServiceInstall, the function `ze doctor` runs.
// PREVENTS: a stop-settings check no doctor run reaches.
func TestCheckServiceInstallReportsServiceStop(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withServiceUnit(t, "[Service]\nExecStart="+binPath+" start\n")
	withServiceStop(t, "TimeoutStopUSec=1min\nKillMode=mixed\n", nil)

	codes := codesOf(checkServiceInstall(diagnostic.DoctorCheckContext{}))
	if _, ok := codes[diagnostic.CodeDoctorServiceStop]; !ok {
		t.Fatalf("diagnostics carry no %q: %v", diagnostic.CodeDoctorServiceStop, codes)
	}
}

// withMissingServiceExecutable stands in a host where no ExecStart binary
// exists.
func withMissingServiceExecutable(t *testing.T) {
	t.Helper()
	previous := statServiceExecutable
	statServiceExecutable = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { statServiceExecutable = previous })
}

func platformContext(platformType host.PlatformType) diagnostic.DoctorCheckContext {
	return diagnostic.DoctorCheckContext{Platform: &host.PlatformInfo{Type: platformType}}
}

func codesOf(diags []diagnostic.Diagnostic) map[string]diagnostic.Severity {
	codes := make(map[string]diagnostic.Severity, len(diags))
	for i := range diags {
		codes[diags[i].Code] = diags[i].Severity
	}
	return codes
}

// TestCheckServiceInstallMissingAccountAndExecutable drives the check over a
// unit whose user, group and ExecStart all name something the host lacks.
//
// VALIDATES: doctor reports the missing service user and group and the
// non-executable ExecStart, one diagnostic each.
// PREVENTS: `ze install systemd` regressions that leave a unit systemd cannot
// execute.
func TestCheckServiceInstallMissingAccountAndExecutable(t *testing.T) {
	withServiceUnit(t, "[Service]\nUser=ze\nGroup=ze\nExecStart=/missing/ze start\n")
	withMissingServiceExecutable(t)
	previousUser, previousGroup := lookupServiceUser, lookupServiceGroup
	lookupServiceUser = func(string) (*user.User, error) { return nil, errors.New("missing user") }
	lookupServiceGroup = func(string) (*user.Group, error) { return nil, errors.New("missing group") }
	t.Cleanup(func() {
		lookupServiceUser = previousUser
		lookupServiceGroup = previousGroup
	})

	diags := checkServiceInstall(diagnostic.DoctorCheckContext{})
	if len(diags) != 3 {
		t.Fatalf("diagnostics = %d, want 3: %v", len(diags), diags)
	}
	codes := codesOf(diags)
	for _, code := range []string{
		diagnostic.CodeDoctorServiceExecutable,
		diagnostic.CodeDoctorServiceUser,
		diagnostic.CodeDoctorServiceGroup,
	} {
		if _, ok := codes[code]; !ok {
			t.Errorf("diagnostics carry no %q: %v", code, diags)
		}
	}
}

// TestCheckServiceInstallExecutableOK drives the check over a unit whose
// ExecStart names a file that exists with executable bits.
//
// VALIDATES: no diagnostic for a unit systemd can run.
// PREVENTS: a false-positive service executable diagnostic after `ze install`.
func TestCheckServiceInstallExecutableOK(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withServiceUnit(t, "[Service]\nExecStart="+binPath+" start\n")

	if diags := checkServiceInstall(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
}

// TestParseServiceUnitLastExecStartWins reads a unit whose drop-in clears and
// re-sets ExecStart.
//
// VALIDATES: systemd override semantics, where the last ExecStart= wins.
// PREVENTS: a false "no ExecStart" over a unit systemd itself accepts.
func TestParseServiceUnitLastExecStartWins(t *testing.T) {
	unit := parseServiceUnit([]byte("[Service]\nExecStart=\nExecStart=/opt/ze/bin/ze start\n"))
	if unit.execStart != "/opt/ze/bin/ze" {
		t.Fatalf("execStart = %q, want /opt/ze/bin/ze", unit.execStart)
	}
}

// TestParseServiceUnitIgnoresNonServiceSection reads a unit that carries a
// User key under [Unit].
//
// VALIDATES: the parser reads keys from [Service] only, never from [Unit] or
// [Install].
// PREVENTS: a false diagnostic from a key in the wrong section of an
// operator-edited unit.
func TestParseServiceUnitIgnoresNonServiceSection(t *testing.T) {
	unit := parseServiceUnit([]byte("[Unit]\nDescription=Ze\nUser=bogus\n\n[Service]\nExecStart=/usr/bin/ze start\nUser=ze\nGroup=ze\n\n[Install]\nWantedBy=multi-user.target\n"))
	if unit.execStart != "/usr/bin/ze" {
		t.Errorf("execStart = %q, want /usr/bin/ze", unit.execStart)
	}
	if unit.user != "ze" {
		t.Errorf("user = %q, want ze: User must be read from [Service], not [Unit]", unit.user)
	}
	if unit.group != "ze" {
		t.Errorf("group = %q, want ze", unit.group)
	}
}

// TestFirstSystemdCommandStripsAllPrefixes feeds every systemd exec prefix.
//
// VALIDATES: the prefixes -, +, !, !!, @ and : are stripped from ExecStart.
// PREVENTS: a confusing "not an absolute path" diagnostic on a prefixed exec
// line.
func TestFirstSystemdCommandStripsAllPrefixes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/usr/bin/ze start", "/usr/bin/ze"},
		{"-/usr/bin/ze start", "/usr/bin/ze"},
		{"+/usr/bin/ze start", "/usr/bin/ze"},
		{"!!/usr/bin/ze start", "/usr/bin/ze"},
		{"@/usr/bin/ze start", "/usr/bin/ze"},
		{":/usr/bin/ze start", "/usr/bin/ze"},
		{"-+/usr/bin/ze start", "/usr/bin/ze"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := firstSystemdCommand(tt.input); got != tt.want {
			t.Errorf("firstSystemdCommand(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestCheckServiceInstallSkipsGokrazy and its container twin drive the check
// on a platform that has no systemd.
//
// VALIDATES: the unit is not read on gokrazy, and nothing is reported.
// PREVENTS: appliance readiness depending on a file the appliance never
// carries.
func TestCheckServiceInstallSkipsGokrazy(t *testing.T) {
	called := false
	previous := readServiceUnitFile
	readServiceUnitFile = func(string) ([]byte, error) {
		called = true
		return []byte("[Service]\nExecStart=/missing/ze\n"), nil
	}
	t.Cleanup(func() { readServiceUnitFile = previous })

	diags := checkServiceInstall(platformContext(host.PlatformGokrazy))
	if called {
		t.Error("the systemd unit must not be read on gokrazy")
	}
	if len(diags) != 0 {
		t.Errorf("diagnostics = %v, want none", diags)
	}
}

// VALIDATES: the unit is not read in a container, and nothing is reported.
// PREVENTS: a container doctor run warning about a host-level systemd unit.
func TestCheckServiceInstallSkipsContainer(t *testing.T) {
	called := false
	previous := readServiceUnitFile
	readServiceUnitFile = func(string) ([]byte, error) {
		called = true
		return []byte("[Service]\nExecStart=/missing/ze\n"), nil
	}
	t.Cleanup(func() { readServiceUnitFile = previous })

	diags := checkServiceInstall(platformContext(host.PlatformContainer))
	if called {
		t.Error("the systemd unit must not be read in a container")
	}
	if len(diags) != 0 {
		t.Errorf("diagnostics = %v, want none", diags)
	}
}

// TestCheckServiceInstallRunsOnSystemd drives the check on the platform the
// unit is written for.
//
// VALIDATES: the systemd platform still validates the installed unit, and a
// missing executable is an error.
// PREVENTS: the platform skip disabling the one readiness check systemd hosts
// need.
func TestCheckServiceInstallRunsOnSystemd(t *testing.T) {
	withServiceUnit(t, "[Service]\nExecStart=/missing/ze start\n")
	withMissingServiceExecutable(t)

	diags := checkServiceInstall(platformContext(host.PlatformSystemd))
	severity, ok := codesOf(diags)[diagnostic.CodeDoctorServiceExecutable]
	if !ok {
		t.Fatalf("diagnostics carry no %q: %v", diagnostic.CodeDoctorServiceExecutable, diags)
	}
	if severity != diagnostic.SeverityError {
		t.Fatalf("severity = %q, want error", severity)
	}
}

// TestServiceDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this plugin's check, at the phase and order the declaration
// states, and whether every code it declares resolves for `ze show diagnostic`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestServiceDoctorCheckRegistered(t *testing.T) {
	want := serviceDoctorCheck()
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is not registered", code)
		}
	}
}
