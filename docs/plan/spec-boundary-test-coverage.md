# Spec: boundary-test-coverage

## Task
Add missing boundary tests identified during TDD rule update audit.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/messages.md` - message length constraints
- [ ] `docs/architecture/wire/nlri.md` - prefix length limits

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - hold time, message length constraints

**Key insights:**
- Hold time must be 0 or >= 3 (RFC 4271)
- Standard message length 19-4096, extended 19-65535

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHoldTimeBoundary` | `internal/config/bgp_test.go` | hold time 65535 valid, 65536 invalid | |
| `TestMessageLengthBoundary` | `internal/bgp/message/header_test.go` | 4097 invalid (standard), 65535/65536 (extended) | |
| `TestIPv4PrefixLengthBoundary` | `internal/bgp/nlri/inet_test.go` | prefix len 33 invalid | |
| `TestIPv6PrefixLengthBoundary` | `internal/bgp/nlri/inet_test.go` | prefix len 129 invalid | |
| `TestFlowSpecDSCPBoundary` | `internal/plugin/update_text_test.go` | DSCP 64 invalid | |
| `TestFlowSpecICMPBoundary` | `internal/plugin/update_text_test.go` | ICMP type/code 256 invalid | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hold time | 0, 3-65535 | 0, 65535 | 1, 2 | 65536 |
| Message length (std) | 19-4096 | 4096 | 18 | 4097 |
| Message length (ext) | 19-65535 | 65535 | 18 | 65536 |
| IPv4 prefix len | 0-32 | 32 | N/A | 33 |
| IPv6 prefix len | 0-128 | 128 | N/A | 129 |
| FlowSpec DSCP | 0-63 | 63 | N/A | 64 |
| FlowSpec ICMP | 0-255 | 255 | N/A | 256 |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | | Boundary validation is unit-level | |

## Files to Modify
- `internal/config/bgp_test.go` - add hold time boundary tests
- `internal/bgp/message/header_test.go` - add message length boundary tests
- `internal/bgp/nlri/inet_test.go` - add prefix length boundary tests
- `internal/plugin/update_text_test.go` - add FlowSpec boundary tests

## Files to Create
- None (extending existing test files)

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Write unit tests** - Add boundary test cases to existing test files
2. **Run tests** - Verify FAIL for new invalid cases (paste output)
3. **Implement** - Add validation if missing
4. **Run tests** - Verify PASS (paste output)
5. **Verify all** - `make lint && make test && make functional` (paste output)

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
