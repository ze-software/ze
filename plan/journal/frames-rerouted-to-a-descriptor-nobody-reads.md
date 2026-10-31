# The kernel reroutes frames to a descriptor nobody reads

A kernel interface delivers one kind of frame on more than one descriptor and
switches between them on a state change the daemon itself makes. The daemon
reads the descriptor it started with and only holds the other one open, so
after the switch those frames queue where nobody reads them, and the peer's
packets go unanswered with no error and no log line.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | spec-ppp-l2tp-aaa-one-off-defects | l2tp ppp NCP after PPPIOCCONNECT | In interop `ppp-code-reject-xl2tpd` IPCP Code 12 drew no answer. Linux `ppp_input` queues a frame on the channel only when `proto >= 0xc000` once a unit is connected, and `ppp_receive_nonmp_frame` puts IPCP and IPv6CP on the unit's queue. Ze connects after NCP (`session_run.go`) and reads only `chanFile` (`readFrames`); `unitFile` is never read, so a post-Opened IPCP Terminate-Request is lost | open: read the unit descriptor into the session loop, and add the IPCP 5.6-2 and 5.6-3 assertions to that scenario |
