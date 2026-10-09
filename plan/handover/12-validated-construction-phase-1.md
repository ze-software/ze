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
| D-6 | "Keep it; make it strict", then "change behaviour is fine, make it strict" | Confirmed 2026-10-09: `DefaultLoader` stays and becomes strict (no discarded error). Whatever loads today only because an error is discarded will fail, and the owner accepts that change |
| D-7 | "The narrower input" | MCP `WriteInvocation` takes a name-and-anchor input |

Owner instruction, 2026-10-09: "do it by batches and save progress to resume
correctly in another session".

## Before batch 1

| Item | State on 2026-10-09 |
|------|---------------------|
| Spec records the decisions, the D-2 A batch order and the D-6 census | Agent running when this was written; check `git log -- plan/spec-validated-construction-and-state-types.md` |
| go.mod `replace` of goyang with the ze-software fork (enum numbering fix, journal `plan/journal/zero-value-as-valid-answer.md` row 4) | DONE in the commit that carries this row (subject "deps: build goyang from the ze-software fork with the enum numbering fix"). go.mod replaces goyang with github.com/ze-software/goyang v1.6.4-0.20261009101552-a3cf525c4f98 (branch `fix-enum-implicit-value-after-negative`); upstream PR https://github.com/openconfig/goyang/pull/317. Vendor updated by hand for goyang only, because `go mod vendor` would revert the hand-patched vishvananda/netlink (f0d9c75df4). The owner signed the Google CLA for the PR on 2026-10-09. Still owed: drop the replace once a goyang release holds the fix |
| `./le verify worktree` over the 2026-10-08/09 commits | Owed, never run |

## Batch log

| # | Batch | Commit | State |
|---|-------|--------|-------|
| 1 | Step 2, T3: `command.ArgDef` private fields, per-kind constructors, accessors, D-4, D-7 | "command: build every ArgDef through a validating constructor" (this commit) | Done |

Batch 1 notes for the next batch:

- `ArgDef` and its constructors live in `internal/component/command/argdef.go` (moved out of `node.go`). `ErrArgDef` wraps every refusal; `constructed` is the private marker `ValidateArgs` and `ValidateArgString` refuse a zero definition on (D-4).
- `WriteInvocation` takes `[]command.InvocationArg` (name, anchor, flag). Web projects with `command.InvocationArgs`; MCP builds it in `invocationArgs` (was `argDefs`), D-7.
- Tests outside `command` build definitions with `commandtest.Must(command.NewXArg(...))` (`internal/component/command/commandtest`); inside `command`, `mustArgDef` in `node_test.go`.
- `applyRange`, `applyLength` and `applyPatterns` are gone: `yangTypeToArgDef` takes the `ArgOptions` and calls the constructors, and the pattern compile, with its "pattern Loader.Resolve refuses" BUG, is now `compilePatterns` in `config/yang/command.go`. T1 (step 4) removes that BUG branch under its new name.
- A-11 is confirmed (spec row): goyang's range parsing sorts, coalesces and bounds parts, so the constructor refusal in `yangTypeToArgDef` is a named BUG. `TestCommandTreeBuildsFromEveryRegisteredModule` lives in `config/yang/command_registered_test.go` (external test package, it imports `plugin/all`), not in `command_test.go`.
- `validateUint` no longer defaults a zero width to 64: the constructor refuses it.
- Owner answers recorded 2026-10-09: cross-plugin `*/yang` blank imports approved (spec T4 section); goyang CLA signed (row above).
- `usageValues` (`command/usage.go`) copies an enumeration's values, so a `UsageToken` does not alias the definition it was rendered from.
- Open after batch 1: `./le rfc check` reports RFC7950-9.6.4.2-1 SHIFTED. The audit fingerprint is file-level, and batch 1 had to edit `config/yang/rfc7950_enum_value_test.go` (one untagged line, `def.EnumValues` no longer compiles); the four tagged units are byte-identical. Only `./le rfc reseal` clears it, and it rewrites every RFC's audit file, so it waits for the owner's decision. The other seven `rfc check` findings (L2TP/EAP naming, RFC8907 stale and discrimination) predate batch 1.

## What the next session needs to know

- Already committed: the YANG loader returns its errors (ac6c5ce12d, a5b3180063, 40db22a585), plus its structural and grammar checks (12f339513b, 6c06cb43af, 037cc25e29, a812b18c41, 7070e5861a). `./le rfc check` reports no RFC 7950 finding after 9cbf26548c.
- Only the daemon-builtin and local-data routes validate arguments today (journal `silent-fall-through.md`, edd649a9cc). R4 to R9 are D-3's work.
- The style rule this phase serves: `docs/contributing/ze-go-style.md`, "One type per lifecycle state" (67f55d1b6e). `ze-implement` and `ze-review` check it (4be13ea754).
- Known pre-existing red: `TestRFC7950MandatoryUnderAbsentNonPresenceContainer` in `internal/component/config`.
- Run tests with the feature tags from `feature-gates.txt`. Without them you get a false "no such module: ze-bgp-conf".
- `go mod vendor` reverts the hand-patched vishvananda/netlink (f0d9c75df4): patch `vendor/` by hand for a dependency change. Follow-up for RFC7950-9.6.4.2-1 (still weak): tag `goyang_enum_numbering_test.go` as a positive and record its discrimination against the reverted goyang `EnumType.Set`.
- Changing an RFC-tagged test needs `./le rfc approve unit <unit> reason "<owner's words>"` before `./le commit create`.
- Never run `./le rfc reseal`: it rewrites every RFC's audit file, not only the one in hand.
- Lint one package: `./le go lint run scope <pkg>`.
