package engine

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ecnInstalled drives the production Child SA installer and returns every SAParams that
// actually reached a dataplane backend.
//
// createFirstChildSA derives KEYMAT and calls installChildSA, which is the ONLY producer of
// the SAParams a backend ever sees for an IKEv2 Child SA. Driving it means an installer that
// stops installing, or that installs a differently shaped SA, changes what the callers below
// observe.
func ecnInstalled(t *testing.T) []dataplane.SAParams {
	t.Helper()
	log := slogutil.DiscardLogger()
	local, _, _, _, _ := lcyLoopback(t)
	dp := &rkyDP{}
	if _, err := createFirstChildSA(local, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.installed) == 0 {
		t.Fatal("the Child SA installer programmed no SA at all, so nothing below is measuring an installed SA")
	}
	return dp.installed
}

// ecnSetFieldsMentioning returns the names of the fields of v set to a non-zero value whose
// name mentions any of want. It reflects over a VALUE production produced, never a literal.
func ecnSetFieldsMentioning(t *testing.T, v any, want ...string) []string {
	t.Helper()
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		t.Fatalf("%T is not a struct, so its fields cannot be enumerated", v)
	}
	if rv.NumField() == 0 {
		t.Fatalf("%T exposes no fields, so this scan would pass vacuously", v)
	}
	var hits []string
	for i := range rv.NumField() {
		name := strings.ToLower(rv.Type().Field(i).Name)
		matched := false
		for _, w := range want {
			if strings.Contains(name, w) {
				matched = true
				break
			}
		}
		if matched && !rv.Field(i).IsZero() {
			hits = append(hits, name)
		}
	}
	return hits
}

// VALIDATES: every Child SA ze installs is a tunnel-mode SA carrying no instruction that
// could disable ECN propagation. This is the cross-platform half of the Section 2.24
// evidence: it runs the production installer on every OS, where the netlink half runs only
// on Linux.
//
// PREVENTS: the vacuous predecessor in the dataplane package, which reflected over the
// SAParams TYPE and so passed with the installer disabled. It also prevents a future config
// leaf or proposal field reaching the backend with an ECN knob set.
//
// RFC requirement: RFC7296-2.24-1 positive -- RFC 7296 Section 2.24: "tunnel encapsulators
// and decapsulators for all tunnel mode SAs created by IKEv2 MUST support the ECN
// full-functionality option for tunnels". The SAs ze creates are tunnel mode, and the
// description ze hands the backend asks for no departure from the backend's default.
// RFC requirement: RFC7296-2.24-2 positive -- the same section: implementations "MUST
// implement the tunnel encapsulation and decapsulation processing specified in [IPSECARCH]
// to prevent discarding of ECN congestion indications". Ze delegates that processing, and
// this proves it never asks for it to be turned off.
func TestEcnInstalledChildSAAsksForNoECNChange(t *testing.T) {
	for _, p := range ecnInstalled(t) {
		if p.Mode != dataplane.ModeTunnel {
			t.Errorf("the installer programmed SPI %#x in mode %d, want tunnel (%d); "+
				"Section 2.24 binds tunnel mode", p.SPI, p.Mode, dataplane.ModeTunnel)
		}
		if hits := ecnSetFieldsMentioning(t, p, "ecn", "flag"); len(hits) != 0 {
			t.Errorf("the SA ze installed for SPI %#x sets %v, so a path to ECN behavior exists; "+
				"it must be proven not to disable propagation", p.SPI, hits)
		}
	}
}

// RFC requirement: RFC7296-2.24-1 negative -- the scan above is not vacuous and its subject
// is not a literal. The installer programs BOTH directions of the pair, and each carries the
// endpoints and a key, so a check reading an empty or half-built description fails here
// instead of passing quietly.
// RFC requirement: RFC7296-2.24-2 negative -- Section 2.24 binds a TUNNEL mode SA, so the
// installed description must be able to say it is one. A vocabulary in which tunnel and
// transport are indistinguishable would make the positive rows hold over nothing.
func TestEcnTheInstalledSAsAreRealAndDirectional(t *testing.T) {
	installed := ecnInstalled(t)
	if len(installed) < 2 {
		t.Fatalf("the installer programmed %d SA(s); a Child SA is a PAIR, so both directions must be measured",
			len(installed))
	}
	for _, p := range installed {
		if p.SPI == 0 {
			t.Error("an installed SA carries no SPI, so the scan is reading an unfilled description")
		}
		if p.Src == nil || p.Dst == nil {
			t.Errorf("the SA for SPI %#x carries no endpoint pair, so it is not a tunnel description", p.SPI)
		}
		if len(p.EncKey) == 0 {
			t.Errorf("the SA for SPI %#x carries no key, so it was never really built", p.SPI)
		}
	}
	if installed[0].SPI == installed[1].SPI {
		t.Error("both installed SAs share one SPI, so the pair is not directional")
	}
	if dataplane.ModeTunnel == dataplane.ModeTransport {
		t.Fatal("tunnel and transport share a value, so no SA can be identified as tunnel mode")
	}
}
