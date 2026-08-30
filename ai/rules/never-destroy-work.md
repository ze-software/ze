# Never Destroy Uncommitted Work

**When:** before deleting, reverting, or overwriting any file holding uncommitted or user-visible work
**Severity:** blocking

## Directives

**You MUST NOT delete, revert, or overwrite a file holding uncommitted work the user wrote or requested, without explicit permission.**
**You MUST NOT leave a file undeleted only because deletion needs permission. Ask for the permission instead.**

**You MUST treat each of these as work the user paid for: any file the user asked you to write, any file you wrote at the user's direction in this session, any in-progress implementation even when it does not compile or pass lint, and any scratchpad the user might be reading (`to-check`, `Stub/`, notes).**
**You MAY act without asking on a build artifact that `make`, `go mod vendor` or codegen regenerates, on a cache under `~/.cache/` or similar, and on a file the user EXPLICITLY marked disposable. Care is still owed.**

**You MUST NOT destroy work on any of this reasoning: "lint is blocking", "it will be rewritten anyway", "I just wrote it one turn ago", "it is clearly an error, dead code, or scaffolding", "I am reverting my own mistake", or "the tree is in a broken state".** A hook is advisory, the user might want to diff what is there, you wrote it at their direction so it is theirs now, "clearly" is often wrong, and a broken tree is preferable to lost work.

**When a file SHOULD be deleted and this rule needs permission for it, you MUST ask the user directly, and you MUST NOT call the work complete while an unwanted file stays in place only because permission was needed.**
**When you are unsure whether the file SHOULD be deleted, you MUST state the situation ("file X exists, lint is flagging Y, tests fail because of Z") and ask what to do.**

## Forbidden Without Explicit Permission

**Each operation below MUST NOT be performed without explicit permission:**
- `rm <path>` on any user-visible file, meaning any path that is not already untracked trash. Ask before deleting it, and never leave the file behind as a workaround when deletion is the correct fix.
- `git restore <path>` on a modified working-tree file, and `git reset --hard` or `git clean -f` anywhere. `ai/rules/git-safety.md` forbids these outright.
- Overwriting an existing file with content that drops user edits. Read the current file, then merge or ask.
- Truncating or overwriting a log file, session state, or a `tmp/` artifact the user might be inspecting. Leave it.
