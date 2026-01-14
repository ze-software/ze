# Spec: nlri-format-command-style

## Task

Update ALL NLRI text output formats to use command-style syntax with `set` keyword for consistency.

**Motivation:**
- Current formats are inconsistent (some use `field:value`, others `field=value`)
- Command-style `field set value` provides clear, consistent syntax for display/logging
- Output uses same vocabulary as API commands (enables copy-paste to reconstruct)

**Scope:**
- String() is for **display/debugging** with command-style syntax
- NOT for direct round-trip parsing (parser uses full command syntax with family/action)
- Format enables understanding NLRI content; parser reconstruction requires wrapper command

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command syntax
- [ ] `docs/architecture/wire/nlri.md` - NLRI types and families

### RFC Summaries
- [ ] `docs/rfc/rfc7432.md` - EVPN route types
- [ ] `docs/rfc/rfc4364.md` - VPN routes (IPVPN)
- [ ] `docs/rfc/rfc8277.md` - Labeled Unicast

**Key insights:**
- API uses `<field> set <value>` for scalar attributes
- NLRI output should match input format for round-trip parsing
- All NLRI types should follow same pattern for consistency

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEVPNType1StringCommandStyle` | `pkg/bgp/nlri/evpn_test.go` | Type 1 command-style | ✅ |
| `TestEVPNType2StringCommandStyle` | `pkg/bgp/nlri/evpn_test.go` | Type 2 command-style | ✅ |
| `TestEVPNType2WithIPStringCommandStyle` | `pkg/bgp/nlri/evpn_test.go` | Type 2 with IP | ✅ |
| `TestEVPNType3StringCommandStyle` | `pkg/bgp/nlri/evpn_test.go` | Type 3 command-style | ✅ |
| `TestEVPNType4StringCommandStyle` | `pkg/bgp/nlri/evpn_test.go` | Type 4 command-style | ✅ |
| `TestEVPNType5StringCommandStyle` | `pkg/bgp/nlri/evpn_test.go` | Type 5 command-style | ✅ |
| `TestIPVPNStringCommandStyle` | `pkg/bgp/nlri/ipvpn_test.go` | IPVPN command-style | ✅ |
| `TestLabeledUnicastStringCommandStyle` | `pkg/bgp/nlri/labeled_test.go` | Labeled command-style | ✅ |
| `TestVPLSStringCommandStyle` | `pkg/bgp/nlri/other_test.go` | VPLS command-style | ✅ |
| `TestFlowSpecVPNStringCommandStyle` | `pkg/bgp/nlri/flowspec_test.go` | FlowSpecVPN command-style | ✅ |
| `TestINETStringCommandStyle` | `pkg/bgp/nlri/inet_test.go` | INET command-style | ✅ |
| `TestMVPNStringCommandStyle` | `pkg/bgp/nlri/other_test.go` | MVPN command-style | ✅ |
| `TestRTCStringCommandStyle` | `pkg/bgp/nlri/other_test.go` | RTC command-style | ✅ |
| `TestMUPStringCommandStyle` | `pkg/bgp/nlri/other_test.go` | MUP command-style | ✅ |
| `TestBGPLSNodeStringCommandStyle` | `pkg/bgp/nlri/bgpls_test.go` | BGP-LS Node | ✅ |
| `TestBGPLSLinkStringCommandStyle` | `pkg/bgp/nlri/bgpls_test.go` | BGP-LS Link | ✅ |
| `TestBGPLSPrefixStringCommandStyle` | `pkg/bgp/nlri/bgpls_test.go` | BGP-LS Prefix | ✅ |
| `TestBGPLSSRv6SIDStringCommandStyle` | `pkg/bgp/nlri/bgpls_test.go` | BGP-LS SRv6 SID | ✅ |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Changes are in String() methods, tested via unit tests | |

## Files to Modify

### Phase 1 (EVPN) - ✅ DONE
- `pkg/bgp/nlri/evpn.go` - Update String() for EVPNType1-5
- `pkg/bgp/nlri/evpn_test.go` - Add command-style tests

### Phase 2 (VPN/Labeled) - ✅ DONE
- `pkg/bgp/nlri/ipvpn.go` - Update IPVPN.String()
- `pkg/bgp/nlri/ipvpn_test.go` - Add tests
- `pkg/bgp/nlri/labeled.go` - Update LabeledUnicast.String()
- `pkg/bgp/nlri/labeled_test.go` - Add tests

### Phase 3 (Other NLRI types) - ✅ DONE
- `pkg/bgp/nlri/other.go` - Update VPLS.String()
- `pkg/bgp/nlri/flowspec.go` - Update FlowSpecVPN.String()

### Phase 4 (Remaining NLRI types) - ✅ DONE
- `pkg/bgp/nlri/inet.go` - Update INET.String()
- `pkg/bgp/nlri/other.go` - Update MVPN.String(), RTC.String(), MUP.String()
- `pkg/bgp/nlri/bgpls.go` - Update BGPLSNode.String(), BGPLSLink.String(), BGPLSPrefix.String(), BGPLSSRv6SID.String()
- `pkg/plugin/mpwire_test.go` - Fix existing tests for new path-id format

## Format Specification

### EVPN Types (Phase 1 - ✅ DONE)

| Type | Current | Target |
|------|---------|--------|
| Type 1 | `type1 RD:<rd> ESI:<esi> tag:<tag>` | `ethernet-ad rd set <rd> esi set <esi> etag set <tag> label set <label>` |
| Type 2 | `type2 RD:<rd> MAC:<mac> IP:<ip>` | `mac-ip rd set <rd> mac set <mac> [ip set <ip>] [etag set <tag>] [label set <label>]` |
| Type 3 | `type3 RD:<rd> originator:<ip>` | `multicast rd set <rd> ip set <ip> [etag set <tag>]` |
| Type 4 | `type4 RD:<rd> ESI:<esi> originator:<ip>` | `ethernet-segment rd set <rd> esi set <esi> ip set <ip>` |
| Type 5 | `type5 RD:<rd> prefix:<prefix>` | `ip-prefix rd set <rd> prefix set <prefix> [esi set <esi>] [gateway set <gw>] [label set <label>]` |

### VPN/Labeled Types (Phase 2)

| Type | Current | Target |
|------|---------|--------|
| IPVPN | `RD:<rd> <prefix> labels=<labels>` | `rd set <rd> prefix set <prefix> label set <labels> [path-id set <id>]` |
| LabeledUnicast | `<prefix> label=<label>` | `prefix set <prefix> label set <label> [path-id set <id>]` |

### Other Types (Phase 3)

| Type | Current | Target |
|------|---------|--------|
| VPLS | `vpls:<rd> ve=<id> label=<label>` | `rd set <rd> ve-id set <id> label set <label>` |
| FlowSpecVPN | `flowspec-vpn(rd:<rd> <spec>)` | `rd set <rd> <flowspec-components>` |

### Remaining Types (Phase 4)

| Type | Current | Target |
|------|---------|--------|
| INET | `<prefix> path-id=<id>` | `<prefix> [path-id set <id>]` |
| FlowSpec | `flowspec(<components>)` | `flowspec(<components>)` (keep as-is, internal) |
| MVPN | `mvpn:<type> rd=<rd>` | `<type> [rd set <rd>]` |
| RTC | `rtc:as<asn>:<rt>` | `origin-as set <asn> rt set <rt>` |
| MUP | `mup:<type> rd=<rd>` | `<type> [rd set <rd>]` |
| BGPLSNode | `bgp-ls:node(asn=<n>)` | `node protocol set <proto> asn set <n>` |
| BGPLSLink | `bgp-ls:link(<n>-><m>)` | `link protocol set <proto> local-asn set <n> remote-asn set <m>` |
| BGPLSPrefix | `bgp-ls:prefix(<type>)` | `prefix protocol set <proto> type set <type> asn set <n>` |
| BGPLSSRv6SID | `bgp-ls:srv6-sid(asn=<n>)` | `srv6-sid protocol set <proto> asn set <n>` |

## Implementation Steps

### Phase 1 (EVPN) - ✅ DONE
1. **Write unit tests** - Add tests for command-style String() output
2. **Run tests** - Verify FAIL
3. **Implement** - Update EVPN String() methods
4. **Run tests** - Verify PASS
5. **Verify all** - `make lint && make test && make functional`

### Phase 2 (VPN/Labeled)
1. **Write unit tests** - TestIPVPNStringCommandStyle, TestLabeledUnicastStringCommandStyle
2. **Run tests** - Verify FAIL
3. **Implement** - Update IPVPN.String(), LabeledUnicast.String()
4. **Run tests** - Verify PASS
5. **Verify all** - `make lint && make test && make functional`

### Phase 3 (Other) - ✅ DONE
1. **Write unit tests** - TestVPLSStringCommandStyle
2. **Run tests** - Verify FAIL
3. **Implement** - Update VPLS.String(), FlowSpecVPN.String()
4. **Run tests** - Verify PASS
5. **Verify all** - `make lint && make test && make functional`

### Phase 4 (Remaining)
1. **Write unit tests** - INET, MVPN, RTC, MUP, BGP-LS tests
2. **Run tests** - Verify FAIL
3. **Implement** - Update all remaining String() methods
4. **Fix style** - Normalize IPVPN to use fmt.Sprintf like others
5. **Run tests** - Verify PASS
6. **Verify all** - `make lint && make test && make functional`

## Implementation Summary

### Phase 1 - EVPN (Completed)
- Updated `EVPNType1.String()` to output `ethernet-ad rd set <rd> esi set <esi> etag set <tag> label set <label>`
- Updated `EVPNType2.String()` to output `mac-ip rd set <rd> mac set <mac> [ip set <ip>] [etag set <tag>] [label set <label>]`
- Updated `EVPNType3.String()` to output `multicast rd set <rd> ip set <ip> [etag set <tag>]`
- Updated `EVPNType4.String()` to output `ethernet-segment rd set <rd> esi set <esi> ip set <ip>`
- Updated `EVPNType5.String()` to output `ip-prefix rd set <rd> prefix set <prefix> [esi set <esi>] [etag set <tag>] [gateway set <gw>] [label set <label>]`
- Added 6 tests: TestEVPNType1-5StringCommandStyle, TestEVPNType2WithIPStringCommandStyle

### Phase 2 - VPN/Labeled (Completed)
- Updated `IPVPN.String()` to output `rd set <rd> prefix set <prefix> [label set <labels>] [path-id set <id>]`
- Updated `LabeledUnicast.String()` to output `prefix set <prefix> [label set <labels>] [path-id set <id>]`
- Added TestIPVPNStringCommandStyle (5 test cases)
- Added TestLabeledUnicastStringCommandStyle (6 test cases)

### Phase 3 - Other NLRI (Completed)
- Updated `VPLS.String()` to output `rd set <rd> ve-id set <id> label set <label>`
- Updated `FlowSpecVPN.String()` to output `rd set <rd> <flowspec>`
- Added TestVPLSStringCommandStyle (2 test cases)
- Added TestFlowSpecVPNStringCommandStyle (2 test cases)

### Phase 4 - Remaining NLRI (Completed)
- Updated `INET.String()` to output `<prefix> [path-id set <id>]`
- Updated `MVPN.String()` to output `<type> [rd set <rd>]`
- Updated `RTC.String()` to output `default` or `origin-as set <asn> rt set <rt>`
- Updated `MUP.String()` to output `<type> [rd set <rd>]`
- Updated `BGPLSNode.String()` to output `node protocol set <proto> asn set <n>`
- Updated `BGPLSLink.String()` to output `link protocol set <proto> local-asn set <n> remote-asn set <m>`
- Updated `BGPLSPrefix.String()` to output `prefix protocol set <proto> type set <type> asn set <n>`
- Updated `BGPLSSRv6SID.String()` to output `srv6-sid protocol set <proto> asn set <n>`
- Added TestINETStringCommandStyle (4 test cases)
- Added TestMVPNStringCommandStyle (3 test cases)
- Added TestRTCStringCommandStyle (3 test cases)
- Added TestMUPStringCommandStyle (3 test cases)
- Added TestBGPLSNodeStringCommandStyle (2 test cases)
- Added TestBGPLSLinkStringCommandStyle (2 test cases)
- Added TestBGPLSPrefixStringCommandStyle (2 test cases)
- Added TestBGPLSSRv6SIDStringCommandStyle (2 test cases)
- Fixed existing tests in pkg/plugin/mpwire_test.go (2 tests)

### Critical Review Fixes
- Added protocol field to all BGP-LS String() methods (was missing critical context)
- Clarified spec: String() is for display/debugging, not direct round-trip parsing

### Design Insights
- Using strings.Builder for efficient string concatenation
- Optional fields (ip, etag, gateway, label) only output when present/non-zero
- Label stacks formatted as comma-separated values
- path-id only output when non-zero

### Deviations from Plan
- Also added Type 1 and Type 4 tests (not originally in plan)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (18 decoding, 12 parsing tests)

### Documentation
- [x] Required docs read
- [x] RFC references added to code

### Completion
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/119-nlri-format-command-style.md`
