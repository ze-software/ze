// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_storage.go — platformMismatch consumer for NTP persist-path

// Platform checks: runtime platform resolution and the judgement over the
// resolved platform. The systemd service unit check is owned by
// internal/plugins/systemd, the package that writes the unit; the
// config-vs-platform coherence checks over the resolv.conf path and the
// update-check block are owned by internal/component/config/system, the
// package that declares and consumes those leaves.

package doctor

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const doctorPlatformEnv = "ze.test.doctor.platform"

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorPlatformEnv,
	Type:        envTypeString,
	Description: "Override doctor platform detection for functional tests",
	Private:     true,
})

// resolveDoctorPlatform answers the platform every check reads off its context
// and the phase dispatch filters on (runDoctorChecks, registry.go). It is the
// runner's own input, resolved before the first phase runs, which is why it is
// not a registered check: a check cannot run before the value that decides
// whether it runs exists. It is the twin of resolveStorageWithDiag.
//
// A nil platform is a legitimate answer, and the diagnostic beside it says why:
// the platform-gated checks then run only where their Platforms list carries
// the wildcard, and checkPlatform has nothing to judge.
func resolveDoctorPlatform() (*host.PlatformInfo, []diagnostic.Diagnostic) {
	var tb textbuf.Buffer
	p, err := detectDoctorPlatform()
	if err != nil {
		return nil, []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPlatformDetect,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("platform detection failed: ").Err(err).String(),
		}}
	}
	if p == nil {
		return nil, []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPlatformDetect,
			Severity: diagnostic.SeverityWarning,
			Message:  "platform detection returned no platform information",
		}}
	}
	return p, nil
}

// checkPlatform judges the platform the runner resolved: an unidentified one,
// a gokrazy without a writable /perm, and a container whose root is read-only.
//
// A nil platform is a detection failure the runner already reported through
// resolveDoctorPlatform, so there is nothing here to add to it.
func checkPlatform(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	p := ctx.Platform
	if p == nil {
		return nil
	}
	var diags []diagnostic.Diagnostic
	if p.Type == host.PlatformUnknown {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorPlatformUnknown,
			Severity: diagnostic.SeverityWarning,
			Message:  "could not identify runtime platform",
		})
	}
	if p.Type == host.PlatformGokrazy && !p.PersistentStorageWritable {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorPlatformPerm,
			Severity: diagnostic.SeverityError,
			Message:  "gokrazy /perm partition is not writable; config and state persistence will fail",
		})
	}
	if p.Type == host.PlatformContainer && p.ReadOnlyRoot {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorPlatformContainerRO,
			Severity: diagnostic.SeverityWarning,
			Message:  "running in container with read-only root filesystem; ensure writable volumes are mounted for config and state",
		})
	}
	return diags
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
