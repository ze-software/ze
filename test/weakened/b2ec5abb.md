# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestReferenceAddressReadsTheDefaultAndRefusesAMalformedOverride | Its first half asserted the 1.1.1.1 default, which the owner removed (2026-10-17: the reference is a configured IP, the DNS server by default). Its malformed-override half moved to reference_test.go, which also asserts the leaf, name-server and refusal order. |
| mtu_test | The five assertions that left are the removed test above; no other test in the file changed. |
