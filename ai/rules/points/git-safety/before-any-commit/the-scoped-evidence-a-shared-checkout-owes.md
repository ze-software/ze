---
kind: directive
level: MUST
stage:
---
1. You MUST run the gate the commit owes: `make ze-precommit-verify` when it carries Go,
   and the narrow gate owning each changed surface (the table below) when it does not.
2. You MUST ATTRIBUTE every red you saw, by the table above: name the file, and say
   whose it is. `git status --porcelain` plus a modification time settles it in seconds.
3. You MUST prepare the script with `--unverified "<attribution>"`, giving the gates you ran
   and their verdicts, and naming the concurrent session's paths whose reds you attributed away.
