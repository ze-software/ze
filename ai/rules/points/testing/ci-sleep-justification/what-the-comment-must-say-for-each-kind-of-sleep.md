---
kind: table
level:
stage:
---
| Kind | What the comment should say |
|------|-----------------------------|
| Bounded poll interval | Name the real condition the enclosing loop breaks/returns on ("poll interval; the loop above breaks when the nft table appears"). This is already a deterministic wait; the sleep is only its granularity. |
| Deliberate timer | The delay itself IS the behaviour under test ("the 3s verify hold IS the concurrency race window; do NOT convert"). |
| Timeout under test | The sleep waits out a fixed internal timeout that the test asserts ("the 5s vpp WaitConnected timeout IS the behaviour under test"). |
| needs-linux effect | A dataplane effect (tc/qdisc/nft/kernel FIB) with no readback in the driver, convertible only after a QEMU run ("needs-linux; no queryable signal that the qdisc was programmed"). |
| No readiness signal | The awaited effect exposes no queryable state to this driver ("backgrounded ze gets no ZE_READY_FILE marker; hold until OnConfigure emits the asserted log line"). |
