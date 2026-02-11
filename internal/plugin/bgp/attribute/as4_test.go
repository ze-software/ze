package attribute

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"
)

func TestAS4Path_WriteTo(t *testing.T) {
	tests := []struct {
		name     string
		path     *AS4Path
		expected []byte
	}{
		{
			name:     "empty",
			path:     &AS4Path{},
			expected: []byte{},
		},
		{
			name: "single sequence",
			path: &AS4Path{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: []uint32{65001, 65002}},
				},
			},
			expected: []byte{
				0x02, 0x02, // AS_SEQUENCE, 2 ASNs
				0x00, 0x00, 0xfd, 0xe9, // 65001
				0x00, 0x00, 0xfd, 0xea, // 65002
			},
		},
		{
			name: "large AS numbers",
			path: &AS4Path{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: []uint32{4200000001, 4200000002}},
				},
			},
			expected: []byte{
				0x02, 0x02, // AS_SEQUENCE, 2 ASNs
				0xfa, 0x56, 0xea, 0x01, // 4200000001
				0xfa, 0x56, 0xea, 0x02, // 4200000002
			},
		},
		{
			name: "sequence and set",
			path: &AS4Path{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: []uint32{65001}},
					{Type: ASSet, ASNs: []uint32{65002, 65003}},
				},
			},
			expected: []byte{
				0x02, 0x01, // AS_SEQUENCE, 1 ASN
				0x00, 0x00, 0xfd, 0xe9, // 65001
				0x01, 0x02, // AS_SET, 2 ASNs
				0x00, 0x00, 0xfd, 0xea, // 65002
				0x00, 0x00, 0xfd, 0xeb, // 65003
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 4096)
			n := tt.path.WriteTo(buf, 0)
			got := buf[:n]
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteTo() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseAS4Path(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantLen  int
		wantASNs []uint32
		wantErr  bool
	}{
		{
			name:     "empty",
			data:     []byte{},
			wantLen:  0,
			wantASNs: nil,
		},
		{
			name: "single sequence",
			data: []byte{
				0x02, 0x02,
				0x00, 0x00, 0xfd, 0xe9,
				0x00, 0x00, 0xfd, 0xea,
			},
			wantLen:  1,
			wantASNs: []uint32{65001, 65002},
		},
		{
			name: "large AS",
			data: []byte{
				0x02, 0x01,
				0xfa, 0x56, 0xea, 0x01,
			},
			wantLen:  1,
			wantASNs: []uint32{4200000001},
		},
		{
			name:    "truncated",
			data:    []byte{0x02, 0x02, 0x00, 0x00},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ParseAS4Path(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAS4Path() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(path.Segments) != tt.wantLen {
				t.Errorf("segments len = %d, want %d", len(path.Segments), tt.wantLen)
				return
			}

			if tt.wantLen > 0 && tt.wantASNs != nil {
				for i, asn := range tt.wantASNs {
					if path.Segments[0].ASNs[i] != asn {
						t.Errorf("ASN[%d] = %d, want %d", i, path.Segments[0].ASNs[i], asn)
					}
				}
			}
		})
	}
}

// TestParseAS4PathValidation verifies RFC 6793 Section 6 AS4_PATH validation.
//
// RFC 6793 Section 6: "The AS4_PATH attribute in an UPDATE message SHALL be
// considered malformed under the following conditions:"
//   - "the attribute length is not a multiple of two or is too small
//     (i.e., less than 6) for the attribute to carry at least one AS number"
//   - "the path segment length in the attribute is either zero or is
//     inconsistent with the attribute length"
//   - "the path segment type in the attribute is not one of the types
//     defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET"
//
// VALIDATES: Malformed AS4_PATH is properly rejected.
//
// PREVENTS: Processing corrupt path attributes that could affect routing.
func TestParseAS4PathValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name:    "length too small (1 byte, odd)",
			data:    []byte{0x02},
			wantErr: ErrInvalidLength, // RFC 6793: length must be multiple of 2
		},
		{
			name:    "length too small (5 bytes, odd)",
			data:    []byte{0x02, 0x01, 0x00, 0x00, 0xfd},
			wantErr: ErrInvalidLength, // RFC 6793: length must be multiple of 2
		},
		{
			name:    "truncated ASN (4 bytes even)",
			data:    []byte{0x02, 0x01, 0x00, 0x00}, // count=1 but only 2 bytes for ASN
			wantErr: ErrShortData,                   // truncated ASN data
		},
		{
			name:    "segment count zero",
			data:    []byte{0x02, 0x00}, // AS_SEQUENCE with 0 ASNs
			wantErr: ErrInvalidLength,
		},
		{
			name:    "invalid segment type",
			data:    []byte{0x05, 0x01, 0x00, 0x00, 0xfd, 0xe9}, // type 5 is invalid
			wantErr: ErrMalformedValue,
		},
		{
			name:    "odd length (7 bytes)",
			data:    []byte{0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9, 0x00},
			wantErr: ErrInvalidLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAS4Path(tt.data)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseAS4Path() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAS4Path_RoundTrip(t *testing.T) {
	original := &AS4Path{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{4200000001, 65002}},
			{Type: ASSet, ASNs: []uint32{65003}},
		},
	}

	buf := make([]byte, 4096)
	n := original.WriteTo(buf, 0)
	packed := buf[:n]
	parsed, err := ParseAS4Path(packed)
	if err != nil {
		t.Fatalf("ParseAS4Path() error = %v", err)
	}

	if len(parsed.Segments) != len(original.Segments) {
		t.Fatalf("segments len = %d, want %d", len(parsed.Segments), len(original.Segments))
	}

	for i, seg := range original.Segments {
		if parsed.Segments[i].Type != seg.Type {
			t.Errorf("segment[%d].Type = %d, want %d", i, parsed.Segments[i].Type, seg.Type)
		}
		if len(parsed.Segments[i].ASNs) != len(seg.ASNs) {
			t.Errorf("segment[%d].ASNs len = %d, want %d", i, len(parsed.Segments[i].ASNs), len(seg.ASNs))
			continue
		}
		for j, asn := range seg.ASNs {
			if parsed.Segments[i].ASNs[j] != asn {
				t.Errorf("segment[%d].ASNs[%d] = %d, want %d", i, j, parsed.Segments[i].ASNs[j], asn)
			}
		}
	}
}

func TestAS4Aggregator_WriteTo(t *testing.T) {
	agg := &AS4Aggregator{
		ASN:     4200000001,
		Address: netip.MustParseAddr("10.0.0.1"),
	}

	buf := make([]byte, 4096)
	n := agg.WriteTo(buf, 0)
	got := buf[:n]
	expected := []byte{
		0xfa, 0x56, 0xea, 0x01, // 4200000001
		0x0a, 0x00, 0x00, 0x01, // 10.0.0.1
	}

	if !bytes.Equal(got, expected) {
		t.Errorf("WriteTo() = %v, want %v", got, expected)
	}
}

func TestParseAS4Aggregator(t *testing.T) {
	data := []byte{
		0xfa, 0x56, 0xea, 0x01,
		0x0a, 0x00, 0x00, 0x01,
	}

	agg, err := ParseAS4Aggregator(data)
	if err != nil {
		t.Fatalf("ParseAS4Aggregator() error = %v", err)
	}

	if agg.ASN != 4200000001 {
		t.Errorf("ASN = %d, want 4200000001", agg.ASN)
	}
	if agg.Address.String() != "10.0.0.1" {
		t.Errorf("Address = %s, want 10.0.0.1", agg.Address)
	}
}

func TestAS4Aggregator_RoundTrip(t *testing.T) {
	original := &AS4Aggregator{
		ASN:     4200000001,
		Address: netip.MustParseAddr("192.168.1.1"),
	}

	buf := make([]byte, 4096)
	n := original.WriteTo(buf, 0)
	packed := buf[:n]
	parsed, err := ParseAS4Aggregator(packed)
	if err != nil {
		t.Fatalf("ParseAS4Aggregator() error = %v", err)
	}

	if parsed.ASN != original.ASN {
		t.Errorf("ASN = %d, want %d", parsed.ASN, original.ASN)
	}
	if parsed.Address != original.Address {
		t.Errorf("Address = %s, want %s", parsed.Address, original.Address)
	}
}

func TestMergeAS4Path(t *testing.T) {
	tests := []struct {
		name     string
		asPath   *ASPath
		as4Path  *AS4Path
		wantASNs []uint32
	}{
		{
			name:     "nil as4path",
			asPath:   &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}},
			as4Path:  nil,
			wantASNs: []uint32{65001},
		},
		{
			name:     "nil aspath",
			asPath:   nil,
			as4Path:  &AS4Path{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{4200000001}}}},
			wantASNs: []uint32{4200000001},
		},
		{
			name: "equal length - use as4path",
			asPath: &ASPath{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{ASTrans, ASTrans}},
			}},
			as4Path: &AS4Path{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{4200000001, 4200000002}},
			}},
			wantASNs: []uint32{4200000001, 4200000002},
		},
		{
			name: "aspath longer - prepend from aspath",
			asPath: &ASPath{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{65001, ASTrans, ASTrans}},
			}},
			as4Path: &AS4Path{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{4200000001, 4200000002}},
			}},
			wantASNs: []uint32{65001, 4200000001, 4200000002},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeAS4Path(tt.asPath, tt.as4Path)
			if merged == nil {
				t.Fatal("MergeAS4Path() returned nil")
				return
			}

			// Flatten ASNs for comparison
			totalASNs := 0
			for _, seg := range merged.Segments {
				totalASNs += len(seg.ASNs)
			}
			gotASNs := make([]uint32, 0, totalASNs)
			for _, seg := range merged.Segments {
				gotASNs = append(gotASNs, seg.ASNs...)
			}

			if len(gotASNs) != len(tt.wantASNs) {
				t.Errorf("merged ASNs len = %d, want %d", len(gotASNs), len(tt.wantASNs))
				return
			}

			for i, asn := range tt.wantASNs {
				if gotASNs[i] != asn {
					t.Errorf("ASN[%d] = %d, want %d", i, gotASNs[i], asn)
				}
			}
		})
	}
}

// TestMergeAS4PathWithASSet verifies RFC 4271 Section 9.1.2.2 path counting.
//
// RFC 6793 Section 4.2.3: "it is necessary to first calculate the number
// of AS numbers in the AS_PATH and AS4_PATH attributes using the method
// specified in Section 9.1.2.2 of [RFC4271]"
//
// RFC 4271 Section 9.1.2.2: "an AS_SET counts as 1, no matter how many
// ASes are in the set"
//
// VALIDATES: AS_SET is counted as 1 during merge, not as number of ASNs.
//
// PREVENTS: Incorrect path length causing wrong merge behavior.
func TestMergeAS4PathWithASSet(t *testing.T) {
	// AS_PATH: [SEQ: 65001] [SET: 65002, 65003, 65004]
	// Path length should be: 1 (seq) + 1 (set=1) = 2
	asPath := &ASPath{Segments: []ASPathSegment{
		{Type: ASSequence, ASNs: []uint32{65001}},
		{Type: ASSet, ASNs: []uint32{65002, 65003, 65004}},
	}}

	// AS4_PATH: [SEQ: 4200000001, 4200000002]
	// Path length should be: 2
	as4Path := &AS4Path{Segments: []ASPathSegment{
		{Type: ASSequence, ASNs: []uint32{4200000001, 4200000002}},
	}}

	// Both paths have length 2, so merged should be all of AS4_PATH
	merged := MergeAS4Path(asPath, as4Path)

	// Flatten result
	totalASNs := 0
	for _, seg := range merged.Segments {
		totalASNs += len(seg.ASNs)
	}
	gotASNs := make([]uint32, 0, totalASNs)
	for _, seg := range merged.Segments {
		gotASNs = append(gotASNs, seg.ASNs...)
	}

	// Should be exactly the AS4_PATH since lengths are equal
	wantASNs := []uint32{4200000001, 4200000002}

	if len(gotASNs) != len(wantASNs) {
		t.Errorf("merged ASNs len = %d, want %d (got: %v)", len(gotASNs), len(wantASNs), gotASNs)
		return
	}

	for i, asn := range wantASNs {
		if gotASNs[i] != asn {
			t.Errorf("ASN[%d] = %d, want %d", i, gotASNs[i], asn)
		}
	}
}

// TestMergeAS4PathWithConfed verifies confederation segments are not counted.
//
// RFC 5065: Confederation segments (AS_CONFED_SEQUENCE, AS_CONFED_SET) are
// not counted in path length calculation.
//
// RFC 6793 Section 4.2.3: uses "the method specified in Section 9.1.2.2
// of [RFC4271] and in [RFC5065]"
//
// VALIDATES: Confed segments don't affect merge path length calculation.
//
// PREVENTS: Confed segments incorrectly inflating path length.
func TestMergeAS4PathWithConfed(t *testing.T) {
	// AS_PATH: [CONFED_SEQ: 64512, 64513] [SEQ: 65001]
	// Path length should be: 0 (confed) + 1 (seq) = 1
	asPath := &ASPath{Segments: []ASPathSegment{
		{Type: ASConfedSequence, ASNs: []uint32{64512, 64513}},
		{Type: ASSequence, ASNs: []uint32{65001}},
	}}

	// AS4_PATH: [SEQ: 4200000001]
	// Path length should be: 1
	as4Path := &AS4Path{Segments: []ASPathSegment{
		{Type: ASSequence, ASNs: []uint32{4200000001}},
	}}

	// Both paths have length 1, so merged should be all of AS4_PATH
	merged := MergeAS4Path(asPath, as4Path)

	// Flatten result
	totalASNs := 0
	for _, seg := range merged.Segments {
		totalASNs += len(seg.ASNs)
	}
	gotASNs := make([]uint32, 0, totalASNs)
	for _, seg := range merged.Segments {
		gotASNs = append(gotASNs, seg.ASNs...)
	}

	// Should be exactly the AS4_PATH since lengths are equal
	wantASNs := []uint32{4200000001}

	if len(gotASNs) != len(wantASNs) {
		t.Errorf("merged ASNs len = %d, want %d (got: %v)", len(gotASNs), len(wantASNs), gotASNs)
		return
	}

	for i, asn := range wantASNs {
		if gotASNs[i] != asn {
			t.Errorf("ASN[%d] = %d, want %d", i, gotASNs[i], asn)
		}
	}
}

// TestAS4PathPackExcludesConfed verifies RFC 6793 Section 3 confed handling.
//
// RFC 6793 Section 3:
//
//	"To prevent the possible propagation of Confederation-related path
//	 segments outside of a Confederation, the path segment types
//	 AS_CONFED_SEQUENCE and AS_CONFED_SET are declared invalid for the
//	 AS4_PATH attribute and MUST NOT be included in the AS4_PATH attribute
//	 of an UPDATE message."
//
// VALIDATES: WriteTo excludes confed segments from output.
//
// PREVENTS: Leaking confederation segments in AS4_PATH to peers.
func TestAS4PathWriteToExcludesConfed(t *testing.T) {
	// AS4_PATH with confed segments that should be filtered
	path := &AS4Path{
		Segments: []ASPathSegment{
			{Type: ASConfedSequence, ASNs: []uint32{64512, 64513}}, // Should be excluded
			{Type: ASSequence, ASNs: []uint32{65001, 65002}},
			{Type: ASConfedSet, ASNs: []uint32{64514}}, // Should be excluded
			{Type: ASSet, ASNs: []uint32{65003}},
		},
	}

	buf := make([]byte, 4096)
	n := path.WriteTo(buf, 0)
	packed := buf[:n]

	// Parse back to verify confed segments are gone
	parsed, err := ParseAS4Path(packed)
	if err != nil {
		t.Fatalf("ParseAS4Path() error = %v", err)
	}

	// Should only have 2 segments (SEQ and SET), not 4
	if len(parsed.Segments) != 2 {
		t.Errorf("expected 2 segments after filtering confed, got %d", len(parsed.Segments))
		for i, seg := range parsed.Segments {
			t.Logf("segment[%d]: type=%d, ASNs=%v", i, seg.Type, seg.ASNs)
		}
		return
	}

	// First should be AS_SEQUENCE
	if parsed.Segments[0].Type != ASSequence {
		t.Errorf("segment[0].Type = %d, want AS_SEQUENCE (%d)", parsed.Segments[0].Type, ASSequence)
	}
	if len(parsed.Segments[0].ASNs) != 2 {
		t.Errorf("segment[0].ASNs len = %d, want 2", len(parsed.Segments[0].ASNs))
	}

	// Second should be AS_SET
	if parsed.Segments[1].Type != ASSet {
		t.Errorf("segment[1].Type = %d, want AS_SET (%d)", parsed.Segments[1].Type, ASSet)
	}
}

// TestAS4PathFilterConfedSegments verifies the helper method.
//
// RFC 6793 Section 4.2.2:
//
//	"Whenever the AS path information contains the AS_CONFED_SEQUENCE or
//	 AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such
//	 path segments from the AS4_PATH attribute being constructed."
//
// VALIDATES: FilterConfedSegments removes all confed segments.
//
// PREVENTS: Confed segments in AS4_PATH to OLD speakers.
func TestAS4PathFilterConfedSegments(t *testing.T) {
	path := &AS4Path{
		Segments: []ASPathSegment{
			{Type: ASConfedSequence, ASNs: []uint32{64512}},
			{Type: ASSequence, ASNs: []uint32{65001}},
			{Type: ASConfedSet, ASNs: []uint32{64513, 64514}},
		},
	}

	filtered := path.FilterConfedSegments()

	if len(filtered.Segments) != 1 {
		t.Errorf("expected 1 segment after filter, got %d", len(filtered.Segments))
		return
	}

	if filtered.Segments[0].Type != ASSequence {
		t.Errorf("segment type = %d, want AS_SEQUENCE", filtered.Segments[0].Type)
	}

	if len(filtered.Segments[0].ASNs) != 1 || filtered.Segments[0].ASNs[0] != 65001 {
		t.Errorf("segment ASNs = %v, want [65001]", filtered.Segments[0].ASNs)
	}
}

// TestAS4PathParseAcceptsConfed verifies RFC 6793 Section 6 validation.
//
// RFC 6793 Section 6: "the path segment type in the attribute is not one
// of the types defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and
// AS_CONFED_SET" (listed as condition for malformed)
//
// This means confed types are VALID during parsing (not malformed).
// They should be accepted and can be filtered later.
//
// VALIDATES: Parse accepts confed segments without error.
//
// PREVENTS: Incorrectly rejecting AS4_PATH with confed segments.
func TestAS4PathParseAcceptsConfed(t *testing.T) {
	// AS4_PATH with AS_CONFED_SEQUENCE (type 3 per RFC 5065)
	data := []byte{
		0x03, 0x02, // AS_CONFED_SEQUENCE (3), 2 ASNs
		0x00, 0x00, 0xFC, 0x00, // 64512
		0x00, 0x00, 0xFC, 0x01, // 64513
	}

	path, err := ParseAS4Path(data)
	if err != nil {
		t.Fatalf("ParseAS4Path() should accept confed, got error = %v", err)
	}

	if len(path.Segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(path.Segments))
		return
	}

	if path.Segments[0].Type != ASConfedSequence {
		t.Errorf("segment type = %d, want AS_CONFED_SEQUENCE (%d)", path.Segments[0].Type, ASConfedSequence)
	}
}

func TestAS4Path_PathLength(t *testing.T) {
	tests := []struct {
		name string
		path *AS4Path
		want int
	}{
		{
			name: "empty",
			path: &AS4Path{},
			want: 0,
		},
		{
			name: "sequence of 3",
			path: &AS4Path{Segments: []ASPathSegment{
				{Type: ASSequence, ASNs: []uint32{1, 2, 3}},
			}},
			want: 3,
		},
		{
			name: "set counts as 1",
			path: &AS4Path{Segments: []ASPathSegment{
				{Type: ASSet, ASNs: []uint32{1, 2, 3, 4, 5}},
			}},
			want: 1,
		},
		{
			name: "confed not counted",
			path: &AS4Path{Segments: []ASPathSegment{
				{Type: ASConfedSequence, ASNs: []uint32{1, 2}},
				{Type: ASSequence, ASNs: []uint32{3}},
			}},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.path.PathLength(); got != tt.want {
				t.Errorf("PathLength() = %d, want %d", got, tt.want)
			}
		})
	}
}
