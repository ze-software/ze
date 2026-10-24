# A recovery that tears down more than the error names

A handler answers a narrow, recoverable error by closing the whole enclosing
session, where the protocol names a narrower recovery. The wider teardown
works, so no test goes red, but every other SA, route or tunnel riding that
session drops with it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-24 | - | IKE Child SA rekey, `handleCreateChildSAOwned` (`internal/component/ike/engine/inbound.go`) | CHILD_SA_NOT_FOUND on our Child SA rekey takes the reestablish exit and closes the whole IKE SA. RFC 7296 Section 2.25: the peer "SHOULD silently delete the Child SA (if it still exists) and send a request to create a new Child SA from scratch" | open: found reviewing 34ad745067; the behavior predates it |
