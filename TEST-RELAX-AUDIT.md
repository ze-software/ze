# `test-relax:` audit -- 755 tokens, what they are, what to do with them

Date: 2026-08-10. Scope: every `test-relax:` token in `*_test.go`, `*.ci` and
`*.et`, excluding `vendor/`.

**Status: sessions 1 and 2 of the recommendation are DONE (2026-08-10). The
sweep, session 3, is not started.** What landed:

| Done | Where |
|------|-------|
| D-1 `.ci` line counter replaced by a count of what can fail a run (`expect=`/`reject=`/`cmd=`/`assert`/`fail(`/barriers), comment-stripped and statement-anchored | `_CI_COVERAGE`, `_CI_REJECT`, `_CI_EMPTY_NEEDLE`, `_test_weakening_errs` (`.claude/hooks/pretool-writeedit.py`) |
| D-2 the hatch now opens only on a justification this edit WRITES, keyed on the normalized sentence | `_relax_reasons`, `_writes_new_relax_reason` (same file) |
| D-3 an assertion that cannot fail is now refused | `_TAUTOLOGY` (same file) |
| D-4 added tokens found by multiset difference, not a positional slice | `run_audit` (`scripts/dev/audit-test-relaxation.py`) |
| D-5 the whole multi-line justification is captured and wrapped | `relax_reasons`, `report` (same file) |
| The stock is counted and held down | `scripts/dev/relax-census.py`, `test/relax-ceiling.txt`, `make ze-relax-census`, in `ze-verify` both modes |
| The three refuted l2tp justifications removed | `test/plugin/redistribute-l2tp-{announce,withdraw,multi-peer-nexthop}.ci` |

Tests: 28 relax fixtures in `scripts/dev/hook-fixture-check.py` (394/394 pass),
26 cases in `scripts/dev/relax_census_test.py`, 19 in `audit_relaxation_test.py`.
Four rounds of independent review, seven lenses in all, found 6 BLOCKERs and 26
ISSUEs in successive cuts of these fixes; all are closed, and what they found is
recorded in "What the review changed" below.

Counts, with their basis stated, because the first cut published three
irreconcilable ones. **HEAD on 2026-08-10: 751 tokens across 466 test files.** The
working tree that day held 755 across 468, and moved to 752 and back while three
other sessions edited it, which is why the gate counts HEAD. The ceiling is
**751**; this change's own three removals lower it to 748 once committed.

One correction to the session-2 recommendation below, found on reading the files:
restoring the dispatch polling is NOT required and was not done. The refutation
says the observer is dispatch-silent because its sleep/wait conversion is
deferred, not because polling is unsafe, so the only defect was the false
justification. The non-comment content of all three files is byte-identical to
what it was before, which is stronger evidence than a suite run that the change
could not alter behaviour.

---

## Verdict

**The stock is not 755 relaxed tests. It is about 60 relaxed tests and about 690
receipts for a gate that fired on the wrong thing.**

The token exists to buy a pass from `c_test_weakening`
(`.claude/hooks/pretool-writeedit.py`, `_test_weakening_errs`). That detector
counts lines and assertion calls. It does not read meaning. On a `.ci` file it
blocks **any** net reduction in non-comment lines, so replacing three blind
`time.sleep` calls with one `wait_until` barrier is refused as a weakening. The
author writes a token, explains that the test got stronger, and moves on. 48.7%
of the corpus is exactly that.

Three separate problems sit inside the 755, and they need three different
sessions:

| Problem | Count | Who fixes it |
|---------|-------|--------------|
| A gate that misfires, generating receipts | ~430 tokens | Fix the detector first, then bulk-delete |
| Real coverage loss, documented and correct | ~120 tokens | Keep. Move the record where a reader will find it |
| A test relaxed around a defect, on a justification since **REFUTED** | 3 tokens | Fix now. See "The three that must go today" |

Delete all 755 and leave the detector alone, and the stock rebuilds in about four
months. The June-to-August run rate is 183 commits and roughly 250 tokens a
month.

---

## What the token actually does

Verified by reading the producers, not the callers.

| Surface | Symbol | Behavior |
|---------|--------|----------|
| Edit-time gate | `_test_weakening_errs`, `_has_relax_token` (`.claude/hooks/pretool-writeedit.py`) | A token anywhere in the edit's replacement text returns `None` from `c_test_weakening`. The weakening check is skipped in full. **(As audited. Now `_writes_new_relax_reason`, which opens only on a justification the edit writes.)** |
| Diff-time audit | `run_audit` (`scripts/dev/audit-test-relaxation.py`) | Reports a `[RELAXED]` finding for tokens **added** since the base, and only those |
| RFC-tagged tests | `_rfc_tagged_change_err`, called before the relax hatch | The token buys nothing. This ordering is correct and it holds, before and after the fix |

**No make target, no CI stage and no ratchet reads the token stock.** `grep -rn
"test-relax" mk/ Makefile .github/` returns nothing. The audit script is invoked
only from `/ze-review` and `/ze-review-deep`, and it sees only new tokens. The
755 already in the tree are invisible to every gate in the repository. They have
never been reviewed, and no current mechanism can review them.

*(As audited. `make ze-relax-census` now reads the stock and holds it under
`test/relax-ceiling.txt`, in `ze-verify` both modes.)*

---

## The stock

**Basis: the working tree on 2026-08-10, except where a row says HEAD.** The two
differ because this checkout is shared; HEAD held 751 tokens across 466 files.

| Measure | Value |
|---------|-------|
| Tokens | 755 (HEAD: 751) |
| Files carrying at least one | 468, 1 of them untracked (HEAD: 466) |
| `.ci` / `.et`, at HEAD | 542 tokens in 320 files (18.2% of the 1763 functional tests) |
| `_test.go` | 213 tokens in 148 files (4.9% of the 3014 Go test files) |
| Justification prose | About 2,800 comment lines. 62% of reasons span more than one line, median 3, longest 21 |
| Syntax forms (tokens, HEAD) | 527 `# // test-relax:`, 211 `// test-relax:`, 13 `# test-relax:` |
| Empty reasons | 0 |
| Files carrying both an RFC tag and a token | 22 |

Concentration is low: 282 files carry one token, and the worst file
(`internal/component/bgp/message/update_build_test.go`) carries 10.

The `# // test-relax:` form is a Go comment nested inside a hash comment. It
exists because the hook's pattern read `//` only, until it gained the `(?://|#)`
alternation. 313 files still carry the alien form. It works today and it is not a
defect. It is a fossil, and it tells you these tokens were written to satisfy a
matcher rather than to inform a reader.

---

## Five defects in the gate, each reproduced

Run against the real detectors, not against a description of them.

**This section is the audit as taken, in the present tense of that day. All five
are fixed; the symbols it names (`_has_relax_token`, the `.ci` line counter) no
longer exist.** It is kept unedited because a fix is only as good as the failure
it is measured against, and paraphrasing the failure afterwards is how a fix
stops being checkable.

### D-1: the `.ci` detector is a line counter (false positive, ~430 tokens)

```
if fp.endswith(".ci"):
    ...
    if old_l > new_l:
        errs.append(f"removing .ci test lines ({old_l} -> {new_l})")
```

Any net line reduction is a block. Proven:

| Edit | Result |
|------|--------|
| Three `time.sleep` lines replaced by one `wait_until` barrier | `['removing .ci test lines (3 -> 1)']` -- **blocked** |
| Two real assertions replaced by `assert True` twice | `[]` -- **passes silently** |

The detector refuses the improvement and admits the gutting. This single
mechanism produced the largest bucket in the corpus.

### D-2: a token is sticky under `Write` (false negative, 468 files)

For `tool == "Write"`, `new` is the whole replacement file, and
`_has_relax_token` searches the whole of it. Proven: a rewrite that drops both
assertions and adds `t.Skip` yields
`['removing assertions (2 -> 0)', 'adding t.Skip (0 -> 1)']`, and
`_has_relax_token` returns `True`, so `c_test_weakening` returns `None` and the
edit passes.

**Every one of the 468 files carrying a token is exempt from the weakening gate
under a `Write` overwrite, permanently, for one token.** That is roughly 10% of
the test suite. This is the strongest single argument for cleaning the stock.

### D-3: an expected-value swap is invisible

`require.Equal(t, 179, port)` changed to `require.Equal(t, 0, port)` yields `[]`.
Counts are preserved, so the detector sees nothing. Only an RFC tag catches this,
through `_rfc_tagged_change_err`.

### D-4: the audit names the wrong reason (`run_audit` positional slice)

`added_tokens = new_tokens[old_tokens:]` assumes an addition lands last in file
order. Proven:

| Case | Reported | Truth |
|------|----------|-------|
| A token inserted at the top of a file that had two | `['OLD-B']` | The new token is `NEW-INSERTED-AT-TOP` |
| One token deleted and one added, count unchanged | `[]` | A new relaxation was added and went unreported |

A reviewer following `/ze-review` step 0 is shown a justification that belongs to
a different relaxation, with no signal that the mapping is wrong.

### D-5: the audit truncates the reason to its first line

`_RELAX_LINE` ends at `$` under `re.MULTILINE`. 62% of reasons continue onto
later comment lines, median 3 lines. The reviewer is shown a fragment and asked
to confirm it. Half of the fragments in this corpus do not carry a verb.

---

## Triage of the 755

Classification is keyword-based over the reason text. The first line only, so the
boundary between buckets D and H is soft. Full work lists were generated during
this audit. The classifier itself is NOT in the tree, so these figures are a
one-off measurement rather than something a later reader can re-derive; see
"Reproducing the numbers".

| Bucket | n | % | Action |
|--------|---|---|--------|
| **A** Mechanical no-op: blind sleep to barrier, poll loop to helper, unused import removed | 368 | 48.7% | **Delete the token.** No coverage moved. Several strengthen the test |
| **B** Same-session draft churn: "never-executed revision", "brand-new file", "this session's earlier draft" | 18 | 2.4% | **Delete.** The hook diffed an author against himself |
| **C** Lines moved, test restructured or retargeted | 43 | 5.7% | **Delete after a spot check** that the destination exists |
| **D** Coverage genuinely removed with the symbol or feature | 65 | 8.6% | **Keep the fact, drop the token.** Rewrite as a plain comment naming what went and why |
| **E** Environmental or capability skip (`CAP_NET_RAW`, root, `-short`) | 21 | 2.8% | **Keep.** Convert to an explicit `testenv` guard so it stops being spelled as a relaxation |
| **F** Coverage replaced elsewhere | 30 | 4.0% | **Keep, and add the pointer.** Most name no destination test |
| **G** Relaxed around a live defect | 3 | 0.4% | **Fix the code.** See below |
| **H** Unclassified, needs a human read | 207 | 27.4% | Sampled 25: they read like D and F. Expect roughly 150 to D, 40 to F, 15 to A |

Projected after a human pass: **about 430 deletions, about 120 kept as records,
about 200 rewritten as ordinary comments, 3 fixes.**

---

## The three that must go today

`test/plugin/redistribute-l2tp-announce.ci`,
`redistribute-l2tp-withdraw.ci`, `redistribute-l2tp-multi-peer-nexthop.ci`.

Each carries a token reading `KNOWN ENGINE ISSUE (tracked in
spec-test-coverage-gaps, W-2 findings)`, and each test avoids dispatch polling
during connect to route around it.

Two facts about that justification:

1. **It was refuted in place.** The lines immediately above the token read
   `REFUTED 2026-07-16 -- the "KNOWN ENGINE ISSUE" described below DOES NOT
   EXIST`. The stall was a harness defect, not an engine defect
   (`spec-fixit-redistribute-establishment-stall`, E6/E9).
2. **The spec it cites is gone.** `plan/spec-test-coverage-gaps.md` was closed <!-- doc-links: ignore (the spec was deleted at closure in f2126f54f, which is what this sentence reports) -->
   and removed (`13940424d`, `f2126f54f`). A reader who follows the pointer finds
   nothing.

So three functional tests still carry a workaround for a defect that does not
exist. Its justification names a document that does not exist. No gate will ever
re-read the token. Nobody wrote this in bad faith: the refutation was added
honestly, right above the token. Nothing in the system removes the token when its
reason dies, so both statements now sit in the file and contradict each other.

**This is the shape of the whole problem.** A `test-relax:` token is write-once.
It records a belief at one moment and is never revisited. Over three months the
tree accumulated 755 unfalsifiable claims about itself.

---

## Recommendation

Three sessions, in this order. The order is load-bearing. Clean the stock before
you fix the detector, and the stock rebuilds.

### Session 1 -- fix the gate (small, do it first)

| Change | Why |
|--------|-----|
| Replace the `.ci` line count with a count of assertion-bearing constructs (`expect=`, `contains=`, `assert`, `reject=`) | Kills D-1, the source of ~430 tokens. A sleep removed is not an assertion removed |
| Open the hatch only on a justification this edit WRITES, on every tool including `Write` | Kills D-2. The token stops being a permanent per-file exemption |
| Fix `run_audit` to diff the reason SET, not a positional slice, and capture the full multi-line reason | Kills D-4 and D-5. A reviewer is shown the reason that belongs to the finding |
| Add `make ze-relax-census`: print the token count, and ratchet it downward | Nothing counts the stock today. Without a ratchet it regrows silently |

### Session 2 -- fix the three (do not wait for session 1)

Fix `redistribute-l2tp-*.ci`. Delete the three tokens and the dead
`KNOWN ENGINE ISSUE` text, and keep the refutation as the record. Restoring the
dispatch polling is NOT part of it: the refutation says the observer is
dispatch-silent because its sleep/wait conversion is deferred, not because
polling is unsafe, so the only defect is the false justification. This is a
rung-3 item under
`ai/rules/rule-precedence.md`: coverage was reduced to reach green, and the
reason has since been withdrawn.

### Session 3 -- the sweep (the one you offered to schedule)

Work the buckets in order A, B, C, then H. Two mechanical cautions for whoever
runs it:

- **Deleting a token trips nothing, on either carrier.** This was not true when the
  audit was written. `_ASSERT_PAT` matched `require.` and `assert.` in PROSE, so
  removing a Go token whose reason said "the require.Equal on port" read as removing
  an assertion, and the sweep would have needed a fresh token for every token it
  deleted. Session 1 fixed it: `_test_weakening_errs` now counts comment-stripped
  text on both carriers. Pinned by
  `relax-removing-a-token-whose-prose-names-an-assertion`.

For buckets D and F, do not delete. The fact is worth keeping and the token
spelling is not. Rewrite as a plain comment that names the removed symbol and the
replacing test. A reader who greps `test-relax:` is looking for suspicion. A
reader of `// TestFoo covered narrowTS, which now has no non-test caller` is told
something true.

---

## What this says about the mechanism

The token is a good idea implemented against the wrong signal. Forcing a written
reason instead of a silent edit is right. Deciding *when* to force it by counting
lines is what produced 690 receipts and 60 findings, and buried the 60.

Two properties would make the token useful again:

1. **It must expire.** A justification with no owner and no re-read becomes a
   fossil in weeks. The l2tp three prove it took under a month. Either the token
   carries the spec that authorizes it and a gate checks that spec still exists,
   or the sweep repeats every quarter.
2. **It must be rare.** At 755 the token carries no information. Nobody can read
   them, so nobody does, so writing one costs nothing, so more get written. A
   ratchet that only permits the count to fall is what restores the cost.

---

## What the review changed

Three independent lenses ran over the first cut of these fixes: guard evasion,
ratchet integrity, and correctness of the claims. They found 3 BLOCKERs and 9
ISSUEs. All are closed. The list is here because the pattern in it is the same
pattern the audit is about.

| Found | Was | Now |
|-------|-----|-----|
| `.et` was never judged at all | `is_test` named `_test.go` and `.ci` only, so `c_test_weakening` returned `None` for all 164 editor tests. The new `.et` arm inside the detector was unreachable, and the docs claimed coverage that did not exist | `.et` under `test/` is a test |
| One space defeated the hatch | The hatch compared raw LINES, so re-indenting a months-old token by two columns read as newly written and the whole weakening check went away | Keyed on the normalized sentence (`_relax_reasons`) |
| Two words of comment paid for a deleted expectation | `\bassert\b` matched prose and string literals; 144 of 7410 matches over the tracked `.ci` corpus sat on comment lines | Comment-stripped, statement-anchored |
| A dropped `cmd=` was free | The line counter had counted `cmd=`; the first replacement did not | `cmd=` counted |
| `reject=` to `expect=` inverted an assertion at an unchanged count | Nothing saw it | `_CI_REJECT` counts negatives separately |
| An emptied needle stopped being checked | `validateFileContent` guards on `Contains != ""`; the line survives, so no counter moved | `_CI_EMPTY_NEEDLE` |
| The census reported a clean pass over a corpus it had not read | `except OSError: continue` skipped an unreadable tracked test silently | Refuses (exit 2), naming the paths |
| The census counted the working tree | In a checkout shared by many sessions that number moved 751 to 752 to 755 within an hour, on edits by three sessions that had never touched this gate. The gate would have shipped red | Counts HEAD; prints the working-tree delta as advisory |
| A zero count read as a pass | No floor | Refuses when the count is 0 against a live ceiling |
| Nothing watched the ceiling itself | The whole design rested on "a reviewer sees the line" | A raise needs a `raised-for:` line or the census fails |
| Rewording an old token bought a severity downgrade | The multiset difference could not tell a reword from an addition, so editing prose turned `WEAKENED` (a BLOCKER to `/ze-review`) into `RELAXED` (confirm the reason) -- and quoted the old relaxation's reason back | Only a token that RAISES the count is an addition; a reword is reported as a reword |
| A code comment claimed tests that did not exist | "Both are reproduced in `audit_relaxation_test.py`" -- neither was | The two tests exist and are named |
| Three published counts disagreed | 755/468, 753/467, and a measured 751/466 | One basis, stated at every point of use |
| **Round 2.** The `.et` arm was unreachable | `is_test` named `_test.go` and `.ci` only, so `c_test_weakening` returned `None` for all 164 editor tests. The new code and its documentation claimed coverage that did not exist | `.et` under `test/` is a test |
| One invisible character reopened the hatch | Keyed on the justification's text alone, a zero-width space appended to a months-old token read as newly written and bought a whole-file gutting | The count must rise too: a reword is not a new relaxation |
| A needle may legitimately begin with `#` | Comment-stripping before the empty-needle check turned `contains=# tcp.bind` into `contains=`. It refused that line on sight and allowed genuinely emptying it. 9 of the 14 corpus hits are that form | Judged on raw text |
| The first `raised-for:` justified every later raise | A presence test, so the ratchet self-destructed the first time anyone used it as the file instructs | The justification must be new since HEAD |
| A staged new test reddened the gate | The population came from the index and the content from HEAD, so `git add` on a new test made it "unreadable" -- somebody else's `git add`, in a shared checkout | Both come from HEAD |
| `--lower` could launder a raise | Measured against the local file, a hand-raised ceiling could be rewritten to a value still above HEAD's and reported as a lowering | Measured against HEAD |
| A quoted path was dropped, failing OPEN | `git ls-files` without `-z` quotes non-ASCII paths; the quoted form failed `is_test_path` and vanished from the population | `-z` on both listings |
| The reword verdict was false in both directions | Deleting 3 stale tokens and writing 1 real one was called a REWORD at BLOCKER tier, penalising the drain this gate exists to encourage | States facts: what arrived, and what that was already here is gone |
| The gate refused its own cleanup | `_ASSERT_PAT` matched `require.` in prose, so deleting a token whose reason named an assertion read as deleting one. The sweep would have needed a token per token removed | Go counting arms read comment-stripped text |
| Dead code with a false comment | `_RELAX_TOKEN*` had zero callers and a comment saying they stayed "for callers that only ask" | Deleted |
| **Round 3.** The fix for the reword hole refused the honest drain | Requiring one MORE justification than before left keeping dead tokens as the only way through -- the corpus growth this whole gate exists to stop | Identity is the sentence's letters and digits (`_reason_key`); a drain passes, a cosmetic edit does not |
| Comment-stripping was not symmetric | `//` inside a Go string literal is not a comment. A fixture whose value began `"//go:build ...` lost its `t.Fatal` from the OLD side alone, hiding a real deletion | Whole-line comments only, on both carriers |
| The empty-needle check misfired on a needle ENDING in `=` | `contains=ExecStart=` was refused on sight, and emptying that same line was allowed. Five tracked lines have that shape | The trailing `key=` must be preceded by `:` |
| A duplicated `raised-for:` justified any raise | Compared as a multiset, a copy-paste was a difference | A set |
| A fixture passed with its own fix reverted | It asserted the exit code, and the shape trips a second arm | It asserts the message |
| The file's own justification could be copied into a hunk | On Edit the hook saw only the replaced hunk, so the file's existing token read as newly written | The hook reads the file for the hatch (`already=`) |

Two of those deserve naming for what they are, rather than for what they broke.

**The `.et` arm and the false test-reference are the same failure as the corpus
itself.** Both are a claim that a check exists, sitting where the next reader
will believe it instead of looking. That is exactly what 751 unread
justifications are.

**The working-tree count would have shipped a red gate.** A gate that reds on
another session's half-finished work gets switched off, and then the thing it
guarded is unguarded while still appearing guarded. That is worse than never
having built it.

---

## Reproducing the numbers

```
grep -rn "test-relax:" --include="*_test.go" --include="*.ci" --include="*.et" . | grep -v vendor/
```

Detector behavior was probed by importing `_test_weakening_errs` and, at the time
of the audit, `_has_relax_token` from `.claude/hooks/pretool-writeedit.py` (the
hatch is now `_writes_new_relax_reason`), plus `relax_reasons` from
`scripts/dev/audit-test-relaxation.py`, then running the edits tabled under "Five
defects". Every result quoted above is that function's own output.

The triage buckets in "Triage of the 755" came from a keyword classifier run over
the reason texts, which was NOT in the tree: those figures (368, 48.7%, ~430, ~120)
are a one-off measurement of the 2026-08-10 corpus. Treat them as the shape of the
corpus, never as a count to verify against.

**A committed classifier replaced it on 2026-08-16**, because six days passed with
the sweep unstarted and the reason was always the same: nobody could tell which
token sat in which bucket. It is `classify` in `scripts/dev/relax-census.py`,
reached by `--classify`, and it works from the census rows so it can never
disagree with the count about what a token is.

It does NOT reproduce the figures above, by design. A reason that carries a
mechanical signal AND a coverage signal is counted as a KEEP, because deleting
such a token loses a record nothing can recover while keeping one costs a line of
ceiling. 156 of the 780 reasons carry both. So its delete buckets are a FLOOR on
what a sweep can remove, not an estimate of it:

| Bucket | n | Bucket | n |
|--------|---|--------|---|
| A delete | 179 | D keep | 47 |
| B delete | 7 | E keep | 20 |
| C delete | 14 | F keep | 111 |
| | | H needs a read | 402 |

200 of 780 classify as deletable, which would leave 580 against a ceiling of 761.
Read one bucket with `--list --bucket A`.
