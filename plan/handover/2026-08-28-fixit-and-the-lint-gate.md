# Handover: the fixit backlog, and the gate that blocked the whole checkout

Written 2026-08-28, session `fix-it`. Read this before touching anything.

## Read first, in this order

1. This document, whole.
2. `plan/spec-fixit-peer-pending-sync-settles-too-early.md`, the re-cut Task section at the top. It is the only unfixed RFC MUST defect this session found.
3. The four RFC 9552 rows in `rfc/short/rfc9552.md`, lines 731-737.

## The one-line state

Nothing is committed. `verify-lint` went from thousands of findings to zero, `doc-check/links` from 549 to zero, `rfc/check` from 5 violations to 4, every fixit spec is reviewed and classified, and a full `./le verify current mode full` was running at handover time with two reds already visible.

## The situation you are inheriting

Three other Claude sessions share this checkout and were all working while this ran:

| Session | What it did |
|---------|-------------|
| `le-reorg` | Renamed 13 directories under `internal/le/`, landed 7 commits, is implementing a CLI namespace spec next |
| `green` | goconst in `internal/component/bgp/plugins/**` and `internal/plugins/ddos/**` |
| a fourth | Left `.go` files with mismatched package names in its scratch dir, which poisons gopls |

Commits `53cc3109d` through `e4ad48eed` are theirs, not this session's. **Every path under `internal/le/` moved.** The mapping is in `docs/architecture/core-design.md`; the ones that bite are `lintgate -> verifylint`, `speclifecycle -> specsession`, `weakened -> testweakened`, `verifyworktree -> verify`, `sitebuild -> site`, `devsetup -> setup`, `aisync -> ai`, `lejob -> job`.

## Why nothing is committed, and it is not slowness

`./le commit create` refuses every commit in this checkout. Two conditions hold together:

- `./le verify status check` is not FRESH.
- A deterministic structural gate is recorded red in `tmp/ze-verify-failures.json`.

**The mechanism, verified at the producer, and it is the most important thing in this document.** `structuralGateReds` (`internal/le/commit/verification.go`) reads that JSON. A stage that declares no failure group gets one synthesised by `verifyengine/artifacts.go` as `{Kind: "generic", Related: []string{}}`. `groupRelatedPaths` returns nil for any kind that is not `files`, `lint` or `package`. A group naming no path is *unattributable*, and an unattributable red is charged to **every** commit regardless of its file list.

Only two of the 43 stages publish real groups: `docwiring/groups.go` and `functional/run.go`. **So any one of the other 41 going red blocks the entire repository, not the commit that caused it.** That is what happened tonight, and it is the single highest-value `le` fix outstanding.

Do not repeat this session's mistake: `rfc/check` prints file paths in its OUTPUT TEXT. That is not a failure group and never reaches the attribution machinery.

## What the verify run found

Two reds before handover, of 15 stages completed:

| Stage | Verdict |
|-------|---------|
| `verify-lint/run` | **0 issues**, all 18 flavors, no lock contention |
| `rfc/check` | exit 2 — the four RFC 9552 rows below |
| `staticcheck-feature-matrix/check` | exit 1 — **toolchain mismatch, not a code defect** |

The staticcheck red is `export data version 4 is greater than maximum supported version 2` on stdlib packages. `go.mod` pins `honnef.co/go/tools v0.8.1`, which cannot read Go 1.27 export data. It is an environment problem and it needs a dependency bump, which needs Thomas (`ze-go-style.md`: no new or bumped third-party import without his agreement). It is not caused by any change in this tree.

## The four RFC 9552 rows: the actual blocker

`rfc/check` is red on four MUST requirements with no tagged test and no annotation. All four turn on one fact: **Ze originates no BGP-LS today.** Both families register `Mode: "decode"` (`internal/component/bgp/plugins/nlri/ls/plugin.go`), and `NewBGPLSNode`, `NewBGPLSLink`, `NewBGPLSPrefixV4`, `NewBGPLSPrefixV6` (`types_nlri.go`) have no caller outside `_test.go`.

| Row | Binds | State at handover |
|-----|-------|-------------------|
| `RFC9552-5.2.1.1-1` MUST NOT | a BGP-LS Producer | Ze builds no key. `{not-applicable}` fits the code and is contradicted by the 2026-08-26 Producer ruling |
| `RFC9552-5.2.1.1-2` MUST NOT | same | same |
| `RFC9552-8.2.3-5` MUST | the implementation | **Thomas said "do what is right" — implement it.** See below |
| `RFC9552-8.2.6-2` MUST | **the operator** | Explained below. Awaiting his choice of route |

### `8.2.3-5`, the Instance-ID: authorised, and its shape is not yet settled

Thomas authorised implementing it. Do not implement it as a bare YANG leaf. §5.2.1.1 makes the Identifier field part of the node **key**, so it is load-bearing for the two uniqueness MUST NOTs too, and it only becomes real once Ze originates.

**A read of ExaBGP's new BGP-LS code was in flight at handover and its result is not in this document.** Thomas said Ze must originate BGP-LS and that code was added to ExaBGP for it. The agent was establishing: what ExaBGP originates, its exact wire contract, where Ze's decoder (`ParseBGPLS`, `parseNodeDescriptorTLVs` in `types.go`) and its unused encoders diverge, and whether Ze decodes ExaBGP's output unchanged. **Get that answer before choosing the config shape**, so the surface matches what must go on the wire. Re-run it if lost: the brief is in this session's transcript, and `ai/rules/rfc-compliance.md` puts the RFC above ExaBGP API compatibility, which is above the ExaBGP implementation.

### `8.2.6-2`, which binds the operator

§8.2.6 is two sentences and the extraction split them correctly:

> "An **operator** MUST define an import policy to limit inbound updates as follows: Drop all updates from peers that are only serving BGP-LS Consumers."
> "An **implementation** MUST have the means to limit inbound updates."

The first is `8.2.6-2`. Its subject is the operator, so Ze cannot comply; a deployment does. The second is Ze's own duty, extracted as `8.2.6-1`, which already carries a `{gap}`: the per-family prefix maximum is available for `bgp-ls`, but `countPrefixEntries` (`session_prefix.go`) is a CIDR walk that never parses a type-length Link-State NLRI, so the number it checks bears no relation to the BGP-LS NLRI count.

Ze already ships the means the operator's policy is written with, verified at the producers: `parseFamilyFilters` resolves a family via `family.LookupFamily` (`filter_family/config.go`), `bgp-ls` is registered (`internal/core/family/family.go`, `afiNameBGPLS`), and `handleFilterUpdate` applies `action: remove` on the import direction (`filter_family/handler.go`, `dirImport`).

Two routes, and Thomas has not chosen:
- **Annotate `{not-applicable}`.** Honest to the sentence. Still a classification that lowers what Ze owes, so it is his call.
- **Exceed it.** `plan/spec-bgp-ls-receiver-fault-management.md` Phase 5 proposes a declared `consumer-facing` peer role whose config-apply installs the import filter, so BGP-LS drops with no operator-written policy. Doing MORE needs no permission.

## Every fixit spec, reviewed and classified

Each verified at the producing function, not taken from the spec text.

| Spec | Verdict | State |
|------|---------|-------|
| `peer-pending-sync-settles-too-early` | **Defect, RFC 4724 MUST, UNFIXED** | Task re-cut. See below |
| `eap-tls-escape-hatch-kills-the-daemon` | Defect, fixed | `cmd/ze/main.go` now cites RFC 7627 §6.1 instead of asserting the attack from memory. `rfc/short/rfc7627.md` exists; enrolment is a separate owner question recorded on the `rfc7627` row of `rfc/not-enrolled.txt` |
| `vpp-slaac-no-dataplane-path` | Defect, fix already landed | `ze:backend "netlink"` on both leaves verified present; all 5 `.ci` exist. Header said 2/3, corrected to 3/3. **Only closure is owed** |
| `linger-rejection-reaches-no-verdict` | Defect, shipped at `715a54fad` | Header said `skeleton`; corrected to `in-progress`. AC-4/AC-5 remain: triage the 63 fixtures pairing `option=linger` with `reject=`. That is a suite run, not a code change |
| `lint-blind-to-every-other-build-tag` | Defect, regressed | Two named fixes applied: `testdataSegment` skip in `verifylint.go`, `ze_docvalid_fixture` added to the capability flavor. Blind files 16 -> 2, both documented RESIDUE |
| `functional-suite-pins-the-unshipped-backend` | **Improvement** | Copied to `plan/future/` with the disposition written out. **The source under `plan/` still exists** — its removal must ride the commit as `remove plan/spec-fixit-functional-suite-pins-the-unshipped-backend.md` |
| `dns-rfc1035-conformance` | Owner-blocked | Set to `blocked`. Thomas said "skip DNS specs" on 2026-08-28 |

### The one unfixed defect, and it is an RFC MUST

`plan/spec-fixit-peer-pending-sync-settles-too-early.md`. Read its re-cut Task, not the 2026-08-08 one below it.

Half is fixed: `(*Peer).setState` stores `sendingInitialRoutes = 1` in the same call that publishes Established.

The live half is the `apiSyncExpected` hold in `sendInitialRoutes` (`peer_initial_sync.go`), which the code itself documents under a `KNOWN DEFECT` heading. `RFC9552`-adjacent it is not; it is `RFC4724-4-1`: *"The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update."*

- **Overcount.** `resetAPISync` counts every binding carrying `send [ update ]`, but only `bgp-rib` emits `request peer <addr> plugin session ready` (`(*Reactor).SignalPeerAPIReady`). Any other permitted plugin leaves the count unreachable, so `waitForAPISync` runs the full 2s `apiSyncTimeout`. The marker goes out 2s after the initial update completed, and an event-driven announce raised inside that window drains **before** it — so the marker claims a route that was never part of the initial update.
- **Undercount.** `ze-bgp:peer-raw` is gated on attachment alone (`rawOrigin`), so a hand-built UPDATE from a bare `attach process X { }` binding sits outside the barrier.

**Do not widen the condition to "any process binding".** Measured 2026-08-08: with a 500ms hold, `test/plugin/role-otc-rs-withdraw-eor.ci` delivers the same relayed route twice. The shape the code names is to separate "initial sync running" (gates queueing) from "End-of-RIB not yet sent" (gates the marker). They are one flag today and they are two facts.

Two owner decisions are open on it, both recorded in the spec.

## What was implemented this session

- **`RFC9552-8.2.2-9`**: all seven §8.2.2 syntactic-validation bullets, in `validateBGPLSNLRISyntax` (`internal/component/bgp/message/rfc7606_bgpls_nlri.go`), reached from `validateMPNLRISyntax`, which previously returned nil for AFI 16388 so a malformed BGP-LS NLRI got no validation at all. Tagged positive and negative tests, a fuzz target (586,741 executions clean), and a measured discrimination revert. Also closed four `{gap}` annotations that had become false, moving `docs/features/rfc-status.md` from "Eight MUST gaps" to "Four".
- **`TestRFCCheckRealTreeReportsFiveRFC9552Violations` deleted and replaced.** It asserted against the real checkout that `Check` returned exactly five violations, matched positionally. Its green bar depended on Ze staying non-conformant. Replaced by three fixture-tree tests asserting the report's *shape*, plus one real-tree test encoding no count.
- **Three write-hook guards repaired**, each with tests: the governed-write guard never caught a bare `sed -i` nor any clustered flag; the nolint guard refused *every* `//nolint` including the compliant form the style guide requires; `validateSpec`'s Current Behavior extension list was `go|sh|rs|ts|js|mk`, from a repository Ze is not.
- **229 specs migrated** to the current pre-commit gate name; **84 specs** given the design-doc rows the validator derives. 102 failing specs -> 16, and those 16 fail honestly (unwritten Data Flow entry points).
- **549 broken doc references -> 0**, twice (the rename reintroduced two).
- **goconst 1403 -> 0**, ~814 distinct literals, 66 packages, six agents plus a peer session. **11 pre-existing `//nolint:goconst` were removed**, six of them file-level.

## Process root causes recorded

One rule point, then under `repo-maintenance/gate-population/`, stating that a change entering a gate's population owes that gate a green. The 2026-08-30 collapse of the rule corpus removed it as an instance of `ai/rules/principles.md`. Journal rows in `guard-blocks-its-own-authors-repair`, `gate-fires-outside-its-population`, `gate-excludes-part-of-its-population`, `refactor-removes-feature`, `bulk-rename-corruption`.

The general practices, no case detail:
- A change that moves files into a tree a gate reads, or widens what a gate requires, must leave that gate green over the whole affected population **in the same change**.
- A guard that matches text cannot judge the file that declares that text.
- A rule whose message describes a narrower population than its pattern matches refuses the behaviour it exists to require.
- A linter judges the code as written, never what the code must remain **able** to do. A finding proposing removal of a parameter, branch or return from test infrastructure is a coverage question first.
- A commit's file set must be closed under "declares what the included files reference", and must be judged by compiling the **commit**, not the working tree.
- A rename's blast radius is every file that **names** the symbol, not every file that compiles against it.
- When a lookup or refusal exists in two places, the second is found by asking "who else answers this question", never by grepping the first one's call sites.

## Seven `le` improvements handed to `le-reorg`

Ranked. **Number 1 is the unattributable-red problem above** and is worth more than the other six together. Then: the gate cannot distinguish "did not run" from "found nothing" (global golangci-lint flock + the post-edit hook); every count it prints is a floor across **three** truncation layers (`max-issues-per-linter: 50`, `max-same-issues: 10`, `uniq-by-line` default true) and it never says so; an interrupted verify leaves a record that reads as real stage verdicts; a flavor can lint a package without linting the file the flavor exists for (`internal/component/support` does not compile for FreeBSD and no flavor sees it); `verifydispatch.lookupTool` re-implements `leroot.Dispatch`.

## Traps that cost this session hours

- **Never edit while a gate runs.** The post-edit hook lints each touched file and takes the same global flock; an in-place `./le verify current` is void the moment the tree moves under it.
- **Never trust a count from a capped run.** Three layers. Always `--max-same-issues=0 --max-issues-per-linter=0 --uniq-by-line=false`, and work to a fixed point per package, never to a list.
- **Always check the exit code**, not just the output. This session reported "goconst is zero" from a run that had exited 3 while queued.
- **`unused` and `SA4023` in a package with `_linux.go` or `_other.go` siblings are darwin false positives.** Deleting the symbol breaks the Linux build. Check `GOOS=linux go vet ./<pkg>/`.
- **Two AST-parity tests** recognise only `*ast.BasicLit` case values, so hoisting case literals to constants makes them see zero cases and pass vacuously. `internal/component/bgp/cli` and `internal/component/iface/cli` were repaired to resolve identifiers and error on an unresolvable name; `internal/component/config/schema/cli` and `.../yang/cli` still have the old shape.
- **`embedlit` elision does not compile** for a struct field. The fix is Go 1.27 promoted-field initialisation.
- **A `//nolint` inside a `//` comment does not apply.** Reword the comment.
- **Bare `go test ./internal/component/bgp/...` fails ~124 tests at HEAD** because the plugins compile out. Use the `.golangci.yml` tag list.

## Known reds that are not this session's

- `TestDefaultOriginateAppendsLinkLocalWhenSection3Holds` and `TestSendAnnounceAppendsLinkLocalWhenSection3Holds` fail **at HEAD**, verified in a clean detached worktree. The MP_REACH next hop is the 16-octet global address where the test asserts the 32-octet RFC 2545 form. Row in `plan/journal/unwired-feature.md`.
- `TestRFC7606Section54PropagatesUnknownBGPLSType` is red and **needs Thomas**. Its fixture `lsWireNLRI(1, 0x02, 0x00)` is a Node NLRI with a 2-octet body, which RFC 9552 Figure 7 makes impossible, so the new validator correctly discards it. The fix is fixture bytes only, but the file carries an `RFC requirement:` tag so `testweakened.Proposed` requires an **owner** approval row in `test/rfc-changed.md`. The exact row is drafted in `plan/journal/fixture-encodes-an-impossible-state.md`.
- `internal/component/support` does not compile for FreeBSD. Pre-existing, invisible to the gate.
- `removePeerMetrics` (`reactor_peers.go`) open-codes `"route_refresh"` while the counters stamp `type="refresh"`, so a removed peer leaks its two ROUTE-REFRESH counters forever. Pre-existing.

## Do this next

1. **Wait for the verify to finish** and read `tmp/ze-verify-failures.json`. Do not edit while it runs.
2. **Get Thomas's ruling on `8.2.6-2`** and the two `5.2.1.1` rows. They are the critical path for the whole checkout, not just for this chunk.
3. **Read the ExaBGP compatibility answer** before designing the Instance-ID surface.
4. **Land the commits.** `le-reorg` is blocked on thirteen files that are this session's work; it named them and they are all in `internal/le/`. Use the NEW paths. Include `remove plan/spec-fixit-functional-suite-pins-the-unshipped-backend.md`.
5. **Then close the specs that are complete** — `vpp-slaac` and `lint-blind` both owe only closure — which is what finally reduces the open count.

Commit granularity: these are separate chunks and must not become one commit. The guard fixes, the lint burn-down, the spec migrations, the RFC 9552 implementation, the doc-reference repairs, and the rule point are six focuses.
