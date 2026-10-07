# Packet Capture and Decode

## Meta

| Field | Value |
|-------|-------|
| Name | Packet Capture and Decode |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/pcap, internal/plugins/diag/cmd/pcap.go, internal/plugins/diag/cmd/capture_raw.go, internal/component/bgp/cli/decode_pcap.go |
| Real-path tests | test/ui/bgp-decode-pcap-file.ci, test/ui/bgp-decode-pcap-stdin.ci, test/ui/bgp-decode-pcap-no-bgp.ci |
| Docs | docs/guide/production-diagnostics.md |
| Doc review | 2026-10-07: LinkTypeRaw header and the fabricated port 179 read in internal/plugins/diag/cmd/pcap.go; the empty started list on no provider is stated in the row |
| Defect review | 2026-10-07: plan/journal/silent-fall-through.md (2026-10-07) captureRawStart answers started:[] with established sessions, open |
| Extra criteria | supported: a real daemon starts BGP capture, dumps pcap and decodes it with ze bgp decode pcap = test/plugin/capture-raw-bgp-roundtrip.ci |

## Description

`show capture raw action start protocol bgp` records whole BGP messages in memory, and `show capture raw action dump protocol bgp format pcap` writes them as a pcap under `LINKTYPE_RAW` (101) with a synthetic IP and TCP header around each message, so Wireshark dissects the file as BGP. The 19-byte BGP header is rebuilt at the tap, sequence numbers advance per direction, checksums are computed, and IPv6 peers are written in their own family. A truncated message declares its on-wire length, so a reader shows truncation rather than a malformed packet. Reading back: `ze bgp decode pcap <file>` decodes every BGP message in a capture ze wrote or tcpdump wrote, reassembling each TCP direction first; `ze bgp decode pcap -` and `ze bgp decode -` take a capture and hex lines from standard input. A hole in the capture is reported and never read across, and a file with no BGP in it exits non-zero naming what it examined. The TCP ports are fabricated as 179 at both ends, because reading the real pair costs two mutexes on the session read path, so a ze-written capture is a readable reconstruction rather than a packet-level record. When no BGP capture provider can start, the start command answers with an empty `started` list rather than refusing. <!-- source: internal/core/pcap/ -- the pcap file format, framing and reassembly --> <!-- source: internal/plugins/diag/cmd/pcap.go -- exportBGPPcap, exportBFDPcap --> <!-- source: internal/plugins/diag/cmd/capture_raw.go -- captureRawStart --> <!-- source: internal/component/bgp/cli/decode_pcap.go -- decodePcapInput, frameMessages -->
