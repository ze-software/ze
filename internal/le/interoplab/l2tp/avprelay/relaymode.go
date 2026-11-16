// Design: docs/labs/l2tp-interop.md -- isolated private relay modes.
// Related: main.go -- one bounded socket loop for every registered mode.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// relayIO is owned by the single socket loop. Mode callbacks MUST finish
// before the next callback; neither relayIO nor the callbacks are concurrent.
type relayIO struct {
	conn     *net.UDPConn
	encoder  *json.Encoder
	ze       netip.AddrPort
	lac      netip.AddrPort
	deadline time.Time
}

type relayDatagram func([]byte, string, netip.AddrPort) error

// relayMode is a complete value declaration. A private mode owns its control
// policy, data policy and read deadline, while main owns the socket loop.
type relayMode struct {
	controlLimit int
	control      relayDatagram
	data         relayDatagram
	beforeRead   func(time.Time) (time.Time, error)
}

var relayModes = make(map[string]func(*relayIO) relayMode)

func init() {
	registerRelayMode("", defaultRelayMode)
}

func registerRelayMode(name string, factory func(*relayIO) relayMode) {
	if _, exists := relayModes[name]; exists {
		panic("BUG: duplicate private relay mode")
	}
	relayModes[name] = factory
}

func selectRelayMode(name string, transport *relayIO) (relayMode, error) {
	factory, exists := relayModes[name]
	if !exists {
		return relayMode{}, fmt.Errorf("unknown private relay mode %q", name)
	}
	return factory(transport), nil
}

func defaultRelayMode(transport *relayIO) relayMode {
	var injected injection
	return relayMode{
		controlLimit: 256,
		control: func(packet []byte, direction string, target netip.AddrPort) error {
			original := hex.EncodeToString(packet)
			if direction == directionLAC {
				mandatory, err := mandatoryArmed()
				if err != nil {
					return err
				}
				// The common loop MUST reserve six spare bytes for inject.
				packet = inject(packet, mandatory)
			}
			if _, err := transport.conn.WriteToUDPAddrPort(packet, target); err != nil {
				return err
			}
			return encodeWire(transport.encoder,
				wireObservation{direction, original, hex.EncodeToString(packet)}, injected.restart)
		},
		data: func(packet []byte, direction string, target netip.AddrPort) error {
			return relayData(transport.conn, transport.encoder, packet, direction, target, &injected)
		},
		beforeRead: func(time.Time) (time.Time, error) { return transport.deadline, nil },
	}
}
