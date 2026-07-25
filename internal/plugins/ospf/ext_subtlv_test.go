// VALIDATES: spec-ospf-ext-4 -- the generic Extended Prefix/Link sub-TLV registration hook:
// a registered codec is dispatched for a matching received sub-TLV and an unknown sub-TLV is
// skipped (AC-11); a panicking codec is recovered and counted, never crashing OSPF (AC-16/R-9);
// and this spec's package names no Segment Routing / SID spelling (R-7).
// PREVENTS: an unknown sub-TLV crashing decode, a bad codec taking down the engine, or SR
// leaking into the carrier layer.
package ospf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

func TestRegisterPrefixSubTLVDispatched(t *testing.T) {
	resetExtSubTLVs()
	t.Cleanup(resetExtSubTLVs)

	var gotValue []byte
	if err := registerPrefixSubTLV(7, extSubTLVCodec{Receive: func(v []byte) { gotValue = append([]byte(nil), v...) }}); err != nil {
		t.Fatalf("registerPrefixSubTLV: %v", err)
	}
	// A matching sub-TLV is dispatched to the codec.
	dispatchPrefixSubTLV(packet.ExtSubTLV{Type: 7, Value: []byte{1, 2, 3, 4}}, func() { t.Fatalf("unexpected panic") })
	if !bytes.Equal(gotValue, []byte{1, 2, 3, 4}) {
		t.Fatalf("codec not dispatched, got %v", gotValue)
	}
	// An unknown sub-TLV type is skipped without error and without dispatch.
	gotValue = nil
	dispatchPrefixSubTLV(packet.ExtSubTLV{Type: 99, Value: []byte{9}}, func() { t.Fatalf("unexpected panic") })
	if gotValue != nil {
		t.Fatalf("unknown sub-TLV must not dispatch, got %v", gotValue)
	}
	// Reserved type 0 and duplicates are rejected.
	if err := registerPrefixSubTLV(0, extSubTLVCodec{}); err == nil {
		t.Fatalf("reserved type 0 must be rejected")
	}
	if err := registerPrefixSubTLV(7, extSubTLVCodec{}); err == nil {
		t.Fatalf("duplicate type must be rejected")
	}
}

func TestRegisterLinkSubTLVDispatched(t *testing.T) {
	resetExtSubTLVs()
	t.Cleanup(resetExtSubTLVs)

	called := false
	if err := registerLinkSubTLV(4, extSubTLVCodec{Receive: func([]byte) { called = true }}); err != nil {
		t.Fatalf("registerLinkSubTLV: %v", err)
	}
	dispatchLinkSubTLV(packet.ExtSubTLV{Type: 4, Value: []byte{0, 0, 0, 1}}, func() { t.Fatalf("panic") })
	if !called {
		t.Fatalf("link sub-TLV codec not dispatched")
	}
	// Registered builder contributes bytes during origination.
	if err := registerLinkSubTLV(5, extSubTLVCodec{Build: func(extSubTLVContext) []packet.ExtSubTLV {
		return []packet.ExtSubTLV{{Type: 5, Value: []byte{9, 9, 9, 9}}}
	}}); err != nil {
		t.Fatalf("registerLinkSubTLV build: %v", err)
	}
	subs := buildLinkSubTLVs(extSubTLVContext{}, func() {})
	found := false
	for _, s := range subs {
		if s.Type == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("registered builder did not contribute sub-TLV, got %+v", subs)
	}
}

func TestSubTLVCodecPanicIsolated(t *testing.T) {
	resetExtSubTLVs()
	t.Cleanup(resetExtSubTLVs)

	if err := registerPrefixSubTLV(8, extSubTLVCodec{Receive: func([]byte) { panic("boom") }}); err != nil {
		t.Fatalf("register: %v", err)
	}
	errs := 0
	// A panicking codec is recovered; the onPanic counter fires and no panic propagates.
	dispatchPrefixSubTLV(packet.ExtSubTLV{Type: 8, Value: []byte{1}}, func() { errs++ })
	if errs != 1 {
		t.Fatalf("panic recovery counter = %d, want 1", errs)
	}
	// A panicking builder contributes nothing and is counted.
	if err := registerPrefixSubTLV(9, extSubTLVCodec{Build: func(extSubTLVContext) []packet.ExtSubTLV { panic("boom2") }}); err != nil {
		t.Fatalf("register build: %v", err)
	}
	buildErrs := 0
	out := buildPrefixSubTLVs(extSubTLVContext{}, func() { buildErrs++ })
	if buildErrs != 1 {
		t.Fatalf("build panic counter = %d, want 1", buildErrs)
	}
	if len(out) != 0 {
		t.Fatalf("panicking builder must contribute nothing, got %+v", out)
	}
}

func TestSubTLVRegistryGenericNoSRSpelling(t *testing.T) {
	// R-7: this spec's carrier files must name no Segment Routing / SID concept; the registry
	// is generic and ext-5 registers from its own package.
	forbidden := []string{"segment", "srgb", "srlb", "prefix-sid", "adj-sid", "adjacency-sid", "sid sub-tlv"}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "ext_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		scanned++
		lower := strings.ToLower(string(data))
		for _, tok := range forbidden {
			if strings.Contains(lower, tok) {
				t.Errorf("%s contains forbidden SR spelling %q (self-containment R-7)", name, tok)
			}
		}
	}
	if scanned == 0 {
		t.Fatalf("no ext_*.go carrier files scanned")
	}
}
