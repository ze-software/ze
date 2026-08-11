//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// VALIDATES: spec-fixit-route-removal-protocol-blind -- the operator-visible
// half of stamping interface-layer routes with rtm_protocol 253. protocolName
// fills iface.KernelRoute.Protocol, which `show route`
// (internal/component/iface/cmd/show_route.go) and the web IP Routes page
// (internal/component/web/page_ip_routes.go) print, so 253 must read as a
// producer name and not as a bare number.
// PREVENTS: rtproto.Name losing the Iface entry, or protocolName no longer
// consulting rtproto before the kernel's own table. The boot row is the one
// this spec changed away from: an interface-layer route carried RTPROT_BOOT
// until AddRoute started stamping it, and every route that still carries 3
// must keep reading "boot".
//
// Every kernel number below is written as a literal, taken from
// linux/rtnetlink.h. rtProtoNames spells the same numbers through the
// golang.org/x/sys/unix constants, so a literal here is an independent
// statement of the value rather than a restatement of the map's own source.
// The map used to carry 42 as "ra" and 193 as "babel", where RTPROT_RA is 9,
// RTPROT_BABEL is 42, and 193 is allocated to nothing: a kernel accept_ra
// default route printed as "9" and a Babel route printed as "ra".
func TestProtocolNameNamesEveryProducer(t *testing.T) {
	cases := []struct {
		name  string
		proto int
		want  string
	}{
		{"interface layer", int(rtproto.Iface), "ze-iface"},
		{"static plugin", int(rtproto.Static), "ze-static"},
		{"fib kernel plugin", int(rtproto.FIBKernel), "ze-fib"},
		{"policy routing plugin", int(rtproto.PolicyRoute), "ze-policy-route"},
		{"kernel boot", 3, "boot"},
		{"kernel connected", 2, "kernel"},
		{"iproute2 static", 4, "static"},
		{"router advertisement", 9, "ra"},
		{"legacy quagga zebra", 11, "zebra"},
		{"bird", 12, "bird"},
		{"dhcp client", 16, "dhcp"},
		{"babel", 42, "babel"},
		{"frr bgp", 186, "bgp"},
		{"frr eigrp", 192, "eigrp"},
		{"unallocated daemon id", 193, "193"},
		{"unknown producer", 200, "200"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := protocolName(tc.proto); got != tc.want {
				t.Fatalf("protocolName(%d) = %q, want %q", tc.proto, got, tc.want)
			}
		})
	}
}
