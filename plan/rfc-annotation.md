# RFC Annotation Plan

**Priority:** Before implementing ALIGN items, annotate existing code with RFC references.

**Goals:**
1. Add RFC section references to all protocol implementation code
2. Document RFC violations found (for later fixing)
3. Create foundation for RFC compliance verification

---

## Phase 1: Message Layer (`pkg/bgp/message/`)

### 1.1 Header (`header.go`)
- [ ] Add RFC 4271 Section 4.1 references
- [ ] Document violations: ___

### 1.2 OPEN (`open.go`)
- [ ] Add RFC 4271 Section 4.2 references
- [ ] Document violations: ___

### 1.3 UPDATE (`update.go`)
- [ ] Add RFC 4271 Section 4.3 references
- [ ] Document violations: ___

### 1.4 NOTIFICATION (`notification.go`)
- [ ] Add RFC 4271 Section 4.5 references
- [ ] Document violations: ___

### 1.5 KEEPALIVE (`keepalive.go`)
- [ ] Add RFC 4271 Section 4.4 references
- [ ] Document violations: ___

### 1.6 ROUTE-REFRESH (`routerefresh.go`)
- [ ] Add RFC 2918 references
- [ ] Document violations: ___

---

## Phase 2: Capabilities (`pkg/bgp/capability/`)

| File | RFC | Status |
|------|-----|--------|
| capability.go | RFC 5492 | [ ] |
| multiprotocol.go | RFC 4760 | [ ] |
| asn4.go | RFC 6793 | [ ] |
| addpath.go | RFC 7911 | [ ] |
| routerefresh.go | RFC 2918 | [ ] |
| gracefulrestart.go | RFC 4724 | [ ] |
| extendedmessage.go | RFC 8654 | [ ] |

---

## Phase 3: Path Attributes (`pkg/bgp/attribute/`)

| File | RFC | Status |
|------|-----|--------|
| origin.go | RFC 4271 Section 5.1.1 | [ ] |
| aspath.go | RFC 4271 Section 5.1.2, RFC 6793 | [ ] |
| nexthop.go | RFC 4271 Section 5.1.3 | [ ] |
| med.go | RFC 4271 Section 5.1.4 | [ ] |
| localpref.go | RFC 4271 Section 5.1.5 | [ ] |
| atomicaggregate.go | RFC 4271 Section 5.1.6 | [ ] |
| aggregator.go | RFC 4271 Section 5.1.7 | [ ] |
| community.go | RFC 1997 | [ ] |
| extcommunity.go | RFC 4360 | [ ] |
| largecommunity.go | RFC 8092 | [ ] |
| mpreach.go | RFC 4760 Section 3 | [ ] |
| mpunreach.go | RFC 4760 Section 4 | [ ] |

---

## Phase 4: NLRI Types (`pkg/bgp/nlri/`)

| File | RFC | Status |
|------|-----|--------|
| ipv4.go | RFC 4271 Section 4.3 | [ ] |
| ipv6.go | RFC 4760 | [ ] |
| vpnv4.go | RFC 4364 | [ ] |
| vpnv6.go | RFC 4659 | [ ] |
| evpn.go | RFC 7432 | [ ] |
| flowspec.go | RFC 8955 | [ ] |

---

## Phase 5: FSM (`pkg/bgp/fsm/`)

| File | RFC | Status |
|------|-----|--------|
| fsm.go | RFC 4271 Section 8 | [ ] |
| states.go | RFC 4271 Section 8.2.2 | [ ] |
| events.go | RFC 4271 Section 8.1.2 | [ ] |
| timers.go | RFC 4271 Section 8.1.3 | [ ] |

---

## RFC Violations Found

Track violations here during annotation. Format:
```
FILE:LINE - RFC XXXX Section Y.Z violation
  Current: [what code does]
  Required: [what RFC says]
  Severity: HIGH/MEDIUM/LOW
```

### Message Layer

```
header.go:92 - RFC 4271 Section 4.1 violation
  Current: Only validates Length >= 19, not <= 4096
  Required: "Length field MUST always be at least 19 and no greater than 4096"
  Severity: MEDIUM (may be intentional for RFC 8654 Extended Message)
  Note: Need to validate against negotiated max (4096 or 65535)
```

### Capabilities
(to be filled during annotation)

### Attributes

```
aspath.go:197 - RFC 4271 Section 5.1.2 violation (SHOULD)
  Current: Prepend() doesn't handle segment overflow (>255 ASes)
  Required: "it SHOULD prepend a new segment" when overflow would occur
  Severity: LOW (SHOULD requirement)

as4.go:115-146 - RFC 6793 Section 6 violation
  Current: ParseAS4Path missing validation
  Required: Validate min length (6), length multiple of 2, segment type, non-zero count
  Severity: MEDIUM

as4.go:115-146 - RFC 6793 Section 6 violation
  Current: AS_CONFED_* segments not discarded from OLD speakers
  Required: "MUST discard" confed segments in AS4_PATH from OLD speakers
  Severity: MEDIUM

as4.go:323-329 - RFC 6793 Section 4.2.3 violation
  Current: countASNs counts all ASNs literally
  Required: AS_SET=1, confederation=0 for path length calculation
  Severity: MEDIUM
```

### NLRI
(to be filled during annotation)

### FSM
(to be filled during annotation)

---

## Next Steps

After annotation complete:
1. Triage violations by severity
2. Merge HIGH severity into Phase 1 ALIGN items
3. Create implementation plan for 26 ALIGN items + violations
