# Spec: api-encoder-switching

## Task

Fix API encoder switching - processes with `encoder json` should receive JSON-formatted route updates, not text format.

## Current State (verified)

- `make test`: PASS
- `make lint`: PASS (0 issues)
- Functional tests: 24 passed, 13 failed
- Last commit: `a317ea9`

## Context Loaded

### Architecture Docs
- `.claude/zebgp/api/ARCHITECTURE.md` - API package structure, route injection flow, ProcessConfig

### Source Files Read
- `pkg/api/server.go:400-424` - OnUpdateReceived() always uses text format
- `pkg/api/json.go:121-151` - RouteAnnounce() method, RouteUpdate struct
- `pkg/api/text.go:15-101` - ReceivedRoute struct, FormatReceivedUpdate/Withdraw
- `pkg/api/types.go:334-343` - ProcessConfig struct with Encoder field
- `pkg/config/bgp.go:543-550` - Config parsing sets ReceiveUpdate only for text encoder

## Problem Analysis

**Primary bug (stated):** `OnUpdateReceived` ignores `cfg.Encoder`, always uses text format.

**Secondary bug (found):** Config parsing at `bgp.go:548` only sets `ReceiveUpdate=true` when `encoder=text`. JSON-configured processes have `ReceiveUpdate=false`, so they receive **nothing**.

**Missing feature:** No `OnWithdrawReceived` exists. Withdrawals are not forwarded to processes at all.

## End-to-End User Flow

### Configuration Path
- Config syntax: `process foo { run ./script.py; encoder json; }`
- Parsing: `pkg/config/bgp.go:538-552`
  - Line 543-544: `pc.Encoder = v` ✓
  - Line 548-550: `if pc.Encoder == "text" { pc.ReceiveUpdate = true }` ❌
- **Bug:** JSON processes get `ReceiveUpdate=false`

### Execution Path
- Entry: `server.go:OnUpdateReceived(peerAddr, routes)`
- Processing: `FormatReceivedUpdate()` always called
- Output: Text format sent via `proc.WriteEvent(text)`
- **Bug:** `cfg.Encoder` is never checked

### Related Handlers
- `FormatReceivedWithdraw()` exists in text.go but never called
- `JSONEncoder.RouteWithdraw()` exists in json.go but never called
- No withdrawal forwarding to processes exists

## Goal Achievement Check

### User's Actual Goal
User wants to receive JSON-formatted route updates in their process's stdin when they configure `encoder json`.

### Will Plan Achieve It?
| Step | Status | Notes |
|------|--------|-------|
| Config works? | ✅ | Phase 1 fixes config parsing so JSON processes can receive updates |
| Code works? | ✅ | Phase 2 adds encoder switching in OnUpdateReceived |
| Output correct? | ✅ | JSONEncoder.RouteAnnounce already produces correct JSON |

### Blockers and Coverage
| Blocker | Plan Step | Covered? |
|---------|-----------|----------|
| Config: JSON→ReceiveUpdate=false | Phase 1: Config Parsing Fix | ✅ |
| OnUpdateReceived ignores encoder | Phase 2: Encoder Switching | ✅ |
| No withdrawal forwarding | Phase 3 (optional) | ⚠️ User decision |

**Plan achieves goal:** YES (core goal achieved, withdrawals optional)

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Tests MUST exist and FAIL before implementation begins
- Every test MUST have VALIDATES and PREVENTS documentation
- Show failure output, then implementation, then pass output

### From ESSENTIAL_PROTOCOLS.md
- Post-completion self-review is MANDATORY
- Fix all 🔴/🟡 issues before claiming done
- Report 🟢 minor items to user

## Design Decision

**Option A: Minimal fix** - Only fix OnUpdateReceived encoder switching
- Pros: Smallest change
- Cons: JSON processes still can't receive updates (config bug), no withdrawals

**Option B: Complete fix** - Fix config parsing + encoder switching + add withdrawal support
- Pros: Feature actually works end-to-end
- Cons: More changes

**Chosen: Option B** - The minimal fix doesn't achieve the user goal

## Implementation Steps

### Phase 1: Config Parsing Fix

#### Step 1.1: Write test for ReceiveUpdate parsing
**File:** `pkg/config/bgp_test.go`

```go
// TestProcessConfigReceiveUpdate verifies that ReceiveUpdate is set
// correctly for both json and text encoders.
//
// VALIDATES: JSON encoder processes can receive updates when configured.
//
// PREVENTS: Bug where encoder=json → ReceiveUpdate=false.
func TestProcessConfigReceiveUpdate(t *testing.T) {
    // Test: encoder json with receive-update should set ReceiveUpdate=true
    // Test: encoder text with receive-update should set ReceiveUpdate=true
    // Test: encoder json without receive-update should set ReceiveUpdate=false
}
```

#### Step 1.2: Run test → MUST FAIL

#### Step 1.3: Fix config parsing
**File:** `pkg/config/bgp.go` around line 546

Change from:
```go
// Default: text encoder processes receive updates
if pc.Encoder == "text" {
    pc.ReceiveUpdate = true
}
```

To proper parsing of `receive-update` or `receive { update; }` config directive.

#### Step 1.4: Run test → MUST PASS

### Phase 2: Encoder Switching

#### Step 2.1: Write test for ReceivedRouteToRouteUpdate conversion
**File:** `pkg/api/json_test.go`

```go
// TestReceivedRouteToRouteUpdate verifies conversion from ReceivedRoute to RouteUpdate.
//
// VALIDATES: All fields are correctly converted for JSON encoding.
//
// PREVENTS: Data loss or corruption during type conversion.
func TestReceivedRouteToRouteUpdate(t *testing.T) { ... }
```

#### Step 2.2: Write test for encoder switching
**File:** `pkg/api/server_test.go`

```go
// TestOnUpdateReceivedEncoderSwitching verifies that OnUpdateReceived
// uses the correct encoder format based on process configuration.
//
// VALIDATES: Process receives JSON format when encoder=json.
//
// PREVENTS: Bug where all processes receive text format.
func TestOnUpdateReceivedEncoderSwitching(t *testing.T) { ... }
```

#### Step 2.3: Run tests → MUST FAIL

#### Step 2.4: Add ReceivedRouteToRouteUpdate function
**File:** `pkg/api/json.go`

```go
// ReceivedRouteToRouteUpdate converts a ReceivedRoute to RouteUpdate for JSON encoding.
func ReceivedRouteToRouteUpdate(r ReceivedRoute) RouteUpdate { ... }
```

#### Step 2.5: Fix OnUpdateReceived
**File:** `pkg/api/server.go`

```go
func (s *Server) OnUpdateReceived(peerAddr netip.Addr, routes []ReceivedRoute) {
    // Check encoder and format accordingly
    // Lazy-init both formats (only create when needed)
}
```

#### Step 2.6: Run tests → MUST PASS

### Phase 3: Withdrawal Support (optional - confirm with user)

#### Step 3.1: Add OnWithdrawReceived to server
#### Step 3.2: Wire up withdrawal forwarding
#### Step 3.3: Add tests

### Phase 4: Verification

```bash
make test && make lint
```

## Verification Checklist

- [ ] Config parsing test written and shown to FAIL first
- [ ] Config parsing fix makes test pass
- [ ] ReceivedRouteToRouteUpdate test written and shown to FAIL first
- [ ] Encoder switching test written and shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] **Goal verified**: User can configure `encoder json` and receive JSON-formatted updates
- [ ] Self-review performed
- [ ] No 🔴/🟡 issues remaining

## Test Specification

### TestProcessConfigReceiveUpdate

```go
func TestProcessConfigReceiveUpdate(t *testing.T) {
    tests := []struct {
        name          string
        config        string
        wantEncoder   string
        wantReceive   bool
    }{
        {
            name: "json encoder with receive-update",
            config: `process foo { run ./test; encoder json; receive-update; }`,
            wantEncoder: "json",
            wantReceive: true,
        },
        {
            name: "text encoder with receive-update",
            config: `process foo { run ./test; encoder text; receive-update; }`,
            wantEncoder: "text",
            wantReceive: true,
        },
        {
            name: "json encoder without receive-update",
            config: `process foo { run ./test; encoder json; }`,
            wantEncoder: "json",
            wantReceive: false,
        },
    }
    // ...
}
```

### TestOnUpdateReceivedEncoderSwitching

```go
func TestOnUpdateReceivedEncoderSwitching(t *testing.T) {
    // Setup: Mock process manager with two processes
    // - jsonProc: encoder=json, ReceiveUpdate=true
    // - textProc: encoder=text, ReceiveUpdate=true

    // Call OnUpdateReceived with sample routes

    // Assert: jsonProc.received starts with "{" (JSON)
    // Assert: textProc.received starts with "neighbor" (text)
}
```

## Questions for User

1. **Withdrawal support**: Should Phase 3 (withdrawal forwarding) be included in this task, or deferred to a separate spec?

2. **Config syntax**: The current config uses implicit `receive-update` tied to encoder type. Should we:
   - (A) Add explicit `receive-update;` directive that must be present
   - (B) Change the default so all encoders receive updates unless `no-receive-update;` is specified
   - (C) Keep backward compatibility (text=receive, json=no-receive) and add explicit directive

---

**Created:** 2025-12-29
**Updated:** 2025-12-29 (added end-to-end analysis, config parsing bug, withdrawal gap)
