// Design: docs/architecture/diagnostics/path-mtu.md -- the local-state dependency check
// Related: state.go -- the two readers this check exercises
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// show mtu reads two things from the kernel beside the wire: the
// fragmentation counters of /proc/net/snmp through procfs, and
// net.ipv4.tcp_mtu_probing through the sysctl component. Neither stops a
// measurement, but a run that cannot read them loses the findings that
// name a blackhole in progress or a TCP stack that stalls on one. This
// check tries both readers before an operator's first run, so a missing
// mount or a refused sysctl is a diagnostic with a code rather than a note
// buried in one payload. The ICMP socket the probes need is the probe
// layer's own check (internal/core/probe/doctor.go).

package cmd

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorCheckName is the registry name of the check.
const doctorCheckName = "mtu-local-state"

// codeMTULocalState is the one code the check emits: a local-state reader
// failed, and the message names which and what the run loses.
const codeMTULocalState = "doctor-mtu-local-state"

// The two readers the check runs, as seams so the test drives each failure
// without a kernel to break. Production reads the same functions the run does
// (liveDeps in run.go), so the check answers for the code path show mtu takes.
var (
	doctorFragmentation = func() error {
		_, err := readFragmentationCounters(procMountPoint)
		return err
	}
	doctorTCPMTUProbing = func() error {
		_, err := readTCPMTUProbing()
		return err
	}
)

// checkMTULocalState is the check function. Each reader that fails adds one
// warning, so an operator with both broken reads two named causes rather than
// the first one found.
func checkMTULocalState(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	if err := doctorFragmentation(); err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeMTULocalState,
			Severity: diagnostic.SeverityWarning,
			Message:  localStateMessage("the fragmentation counters of "+procMountPoint+"/net/snmp cannot be read", err, "the blackhole and reassembly findings"),
		})
	}
	if err := doctorTCPMTUProbing(); err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeMTULocalState,
			Severity: diagnostic.SeverityWarning,
			Message:  localStateMessage(sysctlTCPMTUProbing+" cannot be read", err, "the TCP blackhole finding"),
		})
	}
	return diags
}

// localStateMessage names the reader that failed, its error, and the finding
// show mtu reports without it, so the operator knows the measurement still runs.
func localStateMessage(what string, err error, loses string) string {
	var tb textbuf.Buffer
	tb.Str(what).Str(": ").Str(err.Error())
	tb.Str(". show mtu still measures the path and sizes the tunnels, and reports ")
	tb.Str(loses).Str(" as unreadable")
	return tb.String()
}
