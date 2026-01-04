# Spec: fix-test-c

## Task
Split test C into two focused tests with cleaner design

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/behavior/FSM.md` - Session lifecycle, state transitions
- [x] `.claude/zebgp/api/ARCHITECTURE.md` - Persist plugin, API sync protocol

## Current State
- Tests: 13/14 API tests pass, only test C fails
- Last commit: `fc435f2`
- Root cause: Dual teardown mechanisms (script + test peer) race

## New Test Design

### Test C1: Reconnection after peer disconnect
**Purpose:** Verify ZeBGP reconnects when peer sends NOTIFICATION

**Flow:**
1. ZeBGP connects to test peer
2. ZeBGP sends route1 via API
3. Test peer receives route1, sends NOTIFICATION
4. ZeBGP reconnects automatically
5. Test peer sends route to ZeBGP ("job's done" signal)
6. Test passes

**New feature needed:** Test peer can SEND routes to ZeBGP (not just receive)

**CI format:**
```
# Connection 1: ZeBGP sends route, peer disconnects
A1:raw:... (route1 from ZeBGP)
A2:notification:Peer Reset

# Connection 2: Peer sends "done" route to ZeBGP
B1:send:raw:... (route from peer to ZeBGP)
```

### Test C2: Teardown command
**Purpose:** Verify ZeBGP tears down on API teardown command

**Flow:**
1. ZeBGP connects to test peer
2. ZeBGP sends route1 via API
3. API sends teardown command
4. ZeBGP sends NOTIFICATION to peer
5. Test passes

**CI format:**
```
A1:raw:... (route1)
A1:raw:... (EOR)
# ZeBGP sends NOTIFICATION (from teardown command)
A1:notification:Administrative Reset
```

## Files to Modify/Create

### Phase 1: Add "send" action to test peer
- `pkg/testpeer/peer.go` - Add ability to send UPDATE to ZeBGP
- `pkg/testpeer/checker.go` - Parse `send:raw:...` actions

### Phase 2: Create new test files
- `test/data/api/reconnect.conf` - Test C1 config
- `test/data/api/reconnect.run` - Test C1 script
- `test/data/api/reconnect.ci` - Test C1 expectations
- `test/data/api/teardown-cmd.conf` - Test C2 config
- `test/data/api/teardown-cmd.run` - Test C2 script
- `test/data/api/teardown-cmd.ci` - Test C2 expectations

### Phase 3: Fix stale teardown bug
- `pkg/reactor/peer.go` - Don't queue teardown when `p.session == nil`

### Phase 4: Remove old test C
- Delete or rename `test/data/api/teardown.*`

## Implementation Steps

1. Add `send:raw:...` action to test peer
2. Write test C1 (reconnect) - TDD
3. Write test C2 (teardown-cmd) - TDD
4. Fix stale teardown bug in peer.go
5. Run `make test && make lint && make functional`
6. Remove old test C

## Checklist
- [x] Required docs read
- [ ] Test peer send action implemented
- [ ] Test C1 (reconnect) passes
- [ ] Test C2 (teardown-cmd) passes
- [ ] Stale teardown bug fixed
- [ ] Old test C removed
- [ ] make test passes
- [ ] make lint passes
- [ ] make functional passes
