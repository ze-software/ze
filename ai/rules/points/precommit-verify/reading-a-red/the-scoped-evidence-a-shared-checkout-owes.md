---
kind: directive
level: MUST
stage:
---
1. You MUST run the gate the commit owes: `make ze-precommit-verify` when it carries Go,
   and the narrow gate owning each changed surface (the table below) when it does not.
2. You MUST ATTRIBUTE every red you saw, by the table above: name the file, and say
   whose it is. `git status --porcelain` plus a modification time settles it in seconds.
3. You MUST prepare the script and let the helper judge your own paths first: `create` scopes
   the freshness question to your `--file` list, so an edit outside it changes nothing.
   Since 2026-08-21 a stale record does NOT refuse the commit: it records a
   verification-debt row and proceeds, and `--push` is what refuses while a row is
   open (`ai/rules/git-safety.md`, "Verify a Commit, Not the Working Tree"). You
   MUST still pass `--unverified "<attribution>"`, because that reason is the Reason
   cell of the row: give the gates you ran and their verdicts, and name the paths
   whose reds you attributed away. A row with no attribution leaves the next reader
   with a debt nobody can judge. The one red that still REFUSES the commit is a
   structural gate charged to it, and `--unverified` never cleared that.
