---
kind: directive
level: MUST
stage:
---
Read the failure index before opening full logs or re-running.
After a suite or gate fails, the next test command MUST target
only the failing part: a single `.ci`/`.et` case, single Go test, single
package, or the stage-local `Rerun` command from the failure index. If there
are multiple failures, clear each one with its focused rerun. Only after all
focused reruns pass MAY you rerun the whole suite or gate as final
confirmation, except when the suite is the only available reproduction.
