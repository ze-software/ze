# Spec: mandatory-local-address

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/schema/ze-bgp.yang` - YANG peer-fields grouping
4. `internal/config/bgp.go` - PeerConfig parsing

## Task

Make `local-address` mandatory for all peers. Without explicit local address, TCP source IP selection is OS-dependent, causing:
- Inconsistent next-hop self behavior
- Connection issues with multi-homed routers
- Difficulty debugging connection problems

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config validation patterns

### Source Files
- [ ] `internal/plugin/bgp/schema/ze-bgp.yang` - YANG schema (line 110-113: local-address leaf)
- [ ] `internal/config/bgp.go` - Go parsing (lines 917-927: local-address parsing)
- [ ] `internal/config/bgp_test.go` - existing tests with local-address

**Key insights:**
- `peer-as` already uses `mandatory true;` pattern (line 122)
- `local-address` accepts IP address string or "auto" keyword
- "auto" sets `LocalAddressAuto=true` flag for OS-selected binding
- Most existing tests already include `local-address`

## Design Decisions

### "auto" Remains Valid

The `local-address auto;` syntax is still accepted. It means:
- Explicit acknowledgment that OS chooses source IP
- User consciously opted for non-deterministic behavior
- Better than silent default

### Validation Location

Validation happens in Go config parsing, not YANG:
- YANG `mandatory true;` only checks presence, not value validity
- Go parsing already validates IP format
- Add check: reject if neither `LocalAddress` nor `LocalAddressAuto` is set

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerMissingLocalAddress` | `internal/config/bgp_test.go` | Peer without local-address rejected | |
| `TestPeerLocalAddressAuto` | `internal/config/bgp_test.go` | "auto" is accepted | ✅ exists |
| `TestPeerLocalAddressIP` | `internal/config/bgp_test.go` | IP address is accepted | ✅ exists |

### Boundary Tests
N/A - this is presence validation, not numeric range.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `peer-missing-local-address` | `test/data/parse/invalid/peer-missing-local-address.conf` | Config rejected with clear error | |

## Files to Modify

- `internal/plugin/bgp/schema/ze-bgp.yang` - Add `mandatory true;` to local-address leaf
- `internal/config/bgp.go` - Add validation after parsing peer config
- `internal/config/bgp_test.go` - Add rejection test

## Files to Create

- `test/data/parse/invalid/peer-missing-local-address.conf` - Invalid config
- `test/data/parse/invalid/peer-missing-local-address.expect` - Expected error

## Implementation Steps

1. **Write unit test** - Test that peer without local-address is rejected
   → **Review:** Does test check error message mentions "local-address"?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Test fails because validation doesn't exist yet?

3. **Add YANG mandatory** - Add `mandatory true;` to local-address leaf
   → **Review:** Matches peer-as pattern at line 122?

4. **Add Go validation** - Check in `parsePeerConfig()` after parsing
   → **Review:** Clear error message? Mentions "local-address required"?

5. **Run tests** - Verify PASS (paste output)
   → **Review:** All existing tests still pass?

6. **Add functional test** - Create invalid config test
   → **Review:** Error expectation matches actual message?

7. **Verify all** - `make lint && make test && make functional` (paste output)

## Error Message

```
peer 192.0.2.1: local-address is required (use IP address or "auto")
```

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Feature code integrated into codebase (`internal/*`)
- [ ] Functional tests verify end-user behavior (`.conf` + `.expect` files)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] YANG schema updated
- [ ] Error message is clear and actionable
