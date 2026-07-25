// Design: docs/architecture/chaos-web-dashboard.md — multi-target config generation

package scenario

import "github.com/ze-software/ze/internal/core/textbuf"

// zeFamilyToBIRDChannel maps Ze family strings to BIRD 2 channel names.
var zeFamilyToBIRDChannel = map[string]string{
	"ipv4/unicast":   "ipv4",
	"ipv6/unicast":   "ipv6",
	"ipv4/multicast": "ipv4 multicast",
	"ipv6/multicast": "ipv6 multicast",
}

// birdMulticastTable maps multicast channel names to their required table declarations.
var birdMulticastTable = map[string]string{
	"ipv4 multicast": "mrib4",
	"ipv6 multicast": "mrib6",
}

// GenerateBIRDConfig produces a BIRD 2 bird.conf from the given parameters.
// The BGP listen port is set per-protocol via `local <addr> port <port> as <asn>`.
func GenerateBIRDConfig(params ConfigParams) string {
	var b textbuf.Buffer
	b.Reset(4096)

	b.Str("log stderr all;\n")
	b.Str("router id ").Addr(params.RouterID).Str(";\n")
	b.Str("\n")
	b.Str("protocol device {}\n")

	// Declare multicast routing tables if any peer uses multicast families.
	tables := collectBIRDTables(params.Profiles)
	for _, tbl := range tables {
		b.Str("\n").Str(tbl.typ).Str(" table ").Str(tbl.name).Str(";\n")
	}
	b.Str("\n")

	for i := range params.Profiles {
		writeBIRDPeer(&b, params, params.Profiles[i])
	}

	return b.String()
}

func writeBIRDPeer(b *textbuf.Buffer, params ConfigParams, p PeerProfile) {
	b.Str("protocol bgp chaos_peer_").Int(int64(p.Index)).Str(" {\n")
	b.Str("  description \"chaos-peer-").Int(int64(p.Index)).Str("\";\n")
	b.Str("  local ").Str(params.LocalAddr).Str(" port ").Int(int64(params.BasePort)).Str(" as ").Uint32(params.LocalAS).Str(";\n")
	b.Str("  neighbor ").Str(p.Address.String()).Str(" as ").Uint32(p.ASN).Str(";\n")
	b.Str("  hold time ").Uint16(p.HoldTime).Str(";\n")
	b.Str("  passive;\n")
	if p.ASN == params.LocalAS {
		b.Str("  rr client;\n")
		b.Str("  rr cluster id ").Addr(params.RouterID).Str(";\n")
	} else {
		b.Str("  rs client;\n")
	}

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
		tbl, isMcast := birdMulticastTable[ch]
		if isMcast {
			b.Str("    table ").Str(tbl).Str(";\n")
		}
		b.Str("    import all;\n")
		b.Str("    export all;\n")
		b.Str("  };\n")
	}

	b.Str("}\n\n")
}

type birdTable struct {
	typ  string
	name string
}

// collectBIRDTables returns the multicast routing tables needed by any profile.
func collectBIRDTables(profiles []PeerProfile) []birdTable {
	seen := make(map[string]bool)
	var tables []birdTable
	for i := range profiles {
		families := profiles[i].Families
		if len(families) == 0 {
			families = []string{"ipv4/unicast"}
		}
		for _, fam := range families {
			ch, ok := zeFamilyToBIRDChannel[fam]
			if !ok {
				continue
			}
			tbl, isMcast := birdMulticastTable[ch]
			if isMcast && !seen[tbl] {
				seen[tbl] = true
				typ := "ipv4"
				if ch == "ipv6 multicast" {
					typ = "ipv6"
				}
				tables = append(tables, birdTable{typ: typ, name: tbl})
			}
		}
	}
	return tables
}
