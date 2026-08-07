---
kind: note
level: MUST NOT
stage:
---
Python observer plugins inside `tmpfs=*.run` blocks MUST NOT use the
`dispatch(api, 'daemon shutdown') ; sys.exit(1)` pattern to signal failure.
The runner only watches ze's exit code, and ze has already exited 0 from the
clean shutdown by the time the observer's `sys.exit(1)` runs. The test passes
silently. The cmd-4 fix (`1fc98747`) removed three such false-positives.
