// Design: docs/architecture/diagnostics/packet-capture.md -- BGP pcap framing
// Overview: capture_raw.go -- the dispatcher that calls these exporters
// Related: internal/core/pcap -- the one owner of the pcap file format

package cmd

import (
	"bytes"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/pcap"
)

// captureTimeLayout is how a capture entry's timestamp crosses the plugin
// boundary. The reactor writes it and the exporters read it back.
const captureTimeLayout = "2006-01-02T15:04:05Z07:00"

// bgpPort is the TCP port a BGP session uses (RFC 4271 Section 3). Both ends of
// an exported flow carry it, because the real ephemeral port is not on the
// capture path: reading it costs two mutexes on the session read path, so the
// export fabricates the pair and the packet-capture page says so.
const bgpPort = 179

// The directions a capture entry carries, as the reactor spells them.
const (
	captureDirIn  = "in"
	captureDirOut = "out"
)

// The snapshot lengths the two raw exports declare.
const (
	// bgpSnapLen covers the largest message RFC 8654 allows, so a framed
	// record's original length never exceeds what the file header states.
	bgpSnapLen = 65535
	// bfdSnapLen is the BFD ring's slot size, unchanged since before the BGP
	// export gained framing. The BFD file is byte-identical to what it was.
	bfdSnapLen = 4096
)

// exportBGPPcap writes the BGP raw capture as a pcap under LINKTYPE_RAW, with
// each whole BGP message inside a synthetic IP and TCP header. The framing is
// what makes the link type true and what lets Wireshark and `ze bgp decode
// pcap` read the file as BGP.
//
// Entries arrive newest-first, and a pcap reads oldest-first, so the walk runs
// backwards. That order is also what makes the per-direction sequence numbers
// advance the way a reader expects.
func exportBGPPcap(entries []plugin.BGPRawCaptureEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := pcap.WriteFileHeader(&buf, bgpSnapLen, pcap.LinkTypeRaw); err != nil {
		return nil, err
	}

	framer := pcap.NewFramer()
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		ts, err := time.Parse(captureTimeLayout, e.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("capture entry %d carries an unparsable timestamp %q: %w", i, e.Timestamp, err)
		}
		flow, err := bgpFlow(e)
		if err != nil {
			return nil, fmt.Errorf("capture entry %d: %w", i, err)
		}
		if err := framer.WriteMessage(&buf, ts, flow, e.Data, e.OriginalLen); err != nil {
			return nil, fmt.Errorf("capture entry %d: %w", i, err)
		}
	}
	return buf.Bytes(), nil
}

// bgpFlow turns one capture entry into the direction its message traveled.
// A received message runs from the peer to this host, and a sent one the other
// way.
//
// A message the reactor could not match to a peer carries no local address. The
// flow then names the unspecified address of the peer's family, which reads as
// "this host, not known" rather than as some other host's traffic.
func bgpFlow(e *plugin.BGPRawCaptureEntry) (pcap.Flow, error) {
	peer, err := netip.ParseAddr(e.PeerAddr)
	if err != nil {
		return pcap.Flow{}, fmt.Errorf("peer address %q: %w", e.PeerAddr, err)
	}

	local, err := netip.ParseAddr(e.LocalAddr)
	if err != nil || local.Is4() != peer.Is4() {
		local = netip.IPv6Unspecified()
		if peer.Is4() {
			local = netip.AddrFrom4([4]byte{})
		}
	}

	source, target := peer, local
	if e.Direction == captureDirOut {
		source, target = local, peer
	}
	return pcap.Flow{
		SourceAddr: source,
		TargetAddr: target,
		SourcePort: bgpPort,
		TargetPort: bgpPort,
	}, nil
}

// exportBFDPcap writes the BFD raw capture as a pcap. BFD is a UDP payload, so
// its records carry the captured bytes with no framing added. It has its own
// exporter because the BGP one grew TCP and port-179 headers, and a shared
// exporter with a protocol switch is how a correct BFD capture would silently
// have become malformed.
func exportBFDPcap(entries []plugin.BGPRawCaptureEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := pcap.WriteFileHeader(&buf, bfdSnapLen, pcap.LinkTypeRaw); err != nil {
		return nil, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		ts, err := time.Parse(captureTimeLayout, e.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("capture entry %d carries an unparsable timestamp %q: %w", i, e.Timestamp, err)
		}
		if err := pcap.WriteRecord(&buf, ts, e.Data, len(e.Data)); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
