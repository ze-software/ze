# A decision reads state that the teardown has already reset

Cleanup that branches on "how far did this get" reads the state after a deferred
stop has put it back to its initial value, so the branch always takes one side.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-02 | rfc-verdict-fix-ospf (c29) | `startSessionForAdj` onDone (`internal/plugins/ldp/register.go`) | runSession defers onDone then Stop, so onDone reads the state Stop reset: every end calls `recordSetupFailure`, and an operational session never clears its backoff. onDone also emits SessionDown for a session that never sent SessionUp, re-arming the OSPF RFC 5443 stuck timer | runSession records that it reached operational and hands it to onDone, which clears the backoff and emits SessionDown only then; test both |
