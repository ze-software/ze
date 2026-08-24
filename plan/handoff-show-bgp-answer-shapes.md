# Handoff: closing the two answer-shape specs

Written 2026-08-24 by session `d8974112` (`feature`). Both specs are
implementation-complete and committed. Neither is closed, and neither can be
closed by the session that wrote them: `ai/rules/planning.md` requires the
Review Gate to run in a context that did not write the code, and
`review_gate.py record` and `.claude/hooks/pretool-agent-skill.py` enforce it at
both ends.

Everything below was measured against the tree, not read off an audit. Where I
did not verify something, the row says so.

## What closing needs

| Spec | Status | Deferral shard |
|------|--------|----------------|
| `plan/spec-cli-show-bgp-answer-shapes.md` | in-progress, 5 of 5 phases done | `plan/deferrals/cli-show-bgp-answer-shapes.md` |
| `plan/spec-plugin-declares-answer-shape.md` | in-progress, 5 of 5 phases done | `plan/deferrals/plugin-declares-answer-shape.md` |

Run `/ze-close` on each. It appends `plan/TEMPLATE-CLOSURE.md`, runs the Review
Gate over the committed diff, records the artifact with `review_gate.py`, and
produces the two closure commits.

## The commits to review

Ten, in order. `make ze-repository-tracked-build-check` was run and passed after
every one that carried Go.

| SHA | Subject |
|-----|---------|
| `660c537a1` | fix(bgp): answer peers in one order, not the map's |
| `b54a56de0` | feat(cli): declare what every in-tree show bgp answer holds |
| `abeec7f1a` | feat(plugin): let a plugin declare what its answer holds |
| `67a2200ce` | fix(rpki): answer the caches in one order, not the map's |
| `78f2b184b` | feat(rpki): one answer shape whatever the argument, and declare it |
| `74f0b592c` | plan: record the answer-shape phases as implemented |
| `e73d58e8a` | plan: journal what the show bgp answer-shape work found |
| `f7735fd50` | fix(cli): say what fill needs, not what every row operator needs |
| `6141cb9f6` | feat(plugin): declare the last five plugin-served show bgp answers |
| `77e42aeab` | docs(plugin): document the answer-shape declaration channel |

Those SHAs are post-rebase. A rebase onto `origin/main` ran mid-session, so the
SHAs quoted inside some commit BODIES point at pre-rebase commits. That is
cosmetic and was left alone deliberately.

## Two files the closure commits MUST carry

Both are dirty in the working tree and neither can be committed before closure.

    plan/journal/output-not-byte-stable.md
    plan/journal/documentation-shows-config-the-parser-refuses.md

Each holds a row whose Spec cell names one of these two specs, and
`spec_audit_problems` (`scripts/dev/commit_helper.py`) reads a spec-named row as
that spec's closure artifact. It then refuses because the spec has no filled
`## Pre-Commit Verification` section. At closure that section IS filled, so the
refusal lifts and the rows land where they belong.

**Do not route around it.** Two escapes exist and both were refused this session
for a reason that still holds. `--review-override` does not reach this gate; it
clears `review_gate_problems`, and this one fires after. Releasing the spec
claim silences it, because the gate returns early with nothing claimed and
`_release_session` only deletes a marker under `tmp/`, but that is gaming a gate
rather than satisfying it. The seventh row of
`plan/journal/gate-fires-outside-its-population.md` records the whole thing,
including the shape of the real fix.

## Two acceptance criteria are struck and need Thomas

Neither is a defect in the implementation. Both are payload-design questions the
specs deliberately refused to answer, and each has a deferral row.

| AC | Why it is struck |
|----|------------------|
| `spec-cli-show-bgp-answer-shapes` AC-14 | `bestResult` (`internal/component/bgp/plugins/rib/rib_pipeline_best.go`) carries the next hop inside `attributes`, and `selectRecord` cuts a record naming one displayed field to the displayed ones, so `show bgp rib best` cannot answer `display prefix next-hop` while `show bgp rib` can. The AC was re-pointed at two keys of the same row. Flattening the next hop changes a payload "Behavior to preserve" protects |
| `spec-plugin-declares-answer-shape` AC-16 | `AdjRIBInManager.show` keys `adj-rib-in` to ARRAYS, and `rowSet` reads a map as rows only when every value is an object, so the envelope becomes one row holding every peer. The command declares `doc`. Making the AC true means the peer map must hold objects, which three test consumers navigate as it stands |

A closure that marks either AC met without a payload change is marking something
false. Leave both struck and let Thomas rule.

## What the review should look hardest at

Not a list of everything: the places where a green test would not have caught a
mistake.

- **`declarationRegistry.declare` panics; `declareFor` returns an error.** The
  panic is right for an in-tree table written in `init()` and wrong for a string
  that arrived on a socket. `RegisterPluginShapes` must reach only `declareFor`.
  This was proved by mutation, and `TestOnRegistrationRefusesConflictingShapeDeclaration`
  goes red when the conflict branch is made to panic. Re-check the call graph
  rather than the test.
- **Every declared column name must be a key its producer actually writes.** A
  declared name the payload never carries orders nothing and publishes a field
  that does not exist, and NOTHING fails. This is the one risk in the whole two
  specs with no signal. `TestDeclaredColumnsExistInPayload` covers part of it.
- **`test/ui/show-bgp-declared-shapes.ci` and `show-bgp-plugin-shapes.ci` pin
  the string `cannot apply here`**, which only `validateDeclaredShape` writes. A
  post-dispatch refusal shares other substrings with it, so a weaker assertion
  passes with every declaration removed. Two agents hit that vacuity this
  session; both `.ci` files were then proved to go red with the declarations
  stripped.
- **Five map-drain fixes.** `reactorAPIAdapter.Peers`, `SoftClearPeer`, the
  three rpki caches, `peerStatus` and the healthcheck summary branch. Each
  test requires the full row sequence over 32 consecutive calls, so a
  map-ranging implementation fails rather than flakes. The ROA and ASPA ones
  were random MEMBERSHIP rather than just order, because a limit stopped
  mid-range.

## Verification debt

Session `5ac14170` holds 20 open rows in `plan/verification-debt/5ac14170.md`.
`make ze-verify-debt-clear` was started at the end of this session; read that
shard for the outcome rather than assuming it cleared. `--push` refuses while
any row is open, and a successful push is not evidence: the 2026-08-24 push went
through by hand, outside `commit_helper.py`, so it bypassed the debt gate.

## Findings left open, all recorded

None blocks closure. Each has a journal row or a deferral row with a
destination.

| Finding | Where |
|---------|-------|
| `resolve` and `origin` decorate every address-shaped value, so the declared address-field list gates admission and not action | `plan/journal/declared-format-contradicts-payload.md` |
| Five `show bgp` commands take a free-form value in an untyped positional slot | `plan/journal/command-takes-an-untyped-positional-value.md` |
| A plugin can declare a shape on a path a builtin serves, where that builtin declares nothing | `plan/deferrals/plugin-declares-answer-shape.md` |
| `resolve` and `origin` over an identity-keyed row set | `plan/deferrals/cli-show-bgp-answer-shapes.md` |
| One name for the peer address across the `show bgp` tree | `plan/deferrals/cli-show-bgp-answer-shapes.md` |
| `show bgp decode` and `encode` answer text, so no operator chain reaches them | `plan/deferrals/cli-show-bgp-answer-shapes.md` |
| A documented `register command "<name>" ...` text verb with no parser | `plan/journal/documentation-shows-config-the-parser-refuses.md` |
| The published catalog cannot see a plugin's declaration | `plan/deferrals/plugin-declares-answer-shape.md` |
