---
kind: note
level:
stage:
---
Python plugins embedded in `.ci` tests via `tmpfs=*.py` can import `ze_api` for
the 5-stage plugin protocol and runtime assertions. Key functions:
