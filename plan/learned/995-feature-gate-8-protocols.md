# 995 -- feature-gate child 8: routing-protocol compile-out (ze_isis/ze_ldp/ze_ospf/ze_rsvpte)

## Context

Final child of the feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`):
make the optional routing protocols **IS-IS, LDP, OSPF, RSVP-TE** compile-out-able
via per-protocol build tags, for a smaller binary and attack surface. The umbrella
asserted this was BLOCKED on tiers-5 B-2 (codec/engine un-fusing). The spec turned
that into a measurable gate (A-1) instead of inheriting it. Unlike children 1-7
(lg/ssh/web/gnmi/mcp/api/telemetry, all `internal/component/` services), the
protocols are self-registering **plugins** in `internal/plugins/`, so the gating
shape is different: pure blank-import partitioning, no new register_<x>.go/seam.

## Decisions

- **A-1 PASSED -- B-2 NOT required; gate each protocol whole.** A fresh grep found
  zero always-on, non-test, cross-tree importers of any protocol package or codec
  subpkg (`packet`/`types`/`v3/packet`/`wire`), and zero cross-protocol imports
  (R-5 clean), and no always-on plugin declaring a Registration string dependency
  on a protocol. Web/MRT/sysrib/redistribute are decoupled (generic dispatch /
  bytes / string protocol names). So each protocol is gated whole (codec + engine)
  by gating its blank imports; the "needs B-2" claim was a hypothesis, not a fact.
- **Plugin compile-out = blank-import partitioning, NOT source tags.** The protocol
  `.go` files carry NO `//go:build` tag (unlike telemetry's exporter). Compile-out
  is purely: list each owned dir in `feature-gates.txt`, `make generate` moves the
  blank imports from the universal `all.go` into `all_<tag>.go`, and dead-code
  elimination drops the unreferenced packages when the tag is off. Proven by `nm`:
  bare `ze_core` links 0 symbols for each of isis(904)/ldp(191)/ospf(1444)/
  rsvpte(266); `ze_core ze_ospf` links only ospf (1445), others 0.
- **One protocol = several manifest lines (multi-dir per tag, A-2 confirmed).** A
  protocol spans multiple discovered dirs; each is its own `<tag> <dir>` line under
  the shared tag (no generator change -- the gnmi 2-line precedent generalises):
  isis {plugin, cli, transport}, ldp {plugin}, ospf {plugin, cli, transport,
  v3/transport}, rsvpte {plugin}. The generator gates `<pkg>` AND `<pkg>/yang` per
  line, so the protocol's yang (config + command schema) rides the plugin line.
- **Command schema lives in `<proto>/yang/`, NOT a `<proto>-cmd` sibling.** The user
  flagged that `ldp-cmd` / `rsvpte-cmd` broke the isis/ospf convention: isis and ospf
  keep BOTH `ze-<p>-conf.yang` and `ze-<p>-cmd.yang` in `<proto>/yang/` (the `cli`
  dir is for Go command-handler logic, which ldp/rsvpte lack). So `ze-ldp-cmd.yang`
  and `ze-rsvp-te-cmd.yang` (+ their `cmd_schema_test.go`) moved into `ldp/yang/` and
  `rsvpte/yang/`; `yang_glue` regenerates each `<proto>/yang/` register.go to register
  both modules. This makes the protocol self-contained (delete `internal/plugins/ldp/`
  and ALL of ldp vanishes; previously `ldp-cmd/` was orphaned) and collapses the
  manifest from two ldp/rsvpte lines to one each. Only ldp/rsvpte moved: the other
  `*-cmd` dirs (aaa-cmd, mpls-cmd, ping-cmd, ...) are command schemas for *components*
  (`internal/component/`), which cannot nest under a different tier, so they stay
  siblings. The `show` self-containment hint paths in `cmd/show/yang/
  self_containment_test.go` were updated to the new locations.
- **Both composition roots gated (the two-root reality).** Protocols register CLI
  via BOTH the generated `all.go` AND the hand-written `cmd/ze/ze_core_dispatch.go`.
  isis/ospf have a programmatic `cli` package reached from the dispatch root; their
  CLI blank imports moved into per-protocol gated companions
  `cmd/ze/dispatch_{isis,ospf}.go` (`//go:build ze_core && ze_<proto>`). Missing the
  dispatch root leaves the package linked in a no-protocol build (R-2).
- **Default-on = all four (user decision).** The four tags go in `feature-gates.txt`,
  so `ZE_FEATURES` (Makefile awk), `TestBuildTags`, and `dep_audit` DISABLEABLE all
  derive them -- `ze`/`ze-appliance` are byte-unchanged; `ze-stripped`/bare `ze_core`
  drop them. Only `.golangci.yml` build-tags is hand-edited (drift-gated).

## Consequences

- The protocols are the FIRST gated packages that are also `sdk.NewWithConn`
  **engines** AND multi-package features whose sub-packages import each other. Two
  pre-existing `dep_audit.py` model gaps surfaced and were fixed at the source:
  1. `is_registration_importer` recognised only `/all/all.go`, not the generated
     `all_<tag>.go`. When an engine's blank import moved from `all.go` into
     `all_ze_isis.go`, the importer stopped counting as registration-only and
     `engine_depended` wrongly flagged the engine as "misplaced -- move to
     component." Fixed: also match `all_<tag>.go` in `.../plugin/all/`.
  2. `disableable_violations` skipped only the gated pkg's OWN subtree, so
     `isis/config.go` importing `isis/transport` (both ze_isis) was flagged as an
     always-on pin. Fixed: skip importers inside ANY same-tag package
     (`_same_feature_importer`) -- same feature, dropped together, not a pin.
  These are correct generalisations (the gate still verifies each package has no
  truly always-on cross-feature importer); the baseline-file "fix" was rejected (it
  is TRANSITIONAL "scheduled to move," and protocols are intentionally plugins).
- The config-package isis test files (`isis_net_validate_test.go`,
  `isis_auth_algorithm_enum_test.go`) need NO gating: they blank-import the untagged
  `isis/yang` package directly to register the schema for the test, independent of
  `all.go` -- the standard "register schema for test" pattern. Protocol packages
  being untagged means importing them in a test compiles under any tag set.

## Gotchas

- **Absent compile-out tests MUST live in `cmd/ze/hub`** (Makefile: `GO_TEST_CORE`
  runs only `./cmd/ze/hub` with bare `ze_core`). The spec drafted them in `cmd/ze`;
  a `//go:build !ze_<proto>` test elsewhere is silently skipped by both passes. The
  registration assertion works there because `cmd/ze/hub/api_test.go` blank-imports
  `plugin/all`, populating the registry in the hub test binary; the absent test
  guards against a vacuous pass with `len(registry.Names()) != 0`.
- **registry names:** isis/ldp/ospf register under their own name; RSVP-TE registers
  as `"rsvp-te"` (hyphen). Config containers: `isis`/`ldp`/`ospf`/`rsvp-te`.
- **The nm symbol-drop test is consolidated** into one `//go:build !ze_isis &&
  !ze_ldp && !ze_ospf && !ze_rsvpte` test (one bare-core build proves all four),
  not four per-protocol builds.
- **geodns (a concurrent session) was already in HEAD's `all.go`**, so regeneration
  kept it -- the all.go diff was exactly the 15 removed protocol lines, no stripping
  needed this time (contrast child 7).

## Files

- Created: `cmd/ze/dispatch_{isis,ospf}.go` (gated dispatch-root CLI);
  `cmd/ze/hub/build_tag_{isis,ldp,ospf,rsvpte}_{present,absent}_test.go` +
  `build_tag_protocols_absent_test.go` (nm proof);
  `internal/component/plugin/all/all_ze_{isis,ldp,ospf,rsvpte}.go` (generated)
- Moved (self-containment rename): `internal/plugins/ldp-cmd/yang/{ze-ldp-cmd.yang,
  cmd_schema_test.go}` -> `internal/plugins/ldp/yang/`; `internal/plugins/
  rsvpte-cmd/yang/{ze-rsvp-te-cmd.yang,cmd_schema_test.go}` -> `internal/plugins/
  rsvpte/yang/`; deleted the empty `ldp-cmd/` and `rsvpte-cmd/` dirs (their generated
  embed.go/register.go); `yang_glue` regenerated `{ldp,rsvpte}/yang/{embed,register}.go`
- Modified: `feature-gates.txt` (+9 net lines: 11 protocol lines minus the 2 removed
  `-cmd` lines), `.golangci.yml` (+4 tags), `cmd/ze/ze_core_dispatch.go` (removed 3
  protocol CLI imports), `internal/component/plugin/all/all.go` (generated),
  `internal/component/cmd/show/yang/self_containment_test.go` (relocated hint paths),
  `scripts/dev/dep_audit.py` (two model fixes), `docs/features.md` (IS-IS/OSPF/
  MPLS-LDP-RSVP-TE rows), `ai/rules/plugins.md` (plugin
  compile-out shape), `ai/rules/architecture.md` (gated edge engine note)
