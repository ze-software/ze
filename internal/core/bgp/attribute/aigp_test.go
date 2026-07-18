package attribute

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAIGPMetricTLV(metric uint64) []byte {
	buf := make([]byte, 11)
	buf[0] = 1  // type
	buf[1] = 0  // length high
	buf[2] = 11 // length low
	binary.BigEndian.PutUint64(buf[3:], metric)
	return buf
}

// RFC requirement: RFC7311-3-1 positive -- an AIGP TLV of type 1 (AIGP metric)
// with length 11 (3-octet header + 8-octet metric) is accepted and its metric is
// decoded (RFC 7311 sec 3; producer ParseAIGP internal/core/bgp/attribute/aigp.go:118).
func TestParseAIGP(t *testing.T) {
	data := makeAIGPMetricTLV(100)
	aigp, err := ParseAIGP(data)
	require.NoError(t, err)
	require.Len(t, aigp.TLVs, 1)
	assert.Equal(t, uint8(1), aigp.TLVs[0].Type)
	metric, ok := aigp.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(100), metric)
}

func TestParseAIGPZeroMetric(t *testing.T) {
	data := makeAIGPMetricTLV(0)
	aigp, err := ParseAIGP(data)
	require.NoError(t, err)
	metric, ok := aigp.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(0), metric)
}

func TestParseAIGPMaxMetric(t *testing.T) {
	data := makeAIGPMetricTLV(0xFFFFFFFFFFFFFFFF)
	aigp, err := ParseAIGP(data)
	require.NoError(t, err)
	metric, ok := aigp.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), metric)
}

// RFC requirement: RFC7311-3-2 positive -- the total AIGP attribute length is
// consistent with its contained TLVs: a buffer of a length-11 metric TLV followed
// by a length-7 unknown TLV parses into exactly those two TLVs, each consuming its
// declared length with no leftover or overrun (RFC 7311 sec 3; ParseAIGP walks
// off += tlvLen and requires off+tlvLen <= len(data), aigp.go:98-131).
// RFC requirement: RFC7311-3-3 positive -- an unknown AIGP TLV type is PRESERVED,
// not discarded: the type-2 TLV is retained with its exact 4-octet value while the
// type-1 metric is still interpreted (RFC 7311 sec 3, aigp.go:122-128).
func TestParseAIGPMultipleTLVs(t *testing.T) {
	// metric TLV + unknown TLV type 2 with 4 bytes of data
	data := makeAIGPMetricTLV(200)
	unknownTLV := []byte{2, 0, 7, 0xDE, 0xAD, 0xBE, 0xEF} // type=2, length=7, data=4 bytes
	data = append(data, unknownTLV...)

	aigp, err := ParseAIGP(data)
	require.NoError(t, err)
	require.Len(t, aigp.TLVs, 2)

	assert.Equal(t, uint8(1), aigp.TLVs[0].Type)
	metric, ok := aigp.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(200), metric)

	assert.Equal(t, uint8(2), aigp.TLVs[1].Type)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, aigp.TLVs[1].Data)
}

func TestParseAIGPMalformedEmpty(t *testing.T) {
	_, err := ParseAIGP([]byte{})
	assert.Error(t, err)
}

func TestParseAIGPMalformedTruncatedHeader(t *testing.T) {
	_, err := ParseAIGP([]byte{1, 0}) // only 2 bytes, need 3 for header
	assert.Error(t, err)
}

// RFC requirement: RFC7311-3-2 negative -- an attribute length inconsistent with
// its contained TLV is rejected: a type-1 TLV declaring length 11 but with the
// buffer only 8 octets long (5 value octets, not 8) overruns the attribute, so
// ParseAIGP returns an error instead of reading past the end (RFC 7311 sec 3;
// aigp.go:111 off+tlvLen > len(data)).
func TestParseAIGPMalformedTruncatedValue(t *testing.T) {
	// type=1, length=11, but only 5 bytes of value instead of 8
	data := []byte{1, 0, 11, 0, 0, 0, 0, 0}
	_, err := ParseAIGP(data)
	assert.Error(t, err)
}

// RFC requirement: RFC7311-3-1 negative -- a type-1 (AIGP metric) TLV whose
// length is not 11 (here 8) is rejected: ParseAIGP returns an error rather than
// accepting a malformed metric TLV (RFC 7311 sec 3; aigp.go:118 tlvType == 1 &&
// tlvLen != aigpTLVMetricLen).
func TestParseAIGPMalformedMetricWrongLength(t *testing.T) {
	// type=1 but length=8 instead of 11
	data := []byte{1, 0, 8, 0, 0, 0, 0, 0}
	_, err := ParseAIGP(data)
	assert.Error(t, err)
}

func TestParseAIGPMalformedTLVLengthTooSmall(t *testing.T) {
	// length=2 which is less than header length (3)
	data := []byte{1, 0, 2}
	_, err := ParseAIGP(data)
	assert.Error(t, err)
}

func TestAIGPWriteTo(t *testing.T) {
	aigp := NewAIGPMetric(100)
	buf := make([]byte, aigp.Len())
	n := aigp.WriteTo(buf, 0)

	assert.Equal(t, 11, n)
	assert.Equal(t, uint8(1), buf[0])                              // TLV type
	assert.Equal(t, uint16(11), binary.BigEndian.Uint16(buf[1:]))  // TLV length
	assert.Equal(t, uint64(100), binary.BigEndian.Uint64(buf[3:])) // metric
}

func TestAIGPWriteToRoundTrip(t *testing.T) {
	original := NewAIGPMetric(42)
	buf := make([]byte, original.Len())
	original.WriteTo(buf, 0)

	parsed, err := ParseAIGP(buf)
	require.NoError(t, err)
	metric, ok := parsed.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(42), metric)
}

// RFC requirement: RFC7311-3-3 negative -- preservation is not discarded on
// RE-ENCODE either: an AIGP carrying an unknown type-2 TLV is written by WriteTo
// and re-parsed with the unknown TLV still present and its data intact, so the
// codec never drops unknown TLVs on the way back out (RFC 7311 sec 3; WriteTo
// aigp.go:50-61 emits every TLV regardless of type).
func TestAIGPWriteToMultipleTLVs(t *testing.T) {
	aigp := &AIGP{
		TLVs: []AIGPTLV{
			{Type: 1, Data: func() []byte { b := make([]byte, 8); binary.BigEndian.PutUint64(b, 300); return b }()},
			{Type: 2, Data: []byte{0xAB, 0xCD}},
		},
	}
	assert.Equal(t, 11+5, aigp.Len()) // 11 for metric TLV + 5 for unknown TLV (3 hdr + 2 data)
	buf := make([]byte, aigp.Len())
	n := aigp.WriteTo(buf, 0)
	assert.Equal(t, 16, n)

	parsed, err := ParseAIGP(buf)
	require.NoError(t, err)
	require.Len(t, parsed.TLVs, 2)
	metric, ok := parsed.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(300), metric)
	assert.Equal(t, []byte{0xAB, 0xCD}, parsed.TLVs[1].Data)
}

func TestAIGPLen(t *testing.T) {
	tests := []struct {
		name string
		aigp *AIGP
		want int
	}{
		{"single metric", NewAIGPMetric(0), 11},
		{"metric + unknown", &AIGP{TLVs: []AIGPTLV{
			{Type: 1, Data: make([]byte, 8)},
			{Type: 3, Data: make([]byte, 4)},
		}}, 11 + 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.aigp.Len())
		})
	}
}

func TestAIGPCode(t *testing.T) {
	aigp := NewAIGPMetric(0)
	assert.Equal(t, AttrAIGP, aigp.Code())
	assert.Equal(t, AttributeCode(26), aigp.Code())
}

func TestAIGPFlags(t *testing.T) {
	aigp := NewAIGPMetric(0)
	flags := aigp.Flags()
	assert.True(t, flags.IsOptional())
	assert.True(t, flags.IsTransitive())
}

func TestAIGPMetricNotPresent(t *testing.T) {
	aigp := &AIGP{TLVs: []AIGPTLV{{Type: 2, Data: []byte{0x01, 0x02}}}}
	_, ok := aigp.Metric()
	assert.False(t, ok)
}

func TestAIGPAppendText(t *testing.T) {
	aigp := NewAIGPMetric(12345)
	buf := aigp.AppendText(nil)
	assert.Equal(t, "aigp 12345", string(buf))
}

func TestAIGPAppendTextNoMetric(t *testing.T) {
	aigp := &AIGP{TLVs: []AIGPTLV{{Type: 2, Data: []byte{0x01}}}}
	buf := aigp.AppendText(nil)
	assert.Empty(t, buf)
}

func TestAIGPCheckedWriteTo(t *testing.T) {
	aigp := NewAIGPMetric(100)

	// Success case
	buf := make([]byte, 20)
	n, err := aigp.CheckedWriteTo(buf, 0)
	assert.NoError(t, err)
	assert.Equal(t, 11, n)

	// Buffer too small
	small := make([]byte, 5)
	_, err = aigp.CheckedWriteTo(small, 0)
	assert.Error(t, err)
}

func TestWriteAIGPMetric(t *testing.T) {
	buf := make([]byte, AIGPWireLen)
	n := WriteAIGPMetric(buf, 0, 999)
	assert.Equal(t, 11, n)
	assert.Equal(t, uint8(1), buf[0])
	assert.Equal(t, uint16(11), binary.BigEndian.Uint16(buf[1:]))
	assert.Equal(t, uint64(999), binary.BigEndian.Uint64(buf[3:]))
}

func TestNewAIGPMetric(t *testing.T) {
	aigp := NewAIGPMetric(0xDEADBEEF)
	require.Len(t, aigp.TLVs, 1)
	assert.Equal(t, uint8(1), aigp.TLVs[0].Type)
	metric, ok := aigp.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(0xDEADBEEF), metric)
}

func TestBuilderParseAIGP(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "zero", input: "0", want: 0},
		{name: "normal", input: "100", want: 100},
		{name: "large", input: "18446744073709551615", want: 0xFFFFFFFFFFFFFFFF},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseAIGP(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, b.aigpMetric)
			assert.Equal(t, tt.want, *b.aigpMetric)
		})
	}
}

func TestBuilderAIGPWireRoundTrip(t *testing.T) {
	b := NewBuilder()
	b.SetOrigin(0)
	b.SetAIGP(500)
	wireBytes := b.Build()
	require.NotEmpty(t, wireBytes)

	// Parse back: find AIGP attribute (type 26) in wire bytes
	ctxID := setupTestContext(true)
	attrs := NewAttributesWire(wireBytes, ctxID)
	aigp, err := attrs.Get(AttrAIGP)
	require.NoError(t, err)
	require.NotNil(t, aigp)

	a, ok := aigp.(*AIGP)
	require.True(t, ok)
	metric, ok := a.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(500), metric)
}

func TestBuilderAIGPIsEmpty(t *testing.T) {
	b := NewBuilder()
	assert.True(t, b.IsEmpty())
	b.SetAIGP(100)
	assert.False(t, b.IsEmpty())
}

func TestBuilderAIGPReset(t *testing.T) {
	b := NewBuilder()
	b.SetAIGP(100)
	b.Reset()
	assert.True(t, b.IsEmpty())
	assert.Nil(t, b.aigpMetric)
}

func TestBuilderAIGPToAttributes(t *testing.T) {
	b := NewBuilder()
	b.SetAIGP(42)
	attrs := b.ToAttributes()

	var found *AIGP
	for _, attr := range attrs {
		if a, ok := attr.(*AIGP); ok {
			found = a
		}
	}
	require.NotNil(t, found)
	metric, ok := found.Metric()
	assert.True(t, ok)
	assert.Equal(t, uint64(42), metric)
}

func TestParseAIGPBoundaryTLVLength(t *testing.T) {
	// TLV with length exactly 3 (header only, empty value) for unknown type
	data := []byte{5, 0, 3} // type=5, length=3, no value
	aigp, err := ParseAIGP(data)
	require.NoError(t, err)
	require.Len(t, aigp.TLVs, 1)
	assert.Equal(t, uint8(5), aigp.TLVs[0].Type)
	assert.Empty(t, aigp.TLVs[0].Data)
}
