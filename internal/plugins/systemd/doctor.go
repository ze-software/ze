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

// The five probes the check makes against the host, each a variable so a test
// can stand in a host that lacks the unit, the binary, the user or the group,
// or whose systemd applies other stop settings.
var (
	readServiceUnitFile   = os.ReadFile
	statServiceExecutable = os.Stat
	lookupServiceUser     = user.Lookup
	lookupServiceGroup    = user.LookupGroup
	showServiceStop       = systemctlShowStop
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

// serviceStopQueryTimeout bounds the one `systemctl show` the stop check
// makes, so a wedged systemd cannot hold a doctor run.
const serviceStopQueryTimeout = 5 * time.Second

// serviceStopFix is the remedy every stop-settings warning ends with.
const serviceStopFix = "; run `sudo ze install systemd --force` and `sudo systemctl daemon-reload`, " +
	"and remove any drop-in `systemctl cat ze.service` shows overriding it"

// systemctlShowStop asks systemd for the stop settings it applies to ze, as
// it applies them: the unit, its drop-ins and the manager defaults together.
func systemctlShowStop() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceStopQueryTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "systemctl", "show", "-p", "TimeoutStopUSec", "-p", "KillMode", serviceName).Output()
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
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorServiceStop,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("the stop settings systemd applies to ").Str(serviceName).
				Str(" cannot be read, so doctor cannot tell whether systemd cuts ze's stop drain short: ").Err(err).
				Str("; run `systemctl show -p TimeoutStopUSec -p KillMode ").Str(serviceName).Str("` to see why").String(),
			Path: unitPath,
		}}
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

// systemdTimespanUnits are the unit suffixes systemd's format_timespan writes,
// longest first within a shared prefix so "min" and "ms" are tried before "m"
// and "s".
var systemdTimespanUnits = []struct {
	suffix string
	unit   time.Duration
}{
	{"min", time.Minute},
	{"ms", time.Millisecond},
	{"us", time.Microsecond},
	{"y", 31557600 * time.Second},
	{"M", 2629800 * time.Second},
	{"w", 7 * 24 * time.Hour},
	{"d", 24 * time.Hour},
	{"h", time.Hour},
	{"s", time.Second},
}

// parseSystemdTimespan reads a span as `systemctl show` prints it ("1min 30s",
// "90ms", "infinity"): fields of a decimal number and a unit, summed. It
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

// timespanField reads one "<number><unit>" field of a systemd time span.
func timespanField(field string) (time.Duration, bool) {
	for _, candidate := range systemdTimespanUnits {
		number, found := strings.CutSuffix(field, candidate.suffix)
		if !found {
			continue
		}
		amount, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return 0, false
		}
		if amount < 0 {
			return 0, false
		}
		return time.Duration(amount * float64(candidate.unit)), true
	}
	return 0, false
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
