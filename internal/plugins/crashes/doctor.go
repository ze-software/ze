// Design: docs/architecture/diagnostics/crash-capture.md -- crash capture doctor checks
// Related: readiness.go -- the one readiness answer all three checks read
// Related: register.go -- registers these checks via diagnostic.RegisterDoctorCheck
//
// Crash capture depends on three runtime facts the operator cannot see from the
// config: a reserved region on the running kernel's command line, a readable
// pstore filesystem, and a writable crash directory. Each one fails silently,
// and the appliance has no shell to diagnose any of them with, so each one gets
// a check and a registered diagnostic code.

package crashes

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// crashDoctorReadiness is the seam the checks read the machine through. A test
// replaces it; the daemon never does.
var crashDoctorReadiness = func(ctx diagnostic.DoctorCheckContext) crashlog.Readiness {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return crashlog.CrashReadiness(crashlog.Intent{})
	}
	return crashlog.CrashReadiness(system.ExtractSystemConfig(tree).CrashDump.Intent())
}

// checkCrashCaptureArmed warns when the operator asked for kernel crash capture
// and the running kernel booted without the reservation. It is silent on a box
// that did not ask for capture, so a machine that never enabled it gets no
// warning about a feature it does not use.
func checkCrashCaptureArmed(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	readiness := crashDoctorReadiness(ctx)
	if !readiness.Configured || readiness.Region != "" {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-crash-capture-unarmed",
		Severity: diagnostic.SeverityWarning,
		Message:  readiness.Reason,
	}}
}

// checkCrashCapturePstore warns when the reservation is in place but the record
// cannot be read back. That is the worst of the three states: the RAM is spent
// on every boot and nothing can ever be recovered from it.
func checkCrashCapturePstore(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	readiness := crashDoctorReadiness(ctx)
	if !readiness.Configured || readiness.Region == "" || readiness.PstoreAvailable {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-crash-capture-pstore",
		Severity: diagnostic.SeverityWarning,
		Message:  readiness.Reason,
	}}
}

// checkCrashDirectory warns when no crash directory is writable. This one is not
// gated on the configured intent: a Go panic report needs the same directory,
// and that capture is always on.
func checkCrashDirectory(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	readiness := crashDoctorReadiness(ctx)
	if readiness.DirectoryWritable {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     "doctor-crash-directory-unwritable",
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("no crash directory could be created and written, so a crash report has nowhere to go; ").
			Str("set ze.crash.dir to a writable path").String(),
	}}
}
