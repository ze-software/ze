# Environment Variables

## Meta

| Field | Value |
|-------|-------|
| Name | Environment Variables |
| Page | docs/guide/environment-variables.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/environment.go, internal/component/config/apply_env.go, cmd/ze/hub/pidfile.go |
| Real-path tests | test/ui/cli-env-list.ci, test/ui/cli-env-get.ci, test/ui/cli-env-get-log-subsystem.ci, test/ui/env-pid-file.ci, test/ui/env-pprof-serves.ci, test/ui/env-cli-format.ci |
| Docs | docs/guide/environment-variables.md |
| Doc review | 2026-10-07: each named key is read in Go: ze.user (core/privilege), ze.pid.file (config/environment.go), ze.pprof, ze.bgp.openwait and ze.bgp.announce.delay (bgp/reactor), ze.cli.format (command/pipe.go) |
| Defect review | 2026-10-07: no open spec or journal row found naming the env surface |
| Extra criteria | supported: ze.pid.file effect = test/ui/env-pid-file.ci; supported: ze.pprof effect = test/ui/env-pprof-serves.ci; supported: ze.cli.format effect = test/ui/env-cli-format.ci; supported: ze.user effect = none yet; supported: ze.bgp.openwait effect = none yet; supported: ze.bgp.announce.delay effect = none yet |

## Description

Ze-native env surface: `ze.user`, `ze.pid.file`, `ze.pprof`, `ze.bgp.openwait`, `ze.bgp.announce.delay`, `ze.cli.format`; ExaBGP-compat env keys retired 2026-04 <!-- source: internal/component/config/environment.go -- env var registrations --> <!-- source: internal/component/config/apply_env.go -- ApplyEnvConfig --> <!-- source: cmd/ze/hub/pidfile.go -- writePIDFile, removePIDFile --> <!-- source: internal/exabgp/bridge/bridge_ack.go -- exabgp.api.ack -->
