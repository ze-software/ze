# Committing

`./le commit create` is the only staging and commit route in this repository.
It writes a message file and an executable script; you run the script. There is
no hand-written fallback.

What you OWE around a commit is `ai/rules/git-safety.md`. This page is how the
command works.

## Why one command owns it

Several sessions share this checkout, so they share one git index and one
working tree. A loose `git add` followed by a `git commit` therefore carries
whatever another session staged in between, and `git add` reads the working
tree, so it also carries whatever another session wrote into a file your commit
names.

The generated script answers both. It commits from an index of its OWN, seeded
from HEAD when the script runs, so the commit's population is exactly the paths
the block names. And it commits the blob each path held when `create` READ it,
so an edit that arrives afterwards stays in the working tree for whoever wrote
it to commit under their own subject.

Neither property depends on you noticing anything. Eight rows in
`plan/journal/concurrent-session-corruption.md` are one session's unfinished
work published under another session's message, and the last of them happened
to an author who had been warned about those exact files minutes beforehand.

## The keywords

`./le commit create` takes keywords, not flags. Every one takes a value except
`append` and `replace`.

| Keyword | Repeats | Meaning |
|---------|---------|---------|
| `subject` | no | The one-line commit subject. At most 72 characters, and a longer one is refused with the count and the overage |
| `body` | yes | One body chunk, wrapped to 72 characters without breaking a word. Two chunks run together, so a paragraph break is an empty `body ""` between them |
| `file` | yes | One explicit file to stage. Never a directory |
| `file-list` | yes | A file holding one path to stage per line. Blank lines and `#` comments are skipped |
| `remove` | yes | One tracked file to delete, named exactly: a directory or a glob is refused at create time, because the script removes one index entry by its literal name. The commit removes it from Git, then the script deletes the working-tree copy when that copy is what the commit removed. Leave the file in place: no `rm` beforehand |
| `remove-list` | yes | A file holding one tracked path to delete per line, each handled as `remove` handles one |
| `replace` | no | Start a fresh script. Use it for the first commit of a session |
| `append` | no | Add another commit block to a script that already exists. A script an older `./le` prepared, whose blocks do not record the commits they publish, is refused: prepare a fresh one with `replace` |
| `script` | no | The script to append to. `create` with no `script` always gets a distinct path |
| `session` | no | The eight-hex commit namespace. `./le commit session` creates or reuses this harness session's |
| `tag` | no | The block tag inside the script |
| `attest` | no | Two values: a commit and the owner's words, `attest <commit> "<owner words>"`. Prepares an EMPTY commit (its parent's tree) whose message carries `Owner-attested: <stable patch-id>: <owner words>`, which answers every person act the named commit owes. Takes no `file` or `remove`: an attestation that changed the tree would owe the full verify it exists to spare. Refused for a commit already on the upstream, one not on this branch, and one carrying no owed trailer |
| `ledger-carried` | yes | Only on the ledger-deletion commit (`remove` paths, no `file`): `"<sha> <gate key>: <reason>"`, the gate key `review-override` or `rfc-change-ok`. Writes one `Ledger-carried:` trailer. A row on a commit already on the upstream carries no patch-id and is audit only; a row on an unpushed commit carries that commit's stable patch-id, computed here and never given, and holds the push until it is answered |
| `no-test` | no | The reason a commit carries no test evidence |
| `rfc-change-ok` | no | The reason a commit changes an RFC-tagged test unit the owner has not approved yet. Instead of refusing, the message carries one `RFC-approval-owed: <package>.<TestName>: <reason>` trailer per unapproved unit, and the push derivation holds the push until the owner answers |

`Message` in `internal/le/commit/input.go` enforces the subject limit and the
body wrap, and appends one `RFC-approved: <package>.<TestName>: <reason>`
trailer line, never wrapped, for each owner approval the commit uses. The
approval is written once, by `./le rfc approve unit <package>.<TestName> reason
"<the owner's words>"`, into `tmp/commit-rfc-approved-<session>.md`; the
generated script drops the used rows from that file after `git commit`
succeeds, so the trailer in the commit is the only record
(`docs/contributing/rfc-implementation-guide.md`).

Each admission the gates grant is recorded the same way, as one trailer line
after the approvals (`Prepared.Trailers`):

| Admission | Trailer | Read by the push derivation |
|-----------|---------|-----------------------------|
| `review-override "<reason>"` on a closure with no clean review | `Review-owed: <spec stem>: <reason>` | yes, until a clean review, a recorded Review Gate or the owner's attestation answers it |
| `rfc-change-ok "<reason>"` | `RFC-approval-owed: <package>.<TestName>: <reason>`, one per unit | yes, until a later `RFC-approved:` for the unit or the owner's attestation |
| `structural-red-ok "<reason>"` with a charged structural red | `Structural-red-ok: <reason>` | no, audit only |
| `broken-head-fix "<reason>"` for the sole compiles red | `Broken-head-fix: <reason>` | no, audit only |

An admission no gate used records nothing. A reason spanning lines is refused,
because a trailer is one line. `Message` refuses a subject or a body line that
starts with any declared trailer key (`declaredTrailers` in
`internal/le/commit/actions.go`, which also lists `Owner-attested:` and
`Ledger-carried:`), naming the line and the one route that writes it: a
trailer is a record the derivation reads back, never the author's prose.

## What the command refuses

`normalizePath` and `validateAddPath` (`internal/le/commit/input.go`) refuse a
path before the script is written:

- A path outside the repository, or a `..` component.
- Anything under `.git/`.
- A root agent instruction file: the generated `AGENTS.md`, and `CLAUDE.md`, which must not exist.
- A path `git check-ignore` matches. The index is consulted, so a TRACKED file
  that matches an ignore pattern is committable: git already carries it, and the
  pattern governs what is added under that path next.
- A path that does not exist. Use `remove` for a tracked deletion. For a
  relocation, write the destination and leave the source in place; name the old
  path with `remove` and the new one with `file`, because `git mv` stages.
  The same spec filename carried into a different category is a relocation,
  not closure, including migration from the old root, `immediate`,
  `pre-release`, or `to-review` layout into the current categories.
  A changed filename is a rename, not closure, only when exactly one added
  spec absent at HEAD has text equal to the old spec at HEAD with every
  occurrence of the old stem replaced by the new stem. Pairing is one to one.
  Other edits in a stem rename make the removal a closure: rename first and
  edit later (`renamedSpecs`, `docs/contributing/spec-workflow.md`).
<!-- source: internal/le/commit/review.go -- renamedSpecs -->

- A directory. Scripts stage explicit files.

`validateRemovePath` refuses a `remove` path that is not tracked, so you never
have to run `git ls-files --error-unmatch` yourself.

`rfcChangeProblems` (`internal/le/commit/rfcchange.go`) refuses a commit that
changes an RFC-tagged test unit no row in this session's
`tmp/commit-rfc-approved-<session>.md` names, and prints the `./le rfc approve`
command for each unit. `rfc-change-ok "<reason>"` admits the commit with one
`RFC-approval-owed:` trailer per unit instead; a gate that could not name the
units it judged is refused even then, because it has nothing to record. A row a landed commit already carries as a trailer approves
nothing further, and the next successful commit drops it from the file.

A list keyword buys one thing: a population too large to type stays explicit. It
broadens nothing else. Every line is validated as its own path, and the script
spells each one in its block marker and in its index entries. Write the list
with a command that answers what changed, then read it before you pass it:

```bash
cd ../gh-pages && git -c core.quotePath=false status --porcelain |
  grep -v '^ D' | sed 's/^...//' > "$dir/add.txt"
```

The command also checks verification freshness for the named file population,
and never refuses a commit for a missing verification: that is owed before a
push. Its output carries one `debt=` line, read from the derivation in
`docs/architecture/testing/verify-freshness-scope.md`, "Verification debt,
derived". When nothing is owed it reads `debt=nothing owed: HEAD's tree is
covered by the green verify of <G>`. Otherwise it carries the same headline as
the session-start warning, such as `debt=2 commit(s) owe a full verify since
<G>` or `debt=HEAD's tree is not covered by a green verify (no green record):
run ./le verify worktree`, followed by each reason the push is refused,
separated by `; `. It never reads as a count of zero.
`debt=cannot answer: <why>` means the derivation could not answer. Nothing is
written: no commit carries a debt file.
<!-- source: internal/le/commit/push.go -- debtReport, headline -->

The keywords `unverified` and `missing-full-verify-ok` were removed and are
refused naming `./le verify worktree`; `push` on `create` was removed and is
refused naming `./le commit push "<owner authorisation>"`. The verbs
`debt-list`, `debt-clear` and `debt-discharge` were removed too, each refused
naming `./le commit debt-status`, `./le verify worktree` or `create ... attest`.

## Pushing

`./le commit push "<owner authorisation>"` is the one push route, and only the
owner orders it (`ai/rules/git-safety.md`). It derives the verification debt
and chooses the push target P: HEAD when the last green full verification
covers HEAD's tree, else the verified commit the walk from it reaches. It is
refused, exit 1, naming what clears each reason, when no P exists, when nothing
verified is ahead of the upstream, when the upstream does not fast-forward to
P, or while a person act is open on a commit `P ^@{u}` publishes. It answers
exit 2 when the derivation cannot answer, such as a branch with no upstream
(`git branch --set-upstream-to=origin/<branch>`).

When it admits, it writes a push-only script at the session's `script=` path
and prints that line, the commit it pushes, and every commit the push leaves
behind, each owing `./le verify worktree`. The script makes no commit. It
re-runs `./le commit debt-status commit <P>` and pushes nothing when that
fails (a lost green record, a moved upstream, a rebase that took P off the
branch), then runs `git push <remote> <P>:<upstream branch ref>`. It pushes P
by SHA, never the branch tip, so a commit that lands after the script was
prepared is not published. Outside the Ze checkout P is HEAD and the script
carries no re-check.

| Script already at the session's path | `./le commit push` | `create ... append` |
|--------------------------------------|--------------------|---------------------|
| commit blocks not yet run | refused: run that script first, then verify and push | as before |
| a push-only script | prepared again in place, P chosen again | refused: an appended commit would not be published; run the push script first, or use `replace` |

`./le commit debt-status` answers exit 0 when P is HEAD and nothing is owed, 1
when anything is owed (listing it, P included), and 2 when it cannot answer.
`debt-status commit <rev>` answers 0 when `<rev>` is an admissible push target
with no open item in `<rev> ^@{u}`, whatever follows it on the branch.
<!-- source: internal/le/commit/push.go -- preparePush, pushScriptPath, renderPushScript, answerDebtStatus -->

An owner attestation is `./le commit create subject "<subject>" attest <commit>
"<owner words>"` with no file. Its empty commit leaves HEAD's tree the green
tree, so the push that follows needs no new verification. The script builds it
on its parent's tree and refuses it if the diff names any path.
<!-- source: internal/le/commit/attest.go -- attestationTrailer, ledgerCarriedTrailers -->
<!-- source: internal/le/commit/script.go -- renderEmptyPublish -->

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
  remove plan/release/fixes/spec-widget.md
```

The `script=` line the command prints is the only authoritative path. Its name
carries a random suffix, so no guess reaches another agent's script. Copy it;
never construct it from the session id.

The `message=` line carries a random suffix for the same reason, so a script and
its message are one artifact. A second `create` under the same tag therefore
allocates a second message and cannot write over the first one's. Until
2026-09-05 it could, and the first script then made its commit under the second
one's subject with nothing printed to say so
(`plan/journal/pointer-shared-across-the-names-it-indexes.md`).
<!-- source: internal/le/commit/script.go -- nextTag, allocateMessage, allocateScript -->

## What the generated script contains

`renderBlock` (`internal/le/commit/script.go`) is the only native source that
spells the raw staging and commit verbs, and it emits them rather than running
them. One commit block holds, in order:

1. A `# Commit <tag>: <subject>` comment and a `# ze-commit-block:` marker
   naming the tag and every path.
2. The critical-review gate re-check, when the commit closes a spec.
3. Before the loop, three checks on the message and the signing setting, then
   the starting point. A missing message file is refused first: a block deletes
   its message once its commit lands, so a missing one on a rerun means the
   block already published or the file was cleaned up, and the refusal says to
   check `git log` for the subject and prepare whatever is still uncommitted
   again. The message is then passed through `git stripspace`, the cleanup
   `git commit -F` applies, and an empty result is refused, as `git commit`
   refuses it. `commit.gpgSign` is read with `git config --type=bool`: unset
   means unsigned, as with `git commit`, and a value git cannot read as a
   boolean is refused rather than read as unset. Last, `_ze_parent` is set to
   the block's recorded `PreparedHead`, the HEAD `create` read just before it
   took this block's snapshot. Each block records its own, so an appended block
   prepared later compares with its own moment.
   The loop starts here and runs to the swap in step 6. Each attempt keeps the
   HEAD it compares with in `_ze_seen`, then reads the parent ONCE:
   `_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')`. On the first attempt
   `_ze_seen` is the recorded `PreparedHead`; on a later one it is the parent
   the attempt before it read. Before building on the parent, the block lists
   the commits between `_ze_seen` and the parent which change a path it names,
   leaving out, by exact SHA, the commits this script published itself
   (`_ze_own`), because an appended block may name a path an earlier block
   committed. Any left is a peer's change the snapshot never held, so the block
   refuses, names the paths, and asks for a fresh `create`: building on it would
   write content prepared before the change over it. Then
   `git read-tree "$_ze_parent"` into `<script-path-without-.sh>.index`, the
   private index this block commits from. Steps 4 to 6 run inside the same
   loop, and a refused swap in step 6 whose HEAD moved sends it round again.
4. `git update-index --index-info` with one `git ls-files -s` line per path,
   written by `snapshotIndexEntries` when `create` ran. The line names the blob,
   so the content is fixed at preparation time. A blob a `git gc --prune` has
   collected since is refused here, naming every path whose content is gone.
5. For any `remove` paths, their `git ls-files -s` lines captured from the
   private index into `_ze_removed`, then `git update-index --force-remove`.
   The captured lines are the entries the parent holds, mode included, and step
   8 compares against them.
6. The publish (`renderPublish`). `git write-tree`, then `git commit-tree -p
   "$_ze_parent"` with the stripped message. A failure there, signing as a
   rule, is refused with HEAD untouched. `git commit` is not used, because it reads HEAD a
   second time for the parent: a peer's commit landing between the two reads
   became the parent of a tree that never held the peer's work, and every path
   the block did not name was silently reverted (450f9f1ee0 and d621be6677 on
   2026-10-14). `git commit-tree` ignores `commit.gpgSign`, so the script reads
   it and passes `-S`, which signs with `user.signingKey` as `git commit` does.
   No git hook runs; none is installed. The commit is then checked against its
   parent: it MUST change at least one path, and no path the block does not
   name, or the script refuses with HEAD untouched. Last, `git update-ref -m
   "commit: <subject>" HEAD <commit> <parent>` moves the branch only if it still
   names that parent. When the swap fails, HEAD is read again. If a peer moved
   it, the block goes round again on the new HEAD, up to 5 times, and step 3's
   check refuses if that peer changed a named path. If HEAD did not move, git
   refused for another reason, a stale ref lock or a permission, so the script
   stops with git's own error rather than retrying and reporting a race that
   never happened. When git names a lock file, the refusal names it too, and
   says to run the script again once the other git command finishes, removing
   the lock only when no git process is running: git waited only
   `core.filesRefLockTimeout`, so the holder may be a live git that is slow.
   Every refusal before the branch moves opens `ERROR: commit <tag> (<subject>) was not published`,
   with one exception: a block whose message file is gone opens `ERROR: commit
   <tag> (<subject>) was not run`, because a missing file means the block most
   likely published on an earlier run of the script. Both lines say HEAD is
   unchanged by that block, never by the run, since an earlier block of the same
   script can have published in this run. The subject the refusal prints is
   quoted once as a whole, so no character in it is shell syntax.
   The landed commit is printed as `[<short-sha>] <subject>` with its stat.
7. A drift note, then the message file is deleted. The note stages the working
   tree into a throwaway index and reports any named path whose content, mode,
   or existence moved since preparation. It never refuses: the commit is safe,
   and the difference is still in the working tree. `git update-index
   --refresh` looks like the tool for this and is not, twice over: it rewrites
   the entry it reports, and it refreshes the whole index rather than the
   pathspec. Git holds the message once the commit lands, and the file is what
   holds the block's tag: an automatic tag is taken while any message carries
   it, so a kept file would never free its letter and a session would run out
   after 26 commits. `set -e` keeps a failed commit's message, whose script
   still runs.
8. For a block with `remove` paths, the working-tree deletion
   (`renderWorkingTreeRemoval`). Each removed path's copy is staged into a
   throwaway index, and the copy is deleted only when its entry line is one of
   the lines step 5 captured. A copy whose content or mode differs, a copy Git
   cannot read or stage, a path HEAD never held, and a directory (a removed
   file replaced by one, or a submodule)
   each leave the path in place and print `NOTE: kept <path>: ...` on stderr,
   because that content is in no commit and somebody has to decide about it. A path already absent is
   skipped in silence. The step runs after the publish under `set -e`, so a
   failed commit deletes nothing, and a deleted copy is always one `git show`
   away. A spec closure therefore leaves no file behind: commit A carries the
   spec's final content with `file`, and commit B's `remove` finds the copy
   equal to it. A commit carrying RFC approvals then drops the rows it used
   from the session's approvals file.
9. `git ls-tree HEAD -- <paths> | git update-index --index-info`, and
   `git update-index --force-remove` for the `remove` paths, which point the
   SHARED index at what was just committed, then the private index is deleted.
   Without it every other session reads those paths as staged changes of yours.
   It is the block's last step because it is the one that waits on another
   process: while git refuses over a `.git/index.lock` that a peer holds, it
   retries once a second for up to 30 seconds. Any other failure, or a lock
   still held after that, stops the script with the commit landed and prints
   `ERROR: commit <sha> landed, but the shared index could not be pointed at
   it.`, the paths left stale, and the exact commands to run from the checkout
   once the lock is released. A later block in the same script has not run.
<!-- source: internal/le/commit/script.go -- renderPublish, renderPrivateIndex, renderLostBlobs, publishAttemptsMax, renderWorkingTreeRemoval, renderSharedIndexRepair, indexLockWaitSecondsMax -->

The script opens with `set -euo pipefail` and a `cd` to the checkout it was
PREPARED for, named as an absolute path, so a failed step stops it and a script
prepared for one tree never stages in another. A commit script never pushes:
a push is a script of its own (Pushing, above).

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

## Committing a sibling checkout

`ZE_REPO_ROOT` points the command at another tree, which is how the published
site and the wiki are committed from the Ze checkout:

```bash
ZE_REPO_ROOT=../gh-pages ./le commit create replace \
  subject "site: republish the generated tree" \
  file-list "$dir/add.txt" remove-list "$dir/del.txt"
```

The gates that read Ze's own sources do not run there, and `lepath.IsCheckout`
decides it: a tree with no `go.mod` beside `feature-gates.txt` is not the Ze
checkout. It holds no test ledger, no discovery index, no verification
certificate and no `plan/` specs, so each of those gates would refuse the commit
for want of a file that tree never had, or read a path that means something else
in it. `verify=NOT-APPLICABLE` says so in the output, nothing is derived, and
`./le commit push` there prepares a push of HEAD with no re-check.

Path validation, the message contract, the private index the script commits
from, and the push authorisation apply in every tree.

## After the script runs

For a commit carrying Go, module, or vendor paths, run
`./le repo compiles check` immediately afterwards. It judges the
commit you just made, which no run before that commit could see.

Report the commit SHA, the included files, the message file, the script path,
the push status, and the verification evidence or the skip reason.

## When a commit fails

A failed commit leaves the SHARED index exactly as it found it. The staging
happens in the block's private index, and the shared one is written only after
the branch holds the commit, so a failure really is "nothing happened". Fix the
cause and run the script again.

A signing failure is the usual trigger, because it fails LAST, after every gate
has passed.

On `gpg failed to sign` or `cannot open /dev/tty`, ask the user to run
`! echo test | gpg --clearsign` to unlock the agent, then re-run the script.

## Rebasing onto a diverged main

A rebase of local commits onto a diverged `origin/main` can re-conflict on a
generated file git still tracks. The rendered rules and
`docs/features/test-health.md` are the ones to expect. Rewrite the conflicted
file with its own generator at each rebase stop and continue. A DERIVED artifact
is never one of them: it is untracked, so a rebase cannot conflict on it. Which
files those are is `derived.All`'s answer rather than a list here, and
`./le ai hooks session-start` prints each one it builds. The RFC ledger and
its shards left git on 2026-09-11 and joined that set. Finish the rebase before repairing
bookkeeping, never mid-rebase, then recompute any derived ratchet the rebase
loosened.

`git rebase --continue` refuses with a misleading "You must edit all merge
conflicts" whenever there are unstaged tracked changes, not only when index
entries are unmerged: `ACTION_CONTINUE` in git's `builtin/rebase.c` checks
`has_unstaged_changes()`. Read `git status` for the unstaged tracked files and
stage or discard them. The message names conflicts you do not have.

Thomas's standing request, October 1, 2026:

> do a git pull --rebase and make sure we do this before first edit on any new sessions

On October 3, 2026 Thomas narrowed it to its purpose: a session never edits on
top of a branch that is behind its upstream. This checkout is shared and always
dirty with other sessions' work, so demanding a clean pull blocked every session
even when origin had nothing new.

Before its first edit an agent runs `git fetch` and `git rev-list --count
HEAD..@{u}` under `ai/rules/git-safety.md`, "Never edit behind the upstream". A
count of zero needs no pull, and a dirty tree does not stop it. A failed fetch
or a missing upstream stops the session, because the check has no answer. Only a
count above zero owes the same-branch `git pull --rebase --no-autostash`, and
only then do a dirty tree, a merge or rebase in progress, or a failed pull stop
the session. That standing authorization does not authorize a direct `git rebase` or
integrating a worktree branch. If the pull conflicts, stop and report it;
conflict resolution requires the user's direction.
