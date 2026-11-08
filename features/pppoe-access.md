# PPPoE Access

## Meta

| Field | Value |
|-------|-------|
| Name | PPPoE Access |
| Page | docs/guide/pppoe.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | PADI rate limiting and Service-Name filtering have unit coverage only |
| Level | experimental |
| Components | internal/component/l2tp/pppoe |
| Real-path tests | test/pppoe/pppoe-basic.ci, test/pppoe/pppoe-concurrent-l2tp.ci, test/pppoe/pppoe-padr-flood.ci, test/pppoe/pppoe-per-mac-cap.ci, test/pppoe/pppoe-service-name.ci, test/pppoe/pppoe-vlan.ci |
| Interop | pppoe/01-pppoe-chap-ipv4, pppoe/02-ze-ac-pppd-client, pppoe/pppoe-empty-service-name, pppoe/pppoe-padr-replay, topology/access-reconnect |
| RFCs | rfc2516, rfc1334, rfc1661, rfc1877, rfc5072 |
| Docs | docs/guide/pppoe.md |
| Doc review | 2026-11-08: the realm relay paragraph checked against server.go (CaptureProxyLCP from realmRelayConfigured), startPPPoEAuthDrain (decide runs before the auth handler), realmFromUsername, proxyAuthFrom and decide in realm_relay.go (no realm or no binding authenticates locally; a matched failed call or MS-CHAPv2 is refused through RefuseAuth, which terminates LCP) and LookupRealmRelay. Earlier the same day: re-read after the home-CPE lab commit added topology/access-reconnect; checkAccessReconnect clears the LNS session and waits for the CPE's new address, through the pppoe and relay blocks of test/interop-topology/access-chain/ze-lac.conf; the checkZeAccessConcentrator anchor moved to internal/le/interoplab/pppoe/check_ac.go and now says so; auth-method (default chap-md5) and allow-no-auth (default false) in ze-l2tp-conf.yang, ExtractParameters and extractAuthAllow in config.go, applyInterfaceConfig in subsystem_reload.go and PPPoEMaxMTU 1492 in discovery.go hold the described behavior. Earlier, 2026-10-07: checked auth-method default chap-md5 and allow-no-auth in internal/component/l2tp/yang/ze-l2tp-conf.yang; every source anchor resolves |
| Defect review | 2026-10-07: open: spec-pppoe-padt-ends-session, spec-pppoe-lcp-option-reject, spec-pppoe-subscribers-produce-no-accounting-or-telemetry; journal rows naming a Component, not each re-verified here: blanket-mechanism-hid-missing-cases.md:8, false-synchronization-claim.md:35, feature-test-missing-build-tag.md:13, gate-excludes-part-of-its-population.md:12, gate-excludes-part-of-its-population.md:140, plugin-startup-barrier-deadlock.md:3, registry-read-outruns-its-lazy-creation.md:11, unwired-feature.md:9, unwired-feature.md:80 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

RFC 2516 PPPoE access concentrator: discovery state machine (PADI/PADO/PADR/PADS/PADT), HMAC-SHA256 AC-Cookie for DoS protection, per-interface session tables with bitmap SID allocation (1-65535), per-source-MAC PADI rate limiting, Service-Name filtering, kernel PPPoE sessions via AF_PPPOX + PX_PROTO_OE, and integration with the transport-agnostic PPP Driver (same auth/pool/shaper plugins as L2TP). Subscriber authentication is per-AC: `auth-method` selects `none`, `pap`, `chap-md5` (the default) or `ms-chap-v2`, and the AC advertises it in its own LCP Configure-Request; `allow-no-auth` is the explicit opt-in that `auth-method none` requires, so an unauthenticated concentrator is never the default. The credential is verified by the same `l2tp-auth-local` or `l2tp-auth-radius` handler L2TP uses. <!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- leaf auth-method, leaf allow-no-auth --> YANG config (`pppoe {}`) with per-interface settings. CLI commands: `show pppoe`, `show pppoe sessions`, `show pppoe statistics`, `show pppoe interfaces`. Runs concurrently with L2TP on the same daemon. Functional tests (`test/pppoe/`, `./le test qemu pppoe`) drive a real client over a veth pair inside a per-test network namespace on ze's runtime kernel: PADI to PADO carrying AC-Name and AC-Cookie, PADR to PADS with a kernel AF_PPPOX session and a non-zero session id, a forged AC-Cookie earning no PADS, the same exchange on an 802.1Q sub-interface, and an L2TP SCCRP answered while the AC is bound. Both roles are now interop-tested in `test/interop-pppoe/`: scenario 01 runs Ze as a client against accel-ppp, and scenario 02 runs Ze as the access concentrator with pppd 2.5.1 and the rp-pppoe plugin as the client, asserting discovery, LCP, CHAP-MD5 accept and reject, IPCP address assignment from the pool, ICMP across the session, and a PADT teardown that empties Ze's session table read over its own REST API. `Partial` because PADI rate limiting and Service-Name filtering still have unit coverage only. <!-- source: internal/le/interoplab/pppoe/check_ac.go -- checkZeAccessConcentrator --> The `access-reconnect` topology scenario redials through Ze's AC on a LAC that relays the PPPoE session to an L2TP LNS: once the LNS clears the session, the CPE's fresh discovery is answered and the CPE comes back with the new address RADIUS hands out. <!-- source: internal/le/interoplab/topology/check_vpn.go -- checkAccessReconnect --> <!-- source: test/interop-topology/access-chain/ze-lac.conf -- pppoe and relay blocks --> <!-- source: internal/component/l2tp/pppoe/subsystem.go -- PPPoE subsystem lifecycle --> <!-- source: internal/component/l2tp/pppoe/server.go -- per-interface discovery dispatch --> <!-- source: internal/component/l2tp/pppoe/discovery.go -- RFC 2516 wire format --> <!-- source: test/pppoe/pppoe-basic.ci -- discovery, cookie rejection --> PPPoE ships inside the `ze_l2tp` build tag (the BNG gate); a stripped build rejects a `pppoe {}` block as unknown. <!-- source: feature-gates.txt -- ze_l2tp -->

Subscriber PPP policy uses the flat ordered-set leaf `pppoe authentication-allow`.
Absent or empty lists derive `[chap-md5]` for `auth-method chap-md5`,
`[ms-chap-v2]` for `ms-chap-v2`, `[chap-md5 pap]` for `pap`, and no methods
for `none`. Explicit lists must contain `auth-method` and cannot contain
`none` or accompany `auth-method none`. CHAP variants precede PAP; the
configured CHAP variant precedes the remaining variants in written order.
Startup and reload share the policy parser; reload changes only new sessions.
Configurations relying on implicit PAP fallback must explicitly include PAP.
The PPPoE MRU ceiling remains 1492.
<!-- source: internal/component/l2tp/pppoe/config.go -- ExtractParameters, extractAuthAllow -->
<!-- source: internal/component/l2tp/pppoe/subsystem_reload.go -- applyInterfaceConfig -->

On a LAC with an `l2tp relay-realm` binding, the PPPoE AC records each new
session's LCP exchange and, when the subscriber authenticates, asks the L2TP
subsystem to relay it by the realm of its username (the text after the last
`@`, matched exactly once case is ignored) before any auth handler sees the
credential. A matched realm is relayed: the AC has run LCP and started CHAP
or PAP per `authentication-allow`, and L2TP places an incoming call whose
ICCN carries the subscriber's LCP and authentication as proxy AVPs, then the
PPPoE channel is bridged to the L2TP session. A username with no realm, or
whose realm names no binding, is authenticated locally by the configured auth
handler; a matched realm whose call cannot start, and an MS-CHAPv2
subscriber of a matched realm, are refused with an authentication failure
and an LCP Terminate-Request, never authenticated locally. The L2TPv2 BNG
feature carries the L2TP half and the `show l2tp session` keys.
<!-- source: internal/component/l2tp/pppoe/server.go -- CaptureProxyLCP: realmRelayConfigured -->
<!-- source: internal/component/l2tp/pppoe/drain.go -- startPPPoEAuthDrain -->
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- realmFromUsername, proxyAuthFrom, decide -->
<!-- source: internal/component/l2tp/config.go -- LookupRealmRelay -->
