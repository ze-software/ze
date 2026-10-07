# Feature maturity

Every public statement of how mature a Ze feature is comes from one declaration
per feature, `features/<id>.md`. `./le feature check` refuses a declaration
whose level its evidence does not support, and `./le feature report` says, for
every feature, how far it is from the next level. The live vocabulary and every
feature's state are what `./le feature report` prints; this page explains the
model and does not copy them.

<!-- source: internal/le/feature/vocabulary.go -- Kind, Scope, Level, StatusLabel -->
<!-- source: internal/le/feature/declaration.go -- Parse, Load -->
<!-- source: internal/le/feature/check.go -- Check -->
<!-- source: internal/le/feature/runrecord.go -- RunRecord -->

## The declaration

A declaration is a Markdown file with a `## Meta` table of `| Field | Value |`
rows and a `## Description` section holding the public row prose. The file stem
is the feature id. Adding a feature is adding one file: the directory listing
is the registration, and no Go changes. A list-valued cell separates its items
with a comma and a space. A Go test item is `path/to/x_test.go::TestName`, the
spelling a discrimination record uses.

A feature has two separate facts:

| Axis | Question |
|------|----------|
| Scope | How much of the stated feature exists: complete, partial, future, rejected |
| Level | How well the implemented scope is proven: supported, experimental, stub-backed |

A partial scope lists its Scope gaps, and its public status carries
"(partial)". A future or rejected feature has no level.

Kind decides what each criterion means: a protocol needs interop against a
third-party implementation, a daemon surface needs a test that drives the
user's command, an umbrella has no evidence of its own and is bounded by the
worst of its Parts.

## The ceiling

The check computes the highest level the evidence supports and refuses a
declared level above it. A declared level below it is reported as a promotion
candidate, never refused: maturity is a release decision bounded above by
evidence.

Supported requires, among the criteria the report names:

| Criterion | Evidence the check reads |
|-----------|--------------------------|
| S1 real path | at least one real-path test, and a recorded green run of each one's present content |
| S3 RFC | each listed enrolled stem published as supported by the RFC ledger, with no gated requirement marked `{gap}`; an unenrolled stem is reported as a bound |
| S4 docs | a Docs page and a Doc review |
| S5 defects | no `plan/immediate/` spec naming a Components path in its Files to Modify, and a Defect review |
| S6 not stub-only | at least one real-path or interop item not listed in Stub evidence |

A path that does not exist, escapes the repository, or names a Go test function
its file does not declare is refused at every level.

## Recorded runs

"The test exists" never reaches Supported. A green run is recorded in
`features/runs/<id>.json`, committed beside the declarations, with the git blob
id of the test file when it passed. The run is current while the file's blob
id is unchanged; editing the test makes the run stale. Only passes are
recorded.

## Attestations

What no check can prove, that the page and the row prose match the producing
code, rests on a dated attestation: `Doc review` and `Defect review` each read
`YYYY-MM-DD: what was judged`. A date with nothing judged is refused.
