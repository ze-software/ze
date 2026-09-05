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
| TestExtractPeerAddress | The function it tested is DELETED. `ExtractPeerAddress` read the selector back out of the command string the translator had just built and answered "" for an unrecognized leading token, which its three callers read as "nothing to flush". The selector now travels with the command in `Translation.Selector`, so nothing re-parses and there is no helper to test. The property survives, stronger and driven from the input side: `TestBridgeSelectorCannotBeSilentlyEmpty` asserts that every translated route carries the selector its ExaBGP line named, over five lines including a bare one and a named peer, and it fails on an empty selector where the old test asserted an empty selector was CORRECT for two of its four rows. |
| TestIsRouteCommand | The function it tested is DELETED. `IsRouteCommand` answered on the substring `update text` anywhere in the command, which is exactly why it kept answering true after the leading token moved while its partner stopped answering. `Translation.Route` is set by the builder that translated the line, so the fact is stated once. `TestFlushBlocksUntilResponse` covers it from the bridge's input side and now also asserts the flush names `10.0.0.1`, which no test asserted before. |
| TestExabgpToZebgpCommand_NonNeighbor | Its whole subject was the passthrough: it asserted that `shutdown` reaches ze's dispatcher unchanged. This commit refuses that line by name rather than forwarding it, so an assertion that it is forwarded would pin the behaviour being removed. `TestBridgeRefusesAnUnrecognizedLineByName` asserts the opposite over seven lines, `shutdown` among them, and also asserts `help` still passes through, which the deleted test never covered. |
| TestBridgeTranslatesTheBareForms | Its second half, five passthrough rows, asserted that ze's own `announce unicast`, `announce blackhole`, `withdraw all` and `withdraw id` spellings pass through unchanged. Those four commands moved to `send bgp <selector> <form>` in phase 5, so the assertion pinned a spelling the tree no longer has. Four of the five rows reappear in `TestBridgeRefusesAnUnrecognizedLineByName` as REFUSALS, which is the behaviour this commit gives them, and the fifth, `help`, reappears there as the one passthrough. The first half, five translation rows, is unchanged. |
