---
kind: directive
level: MUST NOT
stage:
---
**Each shortcut below MUST NOT be taken.**

| Banned | Why |
|--------|-----|
| "I'll close it later" | Later never comes. Other sessions see it as in-progress. |
| `git rm` a spec while a deferral row still names it as Destination | The row dangles forever. Nothing blocks: the gate is advisory, so it is reported and ignored. Resolve it in commit A. |
| `git rm` a deferral shard that still holds a live row | Deletes the record AND silences every observer of it, because they all fold over the directory. No gate refuses it. |
| Resolving a row to a record that never mentions the item | Fail-open bookkeeping: the row goes quiet and the knowledge is lost. Verify the record carries it. |
| "The user will handle it" | The user asked us to implement. Closure is part of implementation. |
| `git rm` in the same commit as implementation | Spec edits are lost from history. Two commits required. |
| `git rm -f` without a prior commit of the spec | Destroys uncommitted design work. |
| "Run the commit, then I'll prepare closure" | The user will not ask. One script, one run, done. |
