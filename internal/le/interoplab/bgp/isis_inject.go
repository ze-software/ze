// Design: docs/architecture/testing/interop.md -- native own-LSP purge injection.
package bgp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	isisPurgePDULength  = 27
	isisChecksumOffset  = 12
	isisFletcherModulus = 255
	isisClaimedSequence = 4096
	isisPurgeInterface  = "eth0"
)

type isisPurgeSender func(pid int, interfaceName string, pdu []byte) error

func injectISISOwnLSPPurge(ctx context.Context, lab interoplab.CheckerLab, send isisPurgeSender) error {
	pid, err := lab.PeerPID(ctx, "ze")
	if err != nil {
		return fmt.Errorf("resolve Ze network namespace: %w", err)
	}
	pdu := buildISISL1Purge([6]byte{0, 0, 0, 0, 0, 2}, isisClaimedSequence, 0, 0)
	if err := send(pid, isisPurgeInterface, pdu[:]); err != nil {
		return fmt.Errorf("inject own-LSP purge: %w", err)
	}
	return nil
}

// buildISISL1Purge encodes ISO/IEC 10589 clauses 9.5 and 9.8. Remaining
// Lifetime is zero, so the header-only LSP is a purge.
func buildISISL1Purge(systemID [6]byte, sequence uint32, pseudonode, lspNumber byte) [isisPurgePDULength]byte {
	pdu := [isisPurgePDULength]byte{
		0x83, isisPurgePDULength, 0x01, 0x06, 0x12, 0x01, 0x00, 0x00,
	}
	binary.BigEndian.PutUint16(pdu[8:10], isisPurgePDULength)
	// pdu[10:12] is Remaining Lifetime zero.
	copy(pdu[12:18], systemID[:])
	pdu[18] = pseudonode
	pdu[19] = lspNumber
	binary.BigEndian.PutUint32(pdu[20:24], sequence)
	pdu[26] = 0x01 // Level-1 IS type block.
	high, low := isisFletcher(pdu[12:], isisChecksumOffset)
	pdu[24] = high
	pdu[25] = low
	return pdu
}

func isisFletcher(region []byte, checksumOffset int) (byte, byte) {
	if checksumOffset < 0 || checksumOffset+1 >= len(region) {
		panic("BUG: IS-IS checksum offset lies outside the PDU")
	}
	c0, c1 := 0, 0
	for index, octet := range region {
		if index == checksumOffset || index == checksumOffset+1 {
			octet = 0
		}
		c0 = (c0 + int(octet)) % isisFletcherModulus
		c1 = (c1 + c0) % isisFletcherModulus
	}
	m := len(region) - checksumOffset
	x := ((m-1)*c0 - c1) % isisFletcherModulus
	if x < 0 {
		x += isisFletcherModulus
	}
	y := (c1 - m*c0) % isisFletcherModulus
	if y < 0 {
		y += isisFletcherModulus
	}
	if x == 0 {
		x = isisFletcherModulus
	}
	if y == 0 {
		y = isisFletcherModulus
	}
	return byte(x), byte(y)
}
func buildISISEthernetFrame(source net.HardwareAddr, pdu []byte) ([]byte, error) {
	if len(source) != 6 {
		return nil, errors.New("IS-IS Ethernet source must contain six octets")
	}
	if len(pdu) != isisPurgePDULength {
		return nil, fmt.Errorf("IS-IS purge PDU is %d octets, expected %d", len(pdu), isisPurgePDULength)
	}
	const (
		ethernetHeaderLength = 14
		llcLength            = 3
	)
	payloadLength := llcLength + len(pdu)
	if payloadLength >= 0x0600 {
		return nil, fmt.Errorf("IS-IS 802.3 payload length %d collides with an EtherType", payloadLength)
	}
	frame := make([]byte, ethernetHeaderLength+payloadLength)
	copy(frame[0:6], []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14})
	copy(frame[6:12], source)
	binary.BigEndian.PutUint16(frame[12:14], uint16(payloadLength))
	copy(frame[14:17], []byte{0xfe, 0xfe, 0x03})
	copy(frame[17:], pdu)
	return frame, nil
}

func joinISISInjectionError(resultErr error, operation string, err error) error {
	if err == nil {
		return resultErr
	}
	return errors.Join(resultErr, fmt.Errorf("%s: %w", operation, err))
}
