---
kind: table
level:
stage:
---
| Trigger | Action |
|---------|--------|
| `nil` check on a result | Check every other caller for the same nil case |
| Fallback when external system unavailable | Check every other caller of the dependency |
| Retry / backoff | Check every other caller doing the same I/O |
| New error-wrapping context | Check every other caller wrapping the same error |
| Replace direct call with helper | Check every other caller that should use the helper |
| Change/remove how a binary is invoked (launch form, positional, flag) | Grep EVERY invocation site of the bare token (`\bze <arg>`), not just the framework directive (`exec=ze`): `.ci` `exec=`, embedded `tmpfs=*.sh` bodies, helper `.sh`/`.py`, runner launch code, wrapper scripts, docs. Prove with the FULL suite, never a sample, only it runs the embedded launches (learned 1248) |
