# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| TestWithLastOutputCarriesThePeersAnswer | Its subject is deleted, not its proof: withLastOutput was the per-caller wrapper that put a wait's last answer on its timeout, and interoplab.Wait's own not-ready error now carries that answer for every caller, so the wrapper went (no layering). TestWaitTimeoutCarriesTheLastAnswer (internal/le/interoplab/output_test.go) asserts the same "last output:" suffix at the producer, for text and for a typed value, plus the 40-line bound, and goes red with the suffix removed. The pass-through cases it held (nil error, empty answer) have no producer left to test: Wait returns no error when ready, and a never-measured wait carries its probe error instead. |
