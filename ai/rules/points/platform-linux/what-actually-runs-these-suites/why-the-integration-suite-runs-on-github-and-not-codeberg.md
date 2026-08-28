---
kind: note
level:
stage:
---
Native integration actions run on GitHub because their suites need
`CAP_NET_ADMIN` or `CAP_NET_BIND_SERVICE`. GitHub jobs can run the action with
the required privileges; the shared Codeberg runner cannot grant them.
It is advisory-first (`continue-on-error: true`): a red suite reports without
marking the run failed, until a green baseline lets it flip to blocking.
