package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseUpdateBlock_InvalidMED verifies error on invalid MED value.
//
// VALIDATES: Non-numeric MED produces clear error at parse time.
// PREVENTS: Silent failures with MED=0.
func TestParseUpdateBlock_InvalidMED(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                med abc;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	// YANG schema validates uint32, so error happens at parse time
	require.Error(t, err)
	require.Contains(t, err.Error(), "med")
}
