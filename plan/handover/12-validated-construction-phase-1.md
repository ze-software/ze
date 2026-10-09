# Handover: validated construction, Phase 1

Spec: `plan/spec-validated-construction-and-state-types.md`, Phase 1.
Written 2026-10-09. Development continues on another machine, so this file lives
in the repo: `tmp/` is gitignored and never leaves the machine it was written on.

**Resume here.** Read this file, then the spec's Phase 1 section, before any
code. Update the batch log below in the same commit as each batch.

## Owner decisions (2026-10-09, verbatim)

| D | Answer | Reading in force |
|---|--------|------------------|
| D-1 | "yang.Resolved" | The resolved type is `yang.Resolved` |
| D-2 | "every handler signature" | Option A: every author-facing handler type (`pluginserver.Handler`, `registry.LocalHandler`, `LocalDataHandler`, `StreamingHandler`) takes the validated-arguments value. About 290 production handlers and 560 test call lines |
| D-3 | "Validate on all of them" | Routes R1 to R9 all call `command.ValidateArgs` |
| D-4 | "Refuse it in the validator." | A zero `ArgDef` is refused by `ValidateArgs` |
| D-5 | "One type with per-kind constructors" | `ArgDef` has private fields and one constructor per kind |
| D-6 | "Keep it; make it strict" | Reading NOT confirmed by the owner: `DefaultLoader` stays, and becomes strict (no discarded error). This changes behaviour: whatever loads today only because an error is discarded will fail |
| D-7 | "The narrower input" | MCP `WriteInvocation` takes a name-and-anchor input |

Owner instruction, 2026-10-09: "do it by batches and save progress to resume
correctly in another session".

## Before batch 1

| Item | State on 2026-10-09 |
|------|---------------------|
| Spec records the decisions, the D-2 A batch order and the D-6 census | Agent running when this was written; check `git log -- plan/spec-validated-construction-and-state-types.md` |
| go.mod `replace` of goyang with the ze-software fork (enum numbering fix, journal `plan/journal/zero-value-as-valid-answer.md` row 4) | BLOCKED on a push only the owner can run. Fork https://github.com/ze-software/goyang exists. Fix commit a3cf525c4f98ce2f6946e7ba9ae9849886a20070 on branch `fix-enum-implicit-value-after-negative` (from upstream master a80f279), in a Linux-only scratch clone; if that clone is gone, redo the one-line fix in `types_builtin.go` `EnumType.Set`: record the first value unconditionally (`len(e.ToInt) == 0 \|\| value > e.last`), plus `TestTypeResolve` cases `-5, q, 0` and `-10, q`. After the push: `replace` in go.mod with a comment naming the upstream PR, `go mod tidy`, `go mod vendor` (module mode does not build this tree), add the Ze test (load `enum p { value -5; } enum q; enum r { value 0; }` via `AddModuleFromText`+`Resolve`), and update the now-stale text in `enum_assignment.go` (lines 22-23), the two `rfc7950_enum_value_test.go` doc comments, `yang-config-design.md` (~line 189) and journal row 4. Upstream PR needs the owner to sign the Google CLA |
| `./le verify worktree` over the 2026-10-08/09 commits | Owed, never run |

## Batch log

| # | Batch | Commit | State |
|---|-------|--------|-------|
| - | none started | - | - |

## What the next session needs to know

- Already committed: the YANG loader returns its errors (ac6c5ce12d, a5b3180063, 40db22a585), plus its structural and grammar checks (12f339513b, 6c06cb43af, 037cc25e29, a812b18c41, 7070e5861a). `./le rfc check` reports no RFC 7950 finding after 9cbf26548c.
- Only the daemon-builtin and local-data routes validate arguments today (journal `silent-fall-through.md`, edd649a9cc). R4 to R9 are D-3's work.
- The style rule this phase serves: `docs/contributing/ze-go-style.md`, "One type per lifecycle state" (67f55d1b6e). `ze-implement` and `ze-review` check it (4be13ea754).
- Known pre-existing red: `TestRFC7950MandatoryUnderAbsentNonPresenceContainer` in `internal/component/config`.
- Run tests with the feature tags from `feature-gates.txt`. Without them you get a false "no such module: ze-bgp-conf".
- Changing an RFC-tagged test needs `./le rfc approve unit <unit> reason "<owner's words>"` before `./le commit create`.
- Never run `./le rfc reseal`: it rewrites every RFC's audit file, not only the one in hand.
- Lint one package: `./le go lint run scope <pkg>`.
