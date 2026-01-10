# ZeBGP vs ExaBGP Implementation Comparison Report

**Generated:** 2025-12-20
**Purpose:** Identify logic differences between ZeBGP (Go) and ExaBGP (Python) implementations

---

## Executive Summary

| Area | ZeBGP Approach | ExaBGP Approach | Alignment Level |
|------|----------------|-----------------|-----------------|
| **Message Parsing** | Wire-level, minimal | Deep parsing, full validation | Medium |
| **Capability Negotiation** | 9 capabilities, simple | 18 capabilities, RFC 9072 | Divergent |
| **Path Attributes** | Struct-based, no cache | Class-based, extensive cache | Medium |
| **NLRI Types** | Good coverage | Complete coverage | Good |
| **FSM** | Event-driven, 90s hold | Procedural, 180s hold | Medium |
| **Reactor/Session** | Goroutines | Asyncio/generators | Different paradigm |
| **Config Parsing** | Schema-driven | Section-based | Good |

---

## 1. Message Parsing Comparison

### 1.1 OPEN Message

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Capability Parsing** | Deferred (raw bytes stored) | Immediate (full object parsing) |
| **Version Validation** | Not shown in OPEN | Strict (only BGP v4) |
| **Error Handling** | Returns Go errors | Raises Python Notify exceptions |
| **Memory Model** | References optional params | Parses all capabilities into objects |
| **Extensibility** | Manual capability parsing needed | Registered capability handlers |

### 1.2 UPDATE Message

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Parsing Depth** | 2-level (sections only) | Full recursive parsing |
| **Memory Usage** | Minimal (references) | Significant (parsed objects) |
| **Message Fragmentation** | Not handled | Automatic via generator |
| **EOR Detection** | Simple (empty check) | Multi-family support |
| **MP-BGP Support** | Not in UPDATE level | Full MPRNLRI/MPURNLRI |
| **AddPath Support** | Not shown | Full RFC 7911 support |
| **Message Size Handling** | Assumed correct | Dynamic fragmentation |

### 1.3 NOTIFICATION Message

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Shutdown Communication** | Not handled | Full RFC 8203/9003 support |
| **Error Code Coverage** | 12 subcodes defined | 48+ subcodes mapped |
| **Data Parsing** | No special handling | UTF-8 shutdown messages |
| **Error Messages** | Generic | Detailed per-subcode |
| **Data Validation** | None | Length/UTF-8/overflow checks |

### 1.4 KEEPALIVE Message

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Payload Validation** | Silently ignored | Raises exception |
| **Memory Model** | Singleton | New instance |
| **Error Handling** | Lenient | Strict |

### 1.5 Header Parsing

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Parsing Errors** | Explicit error types | Implicit in protocol layer |
| **Type Validation** | Only extracts type | Per-type length validation |
| **Message Lengths** | No type-specific checks | Per-type min/exact lengths |
| **Extended Messages** | Constants defined | Not visible in code |

---

## 2. Capability Negotiation Comparison

### 2.1 Supported Capabilities

**ZeBGP (9 capabilities):**
| Capability | Code | RFC |
|------------|------|-----|
| Multiprotocol Extensions | 1 | RFC 4760 |
| Route Refresh | 2 | RFC 2918 |
| Extended Next Hop | 5 | RFC 8950 |
| Extended Message | 6 | RFC 8654 |
| Graceful Restart | 64 | RFC 4724 |
| 4-Byte AS Number | 65 | RFC 6793 |
| ADD-PATH | 69 | RFC 7911 |
| FQDN | 73 | RFC 8516 |
| Software Version | 75 | draft-ietf-idr-software-version |

**ExaBGP Additional Capabilities:**
- Outbound Route Filtering (3)
- Dynamic Capability (67)
- Multi-Session BGP (68)
- Enhanced Route Refresh (70)
- Operational (0xB9)
- Cisco variants (0x80, 0x83)
- Extended Optional Parameters (RFC 9072)

### 2.2 Negotiation Logic

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Parsing** | Simple sequential TLV | Complex with RFC 9072 extended format |
| **Negotiation** | Pure intersection, silent | Intersection + validation + error generation |
| **Conflict Handling** | Silent/no reporting | Active detection & RFC error codes |
| **Architecture** | Stateless functions | Stateful Negotiated class |
| **Unknown Caps** | Preserved as-is | Classified by IANA range |
| **Validation** | None | Comprehensive pre-connection validation |
| **Error Return** | Simple errors | RFC 4271 error tuples |

### 2.3 ADD-PATH Handling

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Storage** | Mode field per family | Separate send/recv tracking |
| **Negotiation** | Per-family asymmetric | RequirePath class |
| **Modes** | None/Receive/Send/Both | Same via bitwise ops |

### 2.4 Extended Message

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Size Constants** | Not embedded | INITIAL_SIZE=4096, EXTENDED_SIZE=65535 |
| **Integration** | External handling needed | Applied on negotiation |

---

## 3. Path Attributes Comparison

### 3.1 ORIGIN

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Validation** | Strict - rejects invalid | Permissive - accepts any |
| **Caching** | None | All 3 instances pre-cached |
| **Parsing** | Function-based | Classmethod-based |

### 3.2 AS_PATH / AS4_PATH

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Segment max** | Relies on caller | Auto-splits at 255 |
| **AS_TRANS** | Explicit constant (23456) | Via `ASN.trans()` object |
| **Merge strategy** | Explicit merge function | Implicit during unpack |
| **Peer support** | Parameter-driven | `negotiated.asn4` flag |

### 3.3 Communities

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Design** | Flat types | Hierarchical OOP |
| **Large comm dedup** | None | Yes |
| **Extended IPv6** | Not implemented | Yes (RFC 5701) |
| **Sorting** | None | Auto-sort |

### 3.4 MP_REACH_NLRI / MP_UNREACH_NLRI

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Family negotiation** | Not checked | Strict validation |
| **Extended nexthop** | Not supported | Via capability flag |
| **AddPath** | Not supported | Via negotiation |
| **Route Distinguisher** | Not handled | Full VPN support |
| **Chunking** | None | Respects attr size limits |
| **Error handling** | Strict | Error recovery fallback |

---

## 4. NLRI Types Comparison

### 4.1 Coverage

| Type | ZeBGP | ExaBGP |
|------|-------|--------|
| IPv4/IPv6 Unicast | Full | Full |
| IPv4/IPv6 Multicast | Full | Full |
| IPVPN | Full | Full |
| EVPN (5 types) | Partial (3 explicit) | Full |
| FlowSpec | Partial | Full |
| BGP-LS | Full | Full |
| VPLS | No | Yes |
| RTC | No | Yes |
| MVPN | Yes | No |
| MUP | Yes | No |

### 4.2 ADD-PATH Support

| NLRI Type | ZeBGP | ExaBGP |
|-----------|-------|--------|
| INET | Yes | Yes |
| IPVPN | Yes | Yes |
| EVPN | Yes | No (TODO) |
| BGP-LS | No | No (TODO) |
| FlowSpec | Yes | No |

---

## 5. FSM Comparison

### 5.1 State Definitions

Both use same RFC 4271 states:
- IDLE (0x01)
- ACTIVE (0x02)
- CONNECT (0x04)
- OPENSENT (0x08)
- OPENCONFIRM (0x10)
- ESTABLISHED (0x20)

### 5.2 Timer Defaults

| Timer | ZeBGP | ExaBGP | RFC 4271 |
|-------|-------|--------|----------|
| Hold Time | 90s | 180s | 90s |
| Connect Retry | 120s | N/A | 120s |
| Keepalive | hold/3 | hold/3 | hold/3 |

### 5.3 Architecture

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **FSM Architecture** | Event-driven | Procedural/Direct state change |
| **Timer Implementation** | time.Timer objects | Polling-based with timestamps |
| **Error Handling** | Event + channel signaling | Exception raising (Notify) |
| **Concurrency Model** | Goroutines + sync primitives | Asyncio single-threaded |

---

## 6. Reactor/Session Management Comparison

### 6.1 Architecture

| Aspect | ZeBGP | ExaBGP |
|--------|-------|--------|
| **Concurrency** | Goroutine-per-peer | Single event loop |
| **I/O Model** | Blocking with deadlines | Non-blocking async |
| **Thread Safety** | Mutex-protected | Single-threaded |
| **Timer Accuracy** | Precise (hardware) | Polling-based |

### 6.2 Missing Features in ZeBGP

1. Configuration Reloading (SIGHUP)
2. Advanced RIB (adj-rib-in/out)
3. FlowSpec processing
4. Graceful Restart (RFC 4724)
5. BMP support
6. Dynamic peer management via API
7. Configuration file parsing

---

## 7. Configuration Parsing Comparison

### 7.1 Neighbor Configuration

| Feature | ZeBGP | ExaBGP |
|---------|-------|--------|
| **Hold Time Default** | 90 seconds | Not specified |
| **Hold Time Validation** | 0-65535 | 0 or >= 3 (RFC) |
| **Local Address** | IP required | Supports 'auto' |
| **TTL Config** | ttl-security only | ttl-security + incoming/outgoing |
| **Manual EOR** | Not in schema | Supported |

### 7.2 Capability Configuration

| Feature | ZeBGP | ExaBGP |
|---------|-------|--------|
| **ASN4 Default** | true | true |
| **Route-Refresh Default** | Not specified | true |
| **Extended-Message** | Not present | Leaf, default=true |
| **Add-Path Type** | Flex (flag/value/block) | Enumeration |

---

## Critical Differences Requiring Attention

### Priority 1: Compatibility Issues

1. **RFC 8203/9003 Shutdown Communication**
   - ZeBGP: Not parsing UTF-8 shutdown messages
   - ExaBGP: Full support with length prefix + UTF-8 validation
   - **Impact:** Cannot display/handle graceful shutdown reasons

2. **Per-Message-Type Length Validation**
   - ZeBGP: Generic 19-byte minimum
   - ExaBGP: OPEN>=29, UPDATE>=23, KEEPALIVE==19, ROUTE_REFRESH==23
   - **Impact:** May accept malformed messages

3. **Extended Message Size Integration**
   - ZeBGP: Constants defined but not applied
   - ExaBGP: Applied immediately on negotiation
   - **Impact:** May fail with large updates

### Priority 2: Feature Parity

4. **RFC 9072 Extended Optional Parameters**
   - Allows capabilities >255 bytes
   - Required for large capability sets

5. **Enhanced Route Refresh (RFC 7313)**
   - Allows ORF-based filtering

6. **EVPN Types 1, 4**
   - Currently using generic wrapper

7. **Extended Communities IPv6 (RFC 5701)**
   - 20-byte format not implemented

### Priority 3: Enhancements

8. **MP-NLRI Chunking**
   - Respect attribute size limits in UPDATE fragmentation

9. **Family Validation**
   - Validate against negotiated capabilities

10. **Attribute Caching**
    - Consider selective caching for memory efficiency

---

## Recommended Action Items

### Short-term (Compatibility)
- [ ] Add RFC 8203/9003 shutdown communication parsing to NOTIFICATION
- [ ] Add per-message-type length validation in header parsing
- [ ] Integrate extended message size with capability negotiation
- [ ] Add KEEPALIVE payload validation (reject non-empty)

### Medium-term (Feature Parity)
- [ ] Implement RFC 9072 extended optional parameters
- [ ] Complete EVPN Type 1, 4 parsing
- [ ] Add Enhanced Route Refresh capability
- [ ] Add Extended Communities IPv6 support
- [ ] Add MP-NLRI chunking for large updates

### Long-term (Enhancements)
- [ ] Add configuration file reload (SIGHUP)
- [ ] Implement adj-rib-in/out
- [ ] Add FlowSpec VPN variant
- [ ] Consider attribute caching

---

## File Locations

### ZeBGP
- Messages: `pkg/bgp/message/`
- Capabilities: `pkg/bgp/capability/`
- Attributes: `pkg/bgp/attribute/`
- NLRI: `pkg/bgp/nlri/`
- FSM: `pkg/bgp/fsm/`
- Reactor: `pkg/reactor/`
- Config: `pkg/config/`

### ExaBGP
- Messages: `src/exabgp/bgp/message/`
- Capabilities: `src/exabgp/bgp/message/open/capability/`
- Attributes: `src/exabgp/bgp/message/update/attribute/`
- NLRI: `src/exabgp/bgp/message/update/nlri/`
- FSM: `src/exabgp/bgp/fsm.py`
- Reactor: `src/exabgp/reactor/`
- Config: `src/exabgp/configuration/`
