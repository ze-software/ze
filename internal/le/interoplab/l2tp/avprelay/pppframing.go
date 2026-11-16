// Design: docs/labs/l2tp-interop.md -- complete PPP capture and compressed LCP input.
// Related: pppinject.go -- the existing strict header reader and framed probes.
// RFC 1661 Section 6.6 -- see rfc/short/rfc1661.md.
package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

const compressedMarker = "/run/compressed-lcp"
const compressedIdentifier = 0x6f
const compressedData = "ff03-receive"

// framingCapture belongs to the relay's sequential loop and is not safe for
// concurrent use. It records only capture order and whether the requested
// compressed experiment has already sent its single probe.
type framingCapture struct {
	ordinal        int
	compressedSent bool
}

// normalDataObservation adds completeness and HEAD's marker-phase witness
// without changing the base observation or independent restart extensions.
type normalDataObservation struct {
	wireObservation
	DataOrdinal     int  `json:"data_ordinal"`
	CodeRejectArmed bool `json:"code_reject_armed"`
}

// observe records every forwarded normal datagram and the marker state that
// its caller sampled at HEAD's original logging gate. The caller forwards
// unchanged before sampling, logging, or injecting.
func (capture *framingCapture) observe(encoder *json.Encoder, packet []byte, direction string, armed bool) error {
	capture.ordinal++
	encoded := hex.EncodeToString(packet)
	return encoder.Encode(normalDataObservation{wireObservation{direction, encoded, encoded}, capture.ordinal, armed})
}

// injectCompressed sends exactly one Echo-Request without Address/Control
// after the checker marker exists. It preserves the live L2TP header and
// acknowledged peer Magic-Number; no ordinary forwarded frame is modified.
// RFC 1661 Section 6.6: "If a compressed frame is received when Address-and-Control-Field-Compression has not been negotiated, the implementation MAY silently discard the frame."
func (capture *framingCapture) injectCompressed(conn *net.UDPConn, encoder *json.Encoder, trigger []byte, target netip.AddrPort) error {
	if capture.compressedSent {
		return nil
	}
	marker, err := os.ReadFile(compressedMarker)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	value := strings.TrimSpace(string(marker))
	if len(value) != 8 {
		return errors.New("compressed LCP marker must contain the acknowledged peer Magic-Number as eight hex digits")
	}
	magic, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return err
	}
	start, err := dataHeaderLength(trigger)
	if err != nil {
		return err
	}
	frame := compressedEchoFrame(trigger[:start], uint32(magic))
	if _, err := conn.WriteToUDPAddrPort(frame, target); err != nil {
		return err
	}
	capture.compressedSent = true
	return encoder.Encode(wireObservation{"inject-compressed-lcp", "", hex.EncodeToString(frame)})
}

// compressedEchoFrame constructs the intentionally violating peer packet. The
// output starts at the two-octet PPP Protocol, with no ff 03 prefix.
func compressedEchoFrame(header []byte, magic uint32) []byte {
	start := len(header)
	frame := make([]byte, start+2+8+len(compressedData))
	copy(frame, header)
	if binary.BigEndian.Uint16(frame)&flagLength != 0 {
		binary.BigEndian.PutUint16(frame[2:], uint16(len(frame)))
	}
	binary.BigEndian.PutUint16(frame[start:], protocolLCP)
	lcp := frame[start+2:]
	lcp[0], lcp[1] = 9, compressedIdentifier
	binary.BigEndian.PutUint16(lcp[2:], uint16(len(lcp)))
	binary.BigEndian.PutUint32(lcp[4:], magic)
	copy(lcp[8:], compressedData)
	return frame
}
