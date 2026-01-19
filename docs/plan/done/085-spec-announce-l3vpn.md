# Spec: announce-l3vpn

## Task
Implement `AnnounceL3VPN` and `WithdrawL3VPN` in reactor to enable L3VPN (MPLS VPN) route announcements via API.

## Required Reading
- [ ] `.claude/zebgp/wire/NLRI.md` - VPN NLRI format (RD + labels + prefix)
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - BuildVPN usage pattern
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - API reactor interface

**Key insights:**
- VPN routes use SAFI 128 with RD + label stack + prefix in NLRI
- BuildVPN in UpdateBuilder handles wire encoding
- Existing AnnounceLabeledUnicast pattern can be adapted for L3VPN
- RT (Route Target) is encoded as extended community (RFC 4360)

## Behavior Table

| Item | Behavior | Rationale |
|------|----------|-----------|
| Config nlri-mpls without label | Requires label (error) | RFC 4364 mandates labels for VPN |
| Both label and labels specified | labels takes precedence | Deterministic behavior |
| Withdrawal uses single label | Uses first label from stack | RFC allows - prefix identifies route |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestBuildL3VPNParams` | `internal/reactor/reactor_test.go` | api.L3VPNRoute to VPNParams conversion |
| `TestBuildL3VPNParamsIPv6` | `internal/reactor/reactor_test.go` | IPv6 VPN support |
| `TestBuildL3VPNRIBRoute` | `internal/reactor/reactor_test.go` | RIB route building for queueing |
| `TestParseRouteTarget_2ByteASN` | `internal/reactor/reactor_test.go` | 2-byte ASN RT encoding (Type 0) |
| `TestParseRouteTarget_4ByteASN` | `internal/reactor/reactor_test.go` | 4-byte ASN RT encoding (Type 2) |
| `TestParseRouteTarget_IPv4` | `internal/reactor/reactor_test.go` | IP:NN RT encoding (Type 1) |
| `TestParseRouteTarget_WithPrefix` | `internal/reactor/reactor_test.go` | target: prefix stripping |
| `TestParseRouteTarget_Errors` | `internal/reactor/reactor_test.go` | Error handling for invalid RT |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | Existing VPN functional tests cover wire format |

## Files to Modify
- `internal/reactor/reactor.go` - Add AnnounceL3VPN, WithdrawL3VPN, buildL3VPNParams, buildL3VPNRIBRoute, parseRouteTarget
- `internal/reactor/reactor_test.go` - Add L3VPN and parseRouteTarget tests

## Implementation Steps
1. **Write tests** - Create TestBuildL3VPNParams, TestBuildL3VPNParamsIPv6, TestBuildL3VPNRIBRoute
2. **Run tests** - Verify FAIL (methods don't exist)
3. **Implement** - Add buildL3VPNParams, buildL3VPNRIBRoute, parseRouteTarget, update AnnounceL3VPN/WithdrawL3VPN
4. **Run tests** - Verify PASS
5. **Verify all** - `make lint && make test && make functional`

## RFC Documentation
- RFC 4364 Section 4 - BGP/MPLS IP VPNs (SAFI 128, RD encoding)
- RFC 4360 Section 3 - Extended Communities (Route Target format)
- RFC 8277 Section 2 - MPLS label stack encoding

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output: methods undefined)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (no new issues; 50 pre-existing SA1019 warnings)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references added (comments in code)
- [ ] `.claude/zebgp/` updated if schema changed (N/A)

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
