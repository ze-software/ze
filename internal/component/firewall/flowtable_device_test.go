// VALIDATES: a flowtable holds every device the operator named, whether they
// name one or several.
// PREVENTS: a flowtable with no devices at all. Tree.ToMap collapses a
// one-member leaf-list to a bare string, and the parse asserted []any on
// `device`, so a flowtable configured with a single device offloaded nothing,
// with no error, on the most ordinary flowtable an operator writes. A
// two-device test passes with that bug in place and proves nothing.

package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlowtableSingleDevice(t *testing.T) {
	for _, tc := range []struct {
		name   string
		device any
		want   []string
	}{
		{"one device, the bare string ToMap emits at count one", "eth0", []string{"eth0"}},
		{"two devices, the array ToMap emits at count two", []any{"eth0", "eth1"}, []string{"eth0", "eth1"}},
		{"no device leaf at all", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := map[string]any{"hook": "ingress", "priority": "0"}
			if tc.device != nil {
				m["device"] = tc.device
			}

			ft, err := parseFlowtable("ft0", m)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ft.Devices)
		})
	}
}
