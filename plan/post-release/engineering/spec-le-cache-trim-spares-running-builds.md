# Spec: le-cache-trim-spares-running-builds

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Release-independent le tooling: a cache trim or clean that removes Go build cache entries under a running build turns a correct change red |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

On 2026-11-02, twice, a `go test -race -tags ze_bgp` run of the reactor and
bgp plugin packages under `./le job run` died mid-build with
`could not import ...: no such file or directory` while the disk had space.
The run used the shared `cache/go-cache`. Another session's `./le scratch trim`
or `./le scratch clean` ran at the same time, and the entries the build had
already resolved were gone when the compiler opened them.

Goal: a trim or clean never removes a Go build cache entry that a running
build holds, so a build that started never fails for a cache entry another
session removed. The cause is not yet traced: the age floor below should
already spare an entry a build uses, so the first task is to find which path
removed a young entry, or why a used entry looked old.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` - "When the disk is full", the scratch trim and clean contract
- [ ] `ai/doors/le-tooling.md` - sessions sharing one machine through `./le scratch`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/scratch/cachetrim.go` - `cacheEntryAgeFloor` (3h): no entry younger is removed; `trimCaches`, `removeOldest`, `walkGoFormatCache`
- [ ] `internal/le/scratch/cacheclean.go` - `cleanTargets` (checkout `cache/go-cache`, bootstrap cache, ambient cache), `cleanOldCacheEntries` (age floor only), `goCleanCache` (`go clean -cache`, the `all` mode), `isWriterRace`

**Behavior to preserve:**
- The age floor and the per-store budgets
- `./le scratch clean all` refusing while a build runs

**Behavior to change:**
- Whatever path removed entries a running build held (to be traced)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Cache entry age floor | `internal/le/scratch/cachetrim.go` `cacheEntryAgeFloor`, `removeOldest` | Spares every Go cache entry whose mtime is younger than three hours, on the premise that a running build reads only young entries | extends: Go refreshes an entry's mtime only when it is more than an hour old, so the premise needs checking against what a build actually holds |
| Age-floor clean | `internal/le/scratch/cacheclean.go` `cleanOldCacheEntries` | `./le scratch clean`: the store trim with a zero budget, the floor alone decides | extends: same premise |
| Full clean refusal | `internal/le/scratch/cacheclean.go` `CleanCaches`, `goCleanCache` | `./le scratch clean all` runs `go clean -cache` and refuses while a build runs | extends: check that the running-build detection sees a build started under `./le job run` |
| Store trim | `internal/le/scratch/storetrim.go` `trimStoresScanning` | Hourly trim of the cache rows and liveness stores | extends: it calls the same cache trim |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le scratch trim`, `./le scratch clean`, `./le scratch clean all` and the hourly store trim.

### Transformation Path
[to be traced: which removal path deleted an entry a running build held]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session to session | the shared `cache/go-cache` directory | No |

### Integration Points
- `cacheEntryAgeFloor`, `removeOldest`, `goCleanCache` - the removal paths this spec traces

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The removed entries came from a trim or clean, not from a full disk or another `go clean` | the 2026-11-02 row; the disk had space | the cause lies elsewhere and the row moves | reproduce: trim during a cold `go test` build | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le scratch trim` during a running build | → | `trimCaches` | [test name] |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A trim or clean runs while a `./le job run` build reads the shared Go cache | The build completes; no entry it holds is removed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/le/scratch/cachetrim_test.go` | [description] | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

## Files to Modify
- `internal/le/scratch/cachetrim.go` - [once traced]

## Files to Create
- [none known]

## Implementation Steps
1. Trace the removal path (A-1), then design.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| full-disk-false-red.md | 2026-11-02 | go-cache entries removed under a running `./le job run` build |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] The scoped package test of `internal/le/scratch` passes, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
