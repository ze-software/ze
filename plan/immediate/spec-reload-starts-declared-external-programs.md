# Spec: reload-starts-declared-external-programs

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config reload, plugin lifecycle |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make a configuration reload start an `external` program that the new
configuration declares and the running daemon does not run yet, so a reload that
adds `plugin { external X { ... } }` and attaches X to a peer leaves X running
and receiving what the peer grants it.

Owner decision (Thomas, 2026-10-11): starting external programs that a reload
declares goes in a separate spec, not in
`spec-bgp-live-receive-binding-and-bfd-keep-session`. This is that spec.

Open questions for the design:

- Whether a removed `external` block stops its program on reload, and in which
  step of the transaction relative to the peers that bind it.
- Where the start sits in the verify, apply, rollback order, and what the
  rollback does to a program the failed reload started.
- A changed `external` block (new command line): restart, or refuse.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - the verify, apply, rollback order the start must join
- [ ] `docs/guide/config-reload.md` - what an operator is told a reload changes live

## Current Behavior (MANDATORY)

Today a reload never starts such a program, and says nothing. The binding is
installed, the delivery graph names X, and no process exists to receive it.

**Source files read:** (2026-10-11, while finding why two attach `.ci` saw no fixture output)
- [ ] `internal/component/plugin/server/startup_autoload.go` - `Server.autoLoadForNewConfigPaths` is the only reload path that starts a plugin. It walks `registry.ConfigRootsMap()`, so it starts a registry-internal plugin whose config root the reload added, and never an `external` program
- [ ] `internal/component/plugin/server/` - no reload path updates `Server.config.Plugins`, the list the boot path starts external programs from, so a newly declared block is never seen

Evidence in the tree: `test/reload/bgp-attach-receive-only-live.ci` and
`test/reload/bgp-attach-dynamic-receive-only-live.ci` declare their external
fixture in the BOOT config, attached to no peer, to get past this; their comment
says why. Journal row: `plan/journal/unwired-feature.md`, 2026-10-11.

Related, not the same defect: `plan/journal/gate-excludes-part-of-its-population.md`,
2026-08-25 (`Server.ReloadConfig` verifies only plugins already running, so a
config root a reload adds is verified by nothing).

## Data Flow (MANDATORY)

### Entry Point
- [written at design time: SIGHUP or a CLI commit carrying a new `plugin { external ... }` block]

### Transformation Path
1. [written at design time]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [written at design time] | | No |

### Integration Points
- [written at design time]

## Risks & Assumptions

[written at design time]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [written at design time: the two attach `.ci` with the fixture declared only in the reloaded config] | → | | |

## Acceptance Criteria

[written at design time]

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [written at design time] | | | |

## Files to Modify
- [written at design time]

### Integration Checklist
- [written at design time]

### Documentation Update Checklist (BLOCKING)
- [written at design time: `docs/guide/config-reload.md`, `docs/architecture/config/apply-ordering.md`]

## Implementation Steps
1. [written at design time]

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
