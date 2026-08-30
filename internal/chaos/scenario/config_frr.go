// Design: docs/architecture/chaos-web-dashboard.md — multi-target config generation

package scenario

import (
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// zeFamilyToFRR maps Ze family strings to FRR address-family names.
var zeFamilyToFRR = map[string]string{
	familyIPv4Unicast:   "ipv4 unicast",
	familyIPv6Unicast:   "ipv6 unicast",
	familyIPv4Multicast: "ipv4 multicast",
	familyIPv6Multicast: "ipv6 multicast",
}

// GenerateFRRConfig produces an FRR bgpd.conf from the given parameters.
// The BGP listen port is controlled via bgpd's -p flag, not in the config.
func GenerateFRRConfig(params ConfigParams) string {
	var b textbuf.Buffer
	b.Reset(4096)

	b.Str("frr defaults traditional\n")
	b.Str("hostname chaos-frr\n")
	b.Str("!\n")
	b.Str("router bgp ").Uint32(params.LocalAS).Byte('\n')
	b.Str(" bgp router-id ").Addr(params.RouterID).Byte('\n')
	b.Str(" no bgp ebgp-requires-policy\n")
	b.Str(" no bgp network import-check\n")

	for i := range params.Profiles {
		writeFRRNeighbor(&b, params.Profiles[i])
	}

	families := collectFamilies(params.Profiles)
	for _, fam := range families {
		frrFam, ok := zeFamilyToFRR[fam]
		if !ok {
			continue
		}
		writeFRRAddressFamily(&b, frrFam, fam, params.Profiles)
	}

	b.Str("!\n")

	return b.String()
}

func writeFRRNeighbor(b *textbuf.Buffer, p PeerProfile) {
	addr := p.Address.String()
	b.Str(" !\n")
	b.Str(" neighbor ").Str(addr).Str(" remote-as ").Uint32(p.ASN).Byte('\n')
	b.Str(" neighbor ").Str(addr).Str(" description chaos-peer-").Int(int64(p.Index)).Byte('\n')
	keepalive := p.HoldTime / 3
	if keepalive == 0 {
		keepalive = 1
	}
	b.Str(" neighbor ").Str(addr).Str(" timers ").Uint16(keepalive).Byte(' ').Uint16(p.HoldTime).Byte('\n')
	b.Str(" neighbor ").Str(addr).Str(" passive\n")
}

func writeFRRAddressFamily(b *textbuf.Buffer, frrFam, zeFam string, profiles []PeerProfile) {
	b.Str(" !\n")
	b.Str(" address-family ").Str(frrFam).Byte('\n')
	for i := range profiles {
		families := profiles[i].Families
		if len(families) == 0 {
			families = []string{familyIPv4Unicast}
		}
		if !hasFamily(families, zeFam) {
			continue
		}
		addr := profiles[i].Address.String()
		b.Str("  neighbor ").Str(addr).Str(" activate\n")
		b.Str("  neighbor ").Str(addr).Str(" route-server-client\n")
	}
	b.Str(" exit-address-family\n")
}

// collectFamilies returns the deduplicated set of families across all profiles,
// in the order they first appear. Profiles with empty Families default to
// ipv4/unicast, matching GenerateConfig and GenerateBIRDConfig behavior.
func collectFamilies(profiles []PeerProfile) []string {
	seen := make(map[string]bool)
	var result []string
	for i := range profiles {
		families := profiles[i].Families
		if len(families) == 0 {
			families = []string{familyIPv4Unicast}
		}
		for _, f := range families {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	return result
}

func hasFamily(families []string, fam string) bool {
	return slices.Contains(families, fam)
}
