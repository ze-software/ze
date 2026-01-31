package bgp

import (
	"io"

	"codeberg.org/thomas-mangin/ze/internal/plugin/rib"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// cmdPluginRib runs the RIB (Routing Information Base) plugin.
//
// CLI Mode:
//
//	ze bgp plugin rib --features                   # list supported features
//
// Engine Mode (no flags, no args): Full plugin with startup protocol.
func cmdPluginRib(args []string) int {
	return RunPlugin(PluginConfig{
		Name:         "rib",
		Features:     "yang",
		SupportsNLRI: false,
		SupportsCapa: false,
		GetYANG:      func() string { return ribYANG },
		ConfigLogger: func(level string) {
			rib.SetLogger(slogutil.PluginLogger("rib", level))
		},
		RunEngine: func(in io.Reader, out io.Writer) int {
			return rib.NewRIBManager(in, out).Run()
		},
	}, args)
}

// ribYANG is the YANG schema for the RIB plugin.
const ribYANG = `module ze-rib {
    namespace "urn:ze:rib";
    prefix rib;

    import ze-bgp { prefix ze-bgp; }

    description
        "RIB (Routing Information Base) plugin for Ze.
         Tracks Adj-RIB-In (routes from peers) and Adj-RIB-Out (routes to peers).
         Supports ADD-PATH (RFC 7911) with per-path-id storage.";

    revision 2025-01-31 {
        description "Initial revision.";
    }

    augment "/ze-bgp:bgp" {
        container rib {
            description "RIB plugin state and operations.";
            config false;

            container adj-rib-in {
                description "Routes received from peers.";

                list peer {
                    key "address";
                    description "Per-peer Adj-RIB-In.";

                    leaf address {
                        type string;
                        description "Peer IP address.";
                    }

                    leaf route-count {
                        type uint32;
                        description "Number of routes from this peer.";
                    }
                }
            }

            container adj-rib-out {
                description "Routes sent to peers.";

                list peer {
                    key "address";
                    description "Per-peer Adj-RIB-Out.";

                    leaf address {
                        type string;
                        description "Peer IP address.";
                    }

                    leaf route-count {
                        type uint32;
                        description "Number of routes to this peer.";
                    }
                }
            }
        }
    }
}
`
