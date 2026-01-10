# Unified Commit System

**Status:** ✅ Complete (Phase 1-3 Done)
**Created:** 2025-12-21
**Completed:** 2025-12-22
**Supersedes:** `config-routes-eor.md`, `api-commit-batching.md`

## Implementation Progress

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | CommitService Abstraction | ✅ Complete |
| Phase 2 | Config Route Migration | ✅ Complete |
| Phase 3 | API Commit Commands | ✅ Complete |
| Phase 4 | OutgoingRIB Transaction Cleanup | ⏳ Pending (optional) |
| Phase 5 | Self-Check API Tests | ⏳ Pending (optional) |

### Phase 3 Status

| Task | Description | Status |
|------|-------------|--------|
| 3.1 | CommitManager for concurrent commits | ✅ `pkg/plugin/commit_manager.go` |
| 3.2 | `commit <name> start` | ✅ |
| 3.3 | `commit <name> end` (no EOR) | ✅ |
| 3.4 | `commit <name> eor` (with EOR) | ✅ |
| 3.5 | `commit <name> rollback` | ✅ |
| 3.6 | `commit <name> announce/withdraw` routing | ✅ `commit.go` |
| 3.7 | `commit list` introspection | ✅ |
| 3.8 | `commit <name> show` introspection | ✅ |
| 3.9 | Route conflicts (replace/cancel) | ✅ In Transaction class |
| 3.10 | Dispatcher registration | ✅ |
| 3.11 | Tests | ✅ |

### Key Implementation Details

- **SendRoutes method** added to ReactorInterface (`pkg/plugin/types.go:200`)
- **SendRoutes implementation** in reactor uses CommitService (`pkg/reactor/reactor.go:672-740`)
- **handleNamedCommitEnd** wired to call SendRoutes with routes from Transaction
- **Announce/Withdraw handlers** parse routes and queue to Transaction

---

## Current State

### What Already Exists

| Component | Location | Status |
|-----------|----------|--------|
| EOR message building | `reactor.go:319-344` | ✅ Works |
| EOR sending after config routes | `peer.go:390-399` | ✅ Works |
| Route grouping by attributes | `peer.go:575-657` | ✅ Works |
| OutgoingRIB with transactions | `rib/outgoing.go` | ✅ Exists (unused) |
| MP_UNREACH_NLRI + IsEndOfRIB | `attribute/mpnlri.go` | ✅ Works |
| GroupByAttributes in RIB | `rib/grouping.go` | ✅ Works |

### Current Flow (Config Routes)

```
Session ESTABLISHED
       ↓
sendInitialRoutes()
       ↓
groupRoutesByAttributes()  ← Direct grouping
       ↓
buildGroupedUpdate() per group
       ↓
SendUpdate() per group
       ↓
buildEORUpdate() per family
       ↓
SendUpdate() for each EOR
```

### Current Flow (API Routes)

```
API: announce route ...
       ↓
Reactor.AnnounceRoute()
       ↓
OutgoingRIB.QueueAnnounce()  ← Queued but...
       ↓
??? (flush not triggered)    ← No commit mechanism
```

**Problem:** API routes queue in OutgoingRIB but there's no trigger to flush them with grouping + EOR.

---

## Goal

**Single commit abstraction for both config and API routes:**

```
┌─────────────────────────────────────────────────────────────────┐
│                         COMMIT                                   │
│                                                                  │
│  Input: Routes (from config or API)                             │
│                                                                  │
│  Process:                                                        │
│    1. IF rib.group-updates: group by attributes                 │
│    2. Build UPDATE messages (grouped or individual)             │
│    3. Send UPDATEs                                               │
│    4. Send EOR for affected families (if eor requested)         │
│                                                                  │
│  Output: CommitStats (updates_sent, routes, eor_sent)           │
└─────────────────────────────────────────────────────────────────┘
```

**Grouping is controlled by RIB settings, NOT the commit command.**

**Config routes:** Implicit commit on ESTABLISHED → EOR
**API routes:** Explicit `commit start/end/eor` → optional EOR

---

## Architecture

### Commit Service

```go
// pkg/rib/commit.go

type CommitService struct {
    peer      PeerSender
    rib       *OutgoingRIB
    negotiated *Negotiated
}

type CommitOptions struct {
    Label   string   // Optional transaction label
    SendEOR bool     // Send EOR after commit (true for config, false for API)
}

type CommitStats struct {
    UpdatesSent      int
    RoutesAnnounced  int
    RoutesWithdrawn  int
    FamiliesAffected []Family  // families that had routes
    EORSent          []Family  // families for which EOR was sent
}

// Family represents AFI/SAFI pair for JSON responses
type Family struct {
    AFI  uint16 `json:"afi"`
    SAFI uint8  `json:"safi"`
}

func (c *CommitService) Commit(routes []*Route, opts CommitOptions) (CommitStats, error) {
    var stats CommitStats

    // 1. Group routes IF rib.group-updates is enabled
    if c.ribConfig.GroupUpdates {
        groups := GroupByAttributes(routes)
        for _, group := range groups {
            update := BuildGroupedUpdate(group, c.negotiated)
            if err := c.peer.SendUpdate(update); err != nil {
                return stats, err
            }
            stats.UpdatesSent++
            stats.RoutesAnnounced += len(group.NLRIs)
            trackFamily(&stats.FamiliesAffected, group.Family)
        }
    } else {
        // No grouping: one UPDATE per route
        for _, route := range routes {
            update := BuildSingleUpdate(route, c.negotiated)
            if err := c.peer.SendUpdate(update); err != nil {
                return stats, err
            }
            stats.UpdatesSent++
            stats.RoutesAnnounced++
            trackFamily(&stats.FamiliesAffected, route.Family)
        }
    }

    // 2. Send EOR if requested
    if opts.SendEOR {
        for _, fam := range stats.FamiliesAffected {
            eor := BuildEOR(fam)
            if err := c.peer.SendUpdate(eor); err != nil {
                return stats, err
            }
            stats.EORSent = append(stats.EORSent, fam)
        }
    }

    return stats, nil
}
```

### Config Route Integration

```go
// pkg/reactor/peer.go

func (p *Peer) sendInitialRoutes() error {
    // Collect all config routes
    routes := p.collectConfigRoutes()

    if len(routes) == 0 {
        // No routes, but still send EOR for negotiated families
        return p.sendEORForNegotiatedFamilies()
    }

    // Use commit service (implicit commit with EOR)
    commit := rib.NewCommitService(p, p.outgoingRIB, p.negotiated)
    stats, err := commit.Commit(routes, rib.CommitOptions{
        Label:   "config",
        SendEOR: true,
    })
    if err != nil {
        return err
    }

    p.log.Info("config routes committed",
        "updates", stats.UpdatesSent,
        "routes", stats.RoutesAnnounced,
        "eor", stats.EORSent)

    return nil
}
```

### API Commit Integration

**Command syntax:**
```
commit <name> start                # Start named commit (name always required)
commit <name> announce route ...   # Queue route to named commit
commit <name> withdraw route ...   # Queue withdrawal to named commit
commit <name> end                  # Flush queued routes (no EOR)
commit <name> eor                  # Flush queued routes (with EOR)
commit <name> rollback             # Discard queued routes

announce route ...                 # IMMEDIATE send (not batched)
withdraw route ...                 # IMMEDIATE send (not batched)
```

**Multiple concurrent commits:**
```
commit batch1 start
commit batch2 start
commit batch1 announce route 10.0.0.0/24 next-hop 1.2.3.4
commit batch2 announce route 10.1.0.0/24 next-hop 1.2.3.4
commit batch1 eor
commit batch2 end
```

**Immediate vs batched:**
```
# These are sent IMMEDIATELY (one UPDATE each):
announce route 10.0.0.0/24 next-hop 1.2.3.4
announce route 10.1.0.0/24 next-hop 1.2.3.4

# These are BATCHED (grouped into fewer UPDATEs on commit):
commit batch1 start
commit batch1 announce route 10.0.0.0/24 next-hop 1.2.3.4
commit batch1 announce route 10.1.0.0/24 next-hop 1.2.3.4
commit batch1 eor   # → 1 grouped UPDATE + EOR
```

```go
// pkg/plugin/commit.go

// CommitManager tracks multiple concurrent commits
type CommitManager struct {
    mu       sync.RWMutex
    commits  map[string]*Transaction  // name → transaction
}

func (m *CommitManager) Start(name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if name == "" {
        return fmt.Errorf("commit name required")
    }

    if _, exists := m.commits[name]; exists {
        return fmt.Errorf("commit %q already active", name)
    }

    m.commits[name] = NewTransaction(name)
    return nil
}

func (m *CommitManager) Get(name string) (*Transaction, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    if name == "" {
        return nil, fmt.Errorf("commit name required")
    }

    tx, exists := m.commits[name]
    if !exists {
        return nil, fmt.Errorf("commit %q not found", name)
    }
    return tx, nil
}

// handleCommit dispatches commit subcommands
// Syntax: commit <name> <action> [args...]
//   commit list                     - list active commits
//   commit batch1 start             - start named commit
//   commit batch1 announce route... - queue route to commit
//   commit batch1 end               - flush without EOR
//   commit batch1 eor               - flush with EOR
//   commit batch1 rollback          - discard queued routes
//   commit batch1 show              - show queued count
func handleCommit(ctx *APIContext, args []string) Response {
    if len(args) == 0 {
        return ErrorResponse(fmt.Errorf("usage: commit <name> <action> or commit list"))
    }

    // Special case: commit list (no name)
    if args[0] == "list" {
        return handleCommitList(ctx)
    }

    if len(args) < 2 {
        return ErrorResponse(fmt.Errorf("usage: commit <name> <start|end|eor|rollback|announce|withdraw|show>"))
    }

    name := args[0]
    action := args[1]
    actionArgs := args[2:]

    switch action {
    case "start":
        return handleCommitStart(ctx, name)
    case "end":
        return handleCommitEnd(ctx, name, false)
    case "eor":
        return handleCommitEnd(ctx, name, true)
    case "rollback":
        return handleCommitRollback(ctx, name)
    case "show":
        return handleCommitShow(ctx, name)
    case "announce", "withdraw":
        tx, err := ctx.CommitManager.Get(name)
        if err != nil {
            return ErrorResponse(err)
        }
        return handleRouteCommand(ctx, tx, action, actionArgs)
    default:
        return ErrorResponse(fmt.Errorf("unknown commit action: %s", action))
    }
}
```

---

## Implementation Phases

### Phase 1: CommitService Abstraction

| # | Task | Files |
|---|------|-------|
| 1.1 | Create CommitService with Commit() method | `pkg/rib/commit.go` |
| 1.2 | Move grouping logic to use RIB's GroupByAttributes | `pkg/rib/commit.go` |
| 1.3 | Add BuildEOR() to message package (refactor from reactor) | `pkg/bgp/message/eor.go` |
| 1.4 | Tests for CommitService | `pkg/rib/commit_test.go` |

### Phase 2: Config Route Migration

| # | Task | Files |
|---|------|-------|
| 2.1 | Refactor sendInitialRoutes() to use CommitService | `pkg/reactor/peer.go` |
| 2.2 | Remove duplicate grouping logic from peer.go | `pkg/reactor/peer.go` |
| 2.3 | Verify existing tests still pass | `make test` |
| 2.4 | Update .ci files if EOR format changes | `test/data/encode/*.ci` |

### Phase 3: API Commit Commands

| # | Task | Files |
|---|------|-------|
| 3.1 | Create CommitManager for multiple concurrent commits | `pkg/plugin/commit.go` |
| 3.2 | Implement `commit [name] start` | `pkg/plugin/commit.go` |
| 3.3 | Implement `commit [name] end` (no EOR) | `pkg/plugin/commit.go` |
| 3.4 | Implement `commit [name] eor` (with EOR) | `pkg/plugin/commit.go` |
| 3.5 | Implement `commit [name] rollback` | `pkg/plugin/commit.go` |
| 3.6 | Handle `commit <name> announce/withdraw` routing | `pkg/plugin/commit.go` |
| 3.7 | Implement `commit list` (introspection) | `pkg/plugin/commit.go` |
| 3.8 | Implement `commit <name> show` (introspection) | `pkg/plugin/commit.go` |
| 3.9 | Handle route conflicts (replace) and cancel (withdraw after announce) | `pkg/plugin/commit.go` |
| 3.10 | Register commands in dispatcher | `pkg/plugin/dispatcher.go` |
| 3.11 | Tests for API commit commands | `pkg/plugin/commit_test.go` |

**Command syntax:**
```
commit <name> start                # Start named commit (name required)
commit <name> announce route ...   # Route queued to named commit
commit <name> withdraw route ...   # Withdrawal queued to named commit
commit <name> end                  # Flush without EOR
commit <name> eor                  # Flush with EOR
commit <name> rollback             # Discard queued routes
commit <name> show                 # Show queued count and families
commit list                        # List all active commits

announce route ...                 # IMMEDIATE send (no batching, no commit)
withdraw route ...                 # IMMEDIATE send (no batching, no commit)
```

**Semantics:**
- Commit name is **always required** for commit commands
- `announce`/`withdraw` without commit prefix = immediate send
- `commit <name> announce/withdraw` = queue to named commit
- Multiple commits can be active concurrently

### Phase 4: OutgoingRIB Transaction Cleanup

| # | Task | Files |
|---|------|-------|
| 4.1 | Ensure BeginTransaction/CommitTransaction work with CommitService | `pkg/rib/outgoing.go` |
| 4.2 | Add FlushTransaction() to get pending routes | `pkg/rib/outgoing.go` |
| 4.3 | Handle edge cases (disconnect during transaction) | `pkg/rib/outgoing.go` |
| 4.4 | Tests for transaction edge cases | `pkg/rib/outgoing_test.go` |

### Phase 5: Self-Check API Tests (Future)

| # | Task | Files |
|---|------|-------|
| 5.1 | Enable .run script execution in self-check | `test/cmd/self-check/` |
| 5.2 | Convert .run scripts to use commit commands | `test/data/api/*.run` |
| 5.3 | Remove sleep-based timing from tests | `test/data/api/*.run` |

---

## Data Flow Diagrams

### Config Routes (Implicit Commit)

```
Session ESTABLISHED
       │
       ▼
┌──────────────────────────────────┐
│  peer.sendInitialRoutes()        │
│                                  │
│  routes := collectConfigRoutes() │
└──────────────┬───────────────────┘
               │
               ▼
┌──────────────────────────────────┐
│  CommitService.Commit(routes,    │
│    {SendEOR: true})              │
│                                  │
│  1. GroupByAttributes(routes)    │
│  2. BuildGroupedUpdate() × N     │
│  3. peer.SendUpdate() × N        │
│  4. BuildEOR() per family        │
│  5. peer.SendUpdate(EOR) × N     │
└──────────────┬───────────────────┘
               │
               ▼
        CommitStats{
          UpdatesSent: 3,
          RoutesAnnounced: 15,
          EORSent: [ipv4, ipv6]
        }
```

### API Routes (Explicit Commit)

**Immediate send (no commit):**
```
API Client                    ZeBGP                         Peer
    │                           │                              │
    │  announce route 10.0.0.0/24 next-hop 1.2.3.4            │
    │─────────────────────────►│                              │
    │                           │  Build UPDATE                │
    │                           │─────────────────────────────►│
    │  {"status": "ok",         │  (sent immediately)          │
    │   "sent": 1}              │                              │
    │◄─────────────────────────│                              │
```

**Batched send (with commit):**
```
API Client                    ZeBGP                         Peer
    │                           │                              │
    │  commit batch1 start      │                              │
    │─────────────────────────►│                              │
    │  {"status": "ok",         │                              │
    │   "commit": "batch1"}     │                              │
    │◄─────────────────────────│                              │
    │                           │                              │
    │  commit batch1 announce route 10.0.0.0/24 ...           │
    │─────────────────────────►│                              │
    │  {"status": "ok",         │  (queued, not sent)          │
    │   "queued": 1}            │  ← total in commit           │
    │◄─────────────────────────│                              │
    │                           │                              │
    │  commit batch1 announce route 10.1.0.0/24 ...           │
    │─────────────────────────►│                              │
    │  {"status": "ok",         │  (queued, not sent)          │
    │   "queued": 2}            │  ← total in commit           │
    │◄─────────────────────────│                              │
    │                           │                              │
    │  commit batch1 eor        │                              │
    │─────────────────────────►│                              │
    │                           │  IF rib.group-updates:       │
    │                           │    group by attributes       │
    │                           │  Build UPDATEs               │
    │                           │─────────────────────────────►│
    │                           │  Send EOR                    │
    │                           │─────────────────────────────►│
    │  {"status": "ok",         │                              │
    │   "updates_sent": 1,      │                              │
    │   "routes_announced": 2,  │                              │
    │   "eor_sent": [           │                              │
    │     {"afi": 1, "safi": 1} │  ← IPv4 unicast              │
    │   ]}                      │                              │
    │◄─────────────────────────│                              │
```

**Multiple concurrent commits:**
```
API Client                    ZeBGP
    │                           │
    │  commit batch1 start      │
    │─────────────────────────►│
    │  {"status": "ok", "commit": "batch1"}
    │◄─────────────────────────│
    │                           │
    │  commit batch2 start      │
    │─────────────────────────►│
    │  {"status": "ok", "commit": "batch2"}
    │◄─────────────────────────│
    │                           │
    │  commit batch1 announce route 10.0.0.0/24 next-hop 1.2.3.4
    │─────────────────────────►│
    │  {"status": "ok", "queued": 1}   ← total in batch1
    │◄─────────────────────────│
    │                           │
    │  commit batch2 announce route 10.1.0.0/24 next-hop 5.6.7.8
    │─────────────────────────►│
    │  {"status": "ok", "queued": 1}   ← total in batch2
    │◄─────────────────────────│
    │                           │
    │  commit batch1 eor        │  ← Flush batch1 with EOR
    │─────────────────────────►│
    │  {"status": "ok", "updates_sent": 1, "eor_sent": [{"afi": 1, "safi": 1}]}
    │◄─────────────────────────│
    │                           │
    │  commit batch2 end        │  ← Flush batch2 without EOR
    │─────────────────────────►│
    │  {"status": "ok", "updates_sent": 1}
    │◄─────────────────────────│
```

**commit end vs commit eor:**
- `commit <name> end` → send UPDATEs only (grouping per rib.group-updates)
- `commit <name> eor` → send UPDATEs + EOR for affected families

**Grouping:** Controlled by `rib { group-updates true; }` setting, not by commit command.

---

## Wire Format Reference

### IPv4 Unicast EOR

```
┌────────────────────────────────────────────────────────────────┐
│ Marker (16 bytes): FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF            │
│ Length (2 bytes):  0017 (23 bytes total)                       │
│ Type (1 byte):     02 (UPDATE)                                 │
│ Withdrawn Length:  0000                                        │
│ Path Attr Length:  0000                                        │
│ NLRI:              (none)                                      │
└────────────────────────────────────────────────────────────────┘
```

### IPv6 Unicast EOR

```
┌────────────────────────────────────────────────────────────────┐
│ Marker (16 bytes): FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF            │
│ Length (2 bytes):  001D (29 bytes total)                       │
│ Type (1 byte):     02 (UPDATE)                                 │
│ Withdrawn Length:  0000                                        │
│ Path Attr Length:  0006                                        │
│ Path Attributes:                                               │
│   MP_UNREACH_NLRI (type 15):                                   │
│     Flags: 80 (optional, non-transitive)                       │
│     Type:  0F                                                  │
│     Len:   03                                                  │
│     AFI:   0002 (IPv6)                                         │
│     SAFI:  01 (unicast)                                        │
│     Withdrawn NLRI: (none = EOR)                               │
└────────────────────────────────────────────────────────────────┘
```

---

## Test Strategy

### Unit Tests

```go
// pkg/rib/commit_test.go

func TestCommitService_GroupsRoutesByAttributes(t *testing.T) {
    // 3 routes, 2 with same attrs → 2 UPDATEs
}

func TestCommitService_SendsEORWhenRequested(t *testing.T) {
    // Commit with SendEOR: true → EOR sent
}

func TestCommitService_NoEORWhenNotRequested(t *testing.T) {
    // Commit with SendEOR: false → no EOR
}

func TestCommitService_TracksAffectedFamilies(t *testing.T) {
    // IPv4 + IPv6 routes → both families in stats
}
```

### Integration Tests

```go
// pkg/reactor/peer_test.go

func TestPeer_ConfigRoutesUseCommitService(t *testing.T) {
    // Verify grouped UPDATEs + EOR after config routes
}
```

### Self-Check Tests

Existing `.ci` files should continue to pass. If EOR format or ordering changes, update expectations.

---

## Migration Notes

### From Current Code

1. `peer.go:sendInitialRoutes()` currently calls `groupRoutesByAttributes()` directly
2. EOR is built inline with `buildEORUpdate()`
3. These should be refactored to use CommitService

### Backwards Compatibility

- Config behavior unchanged (grouped UPDATEs + EOR)
- API gains new `commit` commands (additive)
- No breaking changes to wire format

---

## Success Criteria

1. ✅ Config routes use CommitService (single code path)
2. ✅ API `commit start/end` commands work
3. ✅ EOR sent after both config and API commits
4. ✅ Route grouping works for both paths
5. ✅ All existing tests pass
6. ✅ New commit tests pass

---

## Open Questions / Edge Cases

### 1. Peer Targeting

**Question:** Which peer does a commit target?

**Options:**
- A) One CommitManager per peer (commits are peer-scoped)
- B) Global CommitManager, specify peer in commit: `commit batch1 peer 192.0.2.1 start`
- C) Global CommitManager, routes contain peer info

**Recommendation:** Option A - CommitManager is per-peer, accessed via peer's API context.

---

### 2. EOR Semantics

**Question:** RFC 4724 defines EOR for graceful restart initial sync. Is sending EOR on every `commit eor` correct?

**Analysis:**
- Config routes → EOR = correct (initial sync complete)
- API `commit eor` → EOR = signals "this batch is complete"

**Decision:** EOR after API commits is useful for signaling batch completion to peer. Not strictly RFC 4724 usage but commonly accepted. Document this behavior.

---

### 3. Session State

**Question:** What if `commit batch1 announce ...` is called but session is not ESTABLISHED?

**Options:**
- A) Error immediately: `{"status": "error", "error": "session not established"}`
- B) Queue anyway, flush will error on `commit end/eor`
- C) Queue anyway, hold until session established

**Recommendation:** Option A - fail fast on announce if not ESTABLISHED.

---

### 4. Peer Disconnect

**Question:** What happens to queued routes if peer disconnects mid-commit?

**Behavior:**
- Active commits are automatically rolled back on disconnect
- `commit end/eor` returns error if peer not connected
- Log warning: "commit batch1 rolled back: peer disconnected"

---

### 5. Route Conflicts in Commit

**Question:** Same prefix announced twice with different attributes?

```
commit batch1 announce route 10.0.0.0/24 next-hop 1.1.1.1
commit batch1 announce route 10.0.0.0/24 next-hop 2.2.2.2
```

**Behavior:** Last one wins. Second replaces first in queue.

**Response:** `{"status": "ok", "queued": 1, "replaced": 1}`

---

### 6. Announce + Withdraw Same Prefix

**Question:** Announce then withdraw same prefix in same commit?

```
commit batch1 announce route 10.0.0.0/24 next-hop 1.1.1.1
commit batch1 withdraw route 10.0.0.0/24
```

**Behavior:** Net effect = withdrawal only. Announce is removed from queue.

**Response on withdraw:** `{"status": "ok", "queued": 0, "cancelled": 1}`

---

### 7. Error Response Format

**Standard error format:**
```json
{"status": "error", "error": "commit 'batch1' not found"}
{"status": "error", "error": "session not established"}
{"status": "error", "error": "invalid route: bad prefix"}
```

**Partial success (some routes failed validation):**
```json
{
  "status": "partial",
  "queued": 5,
  "failed": 2,
  "errors": [
    {"route": "invalid/33", "error": "prefix length > 32"},
    {"route": "10.0.0.0/24", "error": "missing next-hop"}
  ]
}
```

---

### 8. Commit Timeout

**Question:** What if `commit start` but never `end/eor/rollback`?

**Options:**
- A) No timeout - commits live forever until explicit end/rollback
- B) Configurable timeout (default 5 minutes), auto-rollback with warning
- C) Rollback on API client disconnect

**Recommendation:** Option C - rollback on client disconnect. Option B as enhancement.

---

### 9. Resource Limits

**Question:** What if commit queues millions of routes?

**Limits:**
- `max-commit-routes` config option (default: 100000)
- Error on exceed: `{"status": "error", "error": "commit queue limit exceeded (100000)"}`

---

### 10. Introspection Commands

**Additional commands:**
```
commit list                    # List active commits
commit batch1 show             # Show queued routes count
commit batch1 status           # Show commit state
```

**Responses:**
```json
// commit list
{"status": "ok", "commits": ["batch1", "batch2"]}

// commit batch1 show
{"status": "ok", "commit": "batch1", "queued": 15, "families": [{"afi": 1, "safi": 1}]}
```

---

### 11. Withdrawal Handling

**Withdrawals in commits:**
- Withdrawals are queued separately from announcements
- On flush, withdrawals can be combined with announcements in same UPDATE
- Grouping: withdrawals grouped by family only (no attributes)

```go
type Transaction struct {
    name        string
    announces   map[string]*Route  // key: family+prefix
    withdrawals map[string]nlri.NLRI  // key: family+prefix
}
```

---

### 12. Mixed Families in Commit

**Question:** Commit with IPv4 and IPv6 routes?

**Behavior:**
- Routes grouped by family first, then by attributes
- EOR sent for each family that had routes
- Response shows all families: `"eor_sent": [{"afi": 1, "safi": 1}, {"afi": 2, "safi": 1}]`

---

## References

- RFC 4724: Graceful Restart (EOR definition)
- ExaBGP group commands: `../src/exabgp/reactor/api/command/group.py`
- Current peer.go: `pkg/reactor/peer.go:330-412`
- Current grouping: `pkg/rib/grouping.go`
- OutgoingRIB: `pkg/rib/outgoing.go`
