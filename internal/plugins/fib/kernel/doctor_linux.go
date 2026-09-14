//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the readiness check this plugin owns
// Related: register_linux.go -- the init() that installs the registration below
// Related: kernelcap_linux.go -- the kernel capability this plugin enrolls beside it
// Related: nexthop_linux.go -- the multipath routes the kernel programs without nexthop objects
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. This plugin is the one that programs ECMP routes into
// the kernel, so the kernel's nexthop-object support is its question
// (ai/patterns/registration.md, "Doctor Check Registry"): it is owned here now
// and dropping this plugin drops its check with it. A VPP backend does its own
// forwarding and never reaches this registration.
//
// It is a plain doctor check rather than an enrolled capability, because an
// absent /proc/net/nexthop refuses nothing: the kernel still installs the
// legacy multipath routes this plugin writes, so the answer is a warning about
// how ECMP is expressed, never a refusal to start.

package fibkernel

import (
	"os"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeKernelNexthop names a kernel without nexthop objects.
// internal/core/diagnostic/codes.go declares it, so `ze explain
// doctor-kernel-nexthop` answers.
const codeKernelNexthop = "doctor-kernel-nexthop"

// nexthopStatPath is the probe checkKernelNexthop runs. It is a variable so a
// test stands in a kernel without the file; nothing else assigns it.
var nexthopStatPath = os.Stat

// kernelNexthopDoctorCheck is the registration register_linux.go installs.
//
// Order 170 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: it ran before the runner's registry dispatch, after
// the nftables check (160, internal/plugins/firewall/nft) and before the TLS
// checks that followed it.
var kernelNexthopDoctorCheck = diagnostic.DoctorCheck{
	Name:         "kernel-nexthop",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        170,
	Component:    pluginName,
	Dependencies: []string{"kernel"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeKernelNexthop},
	Check:        checkKernelNexthop,
}

// checkKernelNexthop warns when the kernel exposes no nexthop objects. It reads
// no config: the plugin programs routes on every configuration it runs under,
// so the kernel question is asked whenever the plugin is present.
func checkKernelNexthop(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	path := kernelcap.ProcPath("net", "nexthop")
	if _, err := nexthopStatPath(path); err == nil {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     codeKernelNexthop,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str("kernel nexthop objects unavailable (").Str(path).Str(" not found); ECMP uses legacy multipath").String(),
	}}
}
