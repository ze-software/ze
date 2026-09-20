// Design: docs/architecture/core-design.md -- TCP MD5 authentication (RFC 2385)
// Related: md5_freebsd.go -- the platform predicate this pins
//
// VALIDATES: FreeBSD does not claim that a password configured on ze takes
// effect, because setTCPMD5Sig installs no key there (the kernel reads the
// Security Association Database, which setkey(8) fills outside ze).

//go:build freebsd

package network

import "testing"

// TestTCPMD5NotClaimedSupportedOnFreeBSD: the predicate both warning surfaces
// read -- parsePeer (internal/component/bgp/reactor/config.go) and
// doctorCheckBGPMD5 (internal/component/bgp/config/doctor_checks.go) -- answers
// false, so an operator who configures a password on FreeBSD is told that ze
// installs no key. A true answer made both warnings unreachable while the
// segments went unsigned (plan/journal/silent-fall-through.md, 2026-09-01).
func TestTCPMD5NotClaimedSupportedOnFreeBSD(t *testing.T) {
	if TCPMD5Supported() {
		t.Error("TCPMD5Supported() = true on FreeBSD, where setTCPMD5Sig installs no key; " +
			"either write the SAD entry from ze, or keep the answer false")
	}
}
