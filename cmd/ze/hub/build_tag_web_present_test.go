// Design: ai/rules/plugins.md -- ze_web present build validation
//
//go:build ze_web

package hub

// VALIDATES: with the ze_web build tag (the default ze / ze-appliance feature
// set), the web service factory is registered and the web-only daemon hook is
// installed.
// PREVENTS: a regression where ze_web is set but web is not wired through the
// construction registry or standalone start seam.

import "testing"

func TestBuildTag_Web_Present(t *testing.T) {
	if !registeredServiceName("web") {
		t.Fatal("ze_web build: web service factory not registered")
	}
	if webBuildStandalone == nil {
		t.Fatal("ze_web build: web-only daemon hook not installed")
	}
}
