# BFD Liveness Detection

## Meta

| Field | Value |
|-------|-------|
| Name | BFD Liveness Detection |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 5880 and RFC 5881 obligations the ledger marks Partial, Echo mode transport half (spec-bfd-6b-echo-transport) |
| Level | experimental |
| Components | internal/component/bfd, internal/core/bgp/capability |
| Real-path tests | test/bfd/bfd-detection-interval.ci, test/bfd/bfd-first-packet-pktinfo.ci, test/bfd/bfd-first-packet-pktinfo-v6.ci |
| Interop | bgp/bfd-frr, bgp/bfd-simple-password-bird, bgp/bgp-bfd-strict-frr |
| RFCs | rfc5880, rfc5881, rfc5883 |
| Docs | docs/guide/bfd.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; UDP 3784 single-hop and 4784 multi-hop match internal/component/bfd/api/events.go; Loop.Snapshot exists in internal/component/bfd/engine/snapshot.go |
| Defect review | 2026-10-07: plan/pre-release/spec-rfc-verdict-fix-bfd.md open (ledger verdict repair); no plan/immediate spec names internal/component/bfd |

## Description

RFC 5880 Bidirectional Forwarding Detection plugin: pinned single-hop (UDP 3784) and multi-hop (UDP 4784) sessions, profile-driven timer bundles, GTSM enforcement (IP_TTL=255 outbound / IP_RECVTTL ingress gate), multi-hop min-TTL floor, RFC 5880 §6.8.7 TX jitter (0-25%, clamped to [10%, 25%) when detect-multiplier=1), SO_BINDTODEVICE for single-hop interface and multi-VRF binding, BGP peer opt-in with RFC 9384 Cease subcode 10 teardown, BFD strict mode (draft-ietf-idr-bgp-bfd-strict-mode: capability 74 negotiated in OPEN, the BFD session opened before the BGP FSM starts, the BGP session held in OpenSent until BFD is Up, the BfdHoldTimer for a negotiated hold time of zero, and FSM events 30 to 35), <!-- source: internal/core/bgp/capability/capability.go -- BFDStrictMode --> <!-- source: internal/component/bgp/reactor/session_bfd_strict.go -- advanceAfterOpen, handleBFDEvent --> `show bfd sessions/session/profile` commands, `ze_bfd_*` Prometheus metrics, RFC 5880 §6.7 Keyed SHA1/MD5 (meticulous variants included) authentication with file-backed sequence-number persistence, and RFC 5880 §6.4 Echo mode config/wire advertisement (transport half tracked as spec-bfd-6b-echo-transport). <!-- source: internal/component/bfd/engine/loop.go -- passesTTLGate, applyJitter --> <!-- source: internal/component/bfd/transport/udp_linux.go -- applySocketOptions, parseReceivedTTL --> <!-- source: internal/component/bfd/bfd.go -- runtimeState, resolveLoopDevices, newUDPTransport --> <!-- source: internal/component/bfd/engine/snapshot.go -- Loop.Snapshot, SessionDetail --> <!-- source: internal/component/bfd/metrics.go -- bindMetricsRegistry, metricsHook --> Compile-out-able with the `ze_bfd` build tag (default-on in `ZE_FEATURES`): a stripped build drops the engine, session, transport, auth, and command surface and rejects a `bfd {}` block, while the nil-able client seam (`bfd/api`, with the `bfd/packet` State/Diag types) stays always-on so BGP, OSPF, and static route monitors run without BFD by their existing warn-and-degrade path. <!-- source: feature-gates.txt -- ze_bfd --> <!-- source: internal/component/bfd/api/registry.go -- nil-able GetService seam -->
