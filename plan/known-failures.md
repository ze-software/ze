# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" → pre-existing failures >10 min): logged, not blocking unrelated commits.


### 2026-06-10: routewatch QEMU integration tests flaky (netns roulette)

**Resolved 2026-06-10** (commit pending). Fixed in `routewatch`:
`Watcher.Start` now captures the caller's network namespace
(`captureNamespace`, `routewatch.go` / `routewatch_linux.go`) and passes it
as `RouteSubscribeOptions.Namespace`, so the subscription socket always opens
in the namespace of the goroutine that called Start. The subscribe loop also
resubscribes after the netlink library kills its receive loop on a transient
error (ENOBUFS/EAGAIN class) instead of dying silently for the process
lifetime. Tests replaced fixed sleeps with event polling (`eventRecorder.waitFor`).
Evidence: 5 consecutive QEMU iterations all green (45/45 PASS,
`tmp/qemu-routewatch-fix.log`). Original diagnosis kept below.

**Was Open (root cause known).** `TestIntegration_FanoutFromNetlink` and
`TestIntegration_RouteDelete` (`internal/core/routewatch/integration_linux_test.go`)
fail/pass run-to-run in the QEMU VM on identical code.

**Root cause: the netlink subscription socket lands in a random network
namespace.** `withNetNS` switches only the locked test thread into the fresh
netns. `Watcher.Start` (`internal/core/routewatch/routewatch.go:71`) spawns the
subscription goroutine, which the scheduler places on an arbitrary OS thread:
threads created before the netns switch are in the init namespace, threads
cloned from the locked thread after the switch inherit the test namespace.
When the subscriber lands in the init namespace it never sees the routes added
in the test namespace. Hard evidence from the failing run
(`tmp/qemu-routewatch.log`): the received event list contains the HOST
namespace routes (`10.0.2.0/24` QEMU SLIRP, `fe80::/64`, ...) from the
`ListExisting` dump, not the test netns routes.

**The logged "watcher error: Receive failed: resource temporarily unavailable"
(EAGAIN) is teardown noise, not the cause.** `defer w.Stop()` closes the
subscription socket while the vishvananda/netlink `Receive` is parked in the
poller; `NetlinkSocket.Receive` (v1.3.1 `nl/nl_linux.go`) checks the stale
`innerErr` (EWOULDBLOCK from the previous recvfrom attempt) before the poll
close error, so close-during-receive surfaces as EAGAIN. It happens on passing
runs too; `go test` only prints t.Logf output for failing tests.

**Fix sketch:** capture the caller's netns in `Watcher.Start` (callers in
tests run on the netns-locked thread) and pass it as
`RouteSubscribeOptions.Namespace` in `routewatch_linux.go subscribe()`.
No-op in production (all threads share the init netns).

**Related robustness gap (production, separate fix):** any `Receive` error
closes the library channel and `subscribe()` returns; the watcher dies
permanently with no resubscribe (`routewatch_linux.go:38-41`). Under route
churn, ENOBUFS would silently kill route watching for
`internal/plugins/fib/kernel/monitor_linux.go` and
`internal/plugins/kernel/kernel.go`. The tests' `time.Sleep` syncs should also
become poll-until-event.

**Reproduce:**
```
make ze-qemu-integration-test   # flaky; log seen in tmp/qemu-routewatch.log
```

### 2026-06-04 — `make ze-verify-wiring-docs` command validation drift

**Open (triage).** While verifying a rules-only change, `make ze-verify-wiring-docs`
failed in the `ze-doc-test` YANG/handler contract stage. Wiring passed and
documentation drift passed. Command validation reported two YANG commands with no
handler:

- `ze-show:gnmi` (`show > gnmi` in `ze-cli-show-cmd`)
- `ze-show:storage-smart` (`show > storage > smart` in `ze-cli-show-cmd`)

It also reported 15 local handlers with no YANG command: `crashes`, `crashes show`,
`debug disable`, `debug enable`, `debug show`, `doctor`, `explain`,
`generate wireguard keypair`, `help command`, `host`, `host show`,
`show config graph`, `skills`, `support`, and `validate config`.

**Root-cause hypothesis:** command registration/YANG schema drift in the CLI command
inventory, unrelated to rules documentation. Needs focused triage before treating
`make ze-verify-wiring-docs` as a clean baseline.

**Reproduce:**
```
make ze-verify-wiring-docs
```

Observed output artifact from the first run: `artifact://4`.

### 2026-05-31 — pppoe-client `no-default-route` rejected by config parser

**Resolved 2026-05-31** (commit pending). Fixed with a dedicated `TypeEmpty`
value type wired end-to-end: `yangTypeToValueType` maps `gyang.Yempty → TypeEmpty`
(`yang_schema.go`); `parseLeaf` accepts a bare presence flag, tolerating the
explicit `name true;` form (`parser.go`); `ValidateValue` / `ValueType.String`
cover it (`schema.go`, `valueTypeEmpty` constant in `constants.go`); the set-style
parser accepts the bare flag (`setparser.go`); the serializer emits a bare flag
that round-trips (`serialize.go`). Tests: `parser_type_empty_test.go` (9 cases:
parse bare/value/absent/ASI, serialize + nested-container round-trip, set-parser,
YANG-load `Yempty→TypeEmpty`, `ValidateValue`). Original diagnosis kept below.

**Was Open.** `test/parse/iface-netlink-accepts-pppoe-client.ci` failed. It is
`option=skip-os:value=darwin`, so it never ran on macOS and was first observed in
the QEMU Linux VM run. `ze config validate` rejects the bare `no-default-route`
flag:

```
configuration invalid: -
Errors:
  line 13: line 13: expected value for no-default-route, got SEMICOLON
```

**Root cause (confirmed):** `no-default-route` is `leaf no-default-route { type
empty; }` (`internal/component/iface/yang/ze-iface-conf.yang:1042`) — a valueless
flag. `Parser.parseLeaf` (`internal/component/config/parser.go:179`) is schema-aware
(it receives `node *LeafNode`) but unconditionally requires a `TokenWord`/`TokenString`
value (line 183-188), never checking the leaf's YANG type. So a bare `type empty`
leaf, where the next token is the statement terminator, hits "expected value for ...,
got SEMICOLON". Fix: in `parseLeaf`, when `node` is `type empty`, accept the leaf with
no value (store presence) instead of erroring. Any `type empty` leaf used as a bare
flag hits this, so add a unit test for the parser path plus the existing functional
test.

**Reproduce (linux-only; needs the cross-compiled linux bin/ze):**
```
make ze-qemu-debug NOBUILD=1 RUN='bin/ze-test bgp parse 91 -v'   # 91 = iface-netlink-accepts-pppoe-client (from --list)
# interactive: make ze-qemu-shell  then in VM: ./bin/ze config validate tmp/pppoe-test.conf
```

### 2026-06-04 — QEMU baseline triage (clean run, host load ~2.3)

**Triaged.** Clean re-run of `make ze-qemu-all-test` on a quiet host confirmed
that the original run's high timeout counts (54 plugin, 6 reload) were
host-load artifacts. The real failures are classified below.

**Suites passing clean:** encode (51/51), parse (221/221), decode (33/33),
editor (149/149), managed (13/13).

#### Fixed in this triage

- **UI 124-127 (service-help/status/uninstall/unit-gen) + install suite:** build
  bug. The functional test runner built ze with `zetest` only, but ze IS the
  distro binary. Fixed: `TestBuildTags()` now includes `ze_distro` so the test
  binary has install/uninstall/connect/systemd. QEMU cross-build updated to
  match.
- **UI 137 (ze-stripped-surface):** test bug. Expected `"self-update unavailable
  in ze-stripped"` but the message changed to `"minimal build"` in the
  `ze_stripped` -> positive build tag refactor. Fixed: updated expected string.
- **Plugin 198 (mpls-doctor):** test bug. Inline config block
  `remote { ip 10.0.0.2 }` missing semicolons. ASI only fires on newlines, not
  inline `}`. Fixed: added explicit semicolons.
- **Firewall VM crash (exit 255):** environment dependency. Even a single
  firewall test (009-set-element-timeout, per-element nft timeouts) crashes the
  Alpine QEMU kernel. Not parallel contention. Fixed: added `firewall` to
  `ZE_QEMU_SKIP_SUITES` default. Firewall tests need a real Linux host.

#### Product bugs (linux-only, need QEMU investigation)

- **L2TP 13 (session-cdn-teardown):** CDN for an established session with kernel
  state does not produce the "session destroyed" log. The `handleCDN` code path
  exists (`session_fsm.go:363`) but is not reached. Likely a control message
  sequence number or dispatch issue specific to the kernel-integrated session.
- **L2TP 15 (session-stopccn-cascade):** Kernel tunnel from test 13 leaks
  ("file exists" on genl create), and "StopCCN clearing sessions" log never
  appears. Two issues: kernel tunnel cleanup between tests, and a missing code
  path in StopCCN handling.

#### Environment dependencies (linux-only)

- **Plugin 355 (show-policy-routes):** nftables "operation not supported" error
  in the QEMU VM. The nft kernel module may not fully support the genl
  operations this test requires.
- **Reload 28, 29 (wireguard-invalid-bad-public-key, wireguard-invalid-no-private-key):**
  Linux-only (skip-os:darwin). Expected wireguard validation error messages not
  appearing. The wireguard key validation code path may not fire on the QEMU
  kernel configuration.

#### Host-load artifacts (confirmed not real)

- **Plugin timeouts (128, 199, 200, 398):** `exabgp-bridge-sdk`, `mpls-push`,
  `mpls-withdraw`, `text-handshake`. The non-linux ones (128, 398) pass on
  macOS. The linux-only ones (199, 200) timeout only in the VM.
- **Reload 19, 20 (config-apply-ordering-rotation/swap):** Pass on macOS. The
  three-way router-id rotation fails under VM timing but succeeds on a real
  host.

> Debug tooling: `make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp <suite>
> <idx> -v'` runs targeted tests verbosely in the VM; `make ze-qemu-shell`
> boots a persistent VM for interactive inspection.


### 2026-05-31 — host `make ze-verify` still has open BGP functional failures

**Open (triage).** A host `make ze-verify` run after the verb-first command
cutover got past lint, unit, race, and builds, but `ze-test bgp` is still not
clean end-to-end. The command-migration fallout was real and partially fixed:
`bin/ze-test bgp encode 50` (`watchdog`) is green again, and the focused plugin
rerun for the command-touched cases is green for ids `1, 21, 22, 108, 170, 172,
305, 308, 309, 310, 314, 322, 323, 324, 325, 326, 327, 328, 396, 398, 400`.
One touched case still times out: plugin test `2` (`adj-rib-in-replay-on-peerup`)
receives its expected UPDATE but never exits cleanly. The full `tmp/ze-verify-full.log`
still shows many additional pre-existing plugin failures and timeouts (`39, 42,
44, 50, 51, 52, 53, 79, 99, 103, 104, 105, 116, 121, 136, 137, 139, 140, 154,
155, 174, 189, 190, 201, 202, 203, 206, 207, 208, 241, 248, 252, 256, 274, 275,
276, 280, 286, 288, 291, 295, 297, 298, 299, 300, 301, 329, 330, 346, 347, 348,
349`), so unrelated commits should not be blocked on this baseline until the
suite is triaged.

### 2026-06-01 — host `make ze-verify` protocol run still fails functional and ExaBGP baselines

**Open (triage).** The verify debugging protocol run completed all seven top-level
stages and wrote `tmp/ze-verify-failures.log`. Lint, wiring/docs, evidence vet,
cached unit tests, and changed-group race tests passed. Remaining failures are:

- `ze-functional-test` plugin groups: `bfd` (`52`), `check` (`79`), `dispatch`
  (`121`), `fib` (`139`), `iface` (`174`), `loop` (`189,190`), `mpls`
  (`201,202`), `rib` (`288,291,289,290`), `show` (`347,356`), and `teardown`
  (`393,395`).
- `ze-exabgp-test` encoding group: `a` (`conf-watchdog`), reproduced by
  `uv run --with psutil --with paramiko ./test/exabgp-compat/bin/functional encoding --timeout 180 a`.

Evidence paths from that run: `tmp/ze-verify-failures.log`,
`tmp/verify/06-ze-functional-test.log`, and `tmp/verify/07-ze-exabgp-test.log`.

## Resolved

### 2026-05-31 — dispatch single-marshal + stale plugin lists (15 packages)

**Resolved 2026-05-31.** The 15 packages that began failing once `make ze-verify`
was runnable again (after the `tmp/go.mod` sentinel landed) are all green. Fixes,
by class:

1. **`single-marshal OnExecuteCommand` (commit 30b025270).** Command handlers now
   return structured `any`; the SDK marshals once. Tests that did
   `assert.Contains(t, data, "substring")` were comparing against a `map`/`[]byte`/
   typed slice (key/element match, never substring). Fixed by asserting on the
   marshaled JSON string: `adj_rib_in`, `healthcheck`, `rs`, `fib/kernel`,
   `fib/p4`, `fakeredist`.
2. **Stale registration / section lists.** Added the new plugins
   (`bgp-filter-aspath-length`, `flow-export`, `ldp`, `rsvp-te`) to the expected
   sets in `cmd/ze` and `internal/component/plugin/all`, and `platform` to the
   `cmd/ze/host` section list.
3. **Migration serializer keyword gap (commit 3da416d31).** The `internal` plugin
   keyword landed with updated goldens but `migrate_serialize.go` still emitted
   `external`. It now emits `internal` for built-in (`use`) plugins, `external`
   for `run` processes.
4. **Multi-line YANG descriptions.** `cmd/ze/completion/words.go` now collapses
   descriptions to their first line so shell completion stays one row per
   candidate; `internal/component/config/yang` description-propagation assertions
   updated to the enriched strings.
5. **CLI grammar catch-up (committed refactors 336cb2472 modes, 72d268c77 view
   consolidation).** `summary` doc lookup → `show summary` (canonical verb-first
   path); `option changes` is a display column, not a pipe redirect (only `blame`
   redirects); 7 `.et` files updated to the shipped grammar (`exit` switches
   config→operational, `show | blame` / `show | changes [all]` for views,
   `disconnect` requires an active session in completions).
