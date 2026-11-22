// Design: docs/features/ai-first.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: doctor_linux.go, doctor_other.go -- the writability probe, per platform
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes /proc/sys belongs to the package
// that writes /proc/sys (ai/patterns/registration.md, "Doctor Check Registry"),
// so it is owned here now and dropping this component drops its check with it.

package sysctl

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorSysctlProcfsCode names a configured sysctl key this process cannot
// write, or whose writability it cannot determine.
const doctorSysctlProcfsCode = "doctor-sysctl-procfs"

// procSysVerdict is what the kernel answered about writing one configured
// sysctl path. Zero is unspecified, never writable.
type procSysVerdict uint8

const (
	procSysUnspecified procSysVerdict = iota
	// procSysWritable means the kernel permits this process (with its effective
	// UID and effective capabilities) to write the key.
	procSysWritable
	// procSysRefused means the kernel refused write permission (EACCES).
	procSysRefused
	// procSysReadOnlyFS means /proc/sys is mounted read-only (EROFS).
	procSysReadOnlyFS
	// procSysUnknown means the probe could not establish whether the key is
	// writable (ENOENT, ENOSYS, EPERM on the probe syscall, or another error).
	procSysUnknown
)

// String names the verdict for diagnostics and test output.
func (v procSysVerdict) String() string {
	switch v {
	case procSysUnspecified:
		return "unspecified"
	case procSysWritable:
		return "writable"
	case procSysRefused:
		return "refused"
	case procSysReadOnlyFS:
		return "read-only-fs"
	case procSysUnknown:
		return "unknown"
	}
	return "invalid"
}

// sysctlProcfsDoctorCheck is the registration register.go installs.
//
// Order 2040 reproduces the sequence the doctor runner printed before the check
// moved onto the registry. It ran after the disk-space check and before the
// clock-skew check, and those two are registered at 2020 and 2100
// (internal/component/doctor/doctor_checks.go). The 700-1000 band that file
// names is for a check the runner ran at the phase dispatch itself; this one
// ran later, so it sorts later.
func sysctlProcfsDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "sysctl-procfs",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        2040,
		Component:    "sysctl",
		Dependencies: []string{"procfs"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{doctorSysctlProcfsCode},
		Check:        checkSysctlProcfs,
	}
}

// checkSysctlProcfs warns when the config carries sysctl settings that this
// process cannot write, or whose writability it cannot determine.
//
// Probing /proc/sys with access(W_OK) gave false warnings for a non-root
// daemon holding CAP_NET_ADMIN: /proc/sys itself is mode 0555 outside
// net->sysctls, and access(2) checks real UID with no capabilities, while
// Linux's net_ctl_permissions (net/sysctl_net.c) lets CAP_NET_ADMIN write
// net.* keys. Instead, each configured key path is probed with effective
// credentials (doctor_linux.go, faccessat2 with AT_EACCESS).
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkSysctlProcfs(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	block := tree.GetContainer("sysctl")
	if block == nil {
		return nil
	}
	settings := block.GetListOrdered("setting")
	if len(settings) == 0 {
		return nil
	}
	root := kernelcap.ProcPath("sys")
	var diags []diagnostic.Diagnostic
	for _, entry := range settings {
		key := entry.Key
		if strings.Contains(key, "..") {
			continue
		}
		full := filepath.Join(root, keyToPath(key))
		verdict, err := probeProcSysPath(full)
		if diag, emit := sysctlKeyDiagnostic(root, key, full, verdict, err); emit {
			diags = append(diags, diag)
		}
	}
	return diags
}

// sysctlKeyDiagnostic builds the warning for one probed sysctl key, or reports
// false when the key is writable.
func sysctlKeyDiagnostic(root, key, full string, verdict procSysVerdict, err error) (diagnostic.Diagnostic, bool) {
	var tb textbuf.Buffer
	switch verdict {
	case procSysWritable:
		return diagnostic.Diagnostic{}, false
	case procSysReadOnlyFS:
		tb.Str("sysctl: cannot write ").Str(key).Str(" (").Str(full).Str("): ").Str(root).Str(" is mounted read-only: ").Err(err)
		return diagnostic.Diagnostic{
			Code:     doctorSysctlProcfsCode,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Path:     full,
			Fix:      "sudo mount -o remount,rw " + root,
		}, true
	case procSysRefused:
		return sysctlRefusedDiagnostic(key, full, err), true
	case procSysUnknown, procSysUnspecified:
		tb.Str("sysctl: cannot determine whether ").Str(key).Str(" (").Str(full).Str(") is writable")
		if err != nil {
			tb.Str(": ").Err(err)
		}
		return diagnostic.Diagnostic{
			Code:     doctorSysctlProcfsCode,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Path:     full,
			Fix:      "ze show log recent level warn component sysctl",
		}, true
	}
	tb.Str("sysctl: cannot determine whether ").Str(key).Str(" (").Str(full).Str(") is writable: unexpected probe verdict ").Str(verdict.String())
	return diagnostic.Diagnostic{
		Code:     doctorSysctlProcfsCode,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Path:     full,
		Fix:      "ze show log recent level warn component sysctl",
	}, true
}

// sysctlRefusedDiagnostic words a permission refusal on one sysctl key.
// Linux's net_ctl_permissions (net/sysctl_net.c) grants write access to net.*
// keys when the process holds CAP_NET_ADMIN; non-network sysctls go through
// test_perm (fs/proc/proc_sysctl.c) and require effective UID 0.
func sysctlRefusedDiagnostic(key, full string, err error) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("sysctl: ").Str(full).Str(" is not writable for ").Str(key).Str(": ").Err(err)
	var fix string
	if strings.HasPrefix(key, "net.") {
		tb.Str("; writing net.* sysctls requires CAP_NET_ADMIN (or root)")
		fix = privilege.CapabilityRemedy("CAP_NET_ADMIN")
	} else {
		tb.Str("; writing non-network sysctls requires root (CAP_NET_ADMIN covers only net.* keys)")
		fix = "delete sysctl setting " + key
	}
	tb.Str(". The probe measured process ").Int(int64(os.Getpid())).Str(" (uid ").Int(int64(os.Getuid())).
		Str("), which is the ze daemon only when the daemon answered this doctor request")
	return diagnostic.Diagnostic{
		Code:     doctorSysctlProcfsCode,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Path:     full,
		Fix:      fix,
	}
}
