package cli_test

import (
	// Trigger plugin init() registrations needed by config tests.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	// Fill the infra BGP seams (resolver, peer validator) this package's
	// commands use for bgp{} handling. In the real ze binary the gated CLI
	// composition root cmd/ze/dispatch_bgp.go links bgp/config; plugin/all
	// does not.
	//
	// That absence is a CONDITION rather than a law. Three of bgp/config's own
	// test files (loader_test.go, peer_keywords_test.go, plugins_test.go) are
	// in the INTERNAL test package `bgpconfig` and import plugin/all, so the
	// edge would be a cycle in test while they stay that way. Moving those
	// blank imports into an external `bgpconfig_test` file, which is the form
	// THIS file uses, removes the obstacle. Test files carry no compile-out
	// obligation, so naming bgp/config here is legal and keeps these tests
	// exercising the same code path the shipped binary takes.
	_ "github.com/ze-software/ze/internal/component/bgp/config"
)
