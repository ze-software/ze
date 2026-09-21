# Spec: LDP over label-controlled ATM and Frame Relay links

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

Every obligation here binds "a label-controlled ATM link or a label-controlled Frame Relay link" (RFC 5036 Section 3.5.3). Ze runs LDP over IP links only: `wire.go` encodes no ATM Label (0x0901) or Frame Relay Label (0x0902) TLV and no ATM/Frame Relay Session Parameters TLV, and `handleInit` negotiates the IP session parameters alone.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5036-3.4.2.2-1` [MUST] "The Reserved field of the ATM Label TLV MUST be set to zero on transmission and MUST be ignored on receipt (§3.4.2.2)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.4.2.2-2` [MUST] "If the VCI is less than 16 bits, the preceding bits of the VCI field MUST be set to 0 (§3.4.2.2)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.4.2.2-3` [MUST] "If Virtual Path switching is indicated in the V-bits field, the VCI field MUST be ignored by the receiver and set to 0 by the sender (§3.4.2.2)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.4.2.3-1` [MUST] "The Reserved field of the Frame Relay Label TLV MUST be set to zero on transmission and MUST be ignored on receipt (§3.4.2.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.5.3-2` [MUST] "If the session is for a label-controlled ATM link or a label-controlled Frame Relay link, then Downstream on Demand MUST be used (§3.5.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.5.3-10` [MUST] "A receiving LSR MUST calculate the intersection between the received label range and its own supported label range (§3.5.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.5.3-11` [MUST NOT] "LSRs MUST NOT establish a session with neighbors for which the intersection of label ranges is NULL (§3.5.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.5.3-12` [MUST] "Where the intersection of label ranges is NULL, the LSR MUST send a Session Rejected/Parameters Label Range Notification message in response to the Initialization message and not establish the session (§3.5.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.5.3-13` [MUST] "The Reserved field that precedes the ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (§3.5.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
- `RFC5036-3.5.3-14` [MUST] "When peer LSRs are connected indirectly by means of an ATM VP, the receiving LSR MUST ignore the Minimum and Maximum VPI fields (§3.5.3)" -- no ATM or Frame Relay label range or session parameter is encoded or decoded in `wire.go`
