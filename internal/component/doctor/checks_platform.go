// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_storage.go — platformMismatch consumer for NTP persist-path

// Platform checks: runtime platform detection, systemd service unit
// validation, and config-vs-platform coherence (resolv.conf paths,
// gokrazy-managed update settings).

package doctor

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	doctorPlatformEnv     = "ze.test.doctor.platform"
	doctorServiceUnitEnv  = "ze.test.doctor.service-unit"
	defaultServiceUnit    = "/etc/systemd/system/ze.service"
	gokrazyResolvConfPath = "/tmp/resolv.conf"
	linuxResolvConfPath   = "/etc/resolv.conf"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorServiceUnitEnv,
	Type:        "string",
	Description: "Override ze.service unit path for doctor functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorPlatformEnv,
	Type:        "string",
	Description: "Override doctor platform detection for functional tests",
	Private:     true,
})

var readServiceUnitFile = os.ReadFile
var statServiceExecutable = os.Stat
var lookupServiceUser = user.Lookup
var lookupServiceGroup = user.LookupGroup

func checkPlatform() (*host.PlatformInfo, []diagnostic.Diagnostic) {
	var tb textbuf.Buffer
	p, err := detectDoctorPlatform()
	if err != nil {
		return nil, []diagnostic.Diagnostic{{
			Code:     "doctor-platform-detect",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("platform detection failed: ").Err(err).String(),
		}}
	}
	if p == nil {
		return nil, []diagnostic.Diagnostic{{
			Code:     "doctor-platform-detect",
			Severity: diagnostic.SeverityWarning,
			Message:  "platform detection returned no platform information",
		}}
	}
	var diags []diagnostic.Diagnostic
	if p.Type == host.PlatformUnknown {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-platform-unknown",
			Severity: diagnostic.SeverityWarning,
			Message:  "could not identify runtime platform",
		})
	}
	if p.Type == host.PlatformGokrazy && !p.PersistentStorageWritable {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-platform-perm",
			Severity: diagnostic.SeverityError,
			Message:  "gokrazy /perm partition is not writable; config and state persistence will fail",
		})
	}
	if p.Type == host.PlatformContainer && p.ReadOnlyRoot {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-platform-container-ro",
			Severity: diagnostic.SeverityWarning,
			Message:  "running in container with read-only root filesystem; ensure writable volumes are mounted for config and state",
		})
	}
	return p, diags
}

func detectDoctorPlatform() (*host.PlatformInfo, error) {
	forced := strings.TrimSpace(env.Get(doctorPlatformEnv))
	if forced != "" {
		return forcedPlatformInfo(forced)
	}
	return host.DetectPlatform()
}

func forcedPlatformInfo(name string) (*host.PlatformInfo, error) {
	switch strings.ToLower(name) {
	case "unknown":
		return &host.PlatformInfo{Type: host.PlatformUnknown}, nil
	case "gokrazy":
		return &host.PlatformInfo{Type: host.PlatformGokrazy, ReadOnlyRoot: true, PermAvailable: true, PersistentStorageWritable: true}, nil
	case "systemd":
		return &host.PlatformInfo{Type: host.PlatformSystemd, SystemdAvailable: true}, nil
	case "container":
		return &host.PlatformInfo{Type: host.PlatformContainer}, nil
	case "plain", "plain-linux":
		return &host.PlatformInfo{Type: host.PlatformPlainLinux}, nil
	case "darwin":
		return &host.PlatformInfo{Type: host.PlatformDarwin}, nil
	default:
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("unknown forced platform: ").Str(name).String())
	}
}

type serviceUnitInfo struct {
	execStart string
	user      string
	group     string
}

func checkSystemdServiceInstall(platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform != nil && (platform.Type == host.PlatformGokrazy || platform.Type == host.PlatformContainer) {
		return nil
	}

	unitPath := env.Get(doctorServiceUnitEnv)
	if unitPath == "" {
		unitPath = defaultServiceUnit
	}

	data, err := readServiceUnitFile(unitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-unit",
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
			unit.execStart = firstSystemdCommand(value)
		case "User":
			unit.user = strings.TrimSpace(value)
		case "Group":
			unit.group = strings.TrimSpace(value)
		}
	}
	return unit
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
			Code:     "doctor-service-executable",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("systemd service unit has no ExecStart command: ").Str(unitPath).String(),
			Path:     unitPath,
		}}
	}
	if !filepath.IsAbs(executable) {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-executable",
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
			Code:     "doctor-service-executable",
			Severity: diagnostic.SeverityError,
			Message:  tb.Reset().Str("systemd service executable not found: ").Str(executable).Str(": ").Err(err).String(),
			Path:     executable,
		}}
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return []diagnostic.Diagnostic{{
			Code:     "doctor-service-executable",
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
			Code:     "doctor-service-user",
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
			Code:     "doctor-service-group",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("systemd service group not found: ").Str(name).String(),
			Path:     unitPath,
			Expected: "existing group",
			Actual:   name,
		}}
	}
	return nil
}
func checkUpdateBackendConfig(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil || platform.Type != host.PlatformGokrazy {
		return nil
	}
	uc := getContainerPath(tree, "system", "update-check")
	if uc == nil {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-config-platform-mismatch",
		Severity: diagnostic.SeverityWarning,
		Message:  "system update-check config is ignored on gokrazy; image updates are managed by gokrazy",
		Path:     "system/update-check",
	}}
}
func checkResolvConfPath(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil {
		return nil
	}
	path := effectiveResolvConfPath(tree)
	if path == "" {
		return nil
	}
	switch platform.Type {
	case host.PlatformGokrazy:
		if strings.HasPrefix(path, "/etc/") {
			return []diagnostic.Diagnostic{platformMismatch(
				"DNS resolv-conf-path points at read-only gokrazy root filesystem",
				"system/dns/resolv-conf-path",
				gokrazyResolvConfPath,
				path,
			)}
		}
	case host.PlatformSystemd, host.PlatformPlainLinux:
		if path == gokrazyResolvConfPath {
			return []diagnostic.Diagnostic{platformMismatch(
				"DNS resolv-conf-path uses gokrazy default on "+platform.Type.String(),
				"system/dns/resolv-conf-path",
				linuxResolvConfPath,
				path,
			)}
		}
	default:
		return nil
	}
	return nil
}

func effectiveResolvConfPath(tree *config.Tree) string {
	path := gokrazyResolvConfPath
	if dns := getContainerPath(tree, "system", "dns"); dns != nil {
		if value, ok := dns.Get("resolv-conf-path"); ok {
			path = value
		}
	}
	return path
}

func platformMismatch(message, path, expected, actual string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     "doctor-config-platform-mismatch",
		Severity: diagnostic.SeverityWarning,
		Message:  message,
		Path:     path,
		Expected: expected,
		Actual:   actual,
	}
}
