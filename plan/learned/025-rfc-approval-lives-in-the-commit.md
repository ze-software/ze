# 025 - The owner's approval lives in the commit

**Spec:** spec-rfc-approval-lives-in-the-commit, closed 2026-09-15
**Class:** `plan/journal/check-cannot-see-the-change-it-looks-for.md`

## What the work built

A change to a test carrying an `RFC requirement:` tag needs the owner's
approval. The approval used to be a row in a tracked ledger shard,
`test/rfc-changed/<session>.md`, with a prune, a foreign-shard refusal and a
guide section on who may write a row. The owner's verdict: the refusal is the
value, the ledger is bean counting.

Now the author runs `./le rfc approve unit <package>.<TestName> reason "<the
owner's words>"` once (`rfc.Approve`, `internal/le/rfc/approve.go`). It writes
one row into `tmp/commit-rfc-approved-<session>.md`, which git never sees. The
Write/Edit hook (`proposedApprovals`, `internal/le/testweakened/proposed.go`)
and `./le commit create` (`rfcChangeProblems`, `internal/le/commit/rfcchange.go`)
both read that file and both refuse a change to a tagged unit no row names,
printing the command. `Message` (`input.go`) appends one unwrapped
`RFC-approved: <package>.<TestName>: <reason>` line per used row, and the
generated script drops the used rows after `git commit` succeeds
(`renderApprovalPrune`, `script.go`). The commit that changed the test is the
record. The 16 tracked shards are deleted; `test/weakened/` is untouched.

## Decisions

- The writer lives in package `rfc`, because `testweakened` imports `rfc` and
  the reverse would cycle; both readers parse the file with
  `testweakened.ParseLedger`, so the row grammar is declared once.
- The prune is a script step after `git commit` under `set -e`, so a row leaves
  the file only once git holds its trailer.
- A row whose whole trailer line a commit already carries approves nothing
  further and is dropped by the next successful commit (R-1).
- `rfc-change-ok` stays: it records a debt when the owner has not answered,
  which is a different fact from an approval.

## What the review found

- A body line the author types beginning `RFC-approved:` was wrapped into the
  commit and read as the owner's record. `refuseHandWrittenTrailer` refuses the
  subject and every body line that starts with the key.
- `git log --grep` matches a substring, so a landed trailer whose reason
  extends a new row's reason marked the new row landed. The verdict is now a
  whole message line equal to the trailer, and the refusal names the carrier
  commit.
- The forgery guard first ran over the INPUT line, before the wrap. A key past
  column 72 lands at the start of an emitted line, which is what
  `trailerLanded` reads. The guard now runs inside `wrapBody`, over the lines
  it emits. A guard on a value a later transform rewrites judges the wrong
  value.

## What rode along

The morning's swap commit b57ec4ab6b moved the YANG length cap from
`description` to `ze:help`, and the hook self-check's `yang-description`
probes still wrote an over-cap `description`, which the gate now rightly
allows. The implementer read that red as another session's. It was this
session's: `yangProbeAllow` and `yangProbeRefuse`
(`internal/le/hookcheck/fixtures.go`) now write a `ze:help`.
