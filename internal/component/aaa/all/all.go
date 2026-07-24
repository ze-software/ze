// Design: ai/patterns/registration.md -- AAA registry (VFS-like)

// Package all blank-imports every AAA backend so their init() functions fire
// and aaa.Default contains the backend factories. Binaries that need the
// composed AAA bundle (ze hub, ze-test) blank-import this package exactly
// once; individual components never import backends by name.
package all

import (
	// Local bcrypt backend.
	_ "codeberg.org/thomas-mangin/ze/internal/component/authz"
	// RADIUS (RFC 2865) backend: gated behind ze_radius, see all_ze_radius.go.
	// TACACS+ (RFC 8907) backend: gated behind ze_tacacs, see all_ze_tacacs.go.
)
