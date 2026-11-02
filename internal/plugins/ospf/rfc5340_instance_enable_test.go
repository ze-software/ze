// Design: docs/architecture/ospf/ospf-ext-15-multi-af.md -- one engine per address family
// Related: instance.go -- openInterface; v3/transport/transport.go -- rxLoop Instance ID filter
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// instanceRecordingTransport records the Instance ID each interface is enabled with.
type instanceRecordingTransport struct {
	Transport
	enabled map[string]uint8
}

func (r *instanceRecordingTransport) EnableInterface(name string, instanceID uint8) {
	r.enabled[name] = instanceID
	r.Transport.EnableInterface(name, instanceID)
}

// VALIDATES: an OSPFv3 engine enables each interface on its transport with the
// engine's configured Instance ID, so the transport's RFC 5340 Section 4.2.1
// receive filter ("discard a packet whose Instance ID does not match") keeps the
// packets of this instance.
// Method: open the interfaces of a v6-codec engine configured with Instance ID 64
// (RFC 5838's IPv4-unicast range) and with the default 0, and read what the
// transport was told.
// PREVENTS: every interface enabled with Instance ID 0, so an instance with any
// other Instance ID discards all its neighbors' packets and never forms an
// adjacency (two Ze routers running IPv4-unicast over OSPFv3 saw no neighbor).
func TestOSPFv3InterfaceEnabledWithConfiguredInstanceID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		af         addressFamily
		instanceID uint8
	}{
		{"ipv4-unicast-64", afIPv4Unicast, 64},
		{"ipv6-unicast-default", afIPv6Unicast, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &instanceRecordingTransport{Transport: transport.New(&fakeBackend{}), enabled: map[string]uint8{}}
			eng := newEngineWithCodecAF(rec, v6Codec{}, tc.af)
			t.Cleanup(eng.shutdown)
			cfg := v6RIConfig(t, "area")
			cfg.InstanceID = tc.instanceID
			eng.setConfig(cfg)
			if err := eng.openInterfaces(); err != nil {
				t.Fatalf("openInterfaces: %v", err)
			}
			got, ok := rec.enabled["eth0"]
			if !ok {
				t.Fatalf("eth0 was never enabled on the transport: %+v", rec.enabled)
			}
			if got != tc.instanceID {
				t.Fatalf("eth0 enabled with Instance ID %d, want the configured %d", got, tc.instanceID)
			}
		})
	}
}
