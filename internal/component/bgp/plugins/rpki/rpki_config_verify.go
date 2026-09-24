// Design: docs/guide/rpki.md -- candidate PKI isolation and partial-root reloads
package rpki

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/pki"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseRPKISections resolves candidate credentials without modifying the live
// PKI store. A runtime delivery replaces only the roots it contains; an explicit
// empty section removes that root, whereas omission retains its committed value.
// Startup and the stateless verifier pass nil for the previous configuration.
func parseRPKISections(sections []sdk.ConfigSection, previous *rpkiConfig) (*rpkiConfig, error) {
	var cfg *rpkiConfig
	var store *pki.PKIConfig
	if previous != nil {
		store = previous.pkiConfig
	}
	for _, section := range sections {
		var err error
		switch section.Root {
		case configRootBGP:
			cfg, err = parseRPKIConfig(section.Data)
		case configRootPKI:
			store, err = pki.ParseJSON(section.Data)
		}
		if err != nil {
			return nil, err
		}
	}
	if cfg == nil {
		cfg = &rpkiConfig{
			OriginNotFoundAction: ASPAPolicyAccept,
			ASPAUnknownAction:    ASPAPolicyAccept,
		}
		if previous != nil {
			*cfg = *previous
			// startSessions sorts this slice; the rollback generation must
			// remain independent of the candidate being applied.
			cfg.CacheServers = slices.Clone(previous.CacheServers)
		}
	}
	if store == nil {
		store = &pki.PKIConfig{}
	}
	if err := pki.Validate(store); err != nil {
		return nil, fmt.Errorf("rpki: candidate PKI: %w", err)
	}
	for _, server := range cfg.CacheServers {
		if server.TLS == nil {
			continue
		}
		if _, err := buildRTRTLSConfig(server.TLS, store); err != nil {
			return nil, fmt.Errorf("rpki: cache %q: %w", server.Address, err)
		}
	}
	cfg.pkiConfig = store
	return cfg, nil
}

func validateRPKISections(sections []sdk.ConfigSection) error {
	_, err := parseRPKISections(sections, nil)
	return err
}
