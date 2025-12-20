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
(to be filled during annotation)

### Capabilities
(to be filled during annotation)

### Attributes
(to be filled during annotation)

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
