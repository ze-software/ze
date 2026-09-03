//go:build ze_bgp

package testing

import (
	// Fill the always-on BGP infra seams for the .et corpus. TestFunctionalETFiles
	// drives the same test/editor/*.et files the ze-test harness runs, and one of
	// them (lifecycle/commit-blocked-missing-leak-filter.et) asserts that commit is
	// refused by infra.ValidateBGPPeers. That seam is filled by this package's
	// init(), which plugin/all cannot carry: bgp/config's own tests import
	// plugin/all, so blank-importing it from bgp/plugin closes an import cycle in
	// test. cmd/ze/infra_bgp.go links it for the binaries.
	_ "github.com/ze-software/ze/internal/component/bgp/config"
)
