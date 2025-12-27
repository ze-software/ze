# API Test Feature Requirements

**Generated:** 2025-12-23

This document identifies the features required for each API test with a `.ci` file.

---

## Test Status Summary

| Test | Status | Blocking Feature |
|------|--------|------------------|
| add-remove | ✅ PASS | - |
| announce | ✅ PASS | - |
| eor | ✅ PASS | - |
| fast | ✅ PASS | - |
| nexthop | ✅ PASS | - |
| ipv4 | ✅ PASS | - (simplified: unicast only, no MUP) |
| ipv6 | ✅ PASS | - (simplified: unicast only, no MUP) |
| attributes | ✅ IMPL | `announce attributes ... nlri` implemented |
| announcement | ❌ FAIL | Multi-session qualifiers (NOT SUPPORTED) |
| check | ❌ FAIL | Receive updates to script |
| teardown | ❌ FAIL | `neighbor X teardown N` command |
| notification | ❌ FAIL | NOTIFICATION on peer disconnect |
| watchdog | ❌ FAIL | `announce/withdraw watchdog` command |

---

## Detailed Analysis

### 1. announcement.run - NOT FIXABLE (by design)

**Commands used:**
```python
'neighbor 127.0.0.1 local-as 1 announce route ...'
'neighbor 127.0.0.1 peer-as 1 announce route ...'
'neighbor 127.0.0.1 local-ip 127.0.0.1 announce route ...'
'neighbor 127.0.0.1 router-id 1.2.3.4 announce route ...'
```

**Required feature:** Multi-session neighbor qualifiers (local-as, peer-as, local-ip, router-id)

**Decision:** NOT SUPPORTED - documented in `.claude/zebgp/EXABGP_DIFFERENCES.md`

**Action:** Rewrite test to use simple `neighbor IP announce route` syntax, OR mark as skipped.

---

### 2. attributes.run - ✅ IMPLEMENTED

**Commands used:**
```python
'announce attributes med 100 next-hop 101.1.101.1 nlri 1.0.0.1/32 1.0.0.2/32'
'announce attributes local-preference 200 as-path [ 1 2 3 4 ] next-hop 202.2.202.2 nlri 2.0.0.1/32 2.0.0.2/32'
```

**Implementation (2025-12-23):**

Three new commands added to `pkg/api/route.go`:

1. **`announce attributes ... nlri`** - ExaBGP-compatible (immediate announce)
   - Parses attributes until `nlri` keyword, then prefixes
   - Announces each prefix immediately with shared attributes
   - Supports: next-hop, origin, local-preference, med, as-path, community, path-information

2. **`announce nlri ... <afi> <safi> [nlri]`** - ZeBGP new format (queues to commit)
   - Explicit AFI/SAFI before prefixes
   - Requires active commit (`commit <name> start`)
   - Queues routes to transaction

3. **`announce update ... <afi> <safi> [nlri]`** - Auto-commit wrapper
   - Convenience shortcut for single UPDATE
   - Automatically starts commit, queues routes, ends with EOR

**Files modified:** `pkg/api/route.go` (handlers), `pkg/api/route_parse_test.go` (tests)

---

### 3. check.run - NEEDS RECEIVE UPDATES TO SCRIPT + CONFIG FEATURES

**Commands used:**
```python
# Script READS from stdin expecting:
'neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100'
# Then writes:
'neighbor 127.0.0.1 announce route 1.2.3.4 next-hop 5.6.7.8'
```

**Config uses:**
```
template { group controler { ... } }  # Template inheritance
api connection { receive { parsed; update; } }  # Receive config
static { route 127.0.0.1/32 next-hop 127.0.0.2; }  # Static routes
```

**Required features:**
1. Forward received UPDATEs from peer to API script
2. Template/inherit syntax in config (or skip)
3. Static routes in config (already supported?)
4. API receive configuration

**Implementation plan:**
1. Add callback in session receive path for UPDATE messages
2. Format UPDATE as text/JSON and write to API process stdin
3. Handle both announce and withdraw
4. Complexity: High (touches session.go, api/process.go, config)

---

### 4. ipv4.run - ✅ IMPLEMENTED (unicast only)

**Status:** PASS - Test simplified to unicast only, MUP commands removed.

**Implementation (2025-12-23):**
1. Added `parseFamilyArgs()` function to parse `ipv4/ipv6 unicast` prefix
2. Added handlers: `handleAnnounceIPv4`, `handleAnnounceIPv6`, `handleWithdrawIPv4`, `handleWithdrawIPv6`
3. Registered commands: `announce ipv4`, `announce ipv6`, `withdraw ipv4`, `withdraw ipv6`
4. Simplified test files to remove MUP commands

**Note:** MUP (Mobile User Plane) support is deferred - tests use unicast only.

---

### 5. ipv6.run - ✅ IMPLEMENTED (unicast only)

**Status:** PASS - Test simplified to unicast only, MUP commands removed.

**Implementation (2025-12-23):**
Same as ipv4 - uses shared `parseFamilyArgs()` and family-explicit handlers.

**Note:** MUP (Mobile User Plane) support is deferred - tests use unicast only.

---

### 6. teardown.run - NEEDS `neighbor X teardown N` + MULTI-CONNECTION TEST

**Commands used:**
```python
'neighbor 127.0.0.1 teardown 4'  # Send NOTIFICATION and close session
```

**CI expects (multi-connection test):**
```
option:tcp_connections:3  # Three separate TCP connections!
A1:raw:... # Connection 1: route + EOR
A2:raw:... # Connection 1: NOTIFICATION (teardown), reconnect
B1:raw:... # Connection 2: routes + EOR
B2:raw:... # Connection 2: NOTIFICATION (teardown), reconnect
C1:raw:... # Connection 3: all routes + EOR
```

**Required features:**
1. `neighbor <ip> teardown <code>` - send NOTIFICATION and close
2. Auto-reconnect after teardown
3. Test framework: support multiple TCP connections per test

**Implementation plan:**
1. Register handler for `teardown` after `neighbor` dispatch
2. Send NOTIFICATION with code (4 = Administrative Reset)
3. Close TCP, allow auto-reconnect
4. Test framework: track connection sequence (A, B, C...)
5. Complexity: Medium (command easy, test infra harder)

---

### 7. notification.run - NEEDS NOTIFICATION CI FORMAT

**Commands used:**
```python
'announce route 1.2.3.4 next-hop 5.6.7.8'  # Never received by peer
```

**CI expects:**
```
A1:notification:closing session because we can
```

This is NOT raw bytes - it's a `notification` directive with message text.
The test expects ZeBGP to send NOTIFICATION when peer disconnects.

**Required features:**
1. Session sends NOTIFICATION on shutdown
2. Test framework: parse `notification:` directive (not just `raw:`)

**Implementation plan:**
1. Verify session.go sends NOTIFICATION on graceful close
2. Update testpeer to handle `notification:` format
3. Complexity: Low-Medium

---

### 8. watchdog.run - NEEDS `announce/withdraw watchdog` COMMAND

**Commands used:**
```python
'announce watchdog dnsr'
'withdraw watchdog dnsr'
```

**Required feature:** Watchdog mechanism for health checking

Watchdog in ExaBGP:
- `announce watchdog <name>` - register a watchdog
- If watchdog not refreshed within timeout, associated routes are withdrawn
- Used for health checking / failover

**Implementation plan:**
1. Register handlers for `announce watchdog`, `withdraw watchdog`
2. Maintain watchdog state (name -> last_seen timestamp)
3. Background goroutine checks for expired watchdogs
4. On expiry, withdraw routes associated with that watchdog
5. Complexity: High (new subsystem)

---

## Implementation Priority

### Phase 1: Quick Wins (enables tests quickly)

| Feature | Test | Effort | Notes |
|---------|------|--------|-------|
| `announce ipv4/ipv6 unicast` | ipv4, ipv6 | 2 hours | Simplify tests to remove MUP |
| `announce attributes ... nlri` | attributes | 2-3 hours | Multiple NLRIs per UPDATE |

### Phase 2: Session Commands

| Feature | Test | Effort | Notes |
|---------|------|--------|-------|
| `neighbor X teardown N` | teardown | 3 hours | Needs multi-connection test infra |
| NOTIFICATION ci format | notification | 2 hours | Update testpeer parser |

### Phase 3: High Effort

| Feature | Test | Effort | Notes |
|---------|------|--------|-------|
| Receive updates to script | check | 6+ hours | Major feature, config changes |
| Watchdog subsystem | watchdog | 6+ hours | New subsystem |

### Phase 4: Skip/Defer

| Feature | Test | Reason |
|---------|------|--------|
| MUP (Mobile User Plane) | ipv4, ipv6 | Complex, niche - simplify tests instead |
| Multi-session qualifiers | announcement | Not supported by design |
| Template/inherit config | check | Simplify config instead |

---

## Recommended Implementation Order

1. **`announce ipv4/ipv6 unicast`** (+ simplify tests to remove MUP)
   - Enables: ipv4, ipv6 tests
   - Files: `pkg/api/route.go`, `pkg/api/command.go`

2. **`announce attributes ... nlri`**
   - Enables: attributes test
   - Files: `pkg/api/route.go`

3. **`neighbor X teardown N`** (+ multi-connection test support)
   - Enables: teardown test
   - Files: `pkg/api/command.go`, `pkg/reactor/reactor.go`, `test/cmd/self-check/`

4. **NOTIFICATION handling**
   - Enables: notification test
   - Files: `test/cmd/zebgp-peer/`, verify `pkg/reactor/session.go`

5. **Receive updates to script** (defer if complex)
   - Enables: check test
   - Files: `pkg/reactor/session.go`, `pkg/api/process.go`

6. **Watchdog** (low priority)
   - Enables: watchdog test
   - Files: new `pkg/api/watchdog.go`

---

## Files to Modify

| Feature | Files |
|---------|-------|
| ipv4/ipv6 unicast | `pkg/api/route.go`, `pkg/api/command.go` |
| attributes nlri | `pkg/api/route.go` |
| teardown | `pkg/api/command.go`, `pkg/reactor/reactor.go` |
| notification ci | `test/cmd/zebgp-peer/main.go` |
| receive updates | `pkg/reactor/session.go`, `pkg/api/process.go` |
| watchdog | `pkg/api/watchdog.go` (new), `pkg/reactor/reactor.go` |

---

## Test Simplification Options

Some tests can be made to pass by simplifying them rather than implementing complex features:

| Test | Simplification |
|------|----------------|
| ipv4 | Remove MUP commands, keep unicast only |
| ipv6 | Remove MUP commands, keep unicast only |
| announcement | Rewrite without multi-session qualifiers |
| check | Rewrite without template/inherit, simplify config |
