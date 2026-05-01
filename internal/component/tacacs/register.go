// Design: .claude/patterns/registration.md -- AAA registry (VFS-like)
// Overview: client.go -- TACACS+ TCP client and wire protocol
// Related: authenticator.go -- bridges client to aaa.Authenticator
// Related: authorizer.go -- bridges client to aaa.Authorizer
// Related: accounting.go -- bridges client to aaa.Accountant

package tacacs

import (
	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// tacacsBackend is the AAA backend for TACACS+ (RFC 8907).
type tacacsBackend struct{}

// Name returns the backend identifier matching AuthResult.Source.
func (tacacsBackend) Name() string { return "tacacs" }

// Priority 100 places tacacs before local (priority 200) in the chain.
func (tacacsBackend) Priority() int { return 100 }

// Build reads the tacacs config subtree and returns the AAA contributions.
// Returns an empty Contribution when no servers are configured.
func (tacacsBackend) Build(params aaa.BuildParams) (aaa.Contribution, error) {
	cfg := ExtractConfig(params.ConfigTree)
	if !cfg.HasServers() {
		return aaa.Contribution{}, nil
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
		Authenticator: NewTacacsAuthenticator(client, privMap, params.Logger),
	}

	if cfg.Authorization {
		contrib.Authorizer = NewTacacsAuthorizerWithFallback(client, params.LocalAuthorizer, params.Logger, cfg.StrictFallback)
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
