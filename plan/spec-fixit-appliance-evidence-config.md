# Spec: fixit-appliance-evidence-config

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 4/4 (complete; AC-3 qemu L2TP evidence green, 5/5 real PPP sessions) |
| Updated | 2026-07-11 |

> **AC-3 closed 2026-07-11.** The full `ze-deployment-gokrazy-l2tp-ppp-test`
> passes end to end (real xl2tpd/pppd L2TP PPP session: session established,
> subscriber route inject, `PPP session up interface=ppp0`, dataplane ping,
> clean teardown; 5/5 runs). Reaching green required fixes OUTSIDE the two
> config bugs -- all in the appliance-logging path or the evidence harness, the
> appliance L2TP code itself being correct. See `plan/learned/1106-gokrazy-l2tp-evidence-networking.md`:
> (a) `printk.devkmsg=on` (`gokrazy/ze/config.json`) -- the kernel `/dev/kmsg`
> rate-limiter was dropping the `web server listening` log the harness greps
> (the "web hang" was a dropped log, not a hang); (b) qemu slirp does not deliver
> inbound UDP hostfwd to the guest, so the harness now uses a TAP+bridge+dnsmasq
> underlay (`scripts/evidence/effective-gokrazy-l2tp-ppp.py`) with a ufw hole for
> DHCP; (c) xl2tpd 1.3.18 long-config-path truncation (LAC files at a short
> path); (d) `IPv6 not supported by static pool` removed from `FATAL_NEEDLES`
> (benign IPv4-only IPv6CP decline); (e) `prepare_instance` rewrites the custom
> kernel replace to absolute. New host dep: `dnsmasq`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/init/main.go` (`daemonRunning`, `runInit` active-config write), `scripts/evidence/effective-gokrazy-l2tp-ppp.py`, `mk/gokrazy.mk`

## Task

Two independent bugs in the appliance build/config flow, both surfaced by
`ze-deployment-gokrazy-l2tp-ppp-test` while verifying `spec-gokrazy-init-bump`
(AC-6) and `spec-iface-absent-link-graceful` (AC-3). They block a green gokrazy
L2TP appliance run on this host and are unrelated to the gokrazy bump or the
interface graceful-skip fix (both of which are done + proven).

**Bug 1 — `daemonRunning` false-positive vs a non-ze sshd.** `ze init --force`
refuses to replace the DB when a daemon is "running", but the check
(`daemonRunning`, `internal/plugins/init/main.go:419-437`) reads the DB's stored
SSH host:port and merely **dials TCP** to it — any listener answers. When the
appliance SSH is configured on `0.0.0.0:22`, the probe hits the **host's own
sshd** and false-reports a running daemon, aborting the fresh init. A build then
silently reuses a stale seed DB. (Worked around at the build layer in
`mk/gokrazy.mk` by deleting the seed DB before `ze init --force`; the real fix is
here.)

**Bug 2 — `ze init` active config shadows the appliance template.** `ze init`
writes discovered interfaces to the **active** config key
(`zefs.KeyFileActive.Key("ze.conf")`, `internal/plugins/init/main.go:279`). A
`GOKRAZY_TEMPLATE` provided at build time is written to a **separate** template
key (`file/template/ze.conf`) that the appliance only consults on a first boot
with no active config. So the template is **never applied**: the appliance boots
the init-written active config (build-host interfaces, SSH, DHCP) and any
template-only settings (web, l2tp) never take effect. Observed: the L2TP
appliance never starts its web server, so the evidence harness times out.

Not covered here: the gokrazy bump and the iface graceful-skip fix (separate, done).

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/init/main.go` - `daemonRunning` + `runInit`
  → Constraint: `daemonRunning` (`:419-437`) proves "running" only by dialing the stored SSH host:port; it must instead confirm a **ze** daemon (protocol handshake or a ze-specific marker), not any TCP listener.
  → Constraint: `runInit` writes discovered interfaces to `zefs.KeyFileActive.Key("ze.conf")` (`:279`) — the ACTIVE config, which shadows `file/template/ze.conf` on boot.
- [ ] `scripts/evidence/effective-gokrazy-l2tp-ppp.py` - the evidence harness
  → Constraint: `build_image` runs `make ze-gokrazy ... GOKRAZY_TEMPLATE=<l2tp cfg>`; the template must actually become the appliance's effective config for web/l2tp to start.
- [ ] `mk/gokrazy.mk` - the appliance build
  → Constraint: the credential step already deletes the stale seed DB (this spec's Bug-1 workaround); a real Bug-1 fix in `daemonRunning` lets that deletion be removed.

### RFC Summaries (MUST for protocol work)
- N/A — no protocol/wire change.

**Key insights:**
- Bug 1 and Bug 2 are independent; either alone breaks a repeatable gokrazy appliance evidence run.
- The interface graceful-skip fix already lets the appliance boot past a missing NIC; these two bugs are why web/l2tp still don't come up in the evidence test.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/init/main.go` - `daemonRunning` dials SSH host:port; `runInit` writes `KeyFileActive`.
  → Constraint: preserve the intent of the daemon guard (don't clobber a DB a live ze is using) while removing the non-ze false positive.

**Behavior to preserve:**
- `ze init` still refuses to replace a DB a live **ze** daemon is actively using.
- Normal (no-template) appliance builds still get a working discovered active config.

**Behavior to change:**
- `daemonRunning` must not treat an arbitrary TCP listener (e.g. host sshd) as a ze daemon.
- A build-time template must become the appliance's effective config (web/l2tp enabled).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Build: `make ze-gokrazy USER/PASS GOKRAZY_TEMPLATE=...` → `ze init` + `ze data write file/template/ze.conf`.
- Boot: appliance loads `file/active/ze.conf` (init-written) — template ignored.

### Transformation Path
1. `ze init --force` → `daemonRunning` (dials SSH port) → false positive → abort OR (post-workaround) fresh DB.
2. `runInit` → `EmitConfig(discovered)` → `WriteFile(KeyFileActive, ...)` (active config).
3. `ze data write file/template/ze.conf` → template file (separate key).
4. Appliance boot → uses active config → template never applied.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| build host ↔ seed DB | `ze init` / `ze data write` | [ ] |
| seed DB ↔ appliance boot | active vs template config keys | [ ] |

### Integration Points
- `daemonRunning`, `runInit` (`internal/plugins/init/main.go`).
- `mk/gokrazy.mk` template handling; `effective-gokrazy-l2tp-ppp.py` build.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable (N/A)
- [ ] Registration over hardcoding (N/A)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A ze daemon can be distinguished from a generic TCP listener via a cheap probe (handshake/banner or a ze-specific PID/lock) | `daemonRunning` currently only dials | can't safely fix Bug 1 without a real ze liveness signal | prototype the probe against a live ze + a bare sshd | **confirmed** — ze's SSH server sends an SSH identification banner on TCP accept. `wish.WithVersion("ze")` sets it to `SSH-2.0-ze` (charmbracelet/ssh `server.go:149-150` prepends `SSH-2.0-`; default is `SSH-2.0-Go`, x/crypto/ssh `transport.go:305`). Probe reads the banner and requires the ze prefix; host sshd sends `SSH-2.0-OpenSSH_*` → rejected. No PID/lock exists (`ze.pid.file` is operator-optional; the zefs DB is not flocked). |
| A-2 | Making the build-time template the effective (active) config yields web/l2tp on boot without breaking normal discovery | template has `dhcp-auto true`; the appliance discovers its own NIC at boot | web/l2tp still absent, or discovery regressions | rebuild L2TP image + boot in QEMU | **confirmed (mechanism)** — removing init's active-config shadow (via `--seed`) leaves no `file/active/ze.conf`, so boot runs the EXISTING merge `bootstrapConfigFromTemplate` (`cmd/ze/ze_core_start.go:196,427`): template (web/l2tp) + on-device `EmitSetConfigWithDHCP(discovered)` (`internal/component/iface/emit.go:131`). Discovery preserved via the merge. End-to-end proof: L2TP evidence test (AC-3). |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stricter `daemonRunning` misses a genuinely-running ze and clobbers a live DB | live ze DB replaced under it | **positive-identification model**: return "running" ONLY on a confirmed `SSH-2.0-ze` banner; every other outcome (dial fail, foreign banner, silent listener, read timeout) → "not running". A live ze serving this DB answers with its banner immediately on accept, so genuine ambiguity is rare; the guard also sits behind `--force` (interactive builds still get the confirmation prompt). Defaulting to "running" on ambiguity (the original mitigation text) is rejected because it resurrects the exact false-positive class this spec fixes — see Key Design Decisions. |
| R-2 | Template-as-active-config drops interface discovery the appliance needs | appliance has no usable NIC config | keep `dhcp-auto` in the template; merge discovery + template rather than replace — satisfied by the existing `bootstrapConfigFromTemplate` merge, which the `--seed` fix re-enables on the appliance |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init --force` with a host sshd on the config'd SSH port | → | `daemonRunning` returns false (not a ze daemon) | `test/appliance/serial-login.ci` (appliance still builds/boots) + a new `daemonRunning` unit test |
| gokrazy image built with a template | → | appliance boots with the template's web/l2tp effective | `ze-deployment-gokrazy-l2tp-ppp-test` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `daemonRunning(db)` where the stored SSH port is served by a non-ze listener (e.g. host sshd) | Returns false; `ze init --force` proceeds. A genuinely-running ze still returns true. |
| AC-2 | Appliance image built with `GOKRAZY_TEMPLATE=<web+l2tp cfg>` | The appliance's effective boot config enables web + l2tp (template applied, not shadowed by the init active config). |
| AC-3 | `ze-deployment-gokrazy-l2tp-ppp-test` end to end | ze web server starts, L2TP listener binds, and a real xl2tpd/pppd session completes (unblocks gokrazy-init-bump AC-6 + iface-absent-link AC-3). Then the `mk/gokrazy.mk` seed-DB-delete workaround can be removed. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds + boots an appliance image with a config template | build → template becomes effective config → web/l2tp start | `ze-deployment-gokrazy-l2tp-ppp-test` |
| 2 | runs `ze init` on a host with sshd on :22 | daemonRunning correctly ignores non-ze sshd → fresh DB | `test/appliance/serial-login.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDaemonRunningIgnoresNonZeListener` | `internal/plugins/init/daemon_internal_test.go` | AC-1: OpenSSH/generic-Go/silent/noise/oversized listeners are not treated as ze | PASS |
| `TestDaemonRunningAcceptsZeBanner` | `internal/plugins/init/daemon_internal_test.go` | AC-1: a real `SSH-2.0-ze` banner IS treated as running | PASS |
| `TestDaemonRunningFalseWhenPortUnreachable` | `internal/plugins/init/daemon_internal_test.go` | AC-1: nothing listening → not running | PASS |
| `TestDaemonRunningTimesOutOnSilentListener` | `internal/plugins/init/daemon_internal_test.go` | probe safety: bounded read, no hang on a silent listener | PASS |
| `TestServerAnnouncesZeBanner` | `internal/component/ssh/ssh_test.go` | wiring: the real ze SSH server announces exactly `SSH-2.0-ze` (what the probe requires) | PASS |
| `TestZeInitSeedSkipsActiveConfig` | `internal/plugins/init/seed_internal_test.go` | AC-2: `--seed` writes no `file/active/ze.conf` (no shadow); creds still written | PASS |
| `TestZeInitWithoutSeedWritesActiveConfig` | `internal/plugins/init/seed_internal_test.go` | AC-2: non-seed DOES write it → the flag gates the write | PASS |
| `TestBootstrapConfigFromTemplateAppliesWebL2TP` | `cmd/ze/bootstrap_template_test.go` | AC-2: real boot path merges template (web+l2tp) into the effective active config when no active exists | PASS |

Note: the AC-1 daemonRunning test lives in a white-box internal test file (`daemon_internal_test.go`, `package init`) rather than `main_test.go` (`package init_test`), because `daemonRunning` is unexported.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/appliance/serial-login.ci` | `test/appliance/` | appliance still builds + boots to serial login after the daemonRunning fix | not run this session (requires image build/boot) |

Integration proof (AC-2/AC-3): `ze-deployment-gokrazy-l2tp-ppp-test` — the appliance boots with the template's web/l2tp effective and completes an L2TP/PPP session. **Not runnable in this session**: the evidence harness requires root/CAP_NET_ADMIN and a built custom kernel (`tmp/kernel/vmlinuz`), neither available here (uid 1000, no vmlinuz). AC-2's mechanism is instead proven deterministically by `TestBootstrapConfigFromTemplateAppliesWebL2TP` (real boot path) + the `--seed` unit tests; AC-1 by the daemonRunning suite. The full qemu L2TP run must be executed on a root-capable host with the kernel built (`make ze-kernel`) to close AC-3.

### Interop Tests (MANDATORY for protocol features)
- N/A — no wire protocol change.

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/plugins/init/main.go` - `daemonRunning` (real ze liveness probe) + the template/active-config precedence.
- `mk/gokrazy.mk` and/or `scripts/evidence/effective-gokrazy-l2tp-ppp.py` - make a build-time template the effective config; remove the seed-DB-delete workaround once Bug 1 is fixed.
- `internal/plugins/init/main_test.go` - AC-1 unit test.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A | no config surface change |
| YANG validation | N/A | — |
| YANG custom validators | N/A | — |
| CLI commands/flags | N/A | — |
| CLI grammar | N/A | — |
| Editor autocomplete | N/A | — |
| Functional test for new RPC/API | N/A | covered by serial-login.ci + the L2TP evidence test |
| Pipe completeness | N/A | — |
| Env var registration | N/A | — |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | N/A | — |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | bug fixes |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | No | `docs/guide/appliance.md` (build flow) if template semantics change |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior? | No | — |
| 10 | Test infrastructure changed? | Yes | `mk/gokrazy.mk` comment + this test flow if the seed-DB workaround is removed |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | Yes | note the init active-vs-template config precedence |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | No | — |
| 15 | Registered plugin/event/command/inventory changed? | No | — |
| 16 | Any changed source file referenced by doc source anchors? | Yes | grep `docs/` for `init/main.go` anchors |
| 17 | Existing docs show examples for this area? | No | — |

## Files to Create
- None (extends existing init + build flow).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify + assumptions |
| 3. Wiring phase | Wiring Test |
| 4. Implement | Phases below |
| 5. Full verification | unit + the L2TP evidence test |
| 6-9. Critical review | Critical Review Checklist |
| 10-12. Deliverables/security/docs | Checklists |
| 13. /ze-review gate | Review Gate |
| 14. Close | Two-commit closure |

### Implementation Phases
1. **Phase: daemonRunning real liveness** — replace the bare TCP dial with a ze-specific liveness probe; unit test AC-1.
2. **Phase: template precedence** — make a build-time template the appliance's effective config (merge template + boot-time discovery), so web/l2tp start.
3. **Phase: remove the workaround** — drop the `mk/gokrazy.mk` seed-DB delete once Bug 1 is fixed.
4. **Verification** — the gokrazy L2TP evidence test goes green.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1/2/3 demonstrated |
| Correctness | daemonRunning still catches a real ze; template applied on boot |
| Data flow | init config precedence correct; no discovery regression |
| Rule: no-workarounds | Bug 1 fixed at source; the Makefile workaround removed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| daemonRunning fix | `go test ./internal/plugins/init/ -run DaemonRunning` |
| Template applied | L2TP appliance boots with web/l2tp |
| Evidence green | `ze-deployment-gokrazy-l2tp-ppp-test` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| DB clobber safety | a stricter daemonRunning must not replace a DB a live ze is using |
| Probe safety | the liveness probe must not act on untrusted responses |

### Failure Routing
| Failure | Route To |
|---------|----------|
| daemonRunning still false-positives | Phase 1 |
| template still shadowed | Phase 2 |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Delete the seed DB in mk/gokrazy.mk | unblocks the build but is a workaround for the daemonRunning false-positive | fix daemonRunning (Bug 1) then remove the delete |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

Surfaced while verifying `spec-gokrazy-init-bump` (AC-6) and
`spec-iface-absent-link-graceful` (AC-3) via `ze-deployment-gokrazy-l2tp-ppp-test`
on a Proxmox-style host (build host has `ens18`+`docker0`, guest NIC is `eth0`,
host sshd on `:22`). The interface graceful-skip fix let the appliance boot past
the missing `ens18`; these two config-flow bugs are why web/l2tp still don't come
up. See `plan/spec-iface-absent-link-graceful.md` Design Insights for the boot log.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| File both bugs as one fixit spec | one spec each | they share the init/build config flow and one integration proof |
| Bug 1: set ze's SSH banner to `SSH-2.0-ze` (`wish.WithVersion`) and require it in the probe | match `SSH-2.0-Go` (library default) only; PID-file check; DB flock | `SSH-2.0-Go` is any Go SSH server, not ze-specific, and is a fragile coupling to a library default (spec Constraint: "a ze-specific marker, not any TCP listener"). No PID/lock signal exists universally. A distinctive banner is a true positive ze marker, costs one line on the server, is not asserted by any test/interop, and does not affect host-key verification. |
| Bug 1: positive-identification model (only a confirmed ze banner ⇒ "running") | R-1's "default to running on ambiguity" | Defaulting to "running" on ambiguity would treat a silent/foreign listener as a live ze and re-block the build — the very false positive being fixed. A real ze answers immediately on accept; the guard also sits behind `--force`. So "cannot confirm ze ⇒ not the ze we protect" is both safe and correct here. AC-1 ("a non-ze listener is not treated as a ze daemon") requires exactly this. |
| Bug 2: add `ze init --seed` that skips baking this host's interface discovery into the active config | reorder Makefile to write template before init (impossible — init creates the DB); write discovery to the template key (pollutes the appliance config with build-host NICs); make boot prefer template over an init-written active config (needs a fragile "init-generated" marker) | init runs before the template exists, so it cannot detect a template. `--seed` cleanly says "this is an appliance seed; the effective config is built at first boot from template + on-device discovery." No active config ⇒ boot runs the existing template+discovery merge. Normal (non-seed) `ze init` is unchanged. |

## Known Limitations
- Until implemented, the gokrazy L2TP appliance evidence test cannot go fully green on a host with sshd on :22; the interface fix is verified only up to "appliance boots + interface config applied".

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| daemonRunning no longer false-positives on a non-ze listener | unit | `TestDaemonRunningIgnoresNonZeListener` (5 non-ze listener kinds → false) + `TestDaemonRunningAcceptsZeBanner` (real ze banner → true), both PASS |
| ze SSH server emits the marker the probe needs | unit (wiring) | `TestServerAnnouncesZeBanner`: live server announces `SSH-2.0-ze`, PASS |
| Build-time template becomes effective config | unit (real boot path) | `TestBootstrapConfigFromTemplateAppliesWebL2TP`: template web+l2tp preserved into the effective active config when no active exists (the `--seed` state), PASS; `--seed` init writes no active shadow (`TestZeInitSeedSkipsActiveConfig`), PASS |
| Full gokrazy L2TP evidence green | integration | `ze-deployment-gokrazy-l2tp-ppp-test` — **NOT run this session** (needs root + built kernel). Must run on a root-capable host (spec kept open until it passes); mechanism proven by the unit tests above. |

## Review Gate

### Run 1 (scoped adversarial self-review of the spec's own diff)
Note: the working tree is SHARED with concurrent sessions (many unrelated
uncommitted files — gokrazy modcache, ntp, authradius, mrt, static/vpp, other
specs), so a whole-tree `/ze-review` would review other sessions' code. Review
was scoped to this spec's files only. A formal `/ze-review` should be re-run,
scoped, once the tree is coordinated at commit time.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `daemonRunning` worst case is now ~4s (2s dial + 2s banner read) vs 2s before | `init/main.go` | Accepted: one-time `--force` gate, not a hot path |
| 2 | NOTE | probe reads untrusted bytes | `isZeSSHBanner` | Mitigated: `io.LimitReader(conn,255)` + 2s deadline; only a prefix compare, no action on content |
| 3 | NOTE | banner/probe could drift if `wish.WithVersion` arg changed by hand | `ssh.go` / `client.go` | Guarded by `TestServerAnnouncesZeBanner` (asserts the real server emits `sshclient.ServerVersionBanner`) |

Checked: correctness (positive-ID model; empty `if seed{}` intentional, nolint'd),
data flow (no active shadow → existing boot merge runs), security (bounded read,
no untrusted-data action), no-workaround (Bug 1 fixed at source, mk workaround
removed), backward-compat (banner change does not affect host-key verification;
ssh-* functional tests pass). 0 BLOCKER, 0 ISSUE in scope.

### Fixes applied
- None required (all findings NOTE-level, each mitigated/accepted as above).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | — | scoped self-review clean on first pass | — | — |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  (scoped self-review clean; formal shared-tree run pending coordination)
- [ ] All NOTEs recorded above (or explicitly "none")  (3 NOTEs recorded, all mitigated)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/ssh/client/client.go` (ServerSoftwareVersion/Banner) | yes | edited |
| `internal/component/ssh/ssh.go` (wish.WithVersion) | yes | edited |
| `internal/plugins/init/main.go` (daemonRunning + --seed) | yes | edited |
| `internal/plugins/init/daemon_internal_test.go` | yes | created |
| `internal/plugins/init/seed_internal_test.go` | yes | created |
| `internal/component/ssh/ssh_test.go` (TestServerAnnouncesZeBanner) | yes | edited |
| `cmd/ze/bootstrap_template_test.go` | yes | created |
| `mk/gokrazy.mk` (--seed, rm -f removed) | yes | edited |
| `docs/guide/appliance.md`, `docs/guide/configuration.md` | yes | edited |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | non-ze listener not treated as ze; real ze is | `go test ./internal/plugins/init -run DaemonRunning` PASS (5+3 cases) |
| AC-2 | template applied, not shadowed | `TestZeInitSeedSkipsActiveConfig` + `TestBootstrapConfigFromTemplateAppliesWebL2TP` PASS |
| AC-3 | full L2TP evidence green + workaround removed | workaround removed (mk/gokrazy.mk); evidence run NOT executed this session (needs root + kernel) — see Functional Tests |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze init --force` with host sshd on config'd port | `daemon_internal_test.go` | yes — returns false for OpenSSH banner |
| ze SSH server → `SSH-2.0-ze` banner → probe | `ssh_test.go` + `daemon_internal_test.go` | yes — server emits and probe requires the same constant |
| built template → effective boot config | `cmd/ze/bootstrap_template_test.go` | yes — real boot path preserves web+l2tp |
| gokrazy image built with template | `ze-deployment-gokrazy-l2tp-ppp-test` | NOT run this session (root/kernel) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | ze SSH banner distinguishes ze; `TestServerAnnouncesZeBanner` + daemonRunning suite |
| A-2 | confirmed (mechanism) | `TestBootstrapConfigFromTemplateAppliesWebL2TP`; full boot proof via the AC-3 run (pending a root+kernel host) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| appliance build runs `ze init --seed` | `docs/guide/appliance.md` build flow + seed-config section | `make ze-doc-test` PASS, anchors valid |
| `ze init --seed` skips discovery | `docs/guide/configuration.md` | `make ze-doc-test` PASS |
| all source anchors resolve | doc-test source-anchor pass | checked 1383 code paths, all valid |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written — `TestDaemonRunningIgnoresNonZeListener`
- [ ] Tests FAIL (paste output) — before the fix
- [ ] Tests PASS (paste output) — after the fix
- [ ] Boundary tests for all numeric inputs — N/A
- [ ] Functional tests for end-to-end behavior — `test/appliance/serial-login.ci` + L2TP evidence
- [ ] Interop tests for protocol features — N/A
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fixit-appliance-evidence-config.md`
- [ ] **Commit A:** init fix + build flow + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-appliance-evidence-config.md`
