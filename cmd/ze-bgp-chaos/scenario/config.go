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

	// Process block for route-reflector plugin.
	fmt.Fprintf(&b, "process route-reflector {\n")
	fmt.Fprintf(&b, "    run \"ze.bgp-rr\";\n")
	fmt.Fprintf(&b, "    encoder json;\n")
	fmt.Fprintf(&b, "}\n\n")

	// One neighbor block per profile.
	for _, p := range params.Profiles {
		writePeerBlock(&b, params, p)
		b.WriteString("\n")
	}

	return b.String()
}

// writePeerBlock writes a single neighbor block to the builder.
func writePeerBlock(b *strings.Builder, params ConfigParams, p PeerProfile) {
	fmt.Fprintf(b, "neighbor %s {\n", params.LocalAddr)
	fmt.Fprintf(b, "    description \"chaos-peer-%d\";\n", p.Index)
	fmt.Fprintf(b, "    router-id %s;\n", params.RouterID)
	fmt.Fprintf(b, "    local-address %s;\n", params.LocalAddr)
	fmt.Fprintf(b, "    local-as %d;\n", params.LocalAS)
	fmt.Fprintf(b, "    peer-as %d;\n", p.ASN)
	fmt.Fprintf(b, "    hold-time %d;\n", p.HoldTime)

	if p.Mode == ModePassive {
		fmt.Fprintf(b, "    passive true;\n")
	}

	// Family block — per-peer families from profile.
	families := p.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}
	fmt.Fprintf(b, "\n    family {\n")
	for _, f := range families {
		// Convert "ipv4/unicast" → "ipv4 unicast" for config syntax.
		fmt.Fprintf(b, "        %s;\n", strings.ReplaceAll(f, "/", " "))
	}
	fmt.Fprintf(b, "    }\n")

	// API block for route-reflector process.
	fmt.Fprintf(b, "\n    api {\n")
	fmt.Fprintf(b, "        processes [ route-reflector ];\n")
	fmt.Fprintf(b, "    }\n")

	fmt.Fprintf(b, "}\n")
}
