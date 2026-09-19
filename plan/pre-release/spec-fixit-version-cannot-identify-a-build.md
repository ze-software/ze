# Spec: container build provenance

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The 2026-08-19 amendment classified this as optional work and narrowed its
scope to containers. That disposition is preserved pending owner triage of
the current pre-release placement. `spec-release-distribution.md` explicitly
excludes containers, so this file is not evidence of an approved first-release
container channel.

## Task

Give the deployment and lab container builds a source identity that a bug report
can resolve to the commit built, and define how their image tags identify that
build. Host builds with Go VCS metadata already report the commit and modified
state through `readInfo` and `Extended` in `internal/core/version/version.go`.
The date-only release string alone is not the whole host identity.

`docs/guide/docker.md` now uses direct `docker build -t ...` commands.
`docker/Dockerfile` and `docker/Dockerfile.lab` build inside the context, pass
only release/build-date ldflags, and default them to `dev` and `unknown`.
`.dockerignore` excludes `.git/`, so those builds have no checkout metadata
from which Go can stamp `vcs.revision`. The retired `ZE_DOCKER_TAG` action-table
claim no longer describes the producer: image tags are supplied to Docker.

An operator using these container recipes can therefore report an image tag
that is reused for different code without the binary exposing the source
commit. Design must settle container commit stamping, tag identity and dirty
or untagged source handling while preserving the host version contract.
`git describe` is an option to evaluate, not a requirement to replace Ze's
existing `YY.MM.DD` release format.

Owner decision before scheduling: retain this as optional container provenance
work outside the first-release gate, or explicitly make these container recipes
a first-release support obligation. No relocation or scope expansion is made
by this reconciliation.

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
- [ ] `internal/core/version/version.go` (`readInfo`, `Extended`) - reads and reports host VCS metadata
- [ ] `docker/Dockerfile`, `docker/Dockerfile.lab` - container compilation and release/build-date arguments
- [ ] `.dockerignore` - excludes Git metadata from those build contexts
- [ ] `docs/guide/docker.md` - direct Docker tagging and build commands

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

- `docker/Dockerfile`, `docker/Dockerfile.lab`, `.dockerignore` and `docs/guide/docker.md` - exact changes to be settled during design

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
