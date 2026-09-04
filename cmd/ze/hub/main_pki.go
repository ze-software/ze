// Design: docs/architecture/pki/tls-listeners.md -- hub PKI configuration parsing

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
)

// preparePKIConfig parses and validates the pki block of a loaded config tree.
//
// It takes the TREE rather than the plugin-facing map every other reload
// consumer takes, because the map cannot carry a leaf-list faithfully. ToMap
// lowers a leaf-list by its member count, so one member becomes a bare string
// that no reader can tell from a plain leaf. A tree rebuilt from that map put
// every `pki certificate <name> intermediate` in the single-value map instead
// of the leaf-list map, and dropped the multi-member case outright, so the
// intermediate pool was always empty and a leaf issued by an intermediate CA
// failed validation with "certificate signed by unknown authority".
//
// The callers all hold the tree already, so the round trip bought nothing and
// cost the chain (plan/journal/validated-value-discarded-by-its-caller.md).
func preparePKIConfig(tree *zeconfig.Tree) (*zepki.PKIConfig, error) {
	cfg, err := zepki.ParseConfig(tree)
	if err != nil {
		return nil, err
	}
	if err := zepki.Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
