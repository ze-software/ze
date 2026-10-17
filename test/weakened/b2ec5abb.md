# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestTheGateSnapshotAgreesWithItself | Moved, not lost: the error check on collectRFCCompliance moved into complianceOfThisCheckout (internal/le/site/checkout_test.go), which calls t.Fatal on the same error; the test now reads the snapshot the build test shares instead of deriving its own, to fit the race stage. |
