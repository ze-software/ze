---
kind: directive
level: MUST
stage:
---
1. For the vendored gokrazy init and `rtr7/kernel`, you MUST fetch the latest upstream `.mod` from the proxy (as in "The fix" above) and note whether a newer commit carries security-relevant fixes.
2. If a fix applies, you MUST run the bump runbook above. If not, you MUST record the review date so the next reviewer knows the pins were checked, not forgotten.
3. You MUST re-confirm the GPLv2 source-offer sign-off below is still current.
