// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS AAA backend
// Related: client.go -- RADIUS client used by the authenticator
// Related: config.go -- system/authentication/radius extraction
// Related: authenticator.go -- aaa.Authenticator over the RADIUS client

package radius

import (
	"os"

	"github.com/ze-software/ze/internal/component/aaa"
)

const (
	aaaName     = "radius"
	aaaPriority = 50
)

type radiusBackend struct{}

func (radiusBackend) Name() string  { return aaaName }
func (radiusBackend) Priority() int { return aaaPriority }

// Build authenticates operator/admin logins (SSH, web, MCP) against the RADIUS
// servers under system/authentication/radius. It returns an empty Contribution
// when no servers are configured, so boxes that use TACACS+ or local auth are
// unaffected. This is entirely separate from the L2TP subscriber RADIUS path,
// which owns its own client under the l2tp config root.
func (radiusBackend) Build(params aaa.BuildParams) (aaa.Contribution, error) {
	cfg := ExtractConfig(params.ConfigTree)
	if !cfg.HasServers() {
		return aaa.Contribution{}, nil
	}

	client, err := NewClient(ClientConfig{
		Servers:       cfg.Servers,
		Timeout:       cfg.Timeout,
		Retries:       cfg.Retries,
		SourceAddress: cfg.SourceAddress,
		Logger:        params.Logger,
	})
	if err != nil {
		// Degrade, do NOT abort: a client-init failure (e.g. an unbindable
		// source-address) must not fail the whole AAA bundle build. The
		// registry turns any backend Build error into a fatal bundle failure,
		// which the hub treats by dropping the entire bundle -- SSH would then
		// not be built at all, locking the operator out (the opposite of R-4).
		// Contribute nothing instead, so local (and tacacs) still authenticate;
		// the misconfig surfaces via this log and doctor-radius-admin-unreachable.
		if params.Logger != nil {
			params.Logger.Error("radius: admin backend disabled, RADIUS client init failed", "error", err)
		}
		return aaa.Contribution{}, nil
	}

	// NAS-Identifier (RFC 2865 Section 4.1) identifies this device to the
	// server. The hostname is the natural identifier; fall back to the backend
	// name so the required attribute is never empty.
	nasID, _ := os.Hostname()
	if nasID == "" {
		nasID = aaaName
	}

	return aaa.Contribution{
		Authenticator: newRadiusAuthenticator(client, cfg, nasID, params.Logger),
		// Close drains the client's UDP socket on every AAA bundle swap
		// (config reload or shutdown) so reloads do not leak sockets.
		Close: client.Close,
	}, nil
}
