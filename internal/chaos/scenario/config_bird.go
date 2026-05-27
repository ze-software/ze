// Design: docs/architecture/chaos-web-dashboard.md — multi-target config generation

package scenario

import "codeberg.org/thomas-mangin/ze/internal/core/textbuf"

// zeFamilyToBIRDChannel maps Ze family strings to BIRD 2 channel names.
// BIRD 2 has no separate multicast channel type; multicast families
// are skipped (they would cause a BIRD config parse error).
var zeFamilyToBIRDChannel = map[string]string{
	"ipv4/unicast": "ipv4",
	"ipv6/unicast": "ipv6",
}

// GenerateBIRDConfig produces a BIRD 2 bird.conf from the given parameters.
// The BGP listen port is embedded in the config (BIRD has no CLI flag for it).
func GenerateBIRDConfig(params ConfigParams) string {
	var b textbuf.Buffer
	b.Reset(4096)

	b.Str("log stderr all;\n")
	b.Str("router id ").Addr(params.RouterID).Str(";\n")
	b.Str("listen bgp address ").Str(params.LocalAddr).Str(" port ").Int(int64(params.BasePort)).Str(";\n")
	b.Str("\n")
	b.Str("protocol device {}\n")
	b.Str("\n")

	for i := range params.Profiles {
		writeBIRDPeer(&b, params, params.Profiles[i])
	}

	return b.String()
}

func writeBIRDPeer(b *textbuf.Buffer, params ConfigParams, p PeerProfile) {
	b.Str("protocol bgp chaos_peer_").Int(int64(p.Index)).Str(" {\n")
	b.Str("  description \"chaos-peer-").Int(int64(p.Index)).Str("\";\n")
	b.Str("  local as ").Uint32(params.LocalAS).Str(";\n")
	b.Str("  neighbor ").Str(p.Address.String()).Str(" as ").Uint32(p.ASN).Str(";\n")
	b.Str("  hold time ").Uint16(p.HoldTime).Str(";\n")
	b.Str("  passive;\n")
	b.Str("  rs client;\n")

	families := p.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}
	for _, fam := range families {
		ch, ok := zeFamilyToBIRDChannel[fam]
		if !ok {
			continue
		}
		b.Str("  ").Str(ch).Str(" {\n")
		b.Str("    import all;\n")
		b.Str("    export all;\n")
		b.Str("  };\n")
	}

	b.Str("}\n\n")
}
