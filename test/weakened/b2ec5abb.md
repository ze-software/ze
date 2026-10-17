# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestIntegration_OutOfZoneNXDOMAIN | Renamed, not lost: now TestIntegration_OutOfZoneRefused in the same file. A name outside every served zone is answered REFUSED (RFC 1035 Section 4.1.1, as zones.go answerQuestions does), so the old NXDOMAIN expectation was wrong; the renamed test asserts REFUSED with AA clear and empty sections. |
