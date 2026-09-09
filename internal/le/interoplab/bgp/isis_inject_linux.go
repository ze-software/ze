//go:build linux

// Design: docs/architecture/testing/interop.md -- namespace-safe AF_PACKET injection.
package bgp

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// injectISISPurgeHost builds the purge frame after entering ze's own network
// namespace, because the frame's source address is ze's peer interface's own
// hardware address, readable only from inside that namespace.
func injectISISPurgeHost(pid int, interfaceName string, pdu []byte) error {
	if len(pdu) != isisPurgePDULength {
		return fmt.Errorf("IS-IS purge PDU is %d octets, expected %d", len(pdu), isisPurgePDULength)
	}
	return interoplab.SendFrameInNamespace(pid, interfaceName, func(link *net.Interface) ([]byte, error) {
		return buildISISEthernetFrame(link.HardwareAddr, pdu)
	})
}
