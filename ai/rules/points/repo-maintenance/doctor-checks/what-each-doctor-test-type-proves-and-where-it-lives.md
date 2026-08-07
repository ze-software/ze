---
kind: table
level:
stage:
---
| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when the relevant config block is present and emits the registered code | Owning package next to the registration, or `internal/component/doctor` only when there is no narrower owner |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point | `internal/component/doctor` or the existing functional test suite for the user entry |
