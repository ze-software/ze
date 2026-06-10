# Handover: close spec-smart-management

RATIONALE (verify this matches what was agreed):
- Decision 1: the spec sat at `Status: done` without closure, violating the
  closure rule; it was reverted to `in-progress` on 2026-06-10 with an audit
  note (see the spec header). Thomas confirmed keeping it open was right and
  the missing functional test gets written in this session. -> STEPS 1-2
- Decision 2: implementation is complete per the spec's own Implementation
  Audit; ONLY two gaps block closure: the AC-8 functional test never existed
  (`test/plugin/smart-show.ci` is claimed but absent; nothing in test/
  exercises `ze-show:storage-smart`) and the Review Gate section was never
  filled. -> STEPS 1, 3
- Decision 3: closure now requires rewriting `// Design:` references to the
  learned summary BEFORE the spec file is deleted (new rule in
  `ai/rules/planning.md` "Design references survive closure";
  `scripts/dev/check_doc_links.py --design-only` gates it). -> STEP 5
- Assumed (verify before relying on it): on a dev host without SMART-capable
  devices the configured path still answers StatusDone with an empty/
  unavailable device list. READ the source before writing expectations.

FILES ALREADY HANDLED (don't re-read for background, they are correct):
- plan/spec-smart-management.md -- header carries the audit note; the
  Implementation Audit and AC tables are accurate.
- internal/component/storage/show.go -- `ze-show:storage-smart` RPC;
  unconfigured path returns StatusError with the stable message
  "storage SMART management not configured".
- internal/plugins/storage-cmd/yang/ze-storage-cmd.yang -- declares
  `show storage smart` -> `ze-show:storage-smart`.

STEP 1: write the missing functional test (AC-8)
- Location: `test/plugin/smart-show.ci` (the path the spec claims) or the
  suite that actually fits; READ `ai/patterns/functional-test.md` and copy
  the structure of `test/plugin/bfd-show-json.ci` (engine + ze_api observer
  dispatching `show storage smart`).
- Minimum honest coverage: (a) unconfigured daemon -> assert the stable
  error string above (proves RPC wiring through the user entry point);
  (b) configured `storage { smart { ... } }` -> assert StatusDone and the
  `devices` key (verify actual darwin/linux-less behavior in
  internal/component/storage/manager.go before asserting content).
- HARD CONSTRAINT: zero `time.sleep` in the new .ci (the sleep ratchet in
  `make ze-verify-wiring-docs` fails if the count rises; baseline lives in
  test/.ci-sleep-baseline). Use ze_api wait_for_event / the bfd test's
  polling dispatch pattern.
- Update the spec's TDD Functional Tests table Status column for that row.

STEP 2: spec bookkeeping
- Fix the Pre-Commit Verification tables (Files Exist / AC Verified) that
  are empty in the spec; fill with real evidence while running STEP 1.

STEP 3: Review Gate
- Run `/ze-review` against the spec's diff scope; loop until 0 BLOCKER and
  0 ISSUE; paste the clean output into the spec's `## Review Gate` section.

STEP 4: learned summary
- Allocate with `scripts/dev/commit_helper.py learned-next smart-management`
  (NEVER hand-read .counter -- the subcommand is collision-free and creates
  the file). Write the summary; add a one-line entry to ai/LEARNED-INDEX.md.

STEP 5: design-reference rewrite (BLOCKING before the git rm)
- `grep -rn "plan/spec-smart-management" --include="*.go" internal/ cmd/`
  and rewrite each `// Design:` hit to the new learned summary path.
- Confirm with `python3 scripts/dev/check_doc_links.py --design-only`.

STEP 6: closure commits (ONE script, TWO commits)
- Commit A: test + spec (with all edits) + learned summary + .counter +
  LEARNED-INDEX + the Design-reference rewrites.
- Commit B: `scripts/dev/commit_helper.py create --append --remove plan/spec-smart-management.md`
- Check `scripts/dev/verify-status.sh check` first; run `make ze-verify`
  only if STALE (budget 4-10 min).

THEN: report; delete this handover file in commit A (its last item is done).
