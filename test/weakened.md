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
| TestStartSenderWiresSessionPriming | REPLACED, not removed, and the replacement is why. `startSender` was a stop-all/start-all: its first statement was `stopSenders()`, so any reload that reached it ended every collector's session, including collectors the operator never touched. That is the behaviour the owner overturned on 2026-08-31 when he ruled ze must implement a real per-peer Peer Down/Peer Up for RFC 8671 Section 7.2 rather than bouncing the whole session. `syncSenders` replaces it as a reconciler: an unchanged collector keeps its session, a removed or moved one is stopped. A test named for wiring a stop-all/start-all cannot assert a reconciler. The priming it covered is asserted in `rfc8671_test.go` through the plugin config-apply callback, which is a stronger position than the old test held -- it drove the function directly, and the function was at that time unreachable from any reload. |
| TestStartSenderIsIdempotent | Same replacement. Its subject was that calling `startSender` twice is safe, which was true and is now the wrong question: `syncSenders` is not idempotent-by-restart, it is a reconciler whose result depends on the difference between two configurations. The property that matters after the change is that an unrelated reload bounces nothing, and `TestRFC8671UnrelatedBGPChangeBouncesNothing` asserts exactly that over the real callback. Its own doc comment had also gone stale in the other direction: it read "LATENT, not live, and deliberately tested anyway", written when no reload could reach the function at all. |
| TestBgpIdentifierFromSentOpen | RENAMED to `TestBgpIdentityPrefersTheFourOctetASNCapability`, following its subject. `bgpIdentifierFromSentOpen` returned only the router id; RFC 9069 Section 5.1 also requires the Peer AS, so `bgpIdentityFromSentOpen` returns both as one `localIdentity`. The rename is not cosmetic: the new test asserts something the old one could not, that the ASN is read from the 4-octet ASN capability rather than the two-octet My AS field, because an AS4 speaker fills My AS with AS_TRANS (23456) and reading it would publish 23456 as the router's AS. Coverage went up by the case that matters. |
