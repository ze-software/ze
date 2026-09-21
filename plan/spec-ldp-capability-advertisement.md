# Spec: LDP capability advertisement (RFC 5561)

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

RFC 5561 Section 2 makes advertising a capability a MAY: "At session establishment time, an LDP speaker MAY advertise a particular capability by including an optional parameter associated with the capability in its Initialization message." Ze advertises none: `wire.go::EncodeInit` writes only the Common Session Parameters TLV, there is no encoder for the Capability message (type 0x0202), and no Dynamic Capability Announcement constant (0x0506) exists. These rows are the send-side and MUST-NOT constraints that only bind once ze advertises a capability.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5561-3-1` [MUST] "The F-bit of a Capability Parameter TLV "MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded" (§3)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-3-2` [MUST NOT] ""An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message" (§3)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-4-1` [MUST NOT] ""Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages" (§4)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-4-2` [MUST NOT] ""An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message" (§4)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-6-1` [MUST] ""The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt" (§6)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-6-3` [MUST] ""An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document" (§6)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-9-1` [MUST] ""The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message" (§9)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-9-2` [MUST] "There is no Capability Data associated with the Dynamic Capability Announcement TLV "and hence the TLV length MUST be set to 1" (§9)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-9-3` [MUST NOT] ""An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers" (§9)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
- `RFC5561-10-1` [MUST] ""LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well" (§10)" -- no Capability Parameter or Capability message is encoded in `wire.go::EncodeInit`
