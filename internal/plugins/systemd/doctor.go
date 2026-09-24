// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Overview: register.go -- the init() that installs the registration below
// Related: unit.go -- the unit file this check reads back
//
// The check lived in internal/component/doctor as checkSystemdServiceInstall
// and the runner reached it by writing its name out. It reads the unit
// `ze install systemd` writes, at the path unit.go declares, so it belongs to
// the package that writes that unit (ai/patterns/registration.md, "Doctor
// Check Registry"): a build without this plugin installs no unit and owes no
// judgement over one.

package systemd

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorServiceUnitEnv points the check at a unit file other than
// defaultUnitPath, for the functional tests that write one under tmpfs.
const doctorServiceUnitEnv = "ze.test.doctor.service-unit"

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorServiceUnitEnv,
	Type:        "string",
	Description: "Override ze.service unit path for doctor functional tests",
	Private:     true,
})

// The four probes the check makes against the host, each a variable so a test
// can stand in a host that lacks the unit, the binary, the user or the group.
var (
	readServiceUnitFile   = os.ReadFile
	statServiceExecutable = os.Stat
	lookupServiceUser     = user.Lookup
	lookupServiceGroup    = user.LookupGroup
)

// serviceDoctorCheck is the registration register.go installs.
//
// Order 110 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: it ran after the store-integrity check and before
// the machine-id check, registered at 100 and 120
// (internal/component/doctor/doctor_checks.go).
//
// Platforms carries the wildcard on purpose. The check reads the platform off
// its context and answers nothing on gokrazy and in a container, and it still
// runs when detection failed and the platform is nil, which a platform list
// would stop.
func serviceDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "systemd-service",
		Phase:        diagnostic.DoctorPhasePreConfig,
		Order:        110,
		Component:    "systemd",
		Dependencies: []string{"filesystem"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes: []string{
			diagnostic.CodeDoctorServiceUnit,
			diagnostic.CodeDoctorServiceExecutable,
			diagnostic.CodeDoctorServiceUser,
			diagnostic.CodeDoctorServiceGroup,
		},
		Check: checkServiceInstall,
	}
}

type serviceUnitInfo struct {
	execStart string
	user      string
	group     string
}

// checkServiceInstall reads the installed unit back and reports an ExecStart,
// a User or a Group that systemd could not run.
//
// An absent unit is silent: nothing was installed, so there is nothing to
// judge. On gokrazy and in a container there is no systemd to read, so those
// platforms are skipped whatever the filesystem holds.
func checkServiceInstall(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	platform := ctx.Platform
	if platform != nil && (platform.Type == host.PlatformGokrazy || platform.Type == host.PlatformContainer) {
		return nil
	}

	unitPath := env.Get(doctorServiceUnitEnv)
	if unitPath == "" {
		unitPath = defaultUnitPath
	}

	data, err := readServiceUnitFile(unitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceUnit,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("systemd service unit cannot be read: ").Str(unitPath).Str(": ").Err(err).String(),
			Path:     unitPath,
		}}
	}

	unit := parseServiceUnit(data)
	var diags []diagnostic.Diagnostic
	diags = append(diags, checkServiceExecutable(unitPath, unit.execStart)...)
	if unit.user != "" {
		diags = append(diags, checkServiceUser(unitPath, unit.user)...)
	}
	if unit.group != "" {
		diags = append(diags, checkServiceGroup(unitPath, unit.group)...)
	}
	return diags
}

func parseServiceUnit(data []byte) serviceUnitInfo {
	var unit serviceUnitInfo
	inService := false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inService = line == "[Service]"
			continue
		}
		if !inService {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ExecStart":
			unit.execStart = execStartExecutable(value)
		case "User":
			unit.user = strings.TrimSpace(value)
		case "Group":
			unit.group = strings.TrimSpace(value)
		}
	}
	return unit
}

// execStartExecutable answers the program an ExecStart line runs. A unit ze
// installs starts ze through `/bin/sh -c 'trap "" HUP; exec <ze> start'`
// (buildUnitFile), and the executable that matters is the one the shell execs,
// not the shell.
func execStartExecutable(value string) string {
	cmd := firstSystemdCommand(value)
	if cmd != execStartShell {
		return cmd
	}
	_, script, ok := strings.Cut(value, "; exec ")
	if !ok {
		return cmd
	}
	return firstSystemdCommand(script)
}

func firstSystemdCommand(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	cmd := strings.Trim(fields[0], `"'`)
	for cmd != "" {
		switch cmd[0] {
		case '-', '+', '!', '@', ':':
			cmd = cmd[1:]
		default:
			return cmd
		}
	}
	return cmd
}

func checkServiceExecutable(unitPath, executable string) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	if executable == "" {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("systemd service unit has no ExecStart command: ").Str(unitPath).String(),
			Path:     unitPath,
		}}
	}
	if !filepath.IsAbs(executable) {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("systemd service ExecStart is not an absolute path: ").Str(executable).String(),
			Path:     unitPath,
			Expected: "absolute executable path",
			Actual:   executable,
		}}
	}
	info, err := statServiceExecutable(executable)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("systemd service executable not found: ").Str(executable).Str(": ").Err(err).String(),
			Path:     executable,
		}}
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("systemd service executable is not executable: ").Str(executable).String(),
			Path:     executable,
			Expected: "executable file",
			Actual:   info.Mode().String(),
		}}
	}
	return nil
}

func checkServiceUser(unitPath, name string) []diagnostic.Diagnostic {
	if _, err := lookupServiceUser(name); err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceUser,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("systemd service user not found: ").Str(name).String(),
			Path:     unitPath,
			Expected: "existing user",
			Actual:   name,
		}}
	}
	return nil
}

func checkServiceGroup(unitPath, name string) []diagnostic.Diagnostic {
	if _, err := lookupServiceGroup(name); err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceGroup,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("systemd service group not found: ").Str(name).String(),
			Path:     unitPath,
			Expected: "existing group",
			Actual:   name,
		}}
	}
	return nil
}
