// Design: docs/guide/bfd.md -- Authenticate the BFD session of an EBGP peer; Session sharing
// Related: plugin_fixture_03_bgp_bfd_strict.go -- the strict and pinned fixtures these follow
//
// The drivers behind test/plugin/bgp-bfd-auth-ebgp-warn.ci and
// test/plugin/bgp-bfd-auth-join-refused.ci. Each starts ze with one config and
// waits for the log lines that prove the behavior through the daemon: the
// RFC 5882 Section 10.2 commit warning for an EBGP peer whose BFD profile has
// no auth block, and the refusal of a BGP peer whose authenticated profile
// would join a session built without authentication.
package fixture

import (
	"context"
	"time"
)

// bgpBFDAuthEBGPWarnConfig03 runs an EBGP peer (AS 65001 to AS 65002) whose
// bfd container names the profile "fast", which carries no auth block. The
// commit is accepted, as the SHOULD permits, and BGP's commit check logs the
// warning with the fix that names the profile.
const bgpBFDAuthEBGPWarnConfig03 = `environment {
}
bfd {
	enabled true;
	profile fast {
		detect-multiplier 3
	}
}
bgp {
	router-id 203.0.113.253;
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

// logBFDAuthEBGPWarn is the warning verifyPeerBFDProfiles writes
// (internal/component/bgp/reactor/peer_bfd.go).
const logBFDAuthEBGPWarn = "bfd profile of an EBGP peer has no authentication; RFC 5882 Section 10.2 recommends BFD authentication" //nolint:gosec // G101: a log line naming missing authentication, no credential

// logBFDAuthEBGPFix is the start of that warning's fix attribute, with the
// profile name the peer configured.
const logBFDAuthEBGPFix = `fix="add auth { type keyed-sha1; key-id <0-255>; secret <secret>; } under bfd { profile fast { } }`

func bgpBFDAuthEBGPWarn03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bgpBFDAuthEBGPWarnConfig03,
		[]string{logBFDRunning, logBFDAuthEBGPWarn, logBFDAuthEBGPFix, "peer=127.0.0.254", "profile=fast"},
		[]string{logRejectInvalidBFD, "is not defined under bfd"},
		20*time.Second, map[string]string{envLogBFD: logLevelDebug, envLogBGP: logLevelDebug})
}

// bgpBFDAuthJoinRefusedConfig03 pins a single-hop session to 127.0.0.254 from
// 127.0.0.1 under the profile "open", which carries no auth block, and runs a
// strict BGP peer to the same neighbor from the same local address under the
// profile "secure", which configures Keyed SHA1. The pinned session is created
// at plugin configure time, before any peer runs, so the peer's EnsureSession
// finds that session's key and asks to join it with a different
// authentication. The join is refused: one session carries one authentication
// configuration (RFC 5882 Section 10.2, RFC 5880 Section 6.7).
const bgpBFDAuthJoinRefusedConfig03 = `environment {
}
bfd {
	enabled true;
	profile open {
		detect-multiplier 3
	}
	profile secure {
		detect-multiplier 3
		auth {
			type keyed-sha1
			key-id 7
			secret "JOIN-SECRET"
		}
	}
	single-hop-session 127.0.0.254 {
		local 127.0.0.1
		profile open
	}
}
bgp {
	router-id 203.0.113.253;
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
				profile secure
				strict true
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

// The refused client's log line carries the refusal joinAuthCheck builds
// (internal/component/bfd/engine/auth_join.go, authMismatchCase) for a live
// session that runs no authentication: what differs, why the client is
// refused, and the one fix, which edits no profile another client may name:
// name profile open, which the session was built from and which still holds
// no auth block (so the bfd plugin's profile lookup reached the engine), or
// another profile with none. The three spans stop short of the quoted profile
// names, which the log handler escapes, and together cover every word after
// the refusal's peer prefix but those names.
const (
	logBFDAuthJoinRefused = "BFD authentication differs from the live session's: " +
		"the session runs none, bfd profile "
	logBFDAuthJoinRefusedWhy = " has an auth block; one session per neighbor runs one authentication " +
		"(RFC 5882 Section 4.4), so this client gets no BFD session; " +
		"to fix it, name bfd profile "
	logBFDAuthJoinRefusedFix = " on this client, or another bfd profile with no auth block"
)

func bgpBFDAuthJoinRefused03(ctx context.Context, _ []string) error {
	return runZeUntilLogsRejecting03(ctx, bgpBFDAuthJoinRefusedConfig03,
		[]string{logBFDRunning, logBFDPinnedSessionCreated, "bfd strict-mode EnsureSession failed; peer is held down",
			logBFDAuthJoinRefused, logBFDAuthJoinRefusedWhy, logBFDAuthJoinRefusedFix},
		[]string{logRejectInvalidBFD, "bfd session shared", logBFDSessionOpenedForPeer},
		20*time.Second, map[string]string{envLogBFD: logLevelDebug, envLogBGP: logLevelDebug})
}
