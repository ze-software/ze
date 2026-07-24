// Design: ai/rules/feature-gate-registration.md -- ze_tacacs AAA composition root gating
//
// TACACS+ backend registration for the AAA composition root, gated on
// ze_tacacs. This is the hand-written sibling of the generated
// plugin/all/all_ze_tacacs.go (which gates the config schema): aaa/all is a
// SECOND composition root the generator does not manage, so the backend's
// blank import moves here by hand. With ze_tacacs off, aaa.Default never sees
// the backend and a `system { authentication { tacacs {} } }` config block is
// rejected as unknown (schema gated), so AAA fails closed instead of silently
// skipping a configured method.

//go:build ze_tacacs

package all

import (
	// TACACS+ (RFC 8907) backend.
	_ "codeberg.org/thomas-mangin/ze/internal/component/tacacs"
)
