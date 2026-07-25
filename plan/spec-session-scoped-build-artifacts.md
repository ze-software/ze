# Spec: session-scoped-build-artifacts

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-07-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/core/paths/paths.go` - `ConfigDirFromBinary` (the constraint that shaped the design)
4. `mk/test-functional.mk:52-136` - the existing isolated-test-binary mechanism
5. `.claude/hooks/lib/session_id.py` - the ONE session-id resolver

## Task

Two objectives from the owner:

1. **Every file created by an AI session lives under one per-session temp root**, so it is
   obvious who owns what and cleanup is a single `rm -rf`. Today functional-test runtime
   scratch goes to system `$TMPDIR` (`os.MkdirTemp("", "ze-functional-*")`) with random
   suffixes and no owner, and it leaks on crash.
2. **Binaries are characterized by the session ID**, so parallel sessions never have one
   session's build delete or modify another's binary mid-test.

Non-goals: moving shared-by-design artifacts (`tmp/test-timings.json`, `tmp/ze-verify.*`
and its lock, `tmp/qemu/`, `tmp/mutation-*`) — those are per-tree, not per-session.

## Required Reading

### Architecture Docs
- [ ] `mk/test-functional.mk:52-136` - the existing per-invocation isolated binary set
  → Decision: test binaries MUST keep canonical bare names (`ze`) inside a `bin/` subdir,
    because `.ci` tests exec them by bare name and the runner puts that dir first on PATH.
  → Constraint: the throwaway root's `bin/` subdir is deliberate — `ze` derives its
    config/DB dir from its own location, so a binary directly in the root yields
    "cannot determine database location".

### RFC Summaries (MUST for protocol work)
N/A — no protocol behavior changes.

**Key insights:**
- There is exactly ONE session-id resolver (`.claude/hooks/lib/session_id.py`), reached from
  Bash via the `_session_id` shim. Inventing a second derivation is banned (it drifted for
  weeks before being consolidated — see spec-fixit-session-id-collision).
- `tmp/s/<session-id>/` already exists as the per-session scratch root
  (`scripts/dev/session-scratch.sh:78`) and is already reaped at SessionEnd with a 24h
  `--reap` backstop. This spec routes new artifacts INTO it rather than inventing a root.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/core/paths/paths.go` - `ConfigDirFromBinary` maps `<prefix>/bin/ze` →
      `<prefix>/etc/ze`; `DefaultConfigDir` feeds it `os.Executable()`.
  → Constraint: relocating a binary MOVES its config/DB dir. The repo `etc/ze` holds live
    state (`database.zefs`), so dev binaries must stay under `<repo>/bin/` to keep resolving
    to `<repo>/etc/ze`. This is why dev binaries take a NAME suffix, not a new directory.
- [ ] `internal/test/runner/runner.go:145-180,216-263` - `NewRunner` defaults
      `zePath=<base>/bin/ze`, `testPath=<base>/bin/ze-test` (env `ze.bin`/`ze.test.bin`
      override); `Build` compiles into those fixed paths with NO lock; `tmpDir` is
      `os.MkdirTemp("", "ze-functional-*")` (system temp).
- [ ] `internal/test/cli/cmd_bgp.go:459-482` - `buildZe` same shared default.
- [ ] `mk/test-functional.mk:94-136` - auto mode already isolates via
      `tmp/testbin-pid-<make-PID>-<target>/bin/`, keyed on PID (not session).

**Behavior to preserve:**
- Humans and CI (no session id in env) keep today's exact paths: `bin/ze`, `bin/ze-test`,
  system-temp scratch. Zero behavior change off-session.
- `ze` config/DB resolution for dev binaries stays `<repo>/etc/ze`.
- `.ci` tests continue to exec `ze` / `ze-stripped` by bare name.
- `ZE_BIN` / `ZE_TEST_BIN` overrides keep winning over any new default.

**Behavior to change:**
- Under an AI session: `make ze` &co emit `bin/<name>-<sid>`; the Go test runner defaults its
  build output under `tmp/s/<sid>/`; test runtime scratch roots at `tmp/s/<sid>/`.

## Data Flow (MANDATORY)

### Entry Point
`CLAUDE_CODE_SESSION_ID`, exported by the CLI into every child process.

### Transformation Path
1. `.claude/hooks/lib/session_id.py` resolves the canonical id (4-source precedence).
2. Make: `mk/session.mk` calls it once → `ZE_SESSION_ID`, `ZE_BIN_SUFFIX`, `ZE_SESSION_DIR`.
3. Go: `sessionpath` helper reads `ZE_SESSION_ID` (fallback `CLAUDE_CODE_SESSION_ID`).
4. Consumers pick session-scoped or shared defaults from those.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hooks ↔ Make | `$(shell python3 .claude/hooks/lib/session_id.py)` | [ ] |
| Hooks ↔ Go | env var read at runtime | [ ] |
| Make ↔ Go runner | `ZE_BIN` / `ZE_TEST_BIN` (unchanged contract) | [ ] |

### Integration Points
- `scripts/dev/session-scratch.sh` — owns `tmp/s/<id>/` lifecycle; extended to reap
  `bin/*-<sid>` so suffixed dev binaries do not accumulate.

### Architectural Verification
- [ ] No bypassed layers (single session-id resolver reused, no second derivation)
- [ ] No unintended coupling
- [ ] No duplicated functionality (reuses `tmp/s/<id>/`, not a new root)
- [ ] Registration over hardcoding — N/A (build plumbing)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Relocating a binary moves its config/DB dir | `internal/core/paths/paths.go:32-60,70-86` | dev binaries could move freely | read the producer | confirmed |
| A-2 | `CLAUDE_CODE_SESSION_ID` reaches make/go subprocesses | `session_id.py:21-26` | need process-tree walk | resolver has 4-source fallback incl. minted cache | confirmed |
| A-3 | Go build cache is keyed by source, not `-o` path, so per-session binaries only re-link | Go toolchain behavior | per-session builds would be expensive | timing of second build | unvalidated |
| A-4 | `.ci` tests exec `ze` by bare name from the dir first on PATH | `mk/test-functional.mk:72-74`, `runner_exec.go:180-186` | suffixed test binaries would break tests | functional suite run | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Suffixed dev binaries accumulate in `bin/` | `ls bin/` grows | reap `bin/*-<sid>` in session-scratch `--clean`/`--reap` + SessionEnd |
| R-2 | A doc/script hardcoding `bin/ze` breaks under a session | grep hits | `make ze-path` indirection + docs updated; `bin/ze` still built off-session |
| R-3 | ~~`session_id.py` invoked per make run adds latency~~ | ~~slow `make`~~ | RESOLVED: make never invokes the resolver; it reads the exported variable. No subprocess at all |
| R-4 | Suffixed binaries in `bin/` are missed by the `tmp/s/<id>` reaper | `ls bin/` grows across sessions | `reap_binaries` in `session-scratch.sh` + the SessionEnd hook sweep `bin/*-<sid>`; verified by a fixture test that the shared `bin/ze` survives |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ZE_SESSION_ID` set + `make ze-path` | → | `mk/session.mk` suffix derivation | `TestSessionBinSuffixMakefile` (scripts/dev test) |
| `ZE_SESSION_ID` set + runner build | → | `sessionpath.SessionBinDir` | `TestSessionBinDirIsolatesSessions` |
| `ZE_SESSION_ID` unset (CI/human) | → | shared defaults | `TestSessionPathsFallBackToShared` |
| functional suite run | → | scratch under `tmp/s/<id>` | `TestScratchRootUnderSession` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ZE_SESSION_ID=abc`, `make ze` | binary at `bin/ze-abc`; `bin/ze` untouched |
| AC-2 | no session id in env, `make ze` | binary at `bin/ze` (today's behavior, unchanged) |
| AC-3 | `ZE_SESSION_ID=abc`, run `bin/ze-abc` | config dir still resolves to `<repo>/etc/ze` |
| AC-4 | two different session ids build concurrently | two distinct binary paths, neither overwrites the other |
| AC-5 | `ZE_SESSION_ID=abc`, Go runner builds | output under `tmp/s/abc/`, not shared `bin/ze` |
| AC-6 | no session id, Go runner builds | output at `<base>/bin/ze` (unchanged) |
| AC-7 | `ZE_BIN` set explicitly | override wins over session default |
| AC-8 | functional test runs under a session | runtime scratch created under `tmp/s/<id>/`, not system `$TMPDIR` |
| AC-9 | session ends | `tmp/s/<id>/` and `bin/*-<id>` both removed |
| AC-10 | unsafe/empty session id | falls back to shared paths; never escapes `tmp/s/` or `bin/` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Two sessions each run `make ze` concurrently | session_id.py → mk/session.mk → distinct `-o bin/ze-<sid>` | AC-4 test |
| 2 | Session runs a functional suite while the other builds | mk/test-functional.mk session suffix → isolated dir | functional suite green |
| 3 | Session ends; owner inspects the tree | SessionEnd → session-scratch cleanup | AC-9 test |
| 4 | CI (no session) builds and tests | fallback branch → `bin/ze` | AC-2/AC-6 tests |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionBinDirIsolatesSessions` | `internal/core/sessionpath/sessionpath_test.go` | two ids → two dirs | |
| `TestSessionPathsFallBackToShared` | same | no id → `<base>/bin` + system temp | |
| `TestSessionPathsRejectUnsafeID` | same | unsafe id → shared fallback (AC-10) | |
| `TestScratchRootUnderSession` | same | scratch root under `tmp/s/<id>` | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing functional suites | `test/**/*.ci` | suites still pass with session-scoped binaries (regression gate) | |

### Interop Tests (MANDATORY for protocol features)
N/A — build/test plumbing only, no wire-visible behavior.

## Files to Modify
- `Makefile` - include `mk/session.mk`; suffix binary outputs; add `ze-path`
- `mk/test-functional.mk` - session-keyed `ZE_RUN_SUFFIX` / dir under `tmp/s/<id>`
- `internal/test/runner/runner.go` - session-scoped build + scratch defaults
- `internal/test/cli/cmd_bgp.go` - session-scoped build default
- `internal/test/tmpfs/tmpfs.go`, `internal/test/runner/runner_exec.go`,
  `internal/test/runner/parsing.go` - scratch under session root
- `scripts/dev/session-scratch.sh` - reap `bin/*-<sid>`
- `.claude/hooks/pretool-bash.py` - accept session bin paths in `check_root_build`
- `ai/rules/bash-output.md`, `ai/rules/testing.md`, `ai/INDEX.md` - discovery

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | build plumbing only |
| CLI commands/flags | No | `ze-path` is a make target, not a ze command |
| Env var registration | No | `ZE_SESSION_ID` is harness-provided, not read via `env.Get` in daemon code |
| Doctor check | No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | build-plumbing only |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if it names binary paths |
| 12 | Internal architecture changed? | Yes | `ai/rules/testing.md` (Temporary Files), `ai/rules/bash-output.md` |
| 16 | Changed files referenced by doc source anchors? | Yes | grep `docs/` for anchors on changed files |

## Files to Create
- `mk/session.mk` - session id + suffix derivation for Make
- `internal/core/sessionpath/sessionpath.go` - session-scoped path helper
- `internal/core/sessionpath/sessionpath_test.go` - unit tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify / Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify-changed` + functional suites |
| 6. Critical review | Critical Review Checklist |
| 13. /ze-review gate | Review Gate |
| 14. Present summary + close | Executive Summary |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — `sessionpath` helper + `mk/session.mk`, failing tests
   - Tests: `TestSessionBinDirIsolatesSessions`, `TestSessionPathsFallBackToShared`
   - Files: `internal/core/sessionpath/`, `mk/session.mk`
   - Verify: tests fail on stub, pass on implementation
2. **Phase: Dev binary suffix** — Makefile outputs `bin/<name>-<sid>`, add `ze-path`
   - Files: `Makefile`
   - Verify: AC-1..AC-4 by building with/without a session id
3. **Phase: Test binary + scratch isolation** — runner/cmd_bgp defaults, scratch roots,
   `mk/test-functional.mk` session key
   - Files: `internal/test/**`, `mk/test-functional.mk`
   - Verify: AC-5..AC-8; functional suites still green
4. **Phase: Cleanup + guardrails** — reap `bin/*-<sid>`, hook accepts session paths, docs
   - Files: `scripts/dev/session-scratch.sh`, `.claude/hooks/pretool-bash.py`, rules/docs
   - Verify: AC-9; `make ze-hook-test`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Off-session paths byte-identical to today; overrides still win |
| Config resolution | Dev binaries still resolve `<repo>/etc/ze` (A-1) |
| Single source of truth | No second session-id derivation anywhere |
| Fail-closed | Unsafe/empty id falls back to shared, never escapes the root |
| Rule: no-layering | No compat shim; one path-resolution mechanism |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `mk/session.mk` | `ls mk/session.mk`; `make ze-path` prints suffixed path under a session |
| `sessionpath` helper + tests | `go test ./internal/core/sessionpath/` |
| Runner isolation | grep runner.go for `sessionpath.` |
| Reaping | `scripts/dev/session-scratch.sh --clean` removes `bin/*-<sid>` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Path traversal | session id is filename-safe (`_sid_safe`); reject `.`/`..`/`/` before joining |
| Deletion scope | reaper only ever removes `tmp/s/<id>` and `bin/*-<id>`, never `bin/ze` itself |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Functional test fails | Check binary name/PATH assumptions (A-4) |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Dev binaries could move to `tmp/s/<id>/bin/` freely | Binary location determines config/DB dir; repo `etc/ze` holds a live database | Read `paths.go:32-60,70-86` before implementing | Design changed to a NAME suffix for dev binaries; owner chose this trade-off |
| Make could resolve the id by calling `session_id.py` | `session_id()` ALWAYS returns an id -- source 4 MINTS one keyed on the topmost ancestor when no CLI ancestor exists (`session_id.py:198-217,278-286`). A human's `make ze` would have invented a session and suffixed their binaries | Read the resolver before wiring `mk/session.mk` | Make keys off the exported `CLAUDE_CODE_SESSION_ID` only. Side-effect-free, and risk R-3 (subprocess latency) disappears |
| `CLAUDE_PROJECT_DIR` is available to anchor the scratch root | It is exported to hooks but NOT into the shell that runs make (`python3 -c` in a Bash tool call prints unset) | Ran the suite, then checked where scratch actually landed | `DefaultScratchRoot` gained a go.mod repo-root walk; without it the scratch routing was silently inert |
| Session scoping should apply to the pre-built binary LOOKUP as well as the build | Scoping exists to stop one session's build overwriting another's binary; reading an existing binary clobbers nothing. Scoping the lookup broke `ZE_TEST_NO_BUILD` for a binary pre-built into the shared `bin/` | `TestBuildZeNoBuild` + `TestBuildNoBuildSkip` went red | `FindPrebuilt` tries the session bin/ then the shared bin/; an explicit `ZE_BIN`/`ZE_TEST_BIN` stays exempt so a miss there is still fatal. The two tests were left untouched -- they were right |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The repo already had every primitive needed (one session-id resolver, a reaped
  per-session root, an isolated-test-binary pattern). The work is routing existing
  producers into them, not building new infrastructure.
- Binary location is not a free variable in Ze: it is an input to config/DB resolution.

## Core Insight
A binary's path is part of its runtime contract. "Where do we put the binary" could not be
answered from build concerns alone — `ConfigDirFromBinary` made it a data-durability
question, which is why dev binaries take a suffix and test binaries take a directory.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Dev binaries suffixed `bin/<name>-<sid>` | `tmp/s/<id>/bin/<name>` | Preserves `<repo>/etc/ze` resolution and the live database; owner-selected |
| Test binaries under `tmp/s/<id>/…/bin/` | suffix like dev binaries | `.ci` tests exec bare names; isolated `etc/ze` is desirable for tests |
| Reuse `tmp/s/<id>/` | new `tmp/build/<id>/` root | Already created, already reaped at SessionEnd with a crash backstop |
| Gate on session-id presence | always session-scope | Zero behavior change for humans and CI |

## Known Limitations
- Shared-by-design artifacts stay shared (timings, verify logs/lock, qemu, mutation caches).
- `ZE_SUFFIX=<name>` remains explicitly non-isolated (documented at `mk/test-functional.mk:100-108`).
- Port-allocation locks stay global by design (`internal/test/runner/ports.go`).

## RFC Documentation
N/A — no protocol behavior.

## Implementation Summary

### What Was Implemented
- `mk/session.mk`: resolves `ZE_SESSION_ID` from the exported `CLAUDE_CODE_SESSION_ID`
  (validated: one word, no `/`, not `.`/`..`), derives `ZE_BIN_SUFFIX`, `ZEBIN_*` paths for
  all 8 canonical binaries, `ZE_SCRATCH_DIR`, and a `ze-path` target.
- `Makefile` + 8 `mk/*.mk`: 117 literal binary references replaced by `$(ZEBIN_*)`, so recipe
  outputs and prerequisites cannot disagree. QEMU DUT binaries suffixed too.
- `internal/test/sessionpath`: `ID`, `Root`, `BinDir`, `BinName`, `ScratchRoot`,
  `EnsureScratchRoot`, `DefaultScratchRoot` (+ `repoRoot` go.mod walk).
- `internal/test/runner/runner.go`: build output via `sessionpath.BinDir`, run scratch via
  `EnsureScratchRoot` — closes the unlocked shared-binary rebuild (Layer B).
- `internal/test/cli/cmd_bgp.go` (`buildZe`), `parsing.go`, `runner_exec.go`, `cmd_web.go`,
  `tmpfs/tmpfs.go`, `tmpfs/cleanup.go`: scratch rooted in the session dir.
- `mk/test-functional.mk`: throwaway test-binary sets moved under `$(ZE_SCRATCH_DIR)`.
- `scripts/dev/session-scratch.sh` + `.claude/hooks/session-end-scratch.sh`: `reap_binaries`
  sweeps `bin/*-<sid>`.
- `.claude/hooks/pretool-bash.py`: `check_root_build` accepts the session bin dir.

### Bugs Found/Fixed
- None in shipped product code. Two design defects were caught before shipping: Make minting
  a session id for humans, and the scratch anchor being silently inert (both in Mistake Log).

### Documentation Updates
- `ai/rules/bash-output.md`: new section on session-suffixed binaries (use the `ze-path`
  target, never hardcode the shared binary name).
- `ai/rules/testing.md` Temporary Files: session scratch dir is the preferred location, and
  the runner now roots its working dirs there.

### Deviations from Plan
- Make reads the exported variable instead of invoking `session_id.py` (the resolver mints an
  id as a last resort, which would have suffixed humans' binaries).
- `DefaultScratchRoot` needed a go.mod repo-root walk; `CLAUDE_PROJECT_DIR` is not exported
  into the shell that runs make.
- `bin/gok` and the cross-compiled installer left unsuffixed (see Known Limitations).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Session files under one per-session root | Done | `internal/test/sessionpath`, `ZE_SCRATCH_DIR` | runner scratch + test binary sets under the session root; observed live |
| 2. Binaries characterized by the session id | Done | `mk/session.mk` `ZE_BIN_SUFFIX`, `ZEBIN_*` | suffixed on-session, plain off-session |
| Cleanup is one operation | Done | `reap_binaries`, `session-end-scratch.sh` | dir removal + suffixed-binary sweep |
| No change for humans / CI | Done | fallback branches throughout | off-session dry run byte-identical |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ze-path` with a test id yields the suffixed name; `TestBinNameSuffixesUnderSession` | |
| AC-2 | Done | off-session dry run emits the shared name; `TestSessionPathsFallBackToShared` | identical to pre-change |
| AC-3 | Done | binary stays in `bin/`, so `ConfigDirFromBinary`'s prefix is still the repo root | design property |
| AC-4 | Done | `TestSessionBinDirIsolatesSessions` | two ids, two paths |
| AC-5 | Done | `NewRunner` uses `sessionpath.BinDir`; suites built under the session root | |
| AC-6 | Done | `TestSessionPathsFallBackToShared` | |
| AC-7 | Done | override branches left intact (`runner.go:155-167`) | |
| AC-8 | Done | live `ze-functional-*` dir observed under the session root during the encode run; `TestDefaultScratchRootFallsBackToRepoRoot` | |
| AC-9 | Done | reaper fixture: dead session's binary + scratch removed, shared and live preserved | |
| AC-10 | Done | `TestSessionPathsRejectUnsafeID`; unsafe id falls back to the shared path | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSessionBinDirIsolatesSessions` | Done | `internal/test/sessionpath/sessionpath_test.go` | |
| `TestSessionPathsFallBackToShared` | Done | same | |
| `TestSessionPathsRejectUnsafeID` | Done | same | |
| `TestScratchRootUnderSession` | Done | same | |
| `TestBinNameSuffixesUnderSession` | Done | same | added during implementation |
| `TestClaudeSessionIDFallback` | Done | same | added during implementation |
| `TestDefaultScratchRootFallsBackToRepoRoot` | Done | same | added after the inert-anchor finding |
| `TestDefaultScratchRootEmptyOffSession` | Done | same | added after the inert-anchor finding |
| Functional regression | Done | `test/parse` 254 PASS / 0 FAIL; `test/encode` 55 PASS / 0 FAIL | run with session-scoped binaries |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `mk/session.mk` | Created | |
| `internal/core/sessionpath/` | Changed | landed as `internal/test/sessionpath/` — only test infra consumes it, so it lives with its owner rather than in core |
| `Makefile`, `mk/test-functional.mk`, `mk/test-integration.mk` | Modified | |
| `internal/test/runner/{runner,parsing,runner_exec}.go`, `internal/test/cli/{cmd_bgp,cmd_web}.go`, `internal/test/tmpfs/{tmpfs,cleanup}.go` | Modified | |
| `scripts/dev/session-scratch.sh`, `.claude/hooks/session-end-scratch.sh`, `.claude/hooks/pretool-bash.py` | Modified | |
| `ai/rules/bash-output.md`, `ai/rules/testing.md` | Modified | |

### Audit Summary
- **Total items:** 24 (4 requirements, 10 ACs, 9 tests, 1 placement decision)
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (helper package placement; see Files from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Session files under one root, easy to delete | live functional run | Encode suite created a `ze-functional-*` working dir under `tmp/s/<id>/`; parse suite built its binaries into `tmp/s/<id>/testbin-pid-7756-ze-parse-test/bin/`. System-temp `ze-*` dir count unchanged across a full run (43 before, 43 after — all dated 2026-07-13/14, i.e. pre-change orphans) |
| Binaries characterized by the session id | build output, both modes | on-session output is `bin/ze-<full session uuid>`; off-session is `bin/ze`; the dependent `ze-exabgp-test` target resolves both suffixed prerequisites correctly |
| Parallel sessions cannot clobber | unit test + isolation by construction | `TestSessionBinDirIsolatesSessions`; the runner no longer writes the shared binary under a session, which was the last unlocked path |
| Deletion is one step | reaper fixture test | dead session's `bin/ze-deadsid`, `bin/ze-test-deadsid` and `tmp/s/deadsid/` removed; shared `bin/ze`, `bin/ze-test` and the live session's artifacts preserved |
| No regression for existing suites | functional suites | `test/parse` 254 PASS / 0 FAIL / 32 SKIP; `test/encode` 55 PASS / 0 FAIL; lint 0 issues; hook golden 131/131 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | QEMU netns driver hardcodes the unsuffixed DUT binary name while the make target now builds a suffixed one, so it execs a path that never exists under a session. No CI covers `ze-qemu-*` | `scripts/evidence/netns_qemu.py:151,157` vs `mk/test-integration.mk:385-387` | fixed: driver takes the paths from `ZE_QEMU_*`; target passes and exports them; new mutation-verified gate |
| 2 | BLOCKER | `make ze-validate` red (exit 2), a structural gate that blocks commits. 3 unwired exports mine, 10 pre-existing pulled into scope because the checker is changed-file scoped (`validate.py:430`) | `internal/test/{sessionpath,cli,runner,tmpfs}` | fixed via 3, 4 and the dead-code removals; gate now exits 0 |
| 3 | ISSUE | `BinName` was dead code -- dev-binary suffixing lives in `mk/session.mk`, so the Go twin had no production caller | `internal/test/sessionpath/sessionpath.go:127` | fixed: deleted, with its test |
| 4 | ISSUE | `SharedBinDir`, `ScratchRoot` exported with no cross-package caller | same file | fixed: unexported |
| 5 | ISSUE | Binary reaping inherited the 24h-idle heuristic that was safe only for scratch: scratch is recreated by `mkdir -p`, a binary is not, so a live-but-idle session could lose the binary it is about to run | `scripts/dev/session-scratch.sh` `reap_binaries` | fixed: `--reap` also requires the BINARY to be idle; `--clean`/SessionEnd stay unconditional |
| 6 | ISSUE | New make target undiscoverable: absent from `ai/INDEX.md` and `make help` | `ai/INDEX.md`, `Makefile` | fixed: Dev Tools row + help line |
| 7 | NOTE | `verifyPrebuilt` resolved each binary independently, so `ze` and `ze-test` could come from different directories while `.ci` tests exec siblings by bare name off one PATH entry | `internal/test/runner/runner.go` | fixed: `FindPrebuiltDir` resolves ONE directory holding every name |
| 8 | NOTE | Id validation guarded only the env source; `make ZE_SESSION_ID=../../etc` still reached the `-o` path (command-line vars outrank file assignments) | `mk/session.mk` | fixed: validation on the resolved id + `override` |
| 9 | NOTE | QEMU copy-paste hint printed unsuffixed names | `scripts/evidence/qemu-run.py:426` | fixed: prints the exported `ZE_QEMU_*` paths |
| 10 | NOTE | 10 `docs/` files reference `bin/ze` | `docs/` | acknowledged, no change: correct for humans and CI (off-session); agents covered by the new `ai/rules/bash-output.md` section |

### Fixes applied
- `scripts/evidence/netns_qemu.py`: `_qemu_bin()` resolves each DUT binary from `ZE_QEMU_BIN` / `ZE_QEMU_STRIPPED_BIN` / `ZE_QEMU_TEST_BIN`; the literal survives only as the standalone default. `setcap_binaries` and `run_suite` use them.
- `mk/test-integration.mk`: the netns target passes the three paths, and all three are exported for helper scripts.
- `scripts/dev/qemu_binary_paths_test.py` (new, 6 tests): built names carry `$(ZE_BIN_SUFFIX)`, the target passes them, the driver reads them, no hardcoded literal outside the fallback, env override wins, default preserved. Mutation-verified: reintroducing the literal turns it red.
- `internal/test/sessionpath`: `BinName` deleted; `SharedBinDir`/`ScratchRoot` unexported; `FindPrebuilt` replaced by directory-resolving `FindPrebuiltDir`.
- Dead code removed with owner approval: `internal/test/tmpfs/cleanup.go` (117 lines, zero external refs; the live twin is `WriteToTemp`, used at `runner_exec.go:75`), `Tmpfs.AddFileWithMode`, `(*Runner).Timings`.
- Unexported in-package-only symbols: `getNicks` (`internal/test/cli`), `resolveTmpfsPaths` / `defaultLimits` / `parseWithLimits` (`internal/test/tmpfs`).
- `scripts/dev/session-scratch.sh`: idle guard on `reap_binaries`; `session_scratch_test.py` gained 4 tests. Its `unittest.main()` block sat mid-file, so appended classes were never discovered -- moved to the end (that is why the first run reported 14 tests, not 18).
- `mk/session.mk`: `ZE_SESSION_ID ?=` + validation on the resolved id + `override`.
- `ai/INDEX.md` Dev Tools row and `make help` line for `make ze-path`.

### Run 2 (fresh pass over the fixes themselves)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 11 | ISSUE | Deleting `cleanup.go` left it named in the GENERATED discovery index; `commit_helper.py`'s discovery-index gate blocks a commit that leaves one stale | `ai/DOCS-TO-CODE.md:1471` | fixed: `make ze-discovery-index` (also added the new `internal/test/sessionpath` row to `ai/PACKAGE-MAP.md`) |
| 12 | ISSUE | Same deletion left a stale row in hand-written docs | `docs/functional-tests.md:1692` | fixed: row removed |
| 13 | ISSUE | Editing two rule files left the generated rules digest stale, failing `make ze-doc-test` (exit 2) | `ai/rules/CONDENSED.md` | fixed: `make ze-rules-condensed`; `ze-doc-test` now exits 0 |
| 14 | NOTE | `ze-qemu-debug` passed only `ZE_BIN`, so the runner's own `testPath` fell back to a default that is not what the target cross-compiled (pre-existing; the suffix turns an accidental match into a permanent divergence) | `mk/test-integration.mk:334` | fixed: passes `ZE_TEST_BIN="$(ZE_QEMU_TEST_BIN)"` |

### Run 3 (fresh full-diff pass, different lens: validators and name collisions)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 15 | ISSUE | `repoRoot()` matched the mere PRESENCE of a go.mod, but `tmp/` carries a TRACKED sentinel module (`module ze-tmp-scratch`). Any caller with a cwd under `tmp/` resolved the checkout root to `tmp/`, putting the scratch root at `tmp/tmp/s/<id>` -- a real directory, no error, and outside everything SessionEnd removes. Latent today (callers run from the repo root) but live the moment anything runs from the test-binary dirs, which now live under `tmp/` | `internal/test/sessionpath/sessionpath.go` `repoRoot` | fixed: `isRepoRoot` requires the ze module directive; `TestRepoRootSkipsTmpSentinelModule` builds the sentinel layout and asserts both `repoRoot` and `DefaultScratchRoot`. Mutation-verified |
| 16 | ISSUE | make's id validation was weaker than Go's (`a+b`, `a@b`, `a!b` accepted by make, rejected by `sidSafe`), so the build and the test runner could disagree about which artifacts belong to the session -- the drift the single-resolver design exists to prevent | `mk/session.mk` | fixed: charset check via `tr -d`, matching Go's set. Parity gate `scripts/dev/session_bin_suffix_test.py` asserts make accepts an id iff Go would |
| 17 | ISSUE | A session id equal to a binary suffix collided with a real binary: `make ze ZE_SESSION_ID=test` wrote a `ze` build over `bin/ze-test`. (The reaper half is narrower: it keys off `_session_id`, so deleting the real binary needs the RESOLVED id to be that word) | `mk/session.mk` | fixed: ids reproducing a name in `ZE_BIN_NAMES` are refused; the test also pins that list against the makefile so a new binary cannot escape the guard |
| 18 | NOTE | `FindPrebuiltDir(baseDir)` with no names returned the session bin dir: the "every name present" loop is vacuously true, so a non-existent directory read as a hit | `internal/test/sessionpath/sessionpath.go` | fixed: explicit zero-names guard + `TestFindPrebuiltDirNoNames`. Mutation-verified |

While fixing 16 I introduced and then removed a shell-injection hazard: the charset check interpolates the id into a shell command, because make's `export` reaches recipe environments but NOT `$(shell)` calls expanded during parsing (verified -- the exported-variable form silently validated nothing). A single quote is the only character that can terminate the single-quoted literal, so it is refused in pure make before the shell sees it, and `TestValidationDoesNotRunShell` asserts a quote-bearing id neither suffixes nor executes.

### Run 4 (full `make ze-verify` — found what the targeted gates could not)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 19 | BLOCKER | `TestWorkflowMakeTargetsExist` red: `pages.yml` invokes `make -C main bin/ze`, and after the rename no literal `bin/ze:` target exists (it is now `$(ZEBIN_ZE):`). Off-session the variable expands to `bin/ze` so CI still worked, but ON-session `make bin/ze` does not error -- make finds the existing FILE, prints "Nothing to be done" and silently leaves a stale binary. A silent no-op is worse than a failure | `.github/workflows/pages.yml:59` | fixed: invokes the phony `make -C main ze`, which always exists and always builds. `TestWorkflowMakeTargetsExist` passes and is itself the regression test |

Lesson: referencing a binary as a MAKE TARGET (`make bin/ze`) is no longer safe now that
binary file names are session-scoped; invoke the phony (`make ze`) or ask `make ze-path`.
The existing guard enforces this for every workflow, because no literal `bin/<name>:` target
remains for the canonical binaries.

This is also the answer to "were the targeted gates enough?": `ze-validate`, `ze-lint-changed`,
`ze-doc-test`, `ze-hook-test`, the unit packages and two functional suites were all green while
this was broken. Only the full verify runs `scripts/dev`'s workflow tests.

### Run 5 (gate sweep after Run 3 fixes)
`ze-validate` 0 · `ze-doc-test` 0 · `ze-lint-changed` 0 issues · `ze-hook-test` 0 ·
`go test ./internal/test/...` 0 (22 pkgs) · `qemu_binary_paths_test.py` 6/6 ·
`session_scratch_test.py` 18/18 · `session_bin_suffix_test.py` 6/6 ·
`audit-test-relaxation.py` clean · `test/parse` 254 PASS / 0 FAIL / 32 SKIP.

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (18 findings across 3 passes, all
      resolved; Run 4 gate sweep clean)
- [x] All NOTEs recorded above (7, 8, 9, 14, 18 fixed; 10 acknowledged with reason)

### Resolved during the review: the cross-session index row
Regenerating `ai/LEARNED-FULL-INDEX.md` (required by finding 11) absorbed
`1271-fixit-bgp-egress-rail-divergence`, an UNTRACKED learned summary belonging to another
session -- it was already `??` in this session's opening `git status`, the
shared-generated-file cross-commit hazard documented in `ai/rules/git-safety.md`.

Resolved on owner instruction: commit `6d36fc79e` adds the summary and its index row
together, so the generated index never names a file that does not exist. That commit needed
two overrides, both recorded in it: `--unverified` (docs-only, `git-safety.md` Step 0 exempts
markdown-only commits) and `--review-override` -- the commit-time gate reads "adds a learned
summary" as a spec closure and demands a review artifact keyed to THIS session, but
`plan/spec-fixit-bgp-egress-rail-divergence.md` is still OPEN, no code is in the commit, and
that work was authored and reviewed by session `94f6df1c` (artifact
`tmp/review/fixit-bgp-egress-rail-divergence-94f6df1c-....md`, verdict `findings`; rounds 5-6
at `6b8ee4311`). This session wrote none of that code and could not review it.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
