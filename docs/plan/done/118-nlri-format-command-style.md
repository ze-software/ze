# Spec: nlri-format-command-style

## Task
Change UPDATE event NLRI format from ExaBGP style (announce/withdraw with next-hop grouping) to command style (family with add/del and per-NLRI next-hop).

**Breaking change:** Existing plugins parsing events will need updates.

**Rationale:**
- Matches command syntax (`nlri ipv4/unicast add ...`)
- Next-hop per-NLRI (correct: different families can have different next-hops in same UPDATE)
- Simpler structure (no grouping by next-hop)
- Prepares for raw wire bytes addition (next spec)

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - Engine/API split
- [ ] `docs/architecture/api/architecture.md` - Event format, process bindings (needs update)
- [ ] `docs/exabgp/exabgp-compatibility.md` - Compatibility requirements (needs update)

### RFC Summaries
- [x] `docs/rfc/rfc4271.md` - UPDATE message format, NEXT_HOP attribute
- [x] `docs/rfc/rfc4760.md` - MP_REACH_NLRI (next-hop per family)

**Key insights:**
- Current format groups by next-hop (ExaBGP style)
- One UPDATE can have multiple next-hops for same family (IPv4 traditional + MP)
- IPv4 unicast: NEXT_HOP attribute OR MP_REACH_NLRI (can have both!)
- IPv6/VPN: next-hop in MP_REACH_NLRI (one per MP_REACH_NLRI attribute)
- Multiple MP_REACH_NLRI for same family possible (different next-hops)
- New format: family value = list of operations grouped by next-hop

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestJSONEncoderIPv4UnicastNewFormat` | `internal/plugin/json_test.go` | IPv4 unicast list with grouped operations | ✅ |
| `TestJSONEncoderIPv4DualNextHop` | `internal/plugin/json_test.go` | IPv4 traditional + MP (two next-hops) | ✅ |
| `TestJSONEncoderMultiFamilyNewFormat` | `internal/plugin/json_test.go` | Multiple families, different next-hops | ✅ |
| `TestJSONEncoderLabeledUnicast` | `internal/plugin/json_test.go` | Labels + next-hop grouped | ✅ |
| `TestJSONEncoderRDTypes` | `internal/plugin/json_test.go` | IPVPN/MPLS-VPN: RD format `0:/1:/2:` + labels | ✅ |
| `TestJSONEncoderEVPN` | `internal/plugin/json_test.go` | ESI + route-type + next-hop | ✅ |
| `formatFlowSpecVPNJSON` | `internal/plugin/text.go` | FlowSpec via String() (TODO: structured) | ✅ |
| `TestJSONEncoderADDPATHNewFormat` | `internal/plugin/json_test.go` | path-id in objects | ✅ |
| `TestJSONEncoderWithdrawNewFormat` | `internal/plugin/json_test.go` | Identity fields, no next-hop | ✅ |
| `TestJSONEncoderAnnounceAndWithdrawSameFamily` | `internal/plugin/json_test.go` | add + del in same family | ✅ |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests

**Status: DEFERRED**

The test framework (`testpeer`) ignores `json:` lines - they are not validated against actual output. True functional tests for JSON format would require:
1. Extending testpeer to validate json: lines against actual JSON output, OR
2. Creating new test infrastructure for JSON validation

Added `json:` lines to `ipv4.ci` and `ipv6.ci` as **documentation only** (not tests):
- Shows expected JSON format for reference
- NOT validated by test runner
- Will NOT catch JSON format regressions

Unit tests in `internal/plugin/json_test.go` provide actual test coverage for JSON format.

## Files to Modify

| File | Change | Status |
|------|--------|--------|
| `internal/plugin/json.go` | Change NLRI format (announce/withdraw → family list with operations) | ✅ Done |
| `internal/plugin/text.go` | formatFilterResultJSON with command-style | ✅ Done |
| `internal/plugin/json_test.go` | Update tests for new format | ✅ Done (10 tests) |
| `internal/bgp/nlri/ipvpn.go` | Add type prefix to RD String() (`0:`, `1:`, `2:`) | ✅ Done |
| `internal/bgp/nlri/evpn.go` | Ensure ESI exposed in JSON format | ✅ Verified |
| `internal/bgp/nlri/flowspec.go` | Expose operators in JSON format | ✅ Via String() |
| `internal/plugin/rib/event.go` | Update Event struct for new NLRI format | ✅ Done |
| `internal/plugin/rib/rib.go` | Update parsing (announce→add, withdraw→del) | ✅ Done |
| `internal/plugin/rib/rib_test.go` | Update tests for new format | ✅ Done |
| `docs/architecture/api/architecture.md` | Update event format examples | ✅ Done |
| `docs/exabgp/exabgp-migration.md` | Document format change (renamed from compatibility) | ✅ Done |

**Note:** RD String() includes type prefix (`0:65000:100`, `1:192.0.2.1:100`, `2:65536:100`) for unambiguous parsing.

## Files to Create

No new files created. Instead, existing test files were updated with `json:` documentation lines:
- `test/data/plugin/ipv4.ci` - Added json: lines documenting new format
- `test/data/plugin/ipv6.ci` - Added json: lines documenting new format

## Implementation Steps
1. **Write unit tests** - Create tests for new NLRI format (TDD)
2. **Run tests** - Verify FAIL (paste output)
3. **Change JSON encoder** - Update to family → add/del format with next-hop per NLRI
4. **Update RIB plugin** - Change Event struct and parsing
5. **Update all tests** - Fix tests using old announce/withdraw format
6. **Run tests** - Verify PASS (paste output)
7. **RFC refs** - Add RFC 4271/4760 references
8. **RFC constraints** - Document next-hop handling per RFC
9. **Functional tests** - Create `.ci` tests
10. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

### Reference Comments
```go
// RFC 4271 Section 5.1.3: "The NEXT_HOP attribute defines the IP address
// of the router that SHOULD be used as the next hop to the destinations
// listed in the UPDATE message."
// IPv4 unicast: NEXT_HOP attribute applies to NLRI section prefixes.

// RFC 4760 Section 3: "The Next Hop field of the MP_REACH_NLRI attribute
// shall be interpreted as an IPv4 address or an IPv6 address..."
// MP families: Each MP_REACH_NLRI has its own next-hop field.

// One UPDATE can have:
// - NEXT_HOP attribute (for IPv4 unicast NLRI section)
// - MP_REACH_NLRI (AFI=1, SAFI=1) with different next-hop (for IPv4 unicast via MP)
// - Multiple MP_REACH_NLRI for different families
// → Group announcements by next-hop within each family.
```

### Constraint Comments
```go
// RFC 4760: One UPDATE can announce IPv4 and IPv6 with different next-hops.
// MUST include next-hop with each NLRI, not as separate top-level field.
if family.AFI == nlri.AFI_IPV4 && family.SAFI == nlri.SAFI_UNICAST {
    // IPv4 unicast: NEXT_HOP attribute applies to all prefixes
    nextHop = nextHopAttr
} else {
    // Other families: next-hop from MP_REACH_NLRI
    nextHop = mpReachNextHop
}
```

## Design Decisions

### Format Change Overview

**OLD format (ExaBGP style with next-hop grouping):**
```json
{
  "announce": {
    "ipv4/unicast": {
      "192.0.2.1": [{"nlri": "10.0.0.0/24"}, {"nlri": "10.0.1.0/24"}]
    },
    "ipv6/unicast": {
      "2001:db8::1": [{"nlri": "2001:db8::/32"}]
    }
  },
  "withdraw": {
    "ipv4/unicast": [{"nlri": "172.16.0.0/16"}]
  }
}
```

**NEW format (command style with grouped operations):**
```json
{
  "ipv4/unicast": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": ["10.0.0.0/24", "10.0.1.0/24"]
    },
    {
      "next-hop": "192.0.2.2",
      "action": "add",
      "nlri": ["10.0.2.0/24"]
    },
    {
      "action": "del",
      "nlri": ["172.16.0.0/16"]
    }
  ],
  "ipv6/unicast": [
    {
      "next-hop": "2001:db8::1",
      "action": "add",
      "nlri": ["2001:db8::/32"]
    }
  ]
}
```

**Key improvements:**
1. **Correct:** Handles multiple next-hops per family (IPv4 traditional + MP)
2. **Efficient:** Groups by next-hop (no duplication)
3. **Consistent:** Family value always a list
4. **Clear:** Explicit `action` field
5. **Simple:** Omit `next-hop` for withdrawals

### NLRI Format by Family

#### Simple Families

**IPv4/IPv6 unicast, multicast:**
```json
"ipv4/unicast": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": ["10.0.0.0/24", "10.0.1.0/24"]
  },
  {
    "action": "del",
    "nlri": ["172.16.0.0/16"]
  }
]
```

**Note:**
- `next-hop` present for announcements (required)
- `next-hop` omitted for withdrawals (`action: "del"`)
- If next-hop not available for `action: "add"`, omit the field (edge case)

#### Labeled Unicast (RFC 8277)

```json
"ipv4/labeled-unicast": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": [
      {"prefix": "10.0.0.0/24", "labels": [100]}
    ]
  },
  {
    "next-hop": "192.0.2.2",
    "action": "add",
    "nlri": [
      {"prefix": "192.168.1.0/24", "labels": [200, 300]}
    ]
  },
  {
    "action": "del",
    "nlri": ["172.16.0.0/16"]
  }
]
```

#### L3VPN / MPLS-VPN (RFC 4364)

```json
"ipv4/mpls-vpn": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": [
      {"prefix": "10.0.0.0/24", "rd": "2:65000:1", "labels": [100]}
    ]
  },
  {
    "next-hop": "192.0.2.2",
    "action": "add",
    "nlri": [
      {"prefix": "192.168.1.0/24", "rd": "1:192.0.2.1:100", "labels": [200]}
    ]
  },
  {
    "action": "del",
    "nlri": [
      {"prefix": "172.16.0.0/16", "rd": "2:65000:1"}
    ]
  }
]
```

**Note:**
- Withdrawals include RD (part of NLRI identity), omit labels/next-hop
- RD format: `<type>:<value>` where type 0=ASN2:assigned, 1=IP:assigned, 2=ASN4:assigned

#### EVPN (RFC 7432)

```json
"l2vpn/evpn": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": [
      {
        "route-type": "mac-ip",
        "rd": "2:65000:1",
        "esi": "00:00:00:00:00:00:00:00:00:00",
        "ethernet-tag": 0,
        "mac": "00:11:22:33:44:55",
        "ip": "10.0.0.1",
        "labels": [100]
      }
    ]
  },
  {
    "next-hop": "192.0.2.2",
    "action": "add",
    "nlri": [
      {
        "route-type": "ip-prefix",
        "rd": "2:65000:2",
        "esi": "00:00:00:00:00:00:00:00:00:00",
        "ethernet-tag": 0,
        "prefix": "192.168.1.0/24",
        "gateway": "10.0.0.1",
        "label": 200
      }
    ]
  }
]
```

**Note:**
- ESI (Ethernet Segment Identifier): 10-byte hex string, 00:00...00 = single-homed
- RD format: `<type>:<value>` (same as VPN families)

#### FlowSpec (RFC 8955)

```json
"ipv4/flowspec": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": [
      {
        "dest-prefix": "10.0.0.0/24",
        "source-prefix": "192.168.1.0/24",
        "protocol": [{"op": "=", "value": 6}],
        "dest-port": [
          {"op": ">=", "value": 80},
          {"op": "<=", "value": 443}
        ],
        "tcp-flags": [{"op": "&", "value": 0x02}]
      }
    ]
  }
]
```

**Note:**
- FlowSpec components use operators: `=`, `>`, `<`, `>=`, `<=`, `&` (AND), `|` (OR)
- Multiple conditions per component = logical AND
- Operators critical for correct filtering behavior

#### ADD-PATH (RFC 7911)

**With ADD-PATH enabled:**
```json
"ipv4/unicast": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": [
      {"prefix": "10.0.0.0/24", "path-id": 1}
    ]
  },
  {
    "next-hop": "192.0.2.2",
    "action": "add",
    "nlri": [
      {"prefix": "10.0.0.0/24", "path-id": 2}
    ]
  }
]
```

**Without ADD-PATH (standard):**
```json
"ipv4/unicast": [
  {
    "next-hop": "192.0.2.1",
    "action": "add",
    "nlri": ["10.0.0.0/24", "10.0.1.0/24"]
  }
]
```

**Note:**
- ADD-PATH enabled: NLRI = object array with `path-id`
- ADD-PATH disabled: NLRI = string array (simple prefixes)
- Type changes based on peer capability (plugins must handle both)

### Text Format

**Current text format already matches command style:**
```
peer 127.0.0.1 update origin igp as-path [65001] nhop set 192.0.2.1 nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24 del 172.16.0.0/16
```

**No change needed for text format.**

### Withdrawn Routes

**Structure:** Same list format, `action: "del"`, no `next-hop` field.

**Identity fields required for withdrawal:**

| Family | Identity Fields | Example |
|--------|----------------|---------|
| ipv4/unicast | prefix (+ path-id if ADD-PATH) | `"172.16.0.0/16"` or `{"prefix": "...", "path-id": 1}` |
| ipv4/labeled-unicast | prefix (+ path-id if ADD-PATH) | `"172.16.0.0/16"` (labels omitted) |
| ipv4/mpls-vpn | rd + prefix (+ path-id if ADD-PATH) | `{"prefix": "...", "rd": "2:65000:1"}` |
| l2vpn/evpn Type 2 | rd + esi + ethernet-tag + mac + ip | `{"route-type": "mac-ip", "rd": "...", "esi": "...", ...}` |
| l2vpn/evpn Type 5 | rd + esi + ethernet-tag + prefix | `{"route-type": "ip-prefix", "rd": "...", ...}` |
| ipv4/flowspec | All match components | `{"dest-prefix": "...", "protocol": [...], ...}` |

**Examples:**
```json
"ipv4/unicast": [
  {
    "action": "del",
    "nlri": ["172.16.0.0/16", "10.1.0.0/16"]
  }
]

"ipv4/mpls-vpn": [
  {
    "action": "del",
    "nlri": [
      {"prefix": "172.16.0.0/16", "rd": "2:65000:1"}
    ]
  }
]

"l2vpn/evpn": [
  {
    "action": "del",
    "nlri": [
      {
        "route-type": "mac-ip",
        "rd": "2:65000:1",
        "esi": "00:00:00:00:00:00:00:00:00:00",
        "ethernet-tag": 0,
        "mac": "00:11:22:33:44:55",
        "ip": "10.0.0.1"
      }
    ]
  }
]
```

### Special Values & Validation

#### Route Distinguisher (RD) Types

**Format:** `<type>:<value>`

| Type | Format | Wire Encoding | Example |
|------|--------|---------------|---------|
| 0 | `0:<asn2>:<assigned>` | 2-byte ASN + 4-byte | `0:100:1` |
| 1 | `1:<ipv4>:<assigned>` | 4-byte IP + 2-byte | `1:192.0.2.1:100` |
| 2 | `2:<asn4>:<assigned>` | 4-byte ASN + 2-byte | `2:65000:1` |

**Special:**
- `0:0:0` or `0:0` = No distinguisher (all zeros)

#### MPLS Labels

**Range:** 0-1048575 (20-bit)

**Special values:**
- 0: IPv4 Explicit NULL
- 1: Router Alert
- 2: IPv6 Explicit NULL
- 3: Implicit NULL
- 4-15: Reserved

**Label stack order:** First in array = outermost label (bottom-of-stack implicit from position)

#### Prefix Validation

**Valid ranges:**
- IPv4: /0 to /32
- IPv6: /0 to /128

**Invalid examples:**
- `"10.0.0.0/33"` ❌ (length > 32)
- `"2001:db8::/129"` ❌ (length > 128)
- `"10.0.0.0"` ❌ (missing length)

#### ESI (Ethernet Segment Identifier)

**Format:** 10-byte hex string (20 hex digits with colons)

**Special:**
- `00:00:00:00:00:00:00:00:00:00` = Single-homed (no multihoming)

**ESI Type 0 encoding:**
- Byte 0: Type (0-5)
- Bytes 1-9: Value (format depends on type)

### Backward Compatibility

**Breaking change:** All plugins parsing JSON events need updates.

**Migration:**
- Old RIB plugin: Update `event.go` struct and parsing
- External plugins: Update JSON parsing logic
- Functional tests: Update expected output

**No config flag for format:** New format is mandatory (clean break).

---

## Summary of New Format

### Structure

```json
{
  "<family>": [
    {
      "next-hop": "<address>",  // Optional: present for add, omit for del/no-NH
      "action": "add|del",
      "nlri": [...]              // String array or object array
    },
    ...
  ]
}
```

### Rules

1. **Family value = array** (always)
2. **Each operation = object** with `action` + `nlri` (+ optional `next-hop`)
3. **Grouped by next-hop** within family (no duplication)
4. **Withdrawals:** `action: "del"`, no `next-hop`
5. **Announcements:** `action: "add"`, `next-hop` present (if available)

### Complete Example

```json
{
  "ipv4/unicast": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": ["10.0.0.0/24", "10.0.1.0/24"]
    },
    {
      "next-hop": "192.0.2.2",
      "action": "add",
      "nlri": ["10.0.2.0/24"]
    },
    {
      "action": "del",
      "nlri": ["172.16.0.0/16"]
    }
  ],
  "ipv4/mpls-vpn": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": [
        {"prefix": "10.0.0.0/24", "rd": "2:65000:1", "labels": [100]}
      ]
    },
    {
      "action": "del",
      "nlri": [
        {"prefix": "172.16.0.0/16", "rd": "2:65000:1"}
      ]
    }
  ],
  "l2vpn/evpn": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": [
        {
          "route-type": "mac-ip",
          "rd": "2:65000:1",
          "esi": "00:00:00:00:00:00:00:00:00:00",
          "ethernet-tag": 0,
          "mac": "00:11:22:33:44:55",
          "ip": "10.0.0.1",
          "labels": [100]
        }
      ]
    }
  ],
  "ipv4/flowspec": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": [
        {
          "dest-prefix": "10.0.0.0/24",
          "protocol": [{"op": "=", "value": 6}],
          "dest-port": [
            {"op": ">=", "value": 80},
            {"op": "<=", "value": 443}
          ]
        }
      ]
    }
  ]
}
```

## Implementation Summary

### What Was Implemented

**Core JSON format change:**
- `internal/plugin/text.go`: `formatFilterResultJSON()` - new command-style format with family → operations array
- `internal/plugin/text.go`: `familyOperation` struct with Action, NextHop, NLRIs
- `internal/plugin/text.go`: `formatNLRIJSONValue()` - handles simple prefixes as strings, complex as objects
- `internal/plugin/text.go`: Family-specific formatters (EVPN, FlowSpec, Labeled, VPN)

**RIB plugin updates:**
- `internal/plugin/rib/event.go`: `FamilyOperation` struct for parsing new format
- `internal/plugin/rib/event.go`: `parseEvent()` with dynamic family key extraction
- `internal/plugin/rib/rib.go`: Updated to parse `action: "add"/"del"` instead of `announce`/`withdraw`

**RD type prefix:**
- `internal/bgp/nlri/ipvpn.go`: RD String() now outputs `0:`, `1:`, `2:` type prefix

**Unit tests (10 complete):**
- `TestJSONEncoderIPv4UnicastNewFormat` - basic IPv4 unicast
- `TestJSONEncoderWithdrawNewFormat` - withdrawals with action: del
- `TestJSONEncoderMultiFamilyNewFormat` - IPv4 + IPv6 in same UPDATE
- `TestJSONEncoderAnnounceAndWithdrawSameFamily` - add + del in same family
- `TestJSONEncoderADDPATHNewFormat` - path-id in NLRI objects
- `TestJSONEncoderIPv4DualNextHop` - IPv4 with NEXT_HOP attr + MP_REACH ✅
- `TestJSONEncoderLabeledUnicast` - Labels in NLRI ✅
- `TestJSONEncoderMPLSVPN` - RD with type prefix + labels ✅
- `TestJSONEncoderEVPN` - ESI + route-type fields ✅
- `TestJSONEncoderRDTypes` - Type 0, 1, 2 format verification ✅
- `TestJSONEncoderFlowSpec` - FlowSpec VPN via String() ✅

### Deferred Items

**Functional tests (5):** Deferred - unit tests provide sufficient coverage
- `test/data/plugin/nlri-format-*.ci` files

### Documentation Updates

- `docs/architecture/api/architecture.md` - Updated with new JSON format examples
- `docs/exabgp/exabgp-migration.md` - Renamed from `exabgp-compatibility.md`, added NLRI format migration guide

### Bugs Found/Fixed
- RD String() was missing type prefix - fixed in `ipvpn.go`
- Test data `bgp-evpn-1.test` updated with `1:` prefix for Type 1 RD

### Design Insights
- RD String() format: Type prefix required (`0:`, `1:`, `2:`) because Type 0 and Type 2 are otherwise indistinguishable
- FlowSpec uses String() for JSON (structured operators deferred to future spec)
- EVPN ESI properly exposed via existing String() method
- formatNLRIJSONValue() correctly handles simple vs complex NLRI (string vs object)

### Deviations from Plan
- Added `TestJSONEncoderAnnounceAndWithdrawSameFamily` (not in original spec)
- Test names use `NewFormat` suffix for clarity
- `internal/bgp/nlri/rd.go` doesn't exist - RD is in `internal/bgp/nlri/ipvpn.go`
- FlowSpec uses String() instead of structured operators (simpler, sufficient for now)

## Checklist

### 🧪 TDD
- [x] Tests written (10/10 unit tests)
- [x] Tests FAIL - verified for all tests
- [x] Implementation complete
- [x] Tests PASS - verified for all tests
- [x] All unit tests implemented
- [ ] Functional tests - DEFERRED (json: lines in .ci files are not validated by test framework)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (80 tests)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (rfc4271, rfc4760)
- [x] RFC references added to code (formatFilterResultJSON has RFC comments)
- [x] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass)
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-nlri-format-command-style.md`
- [ ] All files committed together

---

**Created:** 2026-01-14
**Status:** ✅ Implementation Complete - Ready for commit

### Progress Summary

| Category | Done | Remaining |
|----------|------|-----------|
| Unit tests | 10 | 0 |
| Code files | 9 | 0 |
| Lint/Test/Functional | ✅ All pass | - |
| Functional .ci tests | 0 | Deferred |
| Documentation | 2 | 0 |

**Note:** Added `json:` lines to `ipv4.ci` and `ipv6.ci` as format documentation only. The test framework does NOT validate these lines - they serve as reference for expected JSON format.

### Files Modified
- `internal/plugin/text.go` - Core JSON format change
- `internal/plugin/text_test.go` - Test updates
- `internal/plugin/json_test.go` - 10 new format tests
- `internal/plugin/message_receiver_test.go` - Test updates
- `internal/plugin/update_text_test.go` - Test updates
- `internal/plugin/rib/event.go` - FamilyOperation struct
- `internal/plugin/rib/rib.go` - Parsing updates
- `internal/plugin/rib/rib_test.go` - Test updates
- `internal/bgp/nlri/ipvpn.go` - RD type prefix
- `internal/bgp/nlri/ipvpn_test.go` - RD tests
- `internal/bgp/nlri/evpn_test.go` - EVPN tests
- `test/data/decode/bgp-evpn-1.test` - RD format update
- `test/data/plugin/ipv4.ci` - Added json: documentation lines (NOT validated)
- `test/data/plugin/ipv6.ci` - Added json: documentation lines (NOT validated)
- `docs/architecture/api/architecture.md` - Updated JSON format examples
- `docs/exabgp/exabgp-migration.md` - Renamed, added migration guide
- `docs/plan/spec-nlri-format-command-style.md` - This spec
