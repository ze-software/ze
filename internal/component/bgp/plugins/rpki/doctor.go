// Design: docs/features/ai-first.md -- the readiness check this plugin owns
// Overview: register.go -- the init() that installs the registration below
// Related: rtr_session.go -- the RTR client that connects to the caches this check probes
// Related: rpki_config.go -- the cache-server list and the RTR default port, read from JSON at run time
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes RPKI caches belongs to the plugin
// that connects to them (ai/patterns/registration.md, "Doctor Check Registry"),
// so it is owned here now and dropping this plugin drops its check with it.

package rpki

import (
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codeRPKIUnreachable names a configuration none of whose RPKI caches accepts
// a TCP connection. internal/core/diagnostic/codes.go declares it, so
// `ze explain doctor-rpki-unreachable` answers.
const codeRPKIUnreachable = "doctor-rpki-unreachable"

// rtrDefaultPort is the unprotected RTR TCP port (RFC 8210 Section 9).
// A TLS cache with no explicit port uses 324 instead.
const rtrDefaultPort = "323"

// rpkiCacheProbeTimeout bounds one cache probe. DoctorProbeTimeout can only
// shorten it.
const rpkiCacheProbeTimeout = 3 * time.Second

// rpkiTCPReachable is the probe checkRPKICacheServers runs. It is a variable so
// a test can stand in an unreachable cache; nothing else assigns it.
var rpkiTCPReachable = diagnostic.DoctorTCPReachable

// rpkiDoctorCheck describes the check. register.go registers it.
//
// Order 2200 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: after the AS112 coordination checks (2150-2180) and
// before the writable-destination check (2270), both in
// internal/component/doctor/doctor_checks.go. The BMP collector check follows
// it at 2210 (bgp/plugins/bmp/doctor.go).
var rpkiDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-rpki-cache-servers",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2200,
	Component:    "bgp-rpki",
	Dependencies: []string{"rpki-cache"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeRPKIUnreachable},
	Check:        checkRPKICacheServers,
}

// checkRPKICacheServers warns when the config names RPKI caches and none of
// them accepts a TCP connection. One reachable cache is enough: the RTR client
// takes the caches by preference and needs only one to answer.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkRPKICacheServers(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	bgp := tree.GetContainer(configRootBGP)
	if bgp == nil {
		return nil
	}
	rpki := bgp.GetContainer("rpki")
	if rpki == nil {
		return nil
	}

	timeout := diagnostic.DoctorProbeTimeout(rpkiCacheProbeTimeout)
	checked := false
	for _, server := range rpki.GetListOrdered("cache-server") {
		// The list is keyed by the cache address.
		if server.Key == "" {
			continue
		}
		checked = true
		port, hasPort := server.Value.Get("port")
		if !hasPort || port == "" {
			port = rtrDefaultPort
			if server.Value.GetContainer("tls") != nil {
				port = "324"
			}
		}
		if rpkiTCPReachable(net.JoinHostPort(server.Key, port), timeout) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeRPKIUnreachable,
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured RPKI cache servers are reachable",
	}}
}
