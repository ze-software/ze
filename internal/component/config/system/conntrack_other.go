// Design: docs/architecture/config/syntax.md -- conntrack module loading (non-Linux stub)

//go:build !linux

package system

// LoadConntrackModules is a no-op on non-Linux platforms.
func LoadConntrackModules(_ []string) (loaded []string, errs []error) {
	return nil, nil
}

// LoadedConntrackModules returns nil on non-Linux platforms.
func LoadedConntrackModules() []string {
	return nil
}
