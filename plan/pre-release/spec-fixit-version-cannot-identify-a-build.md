# Spec: the version string cannot identify a build

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Moved to `plan/future/` on 2026-08-19. It is an improvement, not a release
defect: it matches none of the five defect kinds in `plan/future/README.md`.
The spec's premise also needs correcting before implementation. A make-built
host binary already resolves to a commit, because `readInfo` and `Extended`
(`internal/core/version/version.go`) read `vcs.revision` and `vcs.modified`
from the build info. The two real gaps are narrower: the Docker image tag is
date-only (`ZE_DOCKER_TAG` in the `internal/le/` native action tables), and `.dockerignore` excludes
`.git/`, so the image's binary carries no `vcs.*` stamp at all.

## Task

`ZE_VERSION := $(shell date +%y.%m.%d)` (`internal/le/` native action tables) is a build stamp, not a
release identity, and it reaches both `-X main.version` (through `ZE_LDFLAGS`)
and `ZE_DOCKER_TAG`. Two unrelated commits built on one day produce one version
and one image tag. One commit built on two days produces two versions. The
repository carries zero git tags, so nothing else recovers the identity.

Symptom for an operator: a bug report names a version that cannot be resolved to
a commit, and two images sharing a tag hold different code.

Goal: a version that identifies the commit it was built from. `git describe`
over an annotated tag is the smallest answer and it needs the first tag to
exist. Decide what an untagged tree reports and whether a dirty tree is marked.

Generating a changelog is a separate feature and belongs in its own spec. The
repository already writes conventional commits, but it also uses types a
generator would silently drop (`spec`, `plan`, `rules`, `journal`, `close`,
`rfc`, `bgp`, `tools`), so that decision must not ride along with this one.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - which checks fire on a the native action tables under `internal/le/` change
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/` native action tables - defines `ZE_VERSION`, `ZE_BUILD_DATE`, `ZE_LDFLAGS`, `ZE_DOCKER_TAG`

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
| <to be filled> | `test/plugin/version-identity.ci` | the reported version resolves to one commit |  <!-- doc-links: ignore (fixture this spec will create; the spec is `skeleton` and the work is not implemented) -->

## Files to Modify

- `internal/le/` native action tables - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify current mode full` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
