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

#### `ze-exabgp-test` (10/40 FAIL) -- product bugs

`1` conf-addpath, `2` conf-aggregator, `20` conf-ipv6grouping,
`25` conf-mvpn, `29` conf-no-asn4, `32` conf-paths-limit,
`33` conf-prefix-sid, `35` conf-srv6-mup, `36` conf-srv6-mup-v3,
`40` conf-watchdog. BGP encoding bugs: ze sends wrong UPDATE messages
(e.g., MP_UNREACH instead of MP_REACH for mpls-vpn in conf-addpath).

## Resolved

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
