package nlritype

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// evpnFamily is the one family RFC 7606 Section 5.4 binds in ze today. The tests
// register their own recognizer against it rather than importing the EVPN plugin,
// which would invert the dependency direction (core must not import a plugin).
var evpnFamily = family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}

// evpnNLRI frames one EVPN NLRI as [route-type][length][body] (RFC 7432 Section 7.1).
func evpnNLRI(routeType byte, body ...byte) []byte {
	out := []byte{routeType, byte(len(body))}
	return append(out, body...)
}

// recognizeUpToFive accepts EVPN route types 1..5, the set ze implements.
func recognizeUpToFive(nlriBytes []byte, addPath bool) Verdict {
	off := 0
	if addPath {
		off = 4
	}
	if off >= len(nlriBytes) {
		return Discard
	}
	if rt := nlriBytes[off]; rt >= 1 && rt <= 5 {
		return Keep
	}
	return Discard
}

// retain runs Retain over an announcement and reads the two results the discard tests
// assert, so those tests keep their shape.
func retain(fn Recognizer, fam family.Family, data []byte, addPath bool) ([]byte, int, error) {
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	r, err := Retain(fn, fam, data, addPath, true, scratch[:])
	return r.Kept, r.Discarded, err
}

func withEVPNRecognizer(t *testing.T) {
	t.Helper()
	if err := Register(evpnFamily, recognizeUpToFive); err != nil {
		t.Fatalf("registering the evpn recognizer: %v", err)
	}
	t.Cleanup(ResetForTest)
}

// VALIDATES: a family nobody has ruled on keeps today's behavior, and pays no allocation.
// PREVENTS: an over-broad discard reaching a family whose specification was never read
// (spec risk R-1). The registry's default is the mitigation, so it is tested directly.
func TestRetainLeavesUnruledFamilyUntouched(t *testing.T) {
	ResetForTest()
	data := append(evpnNLRI(1, 0xaa), evpnNLRI(99, 0xbb)...)

	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, data, false)
	if err != nil {
		t.Fatalf("unruled family must not error: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("unruled family must drop nothing, dropped %d", dropped)
	}
	if &kept[0] != &data[0] {
		t.Fatal("unruled family must return the same backing array, not a copy")
	}
}

// VALIDATES: an NLRI whose route type ze implements survives, with the input array reused.
// PREVENTS: a rewrite on the common path, which would cost an allocation per UPDATE and
// break the zero-copy relay for every conforming peer.
func TestRetainKeepsRecognizedTypesZeroCopy(t *testing.T) {
	withEVPNRecognizer(t)
	data := append(evpnNLRI(2, 0xaa, 0xbb), evpnNLRI(5, 0xcc)...)

	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, data, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("every type is implemented, dropped %d", dropped)
	}
	if &kept[0] != &data[0] {
		t.Fatal("nothing was dropped, so the input array must be returned unchanged")
	}
}

// VALIDATES: an unrecognized route type is removed and the recognized ones around it
// survive in wire order.
// PREVENTS: the whole NLRI section being dropped because one entry was unfamiliar, and
// the surviving entries being reordered.
func TestRetainDropsOnlyTheUnrecognized(t *testing.T) {
	withEVPNRecognizer(t)
	keepA := evpnNLRI(2, 0xaa)
	drop := evpnNLRI(99, 0xde, 0xad)
	keepB := evpnNLRI(3, 0xbb)

	var data []byte
	data = append(data, keepA...)
	data = append(data, drop...)
	data = append(data, keepB...)

	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, data, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 1 {
		t.Fatalf("exactly one NLRI is unrecognized, dropped %d", dropped)
	}

	want := append(append([]byte{}, keepA...), keepB...)
	if !bytes.Equal(kept, want) {
		t.Fatalf("kept = %x, want %x", kept, want)
	}
}

// VALIDATES: when no NLRI survives, Retain reports an empty section rather than nil-with-
// zero-dropped, so the caller can tell "drop the whole attribute" from "nothing to do".
// PREVENTS: an UPDATE carrying only unrecognized types being relayed whole because the
// caller read an empty result as "unchanged".
func TestRetainReportsEverythingDropped(t *testing.T) {
	withEVPNRecognizer(t)
	data := append(evpnNLRI(99, 0xaa), evpnNLRI(200, 0xbb)...)

	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, data, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 2 {
		t.Fatalf("both NLRIs are unrecognized, dropped %d", dropped)
	}
	if len(kept) != 0 {
		t.Fatalf("no NLRI survives, kept %x", kept)
	}
}

// VALIDATES: RFC 7911 path identifier is skipped, so an ADD-PATH NLRI is judged on its
// route type and the surviving bytes keep their path id.
// PREVENTS: reading the first byte of a path id as a route type, which would discard every
// ADD-PATH EVPN route whose path id starts with a byte outside 1..5.
func TestRetainSkipsPathIDUnderAddPath(t *testing.T) {
	withEVPNRecognizer(t)
	pathID := []byte{0x00, 0x00, 0x00, 0x07}
	keep := append(append([]byte{}, pathID...), evpnNLRI(2, 0xaa)...)
	drop := append(append([]byte{}, pathID...), evpnNLRI(99, 0xbb)...)

	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, append(append([]byte{}, keep...), drop...), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 1 {
		t.Fatalf("one NLRI is unrecognized, dropped %d", dropped)
	}
	if !bytes.Equal(kept, keep) {
		t.Fatalf("kept = %x, want %x (path id must survive)", kept, keep)
	}
}

// VALIDATES: a malformed NLRI section is reported and left alone.
// PREVENTS: inventing NLRI boundaries out of untrustworthy framing and rewriting the wire
// from them. When the length field cannot be trusted, no discard decision is possible.
func TestRetainLeavesMalformedFramingAlone(t *testing.T) {
	withEVPNRecognizer(t)
	// Declares 200 body octets but only two follow.
	data := []byte{0x02, 0xc8, 0xaa, 0xbb}

	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, data, false)
	if err == nil {
		t.Fatal("malformed framing must be reported")
	}
	if dropped != 0 {
		t.Fatalf("malformed framing must drop nothing, dropped %d", dropped)
	}
	if &kept[0] != &data[0] {
		t.Fatal("malformed framing must return the input untouched")
	}
}

// VALIDATES: an empty NLRI section is not an error and allocates nothing.
// PREVENTS: an MP_UNREACH with no withdrawn routes being treated as malformed.
func TestRetainAcceptsEmptySection(t *testing.T) {
	withEVPNRecognizer(t)
	kept, dropped, err := retain(Get(evpnFamily), evpnFamily, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 0 || len(kept) != 0 {
		t.Fatalf("empty section: kept %x dropped %d", kept, dropped)
	}
}

// VALIDATES: registering a recognizer for a family with no framing splitter is refused at
// startup, and the family stays unruled.
// PREVENTS: a runtime state where ze knows a family's types must be judged but cannot carve
// its NLRIs, which would either fail open (violating 5.4) or fail closed (discarding every
// route). Neither is permitted at run time, so the contradiction is reported at init.
func TestRegisterWithoutSplitterIsRefused(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	unsplittable := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	if nlrisplit.Supported(unsplittable) {
		t.Skip("bgp-ls gained a splitter; pick another family with none")
	}

	err := Register(unsplittable, recognizeUpToFive)
	if !errors.Is(err, ErrNoSplitter) {
		t.Fatalf("registering with no splitter must return ErrNoSplitter, got %v", err)
	}
	if Get(unsplittable) != nil {
		t.Fatal("a refused registration must leave the family unruled")
	}
}

// VALIDATES: a second ruling for one family is refused.
// PREVENTS: two plugins disagreeing about which route types a family implements, with the
// winner decided by init order.
func TestRegisterDuplicateIsRefused(t *testing.T) {
	withEVPNRecognizer(t)
	if err := Register(evpnFamily, recognizeUpToFive); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("a second recognizer must return ErrDuplicate, got %v", err)
	}
}

// VALIDATES: a nil recognizer is accepted and rules nothing.
// PREVENTS: a family whose specification overrides 5.4 having to be special-cased inside
// this package to say so.
func TestRegisterNilIsNoRuling(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	if err := Register(evpnFamily, nil); err != nil {
		t.Fatalf("a nil recognizer must be accepted: %v", err)
	}
	if Get(evpnFamily) != nil {
		t.Fatal("a nil recognizer must leave the family unruled")
	}
}

// VALIDATES: Get returns the ruling, and returns nil for a family nobody ruled on.
// PREVENTS: a caller reading "no recognizer" as "discard everything".
func TestGetReturnsOnlyRuledFamilies(t *testing.T) {
	withEVPNRecognizer(t)
	if Get(evpnFamily) == nil {
		t.Fatal("evpn has a recognizer, so 5.4 binds it")
	}
	if Get(family.IPv4Unicast) != nil {
		t.Fatal("ipv4/unicast has no recognizer, so nothing binds it")
	}
}

// VALIDATES: RegisterBodyRecognizer installs the recognizer as Register does and marks
// the family as one whose recognizer judges the body; Register alone does not, a nil
// recognizer marks nothing, a refused registration marks nothing, and SnapshotForTest
// restores the mark.
// PREVENTS: a family that withdraws malformed NLRIs left without the held generation its
// withdrawals need, and a type-only family paying for one.
func TestRegisterBodyRecognizerMarksFamily(t *testing.T) {
	t.Cleanup(SnapshotForTest())

	if err := Register(evpnFamily, recognizeUpToFive); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if JudgesBody(evpnFamily) {
		t.Fatal("Register alone must not mark the family as judging the body")
	}
	if err := RegisterBodyRecognizer(mupFamily, nil); err != nil {
		t.Fatalf("a nil recognizer must be accepted: %v", err)
	}
	if JudgesBody(mupFamily) {
		t.Fatal("a nil recognizer must mark nothing")
	}
	noSplitter := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFI(250)}
	if err := RegisterBodyRecognizer(noSplitter, withdrawAll); !errors.Is(err, ErrNoSplitter) {
		t.Fatalf("want ErrNoSplitter, got %v", err)
	}
	if JudgesBody(noSplitter) {
		t.Fatal("a refused registration must mark nothing")
	}
	if err := RegisterBodyRecognizer(mupFamily, withdrawAll); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !JudgesBody(mupFamily) || Get(mupFamily) == nil {
		t.Fatal("RegisterBodyRecognizer must install the recognizer and mark the family")
	}
	restore := SnapshotForTest()
	if JudgesBody(mupFamily) {
		t.Fatal("a snapshot must empty the mark with the recognizers")
	}
	restore()
	if !JudgesBody(mupFamily) {
		t.Fatal("restoring the snapshot must restore the mark")
	}
}

var mupFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}

// mupISD frames an IPv4 Interwork Segment Discovery NLRI (draft-ietf-bess-mup-safi
// Section 3.1.1): architecture 1, route type 1, RD 0:0, the prefix length (every octet of
// prefix counted, so the length always matches the octets), the prefix, then trailing octets.
func mupISD(prefix []byte, trailing ...byte) []byte {
	body := append(make([]byte, 8), byte(len(prefix)*8))
	body = append(append(body, prefix...), trailing...)
	return append([]byte{1, 0, 1, byte(len(body))}, body...)
}

// withdrawAll answers Withdraw for every NLRI, so the tests below see only what Retain does
// with that verdict.
func withdrawAll([]byte, bool) Verdict { return Withdraw }

// VALIDATES: in an announcement a Withdraw verdict moves the NLRI to Withdrawn, path
// identifier included, and an NLRI whose route key cannot form is discarded into Unkeyable.
// PREVENTS: a malformed route staying installed, or a withdrawal that names no route being
// relayed.
func TestRetainSplitsWithdrawnFromUnkeyable(t *testing.T) {
	keyable := mupISD([]byte{10, 0, 0})
	unkeyableISD := mupISD([]byte{10, 0, 1}, 0xff) // one octet past the prefix
	pathID := []byte{0, 0, 0, 7}
	data := append(append(append([]byte{}, pathID...), keyable...), append(append([]byte{}, pathID...), unkeyableISD...)...)

	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	r, err := Retain(withdrawAll, mupFamily, data, true, true, scratch[:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Kept) != 0 {
		t.Fatalf("nothing should survive, kept %x", r.Kept)
	}
	if want := append(append([]byte{}, pathID...), keyable...); !bytes.Equal(r.Withdrawn, want) || r.WithdrawnCount != 1 {
		t.Fatalf("withdrawn %x (%d), want %x (1)", r.Withdrawn, r.WithdrawnCount, want)
	}
	if want := append(append([]byte{}, pathID...), unkeyableISD...); !bytes.Equal(r.Unkeyable, want) || r.Discarded != 1 {
		t.Fatalf("unkeyable %x (discarded %d), want %x (1)", r.Unkeyable, r.Discarded, want)
	}
}

// VALIDATES: in a withdrawal section a Withdraw verdict keeps the NLRI, zero copy.
// PREVENTS: the withdrawal of a malformed route, which an upstream speaker relays after
// its own treat-as-withdraw, being dropped so the route stays installed downstream.
func TestRetainKeepsWithdrawVerdictInAWithdrawal(t *testing.T) {
	data := mupISD([]byte{10, 0, 1}, 0xff)
	r, err := Retain(withdrawAll, mupFamily, data, false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Removed() || &r.Kept[0] != &data[0] {
		t.Fatalf("a withdrawal section must be returned unchanged, got %+v", r)
	}
}

// VALIDATES: the per-NLRI verdict, key check included, allocates nothing.
// PREVENTS: the key check that decides withdraw against discard costing an allocation per
// malformed NLRI a peer sends. (Splitting the section allocates, as it did before.)
func TestRetainVerdictAllocatesNothing(t *testing.T) {
	one := mupISD([]byte{10, 0, 0})
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	key := nlrisplit.GetPrefixKey(mupFamily)
	if n := testing.AllocsPerRun(100, func() {
		if judge(rulings{recognize: withdrawAll}, key, one, false, scratch[:]) != Withdraw {
			t.Fatal("a keyable ISD must stay a withdrawal")
		}
	}); n != 0 {
		t.Fatalf("judge allocated %v times", n)
	}
}
