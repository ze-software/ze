---
kind: note
level:
stage:
---
On 2026-08-02 a session left four `until ! pgrep ...; do sleep 5; done` loops
running on a machine that was also running QEMU, Docker and a full
`ze-verify`. The loops were started because a foreground `sleep` is refused, and
they were never stopped when the thing they watched changed. The wake-ups were
the contention that made the functional suites flaky for the rest of that
session.
