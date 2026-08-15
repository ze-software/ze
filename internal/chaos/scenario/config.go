// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

package scenario

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
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
	MCPPort   int    // When >0, add environment { mcp { port N; } } block.
}

// GenerateConfig produces a Ze configuration string from the given parameters.
// The output is a valid Ze config file that can be passed to `ze bgp server`.
func GenerateConfig(params ConfigParams) string {
	var b textbuf.Buffer

	zeBin := params.ZeBinary
	if zeBin == "" {
		zeBin = "ze"
	}

	// Route reflector plugin — required for route forwarding between peers.
	// In-process mode adds plugins via CLI args to LoadReactorWithPlugins,
	// so emitting an external plugin block would create a duplicate that
	// tries to fork a subprocess and fails.
	if !params.NoPlugin {
		fmt.Fprintf(&b, "plugin {\n")              //nolint:errcheck // config output
		fmt.Fprintf(&b, "    external bgp-rs {\n") //nolint:errcheck // config output
		if params.PprofAddr != "" {
			fmt.Fprintf(&b, "        run \"ze.bgp-rs\";\n") //nolint:errcheck // config output
		} else {
			fmt.Fprintf(&b, "        run \"%s plugin bgp-rs\";\n", zeBin) //nolint:errcheck // config output
		}
		fmt.Fprintf(&b, "    }\n")                  //nolint:errcheck // config output
		fmt.Fprintf(&b, "    external bgp-rib {\n") //nolint:errcheck // config output
		if params.PprofAddr != "" {
			fmt.Fprintf(&b, "        run \"ze.bgp-rib\";\n") //nolint:errcheck // config output
		} else {
			fmt.Fprintf(&b, "        run \"%s plugin bgp-rib\";\n", zeBin) //nolint:errcheck // config output
		}
		fmt.Fprintf(&b, "    }\n") //nolint:errcheck // config output
		fmt.Fprintf(&b, "}\n\n")   //nolint:errcheck // config output
	}

	// Environment block — debug settings, SSH, web UI, looking glass.
	hasEnv := params.PprofAddr != "" || params.SSHPort > 0 || params.WebUIPort > 0 || params.LGPort > 0 || params.MCPPort > 0
	if hasEnv {
		fmt.Fprintf(&b, "environment {\n") //nolint:errcheck // buffer output
		if params.PprofAddr != "" {
			fmt.Fprintf(&b, "    debug {\n")                         //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        pprof %s;\n", params.PprofAddr) //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "    }\n")                               //nolint:errcheck // buffer output
		}
		if params.SSHPort > 0 {
			fmt.Fprintf(&b, "    ssh {\n")                            //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        enabled true;\n")                //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        server main {\n")                //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "            ip 127.0.0.1;\n")            //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "            port %d;\n", params.SSHPort) //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        }\n")                            //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "    }\n")                                //nolint:errcheck // buffer output
		}
		if params.WebUIPort > 0 {
			fmt.Fprintf(&b, "    web {\n")                              //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        enabled true;\n")                  //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        server main {\n")                  //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "            ip 127.0.0.1;\n")              //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "            port %d;\n", params.WebUIPort) //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        }\n")                              //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        insecure true;\n")                 //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "    }\n")                                  //nolint:errcheck // buffer output
		}
		if params.LGPort > 0 {
			fmt.Fprintf(&b, "    looking-glass {\n")                 //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        enabled true;\n")               //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        server main {\n")               //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "            ip 127.0.0.1;\n")           //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "            port %d;\n", params.LGPort) //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        }\n")                           //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "    }\n")                               //nolint:errcheck // buffer output
		}
		if params.MCPPort > 0 {
			fmt.Fprintf(&b, "    mcp {\n")                        //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "        port %d;\n", params.MCPPort) //nolint:errcheck // buffer output
			fmt.Fprintf(&b, "    }\n")                            //nolint:errcheck // buffer output
		}
		fmt.Fprintf(&b, "}\n\n") //nolint:errcheck // buffer output
	}

	// SSH authentication — test user with bcrypt-hashed "test" password.
	if params.SSHPort > 0 {
		fmt.Fprintf(&b, "system {\n")             //nolint:errcheck // buffer output
		fmt.Fprintf(&b, "    authentication {\n") //nolint:errcheck // buffer output
		fmt.Fprintf(&b, "        user test {\n")  //nolint:errcheck // buffer output
		// bcrypt hash of "test" at cost 10.
		fmt.Fprintf(&b, "            password \"$2a$10$4A3D3GHd7l3FZXyL/YgH4.bWB2G1oHD1IXgyUDClqIThEcPEJY8Sq\";\n") //nolint:errcheck // buffer output
		fmt.Fprintf(&b, "        }\n")                                                                              //nolint:errcheck // buffer output
		fmt.Fprintf(&b, "    }\n")                                                                                  //nolint:errcheck // buffer output
		fmt.Fprintf(&b, "}\n\n")                                                                                    //nolint:errcheck // buffer output
	}

	// bgp block with all peer definitions.
	fmt.Fprintf(&b, "bgp {\n") //nolint:errcheck // buffer output
	for i := range params.Profiles {
		writeFullPeerBlock(&b, params, params.Profiles[i])
	}
	fmt.Fprintf(&b, "}\n") //nolint:errcheck // buffer output

	return b.String()
}

// PeerSummary returns a compact one-line-per-peer summary for stderr display.
func PeerSummary(params ConfigParams) string {
	var b textbuf.Buffer
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

		fmt.Fprintf(&b, "  peer %d  %s  local-as=%-5d remote-as=%-5d  %s  hold=%-3d%s  families=[%s]  routes=%d%s\n", //nolint:errcheck // buffer output
			p.Index, peerAddr, params.LocalAS, p.ASN, peerType, p.HoldTime, portInfo,
			textbuf.Join(families, ", "), p.RouteCount, mode)
	}
	return b.String()
}

// writeFullPeerBlock writes a single peer block inside the bgp container.
// This produces valid Ze config syntax.
func writeFullPeerBlock(b *textbuf.Buffer, params ConfigParams, p PeerProfile) {
	peerAddr := params.LocalAddr
	if p.Address.IsValid() {
		peerAddr = p.Address.String()
	}
	fmt.Fprintf(b, "    peer chaos-peer-%d {\n", p.Index)               //nolint:errcheck // output
	fmt.Fprintf(b, "        description \"chaos-peer-%d\";\n", p.Index) //nolint:errcheck // output

	// Connection container — transport-level settings.
	// All chaos peers are passive from Ze's perspective: Ze never dials out.
	// This avoids needing loopback aliases for the fake peer addresses.
	fmt.Fprintf(b, "        connection {\n")                     //nolint:errcheck // output
	fmt.Fprintf(b, "            remote {\n")                     //nolint:errcheck // output
	fmt.Fprintf(b, "                ip %s;\n", peerAddr)         //nolint:errcheck // output
	fmt.Fprintf(b, "                connect false;\n")           //nolint:errcheck // output
	fmt.Fprintf(b, "            }\n")                            //nolint:errcheck // output
	fmt.Fprintf(b, "            local {\n")                      //nolint:errcheck // output
	fmt.Fprintf(b, "                ip %s;\n", params.LocalAddr) //nolint:errcheck // output
	if p.ZePort > 0 {
		fmt.Fprintf(b, "                port %d;\n", p.ZePort) //nolint:errcheck // output
	}
	fmt.Fprintf(b, "            }\n") //nolint:errcheck // output
	fmt.Fprintf(b, "        }\n")     //nolint:errcheck // output

	// Session container — BGP session settings.
	fmt.Fprintf(b, "        session {\n")                          //nolint:errcheck // output
	fmt.Fprintf(b, "            asn {\n")                          //nolint:errcheck // output
	fmt.Fprintf(b, "                remote %d;\n", p.ASN)          //nolint:errcheck // output
	fmt.Fprintf(b, "                local %d;\n", params.LocalAS)  //nolint:errcheck // output
	fmt.Fprintf(b, "            }\n")                              //nolint:errcheck // output
	fmt.Fprintf(b, "            router-id %s;\n", params.RouterID) //nolint:errcheck // output

	// Family block — per-peer families from profile.
	families := p.Families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}
	// Prefix maximum: 10% headroom over route count, minimum 10000.
	maxPrefix := max(p.RouteCount+p.RouteCount/10, 10000)
	fmt.Fprintf(b, "            family {\n") //nolint:errcheck // output
	for _, f := range families {
		fmt.Fprintf(b, "                %s { prefix { maximum %d; } }\n", f, maxPrefix) //nolint:errcheck // output
	}
	fmt.Fprintf(b, "            }\n") //nolint:errcheck // output
	fmt.Fprintf(b, "        }\n")     //nolint:errcheck // output

	fmt.Fprintf(b, "        timer { receive-hold-time %d; }\n", p.HoldTime) //nolint:errcheck // output

	// Attach container — this peer's relationship to each plugin the config runs.
	// A peer that attaches no process is fed nothing and may announce nothing
	// (pluginserver.Server.PeerScopedProcs, reactor/send_permission.go), so route
	// forwarding stops without these blocks. Each receive list restates that
	// plugin's own startup subscription, the mapping
	// reactor.TestEveryStartupSubscriptionIsExpressible holds. bgp-rs takes
	// UPDATEs in the RECEIVED direction alone: granting the sent direction as
	// well is the delivery loop bgp/plugins/rr/rr.go describes.
	b.Str("        attach process bgp-rs {\n").
		Str("            receive [ update-received state open-received refresh ];\n").
		Str("            send [ update ];\n").
		Str("        }\n")
	// bgp-rib runs only when the plugin block declares it. In-process mode loads
	// bgp-rs alone, so attaching the RIB there would name no running program.
	if !params.NoPlugin {
		b.Str("        attach process bgp-rib {\n").
			Str("            receive [ update state refresh ];\n").
			Str("            send [ update ];\n").
			Str("        }\n")
	}

	b.Str("    }\n")
}
