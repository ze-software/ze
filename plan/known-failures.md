# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" -> pre-existing failures >10 min): logged, not blocking unrelated commits.

**Status 2026-07-02: two open failures (below), unrelated to spec-as112/cos/plugin-process-boundary work.**
`internal/component/plugin/all`'s golden-snapshot tests
(`TestRegisteredPluginNames`/`TestRegisteredWireMethods`/`TestYANGSchemaProviders`)
also intermittently fail during the plugin-process-boundary work session --
confirmed under both an untagged `go test` (reports isis/ldp/ospf/rsvp-te as
"missing", a build-tag-scoping artifact of not passing the full tag set) and
the full tagged build (reports new OSPF wire-methods -- graceful-restart,
instance, ipv6, segment-routing -- as "unexpected," i.e. not yet in the
checked-in `testdata/*.snapshot`). `git status` confirms
`internal/component/plugin/all/all.go` and all 3 snapshot files are
modified by the same concurrent OSPF session tracked below, not this
session (which never touched `internal/component/plugin/all/`).
Every previously tracked entry from 2026-07-01 is resolved (see below). Three
were already fixed by shipped specs (621, 887, 888) and merely stale in this
log; the kernel flake and the cos-vendor fixtures were fixed 2026-07-01. The
`ddos/detect` build break (concurrent cp-survival session) that briefly
blocked `go build ./...` and `go test ./internal/component/doctor/...` on
2026-07-02 during the AS112 doctor-check review cycle is now resolved --
`make ze-validate` run 2026-07-02 during the plugin-process-boundary work
(traffic-usage/flow-export/ddos-detect fixes + new ze-plugin-boundary-check)
shows 39 unwired-symbol issues, entirely in `internal/component/ike/dataplane/**`,
`internal/component/trafficstat/service.go`, `internal/core/stats/**`,
`internal/plugins/ldp/**`, and `internal/plugins/ospf/**` -- confirmed via
`git status` that every one of those paths is modified/untracked by other
concurrent sessions (VPP IKE dataplane, an EWMA/stats package, LDP adjacency
work, and the same ongoing OSPF extension work already tracked below), none
touched by this session; `make ze-lint-changed`, `go vet`, and
`go test ./internal/plugins/{trafficusage,flowexport,ddos/detect}/... ./scripts/checks/...`
all pass clean for every file this session changed.
`go build ./...`'s only remaining failure is the OSPF entry below, which has
since moved to a different symbol (`gr.go:282`), consistent with the same
concurrent OSPF session continuing to iterate.

### `internal/component/doctor` -- 4 listener/schema tests fail on this macOS dev machine, pre-existing, not attributable to the AS112 doctor-check additions

Observed 2026-07-02 via `go test ./internal/component/doctor/... -v` while
adding `doctor-as112-watchdog-missing-withdraw` and
`doctor-as112-global-origin-uncoordinated` (AC-10/AC-11 from the closed
as112-3 spec): `TestCheckListeners_PortInUse`, `TestCheckListeners_API`,
`TestCollectSchemaListeners_SSHDefault`, `TestCollectSchemaListeners_SSHExplicit`
fail consistently (3/3 in isolated re-runs, `-count=3`). All four exercise
`checkListeners`/`collectSchemaListeners` in `checks_listener.go`, a file
this session never touched (session's only edits to this package:
`checks_helpers.go` +`nestedSlice`, `doctor.go` +2 `runChecks` call lines,
new `checks_as112_coordination.go`/`_test.go`). `TestCheckListeners_PortInUse`
and `_API` bind a real `127.0.0.1:0` listener, then assert `checkListeners`
reports that same port as unavailable -- consistent with this specific
macOS host's socket stack allowing a second bind where the test expects
exclusivity (a `SO_REUSEPORT`/dual-stack quirk, not exercised by any AS112
code path). `go vet ./internal/component/doctor/...` and every AS112-scoped
test (`-run 'AS112'`, `TestDoctorCoverageCodesRegistered`,
`TestRunChecksExecutesRegisteredPluginCheck`) pass cleanly. Owner: whoever
next investigates macOS listener-probe test portability; not spec-as112 scope.

### `internal/plugins/ospf/multi_instance.go` -- build fails, owned by a concurrent OSPF session, not attributable to spec-as112/cos work

Observed 2026-07-02 via a full `go build -tags 'ze_core ze_distro ze_gnmi
ze_grpc ze_isis ze_ldp ze_lg ze_mcp ze_ospf ze_rest ze_rsvpte ze_ssh
ze_telemetry ze_web' ./cmd/ze` while preparing a commit for the spec-as112
set (which never touches `internal/plugins/ospf`): `e.mInstanceMismatch
undefined`, `cfg.instanceIDSet undefined`, `cfg.forInstance undefined` --
struct fields referenced by `multi_instance.go` that do not yet exist on
`*engine`/`ospfConfig`, consistent with a concurrent Claude session's
in-progress OSPF multi-instance refactor mid-edit (confirmed via `git
status`: `internal/plugins/ospf/**`, `docs/architecture/wire/ospf.md`,
`docs/guide/ospf.md`, `plan/spec-ospf-*.md` all modified, none touched by
this session). Building `./cmd/ze` WITHOUT the `ze_ospf` tag succeeds
cleanly, and every AS112/iface/cos/sdk package this session touched passes
`go vet`/`go test`/`make ze-lint-changed`/`make ze-doc-drift`/`make
ze-validate` with zero related findings. Owner: whichever session is
actively editing `internal/plugins/ospf/multi_instance.go`.

## Harness notes (not failures)

The full plugin suite shows load-induced flakiness under max parallelism -- e.g.
`257`, `258`, `312` failed in one `--all` run but pass 3/3 in isolation. Running
two full `--all` suites back-to-back melts down (resource exhaustion: ~50
timeouts, ~200 "failures"). Triage individual tests in isolation; treat a
contiguous block of failures or a spike of timeouts in `--all` as a
harness/resource artifact, not real regressions.

### `internal/component/l2tp` `TestPeerTeardownWithdrawsSubscriberRoute` -- load-sensitive `-race` flake, not attributable to spec-as112 work

Observed 2026-07-01 during `make ze-verify-changed` while implementing the
spec-as112 set (which never touches `internal/component/l2tp`): `go test
-race` reports a data race between `L2TPReactor.SetRouteObserver`
(`reactor_setters.go:114`, called from the test itself) and
`L2TPReactor.notifyRouteObserverDown` (`reactor_kernel.go:253`, called from
the reactor's own goroutine) -- both racing on the same `RouteObserver`
field with no synchronization between the test's setter call and the
reactor's already-running background goroutine. Reproduced twice in the full
`ze-unit-test-changed` sweep (58 packages under `-race` simultaneously) but
passes 5/5 in isolation (`go test ./internal/component/l2tp/... -race -run
TestPeerTeardownWithdrawsSubscriberRoute -count=5`), consistent with a
genuine, load-sensitive scheduling race in the test's own setup (the test
calls `SetRouteObserver` after `Start()`, with no barrier against the
reactor goroutine already running), not a flake caused by, or fixed by, any
spec-as112 change. Owner: whichever session next touches
`internal/component/l2tp/reactor_test.go` around
`TestPeerTeardownWithdrawsSubscriberRoute` -- likely fix is a synchronized
handoff (set the observer before `Start()`, or via a channel) rather than a
post-`Start()` direct field write.

## Resolved

### 2026-07-01 -- kernel-runtime-deps parallel-execution flake -> per-test isolation

**Resolved 2026-07-01.** `install/26` kernel-runtime-deps TOCTOU race was a
shared-path collision: it read/created `tmp/kernel/build/vmlinuz` while
`ze-kernel-overlay` (which runs `make ze-kernel`) moved/removed that same dir,
so a concurrent `out.stat()` threw `FileNotFoundError`. Fixed by redirecting the
test's build-output artifact to a per-test dir: `make -q -C gokrazy/kernel
OUT="$work/out"` (the Makefile's `OUT :=` at `gokrazy/kernel/Makefile:19` loses
to a command-line override), with the fake vmlinuz created/touched there and the
`out.stat()` hardened with try/except. The mtime dependency graph the test
exercises is unchanged: prerequisites are repo-relative source paths, not `OUT`.
Verified: 4 back-to-back parallel `ze-test install --pattern kernel` runs, all
20/20 PASS with runtime-deps and overlay in the same batch.

### 2026-07-01 -- Plugin cos-vendor (cisco/coexist) -> fixture fixed

**Resolved 2026-07-01.** `cos-vendor-cisco`, `cos-vendor-coexist` (was ids
126/127, now 128/129) nested `radius { server ... }` under `l2tp { authentication
{ ... } }`. The `authentication` container (added by spec 888, l2tp-env-promote)
holds only PPP-phase `timeout`/`reauth-interval`; the RADIUS config path is
`l2tp { auth { radius { ... } } }`, defined by the authradius plugin
(`internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang:11`).
The fixtures named the wrong sibling container. Fixed both `.ci` files to `auth {}`;
`ze config validate` now returns "configuration valid" (was `unknown field in
authentication: radius`). Same class as the paths-limit.ci fixture fix. These
tests are `needs-linux`; the failing part (YANG validation) is host-independent
and was verified on darwin via `ze config validate`.

### 2026-07-01 -- Parse VPP feature gate (bridge/veth/aggregates) -> already passing

**Resolved 2026-07-01.** `iface-vpp-rejects-bridge`, `iface-vpp-rejects-veth`,
`iface-vpp-aggregates-errors` were logged in the 2026-06-17 triage as "VPP backend
feature gate not implemented". Spec 621 (backend-feature-gate) shipped the walker
`config.ValidateBackendFeatures` (`internal/component/config/backend_gate.go`) and
wired it into `ze config validate` (`cmd/ze/config/cmd_validate.go`). Re-run
2026-07-01: all 7 `iface-vpp-*` parse tests PASS. The triage entry was stale.

### 2026-07-01 -- L2TP reauth-interval-clamp -> replaced by YANG range

**Resolved 2026-07-01.** The 2026-06-17 triage flagged the env-var path bypassing
the reauth safety floor. Spec 888 (l2tp-env-promote) removed the L2TP env vars
entirely and moved `reauth-interval` into YANG with `range "0 | 5..86400"`
(`internal/component/l2tp/yang/ze-l2tp-conf.yang:172`), deleting `clampReauthInterval`
and its test. Verified 2026-07-01: `ze config validate` rejects `reauth-interval 3`
(`outside range 0, 5..86400`) and accepts 0/300 -- the floor is now enforced at
commit time, a stronger guarantee than the old runtime clamp.

### 2026-06-18 -- Web suite (was 81/81 FAIL) -> harness fixed, genuine bugs fixed

**Resolved 2026-06-18.** Root cause was the harness, not the product: the
runner launched `ze start --web <port> --insecure-web` against an empty temp
config store, so the daemon refused to start (full `--web` needs a loaded
config) and exited before binding the port -- every test timed out at the
readiness probe (`config "ze.conf" has unknown type`).

Harness fixes (`internal/test/cli/cmd_web.go`,
`internal/component/web/testing/runner.go`):
- Launch `--web-only` (standalone web UI, no daemon/config -- the mode the
  daemon's own error hint recommends). Server now binds; suite 0 -> ~76/81.
- Readiness probe does an HTTPS GET, not a bare TCP connect (TCP accepts the
  instant the listener binds, before routes mount, so a browser could hit an
  empty page).
- `expect` assertions auto-retry up to 5s (standard auto-waiting pattern),
  absorbing HTMX/JS render races a single point-in-time snapshot caught as
  "(empty page)".
- Each test closes its browser session when done (was leaking 80+ live pages
  into the shared agent-browser daemon over a run).
- Seed a zefs local-admin into the temp store so `/show/users/` lists the
  always-on "(system)" power user (verified via curl: page renders `(system)`).

Four genuine pre-existing test bugs the all-failing harness had masked, all
fixed (authorized `.wb` edits): `scenario-interface-setup` and
`interface-configured-display` filled a `field-mac-address` the key-only add
overlay never renders (removed -- mac-address is edited on the detail page);
`logs-live-stream` asserted the transient "Connecting" that SSE replaces with
"Connected" (now asserts "Connected"); `system-users-power` needed the seeded
admin to show the "(system)" marker.

Residual: this is performance-sensitive browser automation driven through one
shared agent-browser daemon. Under heavy host load (this dev box sat at load
avg ~7 from unrelated apps) render races still flake a rotating handful per
full run; every test passes individually. Expected reliable on a quiet CI
host. Verified: each fixed test passes in isolation; `webtesting` unit tests
green; `--web-only` server serves every exercised route.

### 2026-06-18 -- Lint: `internal/analyze/inject.go:64` goconst

`--router-id` had 3 occurrences across inject.go, serve.go, replay.go.
Added `//nolint:goconst` to inject.go and serve.go (replay.go already had it).

### 2026-06-17 -- Plugin observer/RIB visibility (6 tests) -> PASS

**Resolved 2026-06-17.** `40` bestpath-reason, `220` multipath-basic, `224`
nexthop-self, `225` nexthop-unchanged, `308` rib-forward-handle-observed, `350`
rr-basic. Triaged as "routes never appear in RIB within 15-20s timeout (product
bug in forwarding)". Actually the same establishment-time EoR race as the exabgp
suite: these tests have the mock peer wait for ze's End-of-RIB
(`rib-forward-handle-observed.ci:21`) and an observer poll for the prefix; the
bgp-rs duplicate/misordered EoR perturbed establishment so the poll timed out.
Fixed by `99c943404` (`AnnounceEOR` honors `ShouldQueue()`). Now 0 failures
across 5 runs (~2-4s each, not the 15-20s timeout); full plugin suite 422/424
(only 126/127 cos-vendor remain). Causation is circumstantial (not bisected --
reverting a committed fix needs forbidden git ops) but the failing->passing
transition lands in this commit's window and the EoR mechanism is shared.

### 2026-06-17 -- `ze-test bgp encode 38 paths-limit` (broken fixture) -> PASS

**Resolved 2026-06-17.** Not a ze bug. ze emits the route WITH an ADD-PATH
path-id (`00 00 00 00 18 0C 00 02`); the fixture expected it WITHOUT, so the
decoder read the four path-id zero bytes as four `0.0.0.0/0` prefixes. ze is
RFC 7911-correct: the config advertises add-path send/receive, the ze-peer mock
MIRRORS the OPEN so it advertises receive, the family negotiates
(`negotiated.go:279` gates on localSend && remoteReceive), and a path-id is then
mandatory on every ipv4/unicast NLRI. The expected hex in
`test/encode/paths-limit.ci` (added `56f48c85f`) omitted the path-id and was
internally inconsistent. Fixed the fixture (user-authorized per
`ai/rules/testing.md`) to ze's correct output. Encode suite now 53/53.

### 2026-06-17 -- `ze-exabgp-test` (was "10/40 product bugs") -> 40/40 PASS

**Resolved 2026-06-17.** The "10 distinct encoding bugs" was a mis-diagnosis.
Verified non-deterministic: failure set changed every run (e.g. run A
{20,32,35,39,40} vs run B {1,14,18,25,31,33,35,36,40}); conf-addpath passed
5/5 alone yet failed under parallel load. Two real causes:

1. **EoR race (8 tests + watchdog).** Two producers send End-of-RIB on session
   establishment: reactor `sendInitialRoutes` (always, per-family) and the
   bgp-rs plugin's `replayForPeer` goroutine (fast-fails when bgp-adj-rib-in is
   absent, as the exabgp wrapper loads a minimal plugin set). Announce/withdraw
   honor `ShouldQueue()`+opQueue; `AnnounceEOR` wrote directly, so the plugin
   EoR raced ahead of the still-queued route NLRI. Partial fix `af60758d0`
   covered only family-specific routes, not the static-route phase.
   Fix `99c943404`: `AnnounceEOR` skips peers in initial sync (reactor owns the
   EoR). Removes the race and the duplicate EoR.
2. **srv6-mup (1 test) -- the only real encoding bug.** `routeattr_prefixsid.go`
   wrote the SRv6 SID Structure Sub-Sub-TLV header as 4 bytes (`0,1,0,len`)
   instead of RFC 9252 3 bytes (`1,0,len`) -- a spurious leading reserved byte,
   inflating the inner sub-TLV by 1 (0x1F vs 0x1E). Decode side was already
   correct (`srv6sid.go`). Fix: drop the extra byte.

After both: 40/40 pass across repeated full-suite runs; watchdog 12/12 alone.

### 2026-06-17 -- decode suite (37/37 FAIL -> 37/37 PASS)

**Resolved 2026-06-17.** Three root causes:
1. 36 `.ci` files referenced `ze-test decode` (removed); renamed to
   `ze bgp decode`. Added `--family` long flag alias for `-f`.
2. `CombinedOutput()` in `decoding.go` mixed YANG description mismatch
   warnings (stderr) into JSON output. Changed to separate stdout/stderr.
3. Plugin registry maps (`CapabilityMap`, `FamilyMap`, `InProcessDecoders`)
   were captured at package init time before all plugins registered. Changed
   to lazy `sync.OnceValue` so lookups happen after all `init()` complete.

### 2026-06-17 -- plugin JSON-match + cli-show (7 new regressions -> 0)

**Resolved 2026-06-17.** Same YANG-warnings-in-stdout root cause:
`CombinedOutput()` in `runner_validate.go:decodeToEnvelope` mixed stderr
warnings into JSON decode output. Changed to `cmd.Output()`. Also updated
`cli-show.ci` test expectation (`Available commands` -> `Commands`).

### 2026-06-17 -- UI functional tests

**Resolved 2026-06-17.** All 5 UI failures from 2026-06-13 now pass.

### 2026-06-04 -- QEMU baseline triage

**Resolved 2026-06-04.** Clean re-run confirmed host-load artifacts.
Fixed: UI build tags, expected strings, mpls-doctor semicolons, firewall
skip-env. Product bugs fixed: L2TP CDN teardown, StopCCN cascade.
Environment deps (skip-env tagged): show-policy-routes, wireguard-invalid.

### 2026-06-10 -- routewatch QEMU integration tests flaky (netns roulette)

**Resolved 2026-06-10.** Namespace-aware subscribe + event polling.

### 2026-06-11 -- `make ze-verify-wiring-docs` command validation drift

**Resolved 2026-06-11.** Wiring, doc, and inventory gates all green.

### 2026-05-31 -- pppoe-client `no-default-route` + dispatch single-marshal

**Resolved 2026-05-31.** TypeEmpty wired end-to-end. Single-marshal,
stale plugin lists, migration keyword, multi-line descriptions, CLI
grammar catch-up.
