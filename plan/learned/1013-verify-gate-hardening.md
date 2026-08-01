# 1013 - Verify-gate hardening (root cause of a batch of slipped-in breakage)

## Context

A cleanup session fixed four independent breakages: an `aihelp -> cli/client`
import cycle (through `plugin/all`), a missing `zt:decimal-2` YANG typedef, stale
`plugin/all` registry snapshots (copp/ddos/bgp-filter-family added without
`-update`), and 201 dangling `// Design:` refs to closed specs. The user asked
to address the root cause, not just the consequences.

## Root cause (one pattern)

Every one of those is caught by an **existing** check (unit tests for the
cycle/YANG/snapshots; `check_doc_links.py --design-only` for the Design refs).
They landed anyway because **work was committed without a green verify**, and the
tree had no green `ze-verify` to begin with: the last recorded run failed and
nothing restored green, so the "known-red / scope-to-changed" escape hatch
(`ai/rules/git-safety.md`) became permanent and new breakage piled up under the
existing red. Three concrete gaps enabled it:

1. **A rule with a checker but no gate.** `ai/rules/planning.md` "Design
   references survive closure" names `check_doc_links.py --design-only`, but that
   checker ran only in the standalone `ze-doc-links` target, never in
   `ze-verify`. A rule with no gate rots -- 201 accumulated.
2. **`commit_helper.py` never checked verify-status.** "Verify before commit" was
   honor-system; the tool that writes every commit script had zero references to
   `tmp/ze-verify.status`, so committing over a red/stale verify was frictionless.
3. **Scope-to-changed has a transitive blind spot + no expiry.** It tests
   packages you edited, not packages your edit breaks transitively (a new import
   breaking a different package's compile/test), and nothing forced the red to be
   cleared, so it persisted and hid regressions.

## Fixes applied

1. **Gate the Design-ref checker.** `verify_wiring_docs.py` (the
   `ze-verify-wiring-docs` stage, in both `ze-verify` and `ze-verify-changed`)
   now runs `check_doc_links.py --design-only` unconditionally -- closure debt is
   non-local, so it scans the whole tree every verify, not only on related
   changes.
2. **commit_helper verify gate.** `commit_helper.py create` now runs
   `verify-status.sh check`; if not FRESH it refuses unless `--unverified
   "<reason>"` is passed (owner override, or a known-red logged in
   `plan/known-failures.md`). Honor-system -> enforced-but-overridable.
3. **Tighten the known-red rule.** `git-safety.md`: a red is scope-aroundable
   only if attributed (logged in `plan/known-failures.md` or owned by another
   session); an undocumented red is treated as possibly-yours; check the
   reverse-dependency closure before scoping around; do not let a red persist.

## Reusable lesson

When a bug class is "caught by a check that exists but didn't run," the fix is a
**gate**, not more checks. A rule that names a checker but wires it into no verify
stage is decorative. And a verify that is allowed to stay red across sessions is
not a safety net -- it is camouflage for the next regression. Related:
[[1012-root-layout-reorg]].

## Files

None recorded.
