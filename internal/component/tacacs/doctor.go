// Design: ai/rules/repo-maintenance.md -- TACACS+ server reachability readiness check
// Overview: register.go -- the init() that installs the registration below
// Related: config.go -- ExtractConfig, the server list and timeout this check probes
// Related: client.go -- the TCP client that connects to the servers this check probes
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes TACACS+ servers belongs to the
// backend that connects to them (ai/patterns/registration.md, "Doctor Check
// Registry"), so it is owned here now and dropping this backend drops its
// check with it.

package tacacs

import (
	"fmt"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codeTACACSUnreachable names a configuration none of whose TACACS+ servers
// accepts a TCP connection. internal/core/diagnostic/codes.go declares it, so
// `ze explain doctor-tacacs-unreachable` answers.
const codeTACACSUnreachable = "doctor-tacacs-unreachable"

// codeTACACSWeakSecret identifies a key below the RFC's recommended length.
const codeTACACSWeakSecret = "doctor-tacacs-weak-secret" //nolint:gosec // a diagnostic code name, not a credential

// tacacsTCPReachable is the probe checkTACACSServers runs. It is a variable so
// a test can stand in an unreachable server; nothing else assigns it.
var tacacsTCPReachable = diagnostic.DoctorTCPReachable

// tacacsDoctorCheck describes the check. register.go registers it.
//
// Order 2032 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it ran right after the DNS resolver check at
// 2030 (internal/component/config/system/doctor.go) and before the sysctl
// procfs check at 2040 (internal/component/sysctl/doctor.go).
var tacacsDoctorCheck = diagnostic.DoctorCheck{
	Name:         "tacacs-servers",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2032,
	Component:    "tacacs",
	Dependencies: []string{"tacacs-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeTACACSUnreachable, codeTACACSWeakSecret},
	Check:        checkTACACSServers,
}

// checkTACACSServers warns when the config names TACACS+ servers and none of
// them accepts a TCP connection. One reachable server is enough: the client
// takes the servers in failover order and needs only one to answer.
//
// The addresses and the timeout come from ExtractConfig, the same parse the
// backend builds its client from, so the probe goes where the daemon will.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkTACACSServers(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg := ExtractConfig(tree)
	if !cfg.HasServers() {
		return nil
	}

	var diagnostics []diagnostic.Diagnostic
	// RFC 8907 Section 10.5.1: "TACACS+ clients SHOULD NOT allow servers
	// to be configured without a shared secret key or shared key that is
	// less than 16 characters long." Existing short keys remain accepted,
	// but readiness exposes the risk without displaying the credential.
	for _, server := range cfg.Servers {
		if utf8.RuneCount(server.Key) < 16 {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Code:     codeTACACSWeakSecret,
				Severity: diagnostic.SeverityWarning,
				Message:  fmt.Sprintf("TACACS+ server %s: shared secret is shorter than the recommended 16 characters", server.Address),
			})
		}
	}

	timeout := diagnostic.DoctorProbeTimeout(cfg.Timeout)
	for _, server := range cfg.Servers {
		if tacacsTCPReachable(server.Address, timeout) {
			return diagnostics
		}
	}
	return append(diagnostics, diagnostic.Diagnostic{
		Code:     codeTACACSUnreachable,
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured TACACS+ servers are reachable",
	})
}
