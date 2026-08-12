# plan/future/ -- Work That Does Not Block the First Release

A spec in this directory is real work Ze intends to do. It is not a defect in
the shipped product, so it does not hold the first release.

## What goes here

| Kind | Example |
|------|---------|
| A new feature you find when you fix a defect | a protocol option no operator has asked for yet |
| An architecture change that alters no behavior | moving a special case into a filter chain |
| A cleanup with no wire-visible or operator-visible effect | unexporting package-private symbols |

## What does NOT go here

A defect goes to `plan/`, never here. A defect is any of these:

- Ze puts bytes on the wire that an RFC forbids, or omits bytes an RFC requires.
- Ze accepts an operator's configuration and then does something else.
- Ze exposes a management surface without the authentication the operator asked for.
- Ze loses, duplicates, or reorders a route.
- Ze leaks a resource, deadlocks, or stops answering.

**Moving a defect here to reduce the count in `plan/` is banned.** The count is a
measure of the release, and a defect that moves still ships. `ai/rules/completion.md`
governs this: recording a problem is never addressing it.

## Lifecycle

A spec here carries the same header table and the same statuses as `plan/`
(`plan/README.md`). It moves back into `plan/` when the owner schedules it. It
closes by the same two-commit rule.
