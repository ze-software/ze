# Learned: a ledger row cannot be closed by a fact nobody can recompute

The verification-debt ledger could not reach zero. `refusePushWithDebt` refuses a
push while any row is open, and `clearDebtWith` diverted two gate names into an
unrunnable set before a runner was chosen, so no branch in the repository could
write `cleared` for a row naming `independent critical review` or `owner approval
for an RFC-tagged test change`. 67 rows were permanent by construction, and the
push gate was shut for ever on rows whose obligation had in most cases already
been met.

The fix was not to widen the row. It was to record HOW the obligation was met and
recompute the verdict on every read.

## Store the input, never the verdict

A discharge record carries the kind and the evidence: a commit, an artifact path,
an owner's sentence. It does not carry "verified". Every read re-runs the
derivation from the recorded input, so a record whose evidence has gone stale
returns its row to `open` on its own, with nobody watching.

That is what makes the record safe to commit. A reader who trusts a stored
verdict trusts whoever last edited the file, and a ledger that gates a push is
exactly the file worth editing. The cost is one git read per commit-bearing row,
which is nothing against a population bounded by the rows a machine cannot run.

## The evidence was in git the whole time

The debt row carries no SHA, and the obvious repair is to add one. It is the
wrong repair: 3526 rows already exist without it, and adding a cell puts every
one of them behind a dual-format parser.

The row does not need to carry its commit, because it already rides in it.
`recordDebt` runs inside `Create`, which then appends the debt shard to the
commit's own paths, so **the commit that introduced a row is the commit the row
describes**. `git log -S<reason> -- <shard>` names it. Measured over the whole
population: a subject search leaves 3 of 67 rows unresolved and 1 ambiguous; the
introducing commit resolves all 67.

The general shape: before adding a field to carry a fact, ask what already
witnesses it. A write that lands inside a commit is witnessed by that commit.

## Two guards that only looked like guards

An independent pass found both, and both had the same shape: a check that runs on
one path out of four.

**The gate the row names was read by one kind of four.** Only
`verifyKindNotApplicable` called `debtGateAt`. `kind owner` returned on a
non-empty string alone. So one sentence discharged any of 1039 open rows,
including the 274 `discovery-index freshness` rows `debt-clear` genuinely runs.
The repair was not to add the check three more times. It was to hoist it above
the switch, where a fifth kind cannot be written without it.

**A row covering N commits discharged on evidence about one.** `(+N more)` says a
row was extended, `commitCoversRow` stripped the suffix and paired on the first
subject, and the other commits were never derived. Five discharges in the live
pass rested on half their evidence, and the guard that later caught them caught
them unprompted, the moment it landed.

**Put a check where the next author cannot route around it, not where the current
bug is.** Both repairs moved the check up rather than copying it sideways.

## `(+N more)` counts rows, not commits

The obvious predicate for the second fix, "require N+1 distinct commits", is
wrong on real data. The dedup at `de31341fd7` merges by `(shard, gate, reason)`,
and `recordDebt` runs before the commit script, so a `create` re-run with a
reworded subject leaves a duplicate row that later merges. Two rows in the
population say `(+1 more)` over ONE commit.

A predicate derived from the artifact's shape is a guess about how the artifact
was produced. Read the producer before writing the rule: `de31341fd7^` holds both
pre-dedup rows and settles it in one command.

## A permanent false alarm is a broken guard

`INVALID DISCHARGE` is the whole tamper signal: it is how a hand-edited or stale
record fails to un-refuse a push. After the first pass, `debt-status` printed
five of them permanently, every one benign — three superseded by a later correct
record, one a row discharged twice.

Five false positives on the line that catches a real tamper is the same defect as
not printing it at all, because a reader learns to skim it. It was fixed at the
writer (a record replaces the record its row holds) rather than at the reader,
because a reader-side suppression leaves the stale bytes on disk for every future
reader to judge again.

## An attestation must be visibly an attestation

Nine of the 67 rows are closed on the owner's word, because no machine can check
that he ordered a commit or reviewed it himself. The ledger marks them `owner`
and `debt-status` splits the discharged count by kind, so the ratio of attested
to derived is one command away. The row's own reason cell was NOT accepted as his
authorisation: the author of the commit wrote that cell, and one of them says so
outright, that an owner-approval row an author wrote for himself is a forgery.

Related: `ai/rules/principles.md` (fail closed, declare once, derive),
`ai/rules/evidence.md` (read the producer), `plan/journal/`
`record-written-before-the-operation-succeeds.md`.
