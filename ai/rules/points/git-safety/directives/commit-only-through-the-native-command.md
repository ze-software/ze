---
kind: directive
level: MUST
stage:
rationale: ai/rationale/git-safety.md
---
**Every commit MUST go through `./le commit create`, which writes one message file and one commit script; `git commit`, `git add`, `git rm`, `git restore --staged` and `git stash` MUST NOT be invoked as a direct Bash call.** `ai/INSTRUCTIONS.md` carries that ban into every session, and the same verbs inside the generated script are allowed.
**The printed `script=` line is the only authoritative path: MUST copy it, and MUST NOT construct it from the session id.** Read the message file first, name only canonical sources, run the script yourself with `bash`, then report the SHA, the files and the verification evidence. `docs/contributing/committing.md` carries the keywords and the refusals.
