---
kind: note
level:
stage:
---
`ze-integration-test` runs here now, which it could not on Codeberg: its six
suites need `CAP_NET_ADMIN` / `CAP_NET_BIND_SERVICE` (`mk/test-integration.mk`),
and Woodpecker's only lever for that (`privileged: true`) is a BLOCKING lint
error on an untrusted shared instance that aborts the whole pipeline. On GitHub
the job simply runs under `sudo` as root, which has those capabilities natively.
It is advisory-first (`continue-on-error: true`): a red suite reports without
marking the run failed, until a green baseline lets it flip to blocking.
