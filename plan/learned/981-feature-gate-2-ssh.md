# 981 - feature-gate child 2: ssh compile-out (extract-then-gate, dedicated seam)

## Context

Child 2 of the feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`): make
ssh compile-out-able via `ze_ssh`, the headline hardening target. Unlike the lg
listener pilot (980), ssh did not fit the construction registry: it is built inside
the shared daemon-startup function `infraSetup`, interleaved with always-on infra
(AAA bundle, command authorization, accounting, reboot/GR marker), AND in a second
no-`bgp{}` path in `main.go`; it also carries the whole interactive CLI-over-ssh
session model (`session_factory.go`). ssh has NO dependency on routing -- `infraSetup`
lives behind `bgpconfig.SetInfraHook` only because that is where the generic
daemon-startup hook API happens to sit (a naming/packaging artifact).

User decision: **extract ssh into its own module first (behavior-preserving), then
gate it.** Two phases so the risky AAA/ssh untangle is validated by the full
functional suite before the compile-out is introduced.

## Decisions

- **Dedicated `ze_ssh` seam, not the construction registry.** `cmd/ze/hub/ssh_infra.go`
  (always-on) declares an opaque `sshServer` handle (`Address()` only) and three
  nil-able hook vars -- `sshBuild`, `sshWirePostStart`, `sshBuildStandalone` -- plus
  `setSSHInfra`. The hub calls the seam; with `ze_ssh` off the vars are nil and ssh is
  skipped. Generic input structs (`InfraHookParams`, AAA, audit, storage) cross the
  seam; never a `zessh` type. The umbrella's "one registry" is refined: listener
  services use the registry; ssh uses a seam.
- **AAA stays always-on; only ssh construction/wiring moves.** `infraSetup` keeps the
  AAA bundle / authorization / accounting / reboot / GR-marker (MCP/API need them with
  ssh absent); the ssh `zessh.Config`+`NewServer`+`SetSessionModelFactory` and the ~9
  post-start `Set*` closures moved to `service_ssh.go` (`//go:build ze_ssh`). The
  no-`bgp{}` path in `main.go` keeps its AAA build always-on and passes the resolved
  authenticator into `sshBuildStandalone`.
- **Two construction paths, two seam entries.** The hook path uses `sshBuild` +
  `sshWirePostStart` (full executor/monitor/streaming/reboot wiring); the no-`bgp{}`
  path uses `sshBuildStandalone` (lighter: session model + a dispatch executor + start
  + a returned shutdown func). `session_factory.go` (the interactive model) and its
  tests are gated `//go:build ze_ssh`.
- **ze-stripped keeps ssh (user decision).** ssh is the base operator management
  plane, so `make ze-stripped` is `ze_core ze_ssh` (drops lg/web/etc, keeps ssh). The
  fully-hardened no-ssh build is a deliberate bare `go build -tags ze_core`, proven by
  `TestBuildTag_SSH_Absent` + a go-tool-nm symbol check -- not by the ze-stripped target.
- Four-place tag wiring (as 980): `ZE_FEATURES` (Makefile), `TestBuildTags()`
  (runner.go), `.golangci.yml` build-tags, `featureTags` (generator). `dep_audit`
  `DISABLEABLE` += `internal/component/ssh -> ze_ssh`.

## Consequences

- `go tool nm`: bare `ze_core` build links **0** `internal/component/ssh` server
  symbols; `ze_core ze_ssh` links 118. Default `ze`/`ze-appliance`/`ze-stripped` keep ssh.
- The seam pattern is now the template for services that don't fit the listener
  registry (deeply wired into shared startup). web is still a listener-registry case.
- `infraSetup` returns the opaque `sshServer` interface; its regression test
  (`infra_setup_test.go`, gated `ze_ssh`) type-asserts back to `*zessh.Server`.

## Gotchas

- **The AAA build in BOTH paths must stay always-on.** Moving it into the gated seam
  would drop MCP/API auth in a no-ssh build. Only the resolved authenticator crosses
  the seam (set via `inputs.Authenticator = aaaBundle.Authenticator`, which also keeps
  `main.go` free of an `aaa` import).
- **A unit test that requires ssh must skip (or gate) under `ze_core`.**
  `TestEphemeralDaemonStartsSSH` got a `// test-relax:` guard (`if sshBuildStandalone
  == nil { t.Skip }`); `session_factory_test.go` + `infra_setup_test.go` are gated
  `//go:build ze_ssh`. The default unit suite runs `-tags ze_core`, so the present-tag
  tests only run under a `ze_ssh` unit build.
- **A functional test that drives the CLI over ssh into the stripped binary breaks if
  ssh is removed from stripped.** `test/ui/ze-stripped-surface.ci` ssh-es into
  ze-stripped; the resolution was to keep ssh in ze-stripped (the management plane),
  not rework the test. A truly no-management-plane build has no remote way to drive
  the CLI -- a real constraint, not a test bug.
- **Heavy seam-input structs trip `hugeParam` (>280 bytes)**: `sshBuildInputs` embeds
  `InfraHookParams` (424 bytes) -- pass seam inputs by pointer.
- Same no-sprintf-alloc trap as 980: use `textbuf` for the monitor id, not
  `fmt.Sprintf`, when moving code into a freshly-edited file.

## Files

- `cmd/ze/hub/ssh_infra.go` (new) - the seam: `sshServer`, input structs, hook vars, `setSSHInfra`
- `cmd/ze/hub/service_ssh.go`, `register_ssh.go` (new, `//go:build ze_ssh`) - build/wire/standalone impls + seam install
- `cmd/ze/hub/session_factory.go` (now `//go:build ze_ssh`) - interactive ssh session model
- `cmd/ze/hub/infra_setup.go`, `main.go` - call the seam; AAA stays always-on; no `zessh` import
- `cmd/ze/hub/{build_tag_ssh_present,build_tag_ssh_absent}_test.go` (new); `infra_setup_test.go`, `session_factory_test.go`, `main_test.go` (gated/guarded)
- `scripts/codegen/plugin_imports.go` (featureTags += ssh/yang), `internal/component/plugin/all/all_ze_ssh.go` (generated), `all.go` (ssh/yang removed)
- `scripts/dev/dep_audit.py` (DISABLEABLE += ssh), `Makefile` (ZE_FEATURES += ze_ssh; ze-stripped keeps ssh), `internal/test/runner/runner.go` (TestBuildTags += ze_ssh), `.golangci.yml` (+ ze_ssh), `ai/rules/architecture.md`, `docs/features.md`
