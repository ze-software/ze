// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Detail: doctor.go -- checkServiceInstall and its registration
//
// These cases came with the check from internal/component/doctor/doctor_test.go,
// one for one, when the check moved onto the registry.

package systemd

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withServiceUnit stands in a host whose unit file holds data, for the life of
// one test.
func withServiceUnit(t *testing.T, data string) {
	t.Helper()
	previous := readServiceUnitFile
	readServiceUnitFile = func(string) ([]byte, error) { return []byte(data), nil }
	t.Cleanup(func() { readServiceUnitFile = previous })
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
// states, and whether every code it declares resolves for `ze explain`.
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
