package bridge

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBridgeVPLSCommand pins the ze command ExaBGP's `vpls` route becomes.
//
// VALIDATES: the five NLRI keywords are renamed to ze's (endpoint -> ve-id,
// offset -> ve-block-offset, size -> ve-block-size, base -> label-base) and
// written after `mp l2vpn/vpls`, wherever ExaBGP placed them; next-hop and the
// other attributes go before it, as for every family; the withdraw verb and the
// `withdraw` flag both delete. The lines are the 4.2 qa/api api-vpls script's.
// PREVENTS: `announce vpls endpoint ...` being refused as an untranslated line,
// which stalled old-4.2 api-vpls until its timeout.
func TestBridgeVPLSCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"announce with attributes",
			"neighbor 127.0.0.1 announce vpls endpoint 5 base 10702 offset 1 size 8 rd 192.168.201.1:123 next-hop 192.168.201.1 origin igp as-path [ 30740 30740 ] local-preference 100 med 2000 community [ 54591:123] extended-community [ target:54591:6 l2info:19:0:1500:111] originator-id 192.168.22.1 cluster-list [ 3.3.3.3 192.168.201.1 ]",
			"send bgp 127.0.0.1 update text nhop 192.168.201.1 origin igp as-path [ 30740 30740 ] local-preference 100 med 2000 community [ 54591:123] extended-community [target:54591:6 l2info:19:0:1500:111] originator-id 192.168.22.1 cluster-list [ 3.3.3.3 192.168.201.1 ] mp l2vpn/vpls add rd 192.168.201.1:123 ve-id 5 ve-block-offset 1 ve-block-size 8 label-base 10702",
		},
		{
			"withdraw",
			"neighbor 127.0.0.1 withdraw vpls endpoint 5 base 10702 offset 1 size 8 rd 192.168.201.1:123 next-hop 192.168.201.1",
			"send bgp 127.0.0.1 update text nhop 192.168.201.1 mp l2vpn/vpls del rd 192.168.201.1:123 ve-id 5 ve-block-offset 1 ve-block-size 8 label-base 10702",
		},
		{
			"withdraw flag",
			"neighbor 127.0.0.1 announce vpls rd 1:1 endpoint 1 offset 0 size 10 base 1000 withdraw",
			"send bgp 127.0.0.1 update text mp l2vpn/vpls add rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label-base 1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, translatedCommand(t, tt.input))
		})
	}
}

// TestBridgeVPLSRefusesMissingField proves a vpls route missing an NLRI field
// is refused by name rather than sent with a zero in its place.
//
// VALIDATES: each of the five fields is required.
// PREVENTS: a VPLS NLRI on the wire with a VE ID or label base nobody wrote.
func TestBridgeVPLSRefusesMissingField(t *testing.T) {
	for _, line := range []string{
		"neighbor 127.0.0.1 announce vpls base 10702 offset 1 size 8 rd 1:1",
		"neighbor 127.0.0.1 announce vpls endpoint 5 offset 1 size 8 rd 1:1",
		"neighbor 127.0.0.1 announce vpls endpoint 5 base 10702 size 8 rd 1:1",
		"neighbor 127.0.0.1 announce vpls endpoint 5 base 10702 offset 1 rd 1:1",
		"neighbor 127.0.0.1 announce vpls endpoint 5 base 10702 offset 1 size 8",
		"neighbor 127.0.0.1 announce vpls endpoint 5 base 10702 offset 1 size 8 rd",
	} {
		_, err := TranslateLine(line)
		if err == nil {
			t.Errorf("TranslateLine(%q) succeeded, want a refusal", line)
			continue
		}
		if !errors.Is(err, errVPLSFieldMissing) && !errors.Is(err, errMissingAttrValue) {
			t.Errorf("TranslateLine(%q) = %v, want a missing-field refusal", line, err)
		}
	}
}
