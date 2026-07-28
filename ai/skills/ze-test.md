---
name: ze-test
description: Write or Change a Functional Test
---

# Write or Change a Functional Test

Use this whenever you are about to create a `.ci` test, or change an existing
one. It is the repo's modus operandi for test authoring, and step 1 is not
optional.

See also: `/ze-debug` (a test is failing and you do not know why), `/ze-verify`
(the gate, after promotion)

## Step 1 — Draft it where nothing can see it (BLOCKING)

**Never write or iterate on a `.ci` inside `test/<suite>/`.** That directory is
live: every `make ze-verify` in the checkout runs it, including runs by other
sessions working on unrelated things, who then have to work out whether your
half-written test is their regression.

Write it here instead:

```
test/draft/<suite>/<name>.ci
```

The suite name is the real suite's directory name (`plugin`, `encode`, `reload`,
`firewall`, `policy`, `ospf`, `web`, `vpp`, ...). The directory is gitignored and
skipped by every repo-wide gate, so a draft cannot redden anything for anyone.
Full contract: `test/draft/README.md`.

Changing an EXISTING test is the same move: copy it to `test/draft/<suite>/`,
work there, then `mv` it back over the original. Do not edit the live file in
place and hope verify does not run.

## Step 2 — Run it as a draft

```
ze-test bgp plugin --draft -a
ze-test bgp plugin --draft --pattern <name>
ze-test <suite> --draft -a          # every suite takes --draft
```

`--draft` swaps the discovery root from `test/<suite>` to
`test/draft/<suite>` (`internal/test/runner/draft_dir.go` `SuiteDir`). Without
the flag you get the real tests, never the drafts.

## Step 3 — Assert on STATE, never on elapsed time

This is where nearly every flaky test in this repo came from. A functional test
runs alongside ~20 daemons on a loaded machine, so anything that "usually
happens fast enough" eventually does not.

| Banned | Use instead |
|--------|-------------|
| `time.sleep(N)` to let something settle | `api.dispatch_until(cmd, predicate)` on the state you actually need |
| Shutting down when your own assertion passed | wait for the state your PEERS assert on, then shut down |
| `api.quiesce()` as the only barrier | quiesce drains the forward pool; it says nothing about a peer that was not yet an established target |
| A peer that closes as soon as its script completes | `option=linger:value=true` when another peer's assertion is still in flight |

Three barriers that already exist, in `test/scripts/ze_api.py`:

- `api.wait_peer_eor_sent()` — blocks until ze has flushed its initial-sync
  End-of-RIB to the wire. **Required before `request shutdown` in any test whose
  ze-peer asserts the EOR frame** (`expect=bgp:...00170200000000`).
- `api.dispatch_until(cmd, predicate, attempts=N)` — poll a real command until a
  payload predicate holds. Returns the LAST result when attempts run out, so the
  caller MUST re-check the predicate and fail explicitly. `if total < 0` on a
  count that the poll waited for `>= 1` is a guard that passes on timeout.
- `api.quiesce()` — one barrier RPC, drains the forward pool and each peer's
  initial-sync queue. Cheap. Put it BEFORE a poll so the poll is a safety net
  rather than the mechanism.

**Budget the polls.** Every `dispatch_until` attempt is a full engine RPC. A
60-attempt poll on a starved daemon outlasts the test's own timeout and turns a
run whose wire assertions already passed into an opaque timeout. Keep the poll
well inside the test budget.

## Step 4 — Never print an OK you did not verify

An unconditional `print('OK: ...')` in an observer is a lie that survives in the
log and misleads the next reader. Print what you actually checked, and
`runtime_fail(...)` on the path where the check did not hold
(`ai/rules/fail-closed-guards.md`).

## Step 5 — Take a port from the runner, never a literal

`$PORT` and `$PORT2` expand in exec strings, env knobs, config bodies AND tmpfs
file bodies. A hardcoded port sits outside the per-test range the runner hands
out, so a sibling whose shifted range covers it collides, and the failure reads
as something else entirely.

If the resource is a port an RFC FIXES (BFD 3784/3785), no unique address can
partition it: declare `option=exclusive:group=<name>` so the tests that share it
never run concurrently, and register the cluster in
`TestContendingFunctionalTestsDeclareExclusiveGroup`.

## Step 6 — Prove it under load, still as a draft

```
python3 scripts/dev/stress-repro.py "<suite> --draft" --test <id> --any-failure --iterations 80
```

Passing once on an idle machine proves very little. `--any-failure` is required
for assertion flakes: without it, only a crash counts as a reproduction and the
evidence is thrown away.

## Step 7 — Promote

```
mv test/draft/<suite>/<name>.ci test/<suite>/<name>.ci
ze-test <suite> -a
```

A plain move: the draft is untracked, so the destination just appears as a new
file. No `git mv`, no staging.

Promotion is when the gates start applying — accept-only annotation, the
`time.sleep(` ratchet, frame-length validation, the dispatch-command check. So
promote early and iterate against them, rather than polishing for days against
none of them.

## Rules

- **Draft first (BLOCKING).** A `.ci` under development belongs in
  `test/draft/<suite>/`. Editing a live test in place is how another session
  inherits your red.
- **A red test means the code is wrong by default** (`ai/rules/no-test-deletion.md`).
  Do not weaken an assertion to reach green. If the test is genuinely asking for
  something the product does not define (an ordering RFC 4271 leaves free, an
  attribute order, a scheduling interleaving), say so explicitly in the file and
  replace the assertion with the deterministic barrier that makes the real
  subject assertable.
- **Diagnosis before fix** (`ai/rules/diagnosis-before-fix.md`) applies to test
  changes too: symptom, root cause at a cited `file:line`, owning layer, a
  `[workaround]` and a `[source]` candidate, and why the workaround is wrong.
  "The test is flaky" is a symptom, not a diagnosis.
- Every `time.sleep(` in a changed `.ci` must carry a comment justifying it, and
  the repo-wide count is ratcheted (`test/.ci-sleep-baseline`). Prefer deleting
  the sleep.
