package peer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendRouteParsing verifies send-route options are parsed from expect files.
//
// VALIDATES: LoadExpectFile parses option=update:value=send-route correctly.
// PREVENTS: Routes not being sent because of parsing failure.
func TestSendRouteParsing(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "test.msg")
	content := "option=update:value=send-route:prefix=10.0.0.0/24:next-hop=10.0.0.1\noption=update:value=send-route:prefix=10.0.1.0/24:next-hop=10.0.0.1\n"
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))
	_, config, err := LoadExpectFile(f)
	require.NoError(t, err)
	assert.Equal(t, 2, len(config.SendRoutes), "expected 2 send-routes")
	assert.Equal(t, "10.0.0.0/24", config.SendRoutes[0].Prefix)
	assert.Equal(t, "10.0.0.1", config.SendRoutes[0].NextHop)
	assert.Equal(t, "10.0.1.0/24", config.SendRoutes[1].Prefix)
}

// TestSendRouteParsingEdgeCases verifies send-route parsing handles edge cases.
//
// VALIDATES: LoadExpectFile handles missing/empty fields in send-route options.
// PREVENTS: Panic or silent misconfiguration from malformed route options.
func TestSendRouteParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantRoutes  int
		wantPrefix  string
		wantNextHop string
		wantASN     uint32
	}{
		{
			name:        "missing next-hop defaults to empty",
			line:        "option=update:value=send-route:prefix=10.0.0.0/24\n",
			wantRoutes:  1,
			wantPrefix:  "10.0.0.0/24",
			wantNextHop: "",
			wantASN:     0,
		},
		{
			name:        "with origin-as",
			line:        "option=update:value=send-route:prefix=10.0.0.0/24:next-hop=10.0.0.1:origin-as=65001\n",
			wantRoutes:  1,
			wantPrefix:  "10.0.0.0/24",
			wantNextHop: "10.0.0.1",
			wantASN:     65001,
		},
		{
			name:        "with as-set flag",
			line:        "option=update:value=send-route:prefix=10.0.0.0/24:next-hop=10.0.0.1:as-set=true\n",
			wantRoutes:  1,
			wantPrefix:  "10.0.0.0/24",
			wantNextHop: "10.0.0.1",
		},
		{
			name:        "empty prefix still parsed",
			line:        "option=update:value=send-route:prefix=:next-hop=10.0.0.1\n",
			wantRoutes:  1,
			wantPrefix:  "",
			wantNextHop: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := t.TempDir()
			f := filepath.Join(d, "test.msg")
			require.NoError(t, os.WriteFile(f, []byte(tt.line), 0o600))
			_, config, err := LoadExpectFile(f)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRoutes, len(config.SendRoutes))
			if tt.wantRoutes > 0 {
				assert.Equal(t, tt.wantPrefix, config.SendRoutes[0].Prefix)
				assert.Equal(t, tt.wantNextHop, config.SendRoutes[0].NextHop)
				if tt.wantASN > 0 {
					assert.Equal(t, tt.wantASN, config.SendRoutes[0].OriginAS)
				}
			}
		})
	}
}
