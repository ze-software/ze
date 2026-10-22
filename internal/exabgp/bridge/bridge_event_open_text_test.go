package bridge

import "testing"

// VALIDATES: the text OPEN capability list matches ExaBGP 4.2, 5.0 and the pin.
// PREVENTS: ze's own capability words reaching an ExaBGP text script.
//
// TestCapabilitiesText checks the words between the brackets of a text OPEN
// line against what ExaBGP's capability classes write: code order, ", "
// between codes, and the payload read in wire order. Each case is the shape a
// table lookup by name would get wrong: a family-keyed payload whose order
// matters, a family sent twice, a code ExaBGP has no class for, a role past
// the five RFC 9234 assigns, an empty Extended Next Hop, and a payload the
// class would refuse.
func TestCapabilitiesText(t *testing.T) {
	capability := func(code float64, value string) any {
		return map[string]any{"code": code, "value": value}
	}
	for _, tc := range []struct {
		name string
		list []any
		want string
	}{
		{
			name: "code-order",
			list: []any{
				capability(65, "65533"), capability(6, ""), capability(1, "ipv4/unicast"),
				capability(1, "ipv6/unicast"), capability(2, ""), capability(185, ""),
			},
			want: "multiprotocol(ipv4 unicast,ipv6 unicast), route refresh, extended message(65535), asn4(65533), operational",
		},
		{
			name: "graceful-restart-wire-order",
			list: []any{capability(64, "84b0"+"000201c0"+"00010100")},
			want: "graceful restart flags 0x8 time 1200 ipv6/unicast=0x80 ipv4/unicast=0x0",
		},
		{
			name: "graceful-restart-family-twice",
			list: []any{capability(64, "00b4"+"00010180"+"00020180"+"00010100")},
			want: "graceful restart flags 0x0 time 180 ipv4/unicast=0x0 ipv6/unicast=0x80",
		},
		{
			name: "graceful-restart-no-family",
			list: []any{capability(64, "0078")},
			want: "graceful restart flags 0x0 time 120 ",
		},
		{
			name: "addpath",
			list: []any{capability(69, "ipv4/unicast send-receive"), capability(69, "ipv6/unicast receive")},
			want: "addpath(send/receive ipv4 unicast,receive ipv6 unicast)",
		},
		{
			name: "nexthop-empty",
			list: []any{capability(5, "")},
			want: "nexthop()",
		},
		{
			name: "nexthop",
			list: []any{capability(5, "ipv4/unicast ipv6")},
			want: "nexthop(ipv4 unicast ipv6)",
		},
		{
			name: "hostname-lowercased",
			list: []any{capability(73, "06526f75746572"+"00")},
			want: "hostname(router )",
		},
		{
			name: "role-unassigned",
			list: []any{capability(9, "07")},
			want: "role(unassigned 7)",
		},
		{
			name: "role",
			list: []any{capability(9, "03")},
			want: "role(customer)",
		},
		{
			name: "multisession-both",
			list: []any{capability(131, "00"), capability(68, "00")},
			want: "multisession (rfc) , multisession ",
		},
		{
			name: "unassigned-code",
			list: []any{capability(100, "01")},
			want: "unassigned unknown capability 0x64",
		},
		{
			name: "graceful-restart-refused",
			list: []any{capability(64, "00b400")},
			want: "unknown graceful-restart",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(appendCapabilitiesText(nil, tc.list)); got != tc.want {
				t.Errorf("capabilities text = %q\nwant                %q", got, tc.want)
			}
		})
	}
}
