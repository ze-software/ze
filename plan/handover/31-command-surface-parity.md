# Handoff: command-surface-parity, ready for `/ze-close`

Spec: `plan/release/fixes/spec-command-surface-parity.md` (status in-progress).
Per-spec session state, with every phase handoff and the judge verdicts:
`tmp/session/2026-11-01-124358/state/session-state-command-surface-parity-52f37a05-b818-4d4e-a6c8-43342eab7b4a.md`.
The full commit list is `git log --grep 'Spec-work: command-surface-parity'`
(84 commits on 2026-11-02, the plan and journal commits included).

## Status

Every phase landed. `TestEveryCommandOnEverySurface` is green with every
surface tag since a5a167987f, which fixed its last red (`show vrrp interface`
on shell completion). What remains is closure, below.

## What landed, by phase

| Phase | Commits |
|-------|---------|
| 1. Wiring and red check | 3ffb9b5810 |
| 2. Handler contract: answers data, refusals as values | db665e072e, 90adb048ca, 12eac87f4c, f413de06ec, bd5de388c2, 27db274dac, 0da691eacc, ea02a748c7, 099f1c31bb, 93d7042b9d, 442e4e810e, d4f6c88e54, 89069bb7a0, ecbfca85fd, 87fac7a3ae |
| 3. Daemon bridge, path confinement, refusal text | 2f57340305, c49b8dbdbe, 90dfb4432d, 1e7259c400, afdb50bd52, 8653bf598c, 24fb75a3ad, 896a898359, d4f1b485bf, 127c8caa8c, c795f0545e |
| 4. One producer per question | 186c6a07b0, 5a91877b02, 8e81d931c7, 4c12704fe9, 0b403ea3d3, b6bd45b0f2, 91a1faf917 |
| 5. Storage and lifecycle (support archive, update serve) | fa1d850651, 3d9f9d4ad6, 393ef8ce93, 54aaf8f1e2, 3880a118f6, 9b881c9fa2, 060e65ee6a, ab3d61811e, bcec5722d9, 8814a89950, 7c916edfc5, 092105f408 |
| 6. Predicate and surface producers | 5a28ec2260, 7e9f9b5097, 53908ba644, 75cd4f14ed, 2b69bcea50, 9796af55cc, 2dbec6d7b6, 3f30c4f047, c68890c491, 0ee136653b, b8c08b0215, a5a167987f |
| 7. Streaming one-shot answers, monitor streams, catalog labels | 90cd226a7f, c44e1bdc8c, ac3ff0fc1f, 905dc173a5, 5b470c18c5, 592e54c7c5, 925f8bf969, b9ab3fe5e1, 1053c3cbdb, 72dbcb659c, 7b6ec42e3f, 70242945b4, 7aba92d650, 50449f4483, eae9a7f41a |
| Plugin warning text (owner decision) | 5e33094392, 90e124ec67 |
| 8. Docs | fe21373773, 7c5339d4d8 |
| Lint (spec lines only) | e8d71011de |
| Plan and journal | 232ad2a899, 432e04f81f, decafc4b30, cbc558c40b, ed5568a063, 4100e0a64c, and the commit that carries this file |

`./le repo compiles check` passed on e8d71011de in every flavor, which also
covers a5a167987f.

## What remains for `/ze-close`

| Item | Detail |
|------|--------|
| Red in this spec's own test, fixture fix awaiting the judge | `TestLocalCommandDaemonMatchesLocal/request_support_archive` (`cmd/ze/hub`, tags ze_core ze_bgp ze_web ze_mcp ze_ssh ze_rest ze_grpc ze_vrrp) fails at `local_bridge_match_test.go`:146: the local route exits 0 for `request support archive ze-match-no-such-word`, the daemon refuses it as a file outside the configuration directories. Cause: since 127c8caa8c, `command.CallerPath` on the daemon route resolves a relative name against the working directory and accepts it only inside the configuration directories, which no longer include the working directory. `matchRoutes` passes because the test pins `ze.config.dir` to its working directory; `matchArtifact` moves `ze.config.dir` to a directory of its own while the named file stays in the old working directory, so the daemon route refuses it as the owner decision requires. The product is right and the fixture is wrong for the new contract. The fix (in the working tree, not committed): `matchArtifact` works in its own config directory and writes the same files there, so both routes answer again and the documents are still compared. Patch: `tmp/session/2026-11-01-124358/scratch/final/support-match-judge.patch`. Red before: `tmp/session/2026-11-01-124358/scratch/job-lintfixt-a22807fa.log`; green with it: `tmp/session/2026-11-01-124358/scratch/job-supportfix-825a34d4.log`. It lands after a `ze-test-judge` verdict |
| Review gate | `/ze-review` over the whole spec diff. The main thread has not read the whole diff; each phase had its own review, the union has not |
| Lint findings left | In packages this spec touched, on lines other sessions wrote (not fixed): `internal/component/plugin/coordinator_test.go`:140 errcheck (fde38649c5), `plugin/ipc/filter_batch_test.go`:24 unparam (29fc996ece), `plugin/registry/filter_declaration.go`:47 modernize (1a179d341e), `plugin/server/filter_reference_alias_test.go`:84 misspell and :112 modernize (e05a3969a7), `plugin/server/startup_chain_refused_test.go`:94 misspell (9f834185a2), `plugin/server/startup.go`:1320 rangeValCopy (1a179d341e) |
| Journal rows | `registry-contamination.md` 2026-11-02 (the reload decomposer test leaves its decomposer registered, so the binder reload test panics under `-count=2`), owned by `spec-plugin-reload-applies-or-refuses`. The warning-text row (`format-cannot-hold-a-state-the-tree-holds.md` 2026-11-02) is marked fixed |
| Drafts awaiting the owner | `test/draft/plugin/zzred-*.ci`: gitignored red variants kept from the rate test proofs. Deleting a test path needs approval; nothing reads them |
| Gates owed | `./le verify changed` over the closing commits is not run by agents (owner rule); the scoped tests named in each commit are the evidence |

## Owner decisions taken in this work

- The daemon serves every local and local-data command through the same
  registered handler, so every command surface gives the same answer as `ze`.
- `ze doctor`, `ze help command` and `ze help ai` get daemon forms under
  `show`, linked by `registry.Meta.DaemonPath`.
- Every monitor command streams on `ze monitor`, `ze cli`, SSH interactive and
  SSH exec; one-shot surfaces answer one sample or refuse a streaming-only
  command.
- gNMI is not a checked command surface.
- The daemon's working directory is not an allowed directory for a file
  argument on the daemon route; the local route keeps it.
- A plugin that dispatches a command receives its warning text through an
  optional field of the public plugin RPC answer and the SDK, gated on Stage 1
  protocol version 2.
- No separate independent read of 232ad2a899 (plan only): the closure review
  gate reads the spec.
