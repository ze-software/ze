# Spec: lint-linux-residual -- gosec + nilerr findings from linux-platform lint

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.golangci.yml` - the linter config (gosec `excludes` list, misspell exclusion)
3. `plan/learned/` for the `fix(lint)` commit that cleared the mechanical findings
4. Re-run `golangci-lint run <linux packages>` to get the CURRENT residual (line numbers below are pre-cleanup and will have shifted)

## Task

Context: linting on a linux host surfaced ~81 golangci findings in `//go:build linux`
files that had never been linted (the dev host is non-linux, so those files are
build-excluded there). A `fix(lint)` pass fixed the mechanical and correctness
findings in code (gocritic, goconst, errcheck, noctx, unconvert, unparam, unused,
nilnil, staticcheck). This spec owns the RESIDUAL that needs a judgment call rather
than a mechanical fix:

- **gosec (15):** G103 (unsafe.Pointer for ioctl/netlink syscalls), G304 (file read
  from a variable path under `/sys`/`/proc`), G306 (WriteFile permission bits on
  sysfs), G204 (subprocess launched with a variable).
- **nilerr (3):** functions that convert an error into a `plugin.Response.Error`
  field and return a `nil` Go error, or return best-effort/partial results on a
  context deadline. The repo ALREADY treats this pattern with `//nolint:nilerr`
  (see `internal/plugins/host-cmd/cmd/set_fd_linux.go:32`, `// operational error in
  Response`).

**Decision to make (per category):** justified `//nolint` (matching the existing
`set_fd_linux.go:32` precedent and the `.golangci.yml` gosec `excludes` precedent),
an in-code fix (path validation for G304, tighter perms for G306, constant/validated
args for G204), or a restructure. G103 (unsafe for syscalls) has no code alternative
and is a `//nolint` or `.golangci.yml` gosec-exclude decision.

This is a skeleton (captured intent, decision not started). Moves to `design` when picked up.

## Required Reading

### Architecture Docs
- [ ] `.golangci.yml` - the gosec `excludes` list (already excludes G104/G115/G602/G703/G704/G705/G706 with rationale)
  → Decision: the house pattern for a whole-class false positive is a documented `gosec.excludes` entry; per-site judgment calls use `//nolint:gosec // reason`.
- [ ] `internal/plugins/host-cmd/cmd/set_fd_linux.go` - line 32 `//nolint:nilerr // operational error in Response`
  → Constraint: the plugin-Response-conversion nilerr pattern is already sanctioned here; the 4 new nilerr are the same pattern or deliberate partial-on-ctx-deadline.
- [ ] `ai/rules/anti-rationalization.md` - do not silence a real bug; each nilerr must be confirmed intentional before nolint
  → Constraint: verify each nilerr site is genuinely best-effort/Response-conversion, not a swallowed real error, before choosing nolint.

**Key insights:**
- G103 unsafe is mandatory for the ioctl/netlink syscall ABI; there is no code fix.
- nilerr here is the plugin-Response error channel, already nolint'd elsewhere.

## Current Behavior (MANDATORY)

**Source files read:** (re-lint to refresh line numbers; reference by symbol)
- [ ] `internal/component/iface/offload_linux.go` - G103 unsafe (ethtool ioctl via `unsafe.Pointer`) and G306 (WriteFile to `/sys/class/net/.../features` style paths)
  → Constraint: unsafe is required by the ethtool ioctl struct; G306 perms may be tightenable to 0600 if sysfs accepts it.
- [ ] `internal/core/smart/smart_linux.go` - G103 unsafe (SMART/HDIO ioctl)
- [ ] `internal/component/l2tp/ppp/mtu_linux.go` - G103 unsafe (SIOCGIFMTU ioctl)
- [ ] `internal/component/config/system/conntrack_linux.go`, `console_linux.go` - G204 subprocess with a variable argument
- [ ] `internal/component/l2tp/iface_stats_linux.go`, `internal/component/telemetry/collector/conntrack_linux.go`, `internal/plugins/iface/netlink/show_linux.go` - G304 read from a variable `/sys`/`/proc` path
- [ ] `internal/plugins/diag/cmd/capture_interface_linux.go` - nilerr: `capturePcap` returns captured packets with a nil error when the context deadline breaks the loop (intentional partial result; `captureText`'s nilerr was removed when its always-nil error result was dropped)
- [ ] `internal/plugins/host-cmd/cmd/set_fd_linux.go` - nilerr: invalid-limit and getrlimit errors surfaced via `plugin.Response.Error`, nil Go error (same pattern as the file's line 32)
- [ ] `internal/plugins/iface/netlink/xfrm_linux.go` - nilerr: XFRM policy-list error returns best-effort interface info with nil error

**Behavior to preserve:**
- All runtime behavior. The syscall code paths (ethtool/SMART/MTU ioctls, netlink)
  must stay byte-for-byte functional. The plugin `Response` error contract is unchanged.

**Behavior to change:**
- Only the lint disposition: each residual finding either gets a justified `//nolint`
  (or a `gosec.excludes` entry for a whole class), a validated code fix, or a
  restructure. No syscall ABI change.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `make ze-verify` runs `make ze-lint` (full-tree golangci) on a linux host; the
  residual gosec/nilerr findings are the last ones keeping it red.

### Transformation Path
1. golangci lints the `//go:build linux` files (host GOOS=linux).
2. gosec flags unsafe/file/subprocess/perm patterns; nilerr flags error-checked-then-nil-returned.
3. Per finding: choose justified `//nolint` (with a specific reason), a code fix, or a `.golangci.yml` gosec-exclude for a whole false-positive class.
4. `make ze-lint` goes green once every residual is dispositioned.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Syscall ABI ↔ Go | `unsafe.Pointer` for ioctl structs (G103) — no code alternative | [ ] `make ze-lint` green |
| Plugin ↔ Response error channel | nilerr: error surfaced via `Response.Error`, nil Go error | [ ] pattern matches `set_fd_linux.go:32` |
| Filesystem ↔ reader | G304: `/sys`/`/proc` path from a validated interface/device name | [ ] path source audited |

### Integration Points
- `make ze-lint` / `make ze-verify` - the gate that goes green when the residual is cleared.
- `.golangci.yml` - `gosec.excludes` (whole-class) and `//nolint` (per-site) are the two disposition mechanisms.

### Architectural Verification
- [ ] No bypassed layers (lint disposition only; no runtime code path change unless a G304/G306 code fix is chosen)
- [ ] No unintended coupling (no new dependencies)
- [ ] No duplicated functionality (reuse existing `gosec.excludes` / `//nolint` mechanisms)
- [ ] Registration over hardcoding - N/A (lint disposition, no runtime registration)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every nilerr site is intentional (Response-conversion or best-effort), not a swallowed real error | `set_fd_linux.go:32` precedent; reading each site | a real error is being hidden (a bug) | read each of the 4 sites at design; confirm the error is surfaced or genuinely unactionable | unvalidated |
| A-2 | G103 unsafe usages are all syscall-ABI-required | ethtool/SMART/MTU ioctl structs need `unsafe.Pointer` | a G103 is avoidable in Go | read each of the 6 G103 sites | unvalidated |
| A-3 | G304 variable paths derive from validated system names (interface/device), not user input | the paths are `/sys`/`/proc` + a kernel-provided name | a G304 is a genuine path-traversal risk | trace the path variable's source at each site | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A blanket `//nolint` hides a real bug (a nilerr that IS swallowing an error) | reviewer disputes a nolint | per-site confirmation before nolint; propagate the error where it is actionable |
| R-2 | Whole-class `gosec.excludes` over-suppresses (silences future real findings of that rule) | a later real G304/G306 ships unflagged | prefer per-site `//nolint` over `gosec.excludes` unless the class is truly always-false-positive in this repo |

## Wiring Test (MANDATORY)

Wiring is the lint gate itself (contributor-facing; N/A for a `.ci`).

| Entry Point | → | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-lint` on linux | → | every residual gosec/nilerr dispositioned | `make ze-lint` exit 0 |
| `golangci-lint run <linux pkgs>` | → | 0 gosec + 0 nilerr findings remain | scoped lint run clean |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-lint` on a linux host after this spec | Exit 0; no gosec or nilerr findings remain in the linux-platform files |
| AC-2 | Each nilerr site | Confirmed intentional (Response-conversion or best-effort) with a specific `//nolint:nilerr // <reason>` matching the `set_fd_linux.go:32` precedent, OR the error is propagated if it was actually being swallowed (bug) |
| AC-3 | Each gosec site | Disposed as a justified `//nolint:gosec // <reason>`, a code fix (G304 path validation / G306 tighter perms / G204 constant args), or a documented `gosec.excludes` entry; no syscall behavior change |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| lint gate green | `make ze-lint` (golangci) | 0 gosec + 0 nilerr in linux-platform files | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A - lint disposition, no user-facing behaviour; verified by `make ze-lint` on linux, not a `.ci`. Existing syscall-path tests (ethtool/SMART/MTU/netlink) regression-guard that no behavior changed | lint gate | ze-lint green; syscall paths unchanged | |

## Files to Modify

- `internal/component/iface/offload_linux.go` - G103 (unsafe) + G306 (perms) disposition
- `internal/core/smart/smart_linux.go` - G103 disposition
- `internal/component/l2tp/ppp/mtu_linux.go` - G103 disposition
- `internal/component/config/system/conntrack_linux.go`, `internal/component/config/system/console_linux.go` - G204 disposition
- `internal/component/l2tp/iface_stats_linux.go`, `internal/component/telemetry/collector/conntrack_linux.go`, `internal/plugins/iface/netlink/show_linux.go` - G304 disposition
- `internal/plugins/diag/cmd/capture_interface_linux.go`, `internal/plugins/host-cmd/cmd/set_fd_linux.go`, `internal/plugins/iface/netlink/xfrm_linux.go` - nilerr disposition
- `.golangci.yml` - only if a whole-class `gosec.excludes` entry is chosen for a rule

## Implementation Steps

1. **Phase: audit** - re-run golangci scoped to the linux packages to get the current residual + line numbers (they shifted after the `fix(lint)` pass); validate A-1/A-2/A-3 by reading each site.
2. **Phase: nilerr disposition** - confirm each of the 4 sites is intentional; add `//nolint:nilerr // <reason>` matching `set_fd_linux.go:32`, or propagate the error if it was a real swallow.
3. **Phase: gosec disposition** - per site: code fix (G304 validate, G306 tighten, G204 constant args) or justified `//nolint:gosec`; G103 unsafe gets `//nolint:gosec // syscall ABI requires unsafe.Pointer`.
4. **Full verification** - `make ze-lint` green; `make ze-verify` on linux; syscall-path unit tests pass.
5. **Complete spec** - learned summary, two-commit closure per `ai/rules/planning.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 demonstrated
- [ ] `make ze-lint` exit 0 on a linux host
- [ ] Every nilerr confirmed intentional before nolint (no hidden bug)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (N/A here; recorded)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Sibling context: created while cleaning the linux-platform lint debt; the mechanical half landed in a `fix(lint)` commit.
