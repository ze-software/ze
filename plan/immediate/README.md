# plan/immediate/ -- Defects an Operator Meets

A spec here describes something the shipped product gets WRONG today. An operator
on the first release reaches it, so the release carries the defect until this spec
closes.

## What goes here

A spec passes this test when a first-release operator meets it as a bug or as a
missing answer:

- Ze puts bytes on the wire that an RFC forbids, or omits bytes an RFC requires.
- Ze accepts an operator's configuration and then does something else.
- Ze exposes a management surface without the authentication the operator asked for.
- Ze loses, duplicates, reorders, or mis-selects a route.
- Ze leaks a resource, deadlocks, panics, or stops answering.
- A CLI command gives a wrong answer, or refuses to answer at all.

## What does NOT go here

Work that costs the release nothing goes to `plan/`. Work that no operator meets,
but that the release cannot go out without, goes to `plan/pre-release/`.

**Moving a spec out of this directory to shrink its count is banned.** The count
measures the release. `ai/rules/completion.md` governs it.

## Lifecycle

Same header table, same statuses, and the same two-commit closure as `plan/`
(`plan/README.md`). A spec here at `skeleton` is a defect with no design, which is
worse than an ordinary skeleton, not better.
