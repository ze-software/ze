// Design: docs/architecture/core-design.md -- sysctl known-keys registry
// Related: known_linux.go -- refuseMPLSTTLNoPropagate
//
// VALIDATES: net.mpls.ip_ttl_propagate is a known key whose registration
// refuses 0 with an operator error naming RFC 3032 Section 2.4.3, accepts 1,
// and still refuses a value that is not a bool. Method: Validate over the
// registration init() made.
// PREVENTS: an operator turning off TTL propagation, after which Linux writes
// net.mpls.default_ttl into every label fib-kernel pushes instead of the IP
// TTL, with nothing refusing the value.
//
// This file sorts before known_test.go, whose tests reset the registry, so it
// runs against the registrations init() made.

//go:build linux

package sysctl

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

func TestMPLSTTLPropagateRefusesZero(t *testing.T) {
	const key = "net.mpls.ip_ttl_propagate"
	def, ok := Lookup(key)
	if !ok {
		t.Fatalf("%s is not a known key, so a configured value is never checked", key)
	}
	if def.Constraint == nil {
		t.Fatalf("%s registers no Constraint, so 0 is accepted", key)
	}

	if err := Validate(key, "1"); err != nil {
		t.Errorf("%s=1 refused: %v", key, err)
	}

	err := Validate(key, "0")
	if err == nil {
		t.Fatalf("%s=0 accepted; the kernel would then impose net.mpls.default_ttl on every pushed label", key)
	}
	var operr *opererr.Error
	if !errors.As(err, &operr) {
		t.Fatalf("%s=0 refusal is %T, want an operator error with a problem, a why and a fix: %v", key, err, err)
	}
	if !strings.Contains(operr.Why(), "RFC 3032 Section 2.4.3") {
		t.Errorf("refusal why = %q, want it to name RFC 3032 Section 2.4.3", operr.Why())
	}
	if !strings.Contains(operr.Fix(), key) {
		t.Errorf("refusal fix = %q, want it to name %s", operr.Fix(), key)
	}

	if err := Validate(key, "2"); err == nil {
		t.Errorf("%s=2 accepted; the key is a bool", key)
	}
}
