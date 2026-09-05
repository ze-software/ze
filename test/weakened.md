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

**A test in a NEW file needs no row in either ledger, and the two gates disagree
about that.** The commit gate reads the file at HEAD (`committedText` in
`internal/le/commit/rfcchange.go`) and skips a path with no HEAD version, so it
computes no change for a new file and REFUSES a row naming a test in one. The
write hook reads the file on disk instead (`Proposed` in
`internal/le/testweakened/proposed.go`, where `taggedCarrier` tests the current
`oldText`), so it DEMANDS a row before it lets you edit a new file that already
carries `RFC requirement:` tags. An author who obeys the hook is then refused by
the gate. Until one of them changes, write the tags in the same edit that creates
the file, and carry no row for it.

| Test | Reason |
|------|--------|
| TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain | Not a lost assertion: the function MOVED, and the gate compares each path against its own HEAD text with no rename detection (the third row of the table in test/rfc-changed.md names this false finding). It leaves `internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go` and arrives in `internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go` in the same commit, with its assertion unchanged and two positive subtests added. It also goes from RED to GREEN here, because this commit is the fix it was written against. |
| zzprobe_prefixsid_announce_test | The same move, seen from the file the gate resolves by stem. The path is emptied rather than deleted because the pretool-bash test-deletion gate refuses `rm` on a `_test.go` path and an agent cannot answer its prompt; the owner deletes it with `rm internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go`. Nothing this file asserted is gone. |
