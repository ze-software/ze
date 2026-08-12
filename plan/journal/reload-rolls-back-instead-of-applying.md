# Reload rolls back instead of applying

A SIGHUP reload runs its rollback path. The change the operator asked for is not
applied. The log reports the rollback and not the refusal that caused it, so the
test that asserts the applied behavior fails on a missing line. A missing line
says what did not happen. It does not say why.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-12 | - | config.transaction | `test/reload/reload-import-policy-no-bounce.ci` fails deterministically, on a suite run and on two single runs. The reload logs `plugin broken during rollback, restarting` for bgp-healthcheck, bgp-hostname and bgp-filter-community, and never logs `peer settings swapped in place` (`internal/component/bgp/reactor/reactor_api.go`, `reconcilePeersJournaled`). Not caused by the plugin-only-boot work: the only Go changes in that tree were the cmd/ze config routing and `internal/component/config/probe.go`, none on the BGP reload path | not fixed, not diagnosed |
