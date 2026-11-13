# Handover: config-enum-refusal-proposes-valid-values

## Completed work and closure navigation — 2026-11-13

The enum-refusal parent is closed. Commit A is
`13310151fbd2893eee161f623d60aa2446d69886` and commit B is
`9b4f33044a799197dad5d39f81baaafc57cf5930`. Its implementation, required
runtime proofs and independent CLEAN round-6 review are retained in history.
The source-check result covers five checked build flavors at b9c22dfb7d:
distro, appliance, setup, host and installer. Host is development tooling.

The dependent `spec-bmp-route-action-redistribute-acts-as-monitor` has its
implementation and proofs in fa74dc4c00 and fb0f9181ec. The parent closure
satisfies its A-2b dependency. This record belongs to the BMP closing
transaction: commit A retains the final audit, operator news and journal
evidence, and commit B removes that spec. The generated script and resulting
SHAs are reported by main after both commits land. No push is authorized.

BMP monitoring remains isolated from best-path and the FIB. Its unit recorder
proves the exact SDK injection call. The preserved ingest/isolation tests
prove stored routes, and the existing FRR receiver scenario proves the
surviving monitor configuration interoperates. No new redistribution feature
or full RFC-conformance claim is made.

The sections below are historical navigation only. Their open tasks, old
claims, drafts, pending proofs and review-cap blockers are superseded by the
committed final evidence and this closing transaction. Separately owned
defects remain with the specs named in the parent's final Work Not Done.

## Historical continuation — 2026-11-13

This section supersedes the historical handovers below. The implementation and
required runtime proofs are committed at `b9c22dfb7d`; earlier native Spec-work
commits are `0f8c7ccf04`, `024ba1d203` and `1ce3fb78c2`. The parent is now
`verification` with `Handoff: verify`, and main holds the resumed claim.

On 2026-11-13 the owner answered the request for reviews beyond five through
clean closure: "Autorized and continued." The fresh `closure_round6` context
recorded that authorization and its scope before source review. Round 6 is
source CLEAN, with 0 BLOCKER and 0 ISSUE across the secret-suffix repair and
sibling framing, CR codec/readers and actual entry-point tests, browser cleanup
budget policy, security/test preservation, Go style and the spec audit.
Historical round 5 remains not clean. No new source or test changes were made.

The fresh repo/weakened/doc checks are dispositioned below. The covering
native Review Gate record/check passed at round 6 over 152 historical paths,
including 110 code files with matching hashes. Main still owes parent commits
A and B, then the dependent
`spec-bmp-route-action-redistribute-acts-as-monitor` closure. No completed
closure is claimed before those commits land.
No push is authorized.

### Completed evidence

- All implementation phases and closure-found source repairs are present:
  candidate repair and safe publication, full key/path framing, literal and
  placeholder quoting, operational scan/pipeline replay, secret masking,
  required identities, web member deletion, all 26 incidental refresh sites,
  atomic daemon PID publication and browser cleanup budget preservation.
- Offline, editor, REST/gRPC/gNMI, reload and the incidental IPsec load proofs
  completed their required 80-invocation selections; their promoted live
  projections and scoped controls are recorded in the parent spec.
- The final three browser workflows completed a fresh campaign of exactly
  80 unique successful numbered invocations, with 240 successful browser
  closures. The command used `iterations 80 minutes 60 parallel 2 burners 2
  timeout 600 any-failure`; its raw result 1 means `notReproduced`. Earlier
  failed, canceled or incomplete campaigns never count toward these 80.
  `enum-isolated-browser-proof.json` records all three source hashes.
- The browser cost report passed. The three drafts were promoted by plain
  move to `test/web/`, and all three owned ordinary live cases passed in
  `job-enum-browser-promoted-live-863b9506.log`. Its incidental recorded-config-save
  case also passed; the unrelated read-only download skip is not an owned skip.
- Browser children used private agent-browser 0.43.0 and Chrome 157.0.8103.78,
  with the browser CLI and descendant temporary files isolated under owned
  tmpfs paths.
  Ze data and socket placement were unchanged. The completed proof establishes
  the tested behavior; no causal explanation of earlier host stalls is claimed.
- Eight native overlay walks discriminated their intended regressions.
  Actual pre-fix RED then GREEN also covers the required-identity, operational
  literal, non-first-key, diagnostic-path, sensitive-suffix and cleanup-policy
  regressions. Scoped race and count evidence is recorded with its zero spans
  and known health exceptions; no all-tree green or full RFC support is claimed.
- Nineteen owned RFC 7950 producer observations were recorded, both owned
  audit verdicts were rejudged, and the unchanged RFC 8907 accounting audit was
  resealed after an adjacent untagged test moved. Final owned RFC 7950/8907
  freshness passed; the global command still reports 99 other-stem violations.
- Final doc-index `ae12aa2a` passed without an owned tracked map diff.
  Fresh `job-enum-round6-doc-check-f119ecea.log` reports seven unrelated anchors:
  sysrib `ApplyConfiguration` and `recomputeBest`, plus five old report
  `max*Len` symbols. No owned guide/source anchor failed. The current help
  census is 1033 enum values and 1033 summaries; no blanket global green claim.
- Empty owned browser TMPDIR roots were removed by successful atomic `rmdir`
  only (`691e7368`). No payload or process was removed; runtime evidence and
  extracted tools remain available for the independent review.
- Eleven native test-change approvals are recorded. Manual browser, markup
  and golden judgments still require their exact `Test-judge:` trailers in the
  commit body. Do not regenerate unrelated goldens or weaken tests.

### Authoritative continuation records

The parent spec now contains all 52 active AC evidence rows, task/test/file
audit, assumptions, goal validation, documentation/wiring evidence and review
history. Round 6 source review is CLEAN and the covering native Review Gate
record/check passed. Parent closure commits remain pending. Full YANG/leaf-list enforcement and the other separately
owned defects in Work Not Done are not claimed fixed. In particular, the open
leaf-list range/length occurrence belongs to the web/config one-off spec AC-8;
this parent owns the distinct resolved web key-leaf occurrence.

- Current state: `tmp/session/2026-11-12-202721/state/session-state-config-enum-refusal-proposes-valid-values-config-enum-refusal-20261112-codex-continue.md`.
- Scratch: `tmp/session/2026-11-12-202721/scratch/`.
- Frozen scope builder: `enum_review_artifact.py`; outputs
  `enum-manifest.json`, `enum-review.diff` and `enum-review-head.txt`. Its latest
  population has 141 owned paths. After a handback commit, derive committed
  review scope from native `Spec-work:` history; an empty working-tree diff is
  not the spec's change set.
- Shared `docs/functional-tests.md` and
  `plan/journal/guard-added-to-one-half-of-a-pair.md` contain foreign WIP only.
  Their owned selected hunks already landed in `b9c22dfb7d`; review that
  committed blob and preserve the foreign working-tree hunks. Do not reapply
  the old hunk patches or rebuild the old uncommitted manifest.
- Review also includes already committed
  `internal/component/config/cli/cmd_set_refusal_test.go`, listed in
  `enum-review-extra-files.txt`. The four committed blocks above are the exact native `Spec-work:` population.
  Foreign changes, including the untracked address-family journal, stay out.

Source implementation, test authorship, prior review findings, exact test
judgments and measured runtime evidence are already recorded. Do not repeat
those investigations or rerun unchanged passing tests. A reviewer still reads
the producing source of the changes it must independently judge; the digest is
navigation, not a verdict.

### Remaining closure actions, in order

1. Complete parent closure through one generated script: commit A retains the
   final spec/evidence and commit B removes the spec. Clear current citers,
   retain the news/owned journal records and run the exact printed script.
   Main owns claim release and any newly owed native post-commit check.
2. Close the dependent BMP spec through its own ze-close agent and A/B script.
   No unchanged passing test or completed implementation investigation is owed.

### Fresh mechanical checks, dispositioned

- `job-enum-round6-repo-check-89d513b0.log`: 31 global findings, all outside
  this change: missing IKE patch anchors, foreign unused exports and deployment
  ASN parsing. No owned source finding.
- `job-enum-round6-weakened-check-36862edf.log`: 13 unexplained IKE/sysrib
  weakenings, all outside this change. The owned changes are not flagged.
- `job-enum-round6-doc-check-f119ecea.log`: seven unrelated sysrib/report
  anchors; no owned guide/source error. Global checks are not claimed green.
- Final RFC freshness is
  `job-enum-final-closure-rfc-freshness-4fea72ad.log`; the older
  `job-enum-rfc-check-4fea72ad.log` predates the final six observations.

### Covering native Review Gate

Artifact: `tmp/review/config-enum-refusal-proposes-valid-values-config-enum-refusal-20261112-codex-continue.md`.
`job-enum-parent-covering-review-check-valid-f1fca422.log` reports
`review_gate: OK (110 code files, clean, hashes match)`. It covers 152
historical paths, records rounds 6, names the early `Model.cmdSet`
sensitive-suffix product defect and retains the exact owner authorization.
Main observed zero full parent-path citations outside the parent across the
tracked tree. The news text and owned journal work already landed in the native Spec-work
history. The closure news gate found that the fragment used the prefixed spec
filename instead of the canonical stem. Commit A adds
`website/changes/fragments/config-enum-refusal-proposes-valid-values.md` with
`spec: config-enum-refusal-proposes-valid-values`, removes the old prefixed
fragment through the generated script, and corrects the fragments README. The
operator body is unchanged. The final parent, handover and one-line editor-guide
correction also land in commit A. This record NOTE adds no source review round
or Go check.

## Historical blocked handback — 2026-11-13

The following recorded the state before the owner's continuation authorization.
Its authorization blocker is cleared by the current continuation above.

This section supersedes the historical 2026-11-12 handover below. The parent
remains the release/fixes work item `spec-config-enum-refusal-proposes-valid-values`.
Its implementation and required runtime proofs are complete. The parent is
`blocked` because round 5 found a sensitive-suffix leak, that repair passed its
regression tests, and the independent post-fix source review requires explicit
owner authorization past the five-round cap. The outstanding question is about
that review, not permission to implement or commit. No sixth source pass or
CLEAN artifact has been recorded, and elapsed time is not authorization.

Final native metadata checks are recorded. Main is preparing ze-close's blocked
handback: preserve all owned work in a generated commit, without spec removal,
then run native `repo compiles check` and release the claim. Read the native
`Spec-work:` history rather than assuming that handback has already landed;
its SHA belongs in main's final report. No push was
authorized. The dependent `spec-bmp-route-action-redistribute-acts-as-monitor`
remains open and closes after this parent.

## Historical handover — 2026-11-12

Written 2026-11-12 by the session that reworked the design and implemented
most of it. Start here, then the spec, then the per-spec state file.

- Spec: `spec-config-enum-refusal-proposes-valid-values`
  (status `in-progress`, owner-approved 2026-11-12 after review round 25).
- State file with every agent's handoff, in order:
  `tmp/session/2026-11-11-160129/state/session-state-config-enum-refusal-proposes-valid-values-f5ef1144-6c05-40fc-8506-68a94ded2a6f.md`
  (Phase 2 parts 1 to 3, Phase 3 parts 1 to 3, Phase 4 parts 1 and 2, Phase 5).
- Why it matters beyond itself: `spec-bmp-route-action-redistribute-acts-as-monitor.md`
  (Phase 4/4, proven) can only close after this spec closes.

## Claims

The session released every claim it held on 2026-11-12, so the next session
claims what it works on:

| Spec | State left |
|------|------------|
| `spec-config-enum-refusal-proposes-valid-values.md` | `in-progress`, unclaimed. Phases 1, 2, 5 and 6 are complete; phases 3 and 4 are committed except the items under "Left to do". The `Phase` field reads 3/6, the count of phases with nothing owed being four |
| `spec-bmp-route-action-redistribute-acts-as-monitor.md` | `in-progress`, Phase 4/4, proven, unclaimed. Closes after the spec above |
| `spec-fib-depth.md` | `in-progress`, unclaimed. Phase 13 and its follow-up fixes are committed and recorded in the spec; one open defect it owns: fib-vpp programs no IP route without a persistent store |

Claiming the first two again needs the owner's cap exception to be restated or
the WIP count to be under the cap: the exception recorded in each spec was
for this session's work.

## What the spec delivers

A value Ze does not accept is refused the same way on every surface: the
setting's full path, the refused value, the accepted values with their YANG
help (or the bounds, or the format in words, never a regular expression), a
closest match, and a last line `fix: <exact line or command>`.

Owner decisions that bind the rest of the work:

| Decision | Rule |
|----------|------|
| Q-0 | One friendly refusal for every YANG enum, on every path |
| Q-7 (a) | A refused commit shows the header on the status row and one issue per line in the viewport |
| Q-9 | `top` alone moves to the root; `top <command>` is a transient move to the root for whatever command follows. A fix line inside a context reads `top set <full path> ...` |

## Committed

| Commit | Piece |
|--------|-------|
| b301ac39b8 | Phase 1: the shared refusal on the config tree walk |
| 95445948c5, ba5510001c | `top <command>` in the SSH editor, with its functional test |
| 833c08af3f | Phase 2: config file and set parsers, list keys, `suggest.Correction` |
| 463fe18b32 | Phase 5: operational command arguments |
| 83df428503 | Phases 3 and 4: SSH editor and web editors, gNMI `InvalidArgument` |

Each has passing unit tests, a red-walk, functional tests proven over 80 runs
under load, and an independent judge's approval of every changed test.

## Left to do, in order

1. **Load proofs and one draft.** Every functional test written so far is
   committed under `test/`. Four passed and are promoted without their 80-run
   load proof, which is owed: `test/web/config-enum-refusal.wb`,
   `test/web/config-range-length-pattern-refusal.wb`,
   `test/editor/validation/enum-refusal-set.et`,
   `test/editor/pipe/set-cli-format.et`; so is `test/ipsec/ipsec-dataplane-show.ci`.
   One draft is NOT a test yet: the REST test passed alone and timed out under
   load, because its listener port 18088 is fixed and parallel copies collide.
   It is saved as `plan/handover/config-enum-refusal-rest-draft.ci.txt`: restore
   it to `test/draft/plugin/api-config-enum-refusal.ci`, give it a leased port,
   prove it, promote it. The editor suite takes the file path as the test
   name; the others take `--pattern <name>`. The docs pass is committed
   (24c082936a).
2. **Phase 3 remainder** (`internal/component/cli`, `internal/component/config/cli`):
   offline `ze config set` (AC-6, AC-28); `TestKeyCheckOnInsertNestedActivation`
   with its test YANG module (AC-29 to AC-31); session-mode key refusals
   (`Editor.writeThroughCreate`); the `CheckRequired` typed fix;
   `SectionValidationError` in the editor; `enum-refusal-commit.et` with its
   `top` case, `enum-refusal-load.et`, `range-length-pattern-set.et`,
   `test/draft/parse/enum-refusal-config-set.ci`. Unsettled: Phase 3 uses
   `ValidateLeafValue`'s union arm where the spec names `Validator.ValidateEntry`;
   compare verdicts for a union leaf declared by more than one module.
3. **Phase 4 remainder:** `grpc-config-enum-refusal.ci` and
   `gnmi-set-enum-refusal.ci`. No gRPC or gNMI client fixture exists; each
   needs one that dials over TLS with a token. The web activation framing has
   no test (needs a leaf-list under an enum-keyed list).
4. **Phase 6:** done (24c082936a), except what is not built yet: offline
   `ze config set` refusals are not documented. Run `./le doc index update`
   and `./le doc check all` before closure.
5. **RFC records.** Thirteen records in `rfc/discrimination/rfc7950.json` name
   producers Phases 1 and 2 changed and owe a re-record with the same revert
   break: `refuseInvalidCustomSections` (8.3.3-1, 8.1-2, 8.3-1),
   `validateDecimal64` (9.1-1, 9.3.1-1), `validateEnumeration` (9.6-1),
   `yangToLeaf` (7.3.4-1). The unit list is in the state file, "Part C".
6. **Spec bookkeeping** (uncommitted edits are in the working tree): the
   Functional Tests status rows, the pin inventory additions, Goal Validation
   evidence, AC-54's list of incidental views (22, without `commit verify`
   with issues and a refused commit), AC-27's `nssa-propagate true`, the
   `Phase` field.
7. **Closure:** `/ze-review`, `/ze-close`, then close the BMP spec.

## Traps met

- Another session's IKE work leaves the tree unbuildable for long stretches;
  every functional test and stress run needs a full build. Probe with
  `go vet ./internal/component/ike/...` in a bounded loop before a run.
- Run functional tests with `./le --name session ...` once test fixtures
  changed, or the launcher refuses as stale.
- The `.ci` harness takes one `expect=command-error` per command; in `.et`
  files a trailing space is `input=space`.
- `TestWebHandlerGoldenOutput` differs on `post-cli-mode`, `get-show-finder`
  and `get-l2tp-session-json`. They differed before Phase 4; the cause is not
  traced to this spec. Do not regenerate the golden files without a judge.
- The spec edit hook judges the whole file: a single Edit of a stale spec is
  refused for unrelated gaps. Journaled, uncommitted, in
  `plan/journal/guard-blocks-its-own-authors-repair.md` (2026-11-12): the row
  needs an owning spec, and the candidate
  (`plan/post-release/engineering/spec-gates-share-one-semantic-population.md`)
  has no `## Existing Mechanisms` section, so its own edit is refused.
