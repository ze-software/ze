package attribute

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestAS4Path_Pack(t *testing.T) {
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
			got := tt.path.Pack()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("Pack() = %v, want %v", got, tt.expected)
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
			if err != tt.wantErr {
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

	packed := original.Pack()
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

func TestAS4Aggregator_Pack(t *testing.T) {
	agg := &AS4Aggregator{
		ASN:     4200000001,
		Address: netip.MustParseAddr("10.0.0.1"),
	}

	got := agg.Pack()
	expected := []byte{
		0xfa, 0x56, 0xea, 0x01, // 4200000001
		0x0a, 0x00, 0x00, 0x01, // 10.0.0.1
	}

	if !bytes.Equal(got, expected) {
		t.Errorf("Pack() = %v, want %v", got, expected)
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

	packed := original.Pack()
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
			}

			// Flatten ASNs for comparison
			var gotASNs []uint32
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
