# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| gotoolchain_test | Not a weakening: the read-only refusal case of the new TestTheTestTempDirIsMadePrivateAndWritable skips only under root, because root writes a 0o500 directory and the refusal cannot be observed there; as the owner's user it always runs. Added with the GOTMPDIR test-temp change. |
| livetrim_test | Not a weakening: the new 0o000-leftover subtest of TestTrimTestTempLeftovers skips only under root, because root reads a 0o000 directory and no EACCES can arise; as the owner's user it always runs. Added with the GOTMPDIR test-temp change. |
