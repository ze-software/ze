// Design: docs/guide/traffic-control.md -- tc rate admission registration

//go:build linux

package trafficnetlink

import "github.com/ze-software/ze/internal/component/traffic"

func init() {
	if err := traffic.RegisterVerifier("tc", verifyDesired); err != nil {
		// Only duplicate names fail here. Registrations run at startup and
		// cannot be supplied by configuration or a network peer.
		panic("BUG: duplicate tc verifier registration: " + err.Error())
	}
}
