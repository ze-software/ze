// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

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
	ZeBinary  string // Path to ze binary for plugin run directives (default: "ze").
	Profiles  []PeerProfile
	NoPlugin  bool   // When true, omit the plugin block (in-process mode adds plugins via CLI args).
	PprofAddr string // When set, inject environment { debug { pprof <addr>; } } into generated config.
	SSHPort   int    // When >0, add system { ssh + authentication } block with test/test user.
	WebUIPort int    // When >0, add environment { web { insecure; } } block.
	LGPort    int    // When >0, add environment { looking-glass { } } block.
}

// GenerateConfig produces a Ze configuration string from the given parameters.
// The output is a valid Ze config file that can be passed to `ze bgp server`.
func GenerateConfig(params ConfigParams) string {
	var b strings.Builder

	zeBin := params.ZeBinary
	if zeBin == "" {
		zeBin = "ze"
	}

	// Route reflector plugin — required for route forwarding between peers.
	// In-process mode adds plugins via CLI args to LoadReactorWithPlugins,
	// so emitting an external plugin block would create a duplicate that
	// tries to fork a subprocess and fails.
	if !params.NoPlugin {
		fmt.Fprintf(&b, "plugin {\n")
		fmt.Fprintf(&b, "    external bgp-rs {\n")
		if params.PprofAddr != "" {
			fmt.Fprintf(&b, "        run \"ze.bgp-rs\";\n")
		} else {
			fmt.Fprintf(&b, "        run \"%s plugin bgp-rs\";\n", zeBin)
		}
		fmt.Fprintf(&b, "    }\n")
		fmt.Fprintf(&b, "    external bgp-rib {\n")
		if params.PprofAddr != "" {
			fmt.Fprintf(&b, "        run \"ze.bgp-rib\";\n")
		} else {
			fmt.Fprintf(&b, "        run \"%s plugin bgp-rib\";\n", zeBin)
		}
		fmt.Fprintf(&b, "    }\n")
		fmt.Fprintf(&b, "}\n\n")
	}

	// Environment block — debug settings, SSH, web UI, looking glass.
	hasEnv := params.PprofAddr != "" || params.SSHPort > 0 || params.WebUIPort > 0 || params.LGPort > 0
	if hasEnv {
		fmt.Fprintf(&b, "environment {\n")
		if params.PprofAddr != "" {
			fmt.Fprintf(&b, "    debug {\n")
			fmt.Fprintf(&b, "        pprof %s;\n", params.PprofAddr)
			fmt.Fprintf(&b, "    }\n")
		}
		if params.SSHPort > 0 {
			fmt.Fprintf(&b, "    ssh {\n")
			fmt.Fprintf(&b, "        enabled true;\n")
			fmt.Fprintf(&b, "        server main {\n")
			fmt.Fprintf(&b, "            ip 127.0.0.1;\n")
			fmt.Fprintf(&b, "            port %d;\n", params.SSHPort)
			fmt.Fprintf(&b, "        }\n")
			fmt.Fprintf(&b, "    }\n")
		}
		if params.WebUIPort > 0 {
			fmt.Fprintf(&b, "    web {\n")
			fmt.Fprintf(&b, "        enabled true;\n")
			fmt.Fprintf(&b, "        server main {\n")
			fmt.Fprintf(&b, "            ip 127.0.0.1;\n")
			fmt.Fprintf(&b, "            port %d;\n", params.WebUIPort)
			fmt.Fprintf(&b, "        }\n")
			fmt.Fprintf(&b, "        insecure true;\n")
			fmt.Fprintf(&b, "    }\n")
		}
		if params.LGPort > 0 {
			fmt.Fprintf(&b, "    looking-glass {\n")
			fmt.Fprintf(&b, "        enabled true;\n")
			fmt.Fprintf(&b, "        server main {\n")
			fmt.Fprintf(&b, "            ip 127.0.0.1;\n")
			fmt.Fprintf(&b, "            port %d;\n", params.LGPort)
			fmt.Fprintf(&b, "        }\n")
			fmt.Fprintf(&b, "    }\n")
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	// SSH authentication — test user with bcrypt-hashed "test" password.
	if params.SSHPort > 0 {
		fmt.Fprintf(&b, "system {\n")
		fmt.Fprintf(&b, "    authentication {\n")
		fmt.Fprintf(&b, "        user test {\n")
		// bcrypt hash of "test" at cost 10.
		fmt.Fprintf(&b, "            password \"$2a$10$4A3D3GHd7l3FZXyL/YgH4.bWB2G1oHD1IXgyUDClqIThEcPEJY8Sq\";\n")
		fmt.Fprintf(&b, "        }\n")
		fmt.Fprintf(&b, "    }\n")
		fmt.Fprintf(&b, "}\n\n")
	}

	// bgp block with all peer definitions.
	fmt.Fprintf(&b, "bgp {\n")
	for i := range params.Profiles {
		writeFullPeerBlock(&b, params, params.Profiles[i])
	}
	fmt.Fprintf(&b, "}\n")

	return b.String()
}

// PeerSummary returns a compact one-line-per-peer summary for stderr display.
func PeerSummary(params ConfigParams) string {
	var b strings.Builder
	for i := range params.Profiles {
		p := &params.Profiles[i]
		peerAddr := params.LocalAddr
		if p.Address.IsValid() {
			peerAddr = p.Address.String()
		}

		peerType := "eBGP"
		if p.IsIBGP {
			peerType = "iBGP"
		}

		families := p.Families
		if len(families) == 0 {
			families = []string{"ipv4/unicast"}
		}

		mode := ""
		if p.Mode == ModePassive {
			mode = " passive"
		}

		portInfo := ""
		if p.ZePort > 0 {
			portInfo = fmt.Sprintf("  port=%-5d", p.ZePort)
		}

		fmt.Fprintf(&b, "  peer %d  %s  local-as=%-5d remote-as=%-5d  %s  hold=%-3d%s  families=[%s]  routes=%d%s\n",
			p.Index, peerAddr, params.LocalAS, p.ASN, peerType, p.HoldTime, portInfo,
			strings.Join(families, ", "), p.RouteCount, mode)
	}
	return b.String()
}

// writeFullPeerBlock writes a single peer block inside the bgp container.
// This produces valid Ze config syntax.
func writeFullPeerBlock(b *strings.Builder, params ConfigParams, p PeerProfile) {
	peerAddr := params.LocalAddr
	if p.Address.IsValid() {
		peerAddr = p.Address.String()
	}
	fmt.Fprintf(b, "    peer chaos-peer-%d {\n", p.Index)
	fmt.Fprintf(b, "        description \"chaos-peer-%d\";\n", p.Index)

	// Connection container — transport-level settings.
	fmt.Fprintf(b, "        connection {\n")
	fmt.Fprintf(b, "            remote {\n")
	fmt.Fprintf(b, "                ip %s;\n", peerAddr)
	fmt.Fprintf(b, "            }\n")
	fmt.Fprintf(b, "            local {\n")
	fmt.Fprintf(b, "                ip %s;\n", params.LocalAddr)
	// All chaos peers are passive from Ze's perspective: Ze never dials out.
	// This avoids needing loopback aliases for the fake peer addresses.
	fmt.Fprintf(b, "                connect false;\n")
	if p.ZePort > 0 {
		fmt.Fprintf(b, "                port %d;\n", p.ZePort)
	}
	fmt.Fprintf(b, "            }\n")
	fmt.Fprintf(b, "        }\n")

	// Session container — BGP session settings.
	fmt.Fprintf(b, "        session {\n")
	fmt.Fprintf(b, "            asn {\n")
	fmt.Fprintf(b, "                remote %d;\n", p.ASN)
	fmt.Fprintf(b, "                local %d;\n", params.LocalAS)
	fmt.Fprintf(b, "            }\n")
	fmt.Fprintf(b, "            router-id %s;\n", params.RouterID)

	// Family block — per-peer families from profile.
	families := p.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}
	// Prefix maximum: 10% headroom over route count, minimum 10000.
	maxPrefix := max(p.RouteCount+p.RouteCount/10, 10000)
	fmt.Fprintf(b, "            family {\n")
	for _, f := range families {
		fmt.Fprintf(b, "                %s { prefix { maximum %d; } }\n", f, maxPrefix)
	}
	fmt.Fprintf(b, "            }\n")
	fmt.Fprintf(b, "        }\n")

	fmt.Fprintf(b, "        timer { receive-hold-time %d; }\n", p.HoldTime)

	fmt.Fprintf(b, "    }\n")
}
