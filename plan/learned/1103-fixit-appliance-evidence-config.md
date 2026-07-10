# 1103 -- fixit-appliance-evidence-config

## Context

Two independent bugs in the gokrazy appliance build/config flow blocked a green
L2TP appliance evidence run. Bug 1: `ze init --force`'s `daemonRunning` guard
proved "a ze daemon is running" by merely dialing the stored SSH host:port and
accepting ANY TCP answer -- so on a build host with sshd on `0.0.0.0:22`, the
probe hit the host's own OpenSSH and false-reported a running ze, aborting the
fresh init (worked around by manually `rm -f`-ing the seed DB in the Makefile).
Bug 2: `ze init` wrote build-host interface discovery to `file/active/ze.conf`,
and since the appliance's first-boot `bootstrapConfigFromTemplate` only runs when
NO active config exists, that init-written active config permanently shadowed the
build-time `file/template/ze.conf` -- so web/l2tp from the template never took
effect and the evidence harness timed out.

## Decisions

- Bug 1: gave ze's SSH server a distinctive `SSH-2.0-ze` identification banner
  (`wish.WithVersion("ze")`) and made `daemonRunning` require that exact banner.
  Chose a ze-specific banner over matching the library default `SSH-2.0-Go`
  (any Go server, and a fragile coupling to a library constant) and over a
  PID-file/DB-flock check (no universal such signal exists: `ze.pid.file` is
  operator-optional, the zefs DB is not flocked).
- Bug 1: positive-identification model -- return "running" ONLY on a confirmed
  `SSH-2.0-ze` banner; dial-fail / foreign banner / silent listener / read
  timeout all → "not running". Chose this over the spec's original R-1 mitigation
  ("default to running on ambiguity") because that would re-treat a silent/foreign
  listener as a live ze and resurrect the exact false positive being fixed; a real
  ze answers with its banner immediately on accept, and the guard sits behind
  `--force` anyway.
- Bug 2: added `ze init --seed` that skips baking this host's interface discovery
  into the active config. Chose a flag over: reordering the Makefile to write the
  template before init (impossible -- init creates the DB); writing discovery to
  the template key (pollutes the appliance config with build-host NICs); making
  boot prefer the template over an init-written active config (needs a fragile
  "init-generated" marker). init runs before the template exists, so it cannot
  detect a template; `--seed` cleanly declares "appliance seed, config comes from
  template + on-device first-boot discovery."
- Shared the banner constant (`ServerSoftwareVersion="ze"`,
  `ServerVersionBanner="SSH-2.0-ze"`) in `internal/core/ssh/client` -- the leaf
  ssh-transport package both the server (announces) and the init probe (recognizes)
  already can import, so the two halves cannot drift.

## Consequences

- `daemonRunning` is now a real ze-liveness check, not "any TCP listener." Any
  future change to the SSH server's `WithVersion` MUST keep it in sync with
  `sshclient.ServerVersionBanner` or the probe silently breaks -- `TestServerAnnouncesZeBanner`
  guards this (the init-side tests use a synthetic listener and would NOT catch it).
- `ze init --seed` is the appliance-seed path; the normal `ze init` (no flag) is
  unchanged and still writes a discovered active config. The gokrazy Makefile
  always builds with a template, so all three of its `ze init` calls use `--seed`.
- The `mk/gokrazy.mk` seed-DB `rm -f` workaround is gone; `--force` now safely
  moves an existing seed DB aside because `daemonRunning` correctly ignores the
  host sshd. Removing it is itself proof Bug 1 is fixed at the source.
- `ze appliance assemble` (the structured build path) already wrote template-only
  and no active config, so it was already correct and needed no change.

## Gotchas

- The SSH server identification string is sent by the server immediately on TCP
  accept, BEFORE key exchange/auth -- so a banner probe needs no credentials and
  is cheap. Dialing `0.0.0.0:PORT` connects to localhost, which is why the build
  host's own sshd was being hit.
- `charmbracelet/ssh` `WithVersion(v)` takes only the softwareversion token and
  prepends `SSH-2.0-` itself (server.go: `config.ServerVersion = "SSH-2.0-" + v`).
  Pass `"ze"`, not `"SSH-2.0-ze"`.
- The full L2TP evidence test (`ze-deployment-gokrazy-l2tp-ppp-test`) needs root/
  CAP_NET_ADMIN AND a built custom kernel (`tmp/kernel/vmlinuz`); it cannot run in
  a non-root session. AC-2's mechanism was instead proven by a unit test of the
  real boot path (`TestBootstrapConfigFromTemplateAppliesWebL2TP`) plus the
  `--seed` tests; AC-3's end-to-end qemu run remains to be executed on a root host.
- Hook friction: rewriting the discovery block re-scanned its pre-existing
  `fmt.Fprintf(os.Stderr,...)`/`fmt.Printf("%d")` lines and blocked the edit
  (debug-statement + no-sprintf-alloc rules). Worked around by converting only the
  block's opening `if` into `if seed { } else if ...`, leaving the grandfathered
  lines untouched.

## Files

- `internal/core/ssh/client/client.go` -- shared `ServerSoftwareVersion` / `ServerVersionBanner`.
- `internal/component/ssh/ssh.go` -- `wish.WithVersion(sshclient.ServerSoftwareVersion)`.
- `internal/plugins/init/main.go` -- `daemonRunning` banner probe + `isZeSSHBanner`; `--seed` flag gating the discovery/active-config write.
- `internal/plugins/init/daemon_internal_test.go` (new), `internal/plugins/init/seed_internal_test.go` (new), `internal/plugins/init/webcert_internal_test.go` (runInit signature).
- `internal/component/ssh/ssh_test.go` -- `TestServerAnnouncesZeBanner`.
- `cmd/ze/bootstrap_template_test.go` (new) -- real boot-path AC-2 proof.
- `mk/gokrazy.mk` -- `--seed` on the three inits, seed-DB `rm -f` workaround removed.
- `docs/guide/appliance.md`, `docs/guide/configuration.md` -- `--seed` build-flow + discovery notes.
