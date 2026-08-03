# 1308 -- Stop hook re-registration

## Context

`.claude/hooks/block-premature-stop.sh` left the `Stop` event on 2026-06-29.
`plan/learned/1306-delegation-reminder-position.md` recorded the owner decision of
2026-07-30 to leave it unregistered and correct the ten surfaces that called it live.
On 2026-07-31 Thomas reversed that decision. The trigger was a session that worked
inline instead of delegating, which is the one failure the delegation nudge reports.

## Decisions

- **Register the hook FIRST in the `Stop` array.** Order is necessary and not sufficient. Three of the four gates sit behind the claim marker. An array that released the claim first leaves a live hook with three silent gates.
- **Move the claim release from `Stop` to `SessionEnd`.** `session-end-summary.sh` released it at the end of every Stop event, and a Stop hook fires between every turn. The claim therefore died after turn one. The closure gate suffered worst, because it can only fire after commit A lands.
- **Pin BOTH ends of a lifetime change.** The first fixtures all asserted the claim SURVIVES, so deleting the release line left the suite green. When you move WHEN a thing dies, write one fixture for alive and one for dead.
- **Pin registration with a fixture, over describing it in a rule.** Prose was wrong for a month across ten surfaces. The fixtures read the `Stop` array directly.
- **Exempt markup from the stop-phrase scan, over dropping phrases.** The scan blocked its own first live turn, on a report that quoted a banned phrase in backticks. Backtick spans and fenced blocks are stripped before the loop. Backticks only: stripping quoted spans too would hide real permission-seeking that quotes something.
- **Split the phrase list in two, over deleting the phrases that conflicted.** `.claude/rules/session-start.md` REQUIRES `What next?` once the task is done, and the scan blocked that exact sentence, so two live rules contradicted each other. `PHRASES` always blocks. `COMPLETION_PHRASES` joins the scan only while a claimed spec is in progress, so the state check runs before the scan.
- **Bound the retry flag to the PHRASE SCAN alone, over exiting the hook early.** Exiting on `stop_hook_active` also switched off the spec-closure gate. Tripping any phrase on turn N therefore disarmed the closure gate on turn N+1. The scan needs the bound, because its only escape is rewording. The closure gate needs none, because it has two escapes of its own.
- **Allow a stop on input the hook cannot read.** A jq parse failure under `set -eo pipefail` returned exit 5, a code the header never defines. Malformed input now exits 0.
- **Correct the superseded summaries with dated bullets, over rewriting them.** `1306` and `1289` are historical records and each carries a `**CORRECTION (2026-07-31).**` bullet.

## Consequences

- All four gates run on every turn. The phrase scan and the closure check block. The in-progress and delegation reasons warn. On a retry the scan alone stands down.
- Three of the four gates need a CLAIMED spec. Inline work on an unclaimed spec is still unreported. `delegation-no-spec-no-nudge` pins that boundary deliberately, and widening it is a separate decision.
- Three hooks read a per-session marker path that nothing had written since the marker moved. All three now read `tmp/session/`.
- `plan/handover/01-delegation-reminder-blocked-files.md` must go in the closure commit. Its step 2 instructs a future session to write a statement the reversal makes false.
- **Extending a lifetime can wake a dormant reaper.** A 24h stale-marker sweep runs at every session start. The claim mtime was set once and never rewritten. A session running over a day therefore lost its live claim the moment any other session started. The sweep was unreachable while the claim died on the first Stop. The hook now touches the marker when it reads it. When you make something live longer, re-read every janitor that was unreachable for it before.

## Gotchas

- **A hook on disk with green fixtures can still fire on nothing.** Registration lives in `.claude/settings.json`, and nothing read it for a month. Check the array, never the file's existence.
- **A phrase scanner that reads its own documentation will block it.** Any greppable banned-phrase list needs a markup exemption, or writing about the list becomes impossible.
- **A text filter in front of a gate must fail toward scanning MORE, never less.** The first filter dropped every line after an unclosed fence. A message whose fence stayed open then passed a real request. A gate that disables itself is worse than the false positive it prevents.
- **The invariant a filter DECLARES is the test list.** The comment said "scan MORE, never less", and the inline-span strip broke it. A left-to-right pass let a stray backtick pair with the OPENING tick of a later span, and delete the request between them. The strip now runs only on a line whose backticks balance. Write one case per way to violate a declared invariant.
- **Two consumers of one field must share one pattern.** The hook and `spec-closure-check.py` both read a spec's `| Status |` row. The hook's replacement was stricter about spaces. When you copy a field's pattern into a second reader, copy its tolerance too.
- **Three ways a green fixture proves nothing.** It asserts an exit code a leak also produces, so assert WHICH rule fired. Its setup returns early, so the hook never reaches the site under test. It asserts only that something SURVIVED, with no peer that must die in the same run.
- **A phrase gate blind to task state will contradict any rule that mandates a phrase.** Wording alone cannot tell a required question from premature stopping. Read the state first, then choose the pattern set.
- **Grep the WRITER of a path before you trust any reader of it.** Three readers used a marker path with zero writers, and one function read two different marker locations a few lines apart. All three exit 0 regardless, so nothing went red.
- **A liveness heartbeat uses `touch -c`, never a bare `touch`.** A bare `touch` resurrects a missing claim marker EMPTY, so every gate below it skips in silence. On the spawn marker it invents proof of a delegation that never happened. Mutation-verified: a bare `touch` at either site turns five fixtures red.
- **The spawn marker fails in the WORSE direction.** Losing the claim marker makes gates go quiet. Losing the spawn marker makes the nudge accuse a properly supervising session. TWO reapers delete it at 24h, not one: the named sweep, and an unfiltered `find` over `tmp/session/`.
- **The first use of the janitor lesson still missed two of the three per-session markers.** The spawn marker is fixed here. `tmp/session/.closure-ack-<stem>` is NOT. The same unfiltered `find` deletes it at 24h. The documented escape from the closure gate therefore expires, and the gate resumes blocking with no message. This bullet RECORDS that defect. It does not repair it.
- **`make ze-test-health` reads the whole working tree.** It regenerated two ratchet baselines from a concurrent session's uncommitted tests. Committing them blesses numbers no commit contains. They were excluded, and they belong to whoever lands that work.

## Files

- `.claude/settings.json` -- the `Stop` array, guard first
- `.claude/hooks/block-premature-stop.sh` -- markup filter, four fail-toward-scanning guards, retry flag scoped to the scan
- `.claude/hooks/session-end-summary.sh`, `session-end-scratch.sh` -- the claim release moved to `SessionEnd`
- `.claude/hooks/pre-compact-save.sh`, `pretool-writeedit.py`, `mark-agent-spawned.sh` -- marker paths
- `scripts/dev/hook-fixture-check.py` -- 35 fixtures in the `delegation` section
- `scripts/dev/spec-closure-check.py`, `scripts/dev/spec_closure_check_test.go`
- `ai/rules/repo-maintenance.md`, `ai/rules/planning.md`, `ai/rules/planning.md`, `.claude/hooks/README.md`
- `plan/learned/1306-delegation-reminder-position.md`, `plan/learned/1289-delegation-by-default.md` -- dated corrections
