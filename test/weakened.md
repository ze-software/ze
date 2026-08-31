# Test weakenings this commit accepts

**This file is REPLACED for each commit. It never accumulates.** Delete the rows
of the last commit, write the rows of this one. The commit gate refuses a row
naming a test the prospective commit does not weaken, so a row left behind by an
earlier commit blocks the next author rather than helping anybody. Git history
holds every past entry: `git log -p -- test/weakened.md` shows the rows of any
commit beside the change they justified.

**Several sessions share this checkout, and this is one shared path.** Write your
rows immediately before you run `./le commit create`, then read the file again
between writing them and running the script. Rows written earlier are a window
for another session to replace them, and a session that writes with `cat >`
rather than an edit replaces the file whole. The refusal is the safe outcome. The
unsafe one is silent: your commit lands carrying another session's justification,
and no gate sees it, because the file is present and the row count is plausible.
Say so on the message bus before you take the slot.

A row here is the AUTHOR's own justification. The owner's approval for changing a
test that carries an `RFC requirement:` tag is a different file,
`test/rfc-changed.md`, and a row here does not authorize one there.

`parseLedger` (`internal/le/testweakened/ledger.go`) reads the first
`| Test | Reason |` table it finds and every table row under it, so this prose is
safe above the table. Do not write a second such header anywhere in the file: the
parser refuses two tables rather than guess which one the gate should read.

| Test | Reason |
|------|--------|
| show-plugins | SPLIT, not weakened: the five expectations that left this file are in `test/parse/show-plugins-memlock.ci`, added by the same commit, and every one of them is verbatim. They are the `contains=memlock` assertions and the whole `match memlock` pipe block. They moved because memlock is a linux-only plugin -- `register.go` and `memlock_linux.go` both carry `//go:build linux`, and the platform-neutral `memlock.go` declares no `init()` -- so on darwin the plugin never registers, `show plugins` has no memlock row, and this test FAILED rather than skipping. The new file carries `option=skip-os:value=darwin`, the idiom seventeen other files in this suite use. The cheaper repair was rejected: putting the skip on this file needs no row here, and costs darwin the json, yaml and table pipe renderings, which are platform-neutral product behaviour with nothing to do with memlock. Suite coverage went UP, not down: `show-plugins` keeps three pipe renderings on every platform, and `show-plugins-memlock` adds a second command block plus a `not:contains=invalid` the original did not have. Both PASS in `./le functional parse` at 319 tests, 268 and 267. |
