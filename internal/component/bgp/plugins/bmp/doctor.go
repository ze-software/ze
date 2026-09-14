// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Overview: register.go -- the init() that installs the registration below
// Related: sender.go -- the per-collector session that connects to what this check probes
// Related: sender_config.go -- the collector list, read from JSON at run time
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes BMP collectors belongs to the
// plugin that connects to them (ai/patterns/registration.md, "Doctor Check
// Registry"), so it is owned here now and dropping this plugin drops its check
// with it.

package bmp

import (
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codeBMPUnreachable names a configuration none of whose BMP collectors
// accepts a TCP connection. internal/core/diagnostic/codes.go declares it, so
// `ze explain doctor-bmp-unreachable` answers.
const codeBMPUnreachable = "doctor-bmp-unreachable"

// collectorDefaultPort is the port a collector entry that names none is sent
// to. It MUST match the default of the collector port leaf in ze-bmp-conf.yang.
const collectorDefaultPort = "11019"

// collectorProbeTimeout bounds one collector probe. DoctorProbeTimeout can only
// shorten it.
const collectorProbeTimeout = 3 * time.Second

// bmpTCPReachable is the probe checkBMPCollectors runs. It is a variable so a
// test can stand in an unreachable collector; nothing else assigns it.
var bmpTCPReachable = diagnostic.DoctorTCPReachable

// bmpDoctorCheck describes the check. register.go registers it.
//
// Order 2210 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: right after the RPKI cache check (2200,
// bgp/plugins/rpki/doctor.go) and before the writable-destination check (2270,
// internal/component/doctor/doctor_checks.go).
var bmpDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-bmp-collectors",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2210,
	Component:    "bgp-bmp",
	Dependencies: []string{"bmp-collector"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeBMPUnreachable},
	Check:        checkBMPCollectors,
}

// checkBMPCollectors warns when the config names BMP sender collectors and
// none of them accepts a TCP connection. One reachable collector is enough:
// each collector gets its own session, and the report is about the operator
// having somewhere to send at all.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkBMPCollectors(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	bgp := tree.GetContainer(configRootBGP)
	if bgp == nil {
		return nil
	}
	bmp := bgp.GetContainer("bmp")
	if bmp == nil {
		return nil
	}
	sender := bmp.GetContainer("sender")
	if sender == nil {
		return nil
	}

	timeout := diagnostic.DoctorProbeTimeout(collectorProbeTimeout)
	checked := false
	for _, collector := range sender.GetListOrdered("collector") {
		addr, hasAddr := collector.Value.Get("address")
		if !hasAddr || addr == "" {
			continue
		}
		checked = true
		port, hasPort := collector.Value.Get("port")
		if !hasPort || port == "" {
			port = collectorDefaultPort
		}
		if bmpTCPReachable(net.JoinHostPort(addr, port), timeout) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeBMPUnreachable,
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured BMP collectors are reachable",
	}}
}
