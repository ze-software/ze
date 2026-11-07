package topology

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestPPPoEDHCPv6PDVerdicts proves each clause of the pppoe-dhcpv6-pd-ze-client
// checker refuses the state a broken CPE leaves: no link-local or one on a
// kernel pppN device (AC-1), a default route not via accel-ppp on pppoe0 or
// no SLAAC address (AC-5), and a Solicit without the /56 hint, no Request for
// the offered /60, or a Reply for another prefix (AC-6).
func TestPPPoEDHCPv6PDVerdicts(t *testing.T) {
	linkLocal := "7: pppoe0    inet6 fe80::a1b2:c3d4:e5f6:1/64 scope link \\       valid_lft forever preferred_lft forever\n"
	if err := linkLocalVerdict(linkLocal, cpeDevice); err != nil {
		t.Errorf("link-local on pppoe0: %v", err)
	}
	for name, answer := range map[string]string{
		"none":      "1: lo    inet6 ::1/128 scope host\n",
		"on ppp0":   strings.Replace(linkLocal, "pppoe0", "ppp0", 1),
		"zero iid":  "7: pppoe0    inet6 fe80::/64 scope link\n",
		"not a /64": "7: pppoe0    inet6 fe80::1/128 scope link\n",
	} {
		if err := linkLocalVerdict(answer, cpeDevice); err == nil {
			t.Errorf("link-local %s: accepted", name)
		}
	}
	if err := defaultRouterVerdict("default via fe80::1 dev pppoe0 proto ra metric 1024 expires 1799sec hoplimit 64 pref medium\n", pdAccelLinkLocal, cpeDevice); err != nil {
		t.Errorf("RA default route: %v", err)
	}
	if err := defaultRouterVerdict("default via fe80::1 dev eth0 proto ra metric 1024\n", pdAccelLinkLocal, cpeDevice); err == nil {
		t.Error("default route on eth0: accepted")
	}
	pool := netip.MustParsePrefix(pdSLAACPool)
	if err := slaacVerdict("7: pppoe0    inet6 2001:db8:198:1:a1b2:c3d4:e5f6:1/64 scope global dynamic mngtmpaddr\n", pool); err != nil {
		t.Errorf("SLAAC address: %v", err)
	}
	if err := slaacVerdict("7: pppoe0    inet6 2001:db8:99::1/64 scope global\n", pool); err == nil {
		t.Error("an address outside the RA's pool: accepted")
	}
	delegate := netip.MustParsePrefix(pdDelegatePool)
	solicit := "[t]: info: pppoe0: recv [DHCPv6 Solicit XID=1 <Client-ID 3:0001> <IA-PD 1 T1=0 T2=0 {IA-Prefix ::/56 pref_lifetime=0 valid_lifetime=0}> <Elapsed-Time 0>]"
	request := "[t]: info: pppoe0: recv [DHCPv6 Request XID=2 <IA-PD 1 T1=0 T2=0 {IA-Prefix 2001:db8:2000::/60 pref_lifetime=3600 valid_lifetime=7200}>]"
	reply := "[t]: info: pppoe0: send [DHCPv6 Reply XID=2 <IA-PD 1 T1=1800 T2=2880 {IA-Prefix 2001:db8:2000::/60 pref_lifetime=3600 valid_lifetime=7200}>]"
	if err := delegationVerdict(strings.Join([]string{solicit, request, reply}, "\n"), delegate); err != nil {
		t.Errorf("a full delegation: %v", err)
	}
	if err := delegationVerdict(strings.Join([]string{strings.Replace(solicit, "::/56", "::/64", 1), request, reply}, "\n"), delegate); !errors.Is(err, errNoSolicitHint) {
		t.Errorf("a Solicit hinting /64: err = %v, want errNoSolicitHint", err)
	}
	for name, log := range map[string]string{
		"no Request":    strings.Join([]string{solicit, reply}, "\n"),
		"other Reply":   strings.Join([]string{solicit, request, strings.Replace(reply, "2001:db8:2000::/60", "2001:db8:2010::/60", 1)}, "\n"),
		"wrong request": strings.Join([]string{solicit, strings.Replace(request, "/60", "/56", 1), reply}, "\n"),
	} {
		if err := delegationVerdict(log, delegate); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
