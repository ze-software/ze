# 1172 -- rfc-requirement-coverage

## Context

Every RFC 2119 MUST-level obligation in `rfc/short/*.md` had to become traceable to the tests
that enforce it (or to a justified disposition), two-way and machine-checked so the link cannot
silently rot, plus a skill that re-audits whether each test still enforces its requirement's
letter and spirit. Before: 173 summaries carried 3,257 checklist lines (2,111 MUST-level) that
referenced ZERO tests, and nothing stopped a test enforcing an RFC MUST from being deleted or
weakened. Built the gate + tag + polarity rule + dispositions + enrolment ratchet + fingerprinted
audit + a test-protection hook, and piloted end-to-end on RFC 7606.

## Decisions

- Author only the TEST-side tag (`// RFC requirement: <id> <polarity>`); DERIVE the
  requirement->test direction into `ai/RFC-REQUIREMENTS.md`. Chose this over hand-writing the
  back-link into the summary because a hand-written back-link outlives the test it names -- the
  exact silent rot this exists to stop (`ai/rules/derive-not-hardcode.md`). The tag dies with the test.
- Every gated MUST needs BOTH polarities. A single-polarity test proves the code CAN produce an
  outcome, never that it does so for the right reason (a negative-only test passes if the code
  rejects everything). Escape `{single-polarity: <p>; why}` is itself gated: reason mandatory,
  stale the moment the other polarity appears.
- Section-anchored IDs (`RFC7606-5.3-6`) over a per-RFC counter: RFCs are immutable, so section
  numbers are the most stable anchor; reclassifying a keyword must not renumber and break every
  tag. Keyword level is a field, not identity.
- Fingerprint = requirement text + whole tagged-FILE sha, coarse on purpose. Over-triggering
  re-audits (safe); under-triggering ships a verdict for a test that has since changed (unsafe).
  `check_audit_freshness` turns "someone should re-read this" into a signal that fires exactly
  when it can have gone wrong.
- Two different machines for two different failure modes: the gate is mechanical and TOTAL (every
  MUST accounted every run); the audit skill is semantic and SAMPLED (a model judges letter and
  spirit). The fingerprint is the hinge between them.
- RFC-tagged tests get a strict hook path: any behavior-bearing edit blocks (exit 2); `//
  test-relax:` is NOT accepted (self-service justification is not user approval); escape is `//
  rfc-test-change-approved: <date> <what>`. Chose to BLOCK rename-only edits too -- a detector
  cannot tell a rename from a rewrite without a Go parser, so it falls on the safe side (D-1).
- Protection is defense-in-depth, stated honestly: a FORGED approval token defeats BOTH the hook
  and the branch audit (they share one detector); only `grep -rn 'rfc-test-change-approved:'` +
  human review catches it. The audit must not be overclaimed to catch a self-written token.
- Enrolment ratchet (`rfc/enrolled.txt`, grows only) makes partial adoption first-class and
  honest. Only RFC 7606 is enrolled; the remaining 172 summaries are explicit tracked backlog
  owned by `plan/spec-followup-rfc-enrollment.md`.

## Consequences

- The gate runs in `ze-verify` (BOTH branches of `stagesForMode()`) and `ze-doc-test` (ledger
  staleness). A gate added to `_ze-verify-impl` would never run -- it has zero callers.
- Building the gate FORCED real RFC 7606 compliance work (all user-approved): §2 treat-as-withdraw
  was never implemented (added `message.SynthesizeWithdraw`); §3(b) structural length -> session
  reset; §5.3 prefix-over-max -> session reset; and inner MP_REACH/UNREACH NLRI overrun + RFC 4760
  flag-consistency validation (§5.3-4/§5.3-5), IPv4/IPv6 unicast, add-path aware.
- RFC-tagged tests can now only change with recorded user approval -- this teaches every agent to
  fix the CODE, not the test, when a code-behavior bug surfaces.
- MP NLRI-syntax validation stays in the message-level attribute validator
  (`validateMPNLRIField`), with the per-family add-path state the message layer cannot see
  threaded in from the reactor as a callback (`ValidateUpdateRFC7606AddPath`'s `addPathFor`).

## Gotchas

- **A committed pilot at 52/52 "green" was NOT honest.** `RFC7606-5.3-4`/`-5.3-5` were tagged
  both-polarity but UNENFORCED -- the tests passed via an unrelated rule (§7.11 next-hop length 0),
  and one test file even documented §5.3-4 as an uncovered gap while another tagged it covered. The
  gate is green on a lie unless coverage is genuinely verified. Only independent adversarial
  verification (NOT trusting the Implementation Summary) found it.
- **The gate's own guards shipped unwired.** `check_id_allocation` (AC-2 ID-reuse ratchet) had zero
  production callers; ledger-staleness (AC-20) was never wired into `ze-doc-test`;
  `check_status_agreement` (AC-10) failed open on an empty Remaining column. A gate built to catch
  vacuous greens carried vacuous guards. Verify wiring from the entry point (`run_check`), never
  the helper alone.
- **An RFC NLRI-syntax check written at the message level is ADD-PATH-blind.** `ValidateNLRISyntax`
  walks `[len][prefix]`; with RFC 7911 add-path negotiated the on-wire NLRI is
  `[path-id:4][len][prefix]`, so a path-id byte (>32, or one that overruns) is misread as a prefix
  length -> spurious SESSION RESET on conforming UPDATEs. Add-path is per-session, so the check
  needs the negotiated per-family state: the reactor passes an `addPathFor` callback into
  `ValidateUpdateRFC7606AddPath`, and `ValidateNLRISyntaxAddPath` skips the path-id. This
  latently affected the IPv4 withdrawn/NLRI sites too.
- **Two sessions implemented this same spec in parallel** (this one and the session that
  committed `fa224032d` first). Both found the identical dishonest-green and add-path-blind
  bugs independently, but architected the MP check differently (reactor-side vs message-side
  with threaded state). The rebase reconciliation kept the published message-side design and
  ported this session's unique work (gate-ratchet wiring, add-path end-to-end tests,
  enrollment follow-up spec). Check for a pushed sibling commit before deep-implementing a
  claimed spec in a shared tree.
- **Whole-file `test_sha`** means editing ANY tagged test re-stales EVERY requirement tagged in
  that file -- regenerate `rfc/audit/<rfc>.json` after touching tagged tests (recompute fingerprints
  with the script's own `requirement_sha`/`tagged_unit_shas` so they match what the gate recomputes).
- `rfc7606.go` hit the 1000-line soft advisory (`posttool-writeedit.py`); a large per-attribute
  validator wants splitting eventually rather than comment-trimming to fit.

## Files

- `scripts/dev/rfc_requirements.py` (+ `_test.py`, `rfc_requirements_gate_test.go`) -- parser, tag
  scanner, polarity + disposition rules, ID-reuse + enrolment + staleness ratchets, `--check` /
  `--write` / `--check-fresh` / `--selftest`.
- `rfc/enrolled.txt`, `ai/RFC-REQUIREMENTS.md` (generated ledger), `rfc/audit/rfc7606.json` (audit verdicts + fingerprints).
- `ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md` (canonical skills).
- `.claude/hooks/pretool-writeedit.py` (rfc-tagged-test hook), `scripts/dev/audit-test-relaxation.py` (shared detector + branch audit), `scripts/dev/hook-fixture-check.py`.
- `scripts/status/verify_run.go`, `Makefile`, `mk/inventory.mk` (gate wiring).
- `internal/component/bgp/message/rfc7606.go` (`ValidateNLRISyntaxAddPath`, `validateMPNLRIField`, MP flag check), `internal/component/bgp/message/rfc7606_withdraw.go` (`SynthesizeWithdraw`), `internal/component/bgp/reactor/session_validation.go` (`enforceRFC7606`, `session_read.go` dispatch) + their tests.
- `docs/features/rfc-status.md`, `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`, `ai/rules/{testing,tdd,discovery-updates,hook-mapping}.md`, `ai/INDEX.md`.
- `test/plugin/rfc7606-reset.ci`, `test/plugin/rfc7606-withdraw.ci`.
- Backlog: `plan/spec-followup-rfc-enrollment.md`.
