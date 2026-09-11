# draft-abraitis-idr-addpath-paths-limit - PATHS-LIMIT for ADD-PATH

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-abraitis-idr-addpath-paths-limit |
| Title | Scalability Considerations for ADD-PATH with PATHS-LIMIT |
| Status | Internet Draft |
| Date | 2024 |
| Depends | RFC 7911 (ADD-PATH) |
| Enrolment | enrolled |
| Enrolment reason | BGP ADD-PATH Paths-Limit capability (code 76): four MUST-level requirements with positive and negative test carriers. For 3-1, coalescePathsLimit in internal/component/bgp/reactor/session_negotiate.go merges configuration and plugin declarations; TestBuildOpenCoalescesPathsLimit checks multiple families in one emitted instance and prevents repeated input instances from leaking into OPEN. For 3-2 and 3-3, core capability negotiation tests cover ADD-PATH present/absent and matching/unmatched families; for 3-4, TestParsePathsLimitDuplicateFirstWins covers retaining the first tuple and ignoring later duplicates. SHOULD-level zero handling (3-5) and session-wide sender enforcement (3-6) have separate parser, negotiation, and sender regression carriers. |
| Support | drafts 10 |
| Support area | BGP PATHS-LIMIT |
| Support status | Supported |
| Support coverage | Receiver-advertised per-family path-count requests for ADD-PATH, with session-wide outbound enforcement across normal sends and forwarding, including the route-server fast path. The configuration knob states a request to the peer and does not bound what Ze accepts: a peer that ignores it is not policed. |
| Support remaining | - |

**Purpose:** Extends ADD-PATH (RFC 7911) by letting a receiver advertise the maximum number of paths it wants to receive per prefix, per address family. Prevents uncontrolled path proliferation in ADD-PATH deployments.

**Scope:** Capability (OPEN message), not path attribute (UPDATE). Only meaningful when ADD-PATH is also negotiated for the same family.

## Wire Formats

### PATHS-LIMIT Capability

Capability Code: 76 (0x4C)

```
+------------------------------------------------+
| Address Family Identifier (2 octets)           |
+------------------------------------------------+
| Subsequent Address Family Identifier (1 octet) |
+------------------------------------------------+
| Max Paths (2 octets)                           |
+------------------------------------------------+
```

Variable length: sequence of 5-byte entries. Each entry specifies the maximum number of paths the receiver wants per prefix for one AFI/SAFI.

### Field Semantics

| Field | Size | Range | Semantics |
|-------|------|-------|-----------|
| AFI | 2 bytes | IANA AFI | Address Family |
| SAFI | 1 byte | IANA SAFI | Subsequent Address Family |
| Max Paths | 2 bytes | 0-65535 | 0 = skip/ignore, 1-65535 = path count limit |

## Negotiation

- Receiver-advertised: a speaker advertises how many paths it wants to receive per prefix.
- The remote speaker's PATHS-LIMIT constrains our outgoing path count.
- Our PATHS-LIMIT requests a maximum for the peer's outgoing path count; it is not inbound policing by Ze.
- PATHS-LIMIT entries are only accepted for families also present in the peer's ADD-PATH capability.
- If a family has ADD-PATH negotiated but no PATHS-LIMIT, there is no limit.

## Key Requirements

1. Emit at most one PATHS-LIMIT instance in OPEN, coalescing configuration and plugin declarations; an empty instance communicates no limits.
2. Entries with limit 0 are skipped/ignored during parsing.
3. Duplicate AFI/SAFI entries: first entry wins, duplicates silently ignored.
4. Maximum 51 entries per capability (255 value octets / 5 bytes per entry; the two-octet header is separate).
5. Enforcement is on the sender side: the sender counts paths per prefix and drops excess before transmitting UPDATE messages.

## Ze Implementation

- Capability code: `capability.CodePathsLimit` (76)
- Struct: `capability.PathsLimit` with `[]PathsLimitEntry`
- OPEN producer: `coalescePathsLimit()` in `session_negotiate.go` combines configuration and plugin declarations into one capability before encoding.
- Negotiation: `Negotiate()` processes after ADD-PATH, stores in `EncodingCaps.PathsLimitSend/Recv`
- Context: `EncodingContext.PathsLimit(family)` returns direction-specific limit
- Enforcement: `Session.filterPathsLimit()` and `pathsLimitSection()` share per-connection, per-family, per-prefix admitted-path state across normal UPDATE sends and forwarding. Existing path replacements pass at capacity; withdrawals free slots; a new connection starts empty.
- RS fast-path: advertises PATHS-LIMIT normally and shares the session's outbound admission state for raw and re-encoded forwarding. The deliberate raw-message diagnostic command remains outside normal route admission.
- Config: `session > capability > add-path > limit` sets the default receive request; `family > limit` overrides it for one family. Both leaves accept 1..65535. Omit the family leaf to inherit the default. Disabled/refused families advertise no limit. These knobs request sender behavior; they do not enforce a local receive-side cap.

## Compliance Checklist

- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1] [MUST] A BGP speaker wishing to indicate support for multiple AFI/SAFIs "MUST do so by including the information in a single instance of the PATHS-LIMIT capability" (§3)
- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2] [MUST] "The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present" (§3)
- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3] [MUST] "An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability" (§3)
- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4] [MUST] When more than one tuple is received for the same AFI/SAFI pair, only the first tuple is considered and "All others MUST be ignored" (§3)
- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5] [SHOULD] "If the received Paths Limit is zero (0), the tuple SHOULD be ignored" (§3)
- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6] [SHOULD] "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability" (§3)
- [ ] [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7] [SHOULD] "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender" (§3)
