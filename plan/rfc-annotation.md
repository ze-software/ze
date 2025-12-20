# RFC Annotation Plan

**Priority:** Before implementing ALIGN items, annotate existing code with RFC references.

**Goals:**
1. Add RFC section references to all protocol implementation code
2. Document RFC violations found (for later fixing)
3. Create foundation for RFC compliance verification

---

## Phase 1: Message Layer (`pkg/bgp/message/`) ✅

| File | RFC | Status |
|------|-----|--------|
| header.go | RFC 4271 Section 4.1 | ✅ |
| open.go | RFC 4271 Section 4.2 | ✅ |
| update.go | RFC 4271 Section 4.3 | ✅ |
| notification.go | RFC 4271 Section 4.5 | ✅ |
| keepalive.go | RFC 4271 Section 4.4 | ✅ |
| routerefresh.go | RFC 2918 | ✅ |

---

## Phase 2: Capabilities (`pkg/bgp/capability/`) ✅

| File | RFC | Status |
|------|-----|--------|
| capability.go | RFC 5492, 4760, 6793, 7911, 4724, 8654, 8950 | ✅ |
| negotiated.go | RFC 5492, 4760, 6793, 7911, 8654 | ✅ |

---

## Phase 3: Path Attributes (`pkg/bgp/attribute/`) ✅

| File | RFC | Status |
|------|-----|--------|
| attribute.go | RFC 4271 Section 4.3, 5 | ✅ |
| aspath.go | RFC 4271 Section 5.1.2, RFC 6793 | ✅ |
| as4.go | RFC 6793 | ✅ |
| origin.go | RFC 4271 Section 5.1.1 | ✅ |
| community.go | RFC 1997, 4360, 8092 | ✅ |
| simple.go | RFC 4271 Sections 5.1.3-5.1.7 | ✅ |
| mpnlri.go | RFC 4760 Sections 3-4 | ✅ |

---

## Phase 4: NLRI Types (`pkg/bgp/nlri/`) ✅

| File | RFC | Status |
|------|-----|--------|
| nlri.go | RFC 4271, 4760, 7911 | ✅ |
| inet.go | RFC 4271 Section 4.3 | ✅ |
| ipvpn.go | RFC 4364, 4659, 3107 | ✅ |
| evpn.go | RFC 7432, 9136 | ✅ |
| flowspec.go | RFC 8955 | ✅ |
| bgpls.go | RFC 7752, 9514 | ✅ |
| other.go | RFC 6514, 4761, 4684 | ✅ |

---

## Phase 5: FSM (`pkg/bgp/fsm/`) ✅

| File | RFC | Status |
|------|-----|--------|
| fsm.go | RFC 4271 Section 8 | ✅ |
| state.go | RFC 4271 Section 8.2.2 | ✅ |
| timer.go | RFC 4271 Section 8.1.3, 10 | ✅ |

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

```
evpn.go:440 - RFC 9136 Section 3 violation
  Current: Type 5 IP prefix uses variable-length encoding
  Required: Fixed 4-octet (IPv4) or 16-octet (IPv6) fields
  Severity: MEDIUM

bgpls.go:168 - RFC 7752 Section 3.2.2 violation
  Current: TLV 258 used as container for link descriptors
  Required: 258 is "Link Local/Remote Identifiers", not container
  Severity: MEDIUM

bgpls.go:193 - RFC 7752 Section 3.2.3 violation
  Current: TLV 264 used as container for prefix descriptors
  Required: 264 is "OSPF Route Type", not container
  Severity: MEDIUM

bgpls.go:440,521 - RFC 7752 Section 3.2 violation
  Current: Bytes() wraps descriptors in container TLVs
  Required: Descriptors appear directly in NLRI body
  Severity: MEDIUM
```

### FSM

```
fsm.go:291-298 - RFC 4271 Section 8.2.2 deviation
  Current: OpenSent + TcpConnectionFails → Idle
  Required: OpenSent + TcpConnectionFails → Active (with timer restart)
  Severity: LOW (simplification, functionally equivalent)
```

---

## Next Steps

After annotation complete:
1. Triage violations by severity
2. Merge HIGH severity into Phase 1 ALIGN items
3. Create implementation plan for 26 ALIGN items + violations
