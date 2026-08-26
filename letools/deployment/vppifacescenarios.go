// Design: docs/architecture/testing/interop.md -- what one VPP feature proof is
// Overview: vppiface.go -- the run that drives this table
//
// vppifacescenarios.go contains the DATA for the four interface features. Each
// row defines the ze configuration, the vppctl query that checks whether VPP
// created the object and the words that a reader gets back.
//
// This is a table because only the data differs between the four features. Each
// scenario writes a configuration, starts ze on it, polls one VPP query and
// matches a needle. A fifth feature is another row in this table.
//
// LCP must stay off in three of the four configurations. When it is enabled, ze
// creates an lcp_itf_pair for every loopback. This fails the whole apply on a
// VPP build without linux_cp_plugin.so. This is an honest exact-or-reject at the
// binapi layer. Only the LCP scenario turns it on, and only after its plugin
// probe has passed.

package deployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// VPPStartupConfig is what VPP itself is started with. Every plugin except dpdk
// loads. Thus, wireguard and linux-cp are present whenever the image ships them.
// dpdk is disabled because it claims physical devices, and the container has
// none.
const VPPStartupConfig = `unix {
  nodaemon
  cli-listen /run/vpp/cli.sock
  log /run/vpp/vpp.log
}

api-segment {
  prefix vpp
}

socksvr {
  socket-name /run/vpp/api.sock
}

plugins {
  plugin dpdk_plugin.so { disable }
}

statseg {
  socket-name /run/vpp/stats.sock
}
`

// The two ends of the GRE tunnel the first scenario asks for. They are stated
// here rather than left inside a configuration blob because the probe looks for
// them in VPP's own answer.
const (
	GRELocal  = "10.10.10.1"
	GRERemote = "10.10.10.2"
)

// The ze VPP block, with LCP off and with it on. The daemon talks to the VPP
// already running in the container, so the backend is external and every socket
// is the one VPPStartupConfig created.
const (
	vppBase = `vpp {
    enabled true;
    external true;
    api-socket /run/vpp/api.sock;
    stats { socket-path /run/vpp/stats.sock; }
    lcp { enabled false; netns host; }
    plugins { wireguard true; }
}
`
	vppBaseLCP = `vpp {
    enabled true;
    external true;
    api-socket /run/vpp/api.sock;
    stats { socket-path /run/vpp/stats.sock; }
    lcp { enabled true; netns host; }
    plugins { wireguard true; }
}
`
)

// The interface block each scenario adds under the VPP block above.
const (
	greBlock = `interface {
    backend vpp;
    tunnel gre0 {
        encapsulation {
            gre {
                local { ip 10.10.10.1; }
                remote { ip 10.10.10.2; }
            }
        }
    }
}
`
	mirrorBlock = `interface {
    backend vpp;
    dummy mdst0 { }
    dummy msrc0 {
        unit default { mirror { ingress mdst0; } }
    }
}
`
	// The private key is a valid 32-byte base64 Curve25519 value and protects
	// nothing: the container is torn down at the end of the run.
	wireguardBlock = `interface {
    backend vpp;
    wireguard wg0 {
        listen-port 51820;
        private-key "aHZkc2ZqaHZra2hkZnZoamtkc2Zoa2RoaGRma2poZmg=";
    }
}
`
	lcpBlock = `interface {
    backend vpp;
    dummy loop0 {
        unit default { address 10.99.0.1/32; }
    }
}
`
)

// joined answers one string built from parts. It uses textbuf instead of `+`.
// ai/rules/performance.md bans string construction by concatenation. Every
// sentence and configuration in this file names a constant that must have one
// spelling.
func joined(parts ...string) string {
	var tb textbuf.Buffer
	for _, part := range parts {
		tb.Str(part)
	}
	return tb.String()
}

// vppScenario is one interface feature, proven end to end.
//
// probe is the query polled until ScenarioWait. If the first query never
// matches, fallback is a second query that the run asks once. This supports a
// feature whose object VPP names differently between releases. A scenario with
// no fallback leaves it empty.
//
// hostLinks says that the scenario reports the container's Linux link listing
// as evidence. The LCP pair's whole point is that a TAP appears in a Linux
// netns. The listing is the second half of that proof. The Python wrote it to
// standard error, where no operator was able to pipe it.
type vppScenario struct {
	feature         string
	file            string
	config          string
	needsPlugins    []string
	skipDetail      string
	probe           string
	needles         []string
	fallback        string
	fallbackNeedles []string
	provenDetail    string
	missingDetail   string
	hostLinks       bool
}

// vppScenarios is the table in the order that the run performs it. The order is
// the Python's, and it matters. The four scenarios share one VPP daemon. A
// feature that the run was unable to program leaves that daemon in a state that the
// next scenario would use for its verdict.
var vppScenarios = []vppScenario{
	{
		feature: "gre-tunnel",
		file:    "tunnel.conf",
		config:  joined(vppBase, greBlock),
		probe:   "show gre tunnel",
		needles: []string{GRERemote, GRELocal},
		// A VPP release that names the object differently still lists the
		// interface, so the generic listing is asked once before the scenario
		// is called failed.
		fallback:        "show interface",
		fallbackNeedles: []string{"gre"},
		provenDetail:    joined("real VPP created a GRE tunnel ", GRELocal, " -> ", GRERemote),
		missingDetail:   "GRE tunnel not observed on real VPP",
	},
	{
		feature: "span-mirror",
		file:    "mirror.conf",
		config:  joined(vppBase, mirrorBlock),
		probe:   "show interface span",
		// A configured SPAN entry names the source interface and a direction;
		// an empty table has neither.
		needles:       []string{"rx", "both"},
		provenDetail:  "real VPP programmed a SPAN mirror (msrc0 rx -> mdst0)",
		missingDetail: "SPAN mirror not observed on real VPP",
	},
	{
		feature:      "wireguard",
		file:         "wg.conf",
		config:       joined(vppBase, wireguardBlock),
		needsPlugins: []string{WireguardPlugin},
		skipDetail: joined(WireguardPlugin, " not loaded in this image; ze rejects at apply",
			" and doctor-vpp-wireguard flags it (unit-tested). Image limit recorded."),
		probe:         "show wireguard interface",
		needles:       []string{"51820", "wg"},
		provenDetail:  "real VPP created a wireguard interface (listen-port 51820)",
		missingDetail: "wireguard interface not observed on real VPP",
	},
	{
		feature:      "lcp-pair",
		file:         "lcp.conf",
		config:       joined(vppBaseLCP, lcpBlock),
		needsPlugins: []string{LinuxCPPlugin, LinuxNLPlugin},
		skipDetail: joined(LinuxCPPlugin, " / ", LinuxNLPlugin, " not loaded in this image;",
			" LCP pair creation is unit-tested and doctor-vpp-lcp-netns covers the",
			" netns constraint. Image limit recorded."),
		probe:         "show lcp",
		needles:       []string{"loop0", "tap"},
		provenDetail:  "real VPP created an LCP pair shadowing loop0",
		missingDetail: "LCP pair not observed on real VPP",
		hostLinks:     true,
	},
}
