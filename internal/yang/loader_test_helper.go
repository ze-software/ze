package yang

import (
	hubschema "codeberg.org/thomas-mangin/ze/internal/hub/schema"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"
)

// LoadAllForTesting loads all YANG modules including module-specific ones.
// This is intended for tests that need access to the full schema.
// In production code, use LoadEmbedded() + AddModuleFromText() with the
// module-specific YANG from its package.
func (l *Loader) LoadAllForTesting() error {
	// Load core modules first
	if err := l.LoadEmbedded(); err != nil {
		return err
	}

	// Load module-specific YANG from their packages
	if err := l.AddModuleFromText("ze-hub-conf.yang", hubschema.ZeHubConfYANG); err != nil {
		return err
	}
	if err := l.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG); err != nil {
		return err
	}

	return nil
}
