// VALIDATES: metric families bind idempotently to a registry, and protocol
// numbers render to the upstream-compatible names (unknown -> decimal).
// PREVENTS: duplicate-registration panics on reconfigure; wrong protocol labels.

package trafficusage

import (
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

func TestBindMetrics(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	BindMetrics(reg)
	if metricsPtr.Load() == nil {
		t.Fatal("metricsPtr is nil after BindMetrics")
	}
	// Idempotent: a config reload calls BindMetrics again; must not panic on
	// duplicate registration.
	BindMetrics(reg)
	// nil registry must be a no-op, not a panic.
	BindMetrics(nil)
}

func TestProtocolName(t *testing.T) {
	cases := map[uint8]string{
		1:   "icmp",
		6:   "tcp",
		17:  "udp",
		47:  "gre",
		50:  "esp",
		51:  "ah",
		58:  "icmpv6",
		0:   "0",
		99:  "99",
		255: "255",
	}
	for p, want := range cases {
		if got := protoName(p); got != want {
			t.Errorf("protoName(%d) = %q, want %q", p, got, want)
		}
	}
}

func TestIPString(t *testing.T) {
	// Map key is the raw IPv4 header bytes read as a host-order uint32; on a
	// little-endian host 127.0.0.1 is stored as 0x0100007f and must render back
	// to the dotted quad (matches upstream LittleEndian decode).
	if got := ipString(0x0100007f); got != "127.0.0.1" {
		t.Errorf("ipString(0x0100007f) = %q, want 127.0.0.1", got)
	}
}
