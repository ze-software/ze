---
kind: directive
level: MUST
stage:
---
**`skip-os:value=darwin` MUST NOT be used as a substitute for `caps=`.** It hides a test from
macOS and therefore RUNS it, unprivileged, on the Linux CI runner, which is
exactly where it cannot pass. `test/plugin/resolve-ping.ci` carried a bare
`skip-os` and failed every CI run with `resolve ping status=error`, because
`doPingCtx` (`internal/component/ping/cmd/ping.go`) needs CAP_NET_RAW. If the
reason a test cannot run on macOS is a capability, you MUST declare the capability.
