// Design: docs/guide/bfd.md -- the operator surface this fixture drives
// Related: plugin_fixture_03_bgp.go -- the non-strict bgp-bfd-opt-in fixture
//
// The driver behind test/plugin/bgp-bfd-strict.ci. It starts ze with a peer
// whose bfd container carries `strict true`, and waits for the two log lines
// that say the whole path worked: the BFD plugin published its Service, and the
// BGP peer opened its BFD session with strict recorded on it.
//
// The peer's remote address never completes a TCP handshake, so BGP never
// reaches Established. That is the point: draft-ietf-idr-bgp-bfd-strict-mode
// Section 7 says the BFD session opens "prior to the BGP FSM starting", so a
// strict peer that never connects must still open one, and a log line proving
// it could not be produced by the old on-Established path.
package fixture

import (
	"context"
	"time"
)

const bgpBFDStrictConfig03 = `environment {
}

bfd {
	enabled true;
	profile fast {
		detect-multiplier 3
		desired-min-tx-us 50000
		required-min-rx-us 50000
	}
}

bgp {
	peer peer1 {
		connection {
			local {
				ip 127.0.0.1
				accept false
			}
			remote {
				ip 127.0.0.254
			}
			bfd {
				enabled true
				mode single-hop
				profile fast
				strict true
				hold-time 45
				hold-down 250
			}
		}
		session {
			asn {
				local 65001
				remote 65002
			}
			router-id 10.0.0.1
			family {
				ipv4/unicast { prefix { maximum 10000; } }
			}
		}
	}
}
`

// logBFDStrictSessionOpened is the line Peer.startBFDClient writes, with the
// strict attribute this fixture is here to see
// (internal/component/bgp/reactor/peer_bfd.go).
const logBFDStrictSessionOpened = "strict=true"

// logRejectInvalidBFD is the parser's refusal of a bfd block. Every bgp-bfd
// fixture rejects it, because a config that failed to parse would otherwise
// look like a peer that simply never got a BFD session.
const logRejectInvalidBFD = "invalid bfd"

func bgpBFDStrict03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bgpBFDStrictConfig03,
		[]string{logBFDStarting, logBFDConfigured, logBFDRunning, "bfd session opened for peer", logBFDStrictSessionOpened},
		[]string{"unknown key strict", "unknown key hold-time", logRejectInvalidBFD, "out of range"},
		20*time.Second, map[string]string{envLogBFD: logLevelDebug, envLogBGP: logLevelInfo})
}

// bgpBFDStrictPinnedConfig03 is the round-2 blocker's own configuration: a
// top-level `bfd { single-hop-session ... }` pinned entry to the SAME neighbor
// a strict BGP peer names.
//
// The pinned session is created at plugin configure time, before any peer runs,
// so the peer's EnsureSession finds an existing key and only bumps its
// refcount. The `local` leaf is what makes that true: api.SessionRequest.Key
// includes Local, so a pinned entry omitting it builds a different key and the
// engine holds two separate sessions. Until Subscribe delivered the current state, the peer seeded Down
// and waited for a transition that a stable session never makes.
const bgpBFDStrictPinnedConfig03 = `environment {
}

bfd {
	enabled true;
	profile fast {
		detect-multiplier 3
		desired-min-tx-us 50000
		required-min-rx-us 50000
	}
	single-hop-session 127.0.0.254 {
		local 127.0.0.1
		profile fast
	}
}

bgp {
	peer peer1 {
		connection {
			local {
				ip 127.0.0.1
				accept false
			}
			remote {
				ip 127.0.0.254
			}
			bfd {
				enabled true
				mode single-hop
				profile fast
				strict true
				hold-time 45
				hold-down 250
			}
		}
		session {
			asn {
				local 65001
				remote 65002
			}
			router-id 10.0.0.1
			family {
				ipv4/unicast { prefix { maximum 10000; } }
			}
		}
	}
}
`

func bgpBFDStrictPinned03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bgpBFDStrictPinnedConfig03,
		[]string{logBFDStarting, logBFDConfigured, logBFDRunning, "bfd session opened for peer", logBFDStrictSessionOpened},
		[]string{"unknown key strict", "unknown key hold-down", logRejectInvalidBFD, "out of range",
			"bfd service delivered no initial state"},
		20*time.Second, map[string]string{envLogBFD: logLevelDebug, envLogBGP: logLevelDebug})
}
