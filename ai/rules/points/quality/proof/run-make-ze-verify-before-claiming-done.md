---
kind: note
level:
stage:
---
`make ze-precommit-verify` is the ONLY acceptable verification before claiming done. Run it in the foreground and wait for it to finish. Output auto-captured to `tmp/ze-verify.log`. See `ai/rules/git-safety.md` for the full pre-commit workflow, and its "Running ze-precommit-verify" for why you must not kill it for being slow.
