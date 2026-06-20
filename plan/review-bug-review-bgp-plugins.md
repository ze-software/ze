# BGP Plugins and Protocol Codecs Bug Review

## Summary

Reviewed child 4 scope from `plan/spec-bug-review-4-bgp-plugins-and-protocol-codecs.md` against the inventory in `plan/review-bug-review-inventory.md`. No production code, tests, generated files, docs, or specs were changed.

Confirmed findings:

| ID | Severity | Owner | Short description |
|----|----------|-------|-------------------|
| BPLUG-001 | ISSUE | `internal/component/bgp/plugins/nlri/{labeled,mup,vpls,mvpn}` | NLRI encode/config parsers silently ignore unknown or dangling tokens. |
| BPLUG-002 | ISSUE | `internal/component/bgp/plugins/nlri/srpolicy` | SR-Policy is registered as an NLRI family but is not wired into the canonical `ze bgp encode` route encoder path. |

Plausible but not promoted:

| ID | Owner | Reason |
|----|-------|--------|
| BPLUG-P1 | `internal/component/bgp/plugins/nlri/ls` | BGP-LS lacks `InProcessNLRIDecoder` in registration, but CLI has a subprocess/direct fallback and route JSON has `AppendJSON`. Needs an end-to-end failing command before fix-spec promotion. |
| BPLUG-P2 | `internal/component/bgp/plugins/nlri/{mvpn,rtc,ls}` | Registered decode/config-only families lack some encode chain links. This is a feature-completeness gap if the project expects every registered family to support encode, but not enough source evidence proves these families are intended to be user-encodable today. |

## Scope and files read

Required context read:

- `plan/spec-bug-review-4-bgp-plugins-and-protocol-codecs.md:1-499`
- `plan/review-bug-review-inventory.md:1-153`
- `plan/spec-bug-review-0-umbrella.md:1-123`
- `plan/spec-bug-review-1-inventory-and-self-containment.md:1-183`
- `plan/spec-bug-review-3-bgp-engine-core.md:1-153`
- `skill://ze-review`
- `skill://ze-hunt`
- `ai/rules/plugin-design.md:1-438`
- `ai/rules/plugin-self-containment.md:1-189`
- `ai/patterns/bgp-family.md:1-228`
- `ai/patterns/registration.md:1-183`
- `ai/rules/buffer-first.md:1-69`
- `ai/rules/memory-architecture.md:1-340`
- `ai/rules/no-sprintf-alloc.md:1-300`
- `rfc/short/rfc9830.md:1-262`
- `rfc/short/draft-ietf-bess-mup-safi.md:1-300`
- `rfc/short/rfc8277.md:1-300`

Inventory and registry evidence read:

- `internal/component/plugin/all/all.go:12-179,232-249`
- `internal/component/plugin/registry/registry.go:470-510,619-640,682-728`
- `internal/component/bgp/cli/encode.go:120-170,187-230`
- `internal/component/bgp/cli/decode_mp.go:145-303`
- `internal/component/bgp/cli/decode_plugin.go:150-230,430-455`
- `internal/component/bgp/format/text_json.go:219-263`
- `internal/component/bgp/plugins/cmd/update/update_text_nlri.go:303-405`

BGP plugin registration files read:

- Runtime/state plugins: `rib/register.go:1-70`, `adj_rib_in/register.go:1-60`, `rs/register.go:1-55`, `rr/register.go:1-50`, `rpki/register.go:1-60`, `bmp/register.go:1-50`, `route_refresh/register.go:1-75`, `persist/register.go:1-50`, `watchdog/register.go:1-60`
- Capability/small plugins: `capa/register.go:1-50`, `aigp/register.go:1-55`, `hostname/register.go:1-60`, `llnh/register.go:1-50`, `softver/register.go:1-55`, `rpki_decorator/register.go:1-45`, `redistribute_egress/register.go:1-76`
- Filter plugins: `filter_community/register.go:1-80`, `role/register.go:1-80`, `gr/register.go:1-90`, `redistribute_ingress/register.go:1-70`, `filter_aspath/register.go:1-40`, `filter_prefix/register.go:1-40`, `filter_modify/register.go:1-40`, `filter_irr/register.go:1-55`, `filter_remove_private_as/register.go:1-35`, `filter_community_match/register.go:1-35`, `filter_aspath_length/register.go:1-35`
- NLRI plugins: `nlri/evpn/register.go:1-80`, `nlri/flowspec/register.go:1-80`, `nlri/labeled/register.go:1-80`, `nlri/ls/register.go:1-80`, `nlri/mup/register.go:1-80`, `nlri/mvpn/register.go:1-80`, `nlri/rtc/register.go:1-80`, `nlri/srpolicy/register.go:1-80`, `nlri/vpls/register.go:1-80`, `nlri/vpn/register.go:1-85`

Candidate source read:

- `internal/component/bgp/plugins/nlri/labeled/encode.go:108-163`
- `internal/component/bgp/plugins/nlri/mup/config.go:1-119`
- `internal/component/bgp/plugins/nlri/mup/encode.go:57-123`
- `internal/component/bgp/plugins/nlri/mvpn/config.go:36-176`
- `internal/component/bgp/plugins/nlri/srpolicy/config.go:37-157`
- `internal/component/bgp/plugins/nlri/srpolicy/register.go:18-43`
- `internal/component/bgp/plugins/nlri/vpls/encode.go:33-105,148-190`
- `internal/component/bgp/plugins/role/config.go:1-233`
- `internal/component/bgp/plugins/role/role.go:135-197`
- `internal/component/bgp/plugins/bmp/bmp.go:636-916`

Searches executed over `internal/component/bgp/plugins`, `internal/component/bgp/filterapi`, and `internal/component/bgp/attribute`:

- `registry.Register`, `pluginserver.RegisterRPCs`, `filterapi.Register`, attribute formatter/modifier registrations
- Known bug hunts H1, H2, H3, H4 shapes from `skill://ze-hunt`
- `RouteEncoderByFamily`, `EncodeNLRIByFamily`, `ConfigRouteParserByFamily`, `InProcessRouteEncoder`
- Command/YANG wire-method registrations
- `RawMessage`, `WireUpdate`, `AttrsWire` lifetime candidates

## Wiring/coverage audit table

### Package coverage by inventory class

| Class | Assigned packages or directories | Evidence | Result |
|-------|----------------------------------|----------|--------|
| Direct BGP plugins | `adj_rib_in`, `aigp`, `bmp`, `capa`, `filter_*`, `gr`, `healthcheck`, `hostname`, `llnh`, `persist`, `redistribute_*`, `rib`, `role`, `route_refresh`, `rpki`, `rpki_decorator`, `rr`, `rs`, `softver`, `watchdog` | `internal/component/plugin/all/all.go:141-179`; per-plugin `register.go` rows read | Covered. |
| NLRI families | `evpn`, `flowspec`, `labeled`, `ls`, `mup`, `mvpn`, `rtc`, `srpolicy`, `vpls`, `vpn` | `all.go:158-167`; per-family `register.go` rows read | Covered, with family matrix below. |
| BGP command packages | `cmd/cache`, `cmd/commit`, `cmd/monitor`, `cmd/peer`, `cmd/policy`, `cmd/raw`, `cmd/rib`, `cmd/update` | `all.go:17-24,239-246`; RPC/YANG search evidence | Covered, no confirmed unwired handler. |
| BGP plugin schemas | `adj_rib_in/yang`, `bmp/yang`, `cmd/{cache,commit,monitor,peer,policy,raw,rib,update}/yang`, `filter_{aspath,aspath_length,community,community_match,irr,modify,prefix,remove_private_as}/yang`, `gr/yang`, `healthcheck/yang`, `hostname/yang`, `llnh/yang`, `rib/yang`, `role/yang`, `route_refresh/yang`, `rpki/yang`, `rpki_decorator/yang`, `rr/yang`, `rs/yang`, `softver/yang` | `all.go:15-44`; register files and YANG wire-method search | Covered. |
| Plugin RPC handlers | `bmp`, `cmd/{cache,commit,monitor,peer,policy,raw,rib,update}`, `filter_irr`, `route_refresh/handler`, `rr` | `all.go:238-249`; `pluginserver.RegisterRPCs` search | Covered. |
| Attribute and filter surfaces | `aigp`, `filter_community`, `gr`, `role`, NLRI attr-name owners | `filterapi` and `attribute` registration search | Covered, matrix below. |

### Assigned package ledger

| Inventory row | Package | Primary evidence read | Status |
|---------------|---------|-----------------------|--------|
| PLUG-BGP | `internal/component/bgp/plugins/adj_rib_in` | `adj_rib_in/register.go:1-60`, `rib.go` search/read candidates | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/aigp` | `aigp/register.go:1-55` | Covered. |
| PLUG-BGP/RPC-BGP | `internal/component/bgp/plugins/bmp` | `bmp/register.go:1-50`, `bmp.go:636-916`, `cmd_show.go` search | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/capa` | `capa/register.go:1-50` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_aspath` | `filter_aspath/register.go:1-40` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_aspath_length` | `filter_aspath_length/register.go:1-35` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_community` | `filter_community/register.go:1-80` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_community_match` | `filter_community_match/register.go:1-35` | Covered. |
| PLUG-BGP/RPC-BGP | `internal/component/bgp/plugins/filter_irr` | `filter_irr/register.go:1-55`, `cmd_irr.go` search, YANG search | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_modify` | `filter_modify/register.go:1-40` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_prefix` | `filter_prefix/register.go:1-40` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/filter_remove_private_as` | `filter_remove_private_as/register.go:1-35` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/gr` | `gr/register.go:1-90`, `gr` lock/default search | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/healthcheck` | `healthcheck/register.go:1-41` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/hostname` | `hostname/register.go:1-60` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/llnh` | `llnh/register.go:1-50` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/persist` | `persist/register.go:1-50`, `persist/server.go` lock-search hit | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/redistribute_egress` | `redistribute_egress/register.go:1-76` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/redistribute_ingress` | `redistribute_ingress/register.go:1-70` | Covered. |
| PLUG-BGP/RPC-BGP | `internal/component/bgp/plugins/rib` and `rib/events`, `rib/pool`, `rib/storage` | `rib/register.go:1-70`, RIB EventBus search, command search, pool/storage default-search hits | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/role` | `role/register.go:1-80`, `role/config.go:1-233`, `role/role.go:135-197` | Covered. |
| PLUG-BGP/RPC-BGP | `internal/component/bgp/plugins/route_refresh` and `route_refresh/handler` | `route_refresh/register.go:1-75`, handler file inventory and RPC import `all.go:248` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/rpki` | `rpki/register.go:1-60`, default/nil searches over RPKI files | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/rpki_decorator` | `rpki_decorator/register.go:1-45` | Covered. |
| PLUG-BGP/RPC-BGP | `internal/component/bgp/plugins/rr` | `rr/register.go:1-50`, RPC import `all.go:249`, RR default/lock search | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/rs` | `rs/register.go:1-55`, server/worker default and optional-dependency searches | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/softver` | `softver/register.go:1-55` | Covered. |
| PLUG-BGP | `internal/component/bgp/plugins/watchdog` | `watchdog/register.go:1-60` | Covered. |
| PLUG-BGP/NLRI | `internal/component/bgp/plugins/nlri/{evpn,flowspec,labeled,ls,mup,mvpn,rtc,srpolicy,vpls,vpn}` | all NLRI `register.go` files read, candidate encode/config files read | Covered in family-chain matrix. |
| SCHEMA-BGP-CMD/RPC-BGP | `internal/component/bgp/plugins/cmd/{cache,commit,monitor,peer,policy,raw,rib,update}` | RPC/YANG wire-method search and command matrix | Covered. |

### NLRI family-chain matrix

Legend: yes = wired in registration or source read; no = missing in registration; n/a = no implementation found or not expected from source evidence.

| Family plugin | Families | Family registration and splitter | In-process decode | NLRI encode | Route encoder for `ze bgp encode` | Config route parser | JSON route display | Named tests found | Result |
|---------------|----------|----------------------------------|-------------------|-------------|-----------------------------------|--------------------|--------------------|-------------------|--------|
| `evpn` | `l2vpn/evpn` | yes, registration | yes, `register.go:30` | yes, `register.go:31` | yes, `register.go:32` | no | yes, `json.go` | unit plugin/encode tests | Cleared for wiring. |
| `flowspec` | `ipv4/flow`, `ipv6/flow`, `ipv4/flow-vpn`, `ipv6/flow-vpn` | yes | yes, `register.go:30` | yes, `register.go:31` | yes, `register.go:32` | yes, `register.go:33` | yes, `json.go` | `config_test.go`, encode/decode tests | Cleared for wiring. |
| `labeled` | `ipv4/mpls-label`, `ipv6/mpls-label` | yes | yes, `register.go:28` | yes, `register.go:29` | yes, `register.go:30` | no | yes, `json.go` | `encode_label_test.go`, update text decode checks | Finding BPLUG-001 for input strictness. |
| `ls` | `bgp-ls/bgp-ls`, `bgp-ls/bgp-ls-vpn` | yes | no direct `InProcessNLRIDecoder`; SDK `OnDecodeNLRI` exists in `plugin.go:45-64` | no | no | no | yes, `json.go` | decode CLI tests | Plausible BPLUG-P1. |
| `mup` | `ipv4/mup`, `ipv6/mup` | yes | yes, `register.go:26` | yes, `register.go:27` | yes, `register.go:28` | yes, `register.go:29` | yes, `json.go` | `config_test.go`, encode tests | Finding BPLUG-001 for input strictness. |
| `mvpn` | `ipv4/mvpn`, `ipv6/mvpn` | yes | yes, `register.go:31` | no | no | yes, `register.go:32` | yes, `json.go` | `config_test.go` | Finding BPLUG-001 for input strictness, plausible encode gap BPLUG-P2. |
| `rtc` | `ipv4/rtc` | yes | yes, `register.go:27` | no | no | no | yes, `json.go` | decode/type/fuzz tests | Plausible BPLUG-P2 only. |
| `srpolicy` | `ipv4/sr-policy`, `ipv6/sr-policy` | yes, `family.MustRegister` and `nlrisplit.Register` in `register.go:20-26` | yes, `register.go:34` | no | no | yes, `register.go:35` | yes, `json.go` | `test/decode/bgp-srpolicy-*.ci`, config/unit tests | Finding BPLUG-002. |
| `vpls` | `l2vpn/vpls` | yes | yes, `register.go:27` | yes, `register.go:28` | yes, `register.go:29` | yes, `register.go:30` | yes, `json.go` | `config_test.go`, encode tests | Finding BPLUG-001 for input strictness. |
| `vpn` | `ipv4/mpls-vpn`, `ipv6/mpls-vpn` | yes | yes, `register.go:34` | yes, `register.go:35` | yes, `register.go:36` | no | yes, `json.go` | update text tests | Cleared for wiring. |

### Filter, attribute, and capability surface matrix

| Owner | Registry surfaces | Evidence | Result |
|-------|-------------------|----------|--------|
| `filter_community` | attr mod handlers, JSON formatters, filterapi policy ingress/egress, plugin registry, YANG | `filter_community/register.go:13-45` | Covered. |
| `role` | OTC attribute name, attr mod handler, filterapi annotation ingress/egress, capability code 9, YANG, plugin registry | `role/register.go:14-61` | Covered. Invalid config candidate rejected below. |
| `gr` | LLGR community names, filterapi egress annotation, capability codes 64/71, metrics, YANG | `gr/register.go:17-78` | Covered. |
| `redistribute_ingress` | filterapi policy ingress, plugin registry | `redistribute_ingress/register.go:14-35` | Covered. |
| Named filters | `as-path-list`, `as-path-length`, `community-match`, `modify`, `prefix-list`, `remove-private-as` | respective `register.go` files | Covered. |
| `filter_irr` | plugin registry, YANG config and command RPCs | `filter_irr/register.go:13-36`, `cmd_irr.go:20-51` from search | Covered. |
| `aigp` | AIGP JSON formatter and plugin registry | `aigp/register.go:13-45` | Covered. |
| NLRI attribute-name owners | Prefix-SID, BGP-LS, PMSI Tunnel, Tunnel Encapsulation | `labeled/register.go:17-18`, `ls/register.go:17-18`, `mvpn/register.go:17-18`, `vpn/register.go:17-18` | Covered. |
| Capabilities | core capa, route refresh, GR/LLGR, role/OTC, LLNH, hostname, software-version | registration files above | Covered. |

### Command and RPC wiring matrix

| Command package | YANG schema evidence | RPC handler evidence | Result |
|-----------------|----------------------|----------------------|--------|
| `cmd/cache` | `ze-cli-cache-cmd.yang` registers `ze-bgp:cache-*` | `cache.go:30-35` | Wired. |
| `cmd/commit` | `ze-cli-commit-cmd.yang` registers `ze-bgp:commit` | `commit.go:58-60` | Wired. |
| `cmd/monitor` | `ze-monitor-cmd.yang` registers `ze-bgp:monitor` | `monitor.go:37-46` | Wired. |
| `cmd/peer` | `ze-peer-cmd.yang` registers summary, peer ops, health, delete/update | `peer.go:36-48`, `health.go:13-16`, `summary.go:26-31`, `session.go:11-14` | Wired, including `ze-bgp:peer-rib` via `cmd/rib`. |
| `cmd/policy` | `ze-policy-cmd.yang` registers `ze-show:policy-chain/test` | `handler.go` registration search at `register.go:9-18` | Wired. |
| `cmd/raw` | `ze-raw-cmd.yang` registers `ze-bgp:peer-raw` | `raw.go:27-30` | Wired. |
| `cmd/rib` | `ze-rib-cmd.yang`, `ze-rib-poolstats-cmd.yang` | `rib.go:36-49`, `pool_stats.go:22-25` | Wired. |
| `cmd/update` | `ze-update-cmd.yang` registers `ze-bgp:peer-update` | `update_text.go:656-660` | Wired. |
| `bmp` commands | `ze-bmp-cmd.yang` registers `ze-show:bmp-*` | `cmd_show.go:20-40` | Wired. |
| `filter_irr` commands | `ze-filter-irr-cmd.yang` registers `ze-show:irr-*`, `ze-update:irr-*` | `cmd_irr.go:20-51` | Wired. |
| `route_refresh/handler` | `ze-refresh-cmd.yang`, `ze-route-refresh-api.yang` | `all.go:248` and handler package files found | Covered. |
| `rr` commands | `ze-rr-cmd.yang` | `all.go:249` and `rr/cmd_show.go` found by file inventory | Covered. |

### RIB, Adj-RIB-In, RS, RR, RPKI, BMP, GR flow matrix

| Flow | Evidence | Result |
|------|----------|--------|
| RIB EventBus best-change | namespace registration `rib/register.go:18-20`; bus injection `rib/register.go:38-41`; subscription `rib.go:632-635`; emit sites `rib_structured.go:300-304`, command emit search | Covered. |
| Adj-RIB-In structured UPDATE | `adj_rib_in/rib.go:229-263`, `installStructuredNLRIs` lock contract `rib.go:304-306` | Covered. |
| RS optional dependency fallback | optional dep declared `rs/register.go:21-25`; server handlers and worker files reviewed by search for default and lock candidates | Covered, no confirmed bug. |
| RR dependency on Adj-RIB-In | hard dep `rr/register.go:13-18`; withdrawal map lock candidates reviewed | Covered, no confirmed bug. |
| RPKI and RPKI decorator | hard dep on `bgp-adj-rib-in` `rpki/register.go:21-23`; decorator event type `rpki_decorator/register.go:13-20` | Covered, no confirmed bug. |
| BMP zero-copy/raw-message lifetime | Raw bytes used synchronously in `bmp.go:770-828`; OPEN copies cached at `bmp.go:702-710`; reactor comment says UPDATE `RawBytes` is zero-copy valid during callback at `reactor_notify.go:330-339` from search output | Cleared. |
| GR/LLGR | capability/filter registration `gr/register.go:17-78`; RFC summaries read for GR/LLGR names, no candidate promoted | Covered. |
| Redistribute | ingress filter registration `redistribute_ingress/register.go:14-35`; egress consumer registered on started `redistribute_egress/register.go:53-66` | Covered. |

## Confirmed findings

### BPLUG-001: NLRI encode and config parsers silently ignore unknown or dangling tokens

**Severity:** ISSUE

**Owner:** `internal/component/bgp/plugins/nlri/labeled`, `internal/component/bgp/plugins/nlri/mup`, `internal/component/bgp/plugins/nlri/vpls`, `internal/component/bgp/plugins/nlri/mvpn`

**File and line evidence:**

- `internal/component/bgp/plugins/nlri/labeled/encode.go:120-153` loops over `args` and handles `prefix`, `label`, and `path-id` without a `default` branch. Unknown tokens and their following values are skipped.
- `internal/component/bgp/plugins/nlri/mup/encode.go:63-123` loops over `args` and handles known keys without a `default` branch. Unknown tokens and dangling keys are skipped.
- `internal/component/bgp/plugins/nlri/mup/config.go:109-115` copies only known MUP config keys into encoder args and silently drops every other key. The `for i := 2; i+1 < len(content); i += 2` condition also silently drops a dangling final token.
- `internal/component/bgp/plugins/nlri/vpls/encode.go:43-97` handles VPLS encoder keys without a `default` branch. Its route-command parser is stricter at `vpls/encode.go:153-187`, so the in-process encoder is the inconsistent surface.
- `internal/component/bgp/plugins/nlri/mvpn/config.go:59-75` handles `rp/source`, `group`, `rd`, and `source-as` without a `default` branch. Unknown key/value pairs and a dangling final token are ignored.
- Reachability: `internal/component/plugin/registry/registry.go:708-715` dispatches `EncodeNLRIByFamily` to the registered in-process encoder. `internal/component/bgp/plugins/cmd/update/update_text_nlri.go:341-350` uses that registry path for update-text NLRI encoding. `internal/component/bgp/config/bgp_routes.go:144-148` from search output delegates plugin route config to `ConfigRouteParserByFamily`.

**Reachable trigger:**

- RPC/API path: a caller invokes `ze-plugin-engine:encode-nlri` for a registered family with valid required fields plus an unknown key, for example labeled unicast args containing `prefix`, `label`, and `bogus value`. The dispatcher in `internal/component/bgp/server/codec.go:53-56` reaches `registry.EncodeNLRIByFamily`, which reaches the plugin encoder.
- CLI/update path: `ze bgp peer <selector> update text ... nlri ipv4/mpls-label add prefix 10.0.0.0/24 label 100 bogus value` reaches `encodeViaRegistry` for labeled unicast.
- Config path: BGP route config for `ipv4/mup` or `ipv4/mvpn` containing valid required fields plus `typo value`, or ending with a dangling key, reaches the plugin config parser and silently drops the bad input.

**Expected behavior:**

Config, CLI, and RPC input must be exact-or-reject. Unknown family-specific keys and missing values should return an error naming the bad token, as stricter neighboring parsers already do, for example SR-Policy at `srpolicy/config.go:154-155` and VPLS route-command parsing at `vpls/encode.go:185-186`.

**Actual behavior:**

The affected parsers either skip unknown keys entirely or stop before a dangling final token. Required fields may still be present, so the route/NLRI is encoded and sent while the operator's extra token is ignored.

**Impact:**

A mistyped route attribute or family-specific field can be silently lost. For protocol families such as MUP and MVPN, that can advertise a different route than configured, or omit source/TEID/group constraints, with no load-time or command-time failure. This is not compiler or linter catchable because the bad input is runtime config/RPC/CLI data.

**RFC status:**

No direct RFC wire violation was confirmed. `draft-ietf-bess-mup-safi.md` was read for MUP route type and field constraints, including required MUP NLRI field semantics at lines 22-43 and validation requirements at lines 257-273. The bug is input validation and operator-safety, not an on-wire MUST violation. RFC 8277 was read for labeled-unicast semantics and did not require accepting unknown local command keys.

**Existing tests and interop evidence named:**

- `internal/component/bgp/plugins/nlri/labeled/encode_label_test.go:14-22` covers label-stack encode bytes but not unknown-token rejection.
- `internal/component/bgp/plugins/nlri/mup/config_test.go:17-23,31-102` covers MUP parser registration and valid/error field parsing but not unknown-token rejection.
- `internal/component/bgp/plugins/nlri/mvpn/config_test.go:17-23,31-82` covers MVPN parser registration, valid NLRI bytes, and malformed known fields but not unknown-token rejection.
- `internal/component/bgp/plugins/nlri/vpls/config_test.go:17-21,31-69` covers VPLS parser registration and valid NLRI bytes but not unknown-token rejection.
- Functional fixtures found by file inventory: `test/decode/bgp-mup-1.ci`, `test/encode/srv6-mup.ci`, `test/encode/srv6-mup-v3.ci`, `test/decode/bgp-mvpn-1.ci`, `test/encode/mvpn.ci`, and `test/decode/bgp-vpls-1.ci`.

**Regression test plan:**

- Add unit tests in each owner package:
  - `labeled`: `EncodeNLRIHex("ipv4/mpls-label", []string{"prefix","10.0.0.0/24","label","100","bogus","1"})` must error.
  - `mup`: `EncodeNLRIHex` and `parseConfigRoute` must error on unknown keys and dangling final keys.
  - `vpls`: `EncodeNLRIHex` must error on unknown keys and dangling final keys.
  - `mvpn`: `parseConfigRoute` must error on unknown keys and dangling final keys.
- Add a functional config parse test under the BGP route config coverage that includes a misspelled MUP or MVPN token and expects load failure.
- Add an update-text regression for labeled unicast through `ze-bgp:peer-update` or the existing update text parser to prove unknown tokens fail before any route is sent.

### BPLUG-002: SR-Policy family is not wired into the canonical `ze bgp encode` route encoder path

**Severity:** ISSUE

**Owner:** `internal/component/bgp/plugins/nlri/srpolicy`

**File and line evidence:**

- `internal/component/bgp/plugins/nlri/srpolicy/register.go:28-37` registers `bgp-nlri-srpolicy` with `SupportsNLRI`, both families, `InProcessNLRIDecoder`, and `InProcessConfigRouteParser`, but no `InProcessNLRIEncoder` and no `InProcessRouteEncoder`.
- `internal/component/bgp/cli/encode.go:139-155` sends every non-unicast family to `registry.RouteEncoderByFamily(canonicalFamily)` and returns `unsupported family` when the family has no route encoder.
- `internal/component/plugin/registry/registry.go:720-727` returns nil when a registered family has no `InProcessRouteEncoder`.
- `internal/component/plugin/registry/registry.go:708-715` similarly returns `no NLRI encoder for family` when no `InProcessNLRIEncoder` exists.
- `internal/component/bgp/plugins/nlri/srpolicy/config.go:40-44` and `srpolicy/register.go:35` prove the family already has a config route parser that can build SR-Policy routes from user route content.
- `test/decode/bgp-srpolicy-1.ci:8-10` and `test/decode/bgp-srpolicy-2.ci:8-10` cover decode only. `test/exabgp-compat/encoding/conf-sr-policy.ci:8-19` proves SR-Policy encoding exists in the ExaBGP compatibility path, not the canonical `ze bgp encode` path.

**Reachable trigger:**

Run the canonical encoder for SR-Policy, for example `ze bgp encode --family ipv4/sr-policy 'route ...'` with SR-Policy route content matching the config parser. `parseEncodingFamily` accepts registered families via `family.LookupFamily` at `encode.go:188-193`, then the non-unicast branch calls `RouteEncoderByFamily` at `encode.go:148-154`. Because `srpolicy/register.go:28-37` does not provide an encoder, the command returns `unsupported family`.

**Expected behavior:**

A registered NLRI family that has decode, splitter, JSON, config parser, and ExaBGP compatibility encoding should have the same family-chain link as EVPN, FlowSpec, MUP, VPLS, VPN, and labeled unicast: `InProcessNLRIEncoder` for NLRI-only encoding and `InProcessRouteEncoder` for `ze bgp encode`.

**Actual behavior:**

SR-Policy is partially reachable. Decode and config route parsing work, but the canonical encode route fails at registry lookup because the route encoder field is nil.

**Impact:**

Operators and tests cannot use `ze bgp encode` to produce SR-Policy UPDATEs, even though SR-Policy is a registered family and the repository has ExaBGP compatibility fixtures for SR-Policy encoding. This creates a split-brain feature surface: config/compat paths can build SR-Policy, while the normal BGP encode CLI reports it unsupported.

**RFC status:**

`rfc/short/rfc9830.md` was read. Relevant SR-Policy wire facts are SAFI 73 and NLRI format at lines 19-47, MP_REACH/MP_UNREACH carriage at lines 21 and 48-58, and Tunnel Type 15 encoding at lines 60-80. No direct RFC MUST violation is in the current code path because the path refuses to encode. The defect is family-chain completeness and command wiring.

**Regression test plan:**

- Add unit tests in `internal/component/bgp/plugins/nlri/srpolicy` asserting `registry.EncodeNLRIByFamily("ipv4/sr-policy", ...)` and `registry.RouteEncoderByFamily("ipv4/sr-policy")` are non-nil and produce the expected NLRI bytes.
- Add `test/encode/bgp-srpolicy-1.ci` and `test/encode/bgp-srpolicy-2.ci` covering IPv4 and IPv6 SR-Policy through `ze bgp encode`, with hex matched against RFC 9830 field layout.
- Add a command/parser regression if the final user syntax differs from config syntax, so `ze bgp encode` and config route parsing stay in sync.

## Plausible findings

### BPLUG-P1: BGP-LS registration lacks direct in-process NLRI decoder

**Severity:** Plausible ISSUE, needs more evidence before fix-spec promotion
**Classification:** Plausible finding, not accepted for fix-spec creation without an end-to-end failing path.


**Owner:** `internal/component/bgp/plugins/nlri/ls`

**File and line evidence:**

- `internal/component/bgp/plugins/nlri/ls/register.go:20-34` registers `bgp-nlri-ls` with families and `InProcessDecoder`, but not `InProcessNLRIDecoder`.
- `internal/component/bgp/plugins/nlri/ls/plugin.go:45-64` does register an SDK `OnDecodeNLRI` callback.
- `internal/component/plugin/registry/registry.go:686-701` returns `no NLRI decoder for family` when `InProcessNLRIDecoder` is nil.

**Reachable trigger:**

A direct caller of `registry.DecodeNLRIByFamily("bgp-ls/bgp-ls", hexData)` receives `no NLRI decoder for family`. The candidate becomes a confirmed user bug only if a production command or API path requires that direct registry fast path without subprocess/direct fallback.

**Expected behavior:**

Per `ai/patterns/bgp-family.md:61-78`, an internal NLRI plugin that registers families for decode should expose `InProcessNLRIDecoder` so the registry path is complete.

**Actual behavior:**

The registration omits `InProcessNLRIDecoder`, while the SDK plugin callback exists in `plugin.go`.

**Impact:**

Internal code that relies on `registry.DecodeNLRIByFamily` for all registered families cannot decode BGP-LS through that common path. This could produce raw or undecoded BGP-LS output in a path that does not use CLI fallback.

**Why not confirmed:**

`internal/component/bgp/cli/decode_mp.go:286-305` falls back to plugin subprocess/direct decode when the registry fast path is missing, and BGP-LS concrete NLRI types implement `AppendJSON` (`ls/json.go` found by search), so normal route rendering has another path. I did not verify a user-visible command that fails end-to-end because of this missing direct field.

**RFC status:**

Protocol-related but no RFC violation confirmed. The issue is registry completeness, not BGP-LS wire parsing.

**Regression test plan:**

Add a unit test that imports `bgp-nlri-ls` and asserts `registry.DecodeNLRIByFamily("bgp-ls/bgp-ls", <valid hex>)` returns decoded JSON, plus a `ze bgp decode --nlri bgp-ls/bgp-ls` functional test in an environment where the in-process registry path is the asserted route.

### BPLUG-P2: Some registered NLRI families are decode/config-only while the checklist expects full encode links

**Severity:** Plausible ISSUE, needs more evidence before fix-spec promotion
**Classification:** Plausible finding, not accepted for fix-spec creation without a product decision that these families must support canonical encode.


**Owner:** `internal/component/bgp/plugins/nlri/{mvpn,rtc,ls}`

**File and line evidence:**

- `internal/component/bgp/plugins/nlri/mvpn/register.go:20-36` has decode and config parser, but no `InProcessNLRIEncoder` or `InProcessRouteEncoder`.
- `internal/component/bgp/plugins/nlri/rtc/register.go:16-31` has decode only, but no encode route links.
- `internal/component/bgp/plugins/nlri/ls/register.go:20-34` has decode-mode infrastructure only, but no direct route encode links.
- `internal/component/bgp/cli/encode.go:139-155` routes every non-unicast family to `registry.RouteEncoderByFamily`; nil means `unsupported family`.

**Reachable trigger:**

Run `ze bgp encode --family ipv4/mvpn ...`, `ze bgp encode --family ipv4/rtc ...`, or `ze bgp encode --family bgp-ls/bgp-ls ...` with an intended grammar. The command reaches `RouteEncoderByFamily` and fails if no encoder is registered. The missing piece is proof that these families are intended to have a user-facing encode grammar today.

**Expected behavior:**

If these registered families are expected to support encode, each should follow the family-chain pattern with `InProcessNLRIEncoder`, `InProcessRouteEncoder`, route parser, and encode functional coverage.

**Actual behavior:**

The registrations are decode-only or decode/config-only. MVPN has a config parser that can build route bytes, which suggests partial encode support exists outside the canonical encode command.

**Impact:**

Operators would see registered families in family inventory/decode/config surfaces but receive `unsupported family` from the canonical encode command. That splits feature behavior across CLI, config, and compatibility paths.

**Why not confirmed:**

I could not prove from source that BGP-LS, RTC, or MVPN are intended to support `ze bgp encode` today. MVPN is the strongest candidate because it already builds route bytes from config, but command grammar and encode functional tests were not found in the inspected evidence.

**RFC status:**

No RFC MUST violation confirmed. This is a family-chain completeness candidate. RFC summary for MVPN was not present locally, so no MVPN protocol-compliance claim is made.

**Regression test plan:**

For every family the product decision marks as encodable, add a registry unit test for `RouteEncoderByFamily`, a `test/encode/*.ci` command test, and a config-vs-encode parity test using the same NLRI fields.

## Rejected candidates with proof

| Candidate | Proof read | Disposition |
|-----------|------------|-------------|
| H1 default branches in FlowSpec parsing silently accept unknown wire component types | `flowspec/types.go` search hit shows default returns `ErrFlowSpecInvalidType`; summary output cited RFC 8955 comment around `types.go:408-411` | Rejected. The wire decoder fails closed. |
| H1 default branches in command parsers | `cmd/cache/cache.go`, `cmd/commit/commit.go`, `cmd/update/update_text.go`, `cmd/raw/raw.go` search hits return explicit errors for unknown actions or tokens | Rejected. Not silent parser fall-through. |
| H2 signed subtraction for sequence ordering | Regex hunt over `internal/component/bgp/plugins` returned no matches | Cleared. |
| H3 `return nil, nil` in fallible functions | Hits in test mocks, empty raw payload helpers, and absent config sections. Role config `extractPeerRoleConfigs` was read at `role/config.go:169-223`; caller handles nil maps without panic at `role/role.go:146-160,166-183` | Rejected. No confirmed production caller loses an error. |
| H4 lock comments without proof | Lock-contract hits found in Adj-RIB-In, GR, persist, RIB, RPKI, RR, and RS. Candidate areas were not promoted because no caller path was verified to call without the documented lock | Needs future concurrency-specific pass if desired, no confirmed finding here. |
| BMP retains zero-copy UPDATE bytes past callback lifetime | `bmp.go:770-828` writes route monitoring/mirroring synchronously and does not store UPDATE bytes; OPEN cache copies bytes at `bmp.go:702-710`; reactor comment says UPDATE `RawBytes` are valid during callback | Rejected. No lifetime escape found. |
| RS optional dependency on Adj-RIB-In should be hard | `rs/register.go:21-25` explicitly declares `OptionalDependencies` and documents one-shot fallback semantics. This matches `ai/rules/plugin-design.md:203-229` | Rejected. Correct optional-dependency shape. |
| `peer-rib` YANG command appears in peer schema without peer-package handler | Search found handler in `internal/component/bgp/plugins/cmd/rib/rib.go:39-40`, owner is RIB command cluster | Rejected. Wired by owning RIB command package. |

## Cleared classes

| Class | Result |
|-------|--------|
| Generated import coverage | All Child 4 direct plugins, NLRI packages, command schemas, and RPC packages were matched to `all.go` rows and package registrations. |
| Plugin registration completeness | Every assigned direct plugin package has a `registry.Register` row or an intentional command/YANG-only role. |
| Command YANG to RPC wiring | No confirmed unwired command after checking `pluginserver.RegisterRPCs` against YANG wire methods. |
| Attribute/filter ownership | Community, OTC, LLGR, AIGP, and NLRI attribute-name registrations are plugin-owned, not central. |
| EventBus typed RIB flow | RIB best-change namespace, typed payload, bus injection, emit sites, and sysrib consumer path were present in source/search evidence. |
| Zero-copy/raw-message lifetime | No confirmed async retention of UPDATE `RawBytes` or `WireUpdate` outside the callback lifetime in BMP/Adj-RIB-In/RIB paths inspected. |
| Known hunt H2 | Signed modular subtraction pattern absent in BGP plugin scope. |
| Known hunt H3 | No production `(nil, nil)` bug confirmed after caller checks. |

## Assumptions resolved

| ID | Resolution | Evidence |
|----|------------|----------|
| A-1 | MUP is a useful full-chain reference, but not the only one. | MUP has decoder, encoder, route encoder, config parser in `mup/register.go:16-33`; EVPN/FlowSpec/VPLS/VPN/labeled provide additional references. |
| A-2 | BGP command schemas under `cmd/` belong to this review. | Inventory assigns `internal/component/bgp/plugins/cmd/*` to Child 4; `all.go:17-24,239-246` wires schema and RPC rows. |
| A-3 | Attribute formatter/modifier ownership follows plugin ownership. | `filter_community/register.go:14-23`, `role/register.go:15-20`, `aigp/register.go:25-27` keep formatter/modifier registration in owners. |
| A-4 | Optional dependencies are intentional only when declared and handled. | `rs/register.go:21-25` uses `OptionalDependencies`; RR/RPKI use hard dependencies when they require Adj-RIB-In. |
| A-5 | Active spec overlaps matter. | SR-Policy and config route parser findings should be routed carefully with `spec-exabgp-compat-sync.md` and `spec-route-config-plugin-migration.md` if child 5 creates fix specs. |

## Remaining risks

- I did not run project-wide build, lint, format, or verification gates, per assignment.
- I did not create fix specs, per assignment.
- RFC summaries are missing for some plugin RFCs in the local tree, for example `rfc6514.md`. MVPN findings here were classified as input validation and feature completeness, not RFC compliance violations.
- The lock-comment H4 sweep found many concurrency contracts. No confirmed caller violation was found in this pass, but a dedicated lock/caller audit could still find a race.
- Decode-only versus full encode support needs a product decision for BGP-LS, RTC, and MVPN. BPLUG-P2 records this without promoting it beyond evidence.
