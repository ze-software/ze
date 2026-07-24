# Spec: feature-gate-12-remaining -- gate the remaining compile-out candidates

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-07-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/feature-gate-registration.md` - the gate procedure, shapes, and traps
4. `feature-gates.txt` - the manifest (single source of truth)
5. `scripts/dev/dep_audit.py` - the enforcement gate (`disableable_violations`)
6. `scripts/codegen/plugin_imports.go` - `parentTagOf` / `buildConstraint` (dependent gates)
7. `plan/learned/1249-feature-gate-10-bgp.md` - extract-then-gate at subsystem scale
8. `plan/learned/1251-feature-gate-11-bmp-mrt.md` - dependent-gate machinery

## Task

An audit (2026-07-22, this spec's research) compared every top-level subsystem
under `internal/component/` and `internal/plugins/` against the feature-gate
manifest. 16 tags exist (lg, ssh, web, gnmi, mcp, rest, grpc, telemetry, isis,
ldp, ospf, rsvpte, vrrp, bgp, bmp, mrt). The audit found ~20 more subsystems
that match the same criteria the existing gates were chosen by (network-facing
service, optional protocol, optional backend) but are still pinned into every
binary. Gate them all, so a `ze` build can be reduced to exactly the features a
deployment needs (smaller binary, smaller attack surface). All new tags are
default-on via `ZE_FEATURES` (derived from the manifest), so default builds and
the appliance image are unchanged.

### New gates, grouped by effort shape

**Group A -- pure blank-import partitioning (zero always-on pins; the ldp/vrrp
shape).** Sole composition root is the generated `all.go`. Gate = manifest
line(s) + `make generate` + build-tag tests. Audit method: `dep_audit.py`
`classify()` external-importer sets, minus `DISABLEABLE_NONPROD_PREFIXES`
(scripts/dev/dep_audit.py:213-224), all EMPTY for these.

| Tag | Package(s) | LOC (non-test) | What it is |
|-----|-----------|----------------|------------|
| `ze_flowexport` | `internal/plugins/flowexport`, `internal/plugins/flowexport-cmd` | 6,229 | NetFlow/IPFIX exporter |
| `ze_ddos` | `internal/plugins/ddos` | 4,792 | DDoS detection/mitigation |
| `ze_anomaly` | `internal/plugins/anomaly` | 1,970 | traffic anomaly detection |
| `ze_as112` | `internal/plugins/as112` | 1,877 | AS112 DNS responder (listener) |
| `ze_geodns` | `internal/plugins/geodns` | 1,327 | geo-aware DNS server (listener) |
| `ze_dhcpserver` | `internal/plugins/dhcpserver` | 1,642 | DHCP server (listener) |
| `ze_pxe` | `internal/plugins/tftpserver`, `internal/plugins/imageserver` | 1,515 | netboot pair: TFTP + image HTTP server (both from the install spec family, learned 807/811). Shared tag per user decision 2026-07-22 |
| `ze_ntp` | `internal/plugins/ntp` | 1,097 | NTP server |
| `ze_trafficusage` | `internal/plugins/trafficusage` | 1,555 | per-host traffic accounting |
| `ze_policyroute` | `internal/plugins/policyroute` | 1,107 | policy routing engine |
| `ze_cos` | `internal/plugins/cos` | 674 | class-of-service engine (NOTE: has an l2tp import, see Group D coupling) |
| `ze_copp` | `internal/plugins/copp` | 550 | control-plane policing |
| `ze_mpls` | `internal/component/mpls`, `internal/plugins/mpls-cmd` | 241 | MPLS show-forwarding component. Inconsistency today: ldp/rsvpte are gated, this is not. Sole importer is `internal/component/plugin/all/all.go:194` |

**Group B -- extra composition roots, still no functional pins.**

| Tag | Package(s) | LOC | Extra roots to gate |
|-----|-----------|-----|---------------------|
| `ze_tacacs` | `internal/component/tacacs` | 2,207 | AAA method provider. Registration roots: `internal/component/aaa/all/all.go:15` (blank import), `internal/component/plugin/all/all.go`, `cmd/ze/ze_core_dispatch.go`. Needs a hand-gated sibling in `aaa/all/` and a gated dispatch companion. Only other importer is the test mock (`internal/test/mock/tacacs`), excluded by `DISABLEABLE_NONPROD_PREFIXES` |
| `ze_exabgp` | `internal/plugins/exabgp` (+ most of `internal/exabgp`, 4,451 LOC) | 1,027 | ExaBGP bridge plugin. Imported from a `cmd/ze` root (dispatch companion needed, the isis/ospf shape). `internal/component/config/migration/listener.go` imports only the small `internal/exabgp/topics` leaf (verified by reading listener.go imports), which stays always-on for `ze config migrate`/evolve |

**Group C -- inversion seam already exists (`ze_bfd`).**

`internal/component/bfd` (6,582 LOC, engine, UDP listener). The client contract
is ALREADY the nil-able seam this gate needs: `bfd/api` `SetService`/`GetService`
use an atomic pointer, and every consumer is documented to handle nil ("BGP,
OSPF, and other clients run without BFD", registry.go:55-59; producer
`GetService` internal/component/bfd/api/registry.go:62-68). All cross-tree
consumers import `bfd/api`, not the engine: `plugins/static/register.go:12`,
`plugins/static/inject.go:13`, `plugins/ospf/bfd_client*.go`, `plugins/ospf/doctor.go`,
`bgp/reactor/peer_bfd.go`, `bgp/reactor/peersettings.go`. Design: keep `bfd/api`
UNGATED (leaf contract, imports only sync/atomic), gate the engine subtree
(`bfd`, `bfd/engine`, `bfd/session`, `bfd/transport`, `bfd/packet`, `bfd/cmd`,
yang), and move the one always-on schema pin -- the `bfd/cmd` blank import at
`internal/component/config/yang/cli/tree.go:24` -- into a gated file.

**Group D -- extract-then-gate (real seam work; the ze_bgp playbook).**

| Tag | Package(s) | LOC | Always-on pins to clear |
|-----|-----------|-----|-------------------------|
| `ze_vpp` | `internal/component/vpp` + the per-plugin vpp backend sub-packages (`plugins/fib/vpp`, `plugins/firewall/vpp`, `plugins/iface/vpp`, `plugins/traffic/vpp`, `plugins/static/vpp`, `ike/dataplane` vpp file) | 2,497 own; drops vendored govpp 45,712 LOC | 8 backend files across fib/firewall/iface/traffic/static/ike. All are backend registration files that share the tag (multi-package feature, the ze_isis shape). `plugins/static/backend_vpp_linux.go` sits inside ungated static and needs a tag or a move into `static/vpp` |
| `ze_ike` | `internal/component/ike` | 13,430 | 3 pins: hub main.go:37-38 (BLANK imports only -- moves to a gated registration file trivially), `web/page_vpn_ipsec.go` (gated-file-for-a-different-tag trap: needs `ze_web && ze_ike`), `plugins/ospf/ipsec_install.go` (needs `ze_ospf && ze_ike` or a seam) |
| `ze_l2tp` | `internal/component/l2tp` (incl. pppoe, subscriber, plugins/*) | 30,602 | Largest ungated subtree. 6 pins: hub `main.go:542-553` (`l2tp.ExtractParameters` + `NewSubsystem` + web portal entry) and `main.go:558-564` (pppoe) -- needs the subsystem-registration seam or registry shape; `web/handler_l2tp.go` + `web/page_l2tp.go` (compound `ze_web && ze_l2tp`); `plugins/cos/handler.go:106` (`l2tp.LoadSessionMetadata`); `plugins/diag/cmd/capture.go:44` + `capture_raw.go` (`l2tp.LookupService`). The cos/diag consumers use lookup-style accessors -- whether they are already nil-safe seams is UNVERIFIED (A-6) |
| `ze_radius` | `internal/component/radius` | 1,444 | AAA method registration already via `internal/component/aaa/all/all.go:13` (registration root). Functional pins are all `l2tp/plugins/authradius/*` (8 files). Gate WITH or AFTER `ze_l2tp`: manifest line `ze_radius internal/component/l2tp/plugins/authradius` makes authradius a dependent gate -- `parentTagOf` derives `ze_l2tp` from path nesting and `buildConstraint` emits `ze_l2tp && ze_radius` (scripts/codegen/plugin_imports.go:608-626, :630-640). No generator change needed |

### Explicitly NOT in scope (platform, stays always-on)

`config`, `plugin`, `command`, `cli`, `cmd`, `hub`, `engine`, `doctor`,
`authz`, `aaa` (framework), `iface`, `firewall`, `traffic` (frameworks with
30-80 external importers each), `resolve` (pinned by hub/web/bgp/firewall),
`pki` (hub TLS + ike), `sysctl`/`sysrib` (FIB backbone), `host`, `storage`,
`support`, `managed`, `ping`/`traceroute` (CLI session factory),
`internal/plugins/provision`, `internal/plugins/connect` (small, cmd/ze root
imports, low value -- revisit later if wanted).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/feature-gate-registration.md` - the whole procedure, both shapes, all traps
  → Constraint: `feature-gates.txt` is the ONLY declaration point; every consumer derives
  → Constraint: a gated file is still an always-on pin for a DIFFERENT gate (per-tag check `file_requires_tag`, dep_audit.py:258-270)
  → Constraint: removing an always-on import can unlink an `init()` nobody else pulls in -- ask what each deleted import's init() provided
- [ ] `ai/rules/module-tiers.md` - disable-ability axis, `ze-tier-check`
  → Constraint: an engine's blank import in generated `all_<tag>.go` is registration, not a feature dependency
- [ ] `plan/learned/995-feature-gate-8-protocols.md` - blank-import partitioning for plugins, dispatch companions
  → Decision: protocols with a programmatic `cli` package need a gated `cmd/ze/dispatch_<x>.go`; CLIHandler-registered plugins need only the generated all.go root
- [ ] `plan/learned/1249-feature-gate-10-bgp.md` - the three clearing techniques in preference order
  → Decision: transitive drop (no tag) > core-leaf move (no tag) > inversion seam (no tag on always-on side)
- [ ] `plan/learned/1251-feature-gate-11-bmp-mrt.md` - dependent gates
  → Constraint: `parentTagOf` is path-nesting only; `ze_radius` on `l2tp/plugins/authradius` fits it, nothing else here does
- [ ] `plan/learned/983-feature-gate-manifest-ssot.md` - why no consumer is hand-maintained

### RFC Summaries (MUST for protocol work)
- None. This spec moves packages behind build tags; no wire behavior changes.
  RFC-tagged tests in gated packages (e.g. tftpserver RFC 2348/2349) must keep
  running in the default full-feature test build -- `TestBuildTags` derives from
  the manifest, so they do.

**Key insights:**
- Adding a gate is a one-line manifest edit + `make generate` + tests, UNLESS an always-on file imports the package; then extract-then-gate first.
- `dep_audit.py disableable_violations` (scripts/dev/dep_audit.py:290-308) is the enforcement; it checks the EXACT package paths listed in the manifest, so every package that must vanish needs its own line.
- The absent-build proof is `go tool nm` finding zero feature symbols; the present-build proof is the existing functional suites (runner derives tags from the manifest).

## Current Behavior (MANDATORY)

**Source files read:** (read during this spec's research, 2026-07-22)
- [ ] `feature-gates.txt` - 16 tags, ~80 lines; header documents every consumer and the derive/generate split
- [ ] `scripts/dev/dep_audit.py` - `load_feature_gates` (:227-248), `disableable_violations` (:290-308), `is_registration_importer` (:76-99: `*/all/all.go`, `all_<tag>.go` beside `plugin/all`, `cmd/ze` dispatch/setup files), `DISABLEABLE_NONPROD_PREFIXES` (:213-224)
- [ ] `scripts/codegen/plugin_imports.go` - `parentTagOf` (:608-626) path-nesting dependent gates; `buildConstraint` (:630-640) emits `parent && tag`
- [ ] `cmd/ze/hub/service_registry.go` - `Service` interface (:36-40), `ServiceDeps` generic-values-only contract (:47-77)
- [ ] `cmd/ze/hub/main.go` - ike blank imports (:37-38), l2tp subsystem wiring (:542-553), pppoe (:558-564)
- [ ] `internal/component/bfd/api/registry.go` - nil-able atomic Service seam (`SetService` :47-53, `GetService` :62-68); doc contract "callers MUST handle nil" (:15-17, :55-59)
- [ ] `internal/plugins/static/register.go:12`, `inject.go:13` - import `bfd/api` only (aliased `bfdapi`), not the engine
- [ ] `internal/component/config/yang/cli/tree.go:24` - blank import of `bfd/cmd` (command schema pin)
- [ ] `internal/component/aaa/all/all.go` - blank imports radius (:13) and tacacs (:15): AAA method composition root
- [ ] `internal/plugins/cos/handler.go:106` (`l2tp.LoadSessionMetadata`), `internal/plugins/diag/cmd/capture.go:44`, `capture_raw.go:65,97,130` (`l2tp.LookupService`) - consumers only; producers NOT yet read (A-6)
- [ ] `internal/component/config/migration/listener.go` - imports `internal/exabgp/topics` only
- [ ] `docs/guide/quickstart.md:22` - generated `go install -tags '...'` line (regenerated by `feature_tags.go`)
- [ ] `internal/plugins/tftpserver/register.go`, `internal/plugins/imageserver/register.go` - both plugin-registry registrations, all.go-wired

**Behavior to preserve:**
- Default builds unchanged: every new tag lands in `ZE_FEATURES` (derived), the appliance `GoBuildTags` (generated), the lint build (generated), and `TestBuildTags` (derived). Full-feature `make ze-verify` must stay green with zero functional-test edits.
- BGP/OSPF/static behavior with BFD absent is ALREADY defined (nil `GetService` = no BFD wiring); preserve exactly.
- `ze config migrate` (ExaBGP migration) keeps working in every build -- only the bridge plugin is gated.
- The `Reconfigurable`/listener-migration contract for existing services unchanged.

**Behavior to change:**
- A build without a tag: the feature's packages link zero symbols; its YANG roots are unknown keys, so a config using the feature is REJECTED at validate/commit (exact-or-reject, same as the existing protocol gates).
- The hub's l2tp/pppoe/ike construction moves from direct always-on imports to gated registration (seam or service registry).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

This spec changes BUILD-time composition, not runtime data flow.

### Entry Point
- `feature-gates.txt` manifest lines (`<tag> <pkg>`).

### Transformation Path
1. `make generate`: `plugin_imports.go` moves each gated package's blank import from `all.go` into `all_<tag>.go` (compound constraint via `buildConstraint` for dependent gates); `feature_tags.go` regenerates `.golangci.yml` build-tags, `gokrazy/ze/config.json` GoBuildTags, `docs/guide/quickstart.md` install line.
2. `Makefile` `ZE_FEATURES` and `internal/test/runner` `TestBuildTags` re-derive at build/test time (no generation step).
3. Build without a tag: no blank import compiled, no registration `init()` runs, linker drops the subtree (proof: `go tool nm`).
4. `dep_audit.py --check` (in `ze-tier-check`) fails any future always-on import of a gated package.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| hub ↔ l2tp/pppoe/ike | today direct calls / blank imports; after: gated registration (seam var or service registry), generic values only | [ ] |
| cos/diag ↔ l2tp | today direct lookup calls; after: always-on contract leaf or compound-tagged consumer files (decide in phase F design, pending A-6) | [ ] |
| static/ospf/bgp ↔ bfd | `bfd/api` nil-able seam (already exists, stays ungated) | [ ] |
| plugins ↔ vpp backends | per-backend sub-packages join `ze_vpp` (intra-feature imports, dep_audit same-tag skip) | [ ] |
| aaa ↔ radius/tacacs | `aaa/all` composition root gains gated sibling files | [ ] |

### Integration Points
- `cmd/ze/hub/service_registry.go` - the construction registry for listener-shaped services.
- `internal/component/plugin/all/` - generated composition root; all Group A work lands here via `make generate`.
- `cmd/ze/ze_core_dispatch.go` + gated `cmd/ze/dispatch_<x>.go` companions - offline CLI roots (tacacs, exabgp).

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Group A plugins have NO always-on non-test importer outside their subtree | dep_audit `classify()` external sets computed 2026-07-22 (script over `collect_edges`) | that plugin moves to Group B/D; extract first | re-run the audit script per phase; `dep_audit.py --check` after gating | confirmed (2026-07-23 fresh audit run: all Group A blockers=0) |
| A-2 | Group A plugins have no `cmd/ze` root import (all.go is the only root) | grep of `cmd/ze/*.go` plugin imports 2026-07-22: only ospf, isis, systemd, support, skills, signal, provision, passwd, local, init, host, explain, exabgp, diag, debug, crashes, connect, completion | that plugin needs a gated dispatch companion | per-phase grep of `cmd/ze` for the package path | confirmed (2026-07-23 grep of cmd/ze + cmd/ze/*/: zero Group A imports) |
| A-3 | `bfd/api` imports nothing from the bfd engine subtree (leaf contract, safe to leave ungated) | registry.go doc ":9-10 a leaf package with no runtime dependencies"; imports seen: sync/atomic | need a core-leaf move of the contract instead | read all `bfd/api/*.go` import blocks | **broken** (2026-07-23: api/events.go:16 imports `bfd/packet` for State/Diag re-exports :22-45; api/snapshot.go:15 imports `component/plugin`). Recovery: `bfd/packet` is a ~500-line pure-stdlib codec (zero ze imports) -- leave BOTH api and packet UNGATED; gate root/engine/session/transport/cmd/yang. Mistake Log row added |
| A-4 | All BFD consumers already handle nil `GetService` (daemon correct with bfd absent) | api/registry.go:15-17, :55-59 contract; producer GetService :62-68 | absent-build boot test fails; add nil guards at consumers | absent-build functional boot + grep each `GetService` call site for nil check | confirmed (2026-07-23: static/register.go:240, ospf/bfd_client.go:193-200, ospf/doctor.go:135, bgp/reactor/peer_bfd.go:66-71 all nil-check with warn + degrade) |
| A-5 | ike's only hub coupling is the two blank imports (main.go:37-38); no reload-path calls | grep `ike\.` over cmd/ze/hub 2026-07-22 matched only the import lines | ike needs a seam like gnmi_infra, not just a moved blank import | grep hub for `ike` symbols + read `main_reload.go` | confirmed (2026-07-23 grep `ike` over cmd/ze/hub non-test: only main.go:37-38 blank imports) |
| A-6 | `l2tp.LookupService` / `LoadSessionMetadata` are nil-safe lookup seams (consumers verified, producers NOT read) | consumer call shapes in cos/diag | phase F needs a contract extraction to an always-on leaf (events + metadata + capture types) | read the producers in `internal/component/l2tp` before phase F design | confirmed (2026-07-23: `LookupService` service_locator.go:74-80 atomic nil-able; `LoadSessionMetadata` session_metadata.go:72-79 nil on miss. Runtime absence is safe; phase 7 still needs the compile-time contract extraction) |
| A-7 | `ze_radius` on `l2tp/plugins/authradius` yields `ze_l2tp && ze_radius` automatically | parentTagOf path-prefix rule read at the producer (plugin_imports.go:608-626); buildConstraint :630-640 | extend the generator with declared (non-path) dependencies | inspect generated `all_ze_radius.go` constraint in phase 7 | confirmed-by-reading (final proof: generated file in phase 7) |
| A-8 | No functional `.ci` suite requires a feature to be ABSENT (all suites run full-feature) | TestBuildTags derives all manifest tags | a suite breaks when a tag lands; adjust runner | full `make ze-verify` after each phase | unvalidated (validated per phase by the full verify) |
| A-9 | Gating exabgp keeps `ze config migrate` working: migration code lives in `internal/exabgp/migration` + `internal/component/config/migration`, neither in the gated plugin | listener.go imports only `internal/exabgp/topics`; migration importers list (cmd_migrate.go, editor_commit.go, bgp loader, main_evolve.go) | migration breaks in absent build; keep migration subtree ungated and gate only `internal/plugins/exabgp` | absent-build run of `ze config migrate` on an ExaBGP config | confirmed (2026-07-23 grep: only non-plugin importer of `internal/exabgp/*` is `config/migration/listener.go`, which imports `internal/exabgp/topics` only) |
| A-10 | mpls component's only importer is `all.go:194` | grep 2026-07-22 | needs clearing like the others | `dep_audit.py --check` | confirmed (2026-07-23 grep: sole non-test importer is `internal/component/plugin/all/all.go`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An unlinked `init()`: removing an always-on import (hub ike/l2tp) drops a registration nobody else pulls in (the bgp/config trap, learned 1249) | absent-or-present build: feature configured but silently inert; functional suite red | after each import removal, enumerate what the package's `init()` registered; link from the gated dispatch/registration file. OUTCOME: every removed import was a side-effect blank import (or direct construction) whose registration moved WITH the gated file; present tests prove the registrations still fire under full tags |
| R-2 | Web/ospf files importing ike/l2tp are per-tag violations even though those files are gated for ANOTHER tag (`file_requires_tag` is per-tag) | `dep_audit.py --check` red on `ze_web`/`ze_ospf` files | compound constraints on those files, or seam the dependency. OUTCOME: materialized as predicted; resolved with source tags + not-in-this-build stubs (web) and the always-on `ike/dataplane` seam (ospf); dep_audit green |
| R-3 | cos imports l2tp: gating `ze_cos` in Group A while l2tp is still ungated is fine, but once `ze_l2tp` lands, cos's l2tp import becomes a cross-gate pin | dep_audit red at phase F | OUTCOME: materialized; resolved by dependent-file tagging (`handler.go` = `//go:build ze_l2tp` + no-op stub), not contract extraction |
| R-4 | 20 new tags double the manifest; tag-combination space explodes and absent-build tests multiply CI time | slow `ze-verify`; flaky nm tests | OUTCOME: one consolidated bare-core build + nm covers all 20 tags' needles (plus one ze_bgp-without-ze_bfd build); registration/config checks are in-process and cheap |
| R-5 | Appliance image regressions: gokrazy GoBuildTags regenerates to include all new tags; a mistake drops a feature from the shipped image | `TestGokrazyConfigMatchesApplianceBuildTags` red; appliance boot test | OUTCOME: `make generate` regenerated the tag list; drift test runs inside the full verify (see Pre-Commit Verification) |
| R-6 | vpp gating splits files INSIDE ungated plugins (static/backend_vpp_linux.go): tagging single files inside an untagged package risks U1000 unused-symbol lint in no-tag builds | golangci red in either build | OUTCOME: stub counterparts keep every symbol defined in both build modes; `make ze-lint-changed` green |
| R-7 | The l2tp/pppoe hub wiring (ExtractParameters/NewSubsystem/RegisterSubsystem) is construction with config parsing, not a listener service; the service registry may not fit and a new seam shape is needed | phase F design stalls | OUTCOME: confirmed; the `bngRegister` nil-able seam (bng_infra.go, ssh_infra shape) carries generic values only |
| R-8 | (found during implementation) The per-tag dependent-gate machinery compound-gates a MIXED tag's independent packages (ze_radius schema lost in radius-without-l2tp builds) | tier-check red / absent schema in a valid build | OUTCOME: generator extended to per-package constraints with mixed-tag file splitting; ze_bmp output byte-stable |

## Wiring Test (MANDATORY — NOT deferrable)

For a compile-out gate, "wiring" is: (present) the feature still registers and
serves through its normal entry point; (absent) zero symbols link and config
using the feature is rejected. One row per phase; per-tag tests named in the
TDD plan.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| default build (all tags) + existing `.ci` suites | → | every gated plugin's `register.go` init via `all_<tag>.go` | full `make ze-verify` (all functional suites, unchanged) |
| `ze_core`-only build | → | (nothing: linker drops gated subtrees) | `TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (needles for all 20 subtrees + govpp) |
| full-tags build registration | → | plugin-registry entries + seams filled | `TestBuildTag_Gate12GroupA_Present`, `..GroupA_MPLSPresent`, `..GroupB_Present`, `TestBuildTag_{NTP,BFD,IKE,VPP,L2TP}_Present` |
| bare-build registry + `ze config validate` on gated roots | → | unknown-key rejection (schema package gated) | `TestBuildTag_Gate12GroupA_Absent{,RejectsConfig}`, `..GroupB_Absent{,RejectsConfig}`, `TestBuildTag_{NTP,BFD,IKE,VPP,L2TP}_Absent*` |
| hub startup, `ze_l2tp`/`ze_ike` on | → | `bngRegister` seam (register_l2tp.go) / register_ike.go blank imports | `TestBuildTag_L2TP_Present` (seam non-nil) + l2tp/ike functional suites in verify |
| BGP with `ze_bfd` off | → | nil `GetService` path (api/registry.go:62-68) | gate12 nm test's `ze_core,ze_bgp` build (reactor present, bfd engine absent); consumer nil-checks read at producers |
| iface/dataplane backend selection, `ze_vpp` off | → | fail-closed unknown-backend rejection (iface backend.go:283-290; dataplane.go:178-181) | `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed`, `TestVPPBackendAbsent` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `feature-gates.txt` after each phase | Every package listed in the Task tables has a manifest line; `make generate` output is byte-stable on re-run; no hand-edit to any derived/generated consumer (`feature_tags --check`, `dep_audit.py --check`, gokrazy test all green) |
| AC-2 | For each new tag T: build `ze_core` + all tags EXCEPT T | `go tool nm` finds zero symbols from T's gated packages (absent test per tag) |
| AC-3 | Default full-feature build | `make ze-verify` green with no functional-test edits (A-8) |
| AC-4 | For each new tag T: absent build + a config using T's YANG root | `ze config validate` rejects with unknown-key error (exact-or-reject) |
| AC-5 | `ze_bfd` off; BGP peer + OSPF neighbor + static route config WITHOUT bfd blocks | daemon boots, sessions establish, no panic, no BFD wiring attempted (nil-seam path) |
| AC-6 | `ze_vpp` off; netlink-backend config for iface/fib/firewall/traffic | boots and applies via netlink; a config selecting the vpp backend is rejected at verify |
| AC-7 | `ze_l2tp`/`ze_ike` off | hub starts without the subsystems; web (if on) shows no l2tp/ipsec pages; diag capture and cos degrade per their existing no-service path (exact behavior fixed after A-6) |
| AC-8 | `ze_l2tp` on, `ze_radius` off | authradius group file constraint is `ze_l2tp && ze_radius`; no radius symbols link; l2tp local auth works |
| AC-9 | `ze_exabgp` off | `ze config migrate` on an ExaBGP config still works (A-9); the bridge plugin is absent |
| AC-10 | `ze_tacacs` off | aaa runs with remaining methods; no tacacs symbols; `aaa/all` gated sibling carries the blank import |
| AC-11 | Appliance | `gokrazy/ze/config.json` GoBuildTags regenerated to include all new tags; appliance drift test green |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds a BGP-only edge router: `go install -tags 'ze_core ze_bgp'` | manifest-derived tags -> no gated blank imports -> linker drops ~20 subsystems | absent nm tests (per tag) + a compile of that exact tag set in CI (extend the existing bare-`ze_core` compile check) |
| 2 | Runs the default build unchanged | ZE_FEATURES derives every tag -> identical composition to today | full `make ze-verify` |
| 3 | Deploys the appliance image | generated GoBuildTags carries all tags -> image identical in features | `TestGokrazyConfigMatchesApplianceBuildTags` + appliance suites |
| 4 | Feeds an l2tp config to an l2tp-less build | schema gated -> unknown key at validate | AC-4 representative checks |
| 5 | Runs BGP+BFD build with bfd config | bfd engine registers, SetService publishes, BGP wires BFD | existing bfd functional suites (present build) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `build_tag_<x>_present_test.go` (one per new tag; 20 files) | `cmd/ze/hub/` | tag-on build compiles the feature and (where applicable) its factory/seam registers | |
| `build_tag_<x>_absent_test.go` (one per new tag) | `cmd/ze/hub/` | nm proof: zero feature symbols without the tag | |
| generator constraint test extension | `scripts/codegen/` (existing `feature_tags --check` harness) | `all_ze_radius.go` carries `ze_l2tp && ze_radius`; `all_ze_pxe.go` carries both packages | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none -- no numeric inputs; build-tag composition only) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing full suites, re-run per phase | `test/**` | default build behavior unchanged | |
| absent-build config rejection (representative per phase, pattern from feature-gate-8 absent tests) | `cmd/ze/hub/build_tag_*_absent_test.go` or `test/parse/` equivalent | AC-4 | |
| bfd-absent boot check | with phase C absent test | AC-5 | |

### Interop Tests (MANDATORY for protocol features)
Not required: no wire-visible change; existing interop suites run in the
default full-feature build and prove present-build behavior (per
`ai/rules/interop-and-goal-validation.md`, "pure internal refactor, no
wire-visible change").

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `feature-gates.txt` - ~30 new lines across 20 tags (each package that must vanish gets a line)
- `internal/component/plugin/all/` - regenerated (`make generate`): new `all_ze_<x>.go` files, slimmer `all.go`
- `.golangci.yml`, `gokrazy/ze/config.json`, `docs/guide/quickstart.md` - regenerated tag lists (never hand-edited)
- `cmd/ze/hub/main.go` - remove ike blank imports (:37-38) and l2tp/pppoe direct wiring (:542-564) in favor of gated registration
- `internal/component/config/yang/cli/tree.go` - `bfd/cmd` blank import (:24) moves to a gated file
- `internal/component/aaa/all/all.go` - radius (:13) and tacacs (:15) blank imports move to gated siblings
- `cmd/ze/ze_core_dispatch.go` - tacacs and exabgp imports move to gated dispatch companions
- `internal/plugins/cos/handler.go`, `internal/plugins/diag/cmd/capture*.go` - l2tp coupling resolution (phase F, after A-6)
- `internal/component/web/page_vpn_ipsec.go`, `web/handler_l2tp.go`, `web/page_l2tp.go` - compound constraints or registry-based page registration (phases E/F)
- `internal/plugins/ospf/ipsec_install.go` - compound constraint or seam (phase E)
- `internal/plugins/static/backend_vpp_linux.go` (+ siblings) - vpp code consolidated under `static/vpp` (phase D, R-6)
- `internal/component/ike/dataplane/` - vpp file joins the tag or splits into a sub-package (phase D)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | gating only; schemas move behind tags via generated `<pkg>/yang` gating |
| YANG validation constraints | [ ] No | none added |
| CLI commands/flags | [ ] No | none added; dispatch companions relocate existing registrations |
| Functional test for new RPC/API | [ ] No | no new RPC |
| Pipe completeness | [ ] No | no new output command |
| Env var registration | [ ] No | none |
| Doctor check for runtime dependencies | [ ] No | doctor checks travel with their gated plugins (DoctorChecks registration) |
| Prometheus counters/metrics | [ ] No | counters travel with gated packages; collection API stays always-on (telemetry precedent) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` (compile-out list mentions gates; extend), `docs/guide/quickstart.md` (regenerated) |
| 2 | Config syntax changed? | [ ] No | rejection of gated roots is existing exact-or-reject behavior |
| 3 | CLI command added/changed? | [ ] No | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | availability per build documented via #1 |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/ubuntu-build-install.md` and any page listing build tags (grep `ze_lg\|ZE_FEATURES` in docs) |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented/changed? | [ ] No | RFC-tagged tests keep running full-feature |
| 10 | Test infrastructure changed? | [ ] No | TestBuildTags derives; no format change |
| 11 | Affects daemon comparison? | [ ] No | feature set unchanged in default build |
| 12 | Internal architecture changed? | [ ] Yes | `ai/rules/feature-gate-registration.md` if a new shape emerges (subsystem-builder seam, phase F); update the tag examples list |
| 13 | Route metadata keys added/changed? | [ ] No | - |
| 14 | Prometheus counters added/changed? | [ ] No | - |
| 15 | Registered plugin/event/command inventory changed? | [ ] No | inventory is per-build; default unchanged |
| 16 | Changed files referenced by doc source anchors? | [ ] Check | grep `docs/` for anchors on hub main.go, tree.go, aaa/all at implementation time |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] Check | build-tag examples in quickstart/install guides |

## Files to Create
- `feature-gates.txt` additions (see Files to Modify)
- `cmd/ze/hub/build_tag_<x>_present_test.go` + `_absent_test.go` per new tag (~40 test files, following the existing `build_tag_*` pattern)
- `cmd/ze/hub/register_ike.go` (gated blank-import/registration home, `//go:build ze_ike`)
- Hub gated registration for l2tp/pppoe (shape decided in phase F design: seam var or subsystem-builder registry)
- `cmd/ze/dispatch_tacacs.go`, `cmd/ze/dispatch_exabgp.go` (gated dispatch companions, `//go:build ze_core && ze_<x>`)
- `internal/component/aaa/all/all_ze_radius.go`, `all_ze_tacacs.go` (hand-gated siblings; generator does not manage `aaa/all`)
- Gated home for the `bfd/cmd` schema blank import (beside `config/yang/cli/tree.go` or in the bfd plugin's own gated registration)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — per-tag present/absent tests first |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure |

### Implementation Phases

Each phase is separately committable and ends with a Self-Critical Review +
`make ze-verify`. Order: validate the pattern on one small gate, then sweep,
then the hard seams. Update the Phase field (N/7) as phases complete.

1. **Phase 1: Pattern validation on ONE gate (`ze_ntp`)** — smallest Group A
   plugin; manifest line, `make generate`, present/absent tests, absent-build
   config rejection check. Proves the sweep recipe end-to-end before scaling
   (iteration workflow: one change, one test, then scale).
   - Tests: `build_tag_ntp_present_test.go`, `_absent_test.go`
   - Verify: AC-1..AC-4 for ntp; full `ze-verify`
2. **Phase 2: Group A sweep** — flowexport(+cmd), ddos, anomaly, as112,
   geodns, dhcpserver, pxe (tftpserver+imageserver), trafficusage, policyroute,
   cos, copp, mpls(+mpls-cmd). Re-run the A-1/A-2 audit per plugin BEFORE its
   manifest line (assumptions validated per item, not in bulk).
   - Tests: per-tag present/absent pairs
   - Verify: AC-1..AC-4 for each; full `ze-verify`; note the cos->l2tp coupling (R-3) in the phase review
3. **Phase 3: Group B (`ze_tacacs`, `ze_exabgp`)** — gated `aaa/all` sibling +
   dispatch companions; validate A-9 with an absent-build `ze config migrate` run.
   - Tests: per-tag pairs + AC-9/AC-10 checks
4. **Phase 4: `ze_bfd`** — validate A-3/A-4 first (read `bfd/api` imports, grep
   `GetService` call sites); manifest lines for the engine subtree (api stays
   ungated); move the `tree.go:24` schema pin; absent-build boot test (AC-5).
5. **Phase 5: `ze_vpp`** — consolidate stray vpp files into per-plugin `vpp/`
   sub-packages (R-6), manifest lines for all vpp packages under one tag,
   dep_audit same-tag skip covers intra-feature imports; AC-6.
6. **Phase 6: `ze_ike`** — validate A-5; move hub blank imports to
   `register_ike.go`; compound-tag or seam the web page and ospf ipsec pins (R-2).
7. **Phase 7: `ze_l2tp` + `ze_radius`** — read the l2tp lookup producers (A-6)
   and DESIGN the hub subsystem seam (R-7) before edits; clear web/cos/diag
   pins; authradius dependent gate (A-7, AC-8). Largest phase; expect the
   extract-then-gate playbook from learned 1249.
8. **Full verification** → `make ze-verify` + appliance drift test (AC-11)
9. **Complete spec** → audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; every Task-table package has a manifest line |
| Feature completeness | Every End-to-End User Story path works; default build byte-identical feature set |
| Correctness | No unlinked `init()` after import removals (R-1): enumerate registrations per removed import |
| Naming | Tags `ze_<feature>`, lower snake; `ze_pxe` covers both netboot packages |
| Data flow | No feature type in an always-on signature; ServiceDeps stays generic values |
| Registration over hardcoding | Gated construction via registry/seam only; no always-on switch on features |
| Rule: no-layering | Old direct wiring DELETED in the same phase that adds the gated path |
| Rule: fail-closed | Absent-build config using a gated root is REJECTED, never silently ignored |
| Rule: hook-friction | Any new shape (subsystem-builder seam) documented in `ai/rules/feature-gate-registration.md` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Manifest lines for all 20 tags | read `feature-gates.txt`; count against Task tables |
| Generated files regenerated | `make generate` then `git diff --stat` shows only expected files; `feature_tags --check` green |
| Present/absent tests per tag | ls `cmd/ze/hub/build_tag_*` and map to tag list |
| dep_audit green | `python3 scripts/dev/dep_audit.py --check` (via `make ze-tier-check`) |
| Absent nm proof per tag | run the absent tests; paste one representative output |
| Default build unchanged | full `make ze-verify` output |
| Appliance tags | gokrazy drift test green |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Attack-surface claim honest | absent builds genuinely drop listeners: nm shows no tftp/dhcp/dns/l2tp/ike handler symbols; no stray always-on listener remains |
| Fail-closed config | gated-feature config rejected in absent builds (AC-4), never silently accepted then unserved |
| Auth methods | `ze_tacacs`/`ze_radius` off: aaa must fail closed for users configured with a missing method, not fall through to another method silently (verify against `authz` fail-closed rules) |
| No secret-bearing code un-gated by accident | radius/tacacs shared-secret handling stays inside gated packages |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure (U1000 in a tag combination) | R-6 mitigation: whole-package tagging over single-file tagging |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| dep_audit red on a gated file for another tag | R-2: compound constraint or seam |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-3: `bfd/api` is a pure leaf ("no runtime dependencies" per its own doc, registry.go:9-10) | api/events.go:16 imports `bfd/packet` (State/Diag re-exports) and api/snapshot.go:15 imports `component/plugin` | reading the import blocks during the step-3 assumption validation | design adjusted, not invalidated: `bfd/packet` (~500-line pure-stdlib codec) stays ungated beside `bfd/api`; the manifest gates only root/engine/session/transport/cmd/yang. The stale doc comment in registry.go gets corrected in phase 4 |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- `bfd/api` was built as a nil-able seam from day one (registry.go doc), which makes `ze_bfd` far cheaper than its consumer count suggests: the seam work the ze_bgp gate had to invent already exists here.
- The dependent-gate machinery (parentTagOf) generalizes to `ze_radius` with zero generator changes because authradius happens to live under the l2tp path.

## Core Insight
The audit shows the remaining gating cost is concentrated in exactly two
places: the hub's direct subsystem construction (l2tp/pppoe, ike) and
cross-plugin convenience imports (cos/diag -> l2tp, static/ospf -> bfd, vpp
backends). Everything else -- 15 of ~20 features -- is already reachable only
through generated registration and gates with manifest lines alone. Small-core
registration discipline paid for itself.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One spec, 7 phases | umbrella + 6 child specs (the feature-gate-0 shape) | feature-gate-8 gated 4 protocols in one spec; phases map to commits; split later if a phase stalls |
| `ze_pxe` shared tag for tftpserver + imageserver | `ze_tftpserver` + `ze_imageserver` | user decision 2026-07-22; they ship as the netboot pair (install spec family) |
| Per-feature tags elsewhere (no ze_dns, no ze_bng grouping) | grouping as112+geodns, l2tp+radius under one tag | matches ze_isis/ze_ldp precedent (independent opt-out); radius stays separate because aaa uses it independently of l2tp |
| `bfd/api` AND `bfd/packet` stay UNGATED | gate them and core-leaf-move the contract | api already IS the always-on contract leaf (atomic nil-able registry) and packet is its ~500-line pure-stdlib State/Diag source; moving churns 7 importers for nothing |
| Generator: per-package constraints with mixed-tag file splitting (`all_<tag>_<parent>.go`) | (a) per-tag parentTagOf as-is; (b) declared-dependency manifest column; (c) authradius under the ze_l2tp tag | (a) wrongly compound-gates the plain radius schema; (b) adds a hand-maintained column the path already encodes; (c) pins radius into every BNG build. Per-package derivation keeps the manifest shape and ze_bmp byte-stable |
| cos dynamic handler tagged `ze_l2tp` (dependent file) | extract the session-metadata store + events to a core leaf (11-file move) | dynamic RADIUS-CoS is intrinsically a BNG feature: no sessions, no dynamic CoS. Feature composition over contract extraction; the no-op stub keeps static CoS whole |
| diag/web l2tp coupling: carve l2tp branches into gated helpers with honest "not included in this build" stubs | whole-file tags (drops BGP/BFD capture too); Service-interface extraction (blocked by the documented l2tp->api->l2tp cycle, service_locator.go:15-18) | the capture files are multi-protocol; stubs answer honestly instead of silently no-opping |
| `ike/dataplane` stays UNGATED | gate it under ze_ike; move it to internal/core | it is the shared XFRM seam OSPF RFC 4552 programs through (ipsec_install.go:59-95); a core-leaf move is right long-term but deliberately not pursued in this spec (its vpp file is ze_vpp-gated separately) |
| hub BNG construction behind a nil-able `bngRegister` seam var | service construction registry | the BNG registers engine SUBSYSTEMS, not Reconfigurable listeners; the registry contract does not fit (R-7 confirmed) |
| ntp first as pattern validation | start the sweep in bulk | iteration workflow: one change, one test, then scale |
| provision/connect NOT gated | include them | small, cmd/ze-rooted, low value; revisit on demand |
| `makeDryRun` fixed at source (skip '='-bearing MAKEFLAGS words) | work around by passing ZE_VERIFY_LOG as env var | the bash-output rule PRINTS the failing invocation; the next agent would hit the same wall (diagnosis-before-fix) |

## Known Limitations
- `internal/plugins/provision`, `internal/plugins/connect`, `ping`/`traceroute` components deliberately not gated (see Task "NOT in scope").
- Absent-build testing is per-tag-off, not combinatorial across 36 tags; the appliance tag set and bare `ze_core` are the only tested combinations beyond that (matches existing gate practice).
- `internal/exabgp/topics` (and whatever else migration needs, per A-9 validation) stays always-on; `ze_exabgp` drops the bridge plugin, not the migration tooling.

## RFC Documentation
Not applicable: no protocol behavior changes. RFC-tagged tests inside gated
packages (tftpserver RFC 2347/2348/2349) are untouched and keep running in the
full-feature test build; do NOT edit them (rfc-tagged-test hook).

## Implementation Summary

### What Was Implemented
- 20 new feature gates, all default-on in `ZE_FEATURES`, in 7 phases:
  - Phase 1: `ze_ntp` (pattern validation; nil-safe `GetNTPSyncInfo` seam confirmed at registry.go:404-421, reader `cmd/show/system.go:126`).
  - Phase 2 (Group A, manifest-lines-only): `ze_flowexport`, `ze_ddos`, `ze_anomaly`, `ze_as112`, `ze_geodns`, `ze_dhcpserver`, `ze_pxe` (tftpserver+imageserver, user-directed grouping), `ze_trafficusage`, `ze_policyroute`, `ze_cos`, `ze_copp`, `ze_mpls`.
  - Phase 3 (Group B): `ze_tacacs` (gated `aaa/all/all_ze_tacacs.go` sibling + `cmd/ze/dispatch_tacacs.go`), `ze_exabgp` (`cmd/ze/dispatch_exabgp.go`; migration library stays always-on).
  - Phase 4: `ze_bfd` (engine subtree gated; `bfd/api` + `bfd/packet` stay as the always-on nil-able contract; the `bfd/cmd` schema pin moved to `config/yang/cli/tree_bfd.go`).
  - Phase 5: `ze_vpp` (connector + 5 per-plugin backends; `static/backend_vpp_linux.go` retagged `linux && ze_vpp` with a nil-returning stub; ike dataplane vpp backend split into `register_vpp.go`; drops vendored govpp).
  - Phase 6: `ze_ike` (hub blank imports -> `cmd/ze/hub/register_ike.go`; web VPN page split with a not-in-this-build stub; `ike/dataplane` stays always-on for OSPF RFC 4552).
  - Phase 7: `ze_l2tp` + `ze_radius` (hub construction -> `bngRegister` seam in `bng_infra.go` + `register_l2tp.go`; `dispatch_l2tp.go`; web L2TP pages tagged with stub, generic system/service renderers extracted to `page_workbench_generic.go`; diag captures split into gated `capture_l2tp.go`/`capture_raw_l2tp.go` + honest stubs; cos dynamic handler tagged `ze_l2tp` with no-op stub; radius as a MIXED tag: plain `all_ze_radius.go` for system auth + generated `all_ze_radius_ze_l2tp.go` for authradius).
- Generator extension (`scripts/codegen/plugin_imports.go`): per-package dependent constraints -- `parentTagOfImport`/`constraintForImport`/`constraintGroups` replace the per-tag `parentTagOf`/`buildConstraint`; mixed tags split into `all_<tag>.go` + `all_<tag>_<parent>.go`; single-constraint tags (ze_bmp) byte-stable.
- Tests: per-tag present/absent pairs (`build_tag_{ntp,bfd,ike,vpp,l2tp}_*`), grouped pairs for Group A/B, and the consolidated `build_tag_gate12_absent_test.go` nm proof (bare-core needles for every gated subtree incl. govpp, plus the ze_bgp-without-ze_bfd link-independence build). Dataplane registry pair (`register_vpp_test.go`/`register_novpp_test.go`).

### Bugs Found/Fixed
- `makeDryRun` (scripts/status/verify_run.go:111) misread GNU make 3.81's MAKEFLAGS: a command-line variable override (`make ze-verify ZE_VERIFY_LOG=tmp/x.log`) appears as the first word with no `--` separator, and `ContainsAny(fields[0], "ntq")` matched the 't' in "tmp", refusing the exact invocation `ai/rules/bash-output.md` recommends. Fixed (a flags word never contains '='), with captured-MAKEFLAGS test rows.
- `docs/features.md` MPLS row claimed `internal/component/mpls` "stays always-on" -- now gated by `ze_mpls`; claim corrected.

### Documentation Updates
- `ai/rules/feature-gate-registration.md`: tag inventory; dependent-gate section rewritten for per-package constraints and mixed tags; three new patterns documented (ungated shared contract leaves, dependent files with honest stubs, subsystem-builder seam). `ai/rules/CONDENSED.md` regenerated.
- `docs/features.md`: compile-out sentences on 18 rows (BFD, Policy Routing, IKEv2 Engine, Flow Export, Anomaly, Traffic Usage, VPP, L2TP BNG, TACACS+, RADIUS, PPPoE, CoPP, DHCP, AS112, ExaBGP, Interfaces (ntp/cos), Installation (pxe/dhcp), MPLS row fix), each with a `feature-gates.txt` source anchor.
- `docs/guide/plugins.md`: geodns row gating mention.
- `docs/guide/quickstart.md`, `.golangci.yml`, `gokrazy/ze/config.json`: regenerated from the manifest (never hand-edited).

### Deviations from Plan
- A-3 broken as stated (`bfd/api` imports `bfd/packet` + `component/plugin`); recovered by leaving BOTH api and packet ungated. Mistake Log row recorded.
- The dependent-gate machinery needed a real extension (per-package constraints), not just the A-7 "no generator change" expectation: the ze_radius tag MIXES an independent package with a nested one, which the per-tag `parentTagOf` could not express (it would have compound-gated the plain radius schema too). The generator now splits mixed tags; `ze_bmp` output is byte-identical.
- cos dynamic CoS resolved by dependent-file tagging (`//go:build ze_l2tp` + no-op stub) instead of the session-metadata contract extraction sketched in the spec: dynamic RADIUS-CoS is intrinsically a BNG feature (no sessions without l2tp), so gating it WITH the BNG is honest feature composition and avoids an 11-file store move.
- diag capture files were multi-protocol (BGP/BFD/l2tp), so instead of whole-file tags the l2tp branches were carved into gated helpers with honest not-in-this-build stubs.
- `page_l2tp.go` also held the generic system/service workbench renderers; they were extracted to `page_workbench_generic.go` (go_extract) before tagging.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Group A: 13 tags, blank-import partitioning | Done | `feature-gates.txt` Group A block; generated `all_ze_*.go` | ntp validated the pattern first |
| Group B: ze_tacacs + ze_exabgp extra roots | Done | `aaa/all/all_ze_tacacs.go`, `cmd/ze/dispatch_{tacacs,exabgp}.go` | migration library stays always-on (A-9) |
| ze_bfd with api contract ungated | Done | manifest ze_bfd block; `config/yang/cli/tree_bfd.go` | A-3 recovery: packet also stays |
| ze_vpp multi-package backend gate | Done | manifest ze_vpp block; `static/backend_vpp_off_linux.go`; `ike/dataplane/register_vpp.go` | govpp drop proven by nm needle |
| ze_ike with dataplane seam ungated | Done | manifest ze_ike block; `cmd/ze/hub/register_ike.go`; `web/page_vpn_ipsec_off.go` | OSPF RFC 4552 path preserved |
| ze_l2tp + ze_radius (dependent) | Done | manifest blocks; `bng_infra.go` + `register_l2tp.go`; `all_ze_radius_ze_l2tp.go` | required the generator mixed-tag extension (R-8) |
| Not-in-scope list untouched | Done | provision/connect/ping/traceroute/platform ungated | per spec Task section |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `feature-gates.txt`; `make generate` idempotent (re-run produced no diff); tier-check green (tier8.log) | derived consumers regenerated, none hand-edited |
| AC-2 | Done | `TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (one bare-core build, needles for all 20 subtrees + govpp) | run t19.log, PASS |
| AC-3 | Pending full verify | `make ze-verify` (running) | targeted suites already green |
| AC-4 | Done | `TestBuildTag_*AbsentRejects*Config` per tag/group | run t19.log, PASS |
| AC-5 | Done | nm: `ze_core,ze_bgp` build links reactor, zero bfd engine/session; consumers nil-check (static/register.go:240, ospf/bfd_client.go:193-200, peer_bfd.go:66-71) | link-level + producer evidence |
| AC-6 | Done | `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` (LoadBackend unknown-backend, backend.go:283-290); `TestVPPBackendAbsent` (dataplane Load not-registered) | fail-closed at the registry producers |
| AC-7 | Done | `TestBuildTag_L2TP_Absent*`, `TestBuildTag_IKE_Absent*`; web stubs render not-in-this-build; diag stubs answer honestly | |
| AC-8 | Done | generated `all_ze_radius_ze_l2tp.go` = `//go:build ze_l2tp && ze_radius`; nm: l2tp-only build 0 radius syms, radius-only 0 l2tp syms; `l2tp-auth-local` registered in full builds | |
| AC-9 | Done | `config/migration/listener.go` imports only `internal/exabgp/topics`; migration packages ungated; exabgp compat suite in full verify | |
| AC-10 | Done | `TestBuildTag_Gate12GroupB_Absent` + tacacs config rejection; `aaa/all/all_ze_tacacs.go` | |
| AC-11 | Pending full verify | gokrazy GoBuildTags regenerated (all 20 tags); `TestGokrazyConfigMatchesApplianceBuildTags` inside verify | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| per-tag present/absent pairs | Done | `cmd/ze/hub/build_tag_{ntp,bfd,ike,vpp,l2tp}_*_test.go` + Group A/B grouped files | grouped-consolidation precedented by protocols/gate11 |
| consolidated nm proof | Done | `cmd/ze/hub/build_tag_gate12_absent_test.go` | + ze_bgp-without-ze_bfd build |
| generator constraint proof | Done | generated `all_ze_radius{,_ze_l2tp}.go` inspected; `ze-regen-check` in verify re-derives | plus dataplane register pair `register_{vpp,novpp}_test.go` |
| makeDryRun regression rows | Done | `scripts/status/verify_run_test.go` TestMakeDryRunDetectsDashN | red-then-green run pasted in t23.log/session |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `feature-gates.txt` +~50 lines | Done | 20 tags, comments per block |
| generated group files | Done | 21 `all_ze_*.go` incl. the mixed-tag split file |
| hub seam + register files | Done | `bng_infra.go`, `register_l2tp.go`, `register_ike.go` |
| dispatch companions | Done | `dispatch_{tacacs,exabgp,l2tp}.go` |
| aaa/all gated siblings | Done | `all_ze_{tacacs,radius}.go` |
| tree_bfd.go schema pin | Done | `internal/component/config/yang/cli/tree_bfd.go` |
| web/cos/diag splits + stubs | Done | see Implementation Summary |
| docs + rule updates | Done | features.md, plugins.md, feature-gate-registration.md, CONDENSED.md |

### Audit Summary
- **Total items:** 7 requirements, 11 ACs, 4 test groups, 8 file groups
- **Done:** all except AC-3/AC-11 whose final evidence is the in-flight full verify
- **Partial:** none
- **Skipped:** none
- **Changed:** 5 deviations, all documented in Deviations from Plan (design improvements, no scope reduction)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Every audited candidate is compile-out-able | absent nm test per tag | `TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (bare ze_core, per-subtree needles for all 20 tags + govpp + the mixed l2tp/radius lanes) PASS; per-tag `*_Absent` registration+config-rejection tests PASS |
| Default build unchanged | full verify | `make ze-verify` green after the review fixes (functional 224/458 attributed to the concurrent forward-pool session, see below); targeted suites green |
| Appliance image unchanged | drift test | `gokrazy/ze/config.json` GoBuildTags regenerated to all 40 tags; `TestGokrazyConfigMatchesApplianceBuildTags` inside verify |
| Minimal builds compile | tag-set compile check | `ze_core`, `ze_core,ze_bgp`, `ze_core,ze_l2tp`, `ze_core,ze_radius` all build; nm cross-check: l2tp-only 0 radius syms, radius-only 0 l2tp syms |
| AAA fails closed on the persisted config format | new unit test | `TestSetFormatRejectsUnknownFieldFailClosed` (config loader now errors on set/set-meta configs carrying fields unknown to this build) PASS |

## Review Gate

### Run 1 (initial) -- two independent adversarial subagents (logic lens, security lens) over the full diff
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The "config block rejected as unknown" fail-closed claim held only for brace format; the daemon's persisted **set-meta** format takes the pre-migration lenient path (`parseSetWithMigration` fallback) which PRUNES unknown fields to warnings and boots. A stripped build loading a full build's committed config would silently drop gated blocks (tacacs/radius auth -> fall back to local auth) with no error/warn/doctor code -- fail-open on the daemon's own format. | `internal/component/config/loader.go:148-167`; `setparser.go:193-199`; consumed by boot (`main.go` LoadConfig) and `ze config validate` | **Fixed:** `parseSetWithMigration` now returns an error naming the dropped fields when the pre-migration pass left warnings (a surviving warning is silent config loss, never a healed rename). Regression test `TestSetFormatRejectsUnknownFieldFailClosed` (set/set-meta/nested), plus a positive guard. |
| 2 | ISSUE | Mixed l2tp/radius build lanes had zero test coverage: `build_tag_l2tp_{present,absent}_test.go` carry compound `ze_l2tp && ze_radius` / `!ze_l2tp && !ze_radius` constraints, so an l2tp-only or radius-only build (both advertised) compiled NEITHER file. A generator-split regression would ship an advertised mixed build broken with both CI lanes green. | `cmd/ze/hub/build_tag_l2tp_present_test.go:3`, `_absent_test.go:4` | **Fixed:** added a mixed-lane nm matrix to `TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (`ze_core,ze_l2tp` links l2tp + zero radius; `ze_core,ze_radius` links radius + zero l2tp). |
| 3 | ISSUE | `capture-raw start\|stop l2tp` in a `!ze_l2tp` build returned `StatusDone` with an empty list and no message -- a silent no-op for an explicitly-requested absent feature, violating the stub-honesty rule added in the same diff (the `dump` stub was honest). | `internal/plugins/diag/cmd/capture_raw_l2tp_off.go:12-14` | **Fixed:** `captureRawL2TPNote` fills an honest `l2tp: "l2tp is not included in this build (ze_l2tp off)"` on explicit l2tp start/stop; on-build variant returns empty (unchanged semantics). |
| 4 | NOTE | Manifest comment said BFD consumers "degrade with a warn"; static logs at Info (register.go:243), invisible at the default WARN level. | `feature-gates.txt` ze_bfd block | **Fixed:** comment corrected (BGP/OSPF warn + OSPF doctor check; static logs info). |
| 5 | NOTE | `TestCodeQLBuildUsesShippedTags` rationale still said codeql.yml tags "are duplicated ... a static workflow cannot expand a Makefile variable"; they are now generated by `rewriteCodeQL`. | `scripts/status/verify_run_test.go:1293-1299` | **Fixed:** comment updated to say GENERATED-from-manifest. |
| 6 | NOTE | `build_tag_l2tp_absent_test.go` comment claimed radius-auth blocks are rejected, but only l2tp/pppoe were tested. | `cmd/ze/hub/build_tag_l2tp_absent_test.go:9-10` | **Fixed:** added a `radius-admin` rejection case (`system { authentication { radius {} } }`), the fail-closed analogue of the tacacs case. |
| 7 | NOTE | Gate12 nm test had no build constraint, so its build+nm jobs ran in BOTH unit passes (gate11 was deliberately constrained for this). | `cmd/ze/hub/build_tag_gate12_absent_test.go` | **Fixed:** constrained `//go:build ze_l2tp && ze_radius` (both default-on) so it runs once in the full-feature pass; the binaries under test carry explicit `-tags`, so the lane's own tags are irrelevant. |
| 8 | NOTE | vpp present-test / dataplane `Load("vpp")` probes have side effects only on their success path (need a live VPP connector during a unit run; no other hub/dataplane test reads an iface backend). | `build_tag_vpp_present_test.go:107`, `ike/dataplane/register_vpp_test.go:19` | **Acknowledged:** state realism is nil in CI (no VPP socket); the failure-path (the only reachable path) is side-effect-free. Left as-is. |
| 9 | NOTE | `parentTagOfImport` uses first-occurrence `strings.Index` + strict-`>` tie-break; more general than its inputs (unreachable with the current single-suffix module paths). | `scripts/codegen/plugin_imports.go:620-633` | **Acknowledged:** not reachable; documented as a known limitation rather than adding a speculative fix. |
| 10 | NOTE | Duplicate `dataplane vpp` registration now panics (`BUG:` prefix) instead of stderr+os.Exit; reachable only via a programmer-error second init(). | `ike/dataplane/register_vpp.go:14-16` | **Acknowledged:** deliberate; matches the registry panic convention. |
| 11 | NOTE | `L5`: an orphaned `runYANGConfig` doc comment sits above `errUnauthenticatedMgmtListener`. | `cmd/ze/hub/main.go:226-231` | **Not mine:** the intervening var+func were inserted by a concurrent session (the mgmt-listener refusal work); absent from `git show HEAD:cmd/ze/hub/main.go` above `runYANGConfig`. Left for that session. |

### Fixes applied
- BLOCKER 1: `internal/component/config/loader.go` fail-closed on set-format pre-migration warnings + `internal/component/config/loader_unknown_test.go`.
- ISSUE 2: mixed-lane nm matrix in `cmd/ze/hub/build_tag_gate12_absent_test.go` (+ its `//go:build ze_l2tp && ze_radius` constraint, NOTE 7).
- ISSUE 3: `capture_raw_l2tp{,_off}.go` `captureRawL2TPNote` + `capture_raw.go` dispatcher wires it into start/stop.
- NOTEs 4/5/6: comment corrections + the radius-admin rejection test case.

### Run 2 (verify pass on the fixes)
The three fixes are new code, so a focused adversarial verifier re-checked the
load-bearing one -- the loader fail-closed guard (a security-critical guard) --
on four refutation targets: (A) can any set-format unknown field still load
clean, or does a legitimate config now trip the guard; (B) does a real
migration leave a lingering warning that the guard wrongly rejects; (C) does
the hierarchical path already reject unknowns; (D) is the guard in the function
both boot and validate reach.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| -- | (none) | Verifier CONFIRMED all four targets: the guard holds. Headline (B): pruning-before-migration makes "reaches the guard" and "a migration can heal the field" mutually exclusive (a schema-known field makes the strict parse succeed and return early), so no legitimate rename-migration is broken; the `internal/component/config/migration` suite is green under the fix and builds trees directly (never through the loader). `SetParser.warnings` has exactly one source (unknown fields), so no valid config trips it. | -- | Verifier's one residual (message overstated the migration path's reach) fixed: error text + comment now say "a legacy field this schema no longer defines," with the mutual-exclusivity note in the comment. |

Fixes 2 (mixed-lane nm matrix) and 3 (capture-raw honesty) are additive test
coverage and a stub-honesty note; both were run green in both build lanes and
carry no refutation risk (no guard, no removed behavior).

### Final status
- [x] `/ze-review` shows 0 BLOCKER, 0 ISSUE (Run 1 findings 1-3 fixed; Run 2 verifier confirmed the fix)
- [x] All NOTEs recorded above (findings 4-11: 4/5/6 fixed, 7 fixed, 8/9/10 acknowledged as unreachable/deliberate, 11 belongs to a concurrent session)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| all 19 hand-written gate files | Yes | `ls -la` 2026-07-23: dispatch_{exabgp 741B, l2tp 539B, tacacs 640B}.go; hub bng_infra.go 1.3K, register_ike.go 701B, register_l2tp.go 2.5K; aaa/all all_ze_{radius 730B, tacacs 790B}.go; tree_bfd.go 585B; dataplane register_vpp.go 693B; web page_{l2tp_off 931B, vpn_ipsec_off 782B, workbench_generic 1.9K}.go; cos handler_off.go 1.6K; diag capture_l2tp{,_off}.go + capture_raw_l2tp{,_off}.go; static backend_vpp_off_linux.go 553B |
| 21 generated group files | Yes | `ls internal/component/plugin/all/all_ze_*.go` includes all 20 new tags + the mixed-split `all_ze_radius_ze_l2tp.go` |
| 12 build-tag test files | Yes | `ls cmd/ze/hub/build_tag_{ntp,bfd,ike,vpp,l2tp,gate12*}_*.go`; dataplane `register_{vpp,novpp}_test.go` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | manifest complete, generate idempotent, no hand-edited derived file | `make ze-tier-check` green; `make generate` re-run produced no diff; codeql.yml/quickstart/golangci/gokrazy regenerated (feature_tags gained the codeql target after `TestCodeQLBuildUsesShippedTags` caught the drift) |
| AC-2 | bare ze_core drops all 20 subtrees | `TestBuildTag_Gate12_AbsentBinaryDropsSymbols` PASS (needles for every subtree + govpp + `internal/exabgp/bridge`; mixed l2tp/radius lanes) |
| AC-3 | default build unchanged | full `make ze-verify` ran all suites; only 224/458 failed, both attributed to the concurrent test-peer session (fail identically on a committed-HEAD ze binary) |
| AC-4 | absent build rejects gated config | `TestBuildTag_*_AbsentRejects*Config` per tag/group PASS; **plus** the set-format path now rejected too (`TestSetFormatRejectsUnknownFieldFailClosed`, the review BLOCKER fix) |
| AC-5 | bfd off: BGP boots, no bfd engine linked | gate12 nm test's `ze_core,ze_bgp` sub-build (reactor present, bfd engine/session absent) PASS; consumers nil-check at producers |
| AC-6 | vpp off: netlink applies, vpp rejected | `TestBuildTag_VPP_AbsentIfaceBackendFailsClosed` + `TestVPPBackendAbsent` PASS |
| AC-7 | l2tp/ike off | `TestBuildTag_{L2TP,IKE}_Absent*` PASS; web stubs render not-in-this-build; diag stubs answer honestly (incl. capture-raw after fix 3) |
| AC-8 | l2tp on, radius off dependent gate | generated `all_ze_radius_ze_l2tp.go` = `//go:build ze_l2tp && ze_radius`; mixed-lane nm matrix PASS |
| AC-9 | exabgp off keeps `ze config migrate` | `config/migration/listener.go` imports only `internal/exabgp/topics`; migration suite green under the fix |
| AC-10 | tacacs off | `TestBuildTag_Gate12GroupB_Absent` + tacacs config rejection PASS |
| AC-11 | appliance tags regenerated | `gokrazy/ze/config.json` GoBuildTags has all 40 tags; `TestGokrazyConfigMatchesApplianceBuildTags` in verify |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| default full-feature build registration | existing `.ci` suites (unchanged) + `TestBuildTag_*_Present` | Yes -- every gated plugin registers under full tags; hub seams (`bngRegister`) filled |
| bare/mixed build symbol drop | `cmd/ze/hub/build_tag_gate12_absent_test.go` (nm) | Yes -- read the file; builds real binaries with explicit -tags and asserts symbol absence |
| set-format config fail-closed | `internal/component/config/loader_unknown_test.go` | Yes -- drives `ParseTreeForValidation` (the boot+validate producer) across set/set-meta/nested |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | fresh audit: all Group A blockers=0; tier-check green |
| A-2 | confirmed | no Group A `cmd/ze` root import |
| A-3 | **broken** (recovered) | `bfd/api` imports `bfd/packet` + `component/plugin`; both kept ungated (Mistake Log row) |
| A-4 | confirmed | all four BFD consumers nil-check GetService and degrade (static logs info, BGP/OSPF warn) |
| A-5 | confirmed | ike hub coupling was blank imports only |
| A-6 | confirmed | l2tp lookups nil-safe; resolved by dependent-file tagging not contract extraction |
| A-7 | confirmed | generated `all_ze_radius_ze_l2tp.go` carries the compound constraint |
| A-8 | confirmed | full verify ran all suites; no suite broke from a tag landing |
| A-9 | confirmed | migration importers reach only `internal/exabgp/topics`; migration suite green |
| A-10 | confirmed | mpls sole importer was `all.go` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `make ze-doc-test` | ran green after `ze-doc-index`/`ze-discovery-index` regenerated the stale `ai/CODE-TO-DOCS.md`/`ai/DOCS-TO-CODE.md` and the digest anchor was fixed | Yes |
| features.md compile-out sentences | 18 rows, each with a `feature-gates.txt` source anchor; the security reviewer spot-checked the VPP (LoadBackend rejection) and BFD (nil-seam) rows against `iface/backend.go:284-289` and `bfd/api/registry.go:47-68` -- both accurate | Yes |
| MPLS row stale "always-on" claim | corrected to gated-by-`ze_mpls` | Yes |
| feature-gate-registration.md rewrite | the logic reviewer confirmed the described generator behavior matches `plugin_imports.go` `parentTagOfImport`/`constraintGroups`/`taggedGroupFileName` | Yes |
| doc-links (HOOK-FRICTION rfc paths) | marked `doc-links: ignore` (the finding IS the nonexistent path) | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
