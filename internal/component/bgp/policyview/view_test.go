package policyview

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// pathAttr encodes one path attribute: flags, code, one-octet length, value.
func pathAttr(flags byte, code attribute.AttributeCode, value []byte) []byte {
	return append([]byte{flags, byte(code), byte(len(value))}, value...)
}

// updatePayload builds an UPDATE body with no withdrawn routes.
func updatePayload(attrs [][]byte, nlri []byte) []byte {
	var section []byte
	for _, a := range attrs {
		section = append(section, a...)
	}
	out := []byte{0, 0}
	out = binary.BigEndian.AppendUint16(out, uint16(len(section)))
	out = append(out, section...)
	return append(out, nlri...)
}

// asSequence encodes an AS_SEQUENCE segment of four-octet AS numbers.
func asSequence(asns ...uint32) []byte {
	out := []byte{byte(attribute.ASSequence), byte(len(asns))}
	for _, asn := range asns {
		out = binary.BigEndian.AppendUint32(out, asn)
	}
	return out
}

func registerContext(t *testing.T, ctx *bgpctx.EncodingContext) bgpctx.ContextID {
	t.Helper()
	id, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return id
}

func fourOctet(t *testing.T) bgpctx.ContextID {
	t.Helper()
	return registerContext(t, bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
}

// fullUpdate carries one body prefix and every attribute the view reads.
func fullUpdate() []byte {
	return updatePayload([][]byte{
		pathAttr(0x40, attribute.AttrOrigin, []byte{0}),
		pathAttr(0x40, attribute.AttrASPath, asSequence(65001, 4200000000)),
		pathAttr(0x40, attribute.AttrNextHop, []byte{192, 0, 2, 1}),
		pathAttr(0x80, attribute.AttrMED, []byte{0, 0, 0, 50}),
		pathAttr(0x40, attribute.AttrLocalPref, []byte{0, 0, 0, 200}),
		pathAttr(0x40, attribute.AttrAtomicAggregate, nil),
		pathAttr(0xc0, attribute.AttrAggregator, []byte{0, 0, 0xfd, 0xe9, 198, 51, 100, 1}),
		pathAttr(0xc0, attribute.AttrCommunity, []byte{0xfd, 0xe9, 0, 1}),
		pathAttr(0x80, attribute.AttrOriginatorID, []byte{10, 0, 0, 1}),
		pathAttr(0x80, attribute.AttrClusterList, []byte{10, 0, 0, 2, 10, 0, 0, 3}),
		pathAttr(0xc0, attribute.AttrExtCommunity, []byte{0, 2, 0xfd, 0xe9, 0, 0, 0, 1}),
		pathAttr(0x80, attribute.AttrAIGP, []byte{1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 7}),
		pathAttr(0xc0, attribute.AttrLargeCommunity, []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 0, 0, 0, 2}),
	}, []byte{24, 203, 0, 113})
}

func mustReader(t *testing.T, field policy.Field) Reader {
	t.Helper()
	r, err := ReaderFor(field)
	if err != nil {
		t.Fatalf("ReaderFor(%s): %v", field, err)
	}
	return r
}

// TestWireViewDecodesOnDemand proves AC-1 and AC-23 on the view: a member
// reading only prefix decodes the NLRI and no attribute, with zero
// allocations per run; reading one of the four D-8 fields decodes only that
// attribute. Method: one UPDATE carrying every attribute, a reused View, the
// decode counter and testing.AllocsPerRun.
func TestWireViewDecodesOnDemand(t *testing.T) {
	update := wireu.NewWireUpdate(fullUpdate(), fourOctet(t))
	if _, err := update.Attrs(); err != nil { // the reactor indexed it at ingress
		t.Fatalf("attrs: %v", err)
	}
	prefix := mustReader(t, policy.FieldPrefix)
	var view View

	view.Reset(update, Session{EBGP: true})
	got, err := view.Read(prefix)
	if err != nil {
		t.Fatalf("read prefix: %v", err)
	}
	if !got.Present() || len(got.Wire()) != 4 || got.Wire()[0] != 24 {
		t.Fatalf("prefix wire = %v, want the body NLRI 24 203.0.113", got.Wire())
	}
	d := view.Decodes()
	if d.NLRI() != 1 || d.Attributes() != 0 {
		t.Fatalf("prefix read decoded nlri=%d attributes=%d, want 1 and 0", d.NLRI(), d.Attributes())
	}

	allocs := testing.AllocsPerRun(100, func() {
		view.Reset(update, Session{EBGP: true})
		if _, err := view.Read(prefix); err != nil {
			t.Fatal(err)
		}
		view.Clear()
	})
	if allocs != 0 {
		t.Fatalf("prefix-only run allocated %.1f times, want 0", allocs)
	}

	for _, tc := range []struct {
		field policy.Field
		code  attribute.AttributeCode
	}{
		{policy.FieldAtomicAggregate, attribute.AttrAtomicAggregate},
		{policy.FieldAggregator, attribute.AttrAggregator},
		{policy.FieldOriginatorID, attribute.AttrOriginatorID},
		{policy.FieldClusterList, attribute.AttrClusterList},
	} {
		r := mustReader(t, tc.field)
		view.Reset(update, Session{})
		x, err := view.Read(r)
		if err != nil {
			t.Fatalf("read %s: %v", tc.field, err)
		}
		if !x.Present() {
			t.Fatalf("%s absent from an UPDATE carrying it", tc.field)
		}
		if _, err := view.Read(r); err != nil { // a second member reads it again
			t.Fatal(err)
		}
		d := view.Decodes()
		if d.Attributes() != 1 || d.Distinct() != 1 || !d.Decoded(tc.code) || d.NLRI() != 0 {
			t.Fatalf("%s: decoded attributes=%d distinct=%d nlri=%d, want only %s once",
				tc.field, d.Attributes(), d.Distinct(), d.NLRI(), tc.code)
		}
		allocs := testing.AllocsPerRun(50, func() {
			view.Reset(update, Session{})
			if _, err := view.Read(r); err != nil {
				t.Fatal(err)
			}
		})
		if allocs != 0 {
			t.Fatalf("%s read allocated %.1f times, want 0", tc.field, allocs)
		}
	}
}

// TestWireViewFieldValues pins what each reader answers on a well-formed
// UPDATE, so a value is the peer's and never a default.
func TestWireViewFieldValues(t *testing.T) {
	update := wireu.NewWireUpdate(fullUpdate(), fourOctet(t))
	var view View
	view.Reset(update, Session{EBGP: true})
	number := func(field policy.Field) uint64 {
		t.Helper()
		x, err := view.Read(mustReader(t, field))
		if err != nil {
			t.Fatalf("read %s: %v", field, err)
		}
		n, ok := x.Number()
		if !ok {
			t.Fatalf("%s carries no number", field)
		}
		return n
	}
	for field, want := range map[policy.Field]uint64{
		policy.FieldOrigin: 0, policy.FieldMED: 50, policy.FieldLocalPreference: 200,
		policy.FieldAggregator: 65001, policy.FieldAIGP: 7, policy.FieldOriginAS: 4200000000,
	} {
		if got := number(field); got != want {
			t.Errorf("%s = %d, want %d", field, got, want)
		}
	}
	x, err := view.Read(mustReader(t, policy.FieldFamily))
	if err != nil || x.Family() != family.IPv4Unicast {
		t.Errorf("family = %v (%v), want ipv4/unicast", x.Family(), err)
	}
	x, err = view.Read(mustReader(t, policy.FieldProtocol))
	if err != nil || !x.EBGP() {
		t.Errorf("protocol ebgp = %v (%v), want true", x.EBGP(), err)
	}
	x, err = view.Read(mustReader(t, policy.FieldCommunity))
	if err != nil || len(x.Wire()) != 4 {
		t.Errorf("community wire = %v (%v), want 4 octets", x.Wire(), err)
	}
	x, err = view.Read(mustReader(t, policy.FieldClusterList))
	if err != nil || len(x.Wire()) != 8 {
		t.Errorf("cluster-list wire = %v (%v), want 8 octets", x.Wire(), err)
	}
}

// TestWireViewAbsentIsNotZero proves an attribute the UPDATE does not carry
// reads as absent, never as a zero value.
func TestWireViewAbsentIsNotZero(t *testing.T) {
	payload := updatePayload([][]byte{
		pathAttr(0x40, attribute.AttrOrigin, []byte{0}),
		pathAttr(0x40, attribute.AttrASPath, asSequence(65001)),
		pathAttr(0x40, attribute.AttrNextHop, []byte{192, 0, 2, 1}),
	}, []byte{24, 203, 0, 113})
	var view View
	view.Reset(wireu.NewWireUpdate(payload, fourOctet(t)), Session{})
	for _, field := range []policy.Field{policy.FieldMED, policy.FieldCommunity, policy.FieldAggregator,
		policy.FieldAtomicAggregate, policy.FieldOriginatorID, policy.FieldClusterList, policy.FieldAIGP} {
		x, err := view.Read(mustReader(t, field))
		if err != nil {
			t.Fatalf("read %s: %v", field, err)
		}
		if x.Present() {
			t.Errorf("%s present on an UPDATE that does not carry it", field)
		}
		if _, ok := x.Number(); ok {
			t.Errorf("%s answers a number while absent", field)
		}
	}
}

// TestWireViewMalformedAttributeErrors proves AC-28 and R-4 on the view: each
// field whose attribute breaks its RFC 7606 shape answers an error wrapping
// ErrMalformed and no value, so the chain applies the member's on-error.
func TestWireViewMalformedAttributeErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field policy.Field
		attr  []byte
	}{
		{"origin length", policy.FieldOrigin, pathAttr(0x40, attribute.AttrOrigin, []byte{0, 0})},
		{"origin value", policy.FieldOrigin, pathAttr(0x40, attribute.AttrOrigin, []byte{3})},
		{"next-hop length", policy.FieldNextHop, pathAttr(0x40, attribute.AttrNextHop, []byte{192, 0, 2})},
		{"med length", policy.FieldMED, pathAttr(0x80, attribute.AttrMED, []byte{0, 0, 50})},
		{"local-preference length", policy.FieldLocalPreference, pathAttr(0x40, attribute.AttrLocalPref, []byte{0, 0, 0, 0, 1})},
		{"atomic-aggregate length", policy.FieldAtomicAggregate, pathAttr(0x40, attribute.AttrAtomicAggregate, []byte{1})},
		{"aggregator length", policy.FieldAggregator, pathAttr(0xc0, attribute.AttrAggregator, []byte{0xfd, 0xe9, 198, 51, 100, 1})},
		{"community length", policy.FieldCommunity, pathAttr(0xc0, attribute.AttrCommunity, []byte{0xfd, 0xe9, 0})},
		{"extended-community length", policy.FieldExtendedCommunity, pathAttr(0xc0, attribute.AttrExtCommunity, []byte{0, 2, 0, 0})},
		{"large-community length", policy.FieldLargeCommunity, pathAttr(0xc0, attribute.AttrLargeCommunity, []byte{0, 0, 0, 1, 0, 0, 0, 2})},
		{"originator-id length", policy.FieldOriginatorID, pathAttr(0x80, attribute.AttrOriginatorID, []byte{10, 0, 0})},
		{"cluster-list empty", policy.FieldClusterList, pathAttr(0x80, attribute.AttrClusterList, nil)},
		{"aigp overrun", policy.FieldAIGP, pathAttr(0x80, attribute.AttrAIGP, []byte{1, 0, 20, 0, 0})},
		{"as-path overrun", policy.FieldASPath, pathAttr(0x40, attribute.AttrASPath, []byte{2, 3, 0, 0, 0xfd, 0xe9})},
		{"origin-as from a malformed as-path", policy.FieldOriginAS, pathAttr(0x40, attribute.AttrASPath, []byte{2, 0})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := updatePayload([][]byte{tc.attr}, []byte{24, 203, 0, 113})
			var view View
			view.Reset(wireu.NewWireUpdate(payload, fourOctet(t)), Session{})
			x, err := view.Read(mustReader(t, tc.field))
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("read %s: err = %v, want ErrMalformed", tc.field, err)
			}
			if x.Present() {
				t.Fatalf("read %s answered a value beside the error", tc.field)
			}
		})
	}
}

// TestWireViewASPathBuiltOnce proves D-7: the AS path is built once per run
// however many members read as-path and origin-as, and a new run builds it
// again rather than reading the last run's.
func TestWireViewASPathBuiltOnce(t *testing.T) {
	update := wireu.NewWireUpdate(fullUpdate(), fourOctet(t))
	asPath, originAS := mustReader(t, policy.FieldASPath), mustReader(t, policy.FieldOriginAS)
	var view View
	view.Reset(update, Session{})
	first, err := view.Read(asPath)
	if err != nil || first.ASPath() == nil {
		t.Fatalf("read as-path: %v", err)
	}
	for range 3 {
		x, err := view.Read(asPath)
		if err != nil || x.ASPath() != first.ASPath() {
			t.Fatalf("as-path rebuilt within one run (%v)", err)
		}
		if _, err := view.Read(originAS); err != nil {
			t.Fatal(err)
		}
	}
	if d := view.Decodes(); d.Attributes() != 1 || !d.Decoded(attribute.AttrASPath) {
		t.Fatalf("one run decoded %d attributes, want AS_PATH once", d.Attributes())
	}
	view.Reset(update, Session{})
	if _, err := view.Read(originAS); err != nil {
		t.Fatal(err)
	}
	if d := view.Decodes(); d.Attributes() != 1 {
		t.Fatalf("the second run decoded %d attributes, want AS_PATH once", d.Attributes())
	}
}

// TestWireViewOriginASRFC6811 pins the origin AS of RFC 6811 Section 2: the
// rightmost AS of a final AS_SEQUENCE, NONE for a final AS_SET, and the
// speaker's own AS for an empty path.
func TestWireViewOriginASRFC6811(t *testing.T) {
	asSet := []byte{byte(attribute.ASSet), 1, 0, 0, 0xfd, 0xea}
	for _, tc := range []struct {
		name     string
		path     []byte
		want     uint64
		numbered bool
	}{
		{"sequence", asSequence(65001, 65002), 65002, true},
		{"set is NONE", append(asSequence(65001), asSet...), 0, false},
		{"empty is the local AS", nil, 65000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := updatePayload([][]byte{pathAttr(0x40, attribute.AttrASPath, tc.path)}, []byte{24, 203, 0, 113})
			var view View
			view.Reset(wireu.NewWireUpdate(payload, fourOctet(t)), Session{})
			x, err := view.Read(mustReader(t, policy.FieldOriginAS))
			if err != nil {
				t.Fatal(err)
			}
			got, numbered := x.Number()
			if !x.Present() || numbered != tc.numbered || got != tc.want {
				t.Fatalf("origin-as = %d numbered=%v, want %d numbered=%v", got, numbered, tc.want, tc.numbered)
			}
		})
	}
}

// TestWireViewTwoOctetRefused proves an UPDATE the ingest collapse did not
// rewrite to the four-octet form fails the AS-number fields instead of being
// read at the wrong width.
func TestWireViewTwoOctetRefused(t *testing.T) {
	ctx := registerContext(t, bgpctx.EncodingContextForASN4(false))
	payload := updatePayload([][]byte{pathAttr(0x40, attribute.AttrASPath, []byte{2, 1, 0xfd, 0xe9})}, nil)
	var view View
	view.Reset(wireu.NewWireUpdate(payload, ctx), Session{})
	for _, field := range []policy.Field{policy.FieldASPath, policy.FieldOriginAS, policy.FieldAggregator} {
		if _, err := view.Read(mustReader(t, field)); err == nil {
			t.Errorf("read %s on a two-octet UPDATE answered no error", field)
		}
	}
}

// TestWireViewClearedReadsNothing proves R-3 on the view: after Clear, a view
// kept by mistake answers ErrNoUpdate rather than a value from a buffer the
// reactor reused.
func TestWireViewClearedReadsNothing(t *testing.T) {
	var view View
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), Session{})
	prefix := mustReader(t, policy.FieldPrefix)
	if _, err := view.Read(prefix); err != nil {
		t.Fatal(err)
	}
	view.Clear()
	if _, err := view.Read(prefix); !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("read after Clear: err = %v, want ErrNoUpdate", err)
	}
}

// TestReaderForUnregistered proves a field no BGP reader serves is refused
// at compile time, never read as an empty value.
func TestReaderForUnregistered(t *testing.T) {
	if _, err := ReaderFor(policy.FieldMetric); !errors.Is(err, ErrNoReader) {
		t.Fatalf("ReaderFor(metric): err = %v, want ErrNoReader", err)
	}
}

// TestZeroReaderRefused proves a Reader ReaderFor did not produce reads no
// field, rather than silently reading the first registered one.
func TestZeroReaderRefused(t *testing.T) {
	var view View
	view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)), Session{})
	if _, err := view.Read(Reader{}); err == nil {
		t.Fatal("the zero Reader read a field")
	}
}

// TestWireViewEveryReaderAllocationFree proves A-1: every registered field
// reader decodes from the wire bytes without allocating, once the UPDATE's
// attribute index exists. Method: each registered field read alone in a
// reused view, under testing.AllocsPerRun.
func TestWireViewEveryReaderAllocationFree(t *testing.T) {
	update := wireu.NewWireUpdate(fullUpdate(), fourOctet(t))
	var view View
	for field := range readerTable.slots {
		r := mustReader(t, field)
		view.Reset(update, Session{})
		if x, err := view.Read(r); err != nil || !x.Present() {
			t.Fatalf("read %s: present=%v err=%v", field, x.Present(), err)
		}
		allocs := testing.AllocsPerRun(50, func() {
			view.Reset(update, Session{})
			if _, err := view.Read(r); err != nil {
				t.Fatal(err)
			}
		})
		if allocs != 0 {
			t.Errorf("%s read allocated %.1f times, want 0", field, allocs)
		}
	}
}
