# A check probes a peer that its own teardown step stopped

The check ends a peer's process to drive a teardown, then measures the cleanup
inside that peer. When the process is the container's PID 1, the container
exits with it, so every probe fails and the wait expires without one reading.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | - | `checkPPPIPv4`, `waitL2TPClean` (`internal/le/interoplab/l2tp/checkers.go`) | 01-ppp-ipv4 (nightly 39015310430): TERM to the LAC hit xl2tpd as PID 1, the container exited, and every cleanup probe answered `container is not running` | fixed: a shell is the LAC's PID 1 and the check sends TERM to xl2tpd alone, so the LAC stays up to be measured; 01 passes under an le built from the tree (`./le --name`), a stale bin/le ran the old check |
| 2026-10-17 | - | `checkSecondDialAtCapIsRefused` (`internal/le/interoplab/pppoe/check_padr_replay.go`) | pppoe-padr-replay (nightly 39015310430): cleanup `pkill -x pppd` also stopped the first dial's pppd, its session closed, and the check read 0 sessions as a failed per-MAC cap | fixed: the over-cap pppd carries `ipparam ze-over-cap-dial` and only it is stopped (`pkill -f`); `TestOverCapDialCleanupSparesTheLiveSession`; docker scenario red with `-x pppd` restored (client PADT 5s after the refusals), green with the fix |
