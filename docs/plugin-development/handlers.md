# Verify and Apply Handlers

Handlers process configuration changes. Verify handlers validate changes; Apply handlers execute them.

## Candidate/Running Model

Plugins maintain two configuration states:

```
┌─────────────────────────────────────────┐
│              Running                    │
│  (last committed - currently active)    │
├─────────────────────────────────────────┤
│  local_as: 65001                        │
│  peers:                                 │
│    192.0.2.1: {peer_as: 65002}          │
│    192.0.2.2: {peer_as: 65003}          │
└─────────────────────────────────────────┘
                    │
                    │ rollback (copy)
                    ▼
┌─────────────────────────────────────────┐
│              Candidate                  │
│  (pending changes - uncommitted)        │
├─────────────────────────────────────────┤
│  local_as: 65001                        │
│  peers:                                 │
│    192.0.2.1: {peer_as: 65002}          │
│    192.0.2.3: {peer_as: 65004}  ← added │
│    (192.0.2.2 deleted)                  │
└─────────────────────────────────────────┘
                    │
                    │ commit (verify → apply)
                    ▼
              Running updated
```

## Command Flow

Commands modify the candidate configuration:

```
bgp peer create {"address":"192.0.2.3","peer-as":65004}
bgp peer delete {"address":"192.0.2.2"}
```

Then commit applies changes atomically:

```
bgp commit
```

| Command | Effect |
|---------|--------|
| `bgp ... create/modify/delete` | Modifies candidate only |
| `bgp commit` | Diff → verify all → apply all → candidate becomes running |
| `bgp rollback` | Discard candidate, copy running to candidate |
| `bgp diff` | Show pending changes |

## Commit Flow

```
bgp commit
    │
    ▼
┌──────────────────┐
│  Compute Diff    │  Compare candidate vs running
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Verify Phase    │  Call verify handlers for each change
└────────┬─────────┘  (any error → abort, candidate unchanged)
         │
         ▼
┌──────────────────┐
│  Apply Phase     │  Call apply handlers for each change
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Update Running  │  Candidate becomes new running
└──────────────────┘
```

## Verify Handlers

Verify handlers validate changes against current state. Called during commit.

```go
p.OnVerify("bgp.peer", func(ctx *plugin.VerifyContext) error {
    // ctx.Action: "create", "modify", or "delete"
    // ctx.Path: "peer"
    // ctx.Data: JSON config data

    if ctx.Action == "delete" {
        return nil  // Always allow delete
    }

    // Parse proposed config
    var cfg PeerConfig
    if err := json.Unmarshal([]byte(ctx.Data), &cfg); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    // Validate against current state
    if cfg.PeerAS == p.state.LocalAS {
        return fmt.Errorf("peer-as cannot equal local-as")
    }

    if ctx.Action == "create" {
        if _, exists := p.state.Peers[cfg.Address]; exists {
            return fmt.Errorf("peer %s already exists", cfg.Address)
        }
    }

    return nil  // Valid
})
```

### What Verify Should Check

| Check | Example |
|-------|---------|
| Required fields present | `if cfg.Address == "" { error }` |
| Values make sense | `if cfg.HoldTime < cfg.KeepaliveInterval*3 { error }` |
| References exist | `if !exists(cfg.PeerGroup) { error }` |
| No conflicts with running | `if alreadyExists(cfg.Address) { error }` |
| Semantic constraints | `if cfg.PeerAS == localAS { error }` |

### What Verify Should NOT Do

- Start services
- Open connections
- Modify state (running or candidate)
- Write files

Verify is read-only validation.

## Apply Handlers

Apply handlers execute validated changes. Called only after all verify passes.

```go
p.OnApply("bgp.peer", func(ctx *plugin.ApplyContext) error {
    // ctx.Action: "create", "modify", or "delete"
    // ctx.Path: "peer"
    // ctx.Data: JSON config data

    var cfg PeerConfig
    _ = json.Unmarshal([]byte(ctx.Data), &cfg)

    switch ctx.Action {
    case "create":
        return p.startPeerSession(cfg)

    case "modify":
        return p.updatePeerSession(cfg)

    case "delete":
        return p.stopPeerSession(cfg.Address)
    }

    return nil
})
```

### Apply Handler Responsibilities

- Start/stop services
- Open/close connections
- Update running state
- Allocate/free resources

## Handler Routing

Handlers are matched by **longest prefix**:

```go
p.OnVerify("bgp", handleBGP)
p.OnVerify("bgp.peer", handlePeer)
p.OnVerify("bgp.peer-group", handlePeerGroup)

// Command "bgp peer create {...}" → handlePeer
// Command "bgp peer-group create {...}" → handlePeerGroup
// Command "bgp create {...}" → handleBGP
```

## Context Types

### VerifyContext

```go
type VerifyContext struct {
    Action string  // "create", "modify", "delete"
    Path   string  // Handler path (e.g., "peer")
    Data   string  // JSON config data
}
```

### ApplyContext

```go
type ApplyContext struct {
    Action string  // "create", "modify", "delete"
    Path   string  // Handler path (e.g., "peer")
    Data   string  // JSON config data
}
```

## Error Handling

Return clear, actionable errors:

```go
// Good: tells user what's wrong and how to fix
return fmt.Errorf("hold-time must be at least %d seconds, got %d",
    minHoldTime, cfg.HoldTime)

// Bad: cryptic
return fmt.Errorf("validation failed")
```

## State Management Example

```go
type BGPPlugin struct {
    *plugin.Plugin
    running   *BGPState  // Last committed config
    candidate *BGPState  // Pending changes
}

type BGPState struct {
    LocalAS uint32
    Peers   map[string]*PeerConfig
}

func (p *BGPPlugin) OnCommit() error {
    // Diff candidate vs running
    changes := p.diffStates(p.running, p.candidate)

    // Verify all changes
    for _, change := range changes {
        if err := p.verify(change); err != nil {
            return err  // Abort, candidate unchanged
        }
    }

    // Apply all changes
    for _, change := range changes {
        if err := p.apply(change); err != nil {
            return err  // Partial apply, needs recovery
        }
    }

    // Candidate becomes running
    p.running = p.candidate.Clone()
    return nil
}

func (p *BGPPlugin) OnRollback() {
    // Discard candidate, revert to running
    p.candidate = p.running.Clone()
}
```
