// Design: ai/rules/repo-maintenance.md -- RADIUS admin reachability readiness check
// Overview: config.go -- system/authentication/radius config this check reads
// Related: aaa.go -- the admin backend whose dependency this check guards
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
// RFC: rfc/short/rfc2865.md -- Access-Request probe

// The radius component owns the system/authentication/radius dependency, so it
// owns this doctor check (ai/rules/repo-maintenance.md "Components that are not
// plugins"). It is distinct from the L2TP subscriber path's own
// doctor-radius-unreachable check.
package radius

import (
	"context"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// radiusAdminDoctorCheck is the readiness check registered from register.go.
var radiusAdminDoctorCheck = diagnostic.DoctorCheck{
	Name:         "radius-admin-unreachable",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        720,
	Component:    "radius",
	Dependencies: []string{"radius-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{"doctor-radius-admin-unreachable"},
	Check:        checkRadiusAdminServers,
}

// radiusAdminProbe is a test seam over radiusAdminReachable.
var radiusAdminProbe = radiusAdminReachable

// checkRadiusAdminServers warns when system/authentication/radius has servers
// but none answers a probe Access-Request, so an operator sees the lockout
// risk (R-4) via `ze doctor` before the daemon starts rather than only when a
// login falls through to local at runtime. It is a no-op when RADIUS admin
// auth is not configured.
func checkRadiusAdminServers(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg := ExtractConfig(tree)
	if !cfg.HasServers() {
		return nil
	}
	for _, srv := range cfg.Servers {
		if radiusAdminProbe(srv, cfg.SourceAddress, cfg.Timeout) {
			return nil
		}
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-radius-admin-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured RADIUS admin servers are reachable",
	}}
}

// radiusAdminReachable sends a probe Access-Request and reports whether the
// server returned a verifiable response. A missing shared key or an unbound
// port counts as unreachable: a verifiable response requires the correct
// shared secret, so a wrong key also reads as unreachable.
func radiusAdminReachable(srv Server, sourceIP net.IP, timeout time.Duration) bool {
	if len(srv.SharedKey) == 0 {
		return false
	}
	client, err := NewClient(ClientConfig{Timeout: timeout, Retries: 1, SourceAddress: sourceIP})
	if err != nil {
		return false
	}
	defer func() { _ = client.Close() }()

	auth, err := RandomAuthenticator()
	if err != nil {
		return false
	}
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("ze-doctor")},
			{Type: AttrUserPassword, Value: []byte("ze-doctor")},
			{Type: AttrNASIdentifier, Value: AttrString("ze-doctor")},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = client.Exchange(ctx, pkt, srv.SharedKey, srv.Address)
	return err == nil
}
