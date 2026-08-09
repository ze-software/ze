// Design: docs/architecture/ospf/ospf-af-unify.md -- AF-neutral LS Ack / LS Request body lengths
// and the LSA header identity projection shared by both wire codecs.

package types

import "testing"

// VALIDATES: AC-12 - LSAck.EncodedLen is the header count times the 20-octet common LSA header
// width, and an empty body encodes to zero octets.
// PREVENTS: an LS Acknowledgment codec under-sizing its buffer and truncating headers.
func TestLSAckEncodedLen(t *testing.T) {
	cases := []struct {
		count int
		want  int
	}{
		{0, 0},
		{1, LSAHeaderLen},
		{3, 3 * LSAHeaderLen},
	}
	for _, tc := range cases {
		ack := LSAck{Headers: make([]LSAHeader, tc.count)}
		if got := ack.EncodedLen(); got != tc.want {
			t.Errorf("LSAck{%d headers}.EncodedLen() = %d, want %d", tc.count, got, tc.want)
		}
	}
	// LSAHeaderLen is 20 octets in both OSPFv2 and OSPFv3.
	if LSAHeaderLen != 20 {
		t.Fatalf("LSAHeaderLen = %d, want 20", LSAHeaderLen)
	}
}

// VALIDATES: AC-12 - LSReq.EncodedLen is the request count times the 12-octet LS Request entry
// width (type + Link State ID + Advertising Router), and an empty body encodes to zero.
// PREVENTS: an LS Request codec mis-sizing entries and desynchronizing the stream.
func TestLSReqEncodedLen(t *testing.T) {
	cases := []struct {
		count int
		want  int
	}{
		{0, 0},
		{1, LSRequestEntryLen},
		{4, 4 * LSRequestEntryLen},
	}
	for _, tc := range cases {
		req := LSReq{Requests: make([]LSRequestEntry, tc.count)}
		if got := req.EncodedLen(); got != tc.want {
			t.Errorf("LSReq{%d requests}.EncodedLen() = %d, want %d", tc.count, got, tc.want)
		}
	}
	if LSRequestEntryLen != 12 {
		t.Fatalf("LSRequestEntryLen = %d, want 12", LSRequestEntryLen)
	}
}

// VALIDATES: AC-12 - LSAHeader.Key projects exactly the LSDB identity tuple (type, Link State
// ID, Advertising Router) and DROPS age, options, sequence, checksum, and length.
// PREVENTS: an LSA header whose age or sequence leaks into its LSDB key, so a refreshed
// instance would be stored as a distinct LSA instead of replacing the old one.
func TestLSAHeaderKey(t *testing.T) {
	h := LSAHeader{
		Age:               LSAge(100),
		Options:           OptionE,
		Type:              LSTypeRouter,
		LinkStateID:       LinkStateID{192, 0, 2, 1},
		AdvertisingRouter: RouterID{10, 0, 0, 9},
		Sequence:          InitialSequenceNumber,
		Checksum:          0xabcd,
		Length:            48,
	}
	want := LSAKey{
		Type:              LSTypeRouter,
		LinkStateID:       LinkStateID{192, 0, 2, 1},
		AdvertisingRouter: RouterID{10, 0, 0, 9},
	}
	if got := h.Key(); got != want {
		t.Fatalf("LSAHeader.Key() = %+v, want %+v", got, want)
	}

	// A newer instance (different age/sequence/checksum) must key identically.
	newer := h
	newer.Age = LSAge(0)
	newer.Sequence = InitialSequenceNumber.Next()
	newer.Checksum = 0x1234
	if h.Key() != newer.Key() {
		t.Fatalf("LSAHeader.Key() changed when only age/sequence/checksum differ: %+v vs %+v", h.Key(), newer.Key())
	}
}
