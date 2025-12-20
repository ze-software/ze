# ExaBGP Test Suite Inventory

**Purpose:** Catalog all tests for ZeBGP implementation reference
**Source:** `../main/qa/`
**Total tests:** ~92 test files

**Current status:** See `CLAUDE_CONTINUATION.md` in project root.

---

## Test Categories

| Category | Directory | Count | Format |
|----------|-----------|-------|--------|
| Encoding | `qa/encoding/` | 36 | `.ci` (config + expected wire + JSON) |
| Decoding | `qa/decoding/` | 18 | Raw message + expected JSON |
| API | `qa/api/` | 38 | `.ci` (config + commands + expected) |

---

## Test Format

### Encoding Tests (`.ci` format)

Each line contains:
- `option:file:` - Configuration file to use
- `N:cmd:` - Command to execute (N = test number)
- `N:raw:` - Expected wire format (hex)
- `N:json:` - Expected JSON output

**Example (`conf-addpath.ci`):**
```
option:file:conf-addpath.conf
1:cmd:announce route 193.0.2.1/32 next-hop 10.0.0.1 path-information 1.2.3.4 origin igp local-preference 100 extended-community [target:72:1]
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0040:02:00000020...
1:json:{"exabgp":"6.0.0","type":"update",...}
```

### Decoding Tests

Line 1: Message type description
Line 2: Raw hex bytes
Line 3: Expected JSON output

**Example (`bgp-evpn-1`):**
```
update l2vpn evpn
000000EA900F00E6001946...
{ "exabgp": "5.0.0", "type": "update", ... }
```

### API Tests (`.ci` format)

Similar to encoding but focused on API command sequences.

---

## Encoding Tests (36 files)

### Core BGP

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-attributes` | Basic attributes | ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF |
| `conf-extended-attributes` | Extended attributes | Communities, ext-communities, large-communities |
| `conf-aggregator` | Aggregator attribute | AGGREGATOR |
| `conf-ebgp` | EBGP session | External BGP basics |
| `conf-new-v4` | IPv4 routes | IPv4 unicast NLRI |
| `conf-new-v6` | IPv6 routes | IPv6 unicast NLRI |
| `conf-parity` | Parity check | Round-trip encoding |

### ADD-PATH

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-addpath` | ADD-PATH capability | Path-information encoding |
| `conf-path-information` | Path ID handling | Multiple paths per prefix |

### Capabilities

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-cap-software-version` | Software version cap | Capability 73 |
| `conf-hostname` | Hostname capability | FQDN capability |
| `conf-no-asn4` | 2-byte AS | AS_TRANS handling |
| `conf-unknowncap` | Unknown capability | Unknown capability passthrough |

### VPN

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-vpn` | VPNv4/VPNv6 | MPLS VPN encoding |
| `conf-ipself4` | IPv4 self-originated | IPv4 next-hop-self |
| `conf-ipself6` | IPv6 self-originated | IPv6 next-hop-self |
| `conf-ipv46routes4family` | Mixed routes | IPv4/v6 in IPv4 family |
| `conf-ipv46routes6family` | Mixed routes | IPv4/v6 in IPv6 family |
| `conf-ipv6grouping` | IPv6 grouping | Grouped IPv6 updates |

### FlowSpec

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-flow` | FlowSpec basic | Flow rules encoding |
| `conf-flow-redirect` | FlowSpec redirect | Redirect action |

### EVPN/L2VPN

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-l2vpn` | L2VPN | VPLS encoding |

### MVPN

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-mvpn` | Multicast VPN | MVPN route types |

### SR/SRv6

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-prefix-sid` | Prefix SID | SR extensions |
| `conf-srv6-mup` | SRv6 MUP | MUP route types |
| `conf-srv6-mup-v3` | SRv6 MUP v3 | MUP version 3 |

### Communities

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-largecommunity` | Large communities | Large community encoding |

### Other

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `conf-generic-attribute` | Generic attribute | Unknown attribute passthrough |
| `conf-group` | Neighbor groups | Group configuration |
| `conf-group-limit` | Group limits | Rate limiting |
| `conf-name` | Neighbor naming | Neighbor name config |
| `conf-split` | Split horizon | Split updates |
| `conf-template` | Templates | Template inheritance |
| `conf-watchdog` | Watchdog | Health checking |
| `extended-nexthop` | Extended next-hop | IPv6 NH for IPv4 |
| `unknown-message` | Unknown message | Unknown message handling |

---

## Decoding Tests (18 files)

### EVPN

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `bgp-evpn-1` | EVPN decode | MAC/IP advertisement parsing |

### FlowSpec

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `bgp-flow-1` | FlowSpec decode 1 | Basic flow rules |
| `bgp-flow-2` | FlowSpec decode 2 | Complex operators |
| `bgp-flow-3` | FlowSpec decode 3 | Multiple components |
| `bgp-flow-4` | FlowSpec decode 4 | Actions parsing |

### BGP-LS

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `bgp-ls-1` | BGP-LS node | Node NLRI |
| `bgp-ls-2` | BGP-LS link | Link NLRI |
| `bgp-ls-3` | BGP-LS prefix v4 | IPv4 prefix NLRI |
| `bgp-ls-4` | BGP-LS prefix v6 | IPv6 prefix NLRI |
| `bgp-ls-5` | BGP-LS TLVs 1 | Various TLVs |
| `bgp-ls-6` | BGP-LS TLVs 2 | More TLVs |
| `bgp-ls-7` | BGP-LS TLVs 3 | Additional TLVs |
| `bgp-ls-8` | BGP-LS SR | Segment routing |
| `bgp-ls-9` | BGP-LS SRv6 | SRv6 extensions |
| `bgp-ls-10` | BGP-LS complex | Complex message |

### IPv4 Unicast

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `ipv4-unicast-1` | Basic IPv4 | Simple UPDATE |
| `ipv4-unicast-2` | IPv4 with attrs | UPDATE with attributes |

### Capabilities

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `bgp-open-software-version` | Software version | OPEN with software cap |

---

## API Tests (38 files)

### Core API

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `api-api` | API basics | Basic API commands |
| `api-announce` | Announce routes | Route announcement |
| `api-announce-star` | Announce to all | Wildcard neighbor |
| `api-announce-processes-match` | Process matching | Process selector |
| `api-announcement` | Announcement format | Full announcement |
| `api-add-remove` | Add/remove routes | Dynamic routes |
| `api-attributes` | Attribute API | Attribute modification |
| `api-attributes-path` | Path attributes | AS_PATH modification |
| `api-attributes-vpn` | VPN attributes | VPN attribute handling |

### Control

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `api-ack-control` | ACK control | API acknowledgement |
| `api-check` | Config check | Configuration validation |
| `api-eor` | End of RIB | EOR generation |
| `api-manual-eor` | Manual EOR | Explicit EOR |
| `api-notification` | Notification | Send NOTIFICATION |
| `api-open` | OPEN message | OPEN handling |
| `api-reload` | Config reload | Reload configuration |
| `api-teardown` | Teardown session | Session teardown |

### RIB

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `api-rib` | RIB operations | RIB query |
| `api-rr-rib` | Route reflector RIB | RR RIB handling |
| `api-rr` | Route reflector | RR behavior |

### Neighbor

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `api-multi-neighbor` | Multiple neighbors | Multi-peer |
| `api-multisession` | Multiple sessions | Session handling |
| `api-peer-lifecycle` | Peer lifecycle | Connect/disconnect |

### NLRI Types

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `api-ipv4` | IPv4 API | IPv4 unicast |
| `api-ipv6` | IPv6 API | IPv6 unicast |
| `api-vpnv4` | VPNv4 API | VPN routes |
| `api-vpls` | VPLS API | L2VPN |
| `api-mvpn` | MVPN API | Multicast VPN |
| `api-flow` | FlowSpec API | Flow rules |
| `api-flow-merge` | FlowSpec merge | Flow rule merging |
| `api-broken-flow` | Broken FlowSpec | Error handling |

### Other

| Test | Description | Features Tested |
|------|-------------|-----------------|
| `api-fast` | Fast path | Performance |
| `api-health` | Health check | Healthcheck |
| `api-multiple-api` | Multiple APIs | Multi-process |
| `api-nexthop` | Next-hop | NH handling |
| `api-nexthop-self` | Next-hop self | NH self |
| `api-no-respawn` | No respawn | Process control |
| `api-silence-ack` | Silent ACK | Quiet mode |

---

## Test Runner Scripts

| Script | Purpose |
|--------|---------|
| `qa/bin/functional` | Main functional test runner |
| `qa/bin/test_everything` | Run all test suites |
| `qa/bin/test_api_encode` | API encoding tests |
| `qa/bin/test_json` | JSON format tests |
| `qa/bin/test_parsing` | Config parsing tests |
| `qa/bin/check_coverage` | Coverage analysis |
| `qa/bin/check_perf` | Performance tests |

---

## Priority for ZeBGP

### P0: Must Pass (Core functionality)

**Encoding:**
- `conf-attributes` - Basic attributes
- `conf-new-v4` - IPv4 unicast
- `conf-new-v6` - IPv6 unicast
- `conf-addpath` - ADD-PATH
- `conf-ebgp` - EBGP

**Decoding:**
- `ipv4-unicast-1` - Basic IPv4
- `ipv4-unicast-2` - IPv4 with attrs

**API:**
- `api-announce` - Basic announcement
- `api-ipv4` - IPv4 routes
- `api-ipv6` - IPv6 routes

### P1: Should Pass (Common features)

**Encoding:**
- `conf-vpn` - VPN
- `conf-extended-attributes` - Communities
- `conf-l2vpn` - L2VPN

**Decoding:**
- `bgp-evpn-1` - EVPN
- `bgp-flow-*` - FlowSpec

**API:**
- `api-vpnv4` - VPN
- `api-rib` - RIB ops

### P2: Nice to Pass (Advanced features)

**Encoding:**
- `conf-flow*` - FlowSpec
- `conf-mvpn` - MVPN
- `conf-srv6-mup*` - MUP

**Decoding:**
- `bgp-ls-*` - BGP-LS

**API:**
- `api-flow*` - FlowSpec
- `api-mvpn` - MVPN

---

## Test Conversion Strategy

1. **Parse `.ci` format** - Extract config, command, raw, json
2. **Create Go test files** - Table-driven tests
3. **Use raw bytes** - Test wire format directly
4. **Compare JSON** - Verify output format

**Example Go test structure:**
```go
func TestEncodingConfAddpath(t *testing.T) {
    tests := []struct {
        name     string
        command  string
        wantRaw  []byte
        wantJSON map[string]any
    }{
        {
            name:    "route with path-information",
            command: "announce route 193.0.2.1/32 ...",
            wantRaw: mustDecodeHex("FFFFFFFF..."),
            wantJSON: map[string]any{...},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Parse command
            // Generate wire format
            // Compare with expected
        })
    }
}
```

---

**Created:** 2025-12-19
**Last Updated:** 2025-12-19
