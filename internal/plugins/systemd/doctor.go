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
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// doctorServiceStopEnv names a file holding the `systemctl show -p
// TimeoutStopUSec -p KillMode` answer the stop check reads in place of the
// host's systemd, so a functional test that sets doctorServiceUnitEnv judges
// its own unit and never the host's ze.service.
const doctorServiceStopEnv = "ze.test.doctor.service-stop"

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorServiceStopEnv,
	Type:        "string",
	Description: "File holding the systemctl show stop settings for doctor functional tests",
	Private:     true,
})

// The five probes the check makes against the host, each a variable so a test
// can stand in a host that lacks the unit, the binary, the user or the group,
// or whose systemd applies other stop settings.
var (
	readServiceUnitFile   = os.ReadFile
	statServiceExecutable = os.Stat
	lookupServiceUser     = user.Lookup
	lookupServiceGroup    = user.LookupGroup
	showServiceStop       = queryServiceStop
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
			diagnostic.CodeDoctorServiceStop,
		},
		Check: checkServiceInstall,
	}
}

type serviceUnitInfo struct {
	execStart  string
	execReload []string
	user       string
	group      string
}

// checkServiceInstall reads the installed unit back and reports an ExecStart,
// an ExecReload, a User or a Group that systemd could not run.
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
	diags = append(diags, checkServiceExecutable(unitPath, "ExecStart", unit.execStart)...)
	for _, executable := range unit.execReload {
		diags = append(diags, checkServiceExecutable(unitPath, "ExecReload", executable)...)
	}
	diags = append(diags, checkServiceStop(unitPath)...)
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
		case "ExecReload":
			// ExecReload is a list: an empty assignment clears it, and each
			// later line adds one command.
			cmd := firstSystemdCommand(value)
			if cmd == "" {
				unit.execReload = nil
				continue
			}
			unit.execReload = append(unit.execReload, cmd)
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

// serviceReinstallFix is the remedy for a unit command ze's installer writes.
const serviceReinstallFix = "; run `sudo ze install systemd --force` and `sudo systemctl daemon-reload`"

// checkServiceExecutable reports a program the directive (ExecStart or
// ExecReload) runs that systemd could not execute.
func checkServiceExecutable(unitPath, directive, executable string) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	if executable == "" {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message: tb.Str("systemd service unit has no ").Str(directive).Str(" command: ").Str(unitPath).
				Str(serviceReinstallFix).String(),
			Path: unitPath,
		}}
	}
	if !filepath.IsAbs(executable) {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message: tb.Reset().Str("systemd service ").Str(directive).Str(" is not an absolute path: ").Str(executable).
				Str(serviceReinstallFix).String(),
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
			Message: tb.Reset().Str("systemd service ").Str(directive).Str(" runs ").Str(executable).
				Str(", which cannot be found, so systemd refuses the command: ").Err(err).Str(serviceReinstallFix).String(),
			Path: executable,
		}}
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceExecutable,
			Severity: diagnostic.SeverityError,
			Message: tb.Reset().Str("systemd service ").Str(directive).Str(" runs ").Str(executable).
				Str(", which is not an executable file; run `sudo chmod 0755 ").Str(executable).Str("`").String(),
			Path:     executable,
			Expected: "executable file",
			Actual:   info.Mode().String(),
		}}
	}
	return nil
}

// serviceStopQueryTimeout bounds the one `systemctl show` the stop check
// makes, so a wedged systemd cannot hold a doctor run.
const serviceStopQueryTimeout = 5 * time.Second

// serviceStopFix is the remedy every stop-settings warning ends with.
const serviceStopFix = "; run `sudo ze install systemd --force` and `sudo systemctl daemon-reload`, " +
	"and remove any drop-in `systemctl cat ze.service` shows overriding it"

// errStopSeamUnset refuses the systemctl query in a run that pointed the
// check at a test unit: the host's own ze.service is not the unit under test.
var errStopSeamUnset = errors.New(doctorServiceUnitEnv + " is set but " + doctorServiceStopEnv +
	" is not, and the host's own ze.service is not the unit under test; name a file holding the `systemctl show` answer in " +
	doctorServiceStopEnv)

// queryServiceStop answers the stop settings systemd applies to ze. A run that
// set doctorServiceUnitEnv judges a unit the test wrote, so its stop settings
// come from the file doctorServiceStopEnv names and never from the host.
func queryServiceStop() ([]byte, error) {
	if path := env.Get(doctorServiceStopEnv); path != "" {
		return readServiceUnitFile(path)
	}
	if env.Get(doctorServiceUnitEnv) != "" {
		return nil, errStopSeamUnset
	}
	return systemctlShowStop()
}

// systemctlShowStop asks systemd for the stop settings it applies to ze, as
// it applies them: the unit, its drop-ins and the manager defaults together.
func systemctlShowStop() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceStopQueryTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "systemctl", "show", "-p", "TimeoutStopUSec", "-p", "KillMode", serviceName).Output()
}

// stopQueryFailure answers the warning for a stop query that gave no answer.
// A host with no systemctl does not run systemd, so the unit file is stale and
// the only fix is to remove it; any other failure carries systemctl's own
// stderr, which says why ("Failed to connect to system scope bus").
func stopQueryFailure(unitPath string, err error) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	if errors.Is(err, exec.ErrNotFound) {
		return diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorServiceStop,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("systemctl not found: this host does not run systemd, so the unit at ").Str(unitPath).
				Str(" is never used and its stop settings cannot be read; remove it with `sudo rm ").Str(unitPath).
				Str("`, or install systemd if ze is meant to run under it").String(),
			Path: unitPath,
		}
	}
	tb.Str("the stop settings systemd applies to ").Str(serviceName).
		Str(" cannot be read, so doctor cannot tell whether systemd cuts ze's stop drain short: ").Err(err)
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			tb.Str(" (").Str(stderr).Str(")")
		}
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorServiceStop,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str("; run `systemctl show -p TimeoutStopUSec -p KillMode ").Str(serviceName).Str("` to see why").String(),
		Path:     unitPath,
	}
}

// checkServiceStop reports the stop settings that would cut ze's stop drain
// short: a stop timeout below stopTimeoutSeconds, which lets systemd SIGKILL
// ze mid-drain, and a kill mode other than stopKillMode, which SIGTERMs every
// external plugin before ze drains it (spec-daemon-drain-before-stop AC-15).
// An answer it cannot read is a warning too: silence would claim the settings
// were checked.
func checkServiceStop(unitPath string) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	output, err := showServiceStop()
	if err != nil {
		return []diagnostic.Diagnostic{stopQueryFailure(unitPath, err)}
	}

	var timeoutStop, killMode string
	for line := range strings.SplitSeq(string(output), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "TimeoutStopUSec":
			timeoutStop = value
		case "KillMode":
			killMode = value
		}
	}

	var diags []diagnostic.Diagnostic
	if diag, bad := judgeStopTimeout(unitPath, timeoutStop); bad {
		diags = append(diags, diag)
	}
	if killMode != stopKillMode {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorServiceStop,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("systemd stops ").Str(serviceName).Str(" with KillMode=").Str(killMode).
				Str(", which sends the stop SIGTERM to every external plugin before ze can drain it").
				Str(serviceStopFix).String(),
			Path:     unitPath,
			Expected: "KillMode=" + stopKillMode,
			Actual:   "KillMode=" + killMode,
		})
	}
	return diags
}

// judgeStopTimeout answers the warning for a TimeoutStopUSec value systemd
// reported, and false when the value gives ze its whole stop.
func judgeStopTimeout(unitPath, value string) (diagnostic.Diagnostic, bool) {
	var tb textbuf.Buffer
	stop, unlimited, err := parseSystemdTimespan(value)
	if err != nil {
		return diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorServiceStop,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("systemd reported a stop timeout for ").Str(serviceName).Str(" doctor cannot read (TimeoutStopUSec=").
				Str(value).Str("): ").Err(err).Str("; run `systemctl show -p TimeoutStopUSec ").Str(serviceName).
				Str("` and check the unit").String(),
			Path:   unitPath,
			Actual: "TimeoutStopUSec=" + value,
		}, true
	}
	if unlimited {
		return diagnostic.Diagnostic{}, false
	}
	if stop >= stopTimeoutSeconds*time.Second {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorServiceStop,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("systemd kills ").Str(serviceName).Str(" ").Str(value).Str(" after the stop signal, below the ").
			Int(stopTimeoutSeconds).Str(" s ze needs to drain (").Int(rpc.DrainCeilingSeconds).Str(" s) and tear down (").
			Int(rpc.DrainTeardownMarginSeconds).Str(" s)").Str(serviceStopFix).String(),
		Path:     unitPath,
		Expected: tb.Reset().Str("TimeoutStopSec>=").Int(stopTimeoutSeconds).String(),
		Actual:   "TimeoutStopUSec=" + value,
	}, true
}

// errTimespan is the refusal of a value parseSystemdTimespan cannot read.
var errTimespan = errors.New("not a systemd time span")

// systemdTimespanUnits are the unit names systemd's format_timespan writes
// (src/basic/time-util.c, systemd 257): y, month, w, d, h, min, s, ms, us. A
// field is split into its number and its unit before the unit is looked up, so
// one unit can never match the tail of another ("3month" ends in "h").
var systemdTimespanUnits = map[string]time.Duration{
	"y":     31557600 * time.Second,
	"month": 2629800 * time.Second,
	"w":     7 * 24 * time.Hour,
	"d":     24 * time.Hour,
	"h":     time.Hour,
	"min":   time.Minute,
	"s":     time.Second,
	"ms":    time.Millisecond,
	"us":    time.Microsecond,
}

// parseSystemdTimespan reads a span as `systemctl show` prints it ("1min 30s",
// "1min 29.999000s", "500ms", "infinity"): fields of a decimal number and a unit, summed. It
// answers unlimited for infinity, and an error for anything else it cannot
// read, never a zero standing in for an answer. The loop is bounded by the
// fields of the value.
func parseSystemdTimespan(value string) (span time.Duration, unlimited bool, err error) {
	if value == "infinity" {
		return 0, true, nil
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, false, errTimespan
	}
	for _, field := range fields {
		part, ok := timespanField(field)
		if !ok {
			return 0, false, errTimespan
		}
		span += part
	}
	return span, false, nil
}

// timespanField reads one "<number><unit>" field of a systemd time span, as
// format_timespan writes it: "1min", "29.999000s", "3month". The number is
// the leading digits and dot, the unit everything after them, and the unit
// must be one systemdTimespanUnits names exactly.
func timespanField(field string) (time.Duration, bool) {
	split := strings.IndexFunc(field, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
	if split <= 0 {
		return 0, false
	}
	unit, known := systemdTimespanUnits[field[split:]]
	if !known {
		return 0, false
	}
	amount, err := strconv.ParseFloat(field[:split], 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(amount * float64(unit)), true
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
