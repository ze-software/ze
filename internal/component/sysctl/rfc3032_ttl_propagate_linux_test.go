// Design: docs/architecture/core-design.md -- sysctl known-keys registry
// Related: register.go -- verifySysctlConfig
//
// VALIDATES: config verify refuses net.mpls.ip_ttl_propagate 0, both as a
// sysctl setting and inside a sysctl profile, with the operator error that
// names RFC 3032 Section 2.4.3, and accepts 1. A transient `set sysctl` of 0
// and a plugin default of 0 are refused by the same check. Method:
// verifySysctlConfig over the JSON section the config pipeline hands it, and
// the store's transient and default writes over a fake backend.
// PREVENTS: an operator turning off TTL propagation, after which Linux writes
// net.mpls.default_ttl into every label fib-kernel pushes instead of the IP
// TTL.
//
// This file sorts before sysctl_test.go, whose tests reset the known-keys
// registry, so it runs against the registrations the core package's init()
// made.

//go:build linux

package sysctl

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const ttlPropagateKey = "net.mpls.ip_ttl_propagate"

func ttlPropagateSection(value string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{
		Root: configRoot,
		Data: `{"sysctl":{"setting":{"` + ttlPropagateKey + `":{"value":"` + value + `"}}}}`,
	}}
}

func ttlPropagateProfileSection(value string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{
		Root: configRoot,
		Data: `{"sysctl":{"profile":{"mpls-edge":{"setting":{"` + ttlPropagateKey + `":{"value":"` + value + `"}}}}}}`,
	}}
}

// RFC requirement: RFC3032-2.4.3-1 negative -- config verify refuses net.mpls.ip_ttl_propagate 0, as a sysctl setting and in a sysctl profile, with an error naming RFC 3032 Section 2.4.3, and accepts 1.
func TestVerifyRefusesMPLSTTLNoPropagate(t *testing.T) {
	if err := verifySysctlConfig(ttlPropagateSection("1")); err != nil {
		t.Errorf("%s 1 refused at verify: %v", ttlPropagateKey, err)
	}
	if err := verifySysctlConfig(ttlPropagateProfileSection("1")); err != nil {
		t.Errorf("%s 1 in a profile refused at verify: %v", ttlPropagateKey, err)
	}

	for name, sections := range map[string][]sdk.ConfigSection{
		"setting": ttlPropagateSection("0"),
		"profile": ttlPropagateProfileSection("0"),
	} {
		err := verifySysctlConfig(sections)
		if err == nil {
			t.Errorf("%s: %s 0 accepted at verify; the kernel would impose net.mpls.default_ttl on every pushed label", name, ttlPropagateKey)
			continue
		}
		if !strings.Contains(err.Error(), "RFC 3032 Section 2.4.3") {
			t.Errorf("%s: refusal %q does not name RFC 3032 Section 2.4.3", name, err.Error())
		}
		if !strings.Contains(err.Error(), ttlPropagateKey) {
			t.Errorf("%s: refusal %q does not name %s", name, err.Error(), ttlPropagateKey)
		}
	}
}

// VALIDATES: the runtime writes that bypass config verify, a transient
// `set sysctl` and a plugin default, refuse net.mpls.ip_ttl_propagate 0 and
// write nothing to the kernel.
func TestStoreRefusesMPLSTTLNoPropagate(t *testing.T) {
	s, be := newTestStore()

	if _, err := s.setTransient(ttlPropagateKey, "0"); err == nil {
		t.Errorf("transient %s 0 accepted", ttlPropagateKey)
	}
	if _, err := s.setDefault(ttlPropagateKey, "0", "test-plugin"); err == nil {
		t.Errorf("default %s 0 accepted", ttlPropagateKey)
	}
	if v, written := be.values[ttlPropagateKey]; written {
		t.Errorf("the kernel received %s=%s after a refused write", ttlPropagateKey, v)
	}
}
