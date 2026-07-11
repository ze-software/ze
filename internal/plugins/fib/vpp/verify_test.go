// VPP FIB verify: input validation / reject logic -- malformed and empty
// prefixes are refused before reaching the backend, and out-of-range or
// over-deep MPLS label stacks are rejected.
package fibvpp

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessEventInvalidPrefix(t *testing.T) {
	// VALIDATES: invalid prefix rejected at JSON parse (netip.Prefix rejects malformed values)
	// PREVENTS: malformed prefix reaching backend
	var b incomingBatch
	err := json.Unmarshal([]byte(`{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"not-a-prefix","next-hop":"1.1.1.1","protocol":"bgp"}]}`), &b)
	if err == nil {
		t.Error("should fail to unmarshal invalid prefix")
	}
}

func TestProcessEventEmptyPrefix(t *testing.T) {
	// VALIDATES: empty prefix skipped
	// PREVENTS: empty prefix reaching backend
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"","next-hop":"1.1.1.1","protocol":"bgp"}]}`))

	if len(mock.adds) != 0 {
		t.Error("should not add route with empty prefix")
	}
}

func TestMPLSLabelRange(t *testing.T) {
	mb := &mockMPLSBackend{}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	t.Run("max-valid", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{1048575})
		assert.NoError(t, err)
	})

	t.Run("over-max", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{1048576})
		assert.Error(t, err)
	})

	t.Run("zero-valid", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{0})
		assert.NoError(t, err)
	})

	t.Run("empty-stack", func(t *testing.T) {
		err := mb.addMPLSRoute(pfx, nh, []uint32{})
		assert.Error(t, err)
	})

	t.Run("stack-depth-16", func(t *testing.T) {
		labels := make([]uint32, 16)
		for i := range labels {
			labels[i] = uint32(i + 100)
		}
		err := mb.addMPLSRoute(pfx, nh, labels)
		assert.NoError(t, err)
	})

	t.Run("stack-depth-17", func(t *testing.T) {
		labels := make([]uint32, 17)
		err := mb.addMPLSRoute(pfx, nh, labels)
		assert.Error(t, err)
	})
}
