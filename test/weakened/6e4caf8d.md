# Test weakenings this commit accepts

One test is RENAMED, and it now asserts strictly more than it did before.

`TestSessionTableLookupByMAC` proved that `SessionTable.lookupByMAC` resolved
a subscriber MAC to its one session. That producer no longer exists: RFC 2516
places no per-peer session limit, so `byMAC` became a set of sessions per MAC
(`internal/component/l2tp/pppoe/session.go`), and `lookupByMAC` was replaced
by `sessionsByMAC`, which returns every session a MAC holds. A test named for
a function that is gone would name nothing.

The renamed test, `TestSessionTableSessionsByMAC`, drives the replacement over
the same three cases and asserts more than the old one could. Where the old
test read `got.SID != 1` off a single pointer, the new one asserts
`len(got) != 1 || got[0].SID != 1`, so it now fails on a MAC that resolves to
the wrong NUMBER of sessions as well as the wrong one. The unknown-MAC case is
unchanged.

Nothing left the suite. The same commit adds
`TestSessionTableIndexesEverySessionOfOneMAC` (a second session for one MAC
joins the set rather than replacing the first),
`TestSessionTableRemoveKeepsSiblingSessions` (removal is per session and the
MAC entry drops at zero) and `TestSessionTableChurnLeavesNoIndexEntries` (1000
add and remove cycles over 10 MACs leave both maps empty), each of which
exercises behavior the single-pointer index could not have.

| Test | Reason |
|------|--------|
| TestSessionTableLookupByMAC | Renamed to `TestSessionTableSessionsByMAC` in the same commit, because its producer `lookupByMAC` was replaced by `sessionsByMAC` when the MAC index became set-valued. The renamed test covers the same three cases and asserts the session COUNT as well as the identity, which the old single-pointer assertion could not. Three further session-table tests land in the same commit over the multi-session behavior the rename exists for. |
