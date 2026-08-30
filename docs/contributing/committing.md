# Committing

`./le commit create` is the only staging and commit route in this repository.
It writes a message file and an executable script; you run the script. There is
no hand-written fallback.

What you OWE around a commit is `ai/rules/git-safety.md`. This page is how the
command works.

## Why one command owns it

Several sessions share this checkout, so they share one git index. A loose
`git add` followed by a `git commit` therefore carries whatever another session
staged in between. The command bundles the add, the remove, the commit and the
optional push into one script, so no partial-staging window exists.

## The keywords

`./le commit create` takes keywords, not flags. Every one takes a value except
`append` and `replace`.

| Keyword | Repeats | Meaning |
|---------|---------|---------|
| `subject` | no | The one-line commit subject. At most 72 characters, and a longer one is refused with the count and the overage |
| `body` | yes | A body paragraph. Lines are wrapped to 72 characters without breaking a word |
| `file` | yes | One explicit file to stage. Never a directory |
| `remove` | yes | One tracked path to delete |
| `replace` | no | Start a fresh script. Use it for the first commit of a session |
| `append` | no | Add another commit block to a script that already exists |
| `script` | no | The script to append to. `create` with no `script` always gets a distinct path |
| `session` | no | The eight-hex commit namespace. `./le commit session` creates or reuses this harness session's |
| `tag` | no | The block tag inside the script |
| `push` | no | The owner's authorisation text. The script pushes after every commit succeeds |
| `no-test` | no | The reason a commit carries no test evidence |

`Message` in `internal/le/commit/input.go` enforces the subject limit and the
body wrap.

## What the command refuses

`normalizePath` and `validateAddPath` (`internal/le/commit/input.go`) refuse a
path before the script is written:

- A path outside the repository, or a `..` component.
- Anything under `.git/`.
- A generated agent file: `AGENTS.md` and `CLAUDE.md`.
- A path `git check-ignore` matches.
- A path that does not exist. Use `remove` for a tracked deletion.
- A directory. Scripts stage explicit files.

`validateRemovePath` refuses a `remove` path that is not tracked, so you never
have to run `git ls-files --error-unmatch` yourself.

The command also checks verification freshness for the named file population,
records verification debt rather than dropping a local commit, refuses
`push` while any debt row is open, and enforces discovery-index freshness. Run
`./le discovery-index update` when it complains.

## Worked invocations

```bash
# Single commit, the common case:
./le commit create \
  replace \
  subject "hook: allow tee pipe, per-session log paths" \
  body "Explanation of why the change was made." \
  file internal/le/hookruntime/bash.go \
  file ai/rules/points/commands/<section>/<point>.md

# A second commit in the same script:
./le commit create \
  append \
  script tmp/commit-<session>-<tag>-<random>.sh \
  subject "feat: add widget support" \
  body "Implements widget rendering for the dashboard." \
  file internal/component/web/widget.go \
  file internal/component/web/widget_test.go

# Spec closure, removing the spec file:
./le commit create \
  append \
  script tmp/commit-<session>-<tag>-<random>.sh \
  subject "spec: close spec-widget" \
  remove plan/spec-widget.md
```

The `script=` line the command prints is the only authoritative path. Its name
carries a random suffix, so no guess reaches another agent's script. Copy it;
never construct it from the session id.

## What the generated script contains

`renderBlock` (`internal/le/commit/script.go`) is the only native source that
spells the raw staging and commit verbs, and it emits them rather than running
them. One commit block holds, in order:

1. A `# Commit <tag>: <subject>` comment and a `# ze-commit-block:` marker
   naming the tag and every path.
2. The critical-review gate re-check, when the commit closes a spec.
3. `git add -- ` with one quoted path per line.
4. `git rm -- ` for any `remove` paths.
5. A concurrency guard. It reads `git diff --cached --name-only` and refuses
   when the index holds a path this block did not name, which is how a
   concurrent session's staged file is caught before it is committed.
6. `git commit -F <message-file>`.

The script opens with `set -euo pipefail` and a `cd` to the repository root, so
a failed step stops it. A push, when one was authorised, runs after every commit
in the script succeeds.

The message file is read when the SCRIPT runs, not when `create` wrote it. So
repairing the message file in place before you run the script is the correct fix
for a bad message, and it is allowed.

## Never put a backtick in an argument

`./le commit create` is invoked from Bash, so `subject`, `body` and every
override reason are shell words before the command sees them. Inside a
double-quoted argument a backtick opens command substitution. A body reading
``the block declares `encoder json` `` runs `encoder json`, prints
`encoder: command not found` to stderr, and substitutes the EMPTY STRING into
the message.

The failure is silent where it matters: the command still writes its message
file and prints `script=`, so the tail of the output reads as success while the
sentence is already mutilated. Quote code in a commit message with plain double
quotes, or name the thing without quoting it.

Two habits make it self-checking. Read the generated message file before running
the script, at the path on the `message=` line. And treat `command not found`
anywhere in the output as a failed invocation rather than noise.

## After the script runs

For a commit carrying Go, module, or vendor paths, run
`./le repository tracked-build check` immediately afterwards. It judges the
commit you just made, which no run before that commit could see.

Report the commit SHA, the included files, the message file, the script path,
the push status, and the verification evidence or the skip reason.

## When a commit fails

A failed commit leaves the index STAGED, and the next session inherits it. The
script stages first and commits second, so the failure exits non-zero, prints
something like `failed to write commit object`, and reads as "nothing happened".
The staging IS what happened.

After any failed commit, read `git diff --cached --name-only`, then either fix
the cause and re-run at once, or unstage your own paths. A signing failure is
the usual trigger precisely because it fails LAST, after every gate has passed
and every file is already staged.

On `gpg failed to sign` or `cannot open /dev/tty`, ask the user to run
`! echo test | gpg --clearsign` to unlock the agent, then re-run the script.

## Rebasing onto a diverged main

A rebase of local commits onto a diverged `origin/main` can re-conflict on the
one derivable bookkeeping file still tracked, `ai/PACKAGE-MAP.md`. Regenerate it
with `./le discovery-index update` at each rebase stop and continue. Finish the
rebase before repairing bookkeeping, never mid-rebase, then regenerate the
derived indexes and recompute any derived ratchet the rebase loosened.

`git rebase --continue` refuses with a misleading "You must edit all merge
conflicts" whenever there are unstaged tracked changes, not only when index
entries are unmerged: `ACTION_CONTINUE` in git's `builtin/rebase.c` checks
`has_unstaged_changes()`. Read `git status` for the unstaged tracked files and
stage or discard them. The message names conflicts you do not have.

An agent never runs `git rebase` itself. The user starts it, and the agent may
only resolve conflicts inside a rebase already in progress.
