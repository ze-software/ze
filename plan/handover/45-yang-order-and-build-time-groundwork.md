# Handover: YANG definition order, and groundwork for build-time YANG

Written 2026-11-18. Branch `worktree-yang-compiled`, worktree
`.claude/worktrees/yang-compiled` on the server, based on main at `8fb9a9295e`.
Nothing is pushed and nothing is on main.

## What the branch holds

| Commit | What |
|--------|------|
| `a5975cc926`, `86cddcd891`, `2b607beaba` | The build-time YANG spec re-audited against today's tree, steps P0 to P3 recorded, the design reworked under the owner's answers |
| `c1b1ad36c6` | L2TP LCP defaults read the resolved module set (P3, AC-17) |
| `91aa35862c` | The external-plugin join judges only external modules (P2, AC-16); fix: a duplicate-node `augment` is refused where it was dropped in silence (AC-20) |
| `fa47934b25` | `checkStructure` walks each module once (P1, AC-15) |
| `d0d2fd1d88` | Staged benchmark of the schema build (P0, AC-19) |
| `0bd72d74ab` | Journal: two hook defects met in a nested worktree |
| `43b03dcca6`, `0290d55afe`, `81ff5ecf79`, `f7cbd72b27` | The definition-order fix: audit, the order function and every producer, unit tests on every printer, three functional tests |

## Definition order (`plan/release/fixes/spec-config-yang-definition-order.md`)

Built and proven. Not closed: the spec's closure (review gate, the two closing
commits) has not been run.

| Fact | Evidence |
|------|----------|
| Every producer of child order calls `definitionOrder` (`internal/component/config/yang_schema_order.go`); the name-order functions are deleted | `0290d55afe` |
| The unit order tests and the three functional tests go red against the old converter | Observed, both |
| Three existing tests and three web goldens changed expectation | Each approved by an independent judge; trailers in `0290d55afe` |
| Eight lists change what a one-line positional entry means; an old-order line is refused by type, with one narrow exception (`plugin hub server`, a secret that is an IPv6 literal) | Spec A-1; no Ze config in the repository writes one |
| Two positional guards stay, over two module sets (config tests' set, and the full build in `plugin/all`) | The judge refused removing the first: builds may drop CoS or BGP |

## Build-time YANG (`plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md`)

Not started: no generator, no compiled type, no generated file. Status `design`.

| Owner decision, 2026-11-18 | State |
|----------------------------|-------|
| "Per module." (A5; way 3 dropped) | Recorded in the spec and the design page |
| "No leaf ref must work cross module." | Recorded; every leafref is checked at generation inside its import closure, nothing valid refused |
| "Yes, I keep the order of the definition." | Built, above |
| The `--plugin` text hand-over may be deleted ("Okay, do what you need.") | Recorded for C3 |
| The reworked design as a whole | NOT approved: it was explained to him and he has not answered |

The reworked design is one agent's work and has had no independent review. The
size is about 35 to 45 agent work packages. It depends on the one-definer fix
(`plan/release/fixes/spec-config-yang-one-definer-per-node.md`, status
`design`), which is not built.

## Reds seen on this branch that it did not cause

All present at the base commit, causes named in the commit bodies.

| Red | Cause |
|-----|-------|
| `TestAdminExecuteAppendsPostedArguments`, `TestWebHandlerGoldenOutput/post-config-inspect*`, `TestBrowserSuppliedInspectionPasteReachesItsOwner`, and very likely `cmd/ze/hub` `TestWebSuppliedInspection*` | `82b1aab730`: a blank optional form field is written as an empty argument and refused |
| `TestRepositoryConfigsValidate` (18 findings), `validation/enum-refusal-commit.et` | Incomplete dissemination grants (`4a0c0f468a`); a device name prefix clash in `test/reload/cos-profile-reload.ci` |
| `internal/component/plugin/all` `TestGeneratedPluginImportsCurrent`, `TestFeatureGatedModulesEnumerated` | `all_ze_exabgp.go` lacks the import of `internal/plugins/exabgp/yang` |
| `cmd/ze/hub` `TestGraphicalImport*`, `TestWebCLIViewActionRouteRefusesUnknownHandle`, `TestLocalCommandDaemonMatchesLocal`; `internal/component/l2tp` `TestKernelPPPAdmission*` | Recorded as other work, or unattributed: not run at the base commit |

## Next

1. Land the branch on main. Main has moved since `8fb9a9295e`, so this is an
   integration, not a fast-forward.
2. Close the definition-order spec.
3. Put the reworked build-time design to the owner again, after an independent
   review of it, before anything of C1 to C3 is built.
