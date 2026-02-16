package scenario

import (
	"fmt"
	"net/netip"
	"strings"
)

// ConfigParams holds inputs for generating a Ze configuration file.
type ConfigParams struct {
	LocalAS   uint32
	RouterID  netip.Addr
	LocalAddr string
	BasePort  int
	Profiles  []PeerProfile
}

// GenerateConfig produces a Ze configuration string from the given parameters.
// The output is a valid Ze config file that can be passed to `ze bgp server`.
func GenerateConfig(params ConfigParams) string {
	var b strings.Builder

	// bgp block with all peer definitions.
	fmt.Fprintf(&b, "bgp {\n")
	for _, p := range params.Profiles {
		writePeerBlock(&b, params, p)
	}
	fmt.Fprintf(&b, "}\n")

	return b.String()
}

// writePeerBlock writes a single peer block inside the bgp container.
func writePeerBlock(b *strings.Builder, params ConfigParams, p PeerProfile) {
	peerAddr := params.LocalAddr
	if p.Address.IsValid() {
		peerAddr = p.Address.String()
	}
	fmt.Fprintf(b, "    peer %s {\n", peerAddr)
	fmt.Fprintf(b, "        description \"chaos-peer-%d\";\n", p.Index)
	fmt.Fprintf(b, "        router-id %s;\n", params.RouterID)
	fmt.Fprintf(b, "        local-address %s;\n", params.LocalAddr)
	fmt.Fprintf(b, "        local-as %d;\n", params.LocalAS)
	fmt.Fprintf(b, "        peer-as %d;\n", p.ASN)
	fmt.Fprintf(b, "        hold-time %d;\n", p.HoldTime)

	if p.Mode == ModePassive {
		fmt.Fprintf(b, "        passive true;\n")
	}

	// Family block — per-peer families from profile.
	families := p.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}
	fmt.Fprintf(b, "        family {\n")
	for _, f := range families {
		fmt.Fprintf(b, "            %s;\n", f)
	}
	fmt.Fprintf(b, "        }\n")

	fmt.Fprintf(b, "    }\n")
}
