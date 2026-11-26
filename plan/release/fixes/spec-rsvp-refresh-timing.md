# Spec: RSVP refresh timing: immediate forward on change, jitter, lifetime and slew (RFC 2205 Sections 2.1, 3.7)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Required correctness of shipped configuration, routing or forwarding behavior; existing partial implementations and remaining proof are retained. |
| RFC gaps | RFC2205-2-5, RFC2205-3-26, RFC2205-3-27, RFC2205-3-28 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Fix the three unmet RFC 2205 Section 3.7 MUST/MUST NOT timing defects in Ze's implemented RSVP-TE engine (`internal/plugins/rsvpte`) and deliver complete requirement-level behavioral proof for both those fixes and the already-implemented RFC 2205 Section 2.1 immediate-forwarding paths (`RFC2205-2-5`), per the owner's 2026-10-08 ruling that RFC2205-3-26, RFC2205-3-27, and RFC2205-3-28 are defects in implemented RSVP that this spec fixes under `ai/rules/rfc-compliance.md`:

1. **Immediate forward on state modification (`RFC2205-2-5`, MUST, RFC 2205 Section 2.1):** Already implemented in `engine.go::handlePathTransit`, `engine.go::handlePathEgress`, `reservation.go::acceptReservation`, `frr.go::mergeBackupPath`, and `reservation.go::handleResvErr`. Add positive and negative `RFC requirement: RFC2205-2-5` behavioral unit tests with discrimination records proving immediate propagation on PATH/RESV/RRO/merge changes and suppression on locally confirmed equal-or-smaller requests (`confirmHere`).
2. **Synchronization avoidance via jittered refresh timer (`RFC2205-3-26`, MUST; `RFC2205-3.7-1` / `RFC2205-x-2`, SHOULD, RFC 2205 Section 3.7 item 1):** Replace the fixed `time.Ticker` in `register.go::runRefreshLoop` with a reusable `time.Timer` whose wait interval before the first refresh and after every refresh tick is independently drawn uniformly at random from `[0.5R, 1.5R]` (`jitteredRefreshDelay`). `TIME_VALUES` continues to advertise the nominal running period `R`.
3. **Soft-state lifetime floor (`RFC2205-3-27`, MUST, RFC 2205 Section 3.7 item 2):** Keep `refresh-multiplier` as the integer loss tolerance `K` (`uint8`, `1..255`, default `3`), and compute every PSB, merged-branch, and RSB cleanup deadline in `fsm.go::expiredPSBs` and `fsm.go::receivedRSBExpired` through `stateLifetime(R, K) = ceil((2*K + 1) * 3 * R / 4)`, which equals `(K + 0.5) * 1.5 * R` (`5.25 * R` = `157.5s` at defaults `R = 30s`, `K = 3`).
4. **Refresh period increase slew limit (`RFC2205-3-28`, MUST NOT, RFC 2205 Section 3.7 item 5):** Limit dynamic increases of `R` in `register.go::adoptedRefreshPeriod` so successive values satisfy `R2 / R1 <= 1 + Slew.Max` with `Slew.Max = 0.30` (integer floor to whole milliseconds at `R1 * 13 / 10` for `R1 >= 10ms` so wire `TIME_VALUES` and loop cadence agree exactly), while decreases apply immediately. Hold the engine's running refresh period separately from the configured target (`engine.refreshPeriod` atomic `int64`, read through `engine.runningRefreshPeriod()`) and route every local `TIME_VALUES` advertisement (`refreshPaths`, `setupTunnel`, `setupBypass`, `armRerouteLocked`, `sendPathLocked`, `sendResvLocked`, `handlePathEgress`, `acceptReservation`) through it. Advance `engine.setRunningRefreshPeriod(next)` in `refreshTick` before `refreshPaths` so the advertised `R2` and the next cycle's cadence always match.

| Requirement | RFC text | Status at HEAD / Action in this spec |
|---|---|---|
| RFC2205-2-5 | "If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay." (§2.1) | Implemented in `handlePathTransit`, `handlePathEgress`, `acceptReservation`, `mergeBackupPath`, `handleResvErr`. Add positive + negative requirement-tagged tests and discrimination records, then close the `{gap}` row in `rfc/short/rfc2205.md` |
| RFC2205-3-26 | "Since RSVP sends periodic refresh messages, it must avoid message synchronization and ensure that any synchronization that may occur is not stable." (§3.7) | Defect fixed here: `register.go::runRefreshLoop` uses a fixed `time.Ticker`. Replace with per-cycle `[0.5R, 1.5R]` randomized `time.Timer` (`jitteredRefreshDelay`); prove with unit tests and freeRouter interop |
| RFC2205-3.7-1 / RFC2205-x-2 | "For this reason, the refresh timer should be randomly set to a value in the range [0.5R, 1.5R]." (§3.7) | Open SHOULD rows satisfied directly by `jitteredRefreshDelay` in `runRefreshLoop`; cover with positive + negative unit tests |
| RFC2205-3-27 | "To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer." (§3.7) | Defect fixed here: `fsm.go::expiredPSBs` and `receivedRSBExpired` use `K * R`. Replace with `stateLifetime(R, K) = (K + 0.5) * 1.5 * R`; update `freeRtrResvLifetime` and existing expiry tests; close open journal row `plan/journal/bound-too-small-for-its-own-burst.md` (2026-08-29) |
| RFC2205-3-28 | "Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max." (§3.7) | Defect fixed here: `register.go::adoptedRefreshPeriod` adopts an increased period in one step and advertisement sites read `e.cfg().RefreshPeriod`. Enforce `1.30` per-cycle slew cap, store running period on `engine`, and wire every wire-advertisement site; close open journal row `plan/journal/setting-changed-faster-than-its-consumer-allows.md` (2026-09-05) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, RFC precedence, plugin process boundary, goroutine lifecycle
  → Constraint: Protocol peers rely on the RFC first (`ai/rules/rfc-compliance.md`); a goroutine is a long-lived worker, never one per event.
- [ ] `ai/doors/routing-protocols.md` - routing protocol boundaries, RFC quote and discrimination obligations, freeRouter interop gate
  → Constraint: Quote the RFC above enforcing code; wire-visible changes owe a freeRouter interop proof under `test/interop-rsvpte/scenarios/`.
- [ ] `ai/doors/le-tooling.md` - interop lab conventions, no `internal/le` imports from product code
  → Constraint: Interop checkers live under `internal/le/interoplab/rsvpte/` and discover scenarios by directory name with no numeric prefix.
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te.md` - refresh loops, live config reload, sender-derived lifetimes, RSB expiry, and the two open Section 3.7 notes
  → Decision: `runRefreshLoop` and `runCleanupLoop` start once and never restart; received PSB/RSB lifetimes follow the sender's advertised `TIME_VALUES` period `R`, never this node's configured `refresh-period`.
  → Constraint: This spec departs from the prior fixed-ticker, `K * R` lifetime, and one-step increase adoption documented in `mpls-rsvp-te.md` and updates that page in the same change.
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` - PLR backup PATH refresh and merge-point independent branch expiry
  → Decision: At a merge point, incoming branches expire independently in `expiredPSBs`; PLR backup PATH and MP merged PATH advertise this node's local refresh period.
- [ ] `docs/architecture/testing/interop.md` - freeRouter RSVP-TE suite, scenario discovery, `transit-resv-state-expires`, and kernel capability scoping
  → Constraint: `transit-resv-state-expires` waits `timeout + freeRtrResvLifetime`, which must reflect `(3 + 0.5) * 1.5 * 120s` (630 s).

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2205.md` and `rfc/full/rfc2205.txt` (Sections 2.1, 3.7, A.4) - exact normative language for immediate propagation on state change, synchronization avoidance, `[0.5R, 1.5R]` jitter, `L >= (K + 0.5) * 1.5 * R`, and `R2 / R1 <= 1 + Slew.Max` (`Slew.Max = 0.30`)
  → Constraint: Default `K = 3`, default `R = 30s`, `Slew.Max = 0.30`; `TIME_VALUES` carries nominal `R` in milliseconds (32-bit unsigned integer).

**Key insights:**
- `refresh-multiplier` in `yang/ze-rsvp-te-conf.yang` is `uint8` (`1..255`, default `3`), matching RFC 2205 Section 3.7's integer `K`; `protectedStateLifetime` in `internal/le/interoplab/rsvpte/checkers.go:970` already computes `(3 + 0.5) * 1.5 * 2s` = `10.5s`.
- `TIME_VALUES` encodes milliseconds (`refreshMillis` in `build.go:94`), so flooring slewed increases to whole milliseconds at `R1 * 13 / 10` (for `R1 >= 10ms`) ensures the wire-advertised `R2` and the loop's running `R2` are identical and strictly obey `R2 / R1 <= 1.30`.
- Updating `stateLifetime` from `K * R` to `(K + 0.5) * 1.5 * R` lengthens `K = 3, R = 30s` expiry from 90 s to 157.5 s and affects four unit tests (`rfc2205_softstate_timeout_test.go`, `rfc3032_fsm_test.go`, `rfc4090_frr_test.go`) plus `freeRtrResvLifetime` in `internal/le/interoplab/rsvpte/checkers.go` (360 s to 630 s).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/rsvpte/register.go` - `runRefreshLoop` uses fixed `time.NewTicker(period)`; `refreshTick` calls `refreshPaths` then `adoptedRefreshPeriod` (which adopts any configured `1..65535s` period in one step); `refreshPaths`, `setupTunnel`, and `setupBypass` stamp `cfg.RefreshPeriod` directly. `cleanupTick` delegates expiry to `lspTable.expiredPSBs` and `lspTable.expiredRSBs`.
- [ ] `internal/plugins/rsvpte/fsm.go` - `expiredPSBs` computes deadlines as `LastRefresh + RefreshPeriod * factor` for both primary PSB and each merged branch in `MergedPaths`; `receivedRSBExpired` computes `LastRefresh + RSB.RefreshPeriod * factor`.
- [ ] `internal/plugins/rsvpte/engine.go` - `engine` holds `cfgPtr` (`atomic.Pointer[rsvpteConfig]`) with no separate running refresh period. `sendPathLocked` (merged PATH and backup PATH), `sendResvLocked` (ordinary and merged RESV), and `handlePathEgress` advertise `e.cfg().RefreshPeriod`. `handlePathTransit` and `handlePathEgress` already immediately transmit PATH/RESV on received updates, and `handlePathTransit` immediately sends RESV when RRO withdrawal clears `RSB.RRO`. `receivedRefreshPeriod` extracts the peer's `TIME_VALUES` (default 30 s, clamped to 65535 s).
- [ ] `internal/plugins/rsvpte/reservation.go` - `acceptReservation` advertises `e.cfg().RefreshPeriod` when building transit RESV and immediately calls `e.sendResv(lsp)` when `role == RoleTransit && !confirmHere`. `handleResvErr` immediately calls `e.sendResv(lsp)` for an unblocked InPlace error and sets `BlockadeUntil` using `3 * e.cfg().RefreshPeriod`.
- [ ] `internal/plugins/rsvpte/reroute.go` - `armRerouteLocked` sets `newLSP.PSB.RefreshPeriod = e.cfg().RefreshPeriod`.
- [ ] `internal/plugins/rsvpte/frr.go` - `mergeBackupPath` immediately sends `e.sendPath(lsp)` (for transit) and `e.sendResv(lsp)` when a backup branch joins a protected LSP; `backupPath` takes `period` from `sendPathLocked`.
- [ ] `internal/plugins/rsvpte/build.go` - `refreshMillis` encodes `time.Duration` to `TIME_VALUES` milliseconds; `buildPath` reads `psb.RefreshPeriod`; `buildResv` takes caller-supplied `refresh`.
- [ ] `internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang` - `refresh-period` is `uint16` `1..65535` default `30`; `refresh-multiplier` is `uint8` `1..255` default `3` with description claiming `refresh-period * multiplier` (90 s at defaults).
- [ ] `internal/le/interoplab/rsvpte/checkers.go` and `rsvpte.go` - `freeRtrResvLifetime` is `3 * freeRtrRefresh` (360 s) for `transit-resv-state-expires`; `protectedStateLifetime` is already `(3 + 0.5) * 1.5 * 2s` (10.5 s) for `plr-backup-path-to-egress-merge-point`.

**Behavior to preserve:**
- `runRefreshLoop` and `runCleanupLoop` start once in `OnStarted` and never restart on config reload.
- Decreasing `refresh-period` takes effect on the next refresh tick in one step (preserving `TestRefreshTickAdoptsReloadedPeriod` 30 s -> 10 s semantics).
- An unrelated commit that does not change `refresh-period` reports `changed == false` from `refreshTick` / `cleanupTick`.
- Transit relays of ordinary PATH messages preserve the upstream sender's `TIME_VALUES` period (`receivedRefreshPeriod`), never substituting this node's local period; egress and transit PSB/RSB lifetimes derive from the peer's `TIME_VALUES`.
- `handlePathTransit`, `handlePathEgress`, `acceptReservation`, `mergeBackupPath`, and `handleResvErr` immediate-forwarding behavior and `confirmHere` local-confirmation suppression stay intact.
- `refresh-period` (`uint16`, `1..65535`, default `30`) and `refresh-multiplier` (`uint8`, `1..255`, default `3`) YANG syntax, types, ranges, and defaults stay unchanged.

**Behavior to change:**
- `runRefreshLoop` replaces fixed `time.Ticker` with a per-cycle `[0.5R, 1.5R]` uniformly randomized `time.Timer`.
- `expiredPSBs` (primary PSB and merged branches) and `receivedRSBExpired` compute soft-state lifetime as `(K + 0.5) * 1.5 * R` (`stateLifetime`) instead of `K * R`.
- `adoptedRefreshPeriod` caps increases at `R2 / R1 <= 1.30` per refresh cycle; `engine` tracks its running refresh period separately from the configured target, `refreshTick` updates it before `refreshPaths`, and every local `TIME_VALUES` advertisement site reads `engine.runningRefreshPeriod()`.
- `freeRtrResvLifetime` in `internal/le/interoplab/rsvpte/checkers.go` and the affected expiry unit tests align to `(K + 0.5) * 1.5 * R`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Live-reloading refresh and cleanup loops | `internal/plugins/rsvpte/register.go` (`runRefreshLoop`, `refreshTick`, `runCleanupLoop`, `cleanupTick`, `liveConfig`, `adoptedRefreshPeriod`) | Runs one long-lived goroutine per loop, reading live config via `liveConfig` without restarting; `adoptedRefreshPeriod` validates bounds and detects changes | Extends: keeps the single long-lived loop and `liveConfig` reload seam, replaces `runRefreshLoop`'s fixed `time.Ticker` with a per-cycle `[0.5R, 1.5R]` `time.Timer`, and adds the `1.30` increase slew cap to `adoptedRefreshPeriod` |
| Engine atomic config store | `internal/plugins/rsvpte/engine.go` (`engine.cfgPtr`, `engine.cfg`, `engine.setConfig`) | Atomically publishes reloaded `rsvpteConfig` from `OnConfigApply` while preserving restart-class `RouterID` | Extends: adds an atomic `engine.refreshPeriod` holding the active slewed running period beside the configured target `cfgPtr`, read via `engine.runningRefreshPeriod()` at every local `TIME_VALUES` build site |
| Soft-state PSB/RSB expiry evaluators | `internal/plugins/rsvpte/fsm.go` (`lspTable.expiredPSBs`, `lspTable.expiredRSBs`, `receivedRSBExpired`) | Evaluates PSB, merged-branch, and ingress/transit RSB deadlines using each block's peer-derived `RefreshPeriod` and the live `RefreshMultiplier` | Extends: keeps the table/lock structure, independent merged-branch expiry, and sender-derived `RefreshPeriod`, replacing `RefreshPeriod * factor` with `stateLifetime(RefreshPeriod, factor)` = `(K + 0.5) * 1.5 * R` |
| Immediate state-change propagation | `internal/plugins/rsvpte/engine.go` (`handlePathTransit`, `handlePathEgress`), `reservation.go` (`acceptReservation`, `handleResvErr`), `frr.go` (`mergeBackupPath`) | Immediately builds and transmits updated PATH/RESV messages inside packet handling, including RRO withdrawal RESV and merged-branch updates, while suppressing upstream relay on `confirmHere` | Does not replace; retains and adds requirement-level `RFC2205-2-5` positive/negative proof and discrimination records |
| `TIME_VALUES` wire codec and millisecond helper | `internal/plugins/rsvpte/wire.go` (`encodeTimeValues`, `decodeTimeValues`), `build.go` (`refreshMillis`, `buildPath`, `buildResv`), `engine.go` (`receivedRefreshPeriod`) | Encodes/decodes Class-Num 5 C-Type 1 milliseconds and clamps received values to `maxRefreshPeriod` (65535 s) with 30 s fallback on zero | Reuses unchanged; no wire format or parser layout changes |
| FreeRouter RSVP-TE interop lab | `internal/le/interoplab/rsvpte/checkers.go`, `rsvpte.go`, `test/interop-rsvpte/scenarios/` | Runs multi-container freeRouter + Ze RSVP-TE scenarios with tcpdump capture assertions; `protectedStateLifetime` already uses `(3 + 0.5) * 1.5 * R` | Extends: updates `freeRtrResvLifetime` and `checkResvStateExpires` to prove the `(K + 0.5) * 1.5 * R` floor against freeRouter, and adds `ingress-refresh-jitter-and-slew` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order" | agrees | Fixes three unmet RFC 2205 Section 3.7 MUST/MUST NOT requirements and proves RFC 2205 Section 2.1 immediate propagation without altering any `pkg/` contract or breaking forked-plugin operation |
| `ai/doors/routing-protocols.md` | RFC and interop obligations | the door covering `internal/plugins/rsvpte/*.go` | "A wire-visible change owes an interop scenario." | agrees | Quotes RFC 2205 Sections 2.1 and 3.7 above every enforcing function, adds discrimination records for every new RFC tag, updates `transit-resv-state-expires` to prove the lifetime floor, and adds `ingress-refresh-jitter-and-slew` against freeRouter |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/interoplab/rsvpte/*` and `test/interop-rsvpte/*` | "Never import `internal/le` from product code" | agrees | Product timing logic lives solely in `internal/plugins/rsvpte/`; interop checkers in `internal/le/interoplab/rsvpte/` stay cleanly separated and discover the new scenario by directory name with no numeric prefix |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Decision: the refresh loops read the live config, they do not restart | `// Design:` of `register.go`, `fsm.go`, `engine.go`, `reservation.go`, `reroute.go`; modified by this spec | "That message carries the new period, and the neighbor recomputes its lifetime from it." | departs | Departs from fixed `time.Ticker` and one-step adoption of an increased `refresh-period` (per 2026-10-08 owner decision): `runRefreshLoop` now draws each wait uniformly in `[0.5R, 1.5R]` on a `time.Timer`, `adoptedRefreshPeriod` caps increases at `1.30x` per cycle, and `refreshTick` advances `engine.runningRefreshPeriod()` before `refreshPaths` so advertised `TIME_VALUES` and cadence move together. The page is updated in the same change |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Decision: received reservation state expires on its own deadline | `// Design:` of `register.go`, `fsm.go`, `engine.go`, `reservation.go`; modified by this spec | "Two RFC 2205 Section 3.7 timing rules are still open. The cleanup timeout is K*R" | departs | Closes both open RFC 2205 Section 3.7 timing defects named in that paragraph (per 2026-10-08 owner decision): soft-state expiry in `expiredPSBs` and `receivedRSBExpired` now enforces `L = (K + 0.5) * 1.5 * R`, and refresh-period increases slew at `1.30x` per cycle. The page is updated in the same change |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Decision: the lifetime of received state follows the sender's period | `// Design:` of `engine.go`, `register.go`; modified by this spec | "An ordinary transit relay keeps the received period downstream." | agrees | Preserves sender-derived `PSB.RefreshPeriod` / `RSB.RefreshPeriod` (`receivedRefreshPeriod`) for all received state and ordinary transit PATH relays; local `engine.runningRefreshPeriod()` governs only locally originated/refreshed messages |
| `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` | Decision: local repair moves data and control traffic | anchored to `register.go`, `engine.go`, `fsm.go`, `reservation.go`, `reroute.go`, `// Design:` of `producer_integration_linux_test.go` | "Branches expire independently." | agrees | Each merged branch in `expiredPSBs` continues to expire independently using its own `branch.RefreshPeriod`, now evaluated with `stateLifetime(branch.RefreshPeriod, factor)`, and PLR/MP locally originated refreshes advertise `engine.runningRefreshPeriod()` |
| `docs/guide/rsvp-te.md` | Configuration | `// Design:` of `internal/plugins/rsvpte/freertr_interop_integration_linux_test.go`; modified by this spec | "multiplied by this number, which is 90 seconds at the defaults." | departs | Departs from the `K * R` (90 s) default expiry description and one-tick increase description in `What a commit changes`: default expiry becomes `(3 + 0.5) * 1.5 * 30s` (157.5 s), refreshes are jittered in `[0.5R, 1.5R]`, and `refresh-period` increases slew at `<= 1.30x` per cycle; `docs/guide/rsvp-te.md` is updated in the same change |
| `docs/architecture/testing/interop.md` | Design decisions | `// Design:` of `internal/le/interoplab/rsvpte/checkers.go`, `rsvpte.go`; modified by this spec | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Leaves the RSVP-TE suite's `Suite.KernelCapabilities` (`mpls`, `mpls-transit-mtu`) untouched; the new `ingress-refresh-jitter-and-slew` scenario requires no additional kernel capabilities |
| `docs/architecture/testing/interop.md` | The freeRouter RSVP-TE suite | `// Design:` of `internal/le/interoplab/rsvpte/checkers.go`, `rsvpte.go`; modified by this spec | "The freeRouter egress advertises its fixed 120-second refresh period in the RESV's TIME_VALUES, so Ze, with refresh-multiplier 3, times the reservation out about six minutes after the freeze, and the checker waits that long" | departs | Updating Ze's expiry lifetime from `K * R` to `(K + 0.5) * 1.5 * R` changes `transit-resv-state-expires` expiry from ~6 minutes (360 s) to ~10.5 minutes (630 s); this spec updates `freeRtrResvLifetime`, adds a negative assertion past 360 s, and updates the table row in `interop.md` in the same change |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Configuration commit / startup:** `rsvp-te { refresh-period <1..65535>; refresh-multiplier <1..255>; }` enters `parseConfig` (`register.go`), is committed via `OnConfigure` / `OnConfigApply`, and stored into `engine.cfgPtr` via `engine.setConfig(cfg)`.
- **Periodic refresh timer:** `runRefreshLoop` (`register.go`) wakes on a reusable `time.Timer` whose next wait is drawn by `jitteredRefreshDelay(period, nil)` in `[0.5R, 1.5R]`.
- **Periodic cleanup timer:** `runCleanupLoop` (`register.go`) wakes on its ticker and evaluates expired PSBs and RSBs against `liveConfig(cfg, eng).RefreshMultiplier`.
- **Incoming RSVP wire packet (IP protocol 46):** `rawTransport.readLoop` -> `engine.run` -> `engine.handlePacket` -> `DecodeMessage` -> `handlePath` / `handleResv` / `handleResvErr`.

### Transformation Path
1. **Engine initialization:** `newEngine` stores `cfg` into `cfgPtr` and initializes `engine.refreshPeriod` (`atomic.Int64`) to `cfg.RefreshPeriod` (default `DefaultRefreshPeriod` = 30 s).
2. **Config reload (`OnConfigApply`):** `engine.setConfig(cfg)` atomically updates `cfgPtr` with the new configured target `RefreshPeriod` and `RefreshMultiplier`, leaving `engine.refreshPeriod` untouched so increases never jump immediately on the wire.
3. **Refresh cycle (`runRefreshLoop` -> `refreshTick`):**
   - `refreshTick` reads target `liveConfig(cfg, eng).RefreshPeriod` and calls `adoptedRefreshPeriod(period, target)`.
   - If `target <= 0 || target > maxRefreshPeriod` or `target == period`: returns `period, false`.
   - If `target < period` (decrease): adopts `target` in one step and returns `target, true`.
   - If `target > period` (increase): computes `maxNext` capped at `1.30 * period` (floored to whole milliseconds via integer arithmetic `(period/time.Millisecond * 13 / 10) * time.Millisecond` when `period >= 10ms`, or `period * 13 / 10` below 10 ms), returns `min(target, maxNext), true`.
   - When `eng != nil`, `refreshTick` calls `eng.setRunningRefreshPeriod(next)` before invoking `refreshPaths(log, lspTable, eng)`.
   - `refreshPaths` stamps `lsp.PSB.RefreshPeriod = eng.runningRefreshPeriod()` on each ingress LSP and calls `eng.sendPath` / `eng.sendResv`.
   - `runRefreshLoop` updates its local `period = next`, logs `"rsvp-te: refresh period reloaded"` when `changed`, and resets the timer to `jitteredRefreshDelay(period, nil)` in `[0.5 * period, 1.5 * period]`.
4. **Local `TIME_VALUES` advertisement:** `setupTunnel`, `setupBypass`, `armRerouteLocked`, `sendPathLocked` (merged and repaired backup PATH), `sendResvLocked`, `handlePathEgress`, and `acceptReservation` all read `eng.runningRefreshPeriod()` (or `cfg.RefreshPeriod` only when `eng == nil`) and pass it to `buildPath` / `buildResv` -> `encodeTimeValues` -> `refreshMillis`.
5. **Cleanup cycle (`runCleanupLoop` -> `cleanupTick`):**
   - `expiredPSBs(now, live.RefreshMultiplier)` evaluates `deadline = LastRefresh + stateLifetime(RefreshPeriod, factor)` for each PSB and each branch in `MergedPaths`, where `stateLifetime(R, K) = ceil((2*K + 1) * 3 * R / 4) = (K + 0.5) * 1.5 * R`.
   - `expiredRSBs(now, live.RefreshMultiplier)` -> `receivedRSBExpired` evaluates `now.After(RSB.LastRefresh.Add(stateLifetime(RSB.RefreshPeriod, factor)))`.
   - Expired PSBs are torn locally (`tearLSPLocal`); expired ingress/transit RSBs are withdrawn (`removeReservation`, sending `ResvTear` upstream on transit).
6. **Immediate propagation on state change (`RFC2205-2-5`):**
   - `handlePathTransit` updates `PSB`, immediately builds and transmits relayed PATH (or repaired backup PATH), and immediately calls `sendResv` if RRO withdrawal cleared a cached `RSB.RRO`.
   - `handlePathEgress` updates `PSB`/`RSB` and immediately transmits RESV upstream.
   - `acceptReservation` updates `RSB` and, for transit when `!confirmHere`, immediately calls `sendResv`; when `confirmHere` holds, sends `ResvConf` locally without relaying RESV upstream.
   - `mergeBackupPath` updates `MergedPaths`/merged ADSPEC and immediately transmits merged PATH (transit) and RESV.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config reload (`OnConfigApply`) -> Running engine | `engine.setConfig` stores target config in `cfgPtr`; `refreshTick` advances `engine.refreshPeriod` at `<= 1.30x` per cycle | No |
| Engine -> RSVP neighbor (IP protocol 46) | PATH and RESV messages carrying `TIME_VALUES` (Class-Num 5, C-Type 1, nominal running `R` in ms) sent on `[0.5R, 1.5R]` jittered timer or immediately on state change | No |
| RSVP neighbor -> Engine (IP protocol 46) | Received PATH/RESV `TIME_VALUES` decoded by `decodeTimeValues` -> `receivedRefreshPeriod`, setting stored state lifetime `(K + 0.5) * 1.5 * R` | No |
| Engine -> MPLS FIB bus (`mpls-fib:entry`) | Expiry triggers `tearLSPLocal` / `removeReservation`, withdrawing swap/pop/push entries only after `(K + 0.5) * 1.5 * R` | No |

### Changed-Wire Census

The binary layout of the `TIME_VALUES` object (Class-Num 5, C-Type 1, 8 octets total: 4-octet object header + 4-octet big-endian millisecond `RefreshPeriod`, RFC 2205 Section A.4) does **not** change, nor do any RPC, CLI, or JSON event schemas. What changes on the wire is the **sequence of `TIME_VALUES.RefreshPeriod` values emitted during a dynamic increase** (stepped at `R2 / R1 <= 1.30` instead of jumping to the target in one message), the **inter-message refresh timing** (randomized uniformly in `[0.5R, 1.5R]` instead of fixed `R`), and the **expiry time** at which Ze transmits a timeout-triggered `ResvTear` (`(K + 0.5) * 1.5 * R` instead of `K * R`).

**Search evidence:** Scanned `internal/`, `pkg/`, `cmd/`, and `test/` with `grep -rn "encodeTimeValues\|decodeTimeValues\|ClassTimeValues\|TimeValues"` (and `RefreshPeriod\|expiredPSBs\|expiredRSBs` across `internal/plugins/rsvpte/` and `internal/le/interoplab/rsvpte/`). Positive controls: `build.go:110` (`buildPath`) and `build.go:174` (`buildResv`) are the only two production `encodeTimeValues` writers; `wire.go:1016` (`DecodeMessage` -> `decodeTimeValues`) -> `engine.go:561` (`receivedRefreshPeriod`) is the only production decoder/reader in Ze. Every caller of `buildPath` and `buildResv` plus every external RSVP peer reader is enumerated below.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `RSVP TIME_VALUES (RFC 2205 Section A.4, Class-Num 5, C-Type 1)` | writer | `internal/plugins/rsvpte/build.go::buildPath` via `register.go::refreshPaths`, `setupTunnel`, `setupBypass`, `reroute.go::armRerouteLocked`, `engine.go::sendPathLocked` (merged & repaired backup PATH) | `build.go:110` calls `encodeTimeValues(..., refreshMillis(psb.RefreshPeriod))`; all five local PATH origins currently stamp `cfg.RefreshPeriod` / `e.cfg().RefreshPeriod` | Change: read `eng.runningRefreshPeriod()` at all five local PATH origins so emitted `R` slews at `<= 1.30x` per cycle; keep ordinary transit relay (`handlePathTransit`) forwarding the upstream sender's `psb.RefreshPeriod` unchanged |
| `RSVP TIME_VALUES (RFC 2205 Section A.4, Class-Num 5, C-Type 1)` | writer | `internal/plugins/rsvpte/build.go::buildResv` via `engine.go::sendResvLocked`, `engine.go::handlePathEgress`, `reservation.go::acceptReservation` | `build.go:174` calls `encodeTimeValues(..., refreshMillis(refresh))`; all three RESV callers currently pass `e.cfg().RefreshPeriod` | Change: pass `e.runningRefreshPeriod()` at all three RESV callers so advertised `R` agrees with the running slewed refresh cadence |
| `RSVP TIME_VALUES (RFC 2205 Section A.4, Class-Num 5, C-Type 1)` | reader | `internal/plugins/rsvpte/wire.go::DecodeMessage` -> `decodeTimeValues` -> `engine.go::receivedRefreshPeriod` -> `fsm.go::expiredPSBs`, `fsm.go::receivedRSBExpired` | `wire.go:1016-1022` decodes milliseconds into `msg.TimeValues`; `engine.go:561-570` clamps to `maxRefreshPeriod` (65535 s, default 30 s); `fsm.go:365,370,419` currently multiply by `factor` | Change: preserve `decodeTimeValues` and `receivedRefreshPeriod` unchanged; update `fsm.go` expiry calculation to `stateLifetime(RefreshPeriod, factor)` = `(K + 0.5) * 1.5 * R` |
| `RSVP TIME_VALUES (RFC 2205 Section A.4, Class-Num 5, C-Type 1)` | reader | External RSVP peers (e.g. freeRouter `packRsvp.parseTimeVal` / `rtrRsvpIface`, verified in `internal/le/interoplab/rsvpte/checkers.go`) | `checkers.go:49-54` reads peer `TIME_VALUES` expectations (`freeRtrResvLifetime`); `freertr_interop_integration_linux_test.go:103` and `producer_integration_linux_test.go:358` decode `msg.TimeValues.RefreshPeriod` | Preserve wire format; update `freeRtrResvLifetime` and both integration lifetime helpers to `stateLifetime`, and verify freeRouter accepts Ze's jittered and slewed `TIME_VALUES` in `ingress-refresh-jitter-and-slew` |

### Integration Points
- `internal/plugins/rsvpte/engine.go::newEngine` - initializes `engine.refreshPeriod` from `cfg.RefreshPeriod` (falling back to `DefaultRefreshPeriod`).
- `internal/plugins/rsvpte/register.go::runRefreshLoop` - owns the single reusable `time.Timer` reset to `jitteredRefreshDelay(period, nil)` after every tick.
- `internal/plugins/rsvpte/register.go::refreshTick` - computes `next, changed := adoptedRefreshPeriod(period, liveConfig(cfg, eng).RefreshPeriod)`, stores `eng.setRunningRefreshPeriod(next)` when `eng != nil`, then invokes `refreshPaths`.
- `internal/plugins/rsvpte/fsm.go::stateLifetime` - single pure lifetime helper shared by `expiredPSBs` (primary PSB + merged branches), `receivedRSBExpired`, and integration/unit test assertions.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config flows `parseConfig` -> `OnConfigApply` -> `engine.setConfig` -> `refreshTick`/`adoptedRefreshPeriod` -> `engine.setRunningRefreshPeriod` -> `buildPath`/`buildResv`; expiry flows `cleanupTick` -> `expiredPSBs`/`expiredRSBs` -> `stateLifetime` -> `tearLSPLocal`/`removeReservation` |
| No unintended coupling (components stay isolated) | Yes | All protocol logic stays inside `internal/plugins/rsvpte`; no `internal/le` import in product code; no direct netlink or kernel route writes |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `runRefreshLoop`, `adoptedRefreshPeriod`, `expiredPSBs`, `receivedRSBExpired`, and `engine`; single `stateLifetime` helper serves all PSB, merged-branch, and RSB expiry checks |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No extra packet buffer copies introduced; `engine.refreshPeriod` is a single lock-free `atomic.Int64` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new plugin, command, family, or core switch added; existing `rsvp-te` plugin registration is unchanged |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No new config leaves or CLI verbs introduced; searched `internal/le/interoplab/rsvpte/rsvpte.go` (`scenarioCheckerMap`) and `internal/le/interoplab/discover.go` (interop scenarios derive from directory discovery paired with `scenarioCheckerMap`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `refresh-multiplier` continues to represent RFC 2205 integer `K` (`1..255`, default `3`), and `L = (K + 0.5) * 1.5 * R` is derived from it without changing YANG schema types or defaults | `yang/ze-rsvp-te-conf.yang:36-46` (`uint8`, `1..255`, default `3`), RFC 2205 Section 3.7 item 2 ("where K is a small integer. Currently K = 3 is suggested as the default"), and `checkers.go:970` (`protectedStateLifetime`) | Changing YANG type or default would break existing configs and CLI schema contracts | Unit tests `TestRFC2205StateLifetimeFloor` + existing `test/parse/rsvpte-config-valid.ci` + `test/reload/rsvpte-reload.ci` | unvalidated |
| A-2 | Existing freeRouter interop scenarios tolerate `[0.5R, 1.5R]` jitter on Ze's 2 s refresh period (`[1s, 3s]`) and `transit-resv-state-expires` passes with `freeRtrResvLifetime = 630s` | `test/interop-rsvpte/scenarios/*/ingress.conf` (`refresh-period 2`), `checkers.go:49-55` (`freeRtrRefresh = 120s`), `checkers.go:970` (`protectedStateLifetime = 10.5s`) | A tight wait bound in an interop checker could flake if jitter pushes a refresh to `1.5R` | `./le test integration interop-rsvpte` (`transit-resv-state-expires`, `plr-backup-path-to-egress-merge-point`, `ingress-refresh-jitter-and-slew`) | unvalidated |
| A-3 | `tcpdump -vvv` in the freeRouter container prints `Time Values Object (5)` with `Refresh Period` in milliseconds and packet timestamps | `internal/le/interoplab/rsvpte/zenode.go:157` (`tcpdump -i eth0 -nn -l -vvv`) and `checkers.go` (`parseCapture`) | Parser in `checkIngressRefreshJitterAndSlew` would fail to extract `TIME_VALUES` | Unit test over real `tcpdump -vvv` sample in `internal/le/interoplab/rsvpte/rsvpte_test.go` + live `./le test integration interop-rsvpte` run | unvalidated |
| A-4 | Integer millisecond flooring of `R1 * 13 / 10` (for `R1 >= 10ms`) strictly converges from any valid `R1` to any larger `target <= 65535s` with `R2 / R1 <= 1.30` at every step | Arithmetic: minimum YANG `R1` is `1000ms`, giving step `+300ms` (`1300ms`); max `65535000ms * 13` fits `int64` without overflow | Rounding up could violate `R2 / R1 <= 1.30`; rounding down too aggressively could stall | Boundary unit test `TestRFC2205RefreshSlewBoundsAndConvergence` over `1s -> 65535s` and `10s -> 300s` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Existing unit tests that aged PSBs/RSBs by `(K + 1) * R` (e.g. `4 * 30s = 120s < 157.5s`) fail once `stateLifetime` enforces `(K + 0.5) * 1.5 * R` | Failures in `rfc2205_softstate_timeout_test.go`, `rfc3032_fsm_test.go`, `rfc4090_frr_test.go` | Update those four test files in the same phase so their expired fixtures age past `stateLifetime(R, K)` while preserving what each test proves (re-taking any staled RFC discrimination records via `./le rfc record`) |
| R-2 | `transit-resv-state-expires` waits 10.5 min (up from 6 min) and could hit an outer suite deadline | Timeout in `./le test integration interop-rsvpte` on `transit-resv-state-expires` | Update `freeRtrResvLifetime = time.Duration((3 + 0.5) * 1.5 * float64(freeRtrRefresh))` in `checkers.go` so `waitMessage` receives `timeout + 630s` and confirm the scenario completes cleanly inside the interop job budget |
| R-3 | A local `TIME_VALUES` advertisement site still reads `e.cfg().RefreshPeriod` instead of `e.runningRefreshPeriod()`, causing wire `TIME_VALUES` to jump straight to target during a slew | `TestRFC2205RefreshSlewAllAdvertisersAgree` fails or `ingress-refresh-jitter-and-slew` observes target `R` on first tick | Enumerated every site in Changed-Wire Census (`refreshPaths`, `setupTunnel`, `setupBypass`, `armRerouteLocked`, `sendPathLocked`, `sendResvLocked`, `handlePathEgress`, `acceptReservation`) and test all of them directly during an active slew |
| R-4 | A deterministic jitter test could flake if run against wall-clock `time.Timer` under host load | Flaky timing assertion in unit tests | Inject a deterministic PRNG/clock seam into `jitteredRefreshDelay` and `runRefreshLoopWith` for unit tests; use wide tolerance + multi-sample spread check in the live freeRouter interop scenario |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Under-sized lifetime drops live peer LSPs under jitter; unslewed increase drops peer LSPs when one refresh packet is lost; mis-implemented jitter could stall `runRefreshLoop` or stop refreshing LSPs |
| How is it reverted? | Single commit revert; no persisted state or config migration; YANG leaves and defaults are unchanged |
| Who else touches this path? | `internal/plugins/rsvpte` (no other component imports it); `internal/le/interoplab/rsvpte` checkers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `runRefreshLoop` goroutine startup and tick loop | → | `jitteredRefreshDelay` + `refreshTick` -> `adoptedRefreshPeriod` -> `engine.setRunningRefreshPeriod` -> `refreshPaths` -> `buildPath`/`buildResv` | `TestRFC2205RefreshLoopJittersAndSlewsEndToEnd` (`internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`) |
| `cleanupTick` -> `lspTable.expiredPSBs` / `lspTable.expiredRSBs` | → | `stateLifetime` -> `engine.tearLSPLocal` / `engine.removeReservation` | `TestRFC2205CleanupTickEnforcesLifetimeFloor` (`internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`) |
| `engine.handlePacket` (changed PATH, changed RESV, RRO withdrawal, merged backup PATH) | → | `handlePathTransit`, `handlePathEgress`, `acceptReservation`, `mergeBackupPath` | `TestRFC2205ImmediateForwardOnStateChange` (`internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`) |
| Live Ze daemon `ze cli` reload (`set rsvp-te refresh-period 10; commit`) -> `OnConfigApply` -> `engine.setConfig` -> `runRefreshLoop` | → | Wire PATH refreshes captured at freeRouter relay | `ingress-refresh-jitter-and-slew` (`internal/le/interoplab/rsvpte/checkers.go::checkIngressRefreshJitterAndSlew`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A transit or egress engine receives a PATH whose forwarded state changes (updated `SENDER_TSPEC`, updated `ADSPEC` MTU, added/withdrawn `RRO`, updated opaque `ForwardObjects`, or merged backup branch), or a transit engine receives a RESV with updated `FLOWSPEC`/label/`RRO` (`!confirmHere`), via `engine.handlePacket` | The engine updates local state and immediately transmits the updated PATH downstream and/or updated RESV upstream within the same `handlePacket` call, carrying the new objects/values before any refresh timer fires (`RFC2205-2-5`) | Locally confirmed equal-or-smaller RESV with `RESV_CONFIRM` (`confirmHere == true`) answers `ResvConf` to the requester without relaying RESV upstream, as required by RFC 2205 Sections 2.1 and 2.6 |
| AC-2 | `runRefreshLoop` starts and executes repeated refresh cycles at nominal period `R` | Each inter-refresh delay drawn by `jitteredRefreshDelay` falls within `[0.5R, 1.5R]` (`RFC2205-3.7-1`, `RFC2205-x-2`), successive cycles draw independent randomized delays whose intervals vary across cycles so two loops started together diverge rather than staying synchronized (`RFC2205-3-26`), and every emitted PATH/RESV still advertises nominal `R` in `TIME_VALUES` | Two independent draws may coincide on a single tick by chance under uniform sampling; stability of synchronization is prevented by re-drawing every cycle |
| AC-3 | A PSB, merged backup branch, or ingress/transit RSB was last refreshed at `t0` with peer-advertised period `R` and live `refresh-multiplier` `K` | At `t0 + stateLifetime(R, K)` (where `stateLifetime(R, K) = ceil((2*K + 1) * 3 * R / 4) >= (K + 0.5) * 1.5 * R`, e.g. `157.5s` for `R = 30s, K = 3`, strictly above `K * R = 90s`), `cleanupTick` retains the state and forwarding entry; strictly after `t0 + stateLifetime(R, K)`, `cleanupTick` expires the PSB (calling `tearLSPLocal`) or RSB (calling `removeReservation` and sending `ResvTear` on transit) (`RFC2205-3-27`) | Egress RSBs expire with their owning PSB rather than via `expiredRSBs`, and merged branches expire individually so a surviving branch keeps the downstream PSB alive until its own `(K + 0.5) * 1.5 * R` passes |
| AC-4 | An operator commits a larger `refresh-period` (e.g. `10s -> 300s`, or `1s -> 65535s`) through `engine.setConfig` | Each `refreshTick` increases the running refresh period by at most `1.30x` (`R2 / R1 <= 1 + Slew.Max`, `Slew.Max = 0.30`) until the running period reaches the configured target (`RFC2205-3-28`), and every local `TIME_VALUES` writer (`refreshPaths`, `setupTunnel`, `setupBypass`, `armRerouteLocked`, `sendPathLocked`, `sendResvLocked`, `handlePathEgress`, `acceptReservation`) advertises the exact stepped running period `R2` matching the next cycle's cadence | Ordinary transit PATH relays (`handlePathTransit`) continue to forward the upstream sender's received `TIME_VALUES` period rather than this node's local running period (RFC 2205 Section 3.7 item 3) |
| AC-5 | An operator commits a smaller `refresh-period` (e.g. `30s -> 10s`), an unchanged `refresh-period`, or an out-of-range period (`<= 0` or `> 65535s`) | A decrease is adopted in one step on the next `refreshTick` (`R2 = target`, `changed = true`) with matching wire `TIME_VALUES`; an unchanged or out-of-range value keeps the current running period (`changed = false`) | None: every valid decrease applies immediately and every invalid or unchanged input preserves the running period |
| AC-6 | FreeRouter interop scenario `transit-resv-state-expires` freezes the freeRouter egress (`R = 120s`) behind a Ze transit (`refresh-multiplier 3`) | Ze transit retains its MPLS swap and sends no `ResvTear` past the old `3 * 120s` (360 s) mark, then sends `ResvTear` to the freeRouter ingress and withdraws its MPLS swap after `stateLifetime(120s, 3)` = 630 s | None: early teardown before 630 s or failure to tear down after 630 s fails the check |
| AC-7 | FreeRouter interop scenario `ingress-refresh-jitter-and-slew` runs Ze ingress (`refresh-period 2`), freeRouter relay, Ze transit, freeRouter egress, then commits `refresh-period 10` on Ze ingress via `ze cli` | Captured PATH refreshes at freeRouter relay show randomized inter-refresh spacing within `[0.5R, 1.5R]` (plus scheduling slack, with non-zero spread across >= 6 refreshes), followed after reload by monotonically increasing `TIME_VALUES` values from `2000ms` to `10000ms` where every consecutive distinct pair obeys `R2 / R1 <= 1.30`, while the LSP remains Up | OS scheduling latency may add bounded positive slack above `1.5R` on a loaded host; the assertion allows positive scheduling headroom while refusing sub-`0.5R` bursts and fixed-period zero-spread spacing |
| AC-8 | RFC conformance ledger (`rfc/short/rfc2205.md`, `docs/features/rfc-status.md`, `rfc/discrimination/rfc2205.json`) and open journal rows | `RFC2205-2-5`, `RFC2205-3-26`, `RFC2205-3-27`, `RFC2205-3-28`, `RFC2205-3.7-1`, and `RFC2205-x-2` have positive + negative tagged tests with fresh discrimination records (`./le rfc check` clean), `{gap}` annotations are removed from `rfc/short/rfc2205.md`, and both owned open journal rows are marked `fixed:` | None: every requirement row claimed by this spec must pass `./le rfc check` with recorded red-then-green discrimination |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a Ze RSVP-TE head-end and raises `rsvp-te { refresh-period 10; }` from `2` via `commit` while an LSP is up through freeRouter | CLI `commit` -> `OnConfigApply` -> `engine.setConfig` -> `runRefreshLoop` -> `refreshTick` -> `adoptedRefreshPeriod` (1.30x slew) -> `engine.setRunningRefreshPeriod` -> `refreshPaths` -> `buildPath` -> raw socket -> freeRouter relay capture | `ingress-refresh-jitter-and-slew` (`checkIngressRefreshJitterAndSlew`) and `TestRFC2205RefreshLoopJittersAndSlewsEndToEnd` |
| 2 | Runs a Ze RSVP-TE transit node peering with a jittering neighbor (e.g. freeRouter with `R = 120s`, `K = 3`) | Peer RESV -> `DecodeMessage` -> `receivedRefreshPeriod` -> `RSB` -> `runCleanupLoop` -> `cleanupTick` -> `expiredRSBs` -> `stateLifetime` (`630s`) -> `removeReservation` -> `ResvTear` + FIB swap withdrawal | `transit-resv-state-expires` (`checkResvStateExpires`) and `TestRFC2205CleanupTickEnforcesLifetimeFloor` |
| 3 | Changes LSP bandwidth, path MTU, or RRO request upstream on a live LSP | Incoming PATH/RESV -> `engine.handlePacket` -> `handlePathTransit` / `handlePathEgress` / `acceptReservation` -> immediate `SendPath` / `sendResv` without waiting for `runRefreshLoop` | `TestRFC2205ImmediateForwardOnStateChange` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC2205ImmediateForwardOnStateChange` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-2-5` positive: transit PATH modification (`SENDER_TSPEC`, `ADSPEC`, `ForwardObjects`), transit RRO withdrawal -> immediate RESV, egress PATH update -> immediate RESV, transit RESV update -> immediate upstream RESV, merged backup PATH -> immediate merged PATH/RESV | |
| `TestRFC2205LocalConfirmDoesNotForwardImmediateResv` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-2-5` negative: equal-or-smaller RESV with `RESV_CONFIRM` (`confirmHere`) answers `ResvConf` locally without emitting an immediate upstream RESV | |
| `TestRFC2205RefreshDelayJitterBoundsAndDesync` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-3-26` positive, `RFC2205-3.7-1` positive, `RFC2205-x-2` positive: `jitteredRefreshDelay` stays within `[0.5R, 1.5R]` across samples and endpoints, produces distinct non-constant intervals, and two loops started together diverge | |
| `TestRFC2205RefreshDelayNeverEscapesJitterWindow` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-3-26` negative, `RFC2205-3.7-1` negative, `RFC2205-x-2` negative: boundary/extreme RNG outputs and zero/non-positive period inputs never yield delays `< 0.5R` or `> 1.5R`, and `TIME_VALUES` still advertises nominal `R` rather than the jittered wait | |
| `TestRFC2205StateLifetimeFloor` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-3-27` positive + negative: `stateLifetime(R, K)` satisfies `L >= (K + 0.5) * 1.5 * R` for `K = 1, 2, 3, 10, 255` and `R = 1s, 30s, 65535s` (no overflow); PSB, merged branch, and RSB are NOT expired at `K * R` or at exact `(K + 0.5) * 1.5 * R`, and ARE expired strictly after `(K + 0.5) * 1.5 * R` | |
| `TestRFC2205CleanupTickEnforcesLifetimeFloor` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | Wiring test + `RFC2205-3-27`: `cleanupTick` retains PSB/RSB at `K * R` (90 s for `R = 30s, K = 3`) and at `157.5s`, then tears down PSB and withdraws transit RSB with `ResvTear` after `157.5s` | |
| `TestRFC2205RefreshSlewLimitsIncreaseRatio` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-3-28` positive + negative: raising `refresh-period` (`10s -> 300s`, `1s -> 65535s`) slews through `adoptedRefreshPeriod` and `refreshTick` so EVERY consecutive pair obeys `R2 / R1 <= 1.30` in both `time.Duration` and wire `TIME_VALUES` milliseconds, reaches target cleanly, never jumps past `1.30 * R1` in one tick, while decreasing `30s -> 10s` applies in one step | |
| `TestRFC2205RefreshSlewAllAdvertisersAgree` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | `RFC2205-3-28`: during an active slew (`running = 13s`, `target = 300s`), `refreshPaths`, `setupTunnel`, `setupBypass`, `reroute`, `sendPath` (merged & repaired backup), `sendResv`, `handlePathEgress`, and `acceptReservation` all emit `TIME_VALUES = 13000ms` (never `300000ms`), while ordinary transit PATH relay keeps the upstream sender's period | |
| `TestRFC2205RefreshLoopJittersAndSlewsEndToEnd` | `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` | Wiring test: runs the real `runRefreshLoop` goroutine seam with an injected delay observer across startup and reload (`10s -> 300s`), verifying jittered waits in `[0.5R, 1.5R]` and slewed `TIME_VALUES` on wire messages | |
| `TestParseTimeValuesFromTcpdump` | `internal/le/interoplab/rsvpte/rsvpte_test.go` | Unit test for the interop checker's tcpdump timestamp and `TIME_VALUES` millisecond parser and jitter/slew verifier | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `refresh-period` (`R`) | `1s..65535s` | `1s`, `65535s` | `0s`, `-1s` (keeps running period in `adoptedRefreshPeriod`; `DefaultRefreshPeriod` in `jitteredRefreshDelay` / `stateLifetime`) | `65536s` (keeps running period in `adoptedRefreshPeriod`; clamped to `65535s` in `receivedRefreshPeriod`) |
| `refresh-multiplier` (`K`) | `1..255` | `1` (`2.25*R`), `255` (`383.25*R`, no `int64` overflow with `R = 65535s`) | `0`, `-1` (clamped to `DefaultRefreshMultiplier` = 3 in `stateLifetime`) | N/A (`uint8` YANG ceiling is 255) |
| `jitteredRefreshDelay` RNG draw | `[0, span]` | `0` (`0.5R`), `span` (`1.5R`) | `< 0` (clamped to `0` -> `0.5R`) | `> span` (clamped to `span` -> `1.5R`) |
| Slew increase ratio `R2 / R1` | `(1.0, 1.30]` | `1.30` (`10000ms -> 13000ms`; `2856ms -> 3712ms`) | `target <= current` (decrease: immediate; equal: `changed = false`) | `target > 1.30 * current` (clamped to `<= 1.30 * current`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rsvpte-reload` (existing) | `test/reload/rsvpte-reload.ci` | Config commit reloading `rsvp-te` with changed `refresh-period` and removed interface succeeds cleanly with no panic or regression | |
| `rsvpte-config-valid` (existing) | `test/parse/rsvpte-config-valid.ci` | Existing `refresh-period` and `refresh-multiplier` config syntax remains valid | |
| `rsvpte-lsp-setup`, `rsvpte-lsp-teardown`, `rsvpte-bandwidth`, `rsvpte-reroute`, `rsvpte-frr` (existing) | `test/rsvpte/*.ci` | End-to-end RSVP-TE functional suite continues to pass under jittered refresh and `(K + 0.5) * 1.5 * R` lifetime | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `transit-resv-state-expires` (updated) | `test/interop-rsvpte/scenarios/transit-resv-state-expires/` | freeRouter (ingress & egress) | `RFC2205-3-27`: Ze transit (`K = 3`) holds a freeRouter (`R = 120s`) reservation past the old `3 * R` (360 s) bound and expires it with `ResvTear` + MPLS swap withdrawal after `(3 + 0.5) * 1.5 * 120s` (630 s); reverts to red if `stateLifetime` is reverted to `K * R` | |
| `ingress-refresh-jitter-and-slew` (new) | `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/` | freeRouter (relay & egress) | `RFC2205-3-26`, `RFC2205-3.7-1`, `RFC2205-3-28`: Ze ingress (`R = 2s`) sends jittered PATH refreshes in `[0.5R, 1.5R]` with non-zero spread observed at freeRouter relay, then after `set rsvp-te refresh-period 10; commit` via `ze cli`, emitted `TIME_VALUES` steps monotonically from `2000ms` to `10000ms` with every `R2 / R1 <= 1.30` while freeRouter relays and keeps the LSP Up; reverts to red if either jitter or slew is reverted | |
| `plr-backup-path-to-egress-merge-point` (existing) | `test/interop-rsvpte/scenarios/plr-backup-path-to-egress-merge-point/` | freeRouter (relay & egress) | Regression check: local repair and 10.5 s `protectedStateLifetime` stay green under jittered refresh | |

## Files to Modify
- `internal/plugins/rsvpte/register.go` - add `jitteredRefreshDelay`, slew-limited `adoptedRefreshPeriod`, advance `eng.setRunningRefreshPeriod(next)` before `refreshPaths` in `refreshTick`, switch `runRefreshLoop` to a reusable jittered `time.Timer`, and read `eng.runningRefreshPeriod()` in `refreshPaths`, `setupTunnel`, and `setupBypass`
- `internal/plugins/rsvpte/fsm.go` - add `stateLifetime(period, factor)` computing `ceil((2*K + 1) * 3 * R / 4)` and use it in `expiredPSBs` (primary PSB and merged branches) and `receivedRSBExpired`
- `internal/plugins/rsvpte/engine.go` - add atomic `refreshPeriod` on `engine` initialized in `newEngine`, add `runningRefreshPeriod` / `setRunningRefreshPeriod`, and use `e.runningRefreshPeriod()` in `sendPathLocked`, `sendResvLocked`, and `handlePathEgress`
- `internal/plugins/rsvpte/reservation.go` - use `e.runningRefreshPeriod()` in `acceptReservation` and `handleResvErr`
- `internal/plugins/rsvpte/reroute.go` - use `e.runningRefreshPeriod()` in `armRerouteLocked`
- `internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang` - update descriptions of `refresh-period` and `refresh-multiplier`
- `internal/plugins/rsvpte/reload_test.go` - update `TestAdoptedRefreshPeriodBounds` for 1.30x increase slew
- `internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go` - update expiry aging offsets for `(K + 0.5) * 1.5 * R`
- `internal/plugins/rsvpte/rfc3032_fsm_test.go` - update `TestLSPTableExpiredPSBs` aging offset for `(K + 0.5) * 1.5 * R`
- `internal/plugins/rsvpte/rfc4090_frr_test.go` - update `TestRFC4090StateExpiresWithoutRefresh` aging offset for `(K + 0.5) * 1.5 * R`
- `internal/plugins/rsvpte/producer_integration_linux_test.go` - use `stateLifetime` for the soft-state expiry bound check
- `internal/plugins/rsvpte/freertr_interop_integration_linux_test.go` - use `stateLifetime` for the peer soft-state expiry bound check
- `internal/le/interoplab/rsvpte/checkers.go` - update `freeRtrResvLifetime` to `(3 + 0.5) * 1.5 * freeRtrRefresh`, add pre-floor survival check in `checkResvStateExpires`, and add `checkIngressRefreshJitterAndSlew`
- `internal/le/interoplab/rsvpte/rsvpte.go` - register `ingress-refresh-jitter-and-slew` in `scenarioCheckerMap`
- `internal/le/interoplab/rsvpte/rsvpte_test.go` - unit tests for tcpdump `TIME_VALUES`/jitter/slew parsing helpers
- `docs/architecture/rsvpte/mpls-rsvp-te.md` - update refresh-loop and state-expiry decisions and retire the open-timing-rules paragraph
- `docs/architecture/testing/interop.md` - update `transit-resv-state-expires` row and add `ingress-refresh-jitter-and-slew` row in `The freeRouter RSVP-TE suite`
- `docs/guide/rsvp-te.md` - update `refresh-period`, `refresh-multiplier`, and `What a commit changes`
- `rfc/short/rfc2205.md` - remove `{gap}` from `RFC2205-2-5`, `RFC2205-3-26`, `RFC2205-3-27`, `RFC2205-3-28` and update `Enrolment reason`
- `docs/features/rfc-status.md` - update RFC 2205 proven MUST count and open-gap list
- `plan/journal/bound-too-small-for-its-own-burst.md` - mark the 2026-08-29 RSVP-TE `expiredPSBs` row `fixed:`
- `plan/journal/setting-changed-faster-than-its-consumer-allows.md` - mark the 2026-09-05 `adoptedRefreshPeriod` row `fixed:`

## Files to Create
- `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go` - positive/negative RFC-tagged unit and wiring tests for `RFC2205-2-5`, `RFC2205-3-26`, `RFC2205-3.7-1`, `RFC2205-x-2`, `RFC2205-3-27`, `RFC2205-3-28`
- `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/ingress.conf` - Ze ingress with `refresh-period 2` and SSH CLI enabled for live reload
- `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/relay-hw.txt` - freeRouter relay hardware config
- `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/relay-sw.txt` - freeRouter relay software config
- `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/transit.conf` - Ze transit config
- `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/egress-hw.txt` - freeRouter egress hardware config
- `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/egress-sw.txt` - freeRouter egress software config

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang` — description updates only on existing `refresh-period` and `refresh-multiplier` leaves; no new leaves or RPCs |
| YANG validation constraints | N-A | Existing `range "1..65535"` (`uint16`) and `range "1..255"` (`uint8`) constraints remain unchanged |
| YANG custom validators | N-A | No new config leaf or dynamic value introduced |
| CLI commands/flags | N-A | No new CLI commands or flags |
| CLI grammar (keyword before value) | N-A | No CLI grammar changes |
| Editor autocomplete | N-A | Existing YANG leaves already have automatic editor completion |
| Functional test for new RPC/API | N-A | No new RPC/API added; existing `test/reload/rsvpte-reload.ci` and `test/rsvpte/*.ci` cover config and reload |
| Pipe completeness | N-A | No command output surface modified |
| Env var registration | N-A | No new environment variables |
| Doctor check for runtime dependencies | N-A | No new socket, path, binary, or kernel dependency introduced |
| Doctor code per problem class | N-A | No new error/refusal class or diagnostic code introduced |
| Prometheus counters/metrics | N-A | Existing `ze_rsvpte_path_sent_total` and `ze_rsvpte_resv_sent_total` already count refreshes |
| BGP family surface (new SAFI / capability / attribute) | N-A | RSVP-TE protocol timing fix, not a BGP family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `features/mpls-ldp-rsvp-te.md` keeps experimental maturity and existing scope; RFC gap count lives in `rfc/short/rfc2205.md` and `docs/features/rfc-status.md` |
| 2 | Config syntax changed? | No | `refresh-period` and `refresh-multiplier` syntax, types, and defaults are unchanged; descriptions updated in `ze-rsvp-te-conf.yang` and `docs/guide/rsvp-te.md` |
| 3 | CLI command added/changed? | No | No CLI command added or changed |
| 4 | API/RPC added/changed? | No | No RPC or API added or changed |
| 5 | Plugin added/changed? | No | Existing `rsvp-te` plugin registration unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/rsvp-te.md` — update `refresh-period`, `refresh-multiplier`, and `What a commit changes` |
| 7 | Wire format changed? | No | `TIME_VALUES` wire layout (Class-Num 5, C-Type 1) is unchanged |
| 8 | Plugin SDK/protocol changed? | No | No changes under `pkg/plugin/` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2205.md` and `docs/features/rfc-status.md` (with discrimination records in `rfc/discrimination/rfc2205.json`) |
| 10 | Test infrastructure changed? | No | Uses existing `interoplab/rsvpte` suite and scenario discovery |
| 11 | Affects daemon comparison? | No | No daemon comparison matrix row affected |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rsvpte/mpls-rsvp-te.md` and `docs/architecture/testing/interop.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata keys touched |
| 14 | Prometheus counters added/changed? | No | No metrics added or renamed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Plugin inventory unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/rsvpte/mpls-rsvp-te.md` (updated), `docs/architecture/testing/interop.md` (updated), `docs/guide/rsvp-te.md` (updated), `docs/DESIGN.md` (unaffected: mentions `rsvpte/register.go` only in the plugin-registration inventory list, which does not change), `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` (unaffected: merged-branch independent expiry and PLR/MP local period advertising hold unchanged, with `stateLifetime` and `runningRefreshPeriod` preserving those contracts), `docs/guide/configuration.md` (unaffected: anchors `ze-rsvp-te-conf.yang` only for the `rsvp-te {}` container example and does not mention `refresh-period`, `refresh-multiplier`, or expiry timing) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rsvp-te.md` config block stays valid (`refresh-period 30`, `refresh-multiplier 3`); prose under Configuration and What a commit changes updated for 157.5 s lifetime, jitter, and slew |

## Design presentation

### What changes

Ze's RSVP-TE engine becomes compliant with RFC 2205 Sections 2.1 and 3.7 for soft-state refresh timing. Periodic PATH and RESV refreshes are now scheduled on a reusable `time.Timer` whose interval is drawn uniformly at random from `[0.5R, 1.5R]` on every cycle so independent routers avoid synchronization (`RFC2205-3-26`, `RFC2205-3.7-1`). Stored PSB, merged-branch, and RSB cleanup timeouts now use the RFC lifetime floor `L = (K + 0.5) * 1.5 * R` (`157.5` seconds at the default `R = 30s`, `K = 3`) so a conforming jittering peer's reservation is never prematurely deleted (`RFC2205-3-27`). When an operator commits a larger `refresh-period`, the running refresh period slews upward by at most 30 percent per refresh cycle (`R2 / R1 <= 1.30`, `RFC2205-3-28`), and every locally originated `TIME_VALUES` object advertises that exact stepped running period while decreases still take effect on the next tick. Finally, the existing immediate-forwarding paths on state change (`RFC2205-2-5`) receive complete positive and negative requirement-tagged proof and discrimination records, and both open journal rows are closed.

### Before and after

1. **Periodic refresh scheduling (`runRefreshLoop`):**
   - Before: `runRefreshLoop` ticks on a fixed `time.NewTicker(30s)`, emitting PATH/RESV every exact 30.0 s so synchronized routers stay synchronized indefinitely.
   - After: `runRefreshLoop` waits on a reusable `time.Timer` reset every cycle to a uniform random delay in `[15.0s, 45.0s]` (`[0.5R, 1.5R]`), while `TIME_VALUES` continues to advertise `30000 ms`.
2. **Soft-state expiry lifetime (`expiredPSBs` / `receivedRSBExpired` with default `R = 30s`, `K = 3`):**
   - Before: state expires after `3 * 30s = 90s`; three consecutive jittered peer refreshes spaced at `1.5R` (45 s) exceed 90 s and prematurely delete live state.
   - After: state expires after `(3 + 0.5) * 1.5 * 30s = 157.5s` (and `630s` against freeRouter's `120s` refresh at `K = 3`), tolerating `K - 1` consecutive losses under worst-case `1.5R` peer jitter.
3. **Raising `refresh-period` via `commit` (e.g. `10s -> 300s`):**
   - Before: the next tick jumps `TIME_VALUES` and the loop ticker from `10000 ms` straight to `300000 ms` in one step; if that single packet is lost, the neighbor times out at its old lifetime while Ze waits 300 s.
   - After: successive refresh cycles step the running period and every locally emitted `TIME_VALUES` through `10000 -> 13000 -> 16900 -> 21970 -> 28561 -> 37129 -> 48267 -> 62747 -> 81571 -> 106042 -> 137854 -> 179210 -> 232973 -> 300000 ms` (`R2 / R1 <= 1.30` at every step). Lowering `refresh-period` (e.g. `30s -> 10s`) still adopts `10000 ms` on the next tick.
4. **Immediate forwarding on state change (`RFC2205-2-5`):**
   - Before: implemented in `handlePathTransit`, `handlePathEgress`, `acceptReservation`, and `mergeBackupPath`, but carried a `{gap}` row in `rfc/short/rfc2205.md` for lack of requirement-tagged positive and negative proof.
   - After: proven with positive and negative `RFC requirement: RFC2205-2-5` unit tests and discrimination records, and `{gap}` removed from `rfc/short/rfc2205.md`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Redefine `refresh-multiplier` as the total floating-point factor and raise its default from `3` to `5.25` | `refresh-multiplier` is a `uint8` YANG leaf (`1..255`, default `3`) matching RFC 2205 Section 3.7's integer loss tolerance `K`; changing its type breaks existing configs and CLI schema, whereas computing `(K + 0.5) * 1.5 * R` from integer `K` preserves configs and matches `protectedStateLifetime` already in `checkers.go` |
| Keep `time.Ticker` and apply a one-time startup offset only | RFC 2205 Section 3.7 item 1 requires ensuring "that any synchronization that may occur is not stable" and randomizing the refresh timer in `[0.5R, 1.5R]` on every cycle; a one-time offset cannot break synchronization caused by a later network event |
| Slew both increases and decreases of `refresh-period` at `1.30x` | RFC 2205 Section 3.7 item 5 explicitly constrains only how fast `R` may increase ("there is a limit on how fast it may increase. Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max"); decreasing `R` refreshes more frequently, never risks timeout, and preserves `TestRefreshTickAdoptsReloadedPeriod` |
| Keep `engine.cfg().RefreshPeriod` as the single period and slew inside `OnConfigApply` | Slew is defined per refresh cycle (`R2 / R1 <= 1.30` between successive refresh messages), not per config commit; keeping target in `cfgPtr` and running period in `engine.refreshPeriod` ensures every advertisement site steps cleanly once per refresh tick |
| Add a new `slew-max` YANG leaf | RFC 2205 Section 3.7 fixes `Slew.Max = 0.30` for the normative MUST NOT in item 5 (`RFC2205-3-28`); adding config surface is unnecessary to fix the defect |

### Documents departed from

- `docs/architecture/rsvpte/mpls-rsvp-te.md`, section `Decision: the refresh loops read the live config, they do not restart`: previously stated that `runRefreshLoop` uses `time.Ticker.Reset` and refreshes before adopting a lengthened period in one step ("That message carries the new period, and the neighbor recomputes its lifetime from it."). After this work, the section documents the per-cycle `[0.5R, 1.5R]` jittered `time.Timer`, the `1.30x` per-cycle slew on increases via `engine.runningRefreshPeriod()`, and advancing the running period before `refreshPaths` so wire `TIME_VALUES` and cadence agree.
- `docs/architecture/rsvpte/mpls-rsvp-te.md`, section `Decision: received reservation state expires on its own deadline`: previously stated that cleanup timeout is `K * R` and noted the two RFC 2205 Section 3.7 timing rules as open pending owner decision ("Two RFC 2205 Section 3.7 timing rules are still open. The cleanup timeout is K*R"). After this work, per the 2026-10-08 owner decision, the section documents the `(K + 0.5) * 1.5 * R` lifetime floor and removes the open-defect note.
- `docs/guide/rsvp-te.md`, sections `Configuration` and `What a commit changes`: previously stated that default expiry is `refresh-period * refresh-multiplier` ("multiplied by this number, which is 90 seconds at the defaults.") and that a committed `refresh-period` takes effect in full on the next tick. After this work, the guide documents `[0.5R, 1.5R]` jitter, `(K + 0.5) * 1.5 * R` expiry (157.5 s at defaults), and `<= 1.30x` per-cycle slew when `refresh-period` is increased.
- `docs/architecture/testing/interop.md`, section `The freeRouter RSVP-TE suite`: previously stated in the `transit-resv-state-expires` row that Ze with `refresh-multiplier 3` times out freeRouter's 120 s reservation in "about six minutes after the freeze". After this work, the row documents the RFC 2205 Section 3.7 timeout of ~10.5 minutes (630 s) and adds the `ingress-refresh-jitter-and-slew` scenario row.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add the `engine.refreshPeriod` atomic field and `runningRefreshPeriod`/`setRunningRefreshPeriod` seam, add `stateLifetime` and `jitteredRefreshDelay` stubs/wiring in `fsm.go` and `register.go`, and write failing wiring and RFC-tagged unit tests in `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`
   - Tests: `TestRFC2205RefreshLoopJittersAndSlewsEndToEnd`, `TestRFC2205CleanupTickEnforcesLifetimeFloor`, `TestRFC2205ImmediateForwardOnStateChange`, `TestRFC2205LocalConfirmDoesNotForwardImmediateResv`
   - Files: `internal/plugins/rsvpte/engine.go`, `internal/plugins/rsvpte/register.go`, `internal/plugins/rsvpte/fsm.go`, `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`
   - Verify: wiring compiles and the new jitter/lifetime/slew tests fail on the unfixed behavior while `RFC2205-2-5` tests exercise the existing immediate-forwarding paths.
2. **Phase: Lifetime floor (`RFC2205-3-27`)** -- implement `stateLifetime(period, factor)` using exact ceiling integer arithmetic `ceil((2*K + 1) * 3 * R / 4)`, wire it into `fsm.go::expiredPSBs` (primary PSB and merged branches) and `receivedRSBExpired`, add RFC quote comments, and update existing unit/integration tests that assumed `K * R`
   - Tests: `TestRFC2205StateLifetimeFloor`, `TestRFC2205CleanupTickEnforcesLifetimeFloor`, plus updated `rfc2205_softstate_timeout_test.go`, `rfc3032_fsm_test.go`, `rfc4090_frr_test.go`, `producer_integration_linux_test.go`, `freertr_interop_integration_linux_test.go`
   - Files: `internal/plugins/rsvpte/fsm.go`, `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`, `internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go`, `internal/plugins/rsvpte/rfc3032_fsm_test.go`, `internal/plugins/rsvpte/rfc4090_frr_test.go`, `internal/plugins/rsvpte/producer_integration_linux_test.go`, `internal/plugins/rsvpte/freertr_interop_integration_linux_test.go`
   - Verify: tests fail before implementation, pass after; check race-instrumented `./le test unit pkg path ./internal/plugins/rsvpte/`.
3. **Phase: Jitter (`RFC2205-3-26`, `RFC2205-3.7-1`, `RFC2205-x-2`) and Slew (`RFC2205-3-28`)** -- implement `jitteredRefreshDelay` (uniform `[0.5R, 1.5R]` via `math/rand/v2.Int64N`), switch `runRefreshLoop` to a reusable `time.Timer`, enforce the `1.30x` increase slew cap in `adoptedRefreshPeriod`, advance `eng.setRunningRefreshPeriod(next)` before `refreshPaths` in `refreshTick`, and switch every local `TIME_VALUES` advertisement site (`refreshPaths`, `setupTunnel`, `setupBypass`, `armRerouteLocked`, `sendPathLocked`, `sendResvLocked`, `handlePathEgress`, `acceptReservation`, plus `handleResvErr` blockade timer) to `runningRefreshPeriod()`
   - Tests: `TestRFC2205RefreshDelayJitterBoundsAndDesync`, `TestRFC2205RefreshDelayNeverEscapesJitterWindow`, `TestRFC2205RefreshSlewLimitsIncreaseRatio`, `TestRFC2205RefreshSlewAllAdvertisersAgree`, `TestRFC2205RefreshLoopJittersAndSlewsEndToEnd`, updated `TestAdoptedRefreshPeriodBounds` in `reload_test.go`
   - Files: `internal/plugins/rsvpte/register.go`, `internal/plugins/rsvpte/engine.go`, `internal/plugins/rsvpte/reservation.go`, `internal/plugins/rsvpte/reroute.go`, `internal/plugins/rsvpte/reload_test.go`, `internal/plugins/rsvpte/rfc2205_refresh_timing_test.go`
   - Verify: tests fail before implementation, pass after under `-race`.
4. **Phase: FreeRouter interop proofs** -- update `freeRtrResvLifetime` and `checkResvStateExpires` (asserting survival past `3 * R` and teardown after `(3 + 0.5) * 1.5 * R`), add `ingress-refresh-jitter-and-slew` scenario + checker + parser unit tests
   - Tests: `TestParseTimeValuesFromTcpdump`, `./le test integration interop-rsvpte` (`transit-resv-state-expires`, `ingress-refresh-jitter-and-slew`, `plr-backup-path-to-egress-merge-point`)
   - Files: `internal/le/interoplab/rsvpte/checkers.go`, `internal/le/interoplab/rsvpte/rsvpte.go`, `internal/le/interoplab/rsvpte/rsvpte_test.go`, `test/interop-rsvpte/scenarios/ingress-refresh-jitter-and-slew/*`
   - Verify: prove each interop scenario goes red when its corresponding fix is reverted and green when restored.
5. **Phase: YANG descriptions, docs, RFC conformance ledger, discrimination records, and journal rows** -- update `ze-rsvp-te-conf.yang`, `mpls-rsvp-te.md`, `interop.md`, `docs/guide/rsvp-te.md`, `rfc/short/rfc2205.md`, `docs/features/rfc-status.md`, and both journal files; record discrimination records for every new or re-aged RFC-tagged unit via `./le rfc record` and verify `./le rfc check` and `./le doc check all`
   - Files: `internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang`, `docs/architecture/rsvpte/mpls-rsvp-te.md`, `docs/architecture/testing/interop.md`, `docs/guide/rsvp-te.md`, `rfc/short/rfc2205.md`, `docs/features/rfc-status.md`, `plan/journal/bound-too-small-for-its-own-burst.md`, `plan/journal/setting-changed-faster-than-its-consumer-allows.md`
   - Verify: `./le rfc check`, `./le doc check all`, `./le test rsvpte`, `./le test reload` (`rsvpte-reload`).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has an implementation at `file:line` and a matching test |
| Feature completeness | Every user story has a working path from entry point to observable wire/FIB outcome |
| Correctness | Exact integer arithmetic in `stateLifetime` never truncates below `(K + 0.5) * 1.5 * R` and never overflows `int64` at `K = 255, R = 65535s`; exact integer millisecond slew in `adoptedRefreshPeriod` never exceeds `1.30x` in wire `TIME_VALUES` |
| Data flow | Ordinary transit PATH relay (`handlePathTransit`) preserves upstream sender's `RefreshPeriod`; every locally originated PATH/RESV (`refreshPaths`, `setupTunnel`, `setupBypass`, `armRerouteLocked`, `sendPathLocked`, `sendResvLocked`, `handlePathEgress`, `acceptReservation`) reads `engine.runningRefreshPeriod()` |
| Concurrency | `runRefreshLoop` uses a single `time.Timer` drained on `<-timer.C` before `Reset`, deferred `Stop()`, no goroutine-per-tick; `engine.refreshPeriod` is `atomic.Int64` with no lock-order inversion against `engine.control` or `LSP.mu` |
| Rule: `ai/rules/rfc-compliance.md` | Every enforced MUST/MUST NOT carries `// RFC 2205 Section X.Y: "<quoted text>"` directly above the code; every added or modified RFC-tagged unit carries a fresh discrimination record from `./le rfc record` |
| Rule: `ai/rules/interop-and-goal-validation.md` | Both `transit-resv-state-expires` and `ingress-refresh-jitter-and-slew` proven red with the fix reverted and rebuilt, then green with the fix restored |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Jittered refresh timer (`jitteredRefreshDelay` + `time.Timer` in `runRefreshLoop`) | `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205Refresh` |
| `(K + 0.5) * 1.5 * R` lifetime floor (`stateLifetime` in `expiredPSBs` and `receivedRSBExpired`) | `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205StateLifetime\|TestRFC2205CleanupTick` |
| `1.30x` increase slew + `engine.runningRefreshPeriod()` across all local `TIME_VALUES` writers | `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205RefreshSlew\|TestAdoptedRefreshPeriod` |
| `RFC2205-2-5` immediate-forward positive and negative unit tests | `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205ImmediateForward\|TestRFC2205LocalConfirm` |
| FreeRouter interop proofs (`transit-resv-state-expires`, `ingress-refresh-jitter-and-slew`) | `./le test integration interop-rsvpte` |
| RFC discrimination records and conformance ledger | `./le rfc check` |
| Updated architecture docs, guide, YANG descriptions, and closed journal rows | `./le doc check all` and `grep` on both journal files |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Peer-advertised `TIME_VALUES` ceiling | Verify `receivedRefreshPeriod` still clamps peer-advertised `R` to `maxRefreshPeriod` (65535 s) so a malicious peer cannot hold state indefinitely or cause `int64` overflow inside `stateLifetime(R, 255)` |
| Non-positive or oversized reload values | Verify `adoptedRefreshPeriod` and `jitteredRefreshDelay` reject `<= 0` and `> maxRefreshPeriod` so `time.Timer.Reset` can never panic on a non-positive duration |
| Timer resource safety | Verify `runRefreshLoop` creates one `time.Timer` with `defer timer.Stop()` and exits cleanly on `ctx.Done()`, with no timer leaks or unbounded goroutines |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional or interop test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Owner decision (2026-10-08): `RFC2205-3-26`, `RFC2205-3-27`, and `RFC2205-3-28` are defects in implemented RSVP fixed by this spec, not deferred gaps (`ai/rules/rfc-compliance.md`).
- `protectedStateLifetime` in `internal/le/interoplab/rsvpte/checkers.go:970` already encodes `(3 + 0.5) * 1.5 * 2s`, confirming integer `K` on `refresh-multiplier` as the established repository convention.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/bound-too-small-for-its-own-burst.md` | 2026-08-29 | RSVP-TE soft-state lifetime, `lspTable.expiredPSBs` (`internal/plugins/rsvpte/fsm.go`) |
| `plan/journal/setting-changed-faster-than-its-consumer-allows.md` | 2026-09-05 | `adoptedRefreshPeriod` (`internal/plugins/rsvpte/register.go`) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep `refresh-multiplier` as integer `K` (`1..255`, default `3`) and compute `L = ceil((2*K + 1) * 3 * R / 4)` | Redefine leaf as float total factor with default `5.25` | Matches RFC 2205 Section 3.7 ("where K is a small integer. Currently K = 3 is suggested as the default"), preserves `uint8` YANG schema and existing configs, and avoids floating-point truncation below the RFC floor |
| Hold `engine.refreshPeriod` (`atomic.Int64`) separately from target `engine.cfgPtr`, stepping in `refreshTick` before `refreshPaths` | Slew inside `OnConfigApply`; or keep single `cfgPtr` and refresh before stepping | Slew is per refresh cycle, not per commit; updating `runningRefreshPeriod` before `refreshPaths` guarantees advertised `TIME_VALUES` and the next cycle's cadence are always identical across all 8 local advertisement sites |
| Floor slewed increases to whole milliseconds at `R1 * 13 / 10` (for `R1 >= 10ms`) | Floating-point `R1 * 1.30` | `TIME_VALUES` encodes integer milliseconds (`refreshMillis`), so integer millisecond flooring guarantees `R2 / R1 <= 1.30` on the wire with zero rounding-up risk |
| Slew increases only; adopt decreases immediately | Slew both increases and decreases | RFC 2205 Section 3.7 item 5 constrains only how fast `R` may increase; decreases shorten the refresh interval and preserve `TestRefreshTickAdoptsReloadedPeriod` |
| Replace `time.Ticker` in `runRefreshLoop` with reusable `time.Timer` drawn from `[0.5R, 1.5R]` via `math/rand/v2` | One-time startup offset on `time.Ticker` | Re-randomizing every cycle satisfies both "avoid message synchronization" and "ensure that any synchronization that may occur is not stable" (RFC 2205 Section 3.7 item 1) |

## Known Limitations
- Per-interface `Rdef`, `K`, and `Slew.Max` tuning (RFC 2205 Section 3.7 item 7, non-normative `should`) is not added here; Ze configures `refresh-period` and `refresh-multiplier` at the `rsvp-te` container level and uses RFC 2205's specified `Slew.Max = 0.30`.
- Incoming-interface exclusion during immediate forwarding (`RFC2205-3-21`) and routing-derived incoming-interface tracking (`RFC2205-3-6`, `RFC2205-3-33`) belong to `plan/pre-release/spec-rsvp-routing-and-interface-integration.md`.
- RFC 2205 Section 3.5 blockade-state timer (`handleResvErr`, `BlockadeUntil`) is governed by `Kb * R` rather than Section 3.7's state lifetime `L`, and multicast wildcard blockade behavior belongs to `plan/spec-rsvp-multicast-wildcard.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC 2205 Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 2205 Section 2.1 (`RFC2205-2-5`) | Default `rsvp-te` config: state modifications on received PATH/RESV/merge/RRO-withdrawal generate and forward updated refresh messages immediately without waiting for the refresh timer | `RESV_CONFIRM` local confirmation (`confirmHere`) suppresses upstream relay on equal-or-smaller requests per Section 2.6 | `TestRFC2205ImmediateForwardOnStateChange` | `TestRFC2205LocalConfirmDoesNotForwardImmediateResv` |
| RFC 2205 Section 3.7 item 1 (`RFC2205-3-26`, `RFC2205-3.7-1`, `RFC2205-x-2`) | Default `refresh-period 30`: every refresh interval is drawn independently and uniformly from `[0.5R, 1.5R]` (`[15s, 45s]`) while `TIME_VALUES` advertises `R` (`30000ms`) | Operator-configured `refresh-period` across `1..65535` seconds scales the `[0.5R, 1.5R]` jitter window proportionally | `TestRFC2205RefreshDelayJitterBoundsAndDesync`, `TestRFC2205RefreshDelayNeverEscapesJitterWindow`, `TestRFC2205RefreshLoopJittersAndSlewsEndToEnd` | `ingress-refresh-jitter-and-slew` (configured `refresh-period 2` -> `10`) |
| RFC 2205 Section 3.7 item 2 (`RFC2205-3-27`) | Default `refresh-multiplier 3` (`K = 3`), `refresh-period 30`: stored PSB/RSB lifetime `L = (K + 0.5) * 1.5 * R` = `157.5s` (derived from peer's advertised `R`) | Operator-configured `refresh-multiplier` across `1..255` applies the same `(K + 0.5) * 1.5 * R` floor; live reload via `OnConfigApply` updates `K` without restart | `TestRFC2205StateLifetimeFloor`, `TestRFC2205CleanupTickEnforcesLifetimeFloor`, `transit-resv-state-expires` | `TestCleanupTickUsesReloadedMultiplier` (`K = 10` -> `K = 1`), `TestEgressStateLifetimeFollowsSenderPeriod` |
| RFC 2205 Section 3.7 item 5 (`RFC2205-3-28`) | Dynamic increase of `refresh-period` via `commit` steps the running period and every locally advertised `TIME_VALUES` at `R2 / R1 <= 1.30` per refresh cycle | Operator-configured `refresh-period` decreases apply immediately on the next refresh tick; unchanged/out-of-range values keep the running period | `TestRFC2205RefreshSlewLimitsIncreaseRatio`, `TestRFC2205RefreshSlewAllAdvertisersAgree`, `ingress-refresh-jitter-and-slew` | `TestRefreshTickAdoptsReloadedPeriod` (`30s -> 10s`), `TestRefreshTickIdempotentOnUnchangedPeriod`, `TestAdoptedRefreshPeriodBounds` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks every row Yes / No / N-A with reason

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, plus `./le test integration interop-rsvpte` and `./le rfc check`. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/plugins/rsvpte`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (`transit-resv-state-expires`, `ingress-refresh-jitter-and-slew`, `plr-backup-path-to-egress-merge-point`)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + journal row updates + RFC discrimination records
- [ ] **Commit B:** `remove plan/release/fixes/spec-rsvp-refresh-timing.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
