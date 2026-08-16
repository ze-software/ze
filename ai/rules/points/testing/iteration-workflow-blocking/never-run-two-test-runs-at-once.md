---
kind: directive
level: MUST
stage:
---
**Overlapping runs:** If a test run is failing, MUST kill it before starting another. MUST NOT run `make ze-precommit-verify` twice concurrently.
