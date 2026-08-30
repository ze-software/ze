---
kind: directive
level: MUST
stage:
---
- **Between the patch and the run, you MUST verify the MUTATION APPLIED, with a diff that comes back non-empty or a grep for the mutated text.** A patch that fails to apply leaves the test running against unmodified source, so it passes, and the artifact of that attempt is byte-identical to a successful proof. It is the worse half of the trap: a stale cached verdict at least ran once against real code.
- **Restore by copying back a pristine copy saved first; `git checkout --`, `git restore` and `git stash` are banned outright** and would discard another session's uncommitted work in the same file.
