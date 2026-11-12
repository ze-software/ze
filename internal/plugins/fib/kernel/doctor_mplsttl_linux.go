//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- the pushed label carries the IP TTL
// Related: register_linux.go -- the init() that installs the registration below
// Related: nexthop_linux.go -- buildMPLSEncap, the push whose TTL the kernel chooses
// Related: internal/core/sysctl/known_linux.go -- refuseMPLSTTLNoPropagate, the refusal of Ze's own writes
//
// The sysctl registry refuses net.mpls.ip_ttl_propagate 0 on every write Ze
// makes, but another program (`sysctl -w`, a boot script) can still write it.
// The key decides the TTL of every label this plugin pushes, so the question
// is this plugin's: the check lives here, beside the push it judges, and runs
// only under the config that makes this plugin push labels. The sysctl
// component owns the key's writes and knows nothing of label pushes.

package fibkernel

import (
	"errors"
	"io/fs"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/component/sysctl"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// mplsTTLPropagateKey is the kernel key that chooses the TTL of a pushed label.
const mplsTTLPropagateKey = "net.mpls.ip_ttl_propagate"

// mplsTTLPropagateRead is the read checkMPLSTTLPropagate runs. It is a
// variable so a test stands in the kernel's answer; nothing else assigns it.
var mplsTTLPropagateRead = sysctl.Read

// mplsTTLPropagateDoctorCheck is the registration register_linux.go installs.
// Order 737 places it between the MPLS table capability (735) and the transit
// path MTU capability (739), the other kernel MPLS questions this plugin owns.
var mplsTTLPropagateDoctorCheck = diagnostic.DoctorCheck{
	Name:         "mpls-ttl-propagate",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        737,
	Component:    pluginName,
	Dependencies: []string{"kernel", "sysctl"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		diagnostic.CodeDoctorMPLSTTLPropagateOff,
		diagnostic.CodeDoctorMPLSTTLPropagateUnknown,
	},
	Check: checkMPLSTTLPropagate,
}

// checkMPLSTTLPropagate reports a kernel that would not copy the IP TTL into
// the labels this plugin pushes. It asks nothing unless the config makes the
// kernel FIB push labels (kernelcap.MPLSInUse, the predicate the MPLS
// capability rows use), so a box with no MPLS, or one whose labels go to
// another data plane, is never asked about a key it does not use.
//
// RFC 3032 Section 2.4.3: "When an IP packet is first labeled, the TTL field of the label stack entry MUST BE set to the value of the IP TTL field."
func checkMPLSTTLPropagate(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	// The runner hands a parsed *config.Tree or nil. MPLSInUse answers false
	// for nil, which is right: with no config, no label is pushed.
	tree, _ := ctx.Tree.(*config.Tree)
	if !kernelcap.MPLSInUse(tree) {
		return nil
	}
	value, err := mplsTTLPropagateRead(mplsTTLPropagateKey)
	if err != nil {
		return []diagnostic.Diagnostic{mplsTTLPropagateUnreadable(err)}
	}
	if value == "1" {
		return nil
	}
	if value == "0" {
		return []diagnostic.Diagnostic{mplsTTLPropagateOff()}
	}
	// The kernel accepts only 0 and 1 for this key, so another answer is one
	// the check cannot interpret, never a pass.
	return []diagnostic.Diagnostic{mplsTTLPropagateUnexpected(value)}
}

// mplsTTLPropagateOff is the finding for a kernel holding 0.
func mplsTTLPropagateOff() diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorMPLSTTLPropagateOff,
		Severity: diagnostic.SeverityError,
		Message: "the kernel holds net.mpls.ip_ttl_propagate 0, written outside ze, so every label ze pushes " +
			"carries net.mpls.default_ttl instead of the packet's IP TTL; " +
			"RFC 3032 Section 2.4.3 requires the IP TTL",
		Fix: "sysctl -w net.mpls.ip_ttl_propagate=1",
	}
}

// mplsTTLPropagateUnreadable is the finding for a key the check could not
// read. An absent key means the kernel holds no MPLS table (mpls_router not
// loaded); any other error is a read the host refused.
func mplsTTLPropagateUnreadable(err error) diagnostic.Diagnostic {
	fix := "sudo ze doctor"
	if errors.Is(err, fs.ErrNotExist) {
		fix = "modprobe mpls_router"
	}
	var tb textbuf.Buffer
	tb.Str("cannot tell whether the labels ze pushes carry the IP TTL that RFC 3032 Section 2.4.3 requires: ").
		Str(mplsTTLPropagateKey).Str(" could not be read: ").Err(err)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorMPLSTTLPropagateUnknown,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Fix:      fix,
	}
}

// mplsTTLPropagateUnexpected is the finding for a value outside 0 and 1.
func mplsTTLPropagateUnexpected(value string) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("cannot tell whether the labels ze pushes carry the IP TTL that RFC 3032 Section 2.4.3 requires: ").
		Str(mplsTTLPropagateKey).Str(" holds ").Quoted(value).Str(", not 0 or 1")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorMPLSTTLPropagateUnknown,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Fix:      "sysctl -w net.mpls.ip_ttl_propagate=1",
	}
}
