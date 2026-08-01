# 1112 -- netlink .ci harness (Fix A readiness, Fix B netns+setcap, Fix C EOF)

## Context
The Linux-only netlink functional suites (firewall/policy/ospf/ospfv3) carry
`option=skip-os:value=darwin`, so they had never run on macOS and had never run on
Linux either. The first Linux run surfaced three harness defects -- Fix A (background
`ze` got no `daemon.pid`/`daemon.ready`), Fix B (the suites need `CAP_NET_ADMIN` and
either hard-fail unprivileged or reprogram the host firewall with caps), Fix C (the
python `ze_api` plugin busy-spun a core on connection EOF) -- plus a wave of
pre-existing, non-harness test brittleness that only the first real Linux run reveals.

## Decisions
- **Fix A**: one pure helper `zeReadyFileEnabled(mode, binName, tmpfsTempDir)` gates BOTH
  the `ZE_READY_FILE` env and the `daemon.pid` write, so arming and pid-write stay in
  lockstep for background `ze` (driver.py suites) as well as foreground.
- **Fix B**: enter a per-test netns by `LockOSThread` + `netns.NewNamed` on the test
  goroutine's thread; `ze`/`ze-peer`/driver.py are fork+exec'd from it and inherit the
  netns (assumption A-5, now a unit test). `ze` is dropped to a normal user via
  `SysProcAttr.Credential` off a `setcap`'d binary -- root breaks the readiness handshake
  because the readiness file is written after `dropPrivileges` (A-4, re-confirmed). The
  runner (root) creates the netns; only `ze` is dropped (driver.py/`ze-peer` stay root so
  they can read nft and signal the daemon).
- **R-2 host-safety gate**: `refuseHostNetnsFirewall` runs first in nft `(*backend).Apply`
  (the single kernel chokepoint before `conn.Flush()`). The runner passes the host netns
  inode via `ZE_TEST_NETNS_HOST`; if set and the current netns matches it, Apply refuses.
  Fails CLOSED (malformed/unreadable → refuse). Env is test-only → production untouched.
- **Fix C**: `ze_api` `_read_line`/`_read_tls_line` set `self._shutdown = True` on EOF.
- **nft-rendering brittleness → version-robust assertions**: convert brittle
  `expect=stdout:contains=` to `expect=stdout:pattern=` alternations rather than pin one
  nft version. This preserves the behavioral check (the rule still has to be programmed)
  while tolerating how nft prints it. User-chosen scope over fixing ze lowering.

## Consequences
- New `internal/test/runner/netns_linux.go` (+`netns_other.go` stub) and
  `internal/plugins/firewall/nft/host_netns_guard_linux.go`. `runOrchestrated` gained a
  flag-gated netns entry, a `chownTree` of the daemon's tmpfs/config dir, a
  `SysProcAttr.Credential` drop, and the `ZE_TEST_NETNS_HOST` env. Default path is
  byte-identical (all gated on `ZE_TEST_NETNS`).
- `make ze-netns-test` (real Linux host: setcap + `sudo ZE_TEST_NETNS=1` per suite +
  host-nft-unchanged assertion) and `make ze-netns-qemu-test` (macOS via QEMU;
  `scripts/evidence/netns_qemu.py`). Host-safe firewall subset is 15/15 green in QEMU.

## Gotchas
- **Netns teardown must live in ONE `t.Cleanup`**, in order (`Set(orig)` → close both
  handles → `DeleteNamed` → `UnlockOSThread`). A separate `defer origNS.Close()` /
  `defer runtime.UnlockOSThread()` runs BEFORE the Cleanup, so `netns.Set` then operates
  on a closed fd (EBADF) or an unlocked (possibly migrated) thread. Mirror `withNetNS`.
- **`setcap` fails on the 9p workspace mount** (`security_model=none` → no xattr). In QEMU
  copy the binary to a tmpfs dir first -- and that dir must be **world-traversable** (not
  `/root`, 0700), because the credential-dropped `ze` (uid 1000) `execve`s it after setuid.
- **A credential-dropped `ze` must own its workdir.** `MkdirTemp` is 0700-root; chown the
  tmpfs dir AND the tmpfs-less `ze-config-*` dir (the offline `ze config validate -` path)
  to the target uid, or the daemon gets EACCES on its own config (006 red→green).
- **`ze` state/socket dir is derived from the binary path** (`paths.ConfigDirFromBinary`:
  a `bin/` layout → `etc/ze`). An absolute non-`bin/` path returns "" → `ze cli` cannot find
  the daemon. Pin `ze.config.dir` when the binary is not in a `bin/` layout (QEMU).
- **`startWithETXTBSYRetry` must copy `SysProcAttr`** onto the recreated `Cmd`, else a
  retry silently re-runs `ze` as root and breaks the drop.
- **Attribution technique**: to prove a suspected netns regression is actually
  pre-existing, re-run the failing test with `ze` as ROOT (no `ZE_TEST_NETNS`). `policy`'s
  `firewallnft: flush: operation not supported` reproduced identically as root → an Alpine
  QEMU kernel nft-feature gap, not Fix B. `copp-*` "firewall backend not loaded" traced to
  a config with no `firewall { backend nft }` block. Read the producer, don't infer.
- **nft 1.1.1 rendering** (Alpine 3.21): `tcp dport`→`th dport`, `ip saddr @set`→
  `@nh,96,32 @set`, `ip daddr @set`→`@nh,128,32 @set`, `ip dscp set`→`@nh,8,8 set`,
  `icmp type 8`→`echo-request`. A LITERAL `ip saddr <addr>` still renders cooked -- only
  set-lookups and protocol-less transport-port matches go raw. A `contains=` assertion
  that omits the proto prefix (`sport @set`) survives as a substring of `th sport @set`.
- **009-set-element-timeout crashes the Alpine QEMU kernel** on nft set-element-timeout
  ops (why `ZE_QEMU_SKIP_SUITES` defaults to `web,firewall`). Skip it in QEMU subsets.
- **nushell `cd` persists across Bash tool calls** and drifts later relative paths -- use
  absolute paths, never `cd a && b`. The per-session hook `sid` also drifts (getppid),
  so `.source-read-<sid>` / `session-state-<sid>.md` may need symlinking to the fallback.

## Files

None recorded.
