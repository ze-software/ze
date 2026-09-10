# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| TestOSPFConfigApplyReconcile | Removed the assertion that a cost-only reload must appear in the restart journal. The test retains socket, resulting-cost and interface-removal assertions. Explicit-cost regressions additionally prove neighbor retention and updated Router-LSA metrics; they fail under the old restart behavior and pass with the fix. |
