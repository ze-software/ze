# A listener refuses the peer its config names

The daemon runs and completes its work, but the listener the config declares
for a peer refuses that peer's connection, so the session never forms.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | spec-boot-record-after-listen | `test/reload/bgp-rib-maximum-paths-reload.ci`, the DUT's BGP listener | Red alone, on the memory store and with `option=storage:value=disk`: the test peer's 30 s dial (`dialTarget`, `internal/test/peer/peer.go`) gets `connection_refused` every try, while the staged fixture's SIGHUP reload completes. Not the store. The build carried other sessions' uncommitted `bgp/reactor` and `plugins/rib` hunks | open: cause not traced; rerun on a HEAD build, then read when the reactor binds the peer's listener |
