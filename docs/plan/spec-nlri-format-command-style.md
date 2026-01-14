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
- [ ] `docs/architecture/core-design.md` - Engine/API split
- [ ] `docs/architecture/api/architecture.md` - Event format, process bindings
- [ ] `docs/exabgp/exabgp-compatibility.md` - Compatibility requirements

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - UPDATE message format, NEXT_HOP attribute
- [ ] `docs/rfc/rfc4760.md` - MP_REACH_NLRI (next-hop per family)

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
| `TestJSONEncoderIPv4Unicast` | `pkg/plugin/json_test.go` | IPv4 unicast list with grouped operations | |
| `TestJSONEncoderIPv4DualNextHop` | `pkg/plugin/json_test.go` | IPv4 traditional + MP (two next-hops) | |
| `TestJSONEncoderMultiFamily` | `pkg/plugin/json_test.go` | Multiple families, different next-hops | |
| `TestJSONEncoderLabeledUnicast` | `pkg/plugin/json_test.go` | Labels + next-hop grouped | |
| `TestJSONEncoderMPLSVPN` | `pkg/plugin/json_test.go` | RD format `2:65000:1` + labels | |
| `TestJSONEncoderEVPN` | `pkg/plugin/json_test.go` | ESI + route-type + next-hop | |
| `TestJSONEncoderFlowSpec` | `pkg/plugin/json_test.go` | Operators in FlowSpec components | |
| `TestJSONEncoderADDPATH` | `pkg/plugin/json_test.go` | path-id in objects | |
| `TestJSONEncoderWithdrawals` | `pkg/plugin/json_test.go` | Identity fields, no next-hop | |
| `TestJSONEncoderRDTypes` | `pkg/plugin/json_test.go` | RD type 0, 1, 2 formats | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `nlri-format-ipv4` | `test/data/plugin/nlri-format-ipv4.ci` | IPv4 unicast new format | |
| `nlri-format-ipv6` | `test/data/plugin/nlri-format-ipv6.ci` | IPv6 unicast new format | |
| `nlri-format-multi-family` | `test/data/plugin/nlri-format-multi-family.ci` | Multiple families | |
| `nlri-format-mpls-vpn` | `test/data/plugin/nlri-format-mpls-vpn.ci` | MPLS-VPN format | |
| `nlri-format-evpn` | `test/data/plugin/nlri-format-evpn.ci` | EVPN format | |

### Future (if deferring any tests)
- None - full implementation

## Files to Modify
- `pkg/plugin/json.go` - Change NLRI format (announce/withdraw → family list with operations)
- `pkg/plugin/json_test.go` - Update tests for new format
- `pkg/bgp/nlri/rd.go` - Add `String()` method with type prefix (`2:65000:1`)
- `pkg/bgp/nlri/evpn.go` - Ensure ESI exposed in JSON format
- `pkg/bgp/nlri/flowspec.go` - Expose operators in JSON format
- `pkg/plugin/rib/event.go` - Update Event struct for new NLRI format
- `pkg/plugin/rib/rib.go` - Update parsing (announce→add, withdraw→del)
- `pkg/plugin/rib/rib_test.go` - Update tests for new format
- `docs/architecture/api/architecture.md` - Update event format examples
- `docs/exabgp/exabgp-compatibility.md` - Document format change

## Files to Create
- `test/data/plugin/nlri-format-ipv4.ci` - IPv4 test
- `test/data/plugin/nlri-format-ipv6.ci` - IPv6 test
- `test/data/plugin/nlri-format-multi-family.ci` - Multi-family test
- `test/data/plugin/nlri-format-mpls-vpn.ci` - MPLS-VPN test
- `test/data/plugin/nlri-format-evpn.ci` - EVPN test

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

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Investigation → Test Rule
If you had to investigate/debug something, add a test for it.

### Design Insights
- [Key learnings that should be documented elsewhere]

### Deviations from Plan
- [Any differences from original plan and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (rfc4271, rfc4760)
- [ ] RFC references added to code
- [ ] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-nlri-format-command-style.md`
- [ ] All files committed together

---

**Created:** 2026-01-14
**Status:** Ready for implementation
