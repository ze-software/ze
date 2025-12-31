# Spec: API Test Feature Implementation

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/api/*.go, test/data/api/*.ci - Current implementation   │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Implement remaining API features required to pass ExaBGP-compatible tests.

## Current Test Status (Updated 2025-12-31)

| Test | Status | Blocking Feature |
|------|--------|------------------|
| add-remove | ✅ PASS | - |
| announce | ✅ PASS | - |
| eor | ✅ PASS | - |
| fast | ✅ PASS | - |
| nexthop | ✅ PASS | - |
| ipv4 | ✅ PASS | - |
| ipv6 | ✅ PASS | - |
| attributes | ✅ PASS | - |
| teardown | ✅ PASS | - (was already implemented) |
| notification | ✅ PASS | - (was already implemented) |
| check | ❌ FAIL | Static routes not sent (EORs missing) |
| watchdog | ✅ PASS | - (was already implemented) |
| mup4 | ❌ FAIL | MUP API not implemented |
| mup6 | ❌ FAIL | MUP API not implemented |
| announcement | ❌ FAIL | Multi-session qualifiers (NOT SUPPORTED) |

### Progress Notes

**Fixed in this session:**
- API bindings from templates now properly inherited (commit f900669)
- Fixed mup4.conf and mup6.conf to reference correct .run files

**check test status:**
- Previously: timeout (API bindings not inherited from templates)
- Now: mismatch (receives KEEPALIVE instead of EORs/static routes)
- Root cause: Static routes from config not being sent to testpeer
- Receive updates forwarding appears to work (needs verification)

**mup4/mup6 tests:**
- Config fixed but MUP SAFI not supported in API commands
- `parseSAFI()` only supports: unicast, nlri-mpls, mpls-vpn
- Separate feature gap - not in original spec scope

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- Check ExaBGP for API compatibility
- Document intentional deviations from ExaBGP

## Codebase Context

### Files to Modify

| Feature | Files |
|---------|-------|
| teardown | `pkg/api/command.go`, `pkg/reactor/reactor.go` |
| notification | `test/cmd/zebgp-peer/main.go`, verify `pkg/reactor/session.go` |
| receive updates | `pkg/reactor/session.go`, `pkg/api/process.go` |
| watchdog | `pkg/api/watchdog.go` (new), `pkg/reactor/reactor.go` |

## Implementation Steps

### Phase 1: `neighbor X teardown N` Command

1. Check ExaBGP teardown implementation
2. Write test for teardown command parsing - MUST FAIL
3. Register handler for `neighbor <ip> teardown <code>`
4. Send NOTIFICATION with code (4 = Administrative Reset)
5. Close TCP, allow auto-reconnect
6. Run tests - MUST PASS
7. Run functional test: `go run ./test/cmd/functional api teardown`

### Phase 2: NOTIFICATION CI Format

1. Verify session.go sends NOTIFICATION on graceful close
2. Write test for notification parsing in testpeer - MUST FAIL
3. Update testpeer to handle `notification:` directive
4. Run tests - MUST PASS
5. Run functional test

### Phase 3: Receive Updates to Script (High Effort)

1. Check ExaBGP receive implementation
2. Write test for UPDATE forwarding - MUST FAIL
3. Add callback in session receive path for UPDATE messages
4. Format UPDATE as text and write to API process stdin
5. Handle both announce and withdraw
6. Add config: `api { receive { parsed; update; } }`
7. Run tests - MUST PASS

### Phase 4: Watchdog Subsystem (High Effort)

1. Check ExaBGP watchdog implementation
2. Write test for watchdog commands - MUST FAIL
3. Create `pkg/api/watchdog.go`
4. Register handlers for `announce watchdog`, `withdraw watchdog`
5. Maintain watchdog state (name -> last_seen timestamp)
6. Background goroutine checks for expired watchdogs
7. On expiry, withdraw routes associated with that watchdog
8. Run tests - MUST PASS

## Verification Checklist

- [x] TDD followed: Each test shown to FAIL first
- [x] ExaBGP compatibility verified for each feature
- [ ] Functional tests pass for implemented features (11/14 passing)
- [x] `make test` passes
- [ ] `make lint` passes (pre-existing issues)

## Priority Order

| Priority | Feature | Test | Effort |
|----------|---------|------|--------|
| 1 | teardown | teardown | 3h |
| 2 | notification | notification | 2h |
| 3 | receive updates | check | 6h+ |
| 4 | watchdog | watchdog | 6h+ |

## Skip/Defer

| Feature | Test | Reason |
|---------|------|--------|
| Multi-session qualifiers | announcement | Not supported by design |

## Success Criteria

1. ✅ `teardown` and `notification` tests pass
2. ✅ `watchdog` test passes
3. ❌ `check` test - needs static route sending fix
4. ✅ All previously passing tests still pass
5. ✅ `make test` passes

## Remaining Work

### check test (Priority: Medium)
The check test requires static routes from config to be sent to testpeer.
Current issue: ZeBGP sends KEEPALIVE but not EORs or static routes.
Investigation needed: Why aren't static routes being announced?

### mup4/mup6 tests (Priority: Low - separate feature)
MUP SAFI support in API commands:
- Add "mup" to `parseSAFI()` function
- Implement MUP-specific announce/withdraw handlers
- This is a separate feature not in the original spec scope
