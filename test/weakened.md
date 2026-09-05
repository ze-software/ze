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
| test-announce-forms-are-separate-commands | Each form was reachable at TWO paths, bare and under `peer <selector>`, and this spec deletes the second. Cases 5 to 7 asserted the usage line of `ze peer <selector> announce <form>`, a command the tree no longer declares, so they could only assert that a dead path is dead. Every form keeps its own usage assertion at the one path that survives, `ze send bgp <selector> <form>`, and the selector is now IN each of those three lines rather than in three separate cases, which is what the move made true. The merged-line reject was retargeted with them: `ze announce <selector>` cannot fire once `announce` is not a root, so it now rejects `send bgp <selector> <args>`, which is the shape a single handler behind three tail grammars would render. |
| test-withdraw-forms-are-separate-commands | The same deletion, on the withdraw side. Cases 5 to 7 asserted `ze peer <selector> withdraw <form>`, the second of two paths, and the second path is gone. The three forms keep their usage assertions at `ze send bgp <selector> withdraw <form>`, and both rejects are kept unchanged: the four-tail-grammars line and the `withdraw all [selector` tail this spec's predecessor removed. |
