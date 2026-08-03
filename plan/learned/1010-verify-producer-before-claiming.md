# verify-producer-before-claiming

A behavioral claim about code is not verified until you read the function that
PRODUCES the behavior. Reading a value's consumer and inferring its producer is
inference, not evidence. This commit hardens `ai/rules/evidence.md` to
cover behavioral claims and recommendations (not just static file-content
questions), and adds three enforcement layers so the discipline is in front of
the agent at session start, on every turn, and at spec-write time.

## The incident

Asked whether Ze's BGP reconnect loop amplified session flaps, I read `run()`
in `internal/component/bgp/reactor/peer_run.go` (the error *consumer*) and
asserted that the `err == nil` branch resets the backoff and amplifies flapping
peers. I recommended a spec for it. The premise was never checked. Reading the
*producer*, `Session.Run` in `internal/component/bgp/reactor/session.go`,
shows it has no `return nil`: every exit is a non-nil error
(`ErrConnectionClosed` at `session.go`, the stored `closeReason`, or a
read/parse error). So `runOnce`/`safeRunOnce` effectively never return nil, the
`err == nil` branch (`peer_run.go`) is dead, and a real flap takes the
normal-error path (`peer_run.go`) which already sleeps `delay` and backs
off. The claimed gap did not exist. One `Read` of the producer would have shown
it before any spec work.

## The compounding meta-mistakes (the real lesson)

The first error was bad enough; the corrections repeated it at higher levels:

- **Diagnosed the rule without reading it.** I had never read
  `evidence.md` before the claim, then edited it asserting it "was scoped
  too narrowly to catch me." The original already banned "Presenting
  interpretation as fact" for factual questions about code — it substantially
  covered the case. The fix-claim was itself unverified.
- **Almost duplicated an existing check.** Building the enforcement gate, I
  nearly added a new hook. Reading first found `c_design_without_lsp` in
  `.claude/hooks/pretool-writeedit.py` already gated spec writes on recent
  LSP use. The genuine gap was narrower: it counted only LSP invocations, so
  investigating via the `Read` tool (which is how the producer gets read) did
  not satisfy it.

Pattern: a coherent, internally-consistent story was repeatedly trusted in place
of reading the one artifact the claim depended on (the producing function, the
existing rule text, the existing check).

## What was done

- `ai/rules/evidence.md`: extended to behavioral claims and
  recommendations; added the keystone-fact mechanical check (name the single
  fact the claim depends on, read its producer, label hypotheses); recorded the
  incident.
- `.claude/hooks/pretool-writeedit.py` `c_design_without_lsp`: now accepts a
  `.source-read` marker as well as `.lsp-invoked`, so reading a `.go` under
  `internal/`/`pkg/`/`cmd/` satisfies the gate; block message teaches the
  producer-not-caller rule.
- `.claude/hooks/mark-source-read.sh`: new PostToolUse-on-`Read` hook writing
  `.source-read` for implementation source.
- `ai/INSTRUCTIONS.md`: standing output-contract ("Verify before you claim") +
  dispatch-table row, regenerated into `CLAUDE.md`/`AGENTS.md`.
- `.claude/hooks/verify-claim-reminder.sh`: per-turn `UserPromptSubmit` nudge to
  **stdout** (lands in model context, unlike `compaction-reminder.sh` which uses
  stderr by design).

## Traps for the next agent

- **Cite the producer.** Before claiming what code does, or recommending work on
  it, cite the producing function as `file:line`. If you only read the caller,
  it is a hypothesis, not a finding.
- **Verify before adding a rule or check.** Read the existing rule/check that
  would govern the case before concluding it is missing or too narrow. Three of
  the four errors here were "diagnosed without reading."
- **Injection is weak; gates and output-contracts are stronger but not
  guarantees.** A "go read the rules" banner at session start was present and
  ignored. The residual enforcement is human-in-the-loop: the user asking "cite
  the producer" at the moment of the claim is what actually caught this.
- The enforcement gate validates a *ritual* (some producing source was read),
  not *comprehension* (the right function was read and understood). It cannot
  tell whether the code you read is the code your claim depends on.

## Verification

Hook-dispatcher parity green (`python3 scripts/dev/hook-parity-check.py`:
131/131). Gate behavior probed with a pinned session id: no investigation
blocks, a producer `.go` read allows, a docs read does not, a stale marker
blocks. `make ze-ai-check` reports generated agent files in sync. `ze-verify`
does not apply (no `.go`/build/codegen changes).

## Files

None recorded.
