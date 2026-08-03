# 1251 -- feature-gate-11-bmp-mrt

## Context

The `ze_bgp` gate (1249) made the whole BGP subsystem compile-out-able and left
BMP (a BGP plugin) and MRT (a standalone always-on plugin) reachable only as
all-or-nothing with the engine. The goal: make **BMP** (`ze_bmp`) and **MRT**
(`ze_mrt`) independently compile-out-able, default-on via `ZE_FEATURES` so the
shipped `ze` is unchanged. One spec because the two share the BGP message-type
leaf `internal/core/bgp/msgtype`, and the only honest proof is a symbol matrix
across `{ze_bgp, ze_bmp, ze_mrt}`, not two independent present/absent pairs. The
two gates have opposite dependency shapes, which is the whole lesson.

## Decisions

- **BMP is the first DEPENDENT gate (`ze_bmp` ⟹ `ze_bgp`).** BMP imports
  `bgp/message`/`bgp/types` and monitors the BGP RIB, so it cannot exist without
  the engine. Chose **path-nesting inference** over an explicit manifest
  `requires=ze_bgp` column (which the spec had recommended): BMP's package lives
  under `internal/component/bgp/` (a ze_bgp package), so `plugin_imports.go`
  `parentTagOf` finds the longest OTHER gated package that is a path-prefix and
  emits the child group file with a COMPOUND constraint --
  `all_ze_bmp.go` is `//go:build ze_bgp && ze_bmp`. Chosen because it needs **no
  manifest-syntax change**, so all five manifest consumers (Makefile awk,
  `TestBuildTags`, `feature_tags.go`, `stress-repro.py`, `dep_audit.py`) stay
  untouched, and the nesting is a true reflection of the dependency.
- **MRT is a plain plugin-compile-out after one extract-then-gate.** Its only
  always-on pin was `cmd/ze/hub/main.go` setting `MRTMessageCallback`/
  `MRTPeerCallback` on `registry.BGPBootstrap` via a direct `internal/plugins/mrt`
  import. Inverted it onto the registry seam (new `SetMRTMessageCallback` /
  `GetMRTMessageCallback` / `SetMRTPeerCallback` / `GetMRTPeerCallback`, mirroring
  the existing `SetRIBDumpCallback`): MRT self-registers the bridges from `init()`,
  and the ze_bgp-gated consumer `bgp/config` reads them via the getters. Dropped
  the two `BGPBootstrap` fields entirely. `main.go` no longer imports MRT.
- **`msgtype` / `internal/mrt` stay untagged core-leaves, gated by DCE.** They are
  pure libraries (no `init()`); dead-code elimination drops them from any build
  that does not reference them, verified by `nm`, never a source tag. Post-review
  correction (2026-07-23): `msgtype` SYMBOLS survive only under `ze_bgp` -- MRT
  uses only the `MessageType` type and `TypeUPDATE` constant
  (`internal/plugins/mrt/component.go`), which inline away, so an `ze_mrt`-only
  build links zero `msgtype` symbols even though it imports the package. Symbol
  presence is not a proxy for package use when a consumer touches only
  consts/types; the matrix test encodes the empirical truth.
- **Callback signatures already share where correct.** `HealthPeerCallback` and
  `MRTPeerCallback` are both `PeerLifecycleCallback`; `MRTMessageCallback` is
  `MessageCallback` (raw wire bytes), a distinct concern -- merging all three would
  force `HealthRevert` to stub `OnBGPMessage`, violating interface segregation. No
  change.

## Consequences

- A `ze_bgp` build can now ship without BMP (`ze_bmp` off), and any build can ship
  without MRT (`ze_mrt` off). Symbol matrix (proven by `nm`,
  `TestBuildTag_Gate11_SymbolMatrix`): `bmp` present iff `ze_bgp && ze_bmp`;
  `mrt`/`internal/mrt` present iff `ze_mrt`; `msgtype` present iff `ze_bgp`
  (with `ze_mrt` alone the package is imported but fully inlined -- see
  Decisions). `ze_bmp` WITHOUT `ze_bgp` links neither BMP nor the engine (the
  dependent gate holds).
- The dependent-gate mechanism (`parentTagOf`/`buildConstraint` in
  `plugin_imports.go`) is general for one nesting level: any future gate whose
  package nests under another gate's tree AND imports it gets the compound
  constraint automatically. Documented in `ai/rules/plugins.md`.
- `dep_audit` needed **zero** changes: its existing subtree-prefix same-feature
  skip already treats a bmp→bgp import as intra-`ze_bgp`-family (bmp lives under
  `internal/component/bgp`). The anticipated R-2 work was a non-issue.

## Gotchas

- **`writeTaggedGo` had to split "filename tag" from "build constraint."** The
  group filename uses the child tag (`all_ze_bmp.go`), but the `//go:build` line
  uses the compound constraint. A single string can't be both (`&&`/spaces aren't
  filename-safe).
- **Carving BMP out cleared a latent duplicate.** `all_ze_bgp.go` had BMP
  blank-imported twice (BMP is discovered as both a plugin and an RPC package, and
  `filterTagged` pools both). Added adjacent-dedup in `writeTaggedGo`; only BMP was
  affected, so all other gate files stayed byte-stable.
- **A test still set the removed struct fields.** `plugin/coordinator_test.go`
  `TestBootstrapRoundTrip` set `MRTMessageCallback`/`MRTPeerCallback` on
  `BGPBootstrap`; removed them (and the now-dead `fakeMsgCB` helper) -- the seam is
  covered by `registry.TestMRTCallbackSeam`. `make ze-verify`'s unit stage caught
  this, not review.
- **Verify byte-stability before the manifest change.** After editing
  `plugin_imports.go`, `--check` must pass with no diff (no existing gate is
  cross-tag-nested, so `parentTagOf` returns "" for all of them) BEFORE flipping
  BMP's manifest tag -- that isolates the codegen change from the gate change.

## Files

- `feature-gates.txt` -- `ze_mrt internal/plugins/mrt`; BMP line `ze_bgp` -> `ze_bmp`.
- `scripts/codegen/plugin_imports.go` -- `parentTagOf`/`buildConstraint` (dependent gate) + dedup.
- `internal/component/plugin/registry/interfaces.go` -- MRT message/peer seam; removed the two `BGPBootstrap` fields.
- `internal/plugins/mrt/register.go` -- self-register bridges from `init()`.
- `cmd/ze/hub/main.go` -- drop the `mrtcomp` import + field assignments.
- `internal/component/bgp/config/register.go` -- read bridges from the seam.
- `internal/component/plugin/coordinator_test.go` -- drop removed fields.
- `cmd/ze/hub/build_tag_{bmp,mrt}_{present,absent}_test.go`, `build_tag_gate11_absent_test.go`, `registry/mrt_seam_test.go` -- present/absent + nm matrix + seam tests.
- generated: `all.go`, `all_ze_bgp.go`, `all_ze_bmp.go`, `all_ze_mrt.go`, `.golangci.yml`, `gokrazy/ze/config.json`, `docs/guide/quickstart.md`.
- docs: `ai/rules/plugins.md` (dependent-gate pattern), `docs/features.md` (MRT + BMP gate notes).
