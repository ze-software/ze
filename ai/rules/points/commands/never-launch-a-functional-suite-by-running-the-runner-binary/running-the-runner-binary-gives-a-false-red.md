---
kind: note
level:
stage:
---
Running a raw `ze-test` binary is not equivalent to `./le functional <suite>`.
The native action builds the isolated tagged pair, sets `ZE_BIN` and
`ZE_TEST_BIN`, and owns the scratch environment. Bypassing it can produce a convincing false red.
