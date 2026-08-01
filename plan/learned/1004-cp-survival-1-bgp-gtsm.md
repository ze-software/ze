# cp-survival-1-bgp-gtsm

BGP GTSM / TTL-security (RFC 5082) wired from `connection > ttl > {max,set,min}`
into `PeerSettings.{OutTTL,MinTTL}` and enforced with kernel TCP socket options
(`IP_TTL`/`IPV6_UNICAST_HOPS` out, `IP_MINTTL`/`IPV6_MINHOPCOUNT` in). `max N`
derives OutTTL=255, MinTTL=256-N. Platform split `ttl_linux.go`/`ttl_other.go`
mirrors the MD5 precedent; darwin returns unsupported (logged Debug, non-fatal).

## The load-bearing trap: SYN-ACK comes from the listen socket

Setting the per-peer TTL in `connectionEstablished` (post-`accept()`) is **not
enough**. When a GTSM peer *initiates* the connection, the kernel emits the
SYN-ACK from the **listen socket** before the application accepts, so it carries
the default TTL (64). A peer running GTSM (e.g. FRR `ttl-security`) has
`IP_MINTTL=255` on its connecting socket and silently drops that SYN-ACK. The
session then only comes up if *we* are the active connector, after connect-retry
backoff (so it looks like ">90s to establish, then works").

Fix: set the outgoing TTL on the **listen socket** too
(`RealListenerFactory.ListenTTL`, computed as max OutTTL across GTSM peers on the
port via `reactor.listenTTLForListener`). The listen socket is shared, so its
SYN-ACK TTL is global per port — 255 is benign for non-GTSM peers (they do not
gate inbound TTL). The accepted child inherits `IP_TTL` from the listener; the
per-peer value is still re-applied post-accept. This was the spec's deferred
"phase 2"; it is **not optional** for real interop.

## How it was found (and why unit/functional tests missed it)

Unit + functional (`bgp-gtsm.ci`) tests passed because the `ze-peer` test peer
does **not** enforce TTL — they only prove config→settings→socket wiring, not
that a GTSM peer accepts our packets. Only the **FRR interop scenario**
(`test/interop/scenarios/46-gtsm-frr`, ze `ttl { max 1 }` vs FRR `ttl-security
hops 1`) exercised a real receive-side GTSM gate and exposed the SYN-ACK bug. The
smoking gun was `tcpdump` on the FRR container: `ze -> FRR SYN-ACK ttl 64`.
Lesson: for transport-security features, an interop test against a daemon that
*enforces* the mechanism is the only test that actually validates it.

## Traps for the next agent

- **Always run Ze tests with build tags.** A bare `go test ./internal/component/bgp/config/...`
  spuriously fails ssh/authz schema tests because `all_ze_ssh.go` is `//go:build ze_ssh`
  and the feature-gated schema is absent. Use `-tags 'ze_core ze_ssh'` (or the suite tag set).
  Do not report such failures as pre-existing reds without re-running with tags.
- **Integration tests must be registered to run.** `//go:build integration && linux`
  files run nowhere unless their package is in `scripts/evidence/qemu-all-tests.sh`
  `integration_pkgs` (and a `mk/test-integration.mk` target). Writing the file is not enough.
- **Debugging container TTLs:** the quay FRR image has no `tcpdump`/`ss`; `apk add tcpdump`,
  or read `/proc/net/tcp` (port 179 = `:00B3`). Raise the interop `SESSION_TIMEOUT` to keep
  containers alive for live `docker exec` inspection; default teardown is immediate.
- **Conflict validation is in Go, not YANG.** `ttl max` + `set`/`min` is rejected in
  `reactor/config.go parseTTLSettings` and `bgp/config/resolve.go validateDynamicGroupTTL`,
  both reached from `cmd_validate.go` — so it fires at `config validate`, no YANG `must` needed.

## Commit-hygiene note

The `OutTTL` *consumer* in `session.go` was committed early (in `6e084eb6f`, an
unrelated family-filter commit) without its field definitions, so commits
`6e084eb6f`..`c121f84f8` do not build in isolation. The closure commit lands the
field defs as a forward commit (tree builds from there on). When splitting a
feature across commits, the producer (struct field) must land no later than its
first consumer, or `git bisect` breaks.

## Files

None recorded.
