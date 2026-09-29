// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing config resolve.
// Related: sr_config.go -- parseSegmentRouting and applySRConfig, the path these tests drive.
// Related: rfc8665_test.go -- the builder-level ordering test and the shared SR helpers.
//
// VALIDATES: RFC 8665 section 3.2 SID/Label Range order across a restart, driven from the
// configuration text through parseOSPFConfig, applySRConfig (parseSegmentRouting) and
// srBuildSRGB, with the process-global SR store rebuilt in between.
// PREVENTS: a config resolve step that reorders or drops ranges, so the advertised
// SID/Label Range order differs after a restart.
package ospf

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc8665RangeOrderAdvertised applies one configuration document through the production
// config path and returns the encoded SID/Label Range TLVs this router advertises.
func rfc8665RangeOrderAdvertised(t *testing.T, document string) []byte {
	t.Helper()
	sections := ospfSec(document)
	cfg, err := parseOSPFConfig(sections, nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	applySRConfig(sections, cfg)
	return packet.EncodeRITLVs(srBuildSRGB(cfg.RouterID))
}

// RFC requirement: RFC8665-3.2-5 positive -- the SID/Label Range TLVs advertised after a
// restart (the SR store rebuilt from nothing and the same configuration applied again
// through parseSegmentRouting) are byte-identical to those advertised before it, and hold
// the one configured SRGB range (base 16000, 8000 labels). The same configuration written
// with its members in another key order advertises the same bytes.
func TestRFC8665RangeOrderStableThroughConfigRestart(t *testing.T) {
	// Goal: the SRGB order survives a restart on the real config path. Method: resolve the
	// configuration text, advertise, clear the process-global SR store, resolve again.
	srTestReset(t)
	const document = `{"ospf":{"router-id":"10.0.0.1","segment-routing":{"enable":true,` +
		`"srgb":{"lower-bound":16000,"upper-bound":23999},"srlb":{"lower-bound":40000,"upper-bound":40999}}}}`
	before := rfc8665RangeOrderAdvertised(t, document)

	srWire.clear()
	after := rfc8665RangeOrderAdvertised(t, document)
	if !bytes.Equal(before, after) {
		t.Fatalf("SID/Label Range TLVs changed across a restart:\nbefore %x\nafter  %x", before, after)
	}

	const reordered = `{"ospf":{"segment-routing":{"srlb":{"upper-bound":40999,"lower-bound":40000},` +
		`"srgb":{"upper-bound":23999,"lower-bound":16000},"enable":true},"router-id":"10.0.0.1"}}`
	srWire.clear()
	if other := rfc8665RangeOrderAdvertised(t, reordered); !bytes.Equal(before, other) {
		t.Fatalf("SID/Label Range TLVs depend on the document key order:\nwant %x\ngot  %x", before, other)
	}

	tlvs := srBuildSRGB(types.RouterID{10, 0, 0, 1})
	if len(tlvs) != 1 || tlvs[0].Type != sr.V4TypeSRGB {
		t.Fatalf("advertised %+v, want the one configured SRGB range", tlvs)
	}
	r, err := sr.DecodeRangeValue(tlvs[0].Value)
	if err != nil {
		t.Fatalf("SID/Label Range TLV does not decode: %v", err)
	}
	if r.Base != 16000 || r.Size != 8000 {
		t.Fatalf("SID/Label Range = base %d size %d, want base 16000 size 8000", r.Base, r.Size)
	}
}
