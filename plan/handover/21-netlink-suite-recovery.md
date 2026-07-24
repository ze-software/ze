# Handover 21 -- netlink functional suites: recovery and remaining tail

Written 2026-07-23. For an agent picking this up **on a different machine**, so
every environment assumption is spelled out rather than implied.

## Rationale (what was agreed and why)

- `test/firewall`, `test/policy`, `test/ospf`, `test/ospfv3` were failing almost
  wholesale. They were NOT a protocol regression. Nearly every failure was the
  harness erasing its own setup, or a daemon built without the features under test.
- The owner's steer was decisive: *"all these tests were passing and the hardware
  was either this machine or my macbook"*. That is what led to `make ze-netns-test`,
  a purpose-built gate that had itself been broken. Do not re-derive this.
- Two real product bugs were found in policy routing along the way. Both would
  break a live deployment, not just tests.
- Scope was deliberately NOT widened to the `bgp plugin` suite; that stays with
  `plan/spec-fixit-peer-verdict-and-forward-rail`.

## The one thing to know first: which gate runs these suites

**`make ze-netns-test`** (`mk/test-integration.mk:137`). It runs
`firewall policy ospf ospfv3`, each test in its own throwaway network namespace,
with `bin/ze` setcap'd `cap_net_admin,cap_net_raw,cap_net_bind_service+ep`, and
asserts the host's `nft list tables` is byte-identical before and after.

| Host | What you can run |
|------|------------------|
| Linux + passwordless sudo + `setcap` (libcap2-bin) + `nft` (nftables) | `make ze-netns-test` -- the real gate |
| Linux without those | nothing useful; the suites skip or fail on capability |
| macOS | `make ze-netns-qemu-test` (`mk/test-integration.mk:358`). `ze-netns-test` refuses on non-Linux by design (`:138`) |

**A plain `make ze-verify` does NOT meaningfully cover these four suites.** After
the marker change below they SKIP there, which is honest but means a green
`ze-verify` says nothing about them. Always run the netns gate for this work.

## State: measured before -> after (all under `make ze-netns-test`)

| Suite | Before | Now | Remaining ids |
|-------|--------|-----|---------------|
| firewall | 2/23 | **21/23** | 9, 21 |
| policy | 0/6 | **4/6** | 5 (timeout), 6 |
| ospf | 18/97 | **83/97** | 29,30,33,35,37,38,39,45,46,47,48,50,58,68 |
| ospfv3 | 7/14 | **11/14** | 6 (nbma), 7 (ptmp), 14 (vlink) |

## Committed this session (local only, nothing pushed)

| SHA | What |
|-----|------|
| `c27e3fe8e` | RFC 4271 S5.1.2 local-AS prepend on eBGP announces |
| `f9146a35c` | RS fast path stops claiming undelivered forwards |
| `16601c4c5` | `.ci` peer verdict fails when ANY check-mode peer fails |
| `c5d4bc957` | config-push host-key verification wiring pinned |
| `7b4c22675` | learned 1251/1252/1256 + three fixit specs opened |
| `4839ba562` | GOCACHE moved to the durable `cache/` |
| `6f39511a9` | net-admin markers, QEMU daemon feature tags, cache-hijack guard |
| `3b371ed80` | private-ASN fixture follows the prepend |
| `50fda9b48` | **policyroute: nft chain + ip rule the kernel accepts** |

## The four harness defects that were fixed (do not reintroduce)

1. **`ze-netns-test` destroyed its own setcap.** It ran the runner under `sudo`
   without `ZE_TEST_NO_BUILD=1`, so `Build()` rebuilt `bin/ze`
   (`internal/test/runner/runner.go:227`) and file capabilities are an inode
   xattr -- the rebuild silently discarded the `setcap` applied two lines earlier.
   Every test then ran uncapped and failed, looking like a product problem.
2. **`sudo` resets PATH to `secure_path`**, which lacks `/usr/local/go/bin`.
   Several ospf/ospfv3 tests exec `go` themselves. That alone was **65 of the 79**
   ospf failures. `PATH="$PATH"` is now forwarded.
3. **The QEMU daemon was feature-stripped.** Both QEMU targets built `ZE_QEMU_BIN`
   with `-tags 'ze_core zetest ze_distro'` -- no `$(ZE_FEATURES)`, so no ssh, no
   web, no BGP. Now matches `internal/test/runner` `TestBuildTags` (`runner.go:50`).
4. **Every QEMU run hijacked the checkout's `cache/` symlink.** `qemu-run.py`
   exports `HOME=/root`, `qemu-all-tests.sh` runs `make ze-unit-test-cached` inside
   the VM over a read-write 9p mount, and `ensure-links.py` repointed `cache/` to
   `/root/.cache/ze`. Harmless until GOCACHE moved under `cache/`; then the next
   host build died with `mkdir cache: file exists`. `cache/` now never
   auto-repoints; use `python3 scripts/dev/ensure-links.py --repoint-cache`.

## Update 2026-07-24 (session 2d): interface cluster UNBLOCKED; ddos RESOLVED

The "needs a full Linux host" framing in 2c was wrong -- the QEMU Alpine VM IS a
full Linux host (root, netns, nftables, conntrack, eBPF). The two "netns infra"
blockers were real but tractable, and both are now fixed and QEMU-validated:

- **`ze_api` under uid-drop (runner fix).** Root cause: the repo root is often
  0700, so a uid-dropped observer cannot traverse `/workspace` to import `ze_api`
  via PYTHONPATH. Fix: `copyTestScripts` (`internal/test/runner/runner_exec_util.go`)
  copies `test/scripts/*.py` into the tmpfs workdir (the observer script's own
  dir, on sys.path[0], chowned to the child). Proven: `ospf-ldp-sync-broadcast`
  now PASSES under uid-drop netns mode (was `ModuleNotFoundError`).
- **iface backend not loaded (product fix).** OSPF's transport needs the iface
  backend for the interface's IPv4, but it loads only from an `interface {}`
  config block. `iface.EnsureBackend` (`internal/component/iface/backend.go`) loads
  the default when none is loaded (no-op when an explicit backend already did),
  called from `resolveOSPFInterface`. Fixes a real OSPF-on-OS-configured-interface
  bug, not just the tests.

With those + `option=netns-link` provisioning, **`ospf-nbma` (50) is GREEN** under
netns in QEMU (after also fixing the observer to read the kebab `poll-interval` /
`nbma-neighbors` fields the Snapshot actually emits). Added to the netns gate
(`scripts/evidence/netns_qemu.py` OSPF_IDS). The remaining interface-missing tests
(58 ptmp, 68 show, 29 demux, 45-48 multiaf, ospfv3-vlink 14) need the same
treatment -- `needs-linux:caps=net-admin` + per-interface `netns-link` + any
per-test observer kebab/underscore fixes -- the mechanism is proven.

- **ddos-detect-characterize (155) RESOLVED.** On loopback, flooding the closed
  127.0.0.2:9999 makes the kernel emit ICMP-unreachables back to 127.0.0.1; those
  egress toward 127.0.0.1 and (embedding the datagram) are LARGER per packet, so
  `parseTopDestination`'s by-destination-bytes victim pick latched onto 127.0.0.1
  (the source) not 127.0.0.2, and characterization could not narrow to a victim it
  was not tracking. Fix: the driver binds a listener on 127.0.0.2:9999 so no
  reverse ICMP is generated -- a loopback-only artifact a real transit box does
  not have. GREEN in QEMU (`ze-test bgp plugin 155`).

## Update 2026-07-24 (session 2b): ospf ldp-sync cluster RESOLVED via two product-bug fixes

Root-causing the ldp-sync/multiaf "no TLS acceptor" cluster found TWO real
product bugs in the non-BGP (OSPF-only) daemon path, both now fixed and
QEMU-validated (ldp-sync 35/37/38/39 PASS):

1. **Config misrouting (`internal/component/config/probe.go`).** `ProbeConfigType`
   classified any `plugin {}` + no-`bgp {}` config as `ConfigTypeHub`, routing it
   to the plugin-hub Orchestrator (no built-in protocols, no TLS acceptor). An
   OSPF-only config with an external observer plugin was misrouted: built-in OSPF
   never ran and the observer died "no TLS acceptor configured". Fix: any
   top-level block other than `plugin`/`env` (e.g. `ospf`) routes to the full YANG
   daemon. Verified the ONLY `ze -` daemon tests matching the old misroute were
   these 9 ospf tests.
2. **Reactorless shutdown (`server/system.go`, `cmd/ze/hub/main.go`).**
   `handleDaemonShutdown` required a BGP reactor; an OSPF-only daemon has none, so
   `request shutdown` could not stop it and it hung. Subtlety: `Coordinator.
   FullReactor` returns the coordinator ITSELF as a no-op fallback, so
   `ctx.Reactor()` is non-nil even without BGP. Fix: a reactor-independent
   `SetShutdownFunc` (signal-based teardown), used when the reactor is the
   coordinator fallback.

**These two fixes are the real deliverable of the acceptor cluster** -- they fix
an OSPF-only (or any non-BGP) daemon that runs external plugins and can be shut
down by command, not just the tests.

### Interface-missing cluster (9 tests) -- BLOCKED on netns-uid-drop infra, not an ospf bug

50/58/68/29 + multiaf 45/46/47/48 + ospfv3-vlink 14 configure ACTIVE OSPF
interfaces (nbma/ptmp/point-to-point on nbma0/ptmp0/eth0/eth1) that do not exist.
`openConfiguredInterface` (`internal/plugins/ospf/instance.go:668`) opens active
types via `openInterface`, which needs a real link + an IPv4 address
(`interfaceIPv4`, transport/backend_linux.go). The config-syntax part is fixed
(50/58/ospfv3 6/7 semicolons, committed). The interface must be provisioned via
`option=netns-link` under netns mode -- BUT running these ospf OBSERVER tests
under the netns launch mode (uid-dropped ze) surfaced two infrastructure gaps the
firewall/policy netns tests never exercise (they use `driver.py`, not `ze_api`):
- **`ModuleNotFoundError: No module named 'ze_api'`** -- the uid-dropped observer
  cannot import `ze_api` (PYTHONPATH from `runner_exec.go:734` is not reaching the
  forked plugin under uid-drop). ldp-sync proves ze_api works in NORMAL mode.
- **`iface: no backend loaded`** -- the OSPF transport's interface-address query
  finds no iface backend under uid-drop (loads fine as root in normal mode).
Both are netns-launch-mode + uid-drop infrastructure problems, separable from
OSPF. Resolving this cluster needs those two gaps fixed first (plugin env
propagation + backend loading under uid-drop), then per-test `netns-link`
provisioning + an ospf subset in `scripts/evidence/netns_qemu.py`.

### Sharper diagnosis (session 2c): the "no backend loaded" is an OSPF product bug, not netns

`netns-link` provisioning DOES work -- with `nbma0` provisioned, the OSPF
transport's `resolveIfaceBinding` (`iface.Resolve`) succeeds; the failure moves
to `interfaceIPv4` -> `iface.Addresses` -> `osDeviceFor` -> `GetInterface` ->
`backendOrErr` = **"iface: no backend loaded"**
(`internal/plugins/ospf/transport/backend_linux.go:49,246` +
`internal/component/iface/dispatch.go:15`). The iface netlink backend is loaded
ONLY by the iface component's `OnConfigure` when the config has an `interface {}`
block (`internal/component/iface/register.go:400-414`); an OSPF-only config has
none, so the backend never loads and OSPF cannot read the interface's IPv4 for
its multicast join. This is INDEPENDENT of netns/uid-drop -- it would break a
real `ze` daemon running OSPF on an interface whose address is set by the OS/DHCP
(no `interface {}` stanza). Every ospf test masked it by failing at "Link not
found" first; provisioning the link surfaces it.

Candidate fix (NOT taken -- daemon-wide behaviour change I could not end-to-end
validate while the cluster is still blocked on `ze_api`): have the iface plugin
ensure a default backend (`iface.DefaultBackendName`, `backend.go:239`) is loaded
at startup for its dependents (OSPF declares `Dependencies: ["interface", ...]`),
guarded by `GetBackend() == nil` so an explicit `interface { backend vpp }` still
wins. The design currently REQUIRES an explicit backend
(`errInterfaceNoBackendConfiguredAndNo`, `register.go:407`), so this needs an
owner decision, not a drive-by change. So the interface cluster now needs THREE
things, in order: (1) the iface-default-backend fix above, (2) `ze_api`
importable under uid-drop, (3) per-test `netns-link` + ospf subset in the netns
gate.

## Update 2026-07-24 (session 2): items 1/2/5 fixed, ospf tail fully diagnosed

A second session (macOS, validating under `make ze-netns-qemu-test`) landed the
fixable items and diagnosed the rest to root cause. All fixes below are
independent-review-clean (two adversarial reviewer passes, 0 BLOCKER/0 ISSUE).

**Fixed and validated in QEMU:**
- **firewall 9** — `applySet` never set `nftables.Set.HasTimeout`, so the vendored
  library dropped every per-element timeout (`set.go` gates `NFTA_SET_ELEM_TIMEOUT`
  on the parent set's flag). New `lowerSet` (`internal/plugins/firewall/nft/lower_linux.go`)
  sets it. Unit test `TestLowerSetTimeoutFlagAndElementTimeouts` (QEMU-green); the
  `.ci` stays excluded (crashes the Alpine kernel, as documented).
- **policy 5** — new `option=netns-link:name=<if>[:address=<cidr>]` provisions an
  interface inside the per-test netns before `ze` launches (`enterTestNetns` brought
  up only `lo`). `005-next-hop` is **green end-to-end** under `make ze-netns-qemu-test`,
  which now also runs a policy subset (1-5). Netlink-level regression test
  `TestProvisionNetnsLinksMakesNextHopRoutable` QEMU-green.
- **ospf 33** (`ospf-interface-runtime`) — the test asserted a loopback interface's
  state is `down`; the ISM correctly returns `loopback` per RFC 2328 sec 9.1
  (`iface.go` Start(): NetworkLoopback -> StateLoopback; `ism.go`). Test was pinning
  an RFC violation. Corrected; QEMU-green.
- **item 5 (log keys)** — 22 hyphenated `ze.log.*` keys across 14 `.ci` files dotted
  (`getLogEnv` splits on `.`, never resolves the hyphen form). Verified inert:
  rsvpte passes both ways.
- **Harness build bug (was the real "defect #3" remnant)** — the `ze-netns-qemu-test`
  DUT daemon (`mk/test-integration.mk`) was built without `$(ZE_FEATURES)`, so
  `zetest` pulled in `fakeddos` whose YANG imports `ze-ddos-detect-conf` (owned by
  `ze_ddos`) and **every** config load failed "no such module: ze-ddos-detect-conf".
  This is why the netns-qemu firewall subset looked mass-broken. Aligned the tag set
  with the sibling targets + `TestBuildTags`. Full firewall+policy gate now green.

**ospf 50/58 + ospfv3 6/7 — two-part, part 1 fixed:**
- Part 1 (FIXED): the bgp block used a compact inline form (`asn { local 65000 remote
  65000 }`, `remote { ip 127.0.0.1 }`, `capability { ... }`, `behavior { ... }`) with
  no `;` between statements. The tokenizer auto-inserts `;` only on a NEWLINE after a
  value (`tokenizer.go` "Automatic semicolon insertion"), so inline-without-`;` is
  genuinely invalid. 130 other test files use the `;`/multiline form; only these 4
  deviated. Added the semicolons.
- Part 2 (NOT fixed, diagnosed): after parsing, the OSPF engine hard-fails
  `resolve interface nbma0/ptmp0: Link not found`. `openConfiguredInterface`
  (`internal/plugins/ospf/instance.go:668`) starts passive/loopback interfaces WITHOUT
  a kernel link but sends active types (nbma/ptmp/broadcast) through `openInterface`,
  which requires a real link (an active OSPF interface needs a live socket -- by
  design). The tests configure active interfaces on names that do not exist. Fix:
  provision the interface via `option=netns-link` + `option=needs-linux:caps=net-admin`,
  and add ospf/ospfv3 to the netns gate. Same class as 68 below. This is a test-infra
  expansion (run these under netns mode), not a code bug -- do not "fix" OSPF to
  tolerate a missing active link.

**ospf TLS-acceptor cluster 35/37/38/39 (ldp-sync) + 45/46/47/48 (multiaf) -- 8 tests, ONE root cause (NOT fixed):**
- All fail `no TLS acceptor configured (hub config required for external plugins)` at
  `startExternal` (`internal/component/plugin/process/process.go:559`). Root cause: an
  OSPF-only daemon (no `bgp` block) routes `plugin { external ... }` through the hub
  Orchestrator, which registers every plugin as a *subsystem* (`hub.go:62`) and starts
  it via `SubsystemHandler.Start` (`subsystem.go:112`) -- a path that NEVER calls
  `SetAcceptor`, unlike `plugin/manager/manager.go`'s `ensureAcceptor`+`SetAcceptor`.
  id 33 works only because its `bgp` block routes the plugin through the Manager path.
  This is a real product bug (OSPF-only + external plugins), but the fix threads an
  acceptor through hub Orchestrator -> SubsystemManager -> SubsystemHandler -> Process,
  which touches core startup (broad blast radius) -- deliberate, its own change + tests.

**ospf 68 (`ospf-show`) -- interface-missing, same as ospf 50/58 part 2:** OSPF engine
fails `resolve interface eth1: Link not found`, then `ospf-show` dispatch hits "plugin
process not running". Provision eth1 (netns-link) under netns mode.

**ospf 29 (`ospf-instance-demux`) -- partially diagnosed:** instance 0 has
`interface-count 0`, instance 5 has `1`; the test wants both to carry eth0. instance 5
opened its interface, instance 0 did not. Likely the same missing-link issue for
instance 0's interface, but not fully traced -- read `ospf-instance-demux.ci` config
+ the multi-instance interface assignment before assuming.

**Still open (untouched this session):** ospfv3 14 (vlink), and item 4 (ddos
classifier -- the deep conntrack->classifier->narrow chain; needs real-kernel eBPF
observation, unchanged from below).

**id 30 (`ospf-instance-teardown`)** now passes (was a load/build-error artifact of
defect #3, cleared by the daemon-build fix). The ospf failing set is now:
29, 35, 37, 38, 39, 45, 46, 47, 48, 50, 58, 68 (33 and 30 green); ospfv3: 6, 7, 14
(6/7 parse-fixed, blocked on part 2).

## Remaining work, with confidence labels

Do not treat the unverified rows as diagnosed. Two of my earlier "single cause
explains the set" conclusions were wrong (see Traps).

| # | Item | Confidence | What is known |
|---|------|-----------|---------------|
| 1 | firewall 9 `009-set-element-timeout` | symptom VERIFIED, producer NOT traced | nft programs `elements = { 10.0.0.1, 10.0.0.2 }` with **no** per-element timeout; test expects `10.0.0.1 timeout 1h`. Self-contained; start at the nft set-element lowering in `internal/plugins/firewall/nft/`. Best first task |
| 2 | policy 5 `005-next-hop` | cause VERIFIED | `route add (table 2000 via 10.0.0.1): network is unreachable`. `enterTestNetns` (`internal/test/runner/netns_linux.go`) brings up only `lo`; the config wants `interface eth1` and a next hop on 10.0.0.0/24 (`test/policy/005-next-hop.ci:57,64`). Needs the namespace to provision the test's interface |
| 3 | ospf 14 + ospfv3 3 | NOT diagnosed | 10 `exit_code_mismatch`, 4 `logging_mismatch`. One logging case is `lo0: state = 'loopback', want 'down'` -- likely the same netns-`lo` shape as item 2, so item 2 may clear several. Read them before assuming |
| 4 | firewall 21 + `bgp plugin` 155-164 (ddos) | cause VERIFIED, fix unknown | NOT a log-key or capability problem. The plugin runs and installs a drop rule; the real-time classifier keeps it coarse instead of narrowing: `ddos-local did not narrow to udp sport 11211 -> 127.0.0.2 (sent 512000 packets); rule stayed coarse or classifier missed the flow`. Deepest item; treat as its own investigation |
| 5 | 6 hyphenated `ze.log.*` keys | bug VERIFIED, impact VERIFIED NIL | `ddos-detect`, `ddos-fake`, `ddos-local`, `ddos-observe`, `routing-table`, `rsvp-te`. `CanonicalSubsystemName` (`internal/component/plugin/inprocess.go:27`) maps `-` to `.`, so these keys set nothing and the level stays at the WARN default. Real latent bug; will NOT flip any current red because those tests fail earlier on substance. Cleanup only |
| 6 | `bgp plugin` ~39 failures | owned elsewhere | `plan/spec-fixit-peer-verdict-and-forward-rail`, Phase 1 triage, unstarted. Its own Failure Routing says triage before committing; that did not happen |

Suggested order: 1, then 2 (may clear part of 3), then 3, then 4. Item 5 anytime.

## Traps -- I hit all three, do not repeat them

- **A green run that did nothing looks like a green run.** A QEMU re-run reported
  ZERO failures, down from 43. It had never started: the build died on the
  hijacked symlink. **Always confirm a suite EXECUTED before believing an
  improvement** (check suite summary lines, not just the failure count).
- **Do not conclude "kernel lacks support" from one probe.** I nearly filed
  policy as an unfixable kernel gap. `nft add chain ... type route hook prerouting`
  fails, but `type route hook output` succeeds -- route chains are output-only, so
  the real defect was in ze. One command apart from a wrong report.
- **One cause rarely explains a whole failing set.** I assumed the net-admin
  marker covered firewall+policy (policy had two further bugs behind it), then
  assumed the log-key bug covered the ddos tests (it does not; they fail on
  classifier behaviour). Read each failure.

## Verification

```
make ze-netns-test            # Linux + sudo + setcap + nft
make ze-netns-qemu-test       # macOS equivalent
```

Expect the counts in the table above. `host nft tables unchanged (host-safe)` must
appear; if it does not, a test escaped its namespace and that is a stop-the-line bug.

## Do not re-read these (already established)

- `mk/test-integration.mk` netns target -- fixed, comments explain the two traps
- `scripts/dev/ensure-links.py` + `ensure_links_test.py` -- fixed, 7 tests
- `internal/plugins/policyroute/{translate,rules_linux}.go` -- both bugs fixed
- `plan/learned/1258-qemu-gate-ran-a-stripped-daemon.md` -- the full narrative

## Caveats on what is committed

Two commits carry `--review-override` stating plainly that **no independent
review was performed** (reviewer subagents were unavailable). `50fda9b48` changes
real dataplane behaviour in policy routing and has had no second pair of eyes.
Several commits carry `--unverified` because `ze-functional-test` is red for the
reasons above. GPG signing needed an interactive passphrase; commits were run by
the owner from their own terminal.

## Update 2026-07-24 (session 2f): interface cluster RESOLVED; multi-instance root cause was the config parser

The OSPF interface-missing cluster is done and QEMU-validated. All eight run green
under `make ze-netns-qemu-test` (host nft unchanged): `ospf-nbma`, `ospf-ptmp`,
`ospf-show`, `ospf-multiaf`, `ospf-multiaf-reconcile`, `ospf-multiaf-show`,
`ospf-multiaf-v4-route`, `ospf-instance-demux`, plus `ospfv3-vlink`.

The last holdout, `ospf-instance-demux`, was NOT an OSPF bug. `instance-id 0;
instance-id 5;` (two repeated leaf-list statements on one interface) silently kept
only 5 because the brace parser's leaf-list stores called `Tree.SetSlice`, which
REPLACES the whole leaf-list. YANG models repeated leaf-list statements as additive
(RFC 7950 sec 7.7). Fix: `Tree.AppendSlice` (append, dedup as a set, preserve
deactivation markers, re-sync the scalar mirror to active members), used by both
brace-parser leaf-list stores. This is a general fix -- every leaf-list, spelled as
repeated statements, now accumulates instead of silently losing all but the last.
See `plan/learned/1267`.

The netns gate now selects OSPF tests by NAME, not numeric nick: nicks are
load-order ordinals over the sorted glob, so an added/renamed earlier `.ci`
silently renumbers and an in-range-but-shifted nick runs the WRONG test still green
(caught in review: `ospf-multiaf` had been converted for netns but omitted from the
numeric set). Names exact-match and are stable.

### Resolved after the interface cluster
- ospfv3-nbma and ospfv3-ptmp: the OSPFv3 counterparts of the v2 nbma/ptmp tests.
  Same fix -- `needs-linux:caps=net-admin` + `netns-link:name=<if>` (no address,
  IPv6 link-local auto on link-up); both added to the netns_qemu OSPFV3 subset and
  QEMU-validated green.

### Still open (the genuine remaining tail)
- Any items outside the OSPF/OSPFv3 interface cluster listed earlier in this
  handover that a prior session did not close (audit the per-suite "Remaining ids"
  table above against the current suite before assuming completion).

## Update 2026-07-24 (session "left-overs"): ze-qemu-debug/shell tag drift + contention thread RESOLVED

The two threads left for this session are done and committed (unpushed, `main`):

- **ze-qemu-debug / ze-qemu-shell built a stripped DUT.** Same defect class as
  `plan/learned/1258-qemu-gate-ran-a-stripped-daemon.md`: both built
  `$(ZE_QEMU_BIN)` with `ze_core zetest ze_distro` (no
  `ze_setup $(ZE_FEATURES)`), so `zetest` pulled in `fakeddos` whose YANG imports
  `ze-ddos-detect-conf` (`ze_ddos`) and every config load through those two targets
  died "no such module: ze-ddos-detect-conf". Aligned both with `TestBuildTags`
  (`runner.go:50`). `ospf-interface-runtime` (33) and `ospf-route-daemon` (66) both
  PASS under `make ze-qemu-debug` (QEMU-validated). Commit `26155571f`.
- **Contention-detector drift.** `internal/test/runner/hostload.go` and
  `scripts/status/verify_run.go` had two copies of contention detection that
  disagreed (the status tool warned on process count alone, no load gate).
  Extracted into leaf package `internal/core/hostload` as the single source of
  truth; `verify_run.go` now load-gates its warning. Commit `26155571f`; details
  in `plan/learned/1269`.
- **Durable prevention.** All five QEMU DUT build lines now derive their `-tags`
  from one `ZE_QEMU_DUT_TAGS` make variable, so a fourth drift is structurally
  impossible. Commit `a6f930c98`.
