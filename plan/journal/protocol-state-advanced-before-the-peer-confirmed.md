# A protocol state advances before the peer has confirmed it

A state machine moves on when its own half of an exchange is done, not when the
peer's half has arrived. Everything keyed to the new state then runs early.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-28 | rfc-verdict-fix-routing | `handleInit` (`internal/plugins/ldp/session.go`), `startSessionForAdj` (`register.go`) | RFC 5036 §2.5.3 item 2.d makes a session operational only after an acceptable Initialization AND a KeepAlive. handleInit goes operational on the Initialization alone, and startSessionForAdj emits SessionUp right after TCP connect. No rfc5036 row gates items 2.c or 2.d. | fixed: `initAccepted` (`internal/plugins/ldp/session.go`) gates OPERATIONAL on the peer KeepAlive; `rfc5036.md` carries `RFC5036-2.5.3-5` to `-9`, verified at HEAD 5762f6f2be |
