---
kind: directive
level: MUST
stage:
---
1. For the vendored gokrazy init, you MUST fetch the latest upstream `.mod` from the proxy, as in step 1 of the bump runbook, and note whether a newer commit carries security-relevant fixes. For ze's runtime kernel, you MUST read kernel.org's latest stable release; a bump changes `internal/appliance/kernel.version` and its `kernelSourceSHA256` entry (`internal/appliance/kernelbuilder/worker.go`, copied from kernel.org's `sha256sums.asc`) together, and the patch series MUST apply with no fuzz.
2. If a fix applies, you MUST run the bump runbook. If not, you MUST record the review date so the next reviewer knows the pins were checked, not forgotten.
3. You MUST re-confirm that the GPLv2 source-offer sign-off is still current.
