---
kind: directive
level: MUST NOT
stage:
---
`scripts/dev/commit_helper.py` is invoked from Bash, so `--subject`, `--body`
and every override reason are shell words BEFORE Python sees them. Inside a
double-quoted argument a backtick opens command substitution, so a body reading
``the block declares `encoder json` `` runs `encoder json`, prints
`encoder: command not found` to stderr, and substitutes the EMPTY STRING into
the message. You MUST NOT put a backtick in any commit_helper argument.

The failure is silent in the only place that matters. The helper still writes
its message file and still prints `script=`, so a caller reading the tail of the
output sees success; the sentence is already mutilated, and running the script
commits it to permanent history. Quote code in a commit message with plain
double quotes, or name the thing without quoting it at all.

Two habits make it self-checking, and both are cheap next to a bad commit
message that cannot be corrected without rewriting history:

- Read the generated message file before running the script. `create` prints its
  path on the `message=` line. A blanked backtick span is obvious there and
  nowhere else.
- Treat `command not found` anywhere in the helper's output as a failed
  invocation, not as noise from an unrelated tool.

Repairing the message file in place is the fix, and it is allowed: the script
runs `git commit -F <message-file>`, so the file is read when the script runs,
not when `create` wrote it.
