# Spec: tiers-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/6 |
| Updated | 2026-07-22 |

Phase note (cell corrected 2026-07-22; the old "Phase 4 IN PROGRESS" contradicted
the body): Phases 1-4 are COMPLETE -- the body records Phase 4 complete
2026-06-24 with both items resolved and tiers-5 B-1 DONE; the repo confirms the
end-state (isis/ldp/rsvpte/flowexport/mrt under `internal/plugins/`,
sysrib/bfd/sysctl under `internal/component/`, `internal/plugins/ospf/v3/`
nested, and `scripts/dev/tier_migration_baseline.txt` empty = zero-exception
enforcement). Remaining: tiers-5 Path B preconditions (B-2/B-3/config split).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugins.md` - the "delete the folder" invariant this generalizes
4. `ai/rules/plugins.md` - registration patterns, Proximity Principle
5. `ai/rules/rule-placement.md` (global) + `ai/rules/repo-maintenance.md` - where the new rule doc must live
6. `scripts/dev/dep_audit.py` - the reverse-dependency tool that becomes the placement gate
7. `scripts/codegen/plugin_imports.go` - hardcoded `pluginDirs` / `rpcRoot` that encode the current split
8. `internal/component/plugin/all/all.go` - generated composition root (blank imports)

## Task

The `internal/component/` vs `internal/plugins/` split in Ze is currently a matter
of historical placement, not a defined or enforced rule. Some directories under
`internal/component/` are import-independent edge plugins (IS-IS, LDP, RSVP-TE,
flow-export, MPLS) that nothing depends on; some directories under
`internal/plugins/` are platform services that other plugins depend on (the system
RIB, BFD, sysctl). New code has no documented rule telling an author which tier a
package belongs in, and nothing verifies placement.

Establish a **three-tier module taxonomy** defined by dependency direction,
document it as a canonical rule so new code lands in the correct tier, enforce it
with an automated placement audit, and then restructure the misplaced directories
to match - with the moves performed by a deterministic Python tool, not by hand.

**Goal:** Make the core / component / plugin boundary explicit, documented,
machine-auditable, and consistent with the actual import graph.

This is an **umbrella spec**. Phase 0 (the rule + the audit gate + the migration
tool) is specified here and is the foundation. The directory moves are sequenced
into child specs (below), each created from this umbrella, each behavior-preserving,
each gated by the Phase 0 audit returning clean.

## The Taxonomy (the rule to document)

Two binary, mechanically-checkable axes classify every top-level directory under
`internal/component/` and `internal/plugins/`:

| Axis | Test (mechanical) |
|------|-------------------|
| **A. Plugin-shaped?** | Does any file in the directory call `sdk.NewWithConn(` (the config-driven plugin engine entry point)? |
| **B. Depended-upon by another plugin/component?** | Does any `.go` file *outside* the directory's own subtree - excluding the generated composition root, `cmd/ze` dispatch, `cmd/ze/hub` daemon wiring, `internal/core`, `internal/chaos`, `internal/test`, and `_test.go` - import a package under it? |

The three tiers follow:

| Tier | Directory home | Axis A | Axis B | Meaning | Examples |
|------|----------------|--------|--------|---------|----------|
| **core / infra** | `internal/core/` (leaf libs) | No (a library, not an engine) | (either) | Cannot be "run as a plugin." Foundational. | family, events, metrics, diagnostic, bufpool |
| **component** | `internal/component/` | Yes | Yes | Platform plugin: other plugins plug into it / consume its service. | bgp, iface, firewall, traffic, vpp |
| **plugin (edge)** | `internal/plugins/` | Yes | No | Edge plugin: pure leaf, nothing depends on it. | ntp, static, dhcpserver, l2tp-auth-* |

**The defining principle: tier is decided by dependency direction, not by size or
age.** BGP is a `component` because its sub-plugins and other code plug into it; it
is the archetype. IS-IS is a `plugin` because it consumes services (iface,
redistribute) but nothing consumes it. The RIB stays a `component` because edge
protocols install routes through it.

### Authoring rule (so new code lands correctly)

Documented in `ai/rules/` (Phase 0): when adding a new package, decide its tier by
the two axes above BEFORE choosing a directory. If it is an engine that other
plugins will depend on, it is a `component`. If it is an engine nothing depends on,
it is an edge `plugin`. If it is a non-engine library, it is `core`. The placement
audit (below) enforces this on every verification run.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/planning.md` - workflow rules for umbrella + child specs
  -> Constraint: one spec at a time per session; children close via the two-commit rule.
- [ ] `ai/rules/plugins.md` - the "delete the folder" invariant this umbrella generalizes
  -> Constraint: removing a plugin folder plus its blank import must leave every other plugin and the core working; tier moves must preserve that property.
- [ ] `ai/rules/plugins.md` - registration patterns, Proximity Principle
  -> Constraint: both tiers register through the same registry; the tier is decided by dependency direction, not by capability difference.
- [ ] `~/.claude/rules/rule-placement.md` (global) + `ai/rules/repo-maintenance.md` - where the new rule doc must live
  -> Decision: the taxonomy is project-wide agent behavior, so the rule lands in `ai/rules/architecture.md` (shared), never in a tool-specific home.
- [ ] `scripts/dev/dep_audit.py` - the reverse-dependency tool that becomes the placement gate
  -> Constraint: the gate reuses this tool's import-graph walk; no second graph walker is written.
- [ ] `scripts/codegen/plugin_imports.go` - hardcoded `pluginDirs`/`rpcRoot` encode the current split
  -> Constraint: every move must update these lists and re-run the generator; `all.go` is generated, never hand-edited.

**Key insights:**
- The component/plugin boundary is not a capability difference; the enforceable distinction is dependency direction, read off the import graph (see Design Insights).

## Current Behavior (MANDATORY)

**Source files read (this session):**
- [ ] `internal/component/isis/register.go` - registers via `registry.Registration` + `sdk.NewWithConn("isis")` + `registry.RunEngine`; `ConfigRoots: ["isis"]`. Config-driven plugin engine.
  -> Constraint: IS-IS is mechanically already a plugin; only blank-import references exist.
- [ ] `internal/component/plugin/all/all.go` - generated composition root; blank-imports every plugin (isis x3, ldp, rsvpte, flowexport, mpls).
  -> Constraint: moves MUST regenerate this file via the generator, not by hand.
- [ ] `scripts/codegen/plugin_imports.go` - hardcoded `pluginDirs` (lines 122-133) lists `internal/component/{flowexport,iface,ldp,rsvpte,traffic,vpp}` + `internal/plugins`; `rpcRoot = "internal/component"`.
  -> Constraint: the component/plugins split is encoded in the generator. Every move MUST update `pluginDirs`/`rpcRoot` and re-run the generator. IS-IS is discovered via the event-namespace/RPC/yang scanners, NOT `pluginDirs`.
- [ ] `docs/plugin-overview.md` - already states IS-IS, LDP, RSVP-TE, flow-export "register through the same plugin registry."
  -> Constraint: docs already half-acknowledge these are plugins; the rule formalizes it.
- [ ] `internal/core/` - leaf library tier; imports nothing from `internal/component/` except a handful of registry shims (diagnostic, ipc/yang, resolve).
  -> Constraint: moving infra INTO core requires the moved code to be leaf. config is NOT leaf (below), so config cannot move to core as-is.
- [ ] `internal/component/config` import set - imports bgp, isis, iface, cli, web, mcp, host, hub, telemetry, ... (the components it wires).
  -> Constraint: config is a top-level orchestrator, not leaf infra. The infra->core phase must first separate config's leaf primitives from its component-wiring orchestration, or it will create import cycles.

**Behavior to preserve:**
- All runtime behavior. Every move is a pure relocation: same registration, same config roots, same commands, same wire behavior. No functional change.
- The generated `all.go` must blank-import the same set of packages (at new paths) after each move.
- `bin/ze --plugins` output (the runtime plugin inventory) unchanged across moves.

**Behavior to change:**
- Only directory location and import paths of the relocated packages, plus the generator's directory lists and the documentation/inventory lists that name them.

## Findings: current misplacements (audit output, this session)

Produced by `scripts/dev/dep_audit.py`. These drive the child specs.

**In `internal/component/` but are edge plugins -> move to `internal/plugins/`:**

| Dir | Engine | Plugin/component consumers | Move footprint (files referencing path) |
|-----|--------|---------------------------|-----------------------------------------|
| `isis` | yes | 0 | 110 (mostly its own tree) |
| `ldp` | yes | 0 | 5 |
| `rsvpte` | yes | 0 | 3 |
| `flowexport` | yes | 0 | 33 |
| `mpls` | forwarding helper | 0 | 1 |
| `ike` (borderline) | yes (`vpn`) | 1 (web UI page only) | TBD child spec |
| `mrt` (borderline) | yes | 1 (`cmd/ze/hub` only) | TBD child spec |

**In `internal/plugins/` but are platform plugins -> move to `internal/component/`:**

| Dir | Engine | Plugin/component consumers |
|-----|--------|---------------------------|
| `sysrib` (the RIB) | yes | `fib/kernel`, `fib/p4`, `fib/vpp` |
| `bfd` | yes | `bgp/reactor`, `static` |
| `sysctl` | yes | `iface`, `firewall`, `fib/kernel` |

**In `internal/component/` but are non-engine infra -> candidate `internal/core/` (hard phase):**
`config`, `command`, `plugin` (registry), `cli`, `aaa`, `audit`, `authz`, `host`,
`api`, `engine`, `resolve`, `telemetry`, `pki`, `storage`, `ssh`, plus host-service
UIs (`web`, `gnmi`, `mcp`, `lg`) and non-engine subsystems (`l2tp`, `ppp`, `pppoe`,
`subscriber`, `radius`, `tacacs`, `ipsec`). Plus dead stubs `diag`/`update` and the
`doctor` framework. Most are NOT leaf and cannot move to `internal/core/` without
decoupling first; this phase is deferred and may become its own umbrella.

## Data Flow (placement & discovery, see `ai/rules/architecture.md`)

### Entry Point
A package's `register.go` (and its `sdk.NewWithConn`/`registry.Registration`) plus
its directory location.

### Transformation Path
1. Author places a package under a tier directory.
2. `scripts/codegen/plugin_imports.go` scans `pluginDirs` + `rpcRoot` + yang/event scanners and generates `internal/component/plugin/all/all.go`.
3. `all.go` blank-imports the package; its `init()` calls `registry.Register`.
4. At runtime the registry discovers and (for config-driven engines) starts it when its `ConfigRoots` appear in config.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Directory <-> generator | hardcoded `pluginDirs`/`rpcRoot` | [ ] each move updates them |
| Generator <-> composition root | `all.go` regeneration | [ ] `plugin_imports.go --check` clean |
| Placement <-> rule | the new `ai/rules/` doc + audit gate | [ ] audit exits 0 |

### Integration Points
- `scripts/dev/dep_audit.py` already computes axes A/B - it becomes the audit gate.
- `scripts/codegen/plugin_imports_test.go` is the precedent for a Go test that guards generated wiring.

### Architectural Verification
- [ ] No bypassed layers (moves keep registry discovery intact)
- [ ] No unintended coupling (no new cross-tier imports introduced)
- [ ] No duplicated functionality (audit reuses dep_audit.py, not a second graph walker)
- [ ] Zero-copy preserved (N/A - relocation only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Axes A/B classify every dir unambiguously enough to gate WITHOUT an allowlist | dep_audit.py run this session produced a clean candidate list | gate produces false positives/negatives; needs an exceptions allowlist | run `--check` over current tree, count disputed dirs | BROKEN - design probe (Phase 5 Hardening Analysis) shows 65 raw mismatches; a no-allowlist rule needs structural preconditions. See that section. |
| A-2 | Edge-out moves are behavior-preserving relocations | isis/ldp/rsvpte/flowexport have 0 external importers; only blank imports | a hidden importer breaks build | `go build ./...` + `make ze-verify` after each move | unvalidated |
| A-3 | The generator is the only place encoding the split | `pluginDirs`/`rpcRoot` found in plugin_imports.go | another script/glob hardcodes `internal/component/<x>` | grep scripts/ + Makefile for the moved paths | unvalidated |
| A-4 | config cannot move to internal/core as-is | config imports bgp/isis/iface/web | infra->core phase underscoped | trace config sub-package imports | unvalidated |
| A-5 | A Python tool can perform moves without `git mv` (forbidden from Bash) | git-safety.md forbids git add/rm; os.rename + import rewrite is plain FS | staging confusion | tool does FS move + rewrite; user runs the commit script | unvalidated |
| A-6 | Borderline engines (ike, mrt) need per-case decision | each has exactly 1 non-daemon/UI consumer | misclassified move | child spec reviews each | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Import-path rewrite misses a string-built path or struct tag | build break or a runtime registry miss | Python tool rewrites only quoted import paths; `go build ./...` + plugin inventory diff after each move |
| R-2 | Generator not updated -> all.go drops a plugin silently | `plugin_imports.go --check` fails or `--plugins` count drops | run generator + its `--check` test as part of each move; diff `bin/ze --plugins` before/after |
| R-3 | The "core" tier is fuzzy for host-services (web, ssh, ...) -> audit false alarms | audit flags dirs no one wants to move | scope the gate to the two unambiguous engine rules first; host-services and infra->core use a declared exceptions allowlist with justifications |
| R-4 | Moves collide with the active `route-config-plugin-migration` spec | merge conflicts in bgp/ trees | that spec touches bgp route families, not tier dirs; no overlap, but sequence after it if needed |
| R-5 | Cross-platform path/build-tag breakage (linux-only files) | darwin build green, linux build red | tool preserves filenames incl. `_linux.go`; CI/QEMU build both platforms |

## Child Spec Plan (sequenced, lowest risk first)

| # | Child spec | Scope | Risk | Gated by |
|---|-----------|-------|------|----------|
| 1 | `spec-tiers-1-rule-and-audit.md` | Document the taxonomy in `ai/rules/architecture.md`; add to `ai/rules/INDEX.md` + CLAUDE.md Before-You table; extend `dep_audit.py` with **Path C**: `--check` enforces ONLY the engine rule (engine -> component if depended, else plugins), exit 2 on an engine misplacement; core/composition/host printed as advisory, NOT enforced; **no allowlist**. Add Go test gate `TestEnginePlacement`; wire into `make ze-verify`. **No moves.** | low | self (audit over current tree records the 8 engine mismatches as the worklist) |
| 2 | `spec-tiers-2-edge-out.md` | Move `isis, ldp, rsvpte, flowexport, mpls` component->plugins via the Python migration tool; update generator `pluginDirs`/`rpcRoot`; regenerate `all.go`; update docs/inventory. | low-med | Phase 1 audit |
| 3 | `spec-tiers-3-platform-in.md` | Move `sysrib, bfd, sysctl` plugins->component via the tool; update importers, generator, docs. | med | Phase 1 audit |
| 4 | `spec-tiers-4-borderline.md` | Decide + move `ike`, `mrt` per-case. **DONE (Phase 4, 2026-06-24):** `mrt` moved to plugins in Phase 2; `ike` stays component (2 feature deps). Also absorbed the post-Phase-3 `ospf` gate regression (nested `ospfv3` -> `ospf/v3`). | med | Phase 1 audit |
| 5 (FUTURE, Path B) | `spec-tiers-5-preconditions-*` (own umbrella) | Structural work to extend enforcement beyond engines: extract bgp/iface/vpp/ike codec subpackages to `internal/core/` (un-fuse library+engine, blocker B-2); unify plugin-registration discovery so the gate reuses the generator (blocker B-1); decide a home for framework/host - a 4th `internal/host/` tier or fold to core (blocker B-3); then separate config leaf primitives from orchestration and move true leaf infra to `internal/core/`. Only after this can the gate enforce core/composition with no allowlist. | high | Phases 1-4; deferred, not scheduled now |

## Phase 2 (edge-out) -- COMPLETE (2026-06-20)

Performed directly via `scripts/dev/migrate_module.py` (learned summary
the tiers-2 edge-out record). Moved set = the migration baseline's
tiers-2 rows: **`isis, ldp, rsvpte, flowexport, mrt`** `component/` -> `plugins/`.

-> Decision: the moved set is `mrt`, NOT `mpls` (the early prose/AC-6 said mpls).
`mpls` has no `sdk.NewWithConn`, so the engine gate never tracked it; it is a
forwarding helper and stays in `internal/component/`. `mrt` IS an `sdk.NewWithConn`
engine with no feature dependency (its only importer, `cmd/ze/hub`, is non-feature),
so the gate placed it in `plugins/`. This supersedes the "mrt borderline -> tiers-4"
line in the Findings/Child-Spec tables and the "mpls" token in AC-6.

-> Constraint: `rpcRoot` did NOT need widening. The four RPC-bearing roots
(isis/ldp/rsvpte/flowexport carry `RegisterRPCs` in their *root* package) are
re-discovered by `discoverPlugins` once under the `internal/plugins` whole-tree
pluginDir, so the RPC registration survives the move (proven by an `all.go`
blank-import set-diff: 0 dropped).

Results:
- `all.go` registration set preserved (0 dropped). 3 additions, all benign:
  `mrt` (promoted from hub-only registration to a first-class plugin), and
  `isis/cli` + `isis/transport` (cosmetic -- both already ran `init()` before).
- Migration baseline shrank 8 -> 3 (only `bfd, sysctl, sysrib` -> tiers-3 remain);
  `dep_audit.py --check` green.
- `go build ./...`, moved-package unit tests, generator `--check`, and
  golangci-lint (moved trees + importers) all pass. Stale path references fixed
  across `.go/.sh/.mk/.ci/.yang/docs/rfc`; `arch_map.py` + `code_to_docs.py`
  regenerated. `plan/` specs left for their owners.
- Excluded from the migration commit: `mk/test-integration.mk` and
  `docs/functional-tests.md` (carried concurrent uncommitted user work); their
  path fixes remain on disk for the owner to commit.

Remaining: tiers-3 (platform-in: `bfd, sysctl, sysrib` plugins -> component),
tiers-4 (borderline `ike`), tiers-5 (Path B preconditions).

## Phase 3 (platform-in) -- COMPLETE (2026-06-20)

Performed via `scripts/dev/migrate_module.py` (learned summary
the tiers-3 platform-in record). Moved set = the migration baseline's
tiers-3 rows: **`bfd, sysctl, sysrib`** `plugins/` -> `component/`. All three are
`sdk.NewWithConn` engines that a feature DOES depend on (axis B), so the gate
places them in `internal/component/`: BGP reactor + static depend on BFD; iface +
firewall on sysctl; the fib backends + fakefib on sysrib.

-> Decision: `bfd` was a MERGE, not a plain move. `internal/component/bfd/` already
held the BFD CLI command (`cmd/`, discovered via `rpcRoot`), while the engine lived
at `internal/plugins/bfd/`. The engine's files merged into `internal/component/bfd/`
alongside the existing `cmd/` (canonical layout: engine at root, CLI in `cmd/`) with
zero file-level conflict. `migrate_module.py` was extended to perform a
conflict-checked merge when the destination already exists (refusing on any real
path collision), and to disambiguate a both-areas name via `--to`.

-> Constraint: the merge exposed a latent bug in the tool's registration-preservation
proof. It normalised the post-move `all.go` blank-import set BACKWARD (dst->src),
which wrongly remapped the pre-existing `component/bfd/cmd` and reported a false
drop. Fixed by normalising the BEFORE set FORWARD (src->dst): forward leaves
pre-existing destination paths untouched. Both directions stay boundary-safe.

Results:
- `all.go` registration set preserved (0 dropped, 0 added) for all three moves.
- Migration baseline shrank 3 -> 0: **`dep_audit.py --check` now reports
  "engine placement clean; no exceptions (baseline empty)"** -- full
  engine-placement enforcement with zero exceptions, the umbrella's stated end
  state for placement.
- `go build ./...`, moved-package + importer unit tests, generator `--check`,
  `golangci-lint` (0 issues), `code_to_docs.py`/`arch_map.py` `--check`, and
  `go test ./scripts/dev/` all pass. Stale path references fixed across `.go`
  comments / docs / `.mk` / `.ci`; generated indexes regenerated. `plan/` and
  `.claude/plan/` references left for their owners.
- The relocation surfaced TWO pre-existing, tiers-3-independent issues in the
  changed-file-aware commit gate (`verify_wiring_docs.py`), both fixed AT the gate
  (not worked around), with regression tests:
  (1) the wiring check was rename-blind -- a moved file's symbols all looked "added"
      because the baseline reads HEAD at the NEW path (empty); now symbols REMOVED
      from deleted files in the same change are treated as pre-existing relocations,
      so a behaviour-preserving move contributes zero "added" symbols.
  (2) `test/.ci-sleep-baseline` was stale (423) while committed HEAD already held
      424 sleeps; corrected to 424 (no new sleep introduced by this work).

-> Constraint (pre-existing, NOT tiers-3): `make ze-validate-commands` reports 5
`ze-firewall-irr-cmd` commands with no handler (from the committed firewall-IRR
feature `f439d7066`/`d9afefd2d`). The `all.go` diff shows firewall/irr untouched by
this work, and the validator lists zero `bfd/sysctl/sysrib` problems, so tiers-3 adds
nothing here; this gap is the firewall-IRR feature's to close and blocks `ze-verify`
independently of this migration.

Remaining: tiers-4 (borderline `ike`), tiers-5 (Path B preconditions / the infra
core-vs-host classification in the Open Design Decision + Phase 5 analysis below).

## Phase 4 (ospf-in regression + ike borderline) -- COMPLETE (2026-06-24)

Two items closed here: a NEW engine misplacement the Phase-1 gate caught after
Phase 3, and the tiers-4 `ike` borderline.

### OSPF: gate regression resolved by nesting the v6 leaves (NOT a tier move)

After Phase 3 the OSPF/OSPFv3 feature landed. The Phase-1
gate then went RED: `internal/plugins/ospf` (an `sdk.NewWithConn` engine) was flagged
to move to `internal/component/`, because something under `internal/plugins/` imported
the `ospf` engine subtree -- specifically `internal/plugins/ospfv3/transport` imported
`internal/plugins/ospf/wire` (`RawPacket`).

-> Decision: this is NOT a real platform dependency. `ospfv3` is not a peer plugin --
no `sdk.NewWithConn`, no top-level files; it is three leaf library packages (`types`,
`packet`, `transport`) plus one doctor-check registration, all consumed by the single
unified `ospf` engine (learned `972-ospf-af-unify`). OSPF is ONE edge plugin whose code
was split across two top-level dirs by history. The fix is to make the OSPF plugin
self-contained (plugin-self-containment), NOT to relabel it a component.

-> Resolution: nested `internal/plugins/ospfv3/{types,packet,transport}` ->
`internal/plugins/ospf/v3/{...}` via a deterministic relocation (FS move +
boundary-safe quoted-path rewrite over .go/.ci/.sh/.md; plan/ historical specs and the
generated `ai/` index skipped). The back-edge `ospf/v3/transport -> ospf/wire` is now
INTERNAL to the ospf tree, so axis B finds no external feature depending on `ospf`; it
correctly stays an edge plugin in `internal/plugins/`.

-> Constraint: the two import-guard tests (`ospf/v3/{packet,types}/imports_test.go`)
forbid imports by SUBSTRING and assumed the top-level-sibling layout; nesting made
`packet`'s legitimate `ospf/v3/types` import match the forbidden `internal/plugins/ospf/`
prefix. Both guards were rewritten for the nested layout (the codec may import ONLY the
`ospf/v3/types` leaf; the types leaf imports only stdlib). This preserves 972's
discipline -- 972 chose separate guarded leaf packages + a one-way engine->leaf
dependency, NOT a top-level location, so nesting does not reverse it.

Results:
- `dep_audit.py --check`: GREEN (engine placement clean, baseline empty).
- `go build ./...` green; `all.go` registration set preserved (the lone
  `ospfv3/transport` doctor blank-import is now `ospf/v3/transport`; 0 dropped/added).
- `ospf` engine + `ospf/v3/{packet,transport,types}` unit tests + both rewritten guards
  pass. `ai/INSTRUCTIONS.md` arch lists regenerated (`ospfv3` dropped from the top-level
  `internal/plugins/` enumeration); `ai/CODE-TO-DOCS.md` doc index regenerated.
- 102 path rewrites across 65 files (Go + docs + `.ci` + evidence script). `plan/`
  learned summaries and `ai/LEARNED-INDEX.md` left as historical records.

### ike: stays in `internal/component/` (no move)

`ike` is an `sdk.NewWithConn("ike")` engine and TWO features depend on it:
`internal/component/web/page_vpn_ipsec.go` (the VPN/IPsec UI) and
`internal/component/cmd/clear/doc.go` (the `clear` CLI). Per the engine rule (axis B)
an engine a feature depends on is a `component`; the gate does not flag it. The tiers-4
borderline therefore resolves to **no move** -- ike is correctly a component. (The
Findings table's "1 (web UI page only)" undercounted; there are 2 feature consumers.)

Remaining: tiers-5 (Path B preconditions) -- accepted in full, sequenced as its own
child specs (B-1 unify discovery, B-3 host tier, B-2 library extraction, config split).

## tiers-5 Progress (Path B preconditions)

### B-1 -- unify plugin discovery -- DONE (2026-06-24)

The advisory tier classification in `scripts/dev/dep_audit.py` no longer guesses
"is this a plugin?" from a registration grep (the probe's 65-false-positive class).
It now reads the **composition roots** -- the generated `all.go` AND `cmd/ze`
dispatch (via `is_registration_importer`) -- so a subsystem is "wired" iff a
composition root blank-imports it. This catches every plugin shape (registry.Register,
RegisterRPCs, RegisterBackend, doctor checks, and `*-cmd` verb providers wired only
through dispatch) without a per-mechanism heuristic.

-> Decision: the "wired" signal is `len(registration) > 0`, NOT an all.go-only parse.
An early all.go-only version mis-classified the dispatch-wired command plugins
(`completion`, `passwd`, `signal`, `skills`, `init`, `exabgp`, ...) as core
candidates; folding in `cmd/ze` dispatch (already recognized by
`is_registration_importer`) fixed it -- plugins-area core candidates dropped 10 -> 1
(only `ifacenetlink`, a genuine registration=0/external=0 leaf, remains).

-> Constraint: this improves only the ADVISORY (core/composition is still NOT gated --
Path C). Full enforcement stays blocked by B-2 (BGP fuses codec+engine) and B-3 (host
tier). The enforced engine gate is unchanged.

Results: new fields `is_registered`/`is_engine`/`core_candidate` on each advisory row;
the report sections are REGISTERED PLUGINS / CORE CANDIDATES / SHARED LIBRARIES;
`dep_audit.py --selftest` gained B-1 fixtures and is now wired into `make ze-tier-check`
(it ran the gate's `--check` only before, never its own tests); `ai/rules/architecture.md`
updated. ruff clean; selftest + gate green.

Remaining: B-3 (host-tier decision -- the Open Design Decision below), B-2 (extract
bgp/iface/vpp/ike library subpkgs to core), config leaf/orchestration split.

## Open Design Decision (resolve at child-spec time)

**Where does non-engine infra live, and how far does "core" extend?**
- Option (a): only true leaf libs move to `internal/core/`; host-services (web, ssh, gnmi, mcp, lg) and orchestration (config, command) stay in `internal/component/` under a documented "host/orchestration" sub-rule. Smallest, honest.
- Option (b): introduce a new `internal/host/` (or `internal/server/`) tier for daemon services, reserving `component/` strictly for platform plugins. Cleaner three-way split, more churn.
- Option (c): defer all infra movement; only enforce the two engine rules (edge-out, platform-in). Lowest risk; leaves `component/` mixed.

Recommendation to carry into Phase 5: start from (a)/(c) hybrid - enforce engine
rules now, document host/orchestration as an explicit allowlisted category, and
only relocate genuinely-leaf infra.

## Phase 5 Hardening Analysis (design probe results)

A read-only probe (`tmp/tier_classify_probe.py`, not committed) classified every
dir with three import-graph signals - `is_plugin` (registers), `is_engine`
(`sdk.NewWithConn`), `imports_engine`, `depended_by_feature` - to test whether a
fully allowlist-free rule is reachable. It is NOT, with the current code structure.
Three structural blockers, each evidence-backed:

| # | Blocker | Evidence from probe | Consequence |
|---|---------|---------------------|-------------|
| B-1 | "Is a plugin" has no single mechanical signal | `*-cmd` plugins (ldp-cmd, aaa-cmd, ...) match none of `registry.Register` / `pluginserver.RegisterRPCs` / `RegisterBackend` / `sdk.*`; a grep mis-sends them to `core` | the audit MUST reuse the generator's plugin discovery (`plugin_imports.go`), not a grep heuristic, to know what is a plugin |
| B-2 | Fused library+engine dirs poison `imports_engine` | `bgp` is consumed as a library for `attribute, config, events, message, types, yang` (also iface/vpp/ike/plugin) | "imports bgp" cannot distinguish codec use (core) from engine dependence (composition) until those library subpackages are extracted to `internal/core/` |
| B-3 | Framework/host packages defy the trichotomy | rule wants to exile `doctor`, `gnmi`, `storage` to `plugins/`; `web`/`hub`/`mcp`/`lg`/`ssh` land in composition by accident of one import | these need human judgment - an irreducible small allowlist OR a 4th `host` tier |

**Raw probe result: 65 mismatches** - dominated by B-1 (false `core` verdicts for
cmd-plugins) and B-3 (framework packages), NOT by genuine engine misplacements. The
genuine, high-confidence engine misplacements are the original 8:
`isis, ldp, rsvpte, flowexport, mpls` (component->plugins) and
`sysrib, bfd, sysctl` (plugins->component).

### Three paths to the enforced rule

| Path | What the gate enforces | Allowlist | Precondition | When |
|------|------------------------|-----------|--------------|------|
| **C (crisp engine gate)** | ONLY: an `sdk.NewWithConn` engine must be in `component` (if depended) or `plugins` (if not). core/composition reported as advisory, not enforced. | none | none | now - immediately shippable, zero false positives, catches exactly the 8 cases |
| **A (engine gate + minimal allowlist)** | engine rule (enforced) + core/composition (enforced) with a small declared allowlist for framework/host (~8-12 dirs), each justified | small, reviewed | reuse generator discovery for is_plugin (fixes B-1) | after Phase 1 |
| **B (true zero-allowlist)** | full trichotomy enforced mechanically | none | extract bgp/iface/vpp/ike library subpkgs to core (B-2); unify plugin registration discovery (B-1); pick a home for framework/host - 4th `internal/host/` tier or fold to core (B-3) | large multi-spec effort BEFORE the gate |

**Recommendation:** Path C now (it is the part that is genuinely mechanical and
matches the original concern - keep new engines out of the wrong tier and fix the 8
known cases), with Path B's preconditions tracked as the route to extend enforcement
to core/composition later. Path A is the middle option if broader enforcement is
wanted before the B-2 extraction work is done.

-> Decision (CHOSEN: Path C): the Phase 1 gate enforces ONLY the crisp engine rule.
An `sdk.NewWithConn` engine MUST live in `internal/component/` if a feature depends
on it, else in `internal/plugins/`. This subset is fully mechanical and needs NO
allowlist. core vs composition vs host classification is REPORTED (advisory) by the
audit but NOT enforced. Path B (structural preconditions for full enforcement) is
tracked as future work, not gated now.

-> Constraint: regardless of path, the audit's "is a plugin" determination must come
from the generator's discovery, not a grep, or it will repeat blocker B-1. For Path
C specifically, "is an engine" = `sdk.NewWithConn` is sufficient and unambiguous;
the generator-discovery constraint applies when (later) extending to the full rule.

## Wiring Test (MANDATORY - NOT deferrable)

Umbrella-level wiring is the audit gate itself; per-move wiring lives in child specs.

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` | -> | placement audit enforces the engine rule (Path C) | `TestEnginePlacement` (Phase 1) |
| `scripts/dev/dep_audit.py --check` | -> | exit 2 on a misplaced `sdk.NewWithConn` engine; advisory print for core/composition | `dep_audit` self-test (Phase 1) |
| generator run after a move | -> | `all.go` regenerated at new paths | `plugin_imports_test.go` (existing) |
| relocated plugin loads from config | -> | registry starts engine on `ConfigRoots` | per-plugin functional test (child specs) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A canonical rule doc exists under `ai/rules/` | Defines the 3 tiers, the two axes, and the authoring rule; linked from `ai/rules/INDEX.md` and the CLAUDE.md Before-You table |
| AC-2 | `scripts/dev/dep_audit.py --check` run on a tree with a misplaced `sdk.NewWithConn` engine | Exits non-zero (Path C: engine rule), names the dir and its required destination (`component` if depended-on by a feature, else `plugins`) |
| AC-3 | `make ze-verify` on an engine-compliant tree | Placement audit passes (exit 0); on a tree with a misplaced engine it fails |
| AC-4 | Path C gate has **no allowlist** | The enforced engine rule needs none; core/composition/host are printed as advisory and never cause a non-zero exit |
| AC-5 | Each directory move (child specs) | Performed by a Python migration tool, not hand edits; `go build ./...` green; `bin/ze --plugins` inventory unchanged |
| AC-6 | After edge-out + platform-in phases | `dep_audit.py --check` reports zero engine-rule mismatches; `isis/ldp/rsvpte/flowexport/mpls` under `internal/plugins/`; `sysrib/bfd/sysctl` under `internal/component/` |
| AC-7 | Generator after each move | `scripts/codegen/plugin_imports.go --check` passes; `pluginDirs`/`rpcRoot` reflect new locations |
| AC-8 | New engine package created in the wrong tier (regression) | The audit gate fails CI, pointing the author to the rule doc |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnginePlacement` | `scripts/dev/dep_audit_test.py` or a Go gate test | engine-rule mismatch detection against a fixture tree (Path C) | |
| `TestAdvisoryNotEnforced` | same | a core/composition/host "misplacement" prints advisory but exits 0 | |
| `TestGeneratorDirsMatchTiers` | extend `plugin_imports_test.go` | `pluginDirs`/`rpcRoot` consistent with engine placements | |

### Boundary Tests
N/A (no numeric inputs).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| relocated-plugin-loads | `test/plugin/*.ci` (child specs) | config with `isis {}`/`ldp {}` etc. still loads the engine after the move | |
| plugin-inventory-stable | compare `bin/ze --plugins` before/after | no plugin added or dropped by a move | |

### Interop Tests
N/A for the umbrella (relocation only). Child specs that move a protocol engine
re-run that protocol's existing interop suite to prove no behavior change.

### Future
- Phase 5 (infra->core) test plan defined in its own spec.

## Files to Modify
- `scripts/dev/dep_audit.py` - add `--check`, tier classification, exceptions allowlist (Phase 1)
- `scripts/codegen/plugin_imports.go` - update `pluginDirs`/`rpcRoot` per move (Phases 2-4)
- `internal/component/plugin/all/all.go` - regenerated (Phases 2-4)
- `cmd/ze/ze_core_dispatch.go` - dispatch imports of moved packages (Phases 2-4)
- `ai/rules/INDEX.md`, `CLAUDE.md` (Before-You table) - link the new rule (Phase 1)
- `docs/plugin-overview.md`, `docs/architecture/plugin/plugin-relationships.md`, `ai/INSTRUCTIONS.md` (arch lists via `arch_map.py`) - updated inventories (Phases 2-4)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| New rule doc | [ ] yes | `ai/rules/architecture.md` + `ai/rules/INDEX.md` pointer |
| Verification gate | [ ] yes | `make ze-verify` wiring of the placement audit |
| Generator update | [ ] yes | `scripts/codegen/plugin_imports.go` |
| Docs/inventory | [ ] yes | `docs/plugin-overview.md`, arch lists |
| Discovery-updates | [ ] yes | `ai/rules/repo-maintenance.md` - register the gate + keyword in `ai/INDEX.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 5 | Plugin added/changed? | [ ] yes (relocated) | `docs/guide/plugins.md`, `docs/plugin-overview.md` |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/core-design.md`, `plugin-relationships.md` |
| 15 | Runtime inventory changed? | [ ] location only | `docs/plugin-overview.md`, `docs/features/plugins.md` |
| others | - | [ ] no | verified by grep for moved paths in docs/ |

## Files to Create
- `ai/rules/architecture.md` - the canonical tier-placement rule (Phase 1)
- `scripts/dev/migrate_module.py` - the deterministic Python restructuring tool: FS move + quoted-import rewrite + generator-list edit, dry-run by default (Phase 2)
- `scripts/dev/dep_audit_test.py` (or Go gate) - audit gate test (Phase 1)
- child specs `spec-tiers-1..5` as sequenced above

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Findings + Files sections - confirm current mismatches |
| 3. Wiring | Wiring Test - add the audit gate first |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify` (incl. new placement audit) |
| 14. Summary | Executive Summary |

### Implementation Phases
1. **Phase: Rule + audit gate (Phase 0/1, MANDATORY FIRST)** - write `ai/rules/architecture.md`; extend `dep_audit.py` with `--check`, classification, allowlist; add the Go/py gate test; wire into `make ze-verify`; add INDEX/CLAUDE/discovery-updates entries.
   - Tests: `TestModuleTierPlacement`, `TestExceptionsAllowlistHonored`
   - Verify: audit runs in verification and reports the known mismatches as failures (allowlisted until their move lands)
2. **Phase: Migration tool** - write `scripts/dev/migrate_module.py` (dry-run default): moves a dir between tiers, rewrites quoted import paths repo-wide, edits generator `pluginDirs`/`rpcRoot`, then invokes the generator.
   - Verify: dry-run on `mpls` (1 file) prints exact planned edits; real run builds green.
3. **Phase: Edge-out (child spec 2)** - move isis/ldp/rsvpte/flowexport/mpls; per move: tool run -> generator -> `go build ./...` -> functional test -> `dep_audit.py --check` -> remove its allowlist entry.
4. **Phase: Platform-in (child spec 3)** - move sysrib/bfd/sysctl symmetrically.
5. **Phase: Borderline (child spec 4)** - ike/mrt per-case.
6. **Phase: Infra->core (child spec 5)** - deferred; separate config first.
7. **Full verification** - `make ze-verify`; diff `bin/ze --plugins` across the whole reorg.
8. **Complete spec** - learned summary; two-commit closure per child spec.

### Failure Routing
| Failure | Route To |
|---------|----------|
| Build break after a move | the migration tool's rewrite step - fix path handling, re-run |
| `--plugins` inventory changed | generator not updated - fix `pluginDirs`/`rpcRoot` |
| Audit false positive | add justified allowlist entry or refine axis definition |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Path C gate = grep `sdk.NewWithConn` + dependency check; needs no generator discovery | At engine-package granularity this flags 41 dirs, ~33 false positives: nested sub-plugins under `bgp/plugins/*`, `firewall/plugins/irr` (correctly-placed sub-plugins) and grouping dirs like `plugins/iface` (engine is nested `iface/dhcp`). Distinguishing a top-level subsystem engine from a nested sub-plugin REQUIRES the generator's `pluginDirs`/discovery (blocker B-1). | implementation audit (engine-package probe), child spec 1 | Path C gate must reuse generator discovery to scope to top-level subsystems; the clean enforced set is 8 (isis/ldp/rsvpte/flowexport/mrt + bfd/sysctl/sysrib). Affects how the gate is built (Python reading `pluginDirs`, or a Go gate reusing the generator). Needs user decision before implementing. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The component/plugin boundary was never a capability difference - both register
  through the same registry. The real, machine-checkable distinction is dependency
  direction (does anything depend on this?). Encoding that as the rule turns an
  organizational convention into an enforceable invariant.
- The generator (`plugin_imports.go`) hardcodes the split via `pluginDirs`/`rpcRoot`;
  it is the load-bearing coupling that makes "just `git mv`" insufficient.

## Core Insight
Tier = dependency direction, made auditable. `core` cannot run as a plugin;
`component` is a plugin others depend on; `plugin` is an edge plugin nothing depends
on. The audit reads this straight off the import graph.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rule defined by two mechanical axes (engine? depended-on?) | prose-only guidance; size/role heuristic | only a mechanical rule can be enforced by code, which the user requires |
| Moves done by a Python tool | hand edits per file | deterministic, reproducible, reviewable; user-required |
| Umbrella + sequenced child specs | one mega-spec | config has 279 importers; infra->core is high-risk and must be isolated |
| Enforce engine rules first, defer infra->core | reorg everything at once | edge-out/platform-in are unambiguous and low-risk; infra->core needs config decoupling |

## Known Limitations
- The `core` tier classification is fuzzy for daemon host-services; Phase 1 handles
  them via an allowlist rather than forcing a move.
- `git mv` is forbidden from the Bash tool; the Python tool does filesystem moves and
  the user runs the commit script (staging stays user-controlled).

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| Boundary documented | rule file | `ai/rules/architecture.md` exists + linked in INDEX |
| Placement auditable by code | gate test | `dep_audit.py --check` non-zero on a planted misplacement; green on compliant tree |
| Restructure deterministic | tool + build | `migrate_module.py` dry-run output + green `go build ./...` per move |
| Boundary matches import graph | audit clean | `dep_audit.py --check` reports zero engine-rule mismatches after Phases 2-3 |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | (to be run via /ze-review before implementation closure) | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Placement audit wired into `make ze-verify`
- [ ] Rule doc linked from INDEX + CLAUDE Before-You table
- [ ] `make ze-test` passes
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
<!-- Umbrella: the executable tests live in the child specs (tiers-1..5); these
     items are satisfied per child, with output pasted in the child spec. -->
- [ ] Tests written (per child: `TestEnginePlacement`, `dep_audit.py --selftest`, `plugin_imports_test.go` extensions)
- [ ] Tests FAIL (per child -- paste output in the child spec before the gate/tool lands)
- [ ] Tests PASS (per child -- paste output in the child spec)
- [ ] Functional tests for end-to-end behavior (relocated-plugin-loads `.ci` + `bin/ze --plugins` inventory diff, per child)
