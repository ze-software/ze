// Design: docs/guide/traffic-control.md -- kernel traffic control rate admission registration

//go:build linux

package trafficnetlink

import "github.com/ze-software/ze/internal/component/traffic"

func init() {
	if err := traffic.RegisterVerifier(backendName, verifyDesired); err != nil {
		// Only duplicate names fail here. Registrations run at startup and
		// cannot be supplied by configuration or a network peer.
		panic("BUG: duplicate kernel traffic verifier registration: " + err.Error())
	}
}
