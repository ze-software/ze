// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Acct-Interim-Interval clamping

package l2tpauthradius

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClampAcctIntervalWithinRange(t *testing.T) {
	require.Equal(t, uint32(120), clampAcctInterval(120))
	require.Equal(t, uint32(300), clampAcctInterval(300))
}

func TestClampAcctIntervalBelowFloor(t *testing.T) {
	require.Equal(t, uint32(60), clampAcctInterval(59))
	require.Equal(t, uint32(60), clampAcctInterval(1))
	require.Equal(t, uint32(60), clampAcctInterval(0))
}

func TestClampAcctIntervalAboveCeiling(t *testing.T) {
	require.Equal(t, uint32(3600), clampAcctInterval(3601))
	require.Equal(t, uint32(3600), clampAcctInterval(10000))
}

func TestClampAcctIntervalBoundaries(t *testing.T) {
	require.Equal(t, uint32(60), clampAcctInterval(60))
	require.Equal(t, uint32(3600), clampAcctInterval(3600))
}
