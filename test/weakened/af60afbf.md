# Test weakenings this commit accepts

One helper leaves a test file. It leaves because it became PRODUCT code, not
because an assertion was dropped: no test lost a check, and the two call sites
that used it still call it under the same name.

| Test | Reason |
|------|--------|
| suiteNames | `suiteNames` was a test-only helper rendering a suite list for a failure message. Phase 4 gives the gating run an operator-facing decision to print, and that output needs the same rendering, so the function moved WHOLE to `internal/le/functional/suitemap.go` with its body unchanged. The test file now calls the product function instead of declaring its own copy, which is one fact in one place rather than two that can disagree. Both assertions that used it (`the run list is ...` and `the skip list is ...`) are unchanged and still fail on the same conditions. |
