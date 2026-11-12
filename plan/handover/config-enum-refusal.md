# Handover: config-enum-refusal-proposes-valid-values

Written 2026-11-12 by the session that reworked the design and implemented
most of it. Start here, then the spec, then the per-spec state file.

- Spec: `plan/release/fixes/spec-config-enum-refusal-proposes-valid-values.md`
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
