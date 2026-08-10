---
kind: directive
level: MUST
stage:
---
**YOU MUST READ THE WHOLE FAILURE SUMMARY BEFORE YOU RE-RUN.** A verify run ends with
`FAIL N verify stage(s) failed` and one line per failing stage, and
`tmp/ze-verify-failures.log` holds the same list. A re-run started from a partial
read costs another half hour and usually reports the same stages. Two specific
traps, both of which have cost a full run:
