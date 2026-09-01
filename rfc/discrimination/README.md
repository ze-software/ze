# Discrimination records

One file per RFC, `rfc/discrimination/<stem>.json`. Written by
`./le rfc discriminate`, re-verified by `./le rfc check` on every run.

Spec: `plan/spec-rfc-tag-claim-discrimination.md`. Page:
`docs/contributing/rfc-conformance-gates.md`, "The discrimination record".

## What this artifact is

An RFC tag is `RFC requirement: <ID> <polarity>` followed by prose stating what
the test demonstrates. `parseTagRest` (`internal/le/rfc/tags.go`) reads the
structured half. No gate reads the prose, because it is a sentence. A tag can
therefore advertise an assertion its body never makes, and every other gate in
`internal/le/rfc/` stays green.

A record says that a named tagged unit was OBSERVED to fail under a named break
of the code the claim rests on. "The prose is true" is unfalsifiable by a
machine. "This break makes this unit red" is decidable and replayable.

## What this artifact is NOT

**It is not a judgement that the break is a GOOD break.** The gate judges that a
break was recorded, that it lies in a producer the tree still holds, and that
the tagged unit it reddened is still the unit the record names. Whether the
break engages the claim's subject is a reading, and `break` is stored so a
reviewer can make it.

**It is not a re-run.** `./le rfc check` is the third stage of every full
verify, and one interop scenario alone costs 21 to 150 seconds warm. The gate
compares the fingerprints the proof was taken against, which is what
`checkAuditFreshness` already does for an audit verdict. A verified record says
the red WAS observed, and that the code it was observed over has not moved
since. It does not say the red would happen again on a machine that never ran
it.

**It is not a bound over the standing corpus.** 3,923 tags are in scope today
and almost none carry a record. The obligation is change-scoped and keyed on the
tagged UNIT: a unit the TIP COMMIT added against `HEAD^`, on a gated
requirement of an enrolled RFC, owes its record in that commit. Both sides are
COMMITTED (owner decision, 2026-09-01), so a tag sitting only in somebody's
working tree bills nobody, and the author meets the violation where
`./le verify worktree` checks their own commit out detached. What is unproven is
published as `owed`, and what the unpushed commits added is MEASURED against
`origin/main` on a line of its own that changes no exit code, never claimed
away.

## Fields

| Field | Authored or derived | Meaning |
|---|---|---|
| `rfc` | authored, cross-checked | must equal the filename stem |
| `rid` | authored | the requirement the record proves; must be declared in `rfc/short/<stem>.md` |
| `polarity` | authored | `positive` or `negative`, the direction it proves |
| `unit` | authored | the tagged unit key: `<path>::<FuncName>`, or a bare `<path>` when the whole file is the unit. `UnitAt` (`internal/le/rfc/goscope.go`) is the one definition of that unit |
| `unit-sha` | derived | the unit's behavior hash when the red was observed |
| `claim-sha` | derived | the hash of what the TAG claims: the comment paragraph its `<ID> <polarity>` opens |
| `route` | authored | `mutant`, `revert`, or `no-break` |
| `producer` | authored | the code the break was applied to, in the same key form. Required for a proof route. An escape names it too, unless its reason is `foreign-producer`, because the reason is a claim ABOUT that code |
| `producer-sha` | derived | that function's behavior hash when the break was applied |
| `break` | derived | what was done to the producer: the mutant's substitution, or the disabled body. No gate parses it; a reviewer reads it |
| `citation` | authored | the assertion a proof, or a `foreign-producer` escape, rests on: a `fail(N, ...)` number an interop checker writes out, a directive line for a `.ci`. Required for a functional or interop proof and for `foreign-producer`, refused for a unit record and for the two escapes that name a producer. An assertion numbered by expression cannot be cited |
| `reason` | authored | why no break exists, for the escape only. One of the closed vocabulary below |

All three fingerprints are minted by `sealDiscrimination`
(`internal/le/rfc/discriminate.go`), the one place a fingerprint is computed. A
hand-written fingerprint is refused for shape, and a hand-written record whose
fingerprints happen to parse is refused the moment one of them stops matching
the tree.

## Why the key is a name and a behavior hash

`rfc/audit/rfc7606.json` records the cost of the alternative. It hashed the
enclosing FILE, so a 9-line inserted header shifted every key and two whole
paragraphs of that artifact exist to describe the mechanical re-stamping.

A record here names the function and hashes what that function DOES.
`behaviorBytes` (`internal/le/rfc/tags.go`) strips comments and whitespace, so a
reworded comment, a reflow, an inserted header and a blank line each leave the
record verified, while a changed assertion or a rewritten producer voids it. It
is also the predicate `ChangedTags` uses to answer "did this unit's behavior
change", so a record goes stale exactly when the obligation says the unit moved.
A line number is refused outright: `fingerprintKey` rejects the retired
`<path>:<line>` form for both keys.

Measured over this checkout's own records: a nine-line header prepended to every
file they name, which is the edit that cost the audit artifact two paragraphs,
leaves all three verified (`TestDiscriminationRealRecordsSurviveAMechanicalRename`).

## Why the claim is hashed apart from the unit

`behaviorBytes` strips comments, and an RFC tag's claim IS a comment. Without a
field of its own, a proof sealed against a modest claim survives that claim
being widened: an author proves what the body checks, then rewrites the sentence
to say more, with no code edit at all, and the gate publishes the wider claim as
proven. That is the over-claim this artifact exists to refuse, in its cheapest
form, and every mechanism watching for a changed ASSERTION misses it.

`claim-sha` hashes the comment PARAGRAPH the tag opens: the words after the
polarity on the tag's own line, plus every comment line under it, up to the next
tag, an empty comment line, or the first line that is not a comment. 2,701 of
this checkout's 3,900 tags carry a claim that runs past the tag's own line, so
hashing one line would leave two thirds of the corpus free to widen. Whitespace
runs collapse to one space, so re-wrapping a sentence changes nothing.

The accepted cost is stated rather than hidden: rewording a claim, a typo fix
included, stales the record and owes a re-record. An unrelated comment elsewhere
in the unit does not, which is the pair that makes it a property rather than a
rule that fires on every edit.

## The routes (closed set)

| Route | Means | Counted as |
|---|---|---|
| `mutant` | the break was generated, by `gomu` over the producer | proven |
| `revert` | the producer was disabled by hand, which is the method `docs/contributing/testing.md` already prescribes | proven |
| `no-break` | no expressible break exists | escaped |

`no-break` is DEBT, not evidence, and is counted apart everywhere it is
published. It carries no `producer-sha` and no `break`, because it states there
is nothing to observe. It still names what ties it to THIS claim, and which field
carries that tie depends on the reason.

A `mutant` record is recorded from a gomu report. Only a KILLED mutant inside
code the tagged unit's own coverage profile executes is ever proposed: a
NOT_VIABLE mutant does not compile, a SURVIVED one is noticed by no test in the
package, and a mutant the unit never reaches cannot redden it. `./le rfc
discriminate <selector> report <path>` prints the candidates, best first, where
best is the count of symbols the tag's own prose names that the break's text
touches.

A `revert` record disables one producing FUNCTION, which is the method
`docs/contributing/testing.md` prescribes. The body is replaced by a halt, and
`break` records that. `./le rfc discriminate-record` applies it, runs the tagged
unit, and requires a failure that NAMES that unit.

## The escape's reasons (closed set, each precondition checked)

An escape says no break exists. That is a claim about the tree, and the gate
goes and checks it, in the shape `checkSuperseded` already has for its four
dispositions. An unconditioned reason would make the escape cheaper than a
proof, so the escaped count would climb faster than the proven one.

Each reason's fact is about a FILE or a CARRIER KIND, and neither is about one
tag: a declaration-only file exists in every package, and `interop` is a property
of 37 tags at once. So each reason ALSO carries a tie to the claim it discharges,
and both halves are checked.

| Reason | Claims | The gate CHECKS | The tie to the claim |
|---|---|---|---|
| `foreign-producer` | the behavior is produced by an implementation this repository does not build, so no edit here can falsify the claim | the carrier kind is `interop`, and the record names no producer | the `citation` names a `fail(N, ...)` number the tagged checker writes out |
| `declaration-only` | the code the claim rests on holds no function body: a table, an embed, a registration list | the named producer file declares no function | the tag's own claim names an identifier that file declares |
| `generated-producer` | the producer is generated, so a break is undone by the next generator run | the named producer's file carries the `// Code generated ... DO NOT EDIT.` line | the same: the tag's claim names something that file declares |

Coverage cannot supply the second tie. A declaration-only file carries no
statement, so a coverage profile holds no block for it and can never show the
tagged unit reaching it. The claim is what is left, and `claim-sha` pins its
wording.

Both producer-naming reasons owe a third fact, and it is about the FILE the
record picked. The producer must be code the tagged unit REACHES: for a Go unit,
its own package or a package its file imports; for a `.ci` or an interop
scenario, which runs the whole daemon, any file the Go tool compiles, and
nothing under `testdata/`. Without it the two facts above are properties of a
file rather than of this test, and 605 of the 4,020 claims in the tree carry a
whole word that some function-free file somewhere declares (measured
2026-08-31), so an author who could not prove a claim could go and find the file
that fits the words. A producer naming a function its file does not declare is
refused for the same reason: each fact reads the whole file, so an unresolved
symbol would sit in the record read by nothing.

One refusal comes before the reason is read: a `unit`-carrier tag whose producer
resolves and sits in a file gomu mutates is REFUSED the escape whatever reason it
offers. A break can be generated for it, so "no break exists" is false about it.
It runs on the GATE's own path, so a hand-written record meets it too.

## Refusals

`./le rfc check` refuses, and each refusal reds the gate rather than skipping a
record. A corrupt or unreadable file is refused whole: a record that cannot be
read must never present as a corpus with nothing proven
(`ai/rules/principles.md`).

| Refuses | Why |
|---|---|
| A file that cannot be parsed, an unknown JSON key, or a filename that disagrees with its own `rfc` field | A half-read record is the shape a false proof takes |
| A polarity, a route, a key or a fingerprint outside its closed form | The same |
| A record naming a requirement no summary declares | A proof of an obligation nobody wrote down proves nothing |
| Two records claiming one requirement, polarity and tagged unit | The proven count is published, and a duplicate inflates it |
| A record whose `producer` no longer resolves | The break was applied to code that is gone. Judged against HEAD like the row below it: a producer removed by an edit nobody has committed is reported on a `discrimination:` line rather than refused |
| A record whose `unit-sha`, `claim-sha` or `producer-sha` no longer matches COMMITTED code | Nothing observed the red over the code that was committed, or it was observed about a different sentence. The drift is judged against HEAD (owner decision, 2026-08-31), at the granularity the record fingerprints; one staled by an edit nobody has committed is reported on a `discrimination:` line, counted as proven by nothing, and reds the gate at the commit that carries the edit |
| A tagged unit the TIP COMMIT added against `HEAD^`, on a gated requirement of an enrolled RFC, with no verified record | The obligation is what a CHANGE adds, and a floor that only forbids going below zero proves nothing. A tag nobody has committed is not that change |
| A record committed at HEAD, deleted from the tree, while its tag is still there | The proven set only goes up |
| A functional or interop record citing an assertion its carrier does not contain, or citing none at all | No generated break reaches either carrier, so the citation is what ties the recorded red to one assertion rather than to the whole suite |
| An escape whose reason is outside the closed set, whose precondition no longer holds, that names code the tagged unit does not reach, or that is not tied to the claim it discharges | A reason checked on its own discharges every tag equally, which is a blanket opt-out with a green bar on top |

One state is NOT refused. A record whose TAG is gone -- the test deleted, or its
function renamed -- is REPORTED as removable on a `discrimination:` line of its
own and counted as proven by nothing. A record dies with the tag it proves, so
an orphan has nothing left to be wrong about, and billing the session that
correctly deleted a tag would make deleting one more expensive than leaving it.

## Derived versus authored

Only `rid`, `polarity`, `unit`, `route`, `producer`, `citation` and `reason` are
authored. All three fingerprints are DERIVED, and so is `break` and every
published count. Editing a
fingerprint to make a red go green fails the shape check, and matching one by
hand publishes a red nobody observed, which is the single failure this artifact
exists to prevent.
