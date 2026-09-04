// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Overview: client.go -- TACACS+ TCP client and wire protocol
// Related: authenticator.go -- bridges client to aaa.Authenticator
// Related: authorizer.go -- bridges client to aaa.Authorizer
// Related: accounting.go -- bridges client to aaa.Accountant

package tacacs

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/aaa"
)

// backendName is this AAA backend's identifier, and the AuthResult.Source every
// result carries. It is not the config container of the same spelling.
const backendName = "tacacs"

// tacacsBackend is the AAA backend for TACACS+ (RFC 8907).
type tacacsBackend struct{}

// Name returns the backend identifier matching AuthResult.Source.
func (tacacsBackend) Name() string { return backendName }

// Priority 100 places tacacs before local (priority 200) in the chain.
func (tacacsBackend) Priority() int { return 100 }

// Build reads the tacacs config subtree and returns the AAA contributions.
// Returns an empty Contribution when no servers are configured.
func (tacacsBackend) Build(params aaa.BuildParams) (aaa.Contribution, error) {
	cfg := ExtractConfig(params.ConfigTree)
	if !cfg.HasServers() {
		return aaa.Contribution{}, nil
	}

	// RFC 8907 Section 10.5.2: "There MUST always be a shared secret set on
	// the server for the client requesting the connection", and "TACACS+
	// clients MUST NOT set TAC_PLUS_UNENCRYPTED_FLAG". A server with no key
	// therefore has no packet Ze can conformantly send it, and MarshalInto
	// refuses one. Refusing here as well means the operator learns at load,
	// with the address named, rather than at the first login attempt.
	//
	// The YANG `mandatory true` on the leaf reports the same absence, but
	// SectionValidationError.Blocking grades a missing mandatory field a
	// warning on purpose (validate_sections.go), so the schema alone lets the
	// daemon start.
	for _, srv := range cfg.Servers {
		if len(srv.Key) == 0 {
			return aaa.Contribution{}, fmt.Errorf("tacacs server %s: %w", srv.Address, ErrNoSharedSecret)
		}
	}

	client := NewTacacsClient(TacacsClientConfig{
		Servers:       cfg.Servers,
		Timeout:       cfg.Timeout,
		SourceAddress: cfg.SourceAddress,
		Logger:        params.Logger,
	})

	privMap := cfg.PrivLvlMap
	if privMap == nil {
		privMap = map[int][]string{}
	}

	contrib := aaa.Contribution{
		Authenticator: newTacacsAuthenticator(client, privMap, params.Logger),
	}

	if cfg.Authorization {
		contrib.Authorizer = newTacacsAuthorizerWithFallback(client, params.LocalAuthorizer, params.Logger, cfg.StrictFallback)
	}

	var acct *TacacsAccountant
	if cfg.Accounting {
		acct = NewTacacsAccountant(client, params.Logger)
		acct.Start()
		contrib.Accountant = acct
	}

	// Close runs on every AAA bundle swap (config reload or clean shutdown).
	// Always stop the accountant worker (if any) AND drain the client's
	// single-connect pool -- without this, reloading with tacacs still
	// configured leaks pooled TCP connections into the next bundle's
	// replacement client.
	contrib.Close = func() error {
		if acct != nil {
			acct.Stop()
		}
		client.Close()
		return nil
	}

	return contrib, nil
}

func init() {
	if err := aaa.Default.Register(tacacsBackend{}); err != nil {
		panic("BUG: tacacs: register TACACS+ AAA backend: " + err.Error())
	}
}
