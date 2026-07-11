# 1106 -- gokrazy-l2tp-evidence-networking

Closes AC-3 of `fixit-appliance-evidence-config`: the real xl2tpd/pppd L2TP PPP
session against the QEMU gokrazy appliance now passes end to end (5/5 runs).
Getting there uncovered four independent blockers that had nothing to do with the
spec's two config bugs -- three in the test harness, one a real appliance-logging
defect.

## Context

The appliance code was correct throughout; every remaining AC-3 blocker was in
the evidence harness (`scripts/evidence/effective-gokrazy-l2tp-ppp.py`) or the
appliance's serial-logging path. A long "web-start hang" hunt turned out to be a
misdiagnosis: the web server was healthy and serving the whole time (goroutine
dump captured over SSH from a "hung" instance showed it parked in the normal
`serveAll`/`wg.Wait` serve loop), but its `web server listening` slog line never
reached the serial console the harness greps.

## Decisions

- **`printk.devkmsg=on` in `gokrazy/ze/config.json` KernelExtraArgs.** ze's
  primary log backend is kmsg (`ze.log.backend=kmsg,stderr`). The kernel's
  DEFAULT `/dev/kmsg` rate-limiter silently DROPS writes during the first-boot
  logging burst, so `web server listening` was lost ~half the time and the
  harness declared a false failure. Disabling the rate-limiter makes the
  appliance's kmsg logging reliable. This is the real fix; it also dissolved a
  "Heisenbug" -- every non-hanging run happened to carry `printk.devkmsg=on`
  (bundled with an in-process watchdog), so the watchdog looked like the
  variable when the kernel arg was.
- **Replace qemu user-mode (slirp) networking with a TAP+bridge.** qemu 8.2.2
  slirp does NOT deliver inbound UDP hostfwd to the guest (TCP hostfwd works;
  proven: SCCRQ reaches the host `0.0.0.0:1701` socket, appliance L2TP readLoop
  receives 0 datagrams). L2TP's LAC->LNS SCCRQ is inbound UDP, so slirp can never
  work for it. The appliance now attaches via a TAP enslaved to a bridge that
  also holds the LAC veth: pure L2, no NAT, appliance directly reachable.
- **dnsmasq (fixed MAC reservation) gives the appliance its underlay IP.** The
  appliance keeps `dhcp-auto`; a `--dhcp-host=<mac>,172.31.0.10` reservation lets
  xl2tpd target a known LNS address.
- **`prepare_instance` rewrites the rtr7/kernel replace to absolute**, mirroring
  the ze self-replace it already rewrote. `make ze-kernel` writes a RELATIVE
  replace that breaks once `builddir` is copied into the instance dir.

## Consequences

- AC-3 is green and reproducible from a clean checkout: `make ze-kernel` then the
  evidence run. New host dependency: `dnsmasq` (added to `require_cmd`).
- The harness now briefly toggles a `ufw allow in on <bridge>` rule (removed on
  teardown): ufw's default-deny INPUT drops the appliance's DHCP DISCOVER before
  dnsmasq (a host process) can see it. Bridged L2TP traffic bypasses netfilter
  (br_netfilter not loaded) and needs no rule.

## Gotchas

- **A dropped log line reads exactly like a hang.** The harness treats "did not
  see string X on serial" as failure; under kmsg rate-limiting that is a false
  negative. Detect liveness by state (SSH in, dump goroutines) before believing a
  "hang". In-process instrumentation perturbs timing and can mask the symptom --
  prefer zero-perturbation external observation (`ssh <appliance> "show system
  goroutines full"`).
- **xl2tpd 1.3.18 truncates `-c` config paths beyond ~90 chars** (drops the last
  4 chars -> ".conf" -> "unable to open config file"). Put its runtime files at a
  short temp path, not the deep repo `tmp/evidence/<mkdtemp>/`.
- **dnsmasq `--bind-interfaces` cannot receive the broadcast DHCP DISCOVER** (it
  binds the socket to the interface's unicast address). Use `--interface=X` with
  `--port=0` (DNS off) instead.
- **`IPv6 not supported by static pool` is NOT fatal** -- it is the reason string
  the appliance logs when gracefully declining IPv6CP ("continuing IPv4-only")
  against the IPv4-only proof pool. It had been listed in the harness
  `FATAL_NEEDLES`; removed.
- Fresh Linux bridge: set `forward_delay 0` / `stp_state 0` so newly-enslaved
  ports forward the early DISCOVER immediately.

## Files

- `gokrazy/ze/config.json` -- `printk.devkmsg=on`.
- `scripts/evidence/effective-gokrazy-l2tp-ppp.py` -- TAP+bridge+dnsmasq underlay,
  ufw hole, xl2tpd short-path, IPv6CP needle removal, kernel-replace rewrite,
  `require_cmd("dnsmasq")`.
