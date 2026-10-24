| Test | Reason |
|---|---|
| TestMigrateReceivePolicyFromTemplate | Its parse and migrate checks moved into the shared helper migrateNeighborOutput, which fails the test with require.NoError on either, so the count of assertions written in the body fell from 2 to 1. The behavior changed on purpose: as-set accept from a template used to be refused and is now written to the inheriting peer, and the remaining assertion checks that the peer session block holds as-set accept, which the old refusal check could not. |
