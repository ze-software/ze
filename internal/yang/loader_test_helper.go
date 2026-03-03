// Design: docs/architecture/config/yang-config-design.md — YANG schema handling

package yang

import (
	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/schema"
)

// LoadAllForTesting loads all YANG modules including module-specific ones.
// This is intended for tests that need access to the full schema.
// In production code, use LoadEmbedded() + AddModuleFromText() with the
// module-specific YANG from its package.
func (l *Loader) LoadAllForTesting() error {
	// Load core modules (extensions, types, hub, plugin)
	if err := l.LoadEmbedded(); err != nil {
		return err
	}

	// Load module-specific YANG from their packages
	if err := l.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG); err != nil {
		return err
	}

	return nil
}
