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

## Then

`./le spec claim spec-pipe-chain-honored-on-every-command-surface.md` (the claim may need `ZE_SPEC_WIP_CAP=22`, authorised by Thomas for this spec only), read the state file, then `/ze-implement` from phase 5c.
