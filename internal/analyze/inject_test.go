package analyze

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

func TestBuildUpdateFromRIB(t *testing.T) {
	entry := &mrt.RIBEntry{
		PeerIndex:  0,
		OrigTime:   1700000000,
		Attributes: []byte{0x40, 0x01, 0x01, 0x00},
	}

	msg := buildUpdateFromRIB(24, []byte{10, 0, 0}, entry, true)

	require.True(t, len(msg) >= 19)

	for i := range 16 {
		assert.Equal(t, byte(0xff), msg[i], "marker byte %d", i)
	}

	totalLen := binary.BigEndian.Uint16(msg[16:18])
	assert.Equal(t, uint16(len(msg)), totalLen, "BGP length field")
	assert.Equal(t, byte(2), msg[18], "message type should be UPDATE")

	withdrawnLen := binary.BigEndian.Uint16(msg[19:21])
	assert.Equal(t, uint16(0), withdrawnLen, "withdrawn routes length")

	attrLen := binary.BigEndian.Uint16(msg[21:23])
	assert.Equal(t, uint16(4), attrLen, "path attributes length")
	assert.Equal(t, entry.Attributes, msg[23:27], "path attributes content")

	assert.Equal(t, byte(24), msg[27], "prefix length")
	assert.Equal(t, []byte{10, 0, 0}, msg[28:31], "prefix bytes")
}

func TestBuildUpdateFromRIB_EmptyAttrs(t *testing.T) {
	entry := &mrt.RIBEntry{Attributes: nil}
	msg := buildUpdateFromRIB(0, nil, entry, true)

	require.True(t, len(msg) >= 19)
	assert.Equal(t, byte(2), msg[18])

	attrLen := binary.BigEndian.Uint16(msg[21:23])
	assert.Equal(t, uint16(0), attrLen)

	assert.Equal(t, byte(0), msg[23], "prefix length for /0")
}

func TestParseInjectOpts(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOK  bool
		localAS uint32
	}{
		{"basic", []string{"file.mrt", "10.0.0.1:179"}, true, 65000},
		{"with-as", []string{"--local-as", "64512", "file.mrt", "10.0.0.1:179"}, true, 64512},
		{"missing-peer", []string{"file.mrt"}, false, 0},
		{"bad-as", []string{"--local-as", "abc", "file.mrt", "10.0.0.1:179"}, false, 0},
		{"empty", nil, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, ok := parseInjectOpts(tt.args)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.localAS, opts.localAS)
			}
		})
	}
}

func TestParseReplayOpts(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		wantOK bool
		speed  float64
	}{
		{"basic", []string{"file.mrt", "10.0.0.1:179"}, true, 1.0},
		{"fast", []string{"--speed", "10", "file.mrt", "10.0.0.1:179"}, true, 10.0},
		{"no-delay", []string{"--speed", "0", "file.mrt", "10.0.0.1:179"}, true, 0.0},
		{"negative-speed", []string{"--speed", "-1", "file.mrt", "10.0.0.1:179"}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, ok := parseReplayOpts(tt.args)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.InDelta(t, tt.speed, opts.speed, 0.001)
			}
		})
	}
}
