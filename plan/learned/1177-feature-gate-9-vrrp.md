# 1177 -- feature-gate child 9: VRRP compile-out (ze_vrrp)

## Context

Follow-on to the feature-gate umbrella (`plan/learned/995-feature-gate-8-protocols.md`
and siblings 980-990): make the VRRP first-hop-redundancy plugin compile-out-able via a
`//go:build ze_vrrp` tag, for a smaller / more hardened `ze` binary. VRRP is a
self-registering system plugin (`internal/plugins/vrrp`, `register.go` init ->
`registry.Register("vrrp")`), so this was a pure *enrollment* into the existing
manifest-driven machinery, not a new mechanism. The whole task was two manifest lines,
one lint tag, `make generate`, three build-tag tests, and docs.

## Decisions

- **VRRP is the simplest gated-plugin shape (ldp/rsvpte class), NOT the isis/ospf class.**
  It registers its CLI through the plugin registry's `reg.CLIHandler` (register.go:81),
  not a programmatic `cli` dispatch package, so `grep vrrp cmd/ze/*.go` is empty and it
  needs NO `cmd/ze/dispatch_vrrp.go` companion. Only ONE composition root (generated
  `all.go`), so gating is purely blank-import partitioning.
- **Two manifest lines, yang auto-derived.** `ze_vrrp internal/plugins/vrrp` +
  `ze_vrrp internal/plugins/vrrp/transport` (transport has its own `register.go`). The
  generator's `<pkg>/yang` derivation gates `vrrp/yang` (both `ze-vrrp-conf.yang` and
  `ze-vrrp-cmd.yang`) from the first line. `fsm`/`packet` have no `register.go` and are
  imported only transitively, so dead-code elimination drops them -- no manifest line.
- **Default-ON**, matching all 12 existing gates: `ZE_FEATURES` (Makefile awk) auto-emits
  `ze_vrrp`, so `ze`/`ze-appliance` still ship VRRP; only `ze-stripped` / bare `ze_core`
  drop it. User goal was "so code CAN be compiled out" = on by default, optionally out.
- **Zero code changes to the derived consumers.** `dep_audit.py`, `Makefile`,
  `internal/test/runner` all derive from `feature-gates.txt`; the vrrp->transport
  intra-feature import is already handled by the protocol-era `_same_feature_importer`
  generalization (A-2 confirmed by a clean `make ze-tier-check`).
- **The manifest's static consumers are now GENERATED, not hand-maintained (user-directed).**
  What began as "add `ze_vrrp` to `.golangci.yml` + `gokrazy/ze/config.json` by hand" became a
  single-source-of-truth fix: `scripts/codegen/feature_tags.go` (via `make generate`) regenerates
  `.golangci.yml` `build-tags`, `gokrazy/ze/config.json` `GoBuildTags`, and `docs/guide/quickstart.md`'s
  `go install` command from `feature-gates.txt`; `scripts/dev/stress-repro.py` derives its `race_tags`
  at runtime. Chose generation-for-static-files + runtime-derivation-for-programs over
  hand-maintain-plus-drift-gate, because a drift gate only tells you AFTER you forget (as it did here).
- **nm proof folded into the existing consolidated test** rather than a second bare-core
  build: `build_tag_protocols_absent_test.go` gained `&& !ze_vrrp` + vrrp needles, reusing
  its single `go build -tags ze_core` (the cost optimization 995 made). Registration and
  config-rejection stay in a dedicated `build_tag_vrrp_absent_test.go`.

## Consequences

- Proves the "routing-protocol" gating shape in 995 is not protocol-specific: it applies
  to any self-registering `internal/plugins/` engine + `transport` sidecar with
  registry-based CLI. Documented as such in `ai/rules/feature-gate-registration.md`.
- A stripped binary drops the vrrp engine/FSM/packet/transport/schema and rejects a
  `vrrp {}` block under an interface unit as an unknown field (proven by nm + a parse test).
- The `ze explain` catalogue keeps `doctor-vrrp-raw-socket` and the firewall `"vrrp": 112`
  proto-number map always-on by design (both are plugin-import-free string data, exactly
  like the isis/ospf/ldp codes 995 also left catalogued) -- harmless residue this spec
  deliberately does not change (the always-on catalogue is the intended design).

## Gotchas

- VRRP config is a YANG **augment under interface units** (`/iface:interface/.../ipv4`),
  not a top-level block like `ospf {}`/`ldp {}`. The absent-config-rejection test needs a
  minimal-but-valid interface wrapper (from `test/vrrp/vrrp-doctor-quiet.ci`) whose only
  schema-gated token is `vrrp`; assert the error contains "unknown" (ldp/ospf precedent).
- The golden snapshots (`internal/component/plugin/all/testdata/`) already listed vrrp and
  do NOT change: the snapshot test runs under the full feature set (that is why gated
  isis/ospf are in the snapshot), never under bare `ze_core` (Makefile `GO_TEST_CORE` runs
  only `./cmd/ze/hub`). Confirm with a default-tags run; do not "update" the snapshot.
- **FOUR full-tag-list consumers were hand-maintained, not one -- now the manifest is the
  true single source.** The rule listed only `.golangci.yml` as non-deriving. Full `ze-verify`
  caught `gokrazy/ze/config.json` `GoBuildTags` (appliance test), and an independent review +
  an all-file-extension sweep found two more: `docs/guide/quickstart.md`'s `go install -tags`
  command and `scripts/dev/stress-repro.py` `race_tags`. (First canary grep missed the last two
  -- it only scanned `.json/.yml/.go/.mk/.txt`, not `.md`/`.py`; lesson: sweep ALL extensions.)
  Rather than hand-fix + document, the fix (user-directed) makes `feature-gates.txt` the ONE
  source: a new `scripts/codegen/feature_tags.go` (run by `make generate`, surgical byte-stable
  edits, `--check` unit test) regenerates the three static files, and stress-repro DERIVES via
  `_feature_gate_tags()`. Reusable pattern: a static file that embeds a manifest-derived list
  gets GENERATED (with a `--check` test); a program gets a runtime reader. Why full `ze-verify`
  mattered: scoped verification (lint-changed + build-tag tests + tier + doc-test) never runs the
  appliance unit test, so it could not have caught the gokrazy gap.
- A concurrent session's IS-IS RFC5303 work (untracked test, modified `rfc/enrolled.txt`)
  made `ai/RFC-REQUIREMENTS.md` stale mid-session; it had to be regenerated for a green
  `ze-doc-test` but must be EXCLUDED from this commit (it belongs to the isis session).
  Same for `ai/DOCS-TO-CODE.md`: mine to commit (it picked up the two new vrrp test files).

## Files

- Created (feature-tag SSOT): `scripts/codegen/feature_tags.go` (+`_test.go`) -- generates the
  three static tag lists from `feature-gates.txt`.
- Modified (feature-tag SSOT): `Makefile` (`generate` runs feature_tags; `ze-feature-tags-check`),
  `.golangci.yml`, `gokrazy/ze/config.json`, `docs/guide/quickstart.md` (now generated),
  `scripts/dev/stress-repro.py` (derives `race_tags`), `ai/CODE-TO-DOCS.md` (regenerated).
- Modified: `feature-gates.txt` (+2), `docs/features.md` (VRRP row), `ai/rules/feature-gate-registration.md`
  (inventory + single-plugin/no-dispatch note + consumers-are-generated rewrite),
  `internal/component/plugin/all/all.go` (generated, -3 imports), `ai/DOCS-TO-CODE.md`
  (generated), `cmd/ze/hub/build_tag_protocols_absent_test.go` (+vrrp nm coverage)
- Created: `internal/component/plugin/all/all_ze_vrrp.go` (generated),
  `cmd/ze/hub/build_tag_vrrp_present_test.go`, `cmd/ze/hub/build_tag_vrrp_absent_test.go`
