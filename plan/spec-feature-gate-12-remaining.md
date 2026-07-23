# Spec: feature-gate-12-remaining -- gate the remaining compile-out candidates

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
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
| R-1 | An unlinked `init()`: removing an always-on import (hub ike/l2tp) drops a registration nobody else pulls in (the bgp/config trap, learned 1249) | absent-or-present build: feature configured but silently inert; functional suite red | after each import removal, enumerate what the package's `init()` registered; link from the gated dispatch/registration file |
| R-2 | Web/ospf files importing ike/l2tp are per-tag violations even though those files are gated for ANOTHER tag (`file_requires_tag` is per-tag) | `dep_audit.py --check` red on `ze_web`/`ze_ospf` files | compound constraints on those files, or seam the dependency; budget it in phases E/F |
| R-3 | cos imports l2tp: gating `ze_cos` in Group A while l2tp is still ungated is fine, but once `ze_l2tp` lands, cos's l2tp import becomes a cross-gate pin | dep_audit red at phase F | phase F clears it (contract leaf or compound tag on cos's handler file); sequence Group A cos gate first, note the coupling |
| R-4 | 20 new tags double the manifest; tag-combination space explodes and absent-build tests multiply CI time | slow `ze-verify`; flaky nm tests | absent tests are cheap compile+nm, one per tag; no combinatorial testing beyond each tag off individually plus the appliance set |
| R-5 | Appliance image regressions: gokrazy GoBuildTags regenerates to include all new tags; a mistake drops a feature from the shipped image | `TestGokrazyConfigMatchesApplianceBuildTags` red; appliance boot test | the test gates drift; run `make ze-verify` incl. appliance tests per phase |
| R-6 | vpp gating splits files INSIDE ungated plugins (static/backend_vpp_linux.go): tagging single files inside an untagged package risks U1000 unused-symbol lint in no-tag builds | golangci red in either build | prefer moving vpp code into the plugin's `vpp/` sub-package (already exists for fib/firewall/iface/traffic/static), so the tag applies to whole packages |
| R-7 | The l2tp/pppoe hub wiring (ExtractParameters/NewSubsystem/RegisterSubsystem) is construction with config parsing, not a listener service; the service registry may not fit and a new seam shape is needed | phase F design stalls | the engine `RegisterSubsystem` path already takes an interface; a nil-able build-hook var (the ssh_infra shape) fits: gated init registers a subsystem builder, hub iterates builders |

## Wiring Test (MANDATORY — NOT deferrable)

For a compile-out gate, "wiring" is: (present) the feature still registers and
serves through its normal entry point; (absent) zero symbols link and config
using the feature is rejected. One row per phase; per-tag tests named in the
TDD plan.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| default build (all tags) + existing `.ci` suites | → | every gated plugin's `register.go` init via `all_<tag>.go` | full `make ze-verify` per phase (existing suites, unchanged) |
| `ze_core`-only build | → | (nothing: linker drops gated subtrees) | per-tag `cmd/ze/hub/build_tag_<x>_absent_test.go` nm check |
| per-tag build with tag on | → | feature symbols present, service constructible | per-tag `cmd/ze/hub/build_tag_<x>_present_test.go` |
| `ze config validate` in absent build, config uses feature root | → | unknown-key rejection (schema package gated) | one representative `.ci`-style check per phase (pattern from feature-gate-8) |
| hub startup, `ze_l2tp`/`ze_ike` on | → | gated registration builds the subsystems (seam/registry) | existing l2tp/ike functional suites, re-run |
| BGP/OSPF/static with `ze_bfd` off | → | nil `GetService` path (api/registry.go:62-68) | absent-build boot + session-establish functional check |

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
| `bfd/api` stays UNGATED | gate it and core-leaf-move the contract | it already IS the always-on contract leaf (atomic nil-able registry); moving it churns 7 importers for nothing |
| `ze_radius` dependent-gates authradius via path nesting | extend generator with declared dependencies | existing parentTagOf covers it; no new machinery (learned 1251 kept it single-edge deliberately) |
| ntp first as pattern validation | start the sweep in bulk | iteration workflow: one change, one test, then scale |
| provision/connect NOT gated | include them | small, cmd/ze-rooted, low value; revisit on demand |

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
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Every audited candidate is compile-out-able | absent nm test per tag | (fill: test names + output) |
| Default build unchanged | full verify | (fill: `make ze-verify` output) |
| Appliance image unchanged | drift test | (fill: gokrazy test output) |
| Minimal builds compile | tag-set compile check | (fill: `ze_core ze_bgp` build evidence) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during review)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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
