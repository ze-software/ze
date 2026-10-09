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
| 1 | Step 2, T3: `command.ArgDef` private fields, per-kind constructors, accessors, D-4, D-7 | `9cbd3485ca` "command: build every ArgDef through a validating constructor" | Done. Owed: web, cmd/ze/hub and plugin/server unit runs did not finish green on a loaded machine (see commit body) |
| 2a | Step 3, T4 precondition: Go imports follow YANG imports (generator, 47 regenerated `register.go`, 50 edges, tier admission of schema-to-schema blank imports, strict `headUsage`) | `7d69d6a669` "yang: Go imports follow YANG imports in generated register.go" | Done |
| 2b | Step 3, T4: strict `DefaultLoader` (D-6), AC-24 | `5eaa197f32` "yang: DefaultLoader is strict, nothing is best-effort" | Done. Owed: `./le verify worktree` (never run over batches 1-2) |
| 3 | Step 4, T1: `Resolve` returns `yang.Resolved`, the one checked-schema reader; includes the `NewCompleter` error fix and the stale-comment fixes | `180df07922` "yang: Resolve returns yang.Resolved, the one checked-schema reader" | Done. Owed: `./le verify worktree` (never run over batches 1-3) |
| 4a | Step 5, C-T2a: `command.ValidatedArgs`, every route R1 to R9 validates, `test/ui/cli-argument-refused-plugin-route.ci` (R9), root-fallback R6 unit test | `62842be4c7` "command: every dispatch route validates its arguments" | Done. Owed: `./le verify worktree`; the `.ci` needs `./le --update` first |
| 3b | Compound-guard splits: the seven `if a \|\| b { leave }` guards in `command/argbind.go`, `command/usage.go`, `plugin/server/command.go`, `cmdutil/cmdutil.go`, one fact per guard, behavior unchanged | `93ee8363ce` "command: split compound guards into one fact per guard" | Done |
| 4b | Step 5, C-T2b: `pluginserver.StreamingHandler` takes `command.ValidatedArgs`; 5 handlers, R8 (`service_ssh.go`, `api.go`), every test call; `[]string` signature deleted; `commands.md` updated; spec C-T2b evidence line | not committed | Implemented and green, held in the working tree. `./le commit create` refuses it: two RFC8907-8.3-4 tagged units in `cmd/ze/hub/rfc8907_api_test.go`, `hub.TestAPIStreamSourceRunsStreamingHandler` and `hub.TestAPIStreamSourceAuthorizesReadOnly`, change only the handler parameter type (`[]string` to `cmd.ValidatedArgs`, `command` is shadowed there by a local const, so the import is aliased `cmd`) and the first reads `args.Tokens()` in place of copying the slice; no assertion changes. Owed: the owner's words, then `./le rfc approve unit <unit> reason "<words>"` for both, then rerun the same `./le commit create` (17 files, listed in the spec's C-T2b evidence line and in the report) |

C-T2a: done (`62842be4c7`, batch row 4a). The per-route reds are recorded in the spec (C-T2a evidence line).

- Gates seen: `./le repo compiles check` OK on every flavor at `62842be4c7`; `./le go lint run scope ./cmd/ze/...` 0 issues on every flavor. Full-tag `-race` over hub, cli, command, plugin/server, cmdutil, cmd/ze and test/cli on a loaded machine: hub, cli and cli/testing hit the 10m timeout with tests still advancing, and `TestPluginStateTransportParity` failed. All four passed when rerun alone (hub 350s, cli 835s, cli/testing 290s, parity 4s). `internal/component/cli` under `-race` takes longer than the default 10m on this machine even alone, so it needs `-timeout` raised.

- `./le rfc check` RFC8907-3.7-2, RFC8907-8.3-4 and RFC8907-10.5.1-1 STALE, and the 3.7-2 and 8.3-4 discrimination records, are not C-T2a's: the tagged test files and the recorded producers (`tacacs/text.go::validateText`, `cmd/ze/hub/aaa_lifecycle.go::commandStop`) are untouched by it, and batch 1's notes already listed the same findings as foreign (journal `plan/journal/concurrent-rfc-gate-stale.md`, 2026-10-09).
- C-T2c/C-T2d note: `command` imports `command/registry`, so `LocalHandler` and `LocalDataHandler` cannot take `command.ValidatedArgs` where they are declared. Proposal (not implemented, needs a decision before C-T2c): a leaf package under `internal/component/command/` with Go internal visibility (for example `command/internal/args`), imported by both `command` and `registry`, holding the validated-arguments type. It is constructible only through `command.ValidateArgs`, so no route can hand a handler unjudged tokens. The old type is deleted in the same change (no-layering).
- Behavior changes to know: R6 and R7 now load the YANG model (`DefaultLoader`) on every `ze <verb>` local command and refuse a missing mandatory leaf with `required argument missing: <leaf>` where the handler printed its own usage before. R9 judges a builtin proxy's forwarded tokens again for the plugin's path, with `ctx.Selectors` pre-matched.

Batch 3 notes for the next batch:

- Gates seen after `93ee8363ce`: `./le repo compiles check` OK on every flavor. `./le go lint run scope ./internal/component/web/...` and `./le go lint run scope ./cmd/ze/hub/...`: 0 issues on every pass (owed from batch 3). `./le arch compound-guard check`: none of this phase's lines; the one finding left is in `checkZeAccessConcentratorPAP` (`internal/le/interoplab/pppoe/check_pap.go`), from another session's unpushed `62b1df5249`.
- `./le rfc check`: 11 rfc7950 verdicts are STALE and 9 are SHIFTED, all from mechanical changes. STALE: 6.1.3-1, 6.2-1, 6.5-1, 7.6.3-1, 7.19-1, 9.3.4-1, 9.4.4-1, 9.6.4.2-1, 11-3, 7.2.2-1, 7.3.2-1. SHIFTED: 9.6-1, 9.12-1, 7.6.5-1, 7.3.4-1, 7.6.4-2, 7.7.4-1, 8.1-3, 7.9.2-2, 9.4.4-2. An independent `ze-rfc-audit` re-judge (`mode rejudge`) of these 20 is running; check `git log -- rfc/audit` before acting. Never `./le rfc reseal`.

Batch 2 notes for the next batch (verified 2026-10-09 at the source):

- Generator: `internal/le/yang/glue/imports.go`, `dependencyImports` parses every `.yang` with goyang and maps each `import`/`include` to the `yang/` package holding it; a dependency held by the registry's embedded bootstrap modules or by the same package gets no import; one no package holds fails with `errUnheldDependency`. `registerSource(modules, module, dependencies)` renders the blank imports. Test: `TestRegisterImportsEveryCrossPackageDependency`.
- `TestYANGImportsFollowGoImports` (`internal/component/plugin/all/yang_imports_test.go`) checks the same property over the registered set from the Go side. It was red on exactly the 50 edges before the regeneration.
- Tier: `schemaDependency` (`internal/le/arch/tier/ownership.go`) admits a blank import from one `yang` directory of another (owner approval 2026-10-09); a named import, or a schema package importing an implementation package, still fails (`ownership_test.go`, 3 cases).
- `DefaultLoader` (`internal/component/config/yang/loader.go`) = `errors.Join(LoadRegistered(), Resolve())`, nil loader on any error; `LoadRegistered` attempts every module and names each failure. `Resolve` already joins `process`, `checkExtensions`, `checkPatterns`, `checkStructure`. The 21 other production callers already returned or reported the error; `startWebServer` (`cmd/ze/hub/service_web.go`) now disables the web server with a warning, as it does on a schema failure.
- `headUsage` (`internal/le/doc/yangcontract/usage.go`) resolves strictly: it was a third policy that discarded every `Resolve` error but `ErrUndeclaredExtension` (spec T4 table row).
- Reproducible: `./le yang glue check` reports all 162 `yang/` directories current, and `./le --update yang glue write` leaves the diff unchanged.
- Gates seen: `./le go lint run scope` clean on plugin/all, le/doc/yangcontract, cmd/ze/hub, config/yang, le/yang/glue, le/arch/tier. `./le repo compiles check` OK on every flavor at `7d69d6a669` and at `5eaa197f32`. `./le arch compound-guard check`: no batch-2 line (an `||` guard in `imports.go` was split before the commit).
- `./le rfc check`: the same 8 findings as after batch 1. RFC7950-9.6.4.2-1 SHIFTED is batch 1's (see the batch 1 note); the other 7 are foreign. Batch 2 added none.
- `TestTheRealCheckoutPassesAndWasRead` (`internal/le/cli/grammar`) is red at HEAD `8b1af26f28` without batch 2 (run on a `git archive` export) on five R1 root commands. Not batch 2's; journal row in `plan/journal/gate-red-where-nothing-blocks-on-it.md`.
- Full-tag `-race` run over every package with a `DefaultLoader` path, plus `cmd/ze` and `plugin/all`, before the commits: 29 ok, 3 red with no loader-error line: hub `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds` (timing), plugin/server `TestPluginStateTransportParity/socket` (deadline, also red before batch 2), and the grammar test above. Gate-free (no feature tags) runs show reds from absent gated BGP ("unknown top-level keyword: bgp"), also with no loader-error line.
- Open for the next batch: `./le arch compound-guard check` flags seven `if a || b { leave }` guards on unpushed lines outside batch 2: `positionalDef` and `positionalError` (`command/argbind.go`), `Usage` and `appendLeafTokens` (`command/usage.go`), `anchoredDef` and `implicitSelectorDef` (`plugin/server/command.go`), `hasImplicitSelectorArg` (`cmd/ze/internal/cmdutil/cmdutil.go`). Check which ones batch 1 introduced and split those.

Batch 1 notes for the next batch:

- `ArgDef` and its constructors live in `internal/component/command/argdef.go` (moved out of `node.go`). `ErrArgDef` wraps every refusal; `constructed` is the private marker `ValidateArgs` and `ValidateArgString` refuse a zero definition on (D-4).
- `WriteInvocation` takes `[]command.InvocationArg` (name, anchor, flag). Web projects with `command.InvocationArgs`; MCP builds it in `invocationArgs` (was `argDefs`), D-7.
- Tests outside `command` build definitions with `commandtest.Must(command.NewXArg(...))` (`internal/component/command/commandtest`); inside `command`, `mustArgDef` in `node_test.go`.
- `applyRange`, `applyLength` and `applyPatterns` are gone: `yangTypeToArgDef` takes the `ArgOptions` and calls the constructors, and the pattern compile, with its "pattern Loader.Resolve refuses" BUG, is now `compilePatterns` in `config/yang/command.go`. T1 (step 4) removes that BUG branch under its new name.
- A-11 is confirmed (spec row): goyang's range parsing sorts, coalesces and bounds parts, so the constructor refusal in `yangTypeToArgDef` is a named BUG. `TestCommandTreeBuildsFromEveryRegisteredModule` lives in `config/yang/command_registered_test.go` (external test package, it imports `plugin/all`), not in `command_test.go`.
- `validateUint` no longer defaults a zero width to 64: the constructor refuses it.
- Owner answers recorded 2026-10-09: cross-plugin `*/yang` blank imports approved (spec T4 section); goyang CLA signed (row above).
- `usageValues` (`command/usage.go`) copies an enumeration's values, so a `UsageToken` does not alias the definition it was rendered from.
- Open after batch 1: `./le rfc check` reports RFC7950-9.6.4.2-1 SHIFTED. The audit fingerprint is file-level, and batch 1 had to edit `config/yang/rfc7950_enum_value_test.go` (one untagged line, `def.EnumValues` no longer compiles); the four tagged units are byte-identical. Do not run `./le rfc reseal` (it rewrites every RFC's audit file); clear it with an independent `ze-rfc-audit` re-judge of that one verdict (`mode rejudge`), folded into the next batch that touches rfc7950.
- `test/weakened/<session>.md` holds only the rows the next commit owes: `./le commit create` drops rows whose text HEAD already has (`docs/contributing/testing.md`). An empty table after a commit is correct; do not "restore" landed rows. The prepared script `tmp/commit-f93f1d5f-d-8a429f.sh` did that and was deliberately not run. The other seven `rfc check` findings (L2TP/EAP naming, RFC8907 stale and discrimination) predate batch 1.

## Owner exception, 2026-10-09

Owner, verbatim: "I give you an exception for the rule". It answers the question
about batch 5 (C-T2e, the `pluginserver.Handler` change across 69 directories):
for that batch, the rewriter MAY edit a file that holds another session's
uncommitted hunks (`.claude/rules/foreign-files.md` waived for C-T2e only).
Still owed: rewrite only the handler-signature lines the batch needs, leave the
foreign hunks byte-identical, and name in the commit body every file whose
foreign hunks rode along (`ai/rules/git-safety.md`, judged against HEAD). The
RFC-tagged-test approval is NOT covered: it is still asked per unit.

## What the next session needs to know

- Already committed: the YANG loader returns its errors (ac6c5ce12d, a5b3180063, 40db22a585), plus its structural and grammar checks (12f339513b, 6c06cb43af, 037cc25e29, a812b18c41, 7070e5861a). `./le rfc check` reports no RFC 7950 finding after 9cbf26548c.
- Only the daemon-builtin and local-data routes validate arguments today (journal `silent-fall-through.md`, edd649a9cc). R4 to R9 are D-3's work.
- The style rule this phase serves: `docs/contributing/ze-go-style.md`, "One type per lifecycle state" (67f55d1b6e). `ze-implement` and `ze-review` check it (4be13ea754).
- Known pre-existing red: `TestRFC7950MandatoryUnderAbsentNonPresenceContainer` in `internal/component/config`.
- Run tests with the feature tags from `feature-gates.txt`. Without them you get a false "no such module: ze-bgp-conf".
- `go mod vendor` reverts the hand-patched vishvananda/netlink (f0d9c75df4): patch `vendor/` by hand for a dependency change. Follow-up for RFC7950-9.6.4.2-1 (still weak): tag `goyang_enum_numbering_test.go` as a positive and record its discrimination against the reverted goyang `EnumType.Set`.
- Changing an RFC-tagged test needs `./le rfc approve unit <unit> reason "<owner's words>"` before `./le commit create`.
- Never run `./le rfc reseal`: it rewrites every RFC's audit file, not only the one in hand.
- Lint one package: `./le go lint run scope ./<pkg>/...`. A bare path to a feature-gated package (`internal/component/web`) fails with "go list for lint flavor host returned code 0 with no output", because the host flavor's tags exclude it.
