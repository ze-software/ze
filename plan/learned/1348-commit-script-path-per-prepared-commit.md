# 1348 -- A Namespace Keyed On The Session Splits Again When The Session Splits

## Context

`commit_helper.py` keyed its generated script on the Claude session
(`claude_session_fingerprint`). That was a fix made on 2026-06-10, after two
SESSIONS overwrote each other's prepared script. One session now runs many
subagents, and they all resolve to that one fingerprint. The 2026-06-10 failure
came back one level down: measured 2026-08-05, one session held 53
`tmp/commit-msg-*.txt` files against 18 `tmp/commit-*.sh` scripts. An IS-IS
agent's 20-file commit survived only because its message file outlived a
sibling's `--replace`. The script's own staging guard stopped the wrong commit.

## Decisions

- Named the script per PREPARED COMMIT, not per session: `script_rel_path`
  (`scripts/dev/commit_helper.py`) returns `tmp/commit-<session>-<tag>-<nonce>.sh`.
  The tag is the per-commit tag the message file already carried. The script
  follows the idiom that worked, rather than inventing a second one.
- Added the nonce so the path cannot be RECONSTRUCTED. Guessing is what bit the
  session: a path copied on the belief it was one's own belonged to another
  agent. The `script=` line is now the only way to learn a path. Steps 2 and 7
  of `ai/rules/git-safety.md` say to read it rather than build it.
- Made the tag allocation atomic. `next_tag` reserves with an O_EXCL create of
  the empty message file, which is `learned_next`'s idiom. A glob-only allocator
  hands the same letter to two agents, because `create` writes the message file
  at the END. Verify-status and the discovery-index materialization sit inside
  that window.
- Kept `--append` working and gave it a name to work on. With `--script <path>`
  it appends to the script that create printed. Without it, it resolves only
  when the session has exactly ONE prepared script. With several it refuses,
  because picking one would be the guess this change removes.
- Made `--replace` fail closed. A script whose declared paths
  (`# ze-commit-block:`) share nothing with this commit's is another prepared
  commit, so replacing it is refused. There is no override flag and none is
  needed: dropping `--script` yields a fresh path, which is what the caller
  wanted.

## Files

- `scripts/dev/commit_helper.py`
- `scripts/dev/commit_helper_test.py`
- `scripts/dev/commit_helper_test.go`
- `ai/rules/git-safety.md`

## Consequences

- Any tmp/ artifact keyed on "who is running" needs the unit of WORK in its name
  as well. The identity that looked unique when the rule was written subdivided.
  The artifact is one per prepared commit, so the name is too.
- An allocator that reads state its own writer sets seconds later is a race
  whatever the window looks like on one machine. Reserve at allocation time.
- The staging guard now names the owning script in its abort. It is the message a
  reader gets at the moment they have run the wrong script, and "which one did I
  just run" is the question they have.
