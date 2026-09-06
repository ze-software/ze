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
| TestWritePcapGlobalHeader | Its subject `writePcapGlobalHeader` is DELETED. `internal/analyze/convert.go` no longer writes the pcap file header itself: it calls `pcap.WriteFileHeader`, and `TestPcapGlobalHeader` (`internal/core/pcap/pcap_test.go`) asserts the same 24 octets -- magic, version 2.4, the zeroed timezone and sigfigs, snaplen and link type -- over the one remaining producer. The link type it pins changed from 228 to 101 on purpose, which is the defect this commit fixes: 228 is IPv4-only and silently dropped every IPv6 record. |
| TestWritePcapBGPPacket | Its subject `writePcapBGPPacket` is DELETED, replaced by `pcap.Framer.WriteMessage`. `TestFrameIPv4TCP` (`internal/core/pcap/pcap_test.go`) carries every assertion it made -- the 16+20+20+payload record length, the 0x45 version and IHL byte, protocol 6, the source and target addresses, source port 179, and the payload at its offset -- and adds the three the old test could not make, because the old code got them wrong: the IP and TCP checksums verify, the sequence advances by the payload, and the acknowledgement names the reverse direction. |
| TestIpTo4 | Its subject `ipTo4` is DELETED. It mapped a 4-byte or 16-byte slice to a `[4]byte` and returned the zero array for anything else, which is the silent-zero shape that made the IPv6 skip invisible. `convertFlow` replaces it with `netip.AddrFromSlice` plus `Unmap`, and reports false rather than a zero address. `TestConvertFlow` (`internal/analyze/convert_test.go`) carries all four of its cases -- ipv4, ipv4-mapped, nil and short -- and adds the IPv6 and mixed-family cases the old helper could not express. |
| fixedTime | A helper, not a test. It supplied a fixed timestamp to the three deleted tests above and has no remaining caller. The timestamps those tests pinned are pinned by `testStamp` in `internal/core/pcap/pcap_test.go` and by the literal `1700000000` that `writeTempMRTAFI` still writes into every MRT record. |
| writeTempMRT | It gained no less checking: the body moved to `writeTempMRTAFI(t, afi, peerIP, localIP)`, which carries both `require.NoError` calls verbatim, and `writeTempMRT` now calls it with the IPv4 arguments it always used. Its caller `TestRunConvertJSON` is unchanged. The split exists so `TestRunConvertPcapFramesBothFamilies` can build an IPv6 record, which is the case the conversion used to drop. |
