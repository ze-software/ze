# Spec: netlink-xfrm-open-upstream-prs

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Outward-facing upstream pull requests for the ten vendored `vishvananda/netlink` XFRM patches; Ze already builds and tests with the series locally, so opening the pull requests does not block release correctness or proof. |
| RFC gaps | - |
| Depends | - (patch series prepared by `spec-netlink-xfrm-upstream-series`, landed in `b43120b371`, `c9ce6ba81d`, and `8afd1ff560`) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Homed from `spec-netlink-xfrm-upstream-series` (Phase 7 / AC-11) at its closure on 2026-11-23: opening upstream pull requests is outward-facing and runs only on Thomas's explicit order.

When Thomas orders the pull requests opened against `github.com/vishvananda/netlink`:

1. Create one branch per candidate patch (`netlink-xfrm-01-state-readback-esn-replay.patch` through `netlink-xfrm-10-selector-prefix-family.patch`) on a `ze-software` fork of `github.com/vishvananda/netlink`, carrying exactly that patch (stacked on its named predecessors where required) and referencing upstream issue 1213 on patches 01, 02, and 03.
2. Run upstream's own test suite unprivileged on each branch and verify each branch's diff equals its patch file in `internal/le/vendorpatch/patches/`.
3. Open the pull requests on Thomas's order and update the leading text of each candidate patch file under `internal/le/vendorpatch/patches/` with its pull request address.
4. Keep `netlink-xfrm-90-state-extra-attributes.patch` closed to upstream unless Thomas explicitly asks to propose the generic `ExtraAttrs` hook as well.
