# 163 — Hub Phase 6: GR Plugin

## Objective

Complete the GR plugin by creating `ze-gr.yang` (augmenting ze-bgp), removing graceful-restart from ze-bgp.yang, and wiring GR capability injection and peer event handling.

## Decisions

- GR config is delivered to GR plugin as JSON (hub format) but GR sends capabilities back as text commands (`capability hex 64 0078 peer 192.168.1.1`). Config input format changed (text→JSON); capability output format stayed the same.
- `restart-time` valid range is 0-4095 (12-bit field in RFC 4724 capability encoding). Values above 4095 are encoding-invalid.
- GR YANG augments `/bgp:bgp/bgp:peer/bgp:capability` and `/bgp:bgp/bgp:peer-group/bgp:capability` — per-peer and per-group granularity.
- Implementation Summary left blank — actual implementation tracked in 157-hub-separation-phases.

## Patterns

- Capability injection flow: GR receives config JSON → encodes RFC 4724 capability (code 64, restart-time in lower 12 bits of 2-byte value) → sends `capability hex 64 <value> peer <addr>` → BGP stores for OPEN messages.

## Gotchas

- `restart-time` upper bound is 4095 (not 65535). The field is 12 bits in the RFC 4724 capability encoding, not the full 16-bit YANG uint16. YANG typedef must reflect this.

## Files

- `yang/ze-gr.yang` — GR YANG module augmenting ze-bgp
- `yang/ze-bgp.yang` — removed graceful-restart leaf
- `internal/component/plugin/gr/gr.go` — updated for hub integration
