---
kind: directive
level: MUST
stage:
rationale: plan/journal/concurrent-session-corruption.md
---
**Every `make ze-*-update` target derives its output from the WORKING TREE, so in a shared checkout it picks up other sessions' uncommitted sources. You MUST diff a regenerated artifact before you name it in a commit.** Sixteen such targets exist and not one warns you.

The output is correct for the tree it read. It is wrong for the commit you are about to make, because that commit does not carry the sources the regeneration saw. What lands is a derived file that describes code nobody can see.

`commit_helper.py` refuses a commit whose regenerated artifact was derived from a tree holding sources the commit does not carry. That refusal is the only thing that catches this.

**The safe regeneration is HEAD plus your own files.** When an artifact is fully generated and yours was the only edit, `git show HEAD:<path>` written back over it restores the committed state, and the gate then agrees.

**The mirror image is worse and no gate catches it: committing a document that DESCRIBES uncommitted code.** A committed document that names a symbol still sitting in the working tree reddens `ze-doc-links-check` for every session until that code lands. A check that you have not swept somebody's work IN does not check the other direction: prose you committed about work still sitting OUT.
