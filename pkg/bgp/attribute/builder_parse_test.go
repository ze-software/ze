package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuilderParseOrigin verifies text origin parsing.
//
// VALIDATES: Builder correctly parses origin strings.
// PREVENTS: Invalid origin values from text commands.
func TestBuilderParseOrigin(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint8
		wantErr bool
	}{
		{name: "igp", input: "igp", want: 0},
		{name: "egp", input: "egp", want: 1},
		{name: "incomplete", input: "incomplete", want: 2},
		{name: "IGP_uppercase", input: "IGP", want: 0},
		{name: "question_mark", input: "?", want: 2},
		{name: "invalid", input: "invalid", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseOrigin(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			wire := b.Build()
			// Check origin value at offset 3
			assert.Equal(t, tt.want, wire[3])
		})
	}
}

// TestBuilderParseASPath verifies AS_PATH text parsing.
//
// VALIDATES: Builder correctly parses AS_PATH strings.
// PREVENTS: Malformed AS_PATH from text commands.
func TestBuilderParseASPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []uint32
		wantErr bool
	}{
		{name: "bracketed_spaces", input: "[65001 65002]", want: []uint32{65001, 65002}},
		{name: "bracketed_commas", input: "[65001,65002]", want: []uint32{65001, 65002}},
		{name: "single", input: "65001", want: []uint32{65001}},
		{name: "space_separated", input: "65001 65002 65003", want: []uint32{65001, 65002, 65003}},
		{name: "empty_brackets", input: "[]", want: nil},
		{name: "invalid_asn", input: "[abc]", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseASPath(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, b.asPath)
		})
	}
}

// TestBuilderParseCommunity verifies community text parsing.
//
// VALIDATES: Builder correctly parses community strings.
// PREVENTS: Invalid community values from text commands.
func TestBuilderParseCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []uint32
		wantErr bool
	}{
		{name: "standard", input: "65000:100", want: []uint32{0xFDE80064}},
		{name: "no-export", input: "no-export", want: []uint32{0xFFFFFF01}},
		{name: "no-advertise", input: "no-advertise", want: []uint32{0xFFFFFF02}},
		{name: "no-export-subconfed", input: "no-export-subconfed", want: []uint32{0xFFFFFF03}},
		{name: "nopeer", input: "nopeer", want: []uint32{0xFFFFFF04}},
		{name: "multiple", input: "65000:100 65000:200", want: []uint32{0xFDE80064, 0xFDE800C8}},
		{name: "bracketed", input: "[65000:100 65000:200]", want: []uint32{0xFDE80064, 0xFDE800C8}},
		{name: "invalid_format", input: "invalid", wantErr: true},
		{name: "missing_value", input: "65000:", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseCommunity(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, b.communities)
		})
	}
}

// TestBuilderParseLargeCommunity verifies large community text parsing.
//
// VALIDATES: Builder correctly parses large community strings.
// PREVENTS: Invalid large community values from text commands.
func TestBuilderParseLargeCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []LargeCommunity
		wantErr bool
	}{
		{name: "single", input: "65000:1:2", want: []LargeCommunity{{65000, 1, 2}}},
		{name: "multiple", input: "65000:1:2 65001:3:4", want: []LargeCommunity{{65000, 1, 2}, {65001, 3, 4}}},
		{name: "bracketed", input: "[65000:1:2]", want: []LargeCommunity{{65000, 1, 2}}},
		{name: "invalid_format", input: "65000:1", wantErr: true},
		{name: "invalid_number", input: "abc:1:2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseLargeCommunity(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, b.largeCommunities)
		})
	}
}

// TestBuilderParseExtCommunity verifies extended community text parsing.
//
// VALIDATES: Builder correctly parses extended community strings.
// PREVENTS: Invalid extended community values from text commands.
func TestBuilderParseExtCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []ExtendedCommunity
		wantErr bool
	}{
		{
			name:  "target_2byte",
			input: "target:65000:100",
			want:  []ExtendedCommunity{{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}},
		},
		{
			name:  "origin_2byte",
			input: "origin:65000:100",
			want:  []ExtendedCommunity{{0x00, 0x03, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}},
		},
		{
			name:    "invalid_type",
			input:   "invalid:65000:100",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseExtCommunity(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, b.extCommunities)
		})
	}
}

// TestBuilderParseMED verifies MED text parsing.
//
// VALIDATES: Builder correctly parses MED strings.
// PREVENTS: Invalid MED values from text commands.
func TestBuilderParseMED(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{name: "zero", input: "0", want: 0},
		{name: "positive", input: "100", want: 100},
		{name: "max", input: "4294967295", want: 4294967295},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "overflow", input: "4294967296", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseMED(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, b.med)
			assert.Equal(t, tt.want, *b.med)
		})
	}
}

// TestBuilderParseLocalPref verifies LOCAL_PREF text parsing.
//
// VALIDATES: Builder correctly parses LOCAL_PREF strings.
// PREVENTS: Invalid LOCAL_PREF values from text commands.
func TestBuilderParseLocalPref(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{name: "default", input: "100", want: 100},
		{name: "high", input: "200", want: 200},
		{name: "max", input: "4294967295", want: 4294967295},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			err := b.ParseLocalPref(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, b.localPref)
			assert.Equal(t, tt.want, *b.localPref)
		})
	}
}
