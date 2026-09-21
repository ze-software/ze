# Spec: LDP capability error status codes (RFC 5561)

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

When ze reads a Capability Parameter it does not support, RFC 5561 obliges an Unsupported Capability Notification (status 0x0000002E, E-bit 0) carrying a Returned TLVs TLV, and a Malformed TLV Value Notification for a duplicated Capability Parameter. Ze parses no Capability Parameter (`wire.go::DecodeInit` skips every non-Common-Session TLV), so it detects neither condition, and `session.go::sendNotification` carries only the two RFC 5036 status codes. The Unknown TLV path (RFC5036-3.3-1) is now implemented; this spec is the capability-specific status codes and the Returned TLVs TLV those Notifications carry.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5561-3-5` [MUST] "When more than one instance of the same Capability Parameter type is received in a message, "The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message" (§3)" -- no Unsupported Capability or Malformed TLV Value status constant exists, and `encodeNotification` writes exactly one Status TLV with no Returned TLVs TLV
- `RFC5561-6-2` [MUST] "When a speaker terminates a session over a capability the peer did not advertise, or over an unsupported capability whose U-bit is 0, "The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability" (§6)" -- no Unsupported Capability or Malformed TLV Value status constant exists, and `encodeNotification` writes exactly one Status TLV with no Returned TLVs TLV
- `RFC5561-8-1` [MUST] ""The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0" (§8, Unsupported Capability)" -- no Unsupported Capability or Malformed TLV Value status constant exists, and `encodeNotification` writes exactly one Status TLV with no Returned TLVs TLV
- `RFC5561-8-2` [MUST] ""When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV" (§8)" -- no Unsupported Capability or Malformed TLV Value status constant exists, and `encodeNotification` writes exactly one Status TLV with no Returned TLVs TLV
- `RFC5561-8-3` [MUST] ""The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer" (§8)" -- no Unsupported Capability or Malformed TLV Value status constant exists, and `encodeNotification` writes exactly one Status TLV with no Returned TLVs TLV
- `RFC5561-8-4` [MUST] ""When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV" (§8)" -- no Unsupported Capability or Malformed TLV Value status constant exists, and `encodeNotification` writes exactly one Status TLV with no Returned TLVs TLV
