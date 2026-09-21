# Spec: LDP loop detection (Path Vector and Hop Count)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 5036 Section 3.4.5 calls this "the optional LDP Loop Detection mechanism". Ze offers no configuration for it (`register.go::parseLDPConfig` reads no loop-detection leaf), so it advertises no Path Vector Limit and runs none of the Path Vector procedures. The Hop Count check of Section 3.4.4.1 is implemented and proven (RFC5036-3.4.4.1-1, -2); the rows here are the Path-Vector-driven procedures that only run once Loop Detection is configured.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5036-2.8.1-1` [MUST] "If configured for Loop Detection, the Label Request message MUST include a Hop Count TLV (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.1-2` [MUST] "If configured for Loop Detection and R is sending the Label Request because it is a FEC ingress, it MUST include a Hop Count TLV with hop count value 1 (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.1-3` [MUST] "If configured for Loop Detection and R is sending the Label Request as a result of having received one from an upstream LSR that contains a Hop Count TLV, R MUST increment the received hop count value by 1 and MUST pass the resulting value in a Hop Count TLV to its next hop along with the Label Request message (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.1-4` [MUST] "If configured for Loop Detection and R is sending the Label Request because it is a FEC ingress, then if R is non-merge capable it MUST include a Path Vector TLV of length 1 containing its own LSR Id (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.1-5` [MUST] "If configured for Loop Detection, R MUST add its own LSR Id to the Path Vector and MUST pass the resulting Path Vector to its next hop along with the Label Request message (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.1-6` [MUST] "If configured for Loop Detection and the Label Request contains no Path Vector TLV, R MUST include a Path Vector TLV of length 1 containing its own LSR Id (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.1-7` [MUST] "If configured for Loop Detection, when R detects a loop it MUST send a Loop Detected Notification message to the source of the Label Request message and drop the Label Request message (§2.8.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-1` [MUST] "If configured for Loop Detection, R MUST include a Hop Count TLV in the Label Mapping message (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-2` [MUST] "If configured for Loop Detection and R is the egress, the hop count value MUST be 1 (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-3` [MUST] "If configured for Loop Detection and the Label Mapping message propagates one received from the next hop to an upstream peer, the hop count value MUST be determined by the two rules that follow it (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-4` [MUST] "If configured for Loop Detection and R is a member of the edge set of an LSR domain whose LSRs do not perform TTL-decrement and the upstream peer is within that domain, R MUST reset the hop count to 1 before propagating the message (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-5` [MUST] "If configured for Loop Detection and the preceding case does not hold, R MUST increment the hop count received from the next hop before propagating the message (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-6` [MUST] "If configured for Loop Detection and the Label Mapping message is not propagating a received one, the hop count value MUST be the result of incrementing R's current knowledge of the hop count learned from previous Label Mapping messages (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-7` [MUST] "If configured for Loop Detection, R is merge capable, the message propagates one received from the next hop, and R has not previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-8` [MUST] "If configured for Loop Detection and the received message contains an unknown hop count, then R MUST include a Path Vector TLV (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-9` [MUST] "If configured for Loop Detection and R has previously sent a Label Mapping message to the upstream peer, then it MUST include a Path Vector TLV if the received message reports an LSP hop count increase, a change in hop count from unknown to known, or a change from known to unknown (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-10` [MUST] "If configured for Loop Detection and the received Label Mapping message included a Path Vector, the Path Vector sent upstream MUST be the result of adding R's LSR Id to the received Path Vector (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-11` [MUST] "If configured for Loop Detection and the received message had no Path Vector, the Path Vector sent upstream MUST be a Path Vector of length 1 containing R's LSR Id (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-12` [MUST] "If configured for Loop Detection and the Label Mapping message is not propagating a received message upstream, the message MUST include a Path Vector of length 1 containing R's LSR Id (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-2.8.2-13` [MUST] "If configured for Loop Detection, when R detects a loop it MUST stop using the label for forwarding, drop the Label Mapping message, and signal Loop Detected status to the source of the Label Mapping message (§2.8.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.4.1-3` [MUST] "If Loop Detection is configured, the LSR MUST follow the procedures specified in the Loop Detection section (§3.4.4.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.5.1.1-1` [MUST] "An LSR that receives a Path Vector in a Label Request message MUST perform the Loop Detection procedures (§3.4.5.1.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.5.1.1-2` [MUST] "If the LSR detects a loop in a received Label Request message, it MUST reject that Label Request message (§3.4.5.1.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.5.1.1-3` [MUST] "On rejecting a looping Label Request the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.1)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.5.1.2-1` [MUST] "An LSR that receives a Path Vector in a Label Mapping message MUST perform the Loop Detection procedures (§3.4.5.1.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.5.1.2-2` [MUST] "If the LSR detects a loop in a received Label Mapping message, it MUST reject that Label Mapping message in order to prevent a forwarding loop (§3.4.5.1.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
- `RFC5036-3.4.5.1.2-3` [MUST] "On rejecting a looping Label Mapping the LSR MUST perform the steps the section lists: send a Notification message signaling Loop Detected to the LSR that sent the message, and abandon processing of it (§3.4.5.1.2)" -- no loop-detection configuration or Path Vector procedure exists in `register.go`/`session.go`
