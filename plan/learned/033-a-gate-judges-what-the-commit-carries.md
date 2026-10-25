# Learned: a gate judges what the commit carries, and a review pin is never printed

`spec-commit-hunk-selection` added `hunks <path> <patch>` to `./le commit
create`, so a session can commit its own hunks of a file that also holds
another session's, without touching the working-tree file.

## What changed

- `internal/le/commit/hunks` (`Apply`, `Selection`) applies the patch to HEAD's
  blob in a scratch index and refuses a patch that does not apply, names
  another path, deletes, is binary, or names a quoted path. The generated
  script commits that blob and the shared-index repair writes HEAD's entry
  back, so the other session's hunks show as unstaged changes.
- Every create-time content gate reads a hunks path through
  `Selection.Read`: the weakening gate and its prospective index, the
  RFC-tagged change gate, the test-coverage gate, the journal shape check and
  the owned-row guard, the closure sides, and the review pin. A malformed
  selection entry is an error at each, never a fall back to the working tree.
- `./le spec review record ... hunks <path> <patch>` pins the selected content,
  so the pin and the gate hash the same bytes.
- `docs/contributing/committing.md`, "Committing some hunks of a shared file",
  carries the keyword, the gates that read the selected blob and the three
  judgements that do not (verification state, structural gate, drift note).

## What outlives the spec

1. **A gate that reads the working tree judges a commit it is not making.**
   Round 1 found the journal readers still reading the file directly after the
   closure sides had been threaded with the selection. Every reader a gate
   calls needs the selection, not only the gate's own entry; the fix was a
   `specjournal.Reader` parameter, and each reader got a test that drives it
   with a hunks path.
2. **A generated command that re-pins a review is a way around the review.**
   Three rounds patched a printed re-pin command, and each version let a
   paste vouch for content no reviewer had read. Round 4 deleted it. A stale
   or unreviewed pin has one fix, a new review, so the refusal names the
   record verb only as a usage shape with the reviewer's words left as `...`.
3. **A record replaces the artifact.** `recordReview` writes it whole, so a
   re-record after a refusal names every code file of the commit, not only the
   paths the refusal named.
