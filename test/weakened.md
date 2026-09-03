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
| TestBgpIdentityFromSentOpen | The function it tested is DELETED. `bgpIdentityFromSentOpen` read the router's own ASN and router-id out of a cached sent OPEN, which is the defect this change fixes: the cache is empty until a BGP peer comes up, so the Loc-RIB emulated peer described itself as AS 0 on router 0.0.0.0 in exactly the case RFC 9069 Section 1.1 names. The identity now comes from `bgp router-id` and `bgp session asn local` (`parseLocalIdentity`, `internal/component/bgp/plugins/bmp/sender_config.go`), so there is no OPEN reader left to test. `ai/rules/no-layering.md` requires the old path to go rather than stand beside the new one. |
| TestBgpIdentityPrefersTheFourOctetASNCapability | Same deleted function, and its coverage is REPLACED rather than dropped. The test drove `bgpIdentityFromSentOpen` over a fabricated OPEN to prove the ASN was read from the 4-octet capability rather than from the AS_TRANS in My AS. The wire property it demonstrated is now asserted directly on the producer that writes it, `TestFabricatedOpenUsesASTransForFourByteASN` (`internal/component/bgp/plugins/bmp/bmp_locrib_test.go`), which requires AS_TRANS in My AS for an ASN above 65535, the true value in the ASN4 capability, and the unchanged 16-bit ASN in My AS below it. Its `RFC9069-x-6 negative` tag is not orphaned: `TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven` carries the same id and polarity. |
| bmp_locrib_test | The four assertions are the two deleted tests' own, counted again as a file total. No assertion left the suite uncompensated: the file gains `TestFabricatedOpenUsesASTransForFourByteASN`, and `rfc9069_test.go` gains three tests over the config-sourced identity, including one that requires the plugin to emit NOTHING rather than a zero identity. |
