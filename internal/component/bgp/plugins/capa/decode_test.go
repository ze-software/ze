package capa

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeCapability(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON string
	}{
		{
			name:     "multiprotocol ipv4/unicast",
			input:    "decode capability 1 00010001",
			wantJSON: `{"name":"multiprotocol","value":"ipv4/unicast"}`,
		},
		{
			name:     "asn4",
			input:    "decode capability 65 0000FFFD",
			wantJSON: `{"name":"asn4","value":"65533"}`,
		},
		{
			name:     "extended-message empty payload",
			input:    "decode capability 6",
			wantJSON: `{"name":"extended-message"}`,
		},
		{
			name:     "add-path ipv4/unicast send/receive",
			input:    "decode capability 69 0001010300020103",
			wantJSON: `{"name":"add-path","value":["ipv4/unicast","ipv6/unicast"]}`,
		},
		{
			name:     "paths-limit",
			input:    "decode capability 76 000101000A",
			wantJSON: `{"name":"paths-limit","value":["ipv4/unicast 10"]}`,
		},
		{
			name:     "route-refresh zero payload",
			input:    "decode capability 2",
			wantJSON: "",
		},
		{
			name:     "unknown code",
			input:    "decode capability 99 AABB",
			wantJSON: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var in, out bytes.Buffer
			in.WriteString(tc.input + "\n")
			runDecodeMode(&in, &out)

			got := out.String()
			if tc.wantJSON == "" {
				assert.Equal(t, "decoded unknown\n", got)
			} else {
				assert.Equal(t, "decoded json "+tc.wantJSON+"\n", got)
			}
		})
	}
}

func TestDecodeMultiprotocol(t *testing.T) {
	result := decodeCapability(1, "00010001")
	assert.Equal(t, "multiprotocol", result["name"])
	assert.Equal(t, "ipv4/unicast", result["value"])
}

func TestDecodeASN4(t *testing.T) {
	result := decodeCapability(65, "0000FFFD")
	assert.Equal(t, "asn4", result["name"])
	assert.Equal(t, "65533", result["value"])
}

func TestDecodeExtendedNextHop(t *testing.T) {
	result := decodeCapability(5, "000100010002")
	assert.Equal(t, "extended-nexthop", result["name"])
	families, ok := result["value"].([]string)
	assert.True(t, ok)
	assert.Len(t, families, 1)
	assert.Equal(t, "ipv4/unicast->ipv6", families[0])
}

func TestDecodeExtendedMessageEmpty(t *testing.T) {
	result := decodeCapability(6, "")
	assert.Equal(t, "extended-message", result["name"])
	_, hasValue := result["value"]
	assert.False(t, hasValue)
}
