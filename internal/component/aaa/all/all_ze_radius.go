// Design: ai/rules/plugins.md -- ze_radius AAA composition root gating
//
// RADIUS backend registration for the AAA composition root, gated on
// ze_radius. This is the hand-written sibling of the generated
// plugin/all/all_ze_radius.go (which gates the config schema): aaa/all is a
// SECOND composition root the generator does not manage, so the backend's
// blank import moves here by hand. Plain ze_radius (no ze_l2tp) on purpose:
// RADIUS system authentication works without the BNG; only the l2tp
// authradius plugin is the ze_l2tp && ze_radius dependent piece.

//go:build ze_radius

package all

import (
	// RADIUS (RFC 2865) backend.
	_ "github.com/ze-software/ze/internal/component/radius"
)
