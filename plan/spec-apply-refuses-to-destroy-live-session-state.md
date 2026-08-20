# Spec: an apply that would destroy live session state must refuse

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Nothing in Ze asks whether applying a desired configuration would discard state
a subscriber or a peer is currently using. Reconcile paths converge on the
desired state and report what they changed. None classifies a change by whether
converging costs live state, and none gives an operator a way to say "do it
anyway".

`reconcileOnReadyWithJournal` (`internal/component/iface/config_apply.go`) is
the strongest reconciler in the tree and the reference for how readback and
convergence are done here: it re-reads live state on every pass, owns devices
through the `ze:owned:` `IFLA_IFALIAS` kernel marker so it stays stateless
across a crash, and fails closed when a foreign device holds the name. What it
does not do, and nothing else does, is refuse.

Goal: on surfaces holding per-session or per-subscriber state, classify a
pending change as one that converges in place or one that requires teardown,
count what teardown would discard, and refuse the apply when that count is not
zero unless an operator has set an explicit override. The error names every
affected object, not the first one found.

Open at design time: which surfaces qualify. Candidates are IKE/IPsec child SAs,
L2TP sessions, and firewall or NAT state keyed per flow. Decide whether this is
one shared contract or a property each surface implements, and prefer the latter
unless three surfaces genuinely need the same code (`ai/rules/simplicity.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config.md` - how a config option and its apply path are shaped
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/iface/config_apply.go` - reconciles interfaces, never refuses an apply

**Behavior to preserve:**
- <to be filled>

**Behavior to change:**
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| <to be filled> | <to be filled> | <to be filled> |

### Integration Points
- <to be filled>

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <to be filled> | → | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | <to be filled> | <to be filled> |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | `test/plugin/apply-refuses-live-state.ci` | an apply that would drop live state is refused, and the override lets it through |  <!-- doc-links: ignore (fixture this spec will create; the spec is `skeleton` and the work is not implemented) -->

## Files to Modify

- `internal/component/iface/config_apply.go` - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `make ze-precommit-verify` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
