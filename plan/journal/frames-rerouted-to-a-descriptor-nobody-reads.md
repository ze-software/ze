# The kernel reroutes frames to a descriptor nobody reads

A kernel interface delivers one kind of frame on more than one descriptor and
switches between them on a state change the daemon itself makes. The daemon
reads the descriptor it started with and only holds the other one open, so
after the switch those frames queue where nobody reads them, and the peer's
packets go unanswered with no error and no log line.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | spec-ppp-l2tp-aaa-one-off-defects | l2tp ppp NCP after PPPIOCCONNECT | Once the channel is connected Linux queues IPCP and IPv6CP on the unit (`ppp_input`, `ppp_receive_nonmp_frame`), and Ze read only `chanFile`, so post-Opened NCP packets went unanswered (interop `ppp-code-reject-xl2tpd`) | fixed: `readUnitFrames` feeds the unit to the session loop, `awaitUnitDrained` drops the ended lifetime's frames before a reconnect, and the PPPoE client reads its unit; the proofs are in the fix commit |
