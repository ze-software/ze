# Spec: hub-phase6-gr-plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `internal/plugin/gr/` - existing GR plugin code

## Task

Complete GR plugin implementation:
1. Create `yang/ze-gr.yang` that augments ze-bgp
2. Remove graceful-restart from ze-bgp.yang
3. GR injects capabilities via `capability hex <code> <value> peer <addr>`
4. GR subscribes to peer events for restart handling

**Scope:** GR YANG, capability injection, event handling.

**Depends on:** Phase 5 complete

## Required Reading

### Source Files
- [ ] `internal/plugin/gr/gr.go` - existing GR plugin (has 5-stage, capability injection)
- [ ] `yang/ze-bgp.yang` - has graceful-restart to remove (line 274)
- [ ] RFC 4724 summary - `rfc/short/rfc4724.md`

**Key insights:**
- GR plugin already exists with working capability injection
- **Existing format:** `capability hex 64 0078 peer <addr>` - keep this
- **Config change needed:** Currently receives text (`config peer X restart-time Y`), must change to JSON
- graceful-restart boolean at ze-bgp.yang:274 must move to ze-gr.yang
- GR coordinates with BGP and RIB via events/commands

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGRDeclareSchema` | `internal/plugin/gr/gr_test.go` | GR declares ze-gr YANG | |
| `TestGRReceiveConfig` | `internal/plugin/gr/gr_test.go` | GR receives config JSON | |
| `TestGRInjectCapability` | `internal/plugin/gr/gr_test.go` | GR sends capability hex | |
| `TestGRSubscribePeerEvents` | `internal/plugin/gr/gr_test.go` | GR subscribes to bgp.peer.* | |
| `TestGRHandleRestart` | `internal/plugin/gr/gr_test.go` | GR handles peer restart event | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `restart-time` | 0-4095 | 4095 | N/A (0 valid) | 4096 |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `hub-gr-capability` | `test/data/hub/gr-capability.ci` | GR injects cap into BGP OPEN | |
| `hub-gr-restart` | `test/data/hub/gr-restart.ci` | GR handles peer restart | |

## Files to Modify

- `yang/ze-bgp.yang` - Remove graceful-restart leaf (line 274)
- `internal/plugin/gr/gr.go` - Update for hub integration

## Files to Create

- `yang/ze-gr.yang` - GR YANG module (augments ze-bgp)
- `test/data/hub/gr-*.ci` - Functional tests

## Implementation Steps

1. **Write unit tests** - Test GR plugin behavior

   → **Review:** Test capability encoding?

2. **Run tests** - Verify FAIL (paste output)

3. **Create ze-gr.yang** - Augment ze-bgp
   ```yang
   module ze-gr {
       namespace "urn:ze:gr";
       prefix gr;

       import ze-bgp { prefix bgp; }

       augment "/bgp:bgp/bgp:peer/bgp:capability" {
           container graceful-restart {
               leaf enabled { type boolean; default false; }
               leaf restart-time {
                   type uint16 { range "0..4095"; }
                   default 120;
               }
           }
       }

       augment "/bgp:bgp/bgp:peer-group/bgp:capability" {
           // Same structure
       }
   }
   ```

4. **Remove from ze-bgp.yang** - Delete graceful-restart leaf

5. **Update GR plugin** - Hub integration
   - Declare ze-gr schema in Stage 1
   - Receive config JSON in Stage 2
   - Send `capability hex ...` after ready
   - Subscribe to `bgp.peer.*` events
   - Handle restart coordination

6. **Run tests** - Verify PASS (paste output)

7. **Verify** - `make lint && make test && make functional`

## Design Decisions

### GR capability injection flow

```
1. GR receives config JSON: {"enabled": true, "restart-time": 120}
2. GR encodes capability (RFC 4724: code 64, restart-time in lower 12 bits)
3. GR sends: capability hex 64 0078 peer 192.168.1.1
4. BGP stores capability for OPEN messages to that peer
```

**Note:** Existing GR plugin uses `capability hex <code> <value> peer <addr>` format. This is correct - keep it.

### GR event handling

| Event | GR Action |
|-------|-----------|
| `bgp.peer.connecting` | Check if GR enabled for peer |
| `bgp.peer.restart` | Start restart timer, defer routes |
| `bgp.peer.up` | Clear restart state |
| `bgp.peer.down` | If in restart, keep routes |

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [List actual changes]

### Bugs Found/Fixed
- [Any bugs discovered]

### Deviations from Plan
- [Any differences and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
