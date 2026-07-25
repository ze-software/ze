package cli

import (
	// Trigger plugin init() registrations needed by config tests.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	// Fill the infra BGP seams (resolver, peer validator) this package's
	// commands use for bgp{} handling. In the real ze binary the gated CLI
	// composition root cmd/ze/dispatch_bgp.go links bgp/config; plugin/all
	// deliberately does not, because bgp/config's own tests import plugin/all
	// and the edge would be an import cycle in test. Test files carry no
	// compile-out obligation, so naming it here is legal and keeps these tests
	// exercising the same code path the shipped binary takes.
	_ "github.com/ze-software/ze/internal/component/bgp/config"
)
