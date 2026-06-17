# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" -> pre-existing failures >10 min): logged, not blocking unrelated commits.


### 2026-06-17 -- host `make ze-verify` re-triage (supersedes 2026-06-13)

**Open (pre-existing).** Verified against current working tree.

#### Plugin (8/431 FAIL) -- pre-existing from 2026-06-13

**Observer timeout / RIB visibility** (6 tests, 15-20s timeout, observer
never sees route in RIB):

| # | Name | Observer detail |
|---|------|-----------------|
| 40 | bestpath-reason | expected 2+ candidates, got 1 |
| 220 | multipath-basic | multipath-peers empty |
| 224 | nexthop-self | route 10.1.0.0/24 not in RIB |
| 225 | nexthop-unchanged | route 10.1.0.0/24 not in RIB |
| 308 | rib-forward-handle-observed | 192.168.1.0/24 never appeared |
| 350 | rr-basic | route 10.1.0.0/24 not in RIB |

**Exit-code mismatch** (2 tests): `126` cos-vendor-cisco, `127`
cos-vendor-coexist (expected 0, got 1). Config parser rejects
`authentication { radius { ... } }` inside `l2tp {}`.

#### Parse (3/224 FAIL) -- pre-existing

`116` iface-vpp-aggregates-errors, `117` iface-vpp-rejects-bridge, `121`
iface-vpp-rejects-veth. VPP backend feature gate validation not
implemented: `ze config validate` accepts unsupported features under
`backend vpp`.

#### L2TP (1/16 FAIL) -- pre-existing

`12` reauth-interval-clamp: stderr does not contain "below safety floor;
clamping". The clamp function was removed; validation moved to YANG range
checks. Env var path bypasses YANG validation, leaving no safety net.

#### Web (81/81 FAIL) -- systemic: web server startup

All tests hit the 30.1s timeout. The test runner starts
`ze start --web <port> --insecure-web` without a config file, which fails
with `config "ze.conf" has unknown type`. Needs a minimal config in the
temp directory or `--web-only` mode.

#### Lint: `internal/analyze/inject.go:64` goconst (pre-existing)

`--router-id` has 3 occurrences; surfaced by `ze-lint-changed` via the
committed-since-verify set. From `3215ece93` (2026-06-09), unrelated to any
2026-06-17 work.

## Resolved

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
