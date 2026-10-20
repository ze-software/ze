| Test | Reason |
|---|---|
| TestRFC4301FinalEntryIsUnderEveryZeSPDEntry | No assertion left: the two Fatal checks on parsing the `vpn ipsec` block (parse error, absent leaf parses to discard) moved unchanged into the lifecycleConfig helper, which this test and the two new lifecycle tests share, and the test gained one (every Child SA PROTECT entry installed and removed) with the Child SA it now carries, so its RFC4301-4.4.1-3 claim covers every Ze entry (review of efea677e61). |
