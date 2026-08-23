---
kind: directive
level: MUST
stage:
---
**Every `make ze-*-update` target derives its output from the WORKING TREE, so in a shared checkout it picks up other sessions' uncommitted sources. You MUST diff a regenerated artifact before you name it in a commit.** Sixteen such targets exist and not one warns you.

The output is correct for the tree it read. It is wrong for the commit you are about to make, because that commit does not carry the sources the regeneration saw. What lands is a derived file that describes code nobody can see.

Measured on 2026-08-23. `make ze-discovery-index-update` regenerated `ai/PACKAGE-MAP.md` carrying rows for `internal/core/configorder` and `internal/core/configvalue`. Both were another session's uncommitted work. `commit_helper.py` refused it in the right words: regenerated from a tree holding sources this commit does not contain. It cost two attempts, and the gate is the only thing that catches it.

**The safe regeneration is HEAD plus your own files.** When an artifact is fully generated and yours was the only edit, `git show HEAD:<path>` written back over it restores the committed state, and the gate then agrees.

**The mirror image is worse and no gate catches it: committing a document that DESCRIBES uncommitted code.** The same day, a shared doc landed naming a symbol whose function was still in the working tree. It reddened `ze-doc-links-check` for every session until that code arrived. A check that you have not swept somebody's work IN does not check the other direction: prose you committed about work still sitting OUT.
