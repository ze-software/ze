package main

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers: build wire-format test data
// ---------------------------------------------------------------------------

// makeBGP4MPRecord builds a BGP4MP record containing a BGP UPDATE message.
// Layout: peerAS(asSize) + localAS(asSize) + ifIndex(2) + AFI(2) + peerIP(ipSize) + localIP(ipSize) + marker(16) + length(2) + type(1) + body.
func makeBGP4MPRecord(asSize int, peerASN uint32, afi uint16, body []byte) []byte {
	ipSize := 4
	if afi == 2 {
		ipSize = 16
	}

	msgLen := 19 + len(body) // marker(16) + length(2) + type(1) + body
	totalLen := asSize*2 + 2 + 2 + ipSize*2 + 16 + 2 + 1 + len(body)
	buf := make([]byte, totalLen)
	off := 0

	// Peer AS.
	if asSize == 4 {
		binary.BigEndian.PutUint32(buf[off:], peerASN)
	} else {
		binary.BigEndian.PutUint16(buf[off:], uint16(peerASN))
	}
	off += asSize

	// Local AS (don't care, use 0).
	off += asSize

	// Interface index (2 bytes).
	off += 2

	// AFI (2 bytes).
	binary.BigEndian.PutUint16(buf[off:], afi)
	off += 2

	// Peer IP + Local IP (zeroed).
	off += ipSize * 2

	// BGP marker: 16 bytes of 0xFF.
	for i := range 16 {
		buf[off+i] = 0xFF
	}
	off += 16

	// BGP message length.
	binary.BigEndian.PutUint16(buf[off:], uint16(msgLen))
	off += 2

	// BGP message type: 2 = UPDATE.
	buf[off] = 2
	off++

	// Body.
	copy(buf[off:], body)

	return buf
}

// makeAttr builds a single BGP path attribute (flags + typeCode + length + value).
func makeAttr(flags, typeCode uint8, value []byte) []byte {
	if flags&0x10 != 0 { // extended length
		buf := make([]byte, 4+len(value))
		buf[0] = flags
		buf[1] = typeCode
		binary.BigEndian.PutUint16(buf[2:4], uint16(len(value)))
		copy(buf[4:], value)
		return buf
	}
	buf := make([]byte, 3+len(value))
	buf[0] = flags
	buf[1] = typeCode
	buf[2] = byte(len(value))
	copy(buf[3:], value)
	return buf
}

// makeMPReachNLRI builds an MP_REACH_NLRI attribute value.
// Format: AFI(2) + SAFI(1) + NHLen(1) + NH(var) + reserved(1) + NLRI(rest).
func makeMPReachNLRI(afi uint16, safi uint8, nhLen int, nlri []byte) []byte {
	buf := make([]byte, 4+nhLen+1+len(nlri))
	binary.BigEndian.PutUint16(buf[0:2], afi)
	buf[2] = safi
	buf[3] = byte(nhLen)
	// NH bytes left zeroed.
	// reserved byte at 4+nhLen left zeroed.
	copy(buf[4+nhLen+1:], nlri)
	return buf
}

// makeMPUnreachNLRI builds an MP_UNREACH_NLRI attribute value.
// Format: AFI(2) + SAFI(1) + withdrawn(rest).
func makeMPUnreachNLRI(afi uint16, safi uint8, withdrawn []byte) []byte {
	buf := make([]byte, 3+len(withdrawn))
	binary.BigEndian.PutUint16(buf[0:2], afi)
	buf[2] = safi
	copy(buf[3:], withdrawn)
	return buf
}

// packedPrefix builds a single packed prefix entry: prefixLen(1) + ceil(prefixLen/8) bytes.
func packedPrefix(prefixLen int) []byte {
	byteCount := (prefixLen + 7) / 8
	buf := make([]byte, 1+byteCount)
	buf[0] = byte(prefixLen)
	// prefix bytes left zeroed.
	return buf
}

// concatBytes concatenates multiple byte slices.
func concatBytes(parts ...[]byte) []byte {
	var total int
	for _, p := range parts {
		total += len(p)
	}
	buf := make([]byte, 0, total)
	for _, p := range parts {
		buf = append(buf, p...)
	}
	return buf
}

// ---------------------------------------------------------------------------
// mrt.go tests
// ---------------------------------------------------------------------------

func TestExtractBGP4MPUpdate(t *testing.T) {
	// VALIDATES: extractBGP4MPUpdate correctly parses BGP4MP records across all subtypes.
	// PREVENTS: incorrect ASN extraction or body slicing for 2-byte vs 4-byte AS formats.

	updateBody := buildUpdate(nil, nil, nil) // minimal: 2+0+2+0 = 4 bytes

	tests := []struct {
		name       string
		subtype    uint16
		data       []byte
		wantBody   bool
		wantPeerAS uint32
	}{
		{
			name:       "AS2 IPv4 UPDATE",
			subtype:    subtypeBGP4MPMessage,
			data:       makeBGP4MPRecord(2, 65001, 1, updateBody),
			wantBody:   true,
			wantPeerAS: 65001,
		},
		{
			name:       "AS2 Local IPv4 UPDATE",
			subtype:    subtypeBGP4MPMessageLocal,
			data:       makeBGP4MPRecord(2, 65002, 1, updateBody),
			wantBody:   true,
			wantPeerAS: 65002,
		},
		{
			name:       "AS4 IPv4 UPDATE",
			subtype:    subtypeBGP4MPMessageAS4,
			data:       makeBGP4MPRecord(4, 400000, 1, updateBody),
			wantBody:   true,
			wantPeerAS: 400000,
		},
		{
			name:       "AS4 Local IPv4 UPDATE",
			subtype:    subtypeBGP4MPMessageAS4Local,
			data:       makeBGP4MPRecord(4, 400001, 1, updateBody),
			wantBody:   true,
			wantPeerAS: 400001,
		},
		{
			name:       "AS4 IPv6 UPDATE",
			subtype:    subtypeBGP4MPMessageAS4,
			data:       makeBGP4MPRecord(4, 300000, 2, updateBody),
			wantBody:   true,
			wantPeerAS: 300000,
		},
		{
			name:       "AS2 IPv6 UPDATE",
			subtype:    subtypeBGP4MPMessage,
			data:       makeBGP4MPRecord(2, 65100, 2, updateBody),
			wantBody:   true,
			wantPeerAS: 65100,
		},
		{
			name:     "unknown subtype returns nil",
			subtype:  99,
			data:     makeBGP4MPRecord(2, 1, 1, updateBody),
			wantBody: false,
		},
		{
			name:     "too-short data returns nil",
			subtype:  subtypeBGP4MPMessage,
			data:     []byte{0, 1, 2},
			wantBody: false,
		},
		{
			name:     "empty data returns nil",
			subtype:  subtypeBGP4MPMessage,
			data:     nil,
			wantBody: false,
		},
		{
			name:    "non-UPDATE message type returns nil",
			subtype: subtypeBGP4MPMessage,
			data: func() []byte {
				rec := makeBGP4MPRecord(2, 65001, 1, updateBody)
				// Find the message type byte and change it to KEEPALIVE (4).
				// In AS2/AFI1: asSize*2 + 4 + ipSize*2 + 16 + 2 = offset of msgType.
				off := 2*2 + 4 + 4*2 + 16 + 2
				rec[off] = 4
				return rec
			}(),
			wantBody: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, peerASN := extractBGP4MPUpdate(tt.subtype, tt.data)
			if tt.wantBody {
				require.NotNil(t, body, "expected non-nil body")
				assert.Equal(t, tt.wantPeerAS, peerASN)
			} else {
				assert.Nil(t, body)
			}
		})
	}
}

func TestExtractUpdateAttrs(t *testing.T) {
	// VALIDATES: extractUpdateAttrs correctly extracts path attributes from UPDATE body.
	// PREVENTS: off-by-one in withdrawn/attr length parsing.

	tests := []struct {
		name    string
		update  []byte
		wantLen int // -1 means nil
	}{
		{
			name:    "empty attrs in minimal UPDATE",
			update:  buildUpdate(nil, nil, nil),
			wantLen: 0,
		},
		{
			name:    "attrs with content",
			update:  buildUpdate(nil, []byte{0x40, 0x01, 0x01, 0x00}, nil), // ORIGIN IGP
			wantLen: 4,
		},
		{
			name: "attrs after withdrawn routes",
			update: buildUpdate(
				concatBytes(packedPrefix(24), packedPrefix(16)), // 2 withdrawn
				[]byte{0x40, 0x01, 0x01, 0x02},                  // ORIGIN INCOMPLETE
				nil,
			),
			wantLen: 4,
		},
		{
			name:    "too-short input returns nil",
			update:  []byte{0, 0},
			wantLen: -1,
		},
		{
			name:    "nil input returns nil",
			update:  nil,
			wantLen: -1,
		},
		{
			name:    "3 bytes too short",
			update:  []byte{0, 0, 0},
			wantLen: -1,
		},
		{
			name: "truncated attr length returns nil",
			update: func() []byte {
				// wdLen=0, then only 1 byte for attrLen (need 2).
				return []byte{0, 0, 0}
			}(),
			wantLen: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUpdateAttrs(tt.update)
			if tt.wantLen == -1 {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestIterateAttrs(t *testing.T) {
	// VALIDATES: iterateAttrs walks packed attributes correctly, including extended length.
	// PREVENTS: mishandling of extended-length flag (0x10) causing incorrect offset calculation.

	tests := []struct {
		name      string
		attrs     []byte
		wantTypes []uint8
		wantLens  []int
	}{
		{
			name:      "empty attrs",
			attrs:     nil,
			wantTypes: nil,
			wantLens:  nil,
		},
		{
			name:      "single attr normal length",
			attrs:     makeAttr(0x40, attrOrigin, []byte{0x00}),
			wantTypes: []uint8{attrOrigin},
			wantLens:  []int{1},
		},
		{
			name: "two attrs concatenated",
			attrs: concatBytes(
				makeAttr(0x40, attrOrigin, []byte{0x00}),
				makeAttr(0x40, attrLocalPref, []byte{0, 0, 0, 100}),
			),
			wantTypes: []uint8{attrOrigin, attrLocalPref},
			wantLens:  []int{1, 4},
		},
		{
			name:      "extended length attr",
			attrs:     makeAttr(0x50, attrASPath, make([]byte, 300)),
			wantTypes: []uint8{attrASPath},
			wantLens:  []int{300},
		},
		{
			name:      "truncated attr header stops iteration",
			attrs:     []byte{0x40}, // only 1 byte, need 2 for flags+type
			wantTypes: nil,
			wantLens:  nil,
		},
		{
			name: "mixed normal and extended",
			attrs: concatBytes(
				makeAttr(0x40, attrOrigin, []byte{0x02}),
				makeAttr(0x50, attrCommunity, make([]byte, 256)),
				makeAttr(0x40, attrNextHop, []byte{10, 0, 0, 1}),
			),
			wantTypes: []uint8{attrOrigin, attrCommunity, attrNextHop},
			wantLens:  []int{1, 256, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotTypes []uint8
			var gotLens []int
			iterateAttrs(tt.attrs, func(_, typeCode uint8, value []byte) {
				gotTypes = append(gotTypes, typeCode)
				gotLens = append(gotLens, len(value))
			})
			assert.Equal(t, tt.wantTypes, gotTypes)
			assert.Equal(t, tt.wantLens, gotLens)
		})
	}
}

func TestCountAttrs(t *testing.T) {
	// VALIDATES: countAttrs returns correct count for various attribute sections.
	// PREVENTS: counting errors from iterateAttrs edge cases.

	tests := []struct {
		name  string
		attrs []byte
		want  int
	}{
		{"empty", nil, 0},
		{"one attr", makeAttr(0x40, attrOrigin, []byte{0}), 1},
		{
			"three attrs",
			concatBytes(
				makeAttr(0x40, attrOrigin, []byte{0}),
				makeAttr(0x40, attrNextHop, []byte{10, 0, 0, 1}),
				makeAttr(0x40, attrLocalPref, []byte{0, 0, 0, 100}),
			),
			3,
		},
		{"truncated", []byte{0x40}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, countAttrs(tt.attrs))
		})
	}
}

func TestCountPackedPrefixes(t *testing.T) {
	// VALIDATES: countPackedPrefixes correctly counts NLRI prefix entries.
	// PREVENTS: miscounting due to incorrect ceil(prefixLen/8) byte consumption.

	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"empty", nil, 0},
		{"single /24", packedPrefix(24), 1},
		{"single /32", packedPrefix(32), 1},
		{"single /0 (default route)", packedPrefix(0), 1},
		{"single /1", packedPrefix(1), 1},
		{
			"three prefixes",
			concatBytes(packedPrefix(24), packedPrefix(16), packedPrefix(8)),
			3,
		},
		{
			"/25 consumes 4 bytes total",
			packedPrefix(25),
			1,
		},
		{
			"truncated prefix data stops count",
			[]byte{24}, // claims /24 but no prefix bytes follow
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, countPackedPrefixes(tt.data))
		})
	}
}

func TestBuildUpdate(t *testing.T) {
	// VALIDATES: buildUpdate constructs valid UPDATE bodies that extractUpdateAttrs can parse.
	// PREVENTS: incorrect length fields or misaligned sections in constructed UPDATEs.

	tests := []struct {
		name      string
		withdrawn []byte
		attrs     []byte
		nlri      []byte
	}{
		{"all nil", nil, nil, nil},
		{"withdrawn only", concatBytes(packedPrefix(24)), nil, nil},
		{"attrs only", nil, makeAttr(0x40, attrOrigin, []byte{0}), nil},
		{"nlri only", nil, nil, concatBytes(packedPrefix(24))},
		{
			"all present",
			concatBytes(packedPrefix(16)),
			concatBytes(
				makeAttr(0x40, attrOrigin, []byte{0}),
				makeAttr(0x40, attrNextHop, []byte{10, 0, 0, 1}),
			),
			concatBytes(packedPrefix(24), packedPrefix(32)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := buildUpdate(tt.withdrawn, tt.attrs, tt.nlri)
			require.NotNil(t, update)

			// Verify withdrawn length field.
			wdLen := binary.BigEndian.Uint16(update[0:2])
			assert.Equal(t, len(tt.withdrawn), int(wdLen))

			// Verify attr length field.
			attrOff := 2 + int(wdLen)
			attrLen := binary.BigEndian.Uint16(update[attrOff : attrOff+2])
			assert.Equal(t, len(tt.attrs), int(attrLen))

			// Verify total length.
			expectedLen := 2 + len(tt.withdrawn) + 2 + len(tt.attrs) + len(tt.nlri)
			assert.Equal(t, expectedLen, len(update))

			// Round-trip: extractUpdateAttrs should return the attrs section.
			gotAttrs := extractUpdateAttrs(update)
			if len(tt.attrs) == 0 {
				assert.Len(t, gotAttrs, 0)
			} else {
				assert.Equal(t, tt.attrs, gotAttrs)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	// VALIDATES: formatBytes produces correct human-readable byte count strings.
	// PREVENTS: wrong unit boundaries (1024 vs 1000).

	tests := []struct {
		name string
		b    uint64
		want string
	}{
		{"zero", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"1023 bytes", 1023, "1023 B"},
		{"exactly 1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"exactly 1 MB", 1024 * 1024, "1.0 MB"},
		{"exactly 1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"2.5 GB", 2.5 * 1024 * 1024 * 1024, "2.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatBytes(tt.b))
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	// VALIDATES: isAllDigits correctly identifies digit-only strings.
	// PREVENTS: false positives on empty string or strings with non-digit characters.

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"single digit", "0", true},
		{"multiple digits", "12345", true},
		{"large number", "9999999999", true},
		{"empty string", "", false},
		{"contains letter", "123a", false},
		{"only letters", "abc", false},
		{"space", "1 2", false},
		{"negative sign", "-1", false},
		{"decimal point", "1.5", false},
		{"leading zero", "007", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isAllDigits(tt.s))
		})
	}
}

func TestFormatNumber(t *testing.T) {
	// VALIDATES: formatNumber inserts commas at correct thousand-separator positions.
	// PREVENTS: off-by-one in comma placement.

	tests := []struct {
		name string
		n    uint64
		want string
	}{
		{"zero", 0, "0"},
		{"single digit", 9, "9"},
		{"three digits", 999, "999"},
		{"four digits", 1000, "1,000"},
		{"six digits", 999999, "999,999"},
		{"seven digits", 1000000, "1,000,000"},
		{"large number", 1234567890, "1,234,567,890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatNumber(tt.n))
		})
	}
}

// ---------------------------------------------------------------------------
// density.go tests
// ---------------------------------------------------------------------------

func TestCountUpdateNLRIs(t *testing.T) {
	// VALIDATES: countUpdateNLRIs correctly counts announced and withdrawn NLRIs
	//   from all four locations: withdrawn field, trailing NLRI, MP_REACH, MP_UNREACH.
	// PREVENTS: missing counts from any of the four NLRI locations.

	tests := []struct {
		name          string
		body          []byte
		wantAnnounced int
		wantWithdrawn int
	}{
		{
			name:          "empty UPDATE",
			body:          buildUpdate(nil, nil, nil),
			wantAnnounced: 0,
			wantWithdrawn: 0,
		},
		{
			name: "trailing NLRI only (2 announced)",
			body: buildUpdate(nil, nil, concatBytes(
				packedPrefix(24), packedPrefix(16),
			)),
			wantAnnounced: 2,
			wantWithdrawn: 0,
		},
		{
			name: "withdrawn only (3 withdrawn)",
			body: buildUpdate(
				concatBytes(packedPrefix(24), packedPrefix(16), packedPrefix(8)),
				nil, nil,
			),
			wantAnnounced: 0,
			wantWithdrawn: 3,
		},
		{
			name: "MP_REACH_NLRI in attrs (1 announced)",
			body: buildUpdate(nil,
				makeAttr(0x80|0x40, attrMPReachNLRI,
					makeMPReachNLRI(2, 1, 16, packedPrefix(48))),
				nil,
			),
			wantAnnounced: 1,
			wantWithdrawn: 0,
		},
		{
			name: "MP_UNREACH_NLRI in attrs (2 withdrawn)",
			body: buildUpdate(nil,
				makeAttr(0x80|0x40, attrMPUnreachNLRI,
					makeMPUnreachNLRI(2, 1, concatBytes(packedPrefix(48), packedPrefix(64)))),
				nil,
			),
			wantAnnounced: 0,
			wantWithdrawn: 2,
		},
		{
			name: "all four locations",
			body: buildUpdate(
				concatBytes(packedPrefix(24)), // 1 withdrawn (IPv4)
				concatBytes(
					makeAttr(0x80|0x40, attrMPReachNLRI,
						makeMPReachNLRI(2, 1, 16, concatBytes(packedPrefix(48), packedPrefix(64)))), // 2 MP_REACH
					makeAttr(0x80|0x40, attrMPUnreachNLRI,
						makeMPUnreachNLRI(2, 1, packedPrefix(48))), // 1 MP_UNREACH
				),
				concatBytes(packedPrefix(16)), // 1 trailing NLRI
			),
			wantAnnounced: 3, // 2 MP_REACH + 1 trailing
			wantWithdrawn: 2, // 1 IPv4 + 1 MP_UNREACH
		},
		{
			name:          "too short body",
			body:          []byte{0, 0},
			wantAnnounced: 0,
			wantWithdrawn: 0,
		},
		{
			name:          "nil body",
			body:          nil,
			wantAnnounced: 0,
			wantWithdrawn: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ann, wd := countUpdateNLRIs(tt.body)
			assert.Equal(t, tt.wantAnnounced, ann, "announced")
			assert.Equal(t, tt.wantWithdrawn, wd, "withdrawn")
		})
	}
}

func TestCountMPReachNLRI(t *testing.T) {
	// VALIDATES: countMPReachNLRI correctly parses MP_REACH_NLRI attribute values.
	// PREVENTS: incorrect offset calculation when next-hop length varies.

	tests := []struct {
		name string
		val  []byte
		want int
	}{
		{
			name: "single IPv6 prefix with 16-byte NH",
			val:  makeMPReachNLRI(2, 1, 16, packedPrefix(48)),
			want: 1,
		},
		{
			name: "two prefixes with 4-byte NH and multicast SAFI",
			val:  makeMPReachNLRI(1, 2, 4, concatBytes(packedPrefix(24), packedPrefix(16))),
			want: 2,
		},
		{
			name: "no NLRI after header",
			val:  makeMPReachNLRI(2, 1, 16, nil),
			want: 0,
		},
		{
			name: "too short",
			val:  []byte{0, 2, 1},
			want: 0,
		},
		{
			name: "nil",
			val:  nil,
			want: 0,
		},
		{
			name: "32-byte NH (IPv6 link-local)",
			val:  makeMPReachNLRI(2, 1, 32, concatBytes(packedPrefix(128), packedPrefix(64), packedPrefix(48))),
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, countMPReachNLRI(tt.val))
		})
	}
}

func TestCountMPUnreachNLRI(t *testing.T) {
	// VALIDATES: countMPUnreachNLRI correctly parses MP_UNREACH_NLRI attribute values.
	// PREVENTS: wrong offset (must skip 3-byte AFI+SAFI header, no NH).

	tests := []struct {
		name string
		val  []byte
		want int
	}{
		{
			name: "single withdrawn prefix",
			val:  makeMPUnreachNLRI(2, 1, packedPrefix(64)),
			want: 1,
		},
		{
			name: "three withdrawn prefixes IPv4",
			val:  makeMPUnreachNLRI(1, 1, concatBytes(packedPrefix(24), packedPrefix(16), packedPrefix(8))),
			want: 3,
		},
		{
			name: "no withdrawn after header multicast",
			val:  makeMPUnreachNLRI(2, 2, nil),
			want: 0,
		},
		{
			name: "too short",
			val:  []byte{0, 2},
			want: 0,
		},
		{
			name: "nil",
			val:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, countMPUnreachNLRI(tt.val))
		})
	}
}

func TestSortedIntKeys(t *testing.T) {
	// VALIDATES: sortedIntKeys returns map keys in ascending order.
	// PREVENTS: unsorted output causing incorrect percentile calculations.

	tests := []struct {
		name string
		m    map[int]int
		want []int
	}{
		{"empty map", map[int]int{}, nil},
		{"single key", map[int]int{5: 1}, []int{5}},
		{"already sorted", map[int]int{1: 1, 2: 1, 3: 1}, []int{1, 2, 3}},
		{"reverse order input", map[int]int{9: 1, 1: 1, 5: 1}, []int{1, 5, 9}},
		{"negative keys", map[int]int{-1: 1, 0: 1, 1: 1}, []int{-1, 0, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedIntKeys(tt.m)
			if tt.want == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDensityMaxKey(t *testing.T) {
	// VALIDATES: densityMaxKey returns the largest key in the map.
	// PREVENTS: returning value instead of key, or wrong comparison.

	tests := []struct {
		name string
		m    map[int]int
		want int
	}{
		{"empty map", map[int]int{}, 0},
		{"single key", map[int]int{42: 1}, 42},
		{"multiple keys", map[int]int{1: 100, 50: 1, 25: 50}, 50},
		{"all same value", map[int]int{7: 3, 8: 3}, 8},
		{"zero is max", map[int]int{0: 1}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, densityMaxKey(tt.m))
		})
	}
}

func TestDensityPercentile(t *testing.T) {
	// VALIDATES: densityPercentile returns correct value at given percentile.
	// PREVENTS: off-by-one in cumulative counting or ceiling calculation.

	tests := []struct {
		name  string
		dist  map[int]int
		total int
		pct   int
		want  int
	}{
		{
			name:  "P50 of uniform distribution",
			dist:  map[int]int{1: 50, 2: 50},
			total: 100,
			pct:   50,
			want:  1,
		},
		{
			name:  "P100",
			dist:  map[int]int{1: 50, 2: 50},
			total: 100,
			pct:   100,
			want:  2,
		},
		{
			name:  "P99 with skewed distribution",
			dist:  map[int]int{1: 99, 100: 1},
			total: 100,
			pct:   99,
			want:  1,
		},
		{
			name:  "P1 (first percentile)",
			dist:  map[int]int{1: 1, 100: 99},
			total: 100,
			pct:   1,
			want:  1,
		},
		{
			name:  "single bucket",
			dist:  map[int]int{42: 10},
			total: 10,
			pct:   50,
			want:  42,
		},
		{
			name:  "empty map",
			dist:  map[int]int{},
			total: 0,
			pct:   50,
			want:  0,
		},
		{
			name:  "P95 three buckets",
			dist:  map[int]int{1: 80, 5: 15, 20: 5},
			total: 100,
			pct:   95,
			want:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, densityPercentile(tt.dist, tt.total, tt.pct))
		})
	}
}

// ---------------------------------------------------------------------------
// attributes.go tests
// ---------------------------------------------------------------------------

func TestRunBucket(t *testing.T) {
	// VALIDATES: runBucket returns correct bucket labels for all ranges.
	// PREVENTS: boundary errors at 5/6, 10/11, 20/21 transitions.

	tests := []struct {
		name   string
		length uint64
		want   string
	}{
		{"1", 1, "1"},
		{"2", 2, "2"},
		{"3", 3, "3"},
		{"4", 4, "4"},
		{"5 (boundary)", 5, "5"},
		{"6 (first in 6-10)", 6, "6-10"},
		{"10 (last in 6-10)", 10, "6-10"},
		{"11 (first in 11-20)", 11, "11-20"},
		{"20 (last in 11-20)", 20, "11-20"},
		{"21 (first in 21+)", 21, "21+"},
		{"100", 100, "21+"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, runBucket(tt.length))
		})
	}
}

func TestFmtComm(t *testing.T) {
	// VALIDATES: fmtComm formats standard communities as "high:low".
	// PREVENTS: wrong bit shifting producing reversed or truncated values.

	tests := []struct {
		name string
		c    uint32
		want string
	}{
		{"zero", 0, "0:0"},
		{"simple", 0x00010002, "1:2"},
		{"max values", 0xFFFEFFFF, "65534:65535"},
		{"high word only", 0xFFFF0000, "65535:0"},
		{"low word only", 0x0000FFFF, "0:65535"},
		{"typical community", (64500 << 16) | 100, "64500:100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fmtComm(tt.c))
		})
	}
}

// ---------------------------------------------------------------------------
// communities.go tests
// ---------------------------------------------------------------------------

func TestIsActionCommunity(t *testing.T) {
	// VALIDATES: isActionCommunity identifies well-known/action communities by high word.
	// PREVENTS: missing one of the four action community ranges (0, 65535, 65281, 65282).

	tests := []struct {
		name string
		comm uint32
		want bool
	}{
		// High word = 0 (private/reserved).
		{"high=0, low=0", 0x00000000, true},
		{"high=0, low=1", 0x00000001, true},
		{"high=0, low=65535", 0x0000FFFF, true},

		// High word = 65535 (well-known, RFC 1997).
		{"NO_EXPORT (65535:65281)", 0xFFFFFF01, true},
		{"NO_ADVERTISE (65535:65282)", 0xFFFFFF02, true},
		{"65535:0", 0xFFFF0000, true},

		// High word = 65281 (NO_EXPORT subconfed).
		{"65281:0", (65281 << 16), true},
		{"65281:100", (65281 << 16) | 100, true},

		// High word = 65282.
		{"65282:0", (65282 << 16), true},
		{"65282:42", (65282 << 16) | 42, true},

		// Non-action communities.
		{"normal community 64500:100", (64500 << 16) | 100, false},
		{"high=1", 0x00010000, false},
		{"high=65534", (65534 << 16), false},
		{"high=65283", (65283 << 16), false},
		{"high=32768", (32768 << 16), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isActionCommunity(tt.comm))
		})
	}
}

// ---------------------------------------------------------------------------
// Integration: extractBGP4MPUpdate + countUpdateNLRIs round-trip
// ---------------------------------------------------------------------------

func TestExtractAndCountRoundTrip(t *testing.T) {
	// VALIDATES: end-to-end parsing of a BGP4MP record through to NLRI counting.
	// PREVENTS: misalignment between extractBGP4MPUpdate body and countUpdateNLRIs input.

	attrs := concatBytes(
		makeAttr(0x40, attrOrigin, []byte{0x00}),
		makeAttr(0x40, attrNextHop, []byte{10, 0, 0, 1}),
	)
	nlri := concatBytes(packedPrefix(24), packedPrefix(16), packedPrefix(8))
	withdrawn := concatBytes(packedPrefix(32))

	updateBody := buildUpdate(withdrawn, attrs, nlri)
	record := makeBGP4MPRecord(4, 65000, 1, updateBody)

	body, peerASN := extractBGP4MPUpdate(subtypeBGP4MPMessageAS4, record)
	require.NotNil(t, body)
	assert.Equal(t, uint32(65000), peerASN)

	ann, wd := countUpdateNLRIs(body)
	assert.Equal(t, 3, ann, "expected 3 announced (trailing NLRI)")
	assert.Equal(t, 1, wd, "expected 1 withdrawn")

	// Verify attrs extraction.
	gotAttrs := extractUpdateAttrs(body)
	assert.Equal(t, attrs, gotAttrs)
	assert.Equal(t, 2, countAttrs(gotAttrs))
}
