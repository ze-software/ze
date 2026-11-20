# Handover 52: pipe chain honored on every command surface, from phase 5c

**Spec:** `plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` (in-progress, Phase 5/8; design approved by Thomas on 2026-11-20 as recommended; WIP cap raised to 22 for this spec only)
**Branch:** main
**Goal:** a command's own pipe operations reach its handler as a structured list from every caller, the line is never rewritten, and every surface answers a piped line the same.

The per-spec state file holds every phase handoff, owed command and decision of this session:
`tmp/session/2026-11-19-231156/state/session-state-pipe-chain-honored-on-every-command-surface-52e1b840-3c41-4902-b870-17400c250fbb.md`
(`./le spec state latest spec pipe-chain-honored-on-every-command-surface` finds it). Read it before any source.

## Status

Done (committed):

| Commit | What |
|--------|------|
| b6f2383371 | phase 1 wiring tests |
| 83dfe9c2e6 | TACACS+ mock prints the accounted command |
| 3d7157b2be | phase 2: the chain takes the invocation's quote grammar |
| 67c4ea9356 | defect: web engine stand-in registers the BGP peer summarizer |
| d30af7c2f1 | phases 3a-3c: partition, dispatcher applies the chain, surfaces send the typed line |
| 34cc4767ac | defect: out-excluding-owner declares owner and family (GR replay was refused) |
| ccb004dcfb | defect: missing-selector refusal names what a selector is |
| 07f5b7e39c | phase 4a: stream route, ZE_CLI_RENDER=client, daemon always filters |
| a374677264 | phase 4b: no reader splits a piped line outside the partition |
| 3242d2dfb3 | phases 5a+5b: pipe list on execute-command, SDK handler parameter, PipeDecl.argument, 74 registrants rebuilt |
| 1d95c6879f, 99338af5d0 | spec bookkeeping through phase 4 |

In progress when this session handed off (two agents, uncommitted, each appends its handoff to the state file):
- 5c, the RIB readers: pipeline stages built from the declared list (rib_pipeline.go, rib_pipeline_best.go, rib_inject.go, rib_commands.go, rib_pipeline_covering.go, the forward handlers in cmd/rib/rib.go); `TestRIBJSONTerminalEqualsBareCommand` first, then the `json` terminal word removed (D-7).
- 5d, the RIB declarations and YANG: `registerPipeFilters` on the `Argument` kind; own sets for `show bgp rib protocol` (no scope, D-3) and `show bgp peer <sel> rib` (scope words, D-9); `sent`, `sent-received`, `aspath` declared; both `leaf-list args` and `leaf scope` removed; help_ai.go text.
If either agent's handoff in the state file says "continuation needed", spawn a continuation for exactly what it lists.

5d reported after this handover was written (its full handoff is in the state file): written, nothing run, and blocked on two defects in committed code, both fixes owed in this spec, not owner questions:
- B1: the partition looks up a command's declared operations by the typed command part (`command.ReadDispatchLine`, its `ColumnsForCommand` call, and every surface partition). A selector value inside the path (`show bgp peer edge1 rib | received`) never matches the declaration on `show bgp peer rib`, so the operation is refused as unknown. Fix direction: look up on the matched command's registered path with the selector value removed, which the design implies (operations are declared per command path); every route must use the same lookup.
- B2: `Dispatch` (plugin/server `command.go`) passes `matchedCmd.Name` to `command.RefusePipeOperationWords`, so the fix line drops the selector (`fix: show bgp peer rib | received`, and `show bgp rib protocol | peer X` without `bmp`). Build it from the typed line. `TestPeerRibRefusesScopeArgument` pins the correct form.
- 5d test changes owed to ze-test-judge: TestRibBestFiltersSurviveDeclaration, TestProtocolAndFastpathDispatchTheDeclaredValues, usage_model_test.go (TestUsageRendersTheDeclaredValues, TestModifierGroupsLeaveDispatchUntouched), and removal of `rib/yang_enum_agreement_test.go` (TestRIBScopeKeywordsMatchTheYANGModel and its helpers).
5c also reported (full handoff in the state file): written, nothing run. The rib test build waits on one ze-test-judge removal: `TestBestJSONTerminalRowCarriesNoAge` (D-7 names it; it still uses the removed `terminalJSON`). Also for the judge: 17 rib test files converted by codemod to the pipe-list input (`ribPipe` helper, no assertion changed), `TestParsePipelineTwoTerminals` (`count json` to `count histogram`), the terminal-branch helpers now draining `routesDocument`, and the covering/exact fix lines in pipe form. 5c kept pristine copies of every rib test it edited (`scratch/5c-pristine-*.go` in the session tmp). `TestRIBJSONTerminalEqualsBareCommand` passes before and after the removal, so it is an equality proof, not a red-first test. bmp's `show bgp rib protocol bmp peer <key>` and the lg callers are refused until phase 6 converts them.
- Superseded: when this session handed off, 5c was still mid-edit: the rib package's tests did not compile (`parsePipelineArgs`, `newJSONTerminal` and `showPipeline`'s argument type in transition). Read 5c's handoff in the state file for where it stopped.

Remaining after 5c/5d:
- run 5c/5d owed commands, judge their test changes (ze-test-judge), commit (the rib package reds that pin the removed fold clear here)
- phase 6a-6g: convert every in-tree user of the RIB argument form (census in the spec; about 100 files by tree)
- phase 7a: introspection (`pipe-operations` from one function in internal/component/command; operatorsFor and its le/cli/catalog copy deleted; command help, `ze help command --json`, completion read it); 7b: T-5 C ordered `pipe` list on derived MCP tools and REST/gRPC routes
- phase 8: documentation the Documentation checklist names
- spec bookkeeping for 5a/5b (Phase, Progress, TDD rows), then `/ze-close`

Not done here, owned elsewhere:
- plugin startup refusal log lines (pipe alias / registration conflict / answer shape) have no `ze doctor` check: separate work, named in 3242d2dfb3's body
- the fixit spec `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` cannot take its Depends row until it gains `## Existing Mechanisms`

## Facts the next session needs

- Known reds that are other sessions' (do not chase): pkg/plugin/rpc TestCachedWriteBoundaryRefusesPartialTupleAndMissingCompletion; bgp/plugins/adj_rib_in store/replay tests; filter_path_asn; rpki TestRPKIRetainedCacheChangeAndCleanupBothRails; bgp/reactor TestAIGPReplayKeepsSameAddressOriginals; plugins/container TestFetchCommandDeclaresItsBound; test/fixture tooling tests; config/cli TestRepositoryConfigsValidate; hub configured-plugin conformance tests (c9fbf8964c) and the macOS-only container build-tag test.
- `bin/le test bgp plugin <name>` runs one functional test; it builds the committed tree, so a functional proof follows the commit it proves.
- `./le go lint run scope` refuses to plan while other sessions edit dependencies; commits name that.
- Tests are run with set-aside files only when a removal is approved but not yet committed; none is pending now.
- `tmp/commit-weakening-approved-182f508b.md` holds the session's judge rows; rows consumed by commits stay until `./le commit approval clean` runs.

## Update: phases 5c to 7a landed in the same session

| Commit | What |
|--------|------|
| 615753f1c6 | 5c and 5d (RIB readers take the declared list, json word gone, RIB declarations per command, YANG args and scope leaves removed) plus B1 (declarations resolve behind a selector value) and B2 (refusal fix line keeps the selector) |
| 3c642127ca | phase 6 (every in-tree RIB caller uses the pipe form: lg, bmp, REST, interoplab, fixtures, unit tests, docs) and 7a (one pipe-operations list for help, catalog and completion; quote-safe completer stage) |

Remaining, in order:
- Promote the converted functional drafts after they pass against 3c642127ca: the 14 phase 6 drafts under test/draft/plugin and test/draft/reload, plus filter-wire-batch.ci and dispatch-pipe-chain-applied.ci (each: `./le test bgp plugin --draft --pattern <name>`, the stress repro `ai/skills/ze-test.md` names, then `mv` into test/). `test/plugin/dispatch-pipe-chain-refused.ci` tests the removed refusal: remove it, with its fixture registration in `plugin_fixture_16.go` and `plugin16DispatchPipeChainRefused`, under a ze-test-judge verdict.
- The 7a draft `test/draft/plugin/command-help-pipe-operations.ci` still lacks its AC-19 plugin-command row and AC-20 Tab-completion row.
- Website: internal/le/site (the catalog reader for pipe-operations, census rows, testdata, and the operator-guide golden) is uncommitted and waits for Thomas to review the visible sentence "A command may replace it with its own." beside the overridable generics; the judge approved the test changes. Until it lands, internal/le/site reads the removed catalog keys.
- 7b (T-5 C: ordered optional pipe list on derived MCP tools and REST/gRPC routes), phase 8 (documentation checklist), spec bookkeeping from phase 5 on, then `/ze-close`.
- Approval rows consumed by 3c642127ca were not cleaned (le could not rebuild while another session's credits package lacked its embedded data): run `./le commit approval clean` later.

## Update: implementation complete; closure is what remains

Landed after 3c642127ca: 6537dc0f86 (Thomas's 2026-11-20 override ruling: every data operator overridable keeping its answer shape; the site catalog reader with his approved sentence; 7b pipe list on MCP/REST/gRPC; OpenAPI published unserved routes), 4faad7bbe7 (phase 6 continuation, phase 8 docs, RIB match never read large/extended communities), 5ebf3c1c8e, adfaec2d77, 2f50baa36d (26 functional tests promoted, each stress-run 80 times), 72502f3950 (stream match judged per event; a stopped ze cli monitor commits its | save), 18a3f2c17e (spec at phase 8/8 with evidence per goal). Only test/draft/plugin/rib-advertised-user-path.ci stays a draft: its user update is refused "no peers match selector" in another session's route-origin path (a27a951239); TestSendWildcardSelectorReachesTheUpdateHandler rules this spec out.

The /ze-close review (its full report and prepared edits are in the state file, last section) found 0 BLOCKER and 5 ISSUE, to fix first:
1. internal/le/site/testdata published-configuration.md and published-yang-config-tree.json still show `show bgp rib sent`/`received` (step 6g); refresh through ze-test-judge (golden).
2. `./le doc yang-contract doc-drift` fails: the comparator (internal/le/doc/yangcontract/command_surfaces.go, generatedCommandValueIssue) must accept the site's approved overridable sentence (internal/le/site/commands.go overridableNote).
3. The stream route never checks an override's answer shape (CheckOverrideAnswer only in plugin/server/command.go and command/local_data.go; TestOverrideShapeCheckScope pins streams unchecked): add it in StreamLine.Run, or record a deviation in the spec.
4. AC-17 and its Goal row cite TestRIBOverridesKeepGenericMeaning, which does not exist: cite TestFilterMatchCrossField / TestMatchPoolCommunities or write it.
5. Stale comments naming operatorsFor in internal/component/bgp/plugins/cmd/peer/peer_shape_test.go.
Then: the 15 interop scenarios the close report lists (`INTEROP_SCENARIO=<name> ./le test integration interop`), `./le test weakened check`, `./le repo check`, `./le rfc check`; fill the closure sections and Review Gate (needs the doors and the producing sources read in that context); `./le spec review record ...`; commit A (spec, ai/rules/cli.md, the two journal rows set fixed, commands.md citation, website/changes/fragments/pipe-chain-honored-on-every-command-surface.md, the two container-spec citers, the fixes, and `remove` this handover); commit B (`remove` the spec); `./le spec release`. Uncommitted closure edits already in the tree: the two journal rows, the commands.md citation, the news fragment, ai/rules/cli.md.

## Then

`./le spec claim spec-pipe-chain-honored-on-every-command-surface.md` (the claim may need `ZE_SPEC_WIP_CAP=22`, authorised by Thomas for this spec only), read the state file, then `/ze-implement` from phase 5c.
