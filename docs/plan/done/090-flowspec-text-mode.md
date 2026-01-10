# Spec: flowspec-text-mode

## Task
Add FlowSpec NLRI support to the text mode parser (`ParseUpdateText`), enabling API commands like:
```
nlri ipv4/flowspec add destination 10.0.0.0/24 protocol tcp destination-port =80
```

Also add extended community function syntax for FlowSpec actions:
```
extended-community set traffic-rate 65000 1000000
extended-community set discard
```

## Required Reading
- [x] `.claude/zebgp/wire/NLRI.md` - FlowSpec wire format, component types
- [x] `.claude/zebgp/api/ARCHITECTURE.md` - Update text parser structure
- [x] `rfc/rfc8955.txt` - FlowSpec v2 (obsoletes RFC 5575)
- [ ] `rfc/rfc8956.txt` - IPv6 FlowSpec (if needed)

**Key insights:**
- FlowSpec NLRI = ordered list of match components (not a prefix)
- Components: destination, source, protocol, port, tcp-flags, etc. (Types 1-13)
- Actions encoded as extended communities (traffic-rate, redirect, traffic-marking)
- One `add`/`del` = one FlowSpec rule (unlike prefix families with multiple prefixes)
- Existing `pkg/bgp/nlri/flowspec.go` has full wire format support

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestParseUpdateText_FlowSpecBasic` | `pkg/plugin/update_text_test.go` | Basic flowspec with destination only |
| `TestParseUpdateText_FlowSpecProtocol` | `pkg/plugin/update_text_test.go` | Protocol component (tcp/udp/icmp/number) |
| `TestParseUpdateText_FlowSpecPort` | `pkg/plugin/update_text_test.go` | Port with operators (=, >, <, >=, <=) |
| `TestParseUpdateText_FlowSpecPortRange` | `pkg/plugin/update_text_test.go` | Port range (>=1 <=1023) |
| `TestParseUpdateText_FlowSpecMultipleComponents` | `pkg/plugin/update_text_test.go` | Multiple match components (AND logic) |
| `TestParseUpdateText_FlowSpecWithdraw` | `pkg/plugin/update_text_test.go` | del syntax for flowspec |
| `TestParseUpdateText_FlowSpecVPN` | `pkg/plugin/update_text_test.go` | flowspec-vpn with rd |
| `TestParseUpdateText_FlowSpecIPv6` | `pkg/plugin/update_text_test.go` | ipv6/flowspec family |
| `TestParseUpdateText_FlowSpecTCPFlags` | `pkg/plugin/update_text_test.go` | TCP flags matching |
| `TestParseUpdateText_FlowSpecFragment` | `pkg/plugin/update_text_test.go` | Fragment component |
| `TestParseUpdateText_FlowSpecMissingAdd` | `pkg/plugin/update_text_test.go` | Error: components without add/del |
| `TestParseUpdateText_ExtCommTrafficRate` | `pkg/plugin/update_text_test.go` | traffic-rate function syntax |
| `TestParseUpdateText_ExtCommDiscard` | `pkg/plugin/update_text_test.go` | discard sugar |
| `TestParseUpdateText_ExtCommRedirect` | `pkg/plugin/update_text_test.go` | redirect function syntax |
| `TestParseUpdateText_ExtCommTrafficMarking` | `pkg/plugin/update_text_test.go` | traffic-marking function syntax |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `flowspec.run` | `test/data/api/` | End-to-end flowspec announce/withdraw |

## Files to Modify
- `pkg/plugin/update_text.go` - Add flowspec parsing to `parseNLRISection`
- `pkg/plugin/update_text_test.go` - Add tests
- `pkg/plugin/types.go` - Add FlowSpec-related types if needed
- `pkg/bgp/attribute/extcomm.go` - Add extended community function constructors (if not exists)

## Grammar

### FlowSpec NLRI Section
```
<nlri-section>     := nlri <flowspec-family> [rd <value>] <flowspec-op>+
<flowspec-op>      := add <component>+ | del <component>+

<flowspec-family>  := ipv4/flowspec | ipv6/flowspec
                    | ipv4/flowspec-vpn | ipv6/flowspec-vpn

<component>        := destination <prefix>
                    | source <prefix>
                    | protocol <proto>+
                    | port <op><value>+
                    | destination-port <op><value>+
                    | source-port <op><value>+
                    | icmp-type <value>+
                    | icmp-code <value>+
                    | tcp-flags <bitmask-match>+
                    | packet-length <op><value>+
                    | dscp <value>+
                    | fragment <bitmask-match>+

<op>               := = | > | >= | < | <=    # default is =
<proto>            := tcp | udp | icmp | gre | <number>

<bitmask-match>    := [&][!][=]<flag>[&<flag>...]
<flag>             := syn | ack | fin | rst | psh | push | urg | ece | cwr  # tcp-flags (RFC 3168)
                    | dont-fragment | is-fragment | first-fragment | last-fragment  # fragment
```

**Note:** `push` is an alias for `psh` (ExaBGP compatibility). `ece` and `cwr` are ECN flags per RFC 3168.

### Value Ranges (validated at parse time)

| Component | Range | Bits | Error on overflow |
|-----------|-------|------|-------------------|
| protocol, icmp-type, icmp-code | 0-255 | 8 | Yes |
| port, destination-port, source-port, packet-length | 0-65535 | 16 | Yes |
| dscp | 0-63 | 6 | Yes |

### Bitmask Operators (RFC 8955 Section 4.2.1.2)

| Syntax | Meaning | Wire Op |
|--------|---------|---------|
| `flag` | match if ANY of the flags are set | 0x00 (INCLUDE) |
| `=flag` | match if EXACTLY these flags are set | 0x01 (Match) |
| `!flag` | match if flag is NOT set | 0x02 (Not) |
| `!=flag` | match if NOT exactly these flags | 0x03 (Not+Match) |
| `flag1&flag2` | combine flags in same match | combined value |
| `&flag` | AND with previous match (vs OR) | 0x40 (And bit) |

**Examples:**
```
tcp-flags syn          # SYN is set (any bit match)
tcp-flags =syn         # ONLY SYN is set (exact match)
tcp-flags !rst         # RST is NOT set
tcp-flags =syn&ack     # exactly SYN+ACK set
fragment !is-fragment  # NOT a fragment
```

### Extended Community Functions
```
<ext-comm-value>   := <type>:<v1>:<v2>           # existing colon syntax
                    | traffic-rate <asn> <rate>   # function form
                    | redirect <asn|ip> <value>   # function form
                    | traffic-marking <dscp>      # function form
                    | traffic-action <flags>      # function form
                    | discard                     # sugar for traffic-rate 0 0
                    | target <asn|ip> <value>     # RT function form
                    | origin <asn|ip> <value>     # RO function form
```

## Implementation Steps

1. **Write tests** - Create unit tests for flowspec parsing
2. **Run tests** - Verify FAIL (paste output)
3. **Add flowspec families** - Update `isSupportedFamily()` for flowspec
4. **Implement parseFlowSpecSection** - Parse match components until add/del
5. **Add match component parsers** - One per component type
6. **Implement extcomm functions** - traffic-rate, redirect, discard, etc.
7. **Run tests** - Verify PASS (paste output)
8. **Add functional test** - End-to-end flowspec test
9. **Verify all** - `make lint && make test && make functional`

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| `add` before matches | Consistent with prefix families (add comes before content) |
| One rule per add/del | FlowSpec matches combine with AND - can't batch disconnected rules |
| Protocol names (tcp/udp) | User-friendly, translate to IP protocol numbers |
| Operator prefix (=80) | Compact, unambiguous, matches RFC numeric operator concept |
| Extended comm functions | More readable than `[traffic-rate:65000:1000000]` |
| `discard` sugar | Common case deserves shorthand |

## RFC References
- RFC 8955 Section 4 - FlowSpec NLRI format
- RFC 8955 Section 4.2 - Component ordering (strict type order)
- RFC 8955 Section 4.2.2 - Component types 1-12
- RFC 8956 Section 3 - IPv6 FlowSpec extensions (type 13)
- RFC 8955 Section 7 - Traffic Filtering Actions (extended communities)

## Extended Community Types for FlowSpec Actions

| Action | Type Code | Format | Description |
|--------|-----------|--------|-------------|
| traffic-rate | 0x8006 | ASN(2) + Rate(4) | Rate limit in bytes/sec, 0 = discard |
| traffic-action | 0x8007 | Flags(6) | Sample/terminal-action bits |
| redirect | 0x8008 | ASN(2) + Value(4) | Redirect to VRF by RT |
| redirect-ip | 0x0800 | IP(4) + Value(2) | Redirect to VRF by IP-based RT |
| traffic-marking | 0x8009 | DSCP(1) | Set DSCP value |

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (initial: `family not supported in text mode: ipv4/flowspec`)
- [x] Implementation complete
- [x] Tests PASS (all 23 FlowSpec + ExtComm tests pass)

### Verification
- [x] `make lint` passes (nolint added for gocritic ifElseChain - order matters)
- [x] `make test` passes
- [x] `make functional` passes (37 tests)

### Documentation
- [x] Required docs read
- [x] RFC references added (RFC 8955 Section comments in code)
- [x] `.claude/zebgp/api/ARCHITECTURE.md` updated with flowspec grammar

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-flowspec-text-mode.md`

## Implementation Summary

### Files Modified

| File | Changes |
|------|---------|
| `pkg/plugin/update_text.go` | +550 lines: FlowSpec parsing, bitmask operators, value validation |
| `pkg/plugin/update_text_test.go` | +550 lines: 130+ test cases |
| `pkg/plugin/route.go` | +100 lines: ExtComm function syntax |
| `pkg/bgp/nlri/flowspec.go` | +20 lines: FlowOpNot, FlowOpMatch, NewFlowFragmentMatchComponent |

### Bug Fixes from Critical Review

| Issue | Fix |
|-------|-----|
| Missing ECE/CWR TCP flags | Added `ece` (0x40), `cwr` (0x80) per RFC 3168 |
| Missing `push` alias | Added for ExaBGP compatibility |
| No value range validation | Added `flowSpecComponentMaxValue()` with per-component limits |

### Test Coverage

| Category | Tests |
|----------|-------|
| All component types | 16 subtests |
| Numeric operators (all components) | 24 subtests |
| TCP flags bitmask operators | 22 subtests |
| Fragment bitmask operators | 14 subtests |
| Multi-component combinations | 9 subtests |
| IPv6 variants | 5 subtests |
| VPN variants (IPv4/IPv6) | 4 subtests |
| Withdraw variants | 4 subtests |
| Error handling | 11 subtests |
| Extended community + FlowSpec | 4 subtests |

**Total: ~120 test cases across 12 test functions**
