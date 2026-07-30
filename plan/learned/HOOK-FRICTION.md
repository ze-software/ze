# Hook Friction Catalog

Documentation of every pre-write hook under `.claude/hooks/` that has
generated a false positive in the learned corpus, with the exact
trigger regex, what the hook blocks, and the session-verified
workaround.

**Read this before writing any new plugin, subsystem, or test file.**
Most sessions that hit hook friction spend 10-20 minutes rediscovering
a workaround that another session already documented.

**This document is a stopgap.** The correct fix is tightening each
hook's regex so the false positive disappears. Until that happens,
the catalog saves rediscovery cost.

Companion: `RECURRING-PATTERNS.md` names `auto_linter.sh` and
`block-silent-ignore.sh` as the two highest-frequency traps — together
they account for over 50 appearances in the corpus.

---

## Table of hooks by frequency

| Hook | Appearances | Status | Entry |
|------|-------------|--------|-------|
| `auto_linter.sh` (goimports post-hook) | 25+ | Retired 2026-04-19 | [Retired](#retired) |
| `block-silent-ignore.sh` | 30+ | Retired 2026-04-19 | [Retired](#retired) |
| `check-existing-patterns.sh` | 15+ | Retired 2026-04-19 | [Retired](#retired) |
| `require-related-refs.sh` | 7 | Active | [require-related-refs.sh](#require-related-refssh) |
| `block-test-deletion.sh` | 7 | Active | [block-test-deletion.sh](#block-test-deletionsh) |
| `block-legacy-log.sh` | 4 | Retired 2026-04-19 | [Retired](#retired) |
| `block-ignored-errors.sh` | 4 | Active | [block-ignored-errors.sh](#block-ignored-errorssh) |
| `block-temp-debug.sh` | 3 | Active | [block-temp-debug.sh](#block-temp-debugsh-now-c_temp_debug-in-pretool-writeeditpy) |
| `block-root-build.sh` | 3 | Active | [block-root-build.sh](#block-root-buildsh) |
| `block-pipe-tail.sh` | 2 | Active | [block-pipe-tail.sh](#block-pipe-tailsh) |
| `block-init-register.sh` | 2 | Active | [block-init-register.sh](#block-init-registersh) |
| `block-encoding-alloc.sh` | 2 | Active | [block-encoding-alloc.sh](#block-encoding-allocsh) |
| `block-system-tmp.sh` | 1 | Retired earlier | [Retired](#retired) |
| `block-panic-error.sh` | 1 | Active | [block-panic-error.sh](#block-panic-errorsh) |
| `block-layering.sh` | 1 | Retired 2026-04-19 | [Retired](#retired) |
| `pretool-writeedit.py` `c_design_without_lsp` | 1 (2026-07-16) | Fixed 2026-07-16 at the source | [c_design_without_lsp](#pretool-writeeditpy--c_design_without_lsp) |
| `validate-spec.sh` argv false-green | 3 (2026-07-16) | Fixed 2026-07-16 at the source | [F1](#f1-validate-specsh-false-greened-when-invoked-via-argv) |
| `validate-spec.sh` Current Behavior citation regex | 1 (2026-07-16) | Active | [F2](#f2-validate-specsh-rejects-the-citation-form-the-rules-mandate) |
| `validate-spec.sh` RFC-existence check dead (regex typo) | 1 (2026-07-22) | Active | [F11](#f11-validate-specsh-rfc-existence-check-is-dead-code-regex-typo) |
| `spec-closure-check.py` slice-scoped learned false-positive | 1 (2026-07-22) | Active | [F12](#f12-spec-closure-checkpy-high-confidence-signal-misfires-on-slice-scoped-learned-summaries) |
| `commit_helper.py create` leaks a `ze` daemon and stalls | 6+ (2026-07-26) | Active | [F15](#f15-commit_helperpy-create-leaks-a-ze-daemon-and-stalls-forever) |
| `GOCACHE` relocation is make-scoped (disk exhaustion) | 1 (2026-07-26) | Active | [F16](#f16-gocache-relocation-is-make-scoped-so-the-recommended-workflow-bypasses-it) |
| `pretool-writeedit.py` `c_throwaway_tests` | 1 (2026-07-16) | Active | [F5](#f5-c_throwaway_tests-blocks-legitimate-scriptsdev-test-filenames) |
| `session-end-summary.sh` clobbers hand-written digests | 1 (2026-07-22) | Active | [F9](#f9-session-end-summarysh-destroys-the-digests-post-compactionmd-asks-for) |
| `stress-repro.py` broken argv / crash-only detection | 1 (2026-07-22) | Fixed 2026-07-22 at the source | [F10](#f10-stress-repropy-was-broken-for-every-sub-suite-and-said-so-as-reproduced) |

Non-hook tooling friction filed the same day (the LSP gate, the commit gate's
advice, `commit_helper.py --body`, and rule/gate vocabulary drift) is in the same
[2026-07-16 section](#filed-2026-07-16-seven-frictions-one-session-zero-reports),
because that is where a reader looking for "why did the tooling fight me" goes.

---

<!-- auto_linter.sh, block-silent-ignore.sh, and check-existing-patterns.sh
     moved to the Retired section at the bottom on 2026-04-19. -->

---

## `pretool-writeedit.py` — `c_design_without_lsp`

**Trigger.** `Write`/`Edit` on `plan/spec-*.md` or `plan/design-*.md` when neither
marker exists: `tmp/session/.lsp-invoked-<sid>` or `tmp/session/.source-read-<sid>`,
where `<sid>` comes from that file's own `session_id()`.

**What it blocks.** Writing a spec without having investigated the implementation
first. The intent is sound and worth keeping (`ai/rules/no-fabrication.md`): read the
function that PRODUCES the behavior before authoring a spec that claims something
about it.

**This is not a regex false positive — the two ends disagree about the session id.**
The marker *writer* recorded under `claude-session-fallback`
(`.lsp-invoked-claude-session-fallback`, `.source-read-claude-session-fallback`, both
freshly stamped), while `c_design_without_lsp` resolved the *same* session to a pid
(`36334`) and looked for `.lsp-invoked-36334`, which nothing ever writes. The gate is
therefore unsatisfiable in any session where the two resolutions diverge: reading the
source and invoking LSP both fail to clear it, and the message advises doing exactly
what the session has already done — which is the worst kind of block, because it reads
as "you skipped a step" when you did not.

**FIXED 2026-07-16 — no workaround needed.** `session_id()` in
`pretool-writeedit.py` now mirrors `lib/session-id.sh` exactly, so both ends resolve
the same id. Verified: the shell lib and the Python hook both return
`claude-session-fallback` on this host (they returned `claude-session-fallback` and
`36334` before), and a spec write passes with no marker symlinks present.

Three divergences existed, despite the Python's own comment claiming it mirrored the
shell:

| | `lib/session-id.sh` | `pretool-writeedit.py` (before) |
|---|---|---|
| 1st choice | `--session-id` from the process tree | JWT env var |
| 2nd choice | JWT env var | `--session-id` from the process tree |
| Last resort | `claude-session-fallback` (stable) | `str(os.getppid())` (**unstable**) |
| Dead branch | — | name-match `argv0 == "claude"` returning `str(pid)`, which its own comment said never fires |

The last-resort row is what bit: with no `--session-id` in the tree and no JWT, the
shell wrote `.lsp-invoked-claude-session-fallback` while the check looked for
`.lsp-invoked-36334`.

**Why this class of bug is nasty.** It does not fail open, it fails **closed**: the
gate blocks work that *was* done, and doing it again cannot clear the block, because
the evidence lands under a name the reader never looks at. The message then accuses
the session of skipping a step it did not skip.

**Historical note.** `tmp/session/` still holds pid-named symlinks from nine earlier
sessions (`.lsp-invoked-2605`, `-40688`, `-40694`, `-40763`, `-52264`, `-60632`,
`-6503`, `-97940`, `-97941` → `./.lsp-invoked-claude-session-fallback`). Each one is
a session that hit this, worked around it, and moved on. They are harmless to leave
and are no longer created. If you ever need to bridge a marker by hand again, that is
a signal the two resolvers have drifted apart once more — fix `session_id()`, do not
re-add the symlink, and never bridge a marker for work that was not actually done.

**Related.** `state_file(sid)` in the same file keys off the same `session_id()`, so
`session-state-<sid>.md` was mis-resolved too: the SessionStart banner reports
`session-state-claude-session-fallback.md` while the check demanded a pid-named file.
The single fix corrects all four consumers (`.lsp-invoked-`, `.source-read-`,
`.session-`, `session-state-`).

---

## Filed 2026-07-20: `deferral-in-diff` gate false-positives on the rule corpus

**Trigger.** `commit_helper.py`'s `deferral_in_diff_problems` scans a commit's
added prose for un-homed deferral language (`DEFERRAL_PATTERNS`: `future work`,
`out of scope`, `postpone`, `follow-up work`, ...) and BLOCKS unless
`plan/deferrals.md` rides along. It already blanks quoted/backticked spans, so it <!-- doc-links: ignore (historical; deferrals are now sharded under plan/deferrals/) -->
fires only on BARE prose — but the rule corpus is full of bare prose that
DISCUSSES deferral policy: `no-parking.md` ("genuinely separable, out-of-scope
`future work`"), `planning.md` (status vocab, Consequences), `handoff.md`
("Speculative `future work`"), `config-design.md` ("as `follow-up work`"), and
one true word-sense collision — `no-sprintf-alloc.md`'s "buffer goes `out of
scope`" (lexical scope, unrelated to deferring work). The generated
`ai/rules/CONDENSED.md` flattens all of them, so every `make ze-rules-condensed`
regeneration commit re-tripped the gate.

**Old workaround (bad).** Pass `plan/deferrals.md` in `--file`. This is <!-- doc-links: ignore (historical; deferrals are now sharded under plan/deferrals/) -->
all-or-nothing: it disables the ENTIRE gate for that commit, so a genuine un-homed
deferral elsewhere in the same commit would sail through, and it forces staging a
shared file (cross-commit hazard per `git-safety.md`).

**Fix (2026-07-20, at the source).** `_prospective_added_lines` now carries each
`+` line's file path (from the diff's `+++ b/<path>` header), and
`deferral_in_diff_problems` skips lines whose path is under
`DEFERRAL_SCAN_EXEMPT_DIRS` = (`ai/rules/`, `.claude/rules/`). Those trees discuss
deferral as policy; the generated digest lives there too, so the recurring case is
covered by one directory rule. Specs (`plan/spec-*.md`), code, and comments stay
in scope — that is where a real deferral gets written and must be homed. Proven by
`TestDeferralInDiff` (rule-doc + CONDENSED exempt; code + spec still block) and the
`commit-gate` fixtures (`prose-still-caught`, `code-literal-exempt` unchanged).

---

## Filed 2026-07-16: seven frictions, one session, zero reports

One long session hit every friction below and filed none of them at the time.
They are recorded here in the Format `ai/rules/friction-reporting.md` prescribes
(Friction / Pattern / Impact / Rule decision / Proposed fix) rather than this
file's older Trigger/Blocks/Workaround shape, because most are not hook false
positives: two are a validator that passes without checking, one is a gate whose
own advice does not work, one is an argparse edge, one is rule/gate vocabulary
drift. The Trigger/Blocks/Workaround entries below this section are unchanged.

**Read F8 first if you read only one.** It is why the other seven arrived in one
batch at the end of a session instead of one at a time as they were hit.

---

### F1: `validate-spec.sh` false-greened when invoked via argv

**Friction:** `bash .claude/hooks/validate-spec.sh plan/spec-foo.md` exited **0**
without running a single check, which reads as "spec valid". The script takes a
PostToolUse JSON payload on stdin: `INPUT=$(cat)` at `:7`, then jq for
`.tool_name` (at `:8` before the fix, `:32` after). Under argv, stdin is empty,
jq returns empty, `TOOL_NAME` is `""`, and the pre-fix guard at `:11-14`
(`if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then exit 0; fi`)
treated "I could not tell what tool this is" exactly like "this is a tool I do
not handle". Verified before the fix: argv on a real spec gave `EXIT=0` and no
output; the same spec over JSON stdin produced a verdict. All `:NN` citations in
this entry are post-fix line numbers unless marked otherwise.

**Pattern:** Recurs, and demonstrably did. Two to three agents found this
independently on 2026-07-16 and each warned the *next* agent in its handoff. It
travelled three times as folklore and never reached a file. Any agent who
manually validates a spec hits it, because argv is the obvious way to run a
script that takes a path.

**Impact:** Unbounded and silent, which is the worst combination. Every spec
"validated" this way passed unchecked, and the agent then reported the spec as
validated in good faith. This is exactly `ai/rules/fail-closed-guards.md`'s
zero-value trap: exit 0 is a legitimate-looking answer, so the miss is invisible
at every later layer.

**Rule decision:** No new rule. `ai/rules/fail-closed-guards.md` (committed the
same day, `c49a6dcd9`) already names this shape precisely: "a guard must fail
closed or say something", and "a zero value that downstream reads as a legitimate
answer is how it hides". The rule was right and nothing applied it to the hook
layer. What was missing was a durable filing destination, which is F8.

**Proposed fix:** DONE. `validate-spec.sh` now distinguishes the two cases: an
unparseable payload (`:32-34`) or an absent tool name (`:35-37`) calls
`usage_refusal()` (`:21`) and exits 2 with the correct invocation; a tool name
that is present but not `Write`/`Edit` still exits 0 quietly (`:41-44`). The early
exit was never the bug and is kept: a hook MUST no-op on tools it does not
handle. Locked by three new fixtures in `scripts/dev/hook-fixture-check.py`
(`validate-spec-argv-no-stdin-refuses`, `-absent-tool-name-refuses`,
`-other-tool-quiet-pass`), which drive a structurally INVALID spec so that a pass
can only mean no check ran. `make ze-hook-test`: 131/131 parity + 37/37 fixtures.

---

### F2: `validate-spec.sh` rejects the citation form the rules mandate

**Friction:** The Current Behavior check (`:125`, `:127`) requires a source-file
bullet matching `` `path.(go|py|rs|ts|js)` `` with the closing backtick
IMMEDIATELY after the extension. So it REJECTS `` - [ ] `authz.go:385` ``, the
`file:line` form `CLAUDE.md` ("cite the function that PRODUCES the behavior as
`file:line`") and `ai/rules/fail-closed-guards.md` (`validator.go:631`) both
require, and ACCEPTS only `` `authz.go` `` or `` `authz.go` line 385 ``.
Verified against the live regex, six forms:

| Bullet | Verdict |
|---|---|
| `` - [ ] `commit_helper.py:193` `` | REJECT |
| `` - [ ] `commit_helper.py` line 193 `` | ACCEPT |
| `` - [ ] `internal/component/authz/authz.go:385` `` | REJECT |
| `` - [ ] `internal/component/authz/authz.go` `` | ACCEPT |
| `` - [ ] `.claude/hooks/validate-spec.sh` `` | REJECT |
| `` - [ ] `Makefile:243` `` | REJECT |

Two further limits confirmed: the section is truncated to its first 30 lines
(`:122` `| head -30`), so a spec listing many files silently loses the tail; and
the extension allowlist has no `.sh`, `.mk`, `.py`-adjacent shell, or
extensionless `Makefile`, which is perverse for a spec about shell or build code
and such a spec cannot satisfy the check at all except by citing an unrelated
`.go` file.

**Pattern:** Recurs for every spec whose subject is not Go, and for every agent
who follows `no-fabrication.md` literally. Two rules in the same repo demand
opposite things; the agent obeys one and a gate rejects it for obeying.

**Impact:** Minutes per spec, plus a worse second-order effect: the fix that makes
the gate green is to DROP the line number, degrading the citation precision
`no-fabrication.md` exists to enforce. A gate that rewards weaker evidence is
worse than no gate.

**Rule decision:** Update the hook, not the rules. The rules are right:
`file:line` is the correct citation form. This is a hook false positive against
the project's own mandated style.

**Proposed fix:** Widen the regex to accept an optional `:<line>` or
`:<start>-<end>` suffix inside the backticks, add `sh|mk|yang|ci` and an
extensionless `Makefile` to the allowlist, and raise or remove the `head -30`
truncation. NOT done here: out of scope for this task, and the change deserves a
survey across `plan/spec-*.md` first (as spec-followup-hooks AC-4 did for arrows)
to confirm it does not turn ~40 legacy specs red. Filed rather than fixed.

---

### F3: LSP is unavailable to subagents, and the rule shames them for it

**Friction:** `.claude/rules/session-start.md` makes
`ToolSearch query="select:LSP"` the "UNCONDITIONAL FIRST ACTION", BLOCKING, with
a six-row table of "banned excuses" and the line "If it is not, you have violated
this rule. Apologize, load it, proceed." In a subagent, that call returns **"No
matching deferred tools found"**. Verified: this filing's own first action was
that exact call and that exact result; LSP is absent from the subagent's deferred
tool list entirely.

**Pattern:** Recurs for every subagent, every session, permanently. It is not
environmental.

**Impact:** Small in time, corrosive in effect. A rule that cannot be satisfied
teaches agents that BLOCKING rules are negotiable, which is the opposite of what
the six-row table is defending. It also produces false self-reports: an agent that
"loaded LSP" and got nothing may believe it complied.

**Nuance that the folklore version gets wrong.** The gate is still *clearable*.
`block-until-lsp.sh:88-97` writes the marker when the ToolSearch **query text**
matches `/LSP/i` (`grep -qi "LSP"` on `$QUERY`), not when a tool actually loads.
So running the mandated command clears the block even though no LSP tool exists.
The rule is unsatisfiable in the CAPABILITY sense (a subagent can never use LSP),
not in the GATE sense (a subagent can always clear it). "The rule is unsatisfiable
for subagents" is therefore too strong; the honest claim is that the mandated
action is a no-op for subagents that the gate accepts anyway.

**Rule decision:** Update `.claude/rules/session-start.md`. Not by weakening the
rule for main sessions, where it is load-bearing and its diagnosis is right.

**Proposed fix:** Add one row to the checklist scoping step 1: if `select:LSP`
returns no match (subagent context), that is a known environment limit, not an
excuse. Proceed, and satisfy `no-fabrication.md` by reading the producing `.go`
source instead, which `mark-source-read.sh` already accepts as equivalent evidence
for the `design-without-lsp` gate. That equivalence already exists in the hook
layer (`hook-mapping.md:83`); the rule text has simply not caught up. NOT done
here: `.claude/rules/session-start.md` was out of scope for this task.

---

### F4: the commit gate's structural-red advice does not do what it says

**Friction:** `commit_helper.py`'s structural-gate refusal (`:1125-1135`) says:
"re-run `make " + gate_reds[0] + "` (or `make ze-verify`) until green. If you
already fixed it, that re-run refreshes tmp/ze-verify-failures.json and clears
this." It does not. `gate_reds[0]` is a stage name from `STRUCTURAL_GATES`
(`:492-503`: `ze-lint`, `ze-lint-changed`, `ze-tier-check`, ...), and running that
stage directly never writes the JSON. Verified at the producer: the only writer of
`tmp/ze-verify-failures.json` is `scripts/status/verify_run.go:326`
(`os.WriteFile(filepath.Join(root, failuresJSONPath), ...)`, path constant at
`:28`), and `verify_run.go` is invoked only by `ze-verify` (Makefile:279-280) and
`ze-verify-changed` (`:293-294`). `make ze-lint-changed` (`:243-247`) is
`golangci-lint run $pkgs` and touches nothing else. Only the parenthetical
`make ze-verify` actually clears the gate; the advice the message leads with does
not, for any of the eight gates.

**Pattern:** Recurs for anyone who hits a structural red and follows the
instruction, which is precisely the audience the message is written for.

**Impact:** Real time lost today. The failure mode is nasty: you fix the code, run
the named target, watch it go green, re-run the commit helper, and get the
identical refusal with no indication why. The tree is fixed and the gate still
says it is broken, so you start doubting the fix rather than the message.

**Rule decision:** No rule change. This is a wrong string in a tool, and
`ai/rules/git-safety.md:224` already documents the JSON's refresh semantics
correctly ("which `verify_run.go` rewrites after every run"). The rule is right;
the error message contradicts it.

**Proposed fix:** Reword `:1132-1134` to name a target that actually refreshes the
artifact: fix at the source, then `make ze-verify` (or `make ze-verify-changed`)
(optionally suggesting `make <stage>` as a fast inner-loop check while making it
explicit that only a full verify run rewrites the routing index and clears the
gate). NOT done here: `scripts/dev/commit_helper.py` is explicitly out of scope for
this task (it changed today).

---

### F5: `c_throwaway_tests` blocks legitimate `scripts/dev/` test filenames

**Friction:** `c_throwaway_tests` in `.claude/hooks/pretool-writeedit.py`
(`:1225-1233`) blocks a `Write` whose basename matches `test_.*\.(go|py|sh)$` OR
`_test_.*\.(go|py|sh)$` unless the path contains `/internal/`, `/test/`, or
`/cmd/`. `scripts/dev/` is none of those, so `audit_test_relaxation_test.py` was
rejected and had to be renamed `audit_relaxation_test.py`. Verified: the infix
`_test_` in `audit_test_relaxation` matches, and the renamed file matches neither
pattern (`audit_relaxation_test.py` ends `test.py`, not `test_`), which is why the
rename worked.

**Pattern:** Recurs for any `scripts/dev/` tool whose name contains a `test_` or
`_test_` token, a live category, since `scripts/dev/` is exactly where this
repo's test *infrastructure* lives (`hook-fixture-check.py`,
`hook-parity-check.py`, `audit-test-relaxation.py`, `commit_helper_test.py`).

**Impact:** Small per instance (one rename), but it corrupts naming: the file is
now named around the hook instead of its subject, and `audit_relaxation_test.py`
is a worse name than `audit_test_relaxation_test.py` for a thing that audits test
relaxation. The next reader has no idea why.

**Rule decision:** Update the hook. The intent, no throwaway tests in scratch
locations, is sound and worth keeping; `scripts/dev/` is not a scratch location.

**Proposed fix:** Add `scripts/` to the allowed-path set alongside `internal/`,
`test/`, and `cmd/` at `:1228-1232`. Note `/tmp/` and `/var/tmp/` are already
handled separately at `:1222-1224`, so the scratch-location intent survives
intact. NOT done here: outside this task's scope, and `pretool-writeedit.py` is
covered by the parity golden table, so the change needs a re-bless.

---

### F6: `commit_helper.py --body` and `--`-prefixed text (CLAIM CORRECTED)

**The reported version of this friction is WRONG and is filed corrected.** The
report said "a body containing `--mode sink` makes argparse reject the whole
invocation". It does not. Verified by driving the real script:

| Invocation | Result |
|---|---|
| `--body "--mode sink"` | **ACCEPTED**, parses, proceeds to the verify gate |
| `--body "--unverified now"` | **ACCEPTED**, parses, proceeds to the verify gate |
| `--body "--mode"` | **REJECTED**, `argument --body: expected one argument` |
| `--body=--mode` | **ACCEPTED**, parses, proceeds to the verify gate |

**Friction:** `--body` (`:1284-1289`, `action="append"`) rejects a value that is a
single `--`-prefixed token with NO whitespace. A multi-word value starting with
`--` is fine, because argparse's option detection returns "this is a value, not an
option" for any argument string containing a space. So the trigger is not
"`--`-prefixed text" but "`--`-prefixed text with no space in it".

**Pattern:** Rare and narrow. It fires only when a body paragraph is exactly one
`--`-prefixed token, which is an unusual thing to write. The common case that
looks dangerous (quoting a flag with its argument, `--mode sink`) works.

**Impact:** Overstated in the report. The workaround is one character:
`--body=--mode`.

**Rule decision:** **No rule, and no fix.** This entry is filed for a different
reason than the other six: as the worked example of why
`ai/rules/no-fabrication.md` applies to friction reports too. It was passed on as
a confident claim, cost was attributed to it, and driving the producer showed the
stated trigger was not the real one. A false entry in the friction record is the
same disease the record exists to cure: a future agent would have spent time
"fixing" a bug at a shape that does not fire. Verify a friction at the producer
before filing it, exactly as for any other behavioral claim.

---

### F7: `deferral-tracking.md` taught `deferred` while its gate checked `open`

**Friction:** The rule taught a status vocabulary its own gate did not read.
`deferral_unassigned_problems` filtered `if status != "open": continue` while 40
of 68 rows in `plan/deferrals.md` carried `deferred`, the word <!-- doc-links: ignore (historical; deferrals are now sharded under plan/deferrals/) -->
`ai/rules/deferral-tracking.md` uses for exactly that state. The gate never looked
at them. Fixed today in `c4f570214`.

**Pattern:** Recurs whenever a rule and the gate enforcing it name the same
concept in different words, and nothing ties them together. Filed as the worked
example because it is the one that got CAUGHT, so its full shape is visible.

**Impact:** Four deferral rows written, gate run, green, believed enforced,
and nothing had checked. It survived four walkthroughs by its own author. Two
independent improvements had to land together to do anything: a sibling agent had
tightened the destination check earlier the same day into the strictest check in
the repo, and it sat behind a status filter 60 of 68 rows bypassed, so it caught
nothing. Neither half worked alone. That is the signature of vocabulary drift:
each piece looks correct in isolation.

**Rule decision:** Already done in `c4f570214`, and the fix generalizes better
than a one-word patch would have. The status check became a DENYLIST of terminal
states (`done`, `cancelled`, `resolved`) rather than an allowlist of live ones, so
an unknown status is now live and gets checked. An allowlist re-runs this bug the
next time the vocabulary drifts, which is precisely how `deferred` got through.
`deferral-tracking.md` gained a Status Vocabulary table naming the exact terminal
set the gate reads.

**Proposed fix:** None outstanding. The transferable lesson, and the reason it is
in this file: **when a gate filters on a vocabulary, the rule teaching that
vocabulary and the gate reading it must name the same words, and the gate must
fail closed on a word it does not know.** F1 is the same disease in a different
organ: `open` vs `deferred` there, `""` vs `Bash` here. Both let an unrecognized
input take the quiet path.

---

### F8: the rule fired zero times in the session that generated seven

**Friction:** `ai/rules/friction-reporting.md` says to report friction
"immediately", with a Format and a rule decision. Across one very long session
that hit at least seven reportable frictions, it fired **zero** times. Not once
late. Never. Worse, F1 was found INDEPENDENTLY by two to three agents, and each
one warned the next agent in its handoff report. The same finding was rediscovered
and re-transmitted three times, as folklore, and never reached a durable artifact.
Every rediscovery paid full price.

**Pattern:** Structural, not a lapse of attention, and it recurs by construction.
The rule asks the reader to remember to report, and "report" resolves in practice
to "say it in chat". Chat scrolls away, and the next session starts with none of
it. Nothing collects. A rule whose only enforcement is the reader's memory fires
at whatever rate the reader remembers, which across a long session tends to zero:
friction is hit while *inside* another task, and the cost of stopping to file it
is charged against the task the agent is being judged on, so it is always
deferred, and "later" never arrives.

**Impact:** The largest of the eight, because it is the multiplier on the other
seven. It is why they arrived as an end-of-session batch instead of seven timely
reports, and why F1 cost three agents instead of one. A handoff is not a record:
it reaches exactly one successor and dies there.

**Rule decision:** Update `ai/rules/friction-reporting.md`, minimally. Its
diagnosis is correct and its Format is the right one; every entry in this section
uses it. The single missing element was a destination. Do not rewrite a rule that
is right about everything except where the output goes.

**Proposed fix:** DONE, one line, in the Timing section: reporting in chat is not
filing; hook and tooling friction is not reported until it is written to
`plan/learned/HOOK-FRICTION.md` in the prescribed Format, and a finding passed
only to the next agent in a handoff is folklore, not a record.

**Honest limit of this fix.** It converts "report it" into "write it here", which
is necessary but not sufficient: it is still prose asking the reader to remember,
so it will still be skipped under task pressure. The real fix is mechanical, and
this section does not deliver it. A Stop-hook prompt when a session's transcript
shows hook rejections or gate refusals with no diff to this file would collect
what memory does not. Filed as the next step rather than claimed as solved. If a
future session finds this file still gathering seven-at-a-time batches, that is
the evidence the mechanical version is owed.

---

## Filed 2026-07-22: two frictions, one session

### F9: `session-end-summary.sh` destroys the digests `post-compaction.md` asks for

**Friction:** `.claude/rules/post-compaction.md` tells every agent to write file
digests and `-> Decision:` annotations into the per-session state file
(`tmp/session/session-state-<spec-stem>-<SID>.md`) and names it as the Tier-1
recovery source after compaction. The Stop hook `session-end-summary.sh` writes
that same path with a generated snapshot (branch, last commit, an uncommitted
file list). It **overwrites**, so a session that follows post-compaction.md and
writes a careful diagnosis into that file loses all of it the first time the
Stop hook runs, with no warning and no backup.

**Pattern:** Two documented mechanisms own one path, and the one that clobbers
runs last. It is silent: the agent that wrote the digest is usually not the one
that discovers it is gone, and the generated snapshot looks plausible, so the
loss reads as "the previous session did not write digests" rather than "the
hook deleted them". That misattribution is the expensive part -- the next
session concludes digests are not worth writing.

**Impact:** In this session it destroyed a fully cited root-cause diagnosis for
`bgp-rs-reactor-fastpath` (which producer bypasses which gate, with `file:line`
for each). It was recoverable only because the authoring agent still had it in
context. After a compaction it would simply have been gone -- which is exactly
the case post-compaction.md exists to cover.

**Exact mechanism** (`.claude/hooks/session-end-summary.sh:74-79`): the write is
`{ echo "# Session State"; echo "$NEW_SNAPSHOT"; ...previous... } > "$STATE_FILE"`
-- a truncating redirect. What it carries forward is only what its awk at
`:60-66` extracts from the old file: printing starts at the FIRST line matching
`/^## Session:/` and stops after the second such block. Therefore:

- hand-written text ABOVE the first `## Session:` heading is dropped outright
  (this is what happened here: the digest file had no `## Session:` heading at
  all, so the extraction matched nothing and the whole file was replaced);
- hand-written text BELOW it survives only until two more snapshots push it out
  of the two-block window.

**Workaround (session-verified):** keep hand-written detail in a sibling file
the hook does not own, e.g. `tmp/session/notes-<SID>.md`, and leave a pointer to
it *below* the hook's first `## Session:` heading. The notes file is the durable
store; the pointer is a convenience that itself expires after two more
snapshots.

**Rule decision:** a rule change is NOT the fix here; this is mechanical. Either
`session-end-summary.sh` should append under its own heading (and never rewrite
content it did not write), or post-compaction.md should name a different,
hook-free path for hand-written digests. Prefer the former: the state file's
value is that one path is canonical. Not fixed in this session -- the hook is
shared and changing it mid-session would race other live sessions.

### F10: `stress-repro.py` was broken for every sub-suite, and said so as "reproduced"

**Friction:** `ai/rules/flaky-under-load.md` sends you to
`scripts/dev/stress-repro.py` for exactly the failure class this session was
working. Three defects, in descending cost:

1. It appended `-v` AFTER the test selector. The suite runners parse
   `<suite> [options] [tests...]`, so `-v` was consumed as a test name and every
   invocation died with `Error: test "-v" not found`. Worse, with
   `--any-failure` that non-zero exit is itself scored as a reproduction, so the
   tool reports `*** REPRODUCED on invocation 1 ***` for its own broken argv.
2. `suite` and `--test` were single tokens, so a sub-suite (`bgp plugin`) could
   not be expressed at all -- only whole-command suites.
3. Only CRASH signatures (panic / `DATA RACE` / runtime error) counted as a
   reproduction; everything else was truncated to the last 500 bytes and
   discarded. An assertion flake -- a missed `expect=` -- never carries a crash
   signature, so the tool reported "not reproduced" while throwing away the one
   capture that showed the failure.

**Pattern:** a diagnostic tool whose own failure mode is indistinguishable from
the failure it is diagnosing. Defect 1 produces the same "REPRODUCED, exit 1"
banner as a genuine hit, so the operator's next move is to open the log and read
a runner usage error as evidence about the daemon.

**Impact:** ~20 minutes, and it would have been much worse without the verbose
log: the first "reproduction" of the bmp-locrib flake was the tool's own argv
error. A related trap cost another cycle: `ZE_TEST_NO_BUILD=1` means the run
tests whatever `bin/ze` already is, so a landed fix still "reproduces" against a
stale binary until you rebuild.

**Rule decision:** rule updated, not just the tool.
`ai/rules/flaky-under-load.md` gains the sub-suite form, an explicit
"a crash is not the only reproduction -- pass `--any-failure` for assertion
flakes" paragraph, and the stale-`bin/ze` warning; `ai/INDEX.md`'s tool row
gains the same three facts, since that is where an agent looks first.

**Proposed fix:** DONE for all three defects in `scripts/dev/stress-repro.py`
(`-v` before the selector, `shlex.split` on both suite and selector,
`--any-failure`). The tool still cannot tell a runner usage error from a product
failure; a cheap improvement would be to treat a first-invocation non-zero exit
whose output contains no test result line as a setup error (exit 2) rather than
a reproduction.

---

## Filed 2026-07-23 (feature-gate-12 session): verify runner refused its own documented invocation

### F14: `verify_run.go` dry-run guard vs `ZE_VERIFY_LOG=` make override (FIXED at source)

**Friction:** `make ze-verify ZE_VERIFY_LOG=tmp/ze-verify-gate12.log` -- the
exact invocation `ai/rules/bash-output.md` and the `pipe-tail` Bash hook print
as the sanctioned form -- exited 2 with the "refusing to run under make -n"
message. GNU make 3.81 (the macOS system make) writes a command-line variable
override into MAKEFLAGS as the FIRST word with no `--` separator
(`MAKEFLAGS="ZE_VERIFY_LOG=tmp/ze-verify-gate12.log"`, captured), and
`makeDryRun` (scripts/status/verify_run.go:111) read that word as concatenated
short flags: `ContainsAny(field, "ntq")` matched the 't' in "tmp".

**Pattern:** the guard's negative-side test table was built from GNU make 4.x
captures (` -- FOO=bar`); the 3.81 shape was never captured, and the tooling's
own recommended invocation was the collision.

**Fix (same session):** a flags word never contains '=', so `makeDryRun` now
skips '='-bearing first words; two captured-MAKEFLAGS rows added to
`TestMakeDryRunDetectsDashN` (the bare-3.81-override false positive and `-n`
still detected ahead of an override).

## Filed 2026-07-26 (QEMU-rot sweep): three frictions

### F15: `commit_helper.py create` leaks a `ze` daemon and stalls forever

**Friction:** generating a 6-commit script took over an hour and repeatedly
appeared hung. Each `commit_helper.py create` invocation left a `ze -` child
running: `pgrep -P <script pid>` showed `ze -` alive for 18+ minutes, and the
generating shell never advanced past it. Killing that child let generation
resume immediately, then the next `create` stalled the same way. A separate
orphan (`ze start ze-bgp.conf`, ppid 1) was found still running after 1h01m from
an earlier functional run.

The mechanism: a gate spawns `ze` with stdin at EOF. `ze -` reads its config
from stdin (`cmd/ze/ze_core_dispatch.go:404`), gets an empty config, and starts a
daemon -- which by design never exits. `commit_helper.py` itself never invokes
`ze` (`grep` for `bin/ze`/`ZE_BIN` in it returns nothing), so the call is inside a
gate it shells out to; the log line `warning: running without root; privileged
operations (port 179, raw sockets, FIB) will fail` is the daemon announcing
itself.

**Pattern:** a fail-open subprocess contract. Nothing bounds the child, and an
empty config is indistinguishable from a valid one, so "no config" degrades into
"run forever" instead of erroring. Compare `ai/rules/fail-closed-guards.md`: a
check that cannot complete must say so, not hang.

**Workaround used:** wrap the generation in a loop that reaps any `ze -` older
than 60s. Do NOT hand-write the commit script to dodge it -- the gates it runs
(verify-status, discovery-index, deferral homing) are the point.

**Proposed fix:** give the gate's `ze` invocation a bounded context/timeout and a
real config path, or have `ze -` refuse an empty stdin config rather than
starting an empty daemon. Either removes the hang; the second is the fail-closed
one.

### F16: `GOCACHE` relocation is make-scoped, so the recommended workflow bypasses it

**Friction:** a session filled the disk (ENOSPC, 927G volume) largely via
`~/Library/Caches/go-build`, despite the repo relocating the Go build cache.
Every Bash tool call then failed, because the tool could not create its own
output file -- an unrecoverable state from inside the session.

`Makefile:17` sets `export GOCACHE := $(CURDIR)/cache/go-cache`, with `cache/`
symlinked to `~/.cache/ze` (`scripts/dev/ensure-links.py:59-64`). Make `export`
only reaches processes make spawns. A `go` command run directly inherits nothing
and falls back to Go's platform default, `os.UserCacheDir()/go-build` =
`~/Library/Caches/go-build` on macOS. `GOLANGCI_LINT_CACHE` (`Makefile:18`) has
the same scope, and the post-edit format hook runs `golangci-lint` directly.

**Pattern:** two rules pull in opposite directions. `ai/rules/bash-output.md`
says prefer `make`, but also that a bare `go test` lies without the feature tags
-- so agents run `go test -tags "ze_core <36 tags>" ./...` directly, dozens of
times, and every distinct tag set / `GOOS` / `-race` combination is a separate
cache key. Cross-compiling for `linux/arm64` and `linux/amd64` multiplies it
again. The tag guidance is right; it just silently opts out of the cache
relocation the Makefile arranges.

**Proposed fix:** export `GOCACHE` at the shell level (profile or `.envrc`) so it
covers direct invocations, and state in `ai/rules/bash-output.md` that a direct
`go test` must carry `GOCACHE=$(pwd)/cache/go-cache` alongside the tags. `go
clean -cache` is the safe recovery; nothing but rebuild time is lost.

**Related, same area:** `Makefile:22-24` states the `tmp/go.mod` sentinel is
obsolete because `go list` skips a *symlinked* `tmp/`. That holds only after the
opt-in `make ze-migrate-scratch`. In a checkout where `tmp/` is still a real
directory (`ensure-links.py` reports `SKIP tmp: a real path exists here`), the
sentinel is load-bearing and deleting it lets `go list ./...` walk the caches.

### F17: `bin/ze-test <suite> <N>` is not equivalent to the make target, and fails opaquely

**Friction:** `bin/ze-test-<session> bgp plugin 145` failed with a 30s timeout,
"No messages received, no client output -- server likely failed to start or
crashed". The same test had passed in 857ms in a full `make ze-verify` two hours
earlier. Roughly an hour went into isolating it: the editor, the test runner's
production files and the concurrent session's commits were each reverted to HEAD
and re-tested, and the daemon was run by hand against the extracted config (where
it emitted the awaited line immediately). All of them were innocent.

The mechanism: the functional suites are NOT meant to be launched by running the
runner binary. `mk/test-functional.mk:140` builds ISOLATED, BARE-NAMED binaries
into `$(ZE_ALT_BIN)` -- and the daemon it builds carries the `zetest` build tag
(`ze_core ze_distro ze_setup zetest ...`), which the ordinary `make ze` daemon
does not. `:145` then runs the suite as
`env ZE_TEST_NO_BUILD=1 ZE_BIN=$(ZE_ALT_BIN)/ze ZE_TEST_BIN=$(ZE_ALT_BIN)/ze-test
$(ZE_ALT_BIN)/ze-test ...`. Launched directly, the runner instead rebuilds a ze
WITHOUT `zetest`, and a test needing a zetest-only surface fails in a way that
names none of this. Replicating the make invocation by hand turned the same test
green in 2.0s.

**Pattern:** a harness whose contract lives only in a makefile variable. The
binary accepts the invocation and produces a plausible product failure, so the
diagnosis points at the code under test rather than at the launch. Compare
`ai/rules/bash-output.md`, which already says to prefer `make` because a bare
`go test` drops feature tags and fakes reds -- this is the same failure one layer
out, and the rule does not yet cover it.

**Workaround used:** build the two binaries with the make target's tags into a
scratch dir, symlink them bare-named, and export ZE_BIN/ZE_TEST_BIN.

**Proposed fix:** have the runner detect that it was launched without
ZE_BIN/ZE_TEST_BIN for a suite that needs the isolated pair and say so ("run via
make ze-plugin-test; a directly launched runner builds a ze without the zetest
tag"), rather than letting the daemon start and the assertion time out. The
`--server`/`--client` debug hints it prints on failure inherit the same problem
and would mislead the same way.

### F18: the RFC audit-freshness fingerprint covers the whole file, so untouched tests go stale

**Friction:** `make ze-rfc-check` and `TestRFCRequirementsGate` both failed with
"RFC7606-2-5 has a STALE audit verdict -- the requirement text or a tagged test
changed since it was judged". Neither had. The requirement's own
`requirement_sha` was byte-identical (`a8534d7b2f2b4ae6`), and `git log -p`
showed ZERO changed lines in `TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute` or
its two `RFC requirement:` tags. What changed was an unrelated test in the same
file: `b8f64e345` added a `maxMsgID` argument to `buildReplayRoutes` call sites
elsewhere in `adj_rib_in/rib_test.go`.

The mechanism is documented and deliberate: `tagged_unit_shas`
(`scripts/dev/rfc_requirements.py`) says "Coarse on purpose: the whole enclosing
file, not the function. Over-triggering costs a re-read; under-triggering ships a
verdict for a test that has since changed." The bias is the right one -- a false
"fresh" would ship an unenforced compliance claim.

**Pattern:** a correctly-designed over-trigger whose COST is silent. The failure
text names two possible causes ("the requirement text or a tagged test changed")
and neither had, so the reader must re-derive that a third thing -- a sibling
test in the same file -- is what moved. This is the SECOND time for this exact
requirement: the verdict's own note records the same thing on 2026-07-22, when a
package-qualifier rename in the tagged files triggered it.

**Workaround used:** re-read both halves of the tagged test, confirm it still
fails if the implementation stops complying (the negative feeds
`message.SynthesizeWithdraw` of a malformed-ORIGIN re-announce and asserts
`ribIn[peer].Len()==0`), then re-stamp `rfc/audit/rfc7606.json` with the new file
sha, verdict unchanged.

**Proposed fix:** keep the coarse hash, improve the message. When
`requirement_sha` still matches, the checker already knows the requirement text
is not what moved, and it could say so: "the requirement text is unchanged; a
tagged test's enclosing FILE changed (possibly a sibling test) -- re-read the
tagged test and re-stamp if it still enforces". That is a message change, not a
semantics change, and it would have removed the whole investigation both times.


## Filed 2026-07-22 (plan-review session): two frictions

### F11: `validate-spec.sh` RFC-existence check is dead code (regex typo)

**Friction:** the RFC-summary existence check at `.claude/hooks/validate-spec.sh:199`
greps with the pattern `'\rfc/short/rfc[0-9]+\.md'`. The leading `\r` is a
carriage-return escape in grep -E, so the pattern can never match a literal
`rfc/short/...` path; `RFC_REFS` is always empty and the check silently
approves every spec. Found during the 2026-07-22 plan-folder review:
`plan/spec-ike-post-quantum.md` references `rfc/short/rfc9370.md` and <!-- doc-links: ignore (the finding IS that this path does not exist) -->
`rfc/short/rfc4304.md`, neither of which exists, and the hook said nothing. <!-- doc-links: ignore (same: deliberately nonexistent) -->

**Pattern:** a fail-open guard (`ai/rules/fail-closed-guards.md`): the check
that cannot fire looks identical to the check that found nothing.

**Proposed fix:** change the pattern to `'rfc/short/rfc[0-9]+\.md'` (drop the
stray backslash). Before enabling, sweep existing specs for references to
missing summaries (at least ike-post-quantum) so the newly-live check does not
block unrelated edits; treat missing summaries as a warning or fix them first.

### F12: `spec-closure-check.py` high-confidence signal misfires on slice-scoped learned summaries

**Friction:** the closure advisory (surfaced by `make ze-spec-status`) listed
four specs as "Completed but not closed -- high confidence" because a committed
`plan/learned/NNN-<exact-spec-slug>.md` exists while the spec is in-progress.
For three of the four (`fixit-bgp-session-fsm-lifecycle` learned 1202,
`fixit-firewall-concurrency-deadlock` learned 1182,
`fixit-mgmt-listener-auth-guard` learned 1200) the learned summary is
SLICE-scoped -- each one says so explicitly ("covers only the fsm slice...
Parked. Not committed.", "registry slice only, D-2/D-3/D-4 deferred to sibling
agents", "narrowed to AC-1..4 + AC-7") -- and the specs are correctly still
open. Only `relocate-scratch-and-cache` (learned 1173) was genuinely
closure-ready. A session trusting the "high confidence" label would have
closed three specs with live AC work outstanding.

**Pattern:** exact-slug match is treated as proof the summary covers the whole
spec; parallel-session file-contention workflows now legitimately produce
partial, slug-named learned summaries.

**Proposed fix:** in `scripts/dev/spec-closure-check.py`, demote an
exact-slug match to NEEDS VERIFICATION when the learned summary body contains
partial-scope markers (e.g. "slice", "parked", "not committed", "deferred to",
"scope was narrowed"), or require the spec's own Pre-Commit Verification
section to be filled before claiming high confidence.

---

## `require-related-refs.sh`

**Trigger.** `Edit` or `Write` on a `.go` file whose post-edit content
still contains a `// Related:` / `// Detail:` / `// Overview:` comment
pointing at a sibling file that does not exist on disk.

**Blocks.** Writes that forward-reference a not-yet-created file.

**Workaround.** Create referenced files in dependency order BEFORE
writing the referring file. If the ref has to be added to an
existing file, write the file without the ref first, create the
target, then `Edit` in the ref.

**Not a false positive: stale-ref removal.** Earlier versions of the
hook concatenated old + new content, so removing a stale ref via
`Edit` still tripped the grep. The hook was rewritten to simulate
the post-edit state (`content.replace(old, new)` in Python), so a
straightforward `Edit` from "ref line" to "" now clears the stale
ref without any workaround. The `sed`/`python` escape hatch is no
longer needed.

**Evidence.** 400, 462, 476, 555, 619, 620, 631, 633 (pre-fix). The
forward-ref block still fires by design.

---

## `block-test-deletion.sh`

**Now `c_test_weakening` in `.claude/hooks/pretool-writeedit.py`.**

**Trigger.** `Edit`, `Write` or `MultiEdit` on a test file whose
non-comment non-empty line count decreases (plus the assertion-removal,
`t.Skip`, `require`->`assert` and `ignore`-build-tag checks).

**Blocks.** Any line-count reduction on a `.ci` file, including
removing redundant fixture content or debug prints — **and a
strengthening that happens to be shorter than what it replaces.**

**Workaround (verified in 7 specs).** One of:
1. Keep the line count equal. Wrapping a long call across two lines is
   ordinary formatting and is enough:
   `runtime_fail(\n    f'...')` in place of `print(...)` + `sys.exit(1)`.
2. Add a substitute line of equivalent weight to preserve the count
   (e.g. a comment that documents what was removed).
3. `// test-relax: <reason>` when the change genuinely IS a relaxation.
   Do **not** reach for it to get a strengthening past the line count:
   the escape hatch is what a reviewer greps, so a false one is worse
   than the block.

**Do NOT use `Write` to route around it.** The catalog said so for a
long time and it is now wrong: the check runs on `Write` and
`MultiEdit` as well as `Edit` (`ai/rules/hook-mapping.md`). A session
following the old advice loses the time twice.

**Never.** Do not attempt to collapse 4 lines to 1 — the hook will
reject it as a 3-line deletion.

**The shape that recurs.** Replacing the banned observer-exit
antipattern (`print('FAIL: ...'); sys.exit(1)`, two lines) with
`runtime_fail('...')` (one line) is a strict improvement — the old form
cannot reach the runner at all — and the hook reads four such
replacements as a 4-line deletion. 1290 hit exactly this while fixing a
test that had been silently red on every run since it was written.

**Evidence.** 545, 550, 558, 559, 560, 622, 1290.

---

<!-- block-legacy-log.sh moved to Retired on 2026-04-19. -->

---

## `block-ignored-errors.sh`

**Trigger.** Regex matching `_\s*=\s*\w+\.Close\(\)` or
`_,\s*_\s*=\s*\w+\.\w+\(...\)`.

**Blocks.** Ignored errors on `Close()`, `Write()`, `fmt.Fprintf()`.

**Workaround (verified in 4 specs).** One of:
1. In tests: use a `closeOrLog(t, c)` helper.
2. In production: `errors.Join` to aggregate an error with primary
   return.
3. `//nolint:errcheck // <one-sentence-rationale>` with a specific
   reason (not a generic comment).

**Evidence.** 259, 288, 555, 599.

---

## `block-temp-debug.sh` (now `c_temp_debug` in `pretool-writeedit.py`)

**Trigger.** A debug-MARKER print (`DEBUG`/`TRACE`/`>>>`/`<<<`/`***`/`XXX`/
`FIXME`), a bare `println(...)`, or a short bare `fmt.Println("...")`, in a
`.go` file that is not `_test.go`, not under `cmd/` or `/scripts/`, and not
`register.go`.

**Blocks.** Diagnostic prints in production files.

**Does NOT block (changed 2026-07-16).** Plain writes to `os.Stderr`. The check
used to carry a blanket `fmt\.Fprint.*os\.Stderr` rule; it was measured against
the tree and removed. It flagged **1118 committed stderr writes across 123
files**, of which **1117 were legitimate** (usage text, interactive password
prompts, `error:` messages) and the single marker hit was itself a false
positive: a diff header, `fmt.Fprintf(os.Stderr, "--- %s (original)\n", path)`
in `internal/component/config/cli/cmd_fmt.go`, matched only because `---` was in
the marker list (`---` was dropped too). Precision was zero. Writing to stderr is
what a CLI *does*; it is not evidence of a debug statement.

**The old workarounds were WRONG for CLI code -- do not apply them.** This entry
previously advised (1) use `slogutil.Logger(...).Warn(...)`, or (2) move the
print into `register.go`. Both damage a CLI: a logger sends operator-facing
usage and error text to the logs instead of the terminal, and usage strings do
not belong in a registration file. They remain reasonable for a genuine
diagnostic print inside a library. If you are writing operator-facing output
from a CLI package, just write to `os.Stderr` -- that is now allowed, and it is
what the ~200 prints in `internal/component/*/cli/` already do.

**Related trap.** `errcheck` exempts `fmt.Fprint*` only when the writer is a
literal `os.Stdout`/`os.Stderr`. Route the same call through `fs.Output()` (or
any `io.Writer`) and errcheck demands the error be handled, while `_, _ =`
trips `c_ignored_errors`. Before this change those three rules left CLI packages
outside `cmd/` with no legal way to print at all.

**Evidence.** 282, 622, 633; measurement + removal 2026-07-16 (l2tp `--user`
session; `make ze-hook-test` 131/131 golden + 33/33 fixtures still green after).

---

## `block-root-build.sh`

**Trigger.** `go build` without an `-o` flag from the repository root.

**Blocks.** Creating binaries at the repo root.

**Workaround (verified in 3 specs).** One of:
1. Compile check without output: `go vet ./path/...`.
2. Compile check via tests: `go test -run=^$ ./path/...`.
3. Build to a named target: `go build -o bin/<name> ./cmd/<name>`.

**Evidence.** 555, 614, 622.

---

## `block-pipe-tail.sh`

**Trigger.** A `Bash` command containing `| tail` or `| head` applied
to output of `make`, `go`, `golangci-lint`, or `bin/ze-*`.

**Blocks.** Truncating verbose output.

**Workaround (verified in 2 specs).** Redirect to a file and read with
the `Read` tool:

```bash
make ze-verify > tmp/ze-verify.log 2>&1
```

Then use `Read` with `offset` and `limit` to page through the log.

**Why.** Losing a failure line to `| head` means re-running the whole
build.

**Evidence.** 545, 555.

---

## `block-init-register.sh`

**Trigger.** An `init()` function body containing the substring
`Register`.

**Blocks.** Plugins calling registration functions directly from
`init()`.

**Workaround (verified in 2 specs).** Declare a package-level
`var _ = registerFn()` that calls the registration function at
package-init time. The `var _ =` declaration runs at init time but
is not inside an `init()` body, so the substring match does not fire.

```go
// Blocked:
func init() {
    RegisterFamily(...)
}

// Allowed:
var _ = registerFamilyOnce()

func registerFamilyOnce() bool {
    RegisterFamily(...)
    return true
}
```

**Evidence.** 518, 584.

---

## `block-encoding-alloc.sh`

**Trigger.** `append(` or `make([]byte,` in files matching
`update_build*`, `message/pack*`, `reactor_wire*`.

**Blocks.** Heap allocation on wire-encoding hot paths.

**Known false positive.** `append` on a non-byte slice (e.g.
`append(currentBatch, item)` where `currentBatch []MVPNParams`) is
flagged identically.

**Workaround (verified in 2 specs).** For legitimate non-byte append
that the hook flags:
1. Pre-allocate with `make([]T, 0, n)` where `n` is a known bound.
2. Use `slice = slice[:len+1]` to extend.
3. Annotate with `//nolint:prealloc // intentional: bounded by input`
   and an explanation.

**Evidence.** 603, 604.

---

<!-- block-system-tmp.sh moved to Retired (already tightened with token
     boundary checks before this catalog was written). -->

---

## `block-panic-error.sh`

**Trigger.** A `panic(` call in a new or modified `.go` file outside
`_test.go`.

**Blocks.** Runtime bounds assertions in production code.

**Workaround (verified in 1 spec).** Document the caller-obligation
contract in godoc and rely on static capacity invariants (e.g.
compile-time constant sizes):

```go
// WriteTo writes exactly 64 bytes. Caller MUST pass a buffer with
// cap(buf) - off >= 64. Violating this is a programming error that
// produces a runtime slice-bounds panic — no defensive check is
// performed here because the pool size is a compile-time constant.
func (x *Foo) WriteTo(buf []byte, off int) int {
    // ...
}
```

**Evidence.** 555.

---

<!-- block-layering.sh moved to Retired on 2026-04-19. -->

---

## How to submit a missing entry

If you hit a hook false positive not listed above, OR the workaround
for a listed hook changes:

1. Add an entry following the template (Trigger / Blocks / Workaround
   / Evidence).
2. Cite the learned summary number(s) where the pattern appeared.
3. Update the frequency table at the top.

Threshold for listing: the hook must have generated a false positive
in at least one session documented in `plan/learned/`. Do not list
true positives — those are the hook doing its job.

## How to retire an entry

When a hook's regex is fixed so the false positive no longer
occurs:

1. Cite the fix (commit SHA and short description).
2. Move the entry to a `## Retired` section at the bottom with the
   date.

Do not delete retired entries; they document the history of the
hook layer.

---

## Retired

Entries below describe false positives that have been fixed in the
hook itself. Kept for historical context.

### `auto_linter.sh` — retired 2026-04-19

**Former trigger.** Every `Edit` or `Write` on a `.go` file ran
`goimports -w`, which silently removed imports without an identifier
reference in the current file content. A two-Edit sequence "add
import, then add usage" lost the import between Edit 1 and Edit 2,
producing an `undefined` compile error.

**Fix.** The formatter (then `auto_linter.sh`, today the `auto-lint`
check in `.claude/hooks/posttool-writeedit.py`) invokes
`goimports -format-only -w`. `-format-only` groups imports but
neither adds nor removes them. Unused imports are still caught —
by `golangci-lint`, which the same check runs next, so the failure
is now an explicit lint error instead of silent mutation.

### `block-silent-ignore.sh` — retired 2026-04-19

**Former trigger.** Regex `default:\s*$` fired on any `default:` line
with an end-of-line, regardless of whether the body was empty. Every
`switch`/`select` with a real body starting on the next line was
flagged.

**Fix.** The regex is replaced with an `awk` lookahead that only
flags a `default:` followed (after any blanks and comments) by a
closing `}`. `default: return err` and any real body pass cleanly;
the genuine "silent ignore" shape still blocks.

### `check-existing-patterns.sh` — retired 2026-04-19

**Former trigger.** New `.go` file under `internal/` whose first
exported `type` or `func` identifier appeared anywhere under
`internal/`. Package qualification was not considered, so generic
names (`Config`, `State`, `Manager`, `Session`, `Registry`, ...)
collided across every package.

**Fix.** Duplicate grep now runs against the new file's own
package directory only. Real same-package redefinitions (which Go
itself rejects) still block; cross-package types pass. Cleaner,
and it also speeds the hook up from a tree-wide grep to one
directory.

### `block-legacy-log.sh` — retired 2026-04-19

**Former trigger.** The literal substring `"log"` anywhere in the
file content. Fired on `m["log"]`, `json:"log"` struct tags, and
prose.

**Fix.** The import check now anchors to a Go import-line shape:
a line matching `^\s*(import\s+)?(_\s+|<alias>\s+)?"log"\s*$`.
Struct tags, map keys, and comments all pass. The
`log.Print/Fatal/Panic` call-site check is unchanged.

### `block-layering.sh` — retired 2026-04-19

**Former trigger.** Included `for.?compatibility` in the pattern
list. Fired on legitimate comments like "compatibility testing
against the reference implementation".

**Fix.** That pattern was removed; the other patterns were
tightened to require a qualifier (`legacy.?(code|format|shim|...)`
rather than bare `legacy.?support`; `fallback.?to.?(old|legacy|...)`
rather than bare `fallback.?to`). The rule still catches
"backwards compatibility", "hybrid approach", "gradual migration",
"compat layer", and "deprecated but kept".

### `block-system-tmp.sh` — retired earlier

**Former trigger.** Literal `/tmp/` substring match in `Bash`
commands, which collided with `test/tmp/`.

**Fix.** The command pattern now requires a path-token boundary
before `/tmp` (start of line, whitespace, or one of `=`, `'`, `"`,
`$`, `(`, backtick, `:`, `,`). `test/tmp/` and `~/tmp/` no longer
collide.

## Filed 2026-07-29 (spec-mcp2026-2-mrtr): `c_layering` blocks quoting a normative specification sentence

### F-mrtr-1: a MUST-adjacent spec quote cannot be written in a Go comment

**Trigger.** `c_layering` (`.claude/hooks/pretool-writeedit.py:505`) blocks any
non-test `.go` file whose text matches `backwards.?compatib` or
`backward.?compatib`, anywhere, including inside a quotation.

**What it cost.** MCP 2026-07-28 `client/elicitation` says, verbatim: "For
backwards compatibility, an empty capabilities object is equivalent to declaring
support for `form` mode only." That sentence IS the rule the capability gate
implements, and `ai/rules/rfc-compliance.md` asks for the requirement quoted
above the enforcing path. Two separate Write calls were rejected for carrying
it: one in `internal/component/mcp/mrtr.go` (the gate) and one in
`internal/test/cli/cmd_mcp_mrtr.go` (the client's mirror of the same default).

**Worked around, not fixed.** Both sites now paraphrase the lead-in and quote
only the operative clause ("the specification states that an empty capabilities
object is equivalent to declaring support for `form` mode only"). The normative
content survives; the provenance is one step weaker than the rule asks for, and
a future reader cannot grep the spec sentence to find the code.

**Suggested fix.** The pattern exists to catch Ze *maintaining* a compatibility
layer, which is a claim about Ze's own code, not about a cited external
document. Exempting a line that is evidently a quotation would keep the catch
and drop this class of false positive: skip lines whose match sits inside double
quotes, or inside a comment line that also contains `MUST`, `SHOULD`, `MAY`, or
a `://` URL. The same shape already fixed `block-legacy-log.sh` (anchor to the
Go construct rather than the substring) and the retired `block-layering.sh`
`for.?compatibility` pattern, so there is precedent for both the problem and the
remedy.
