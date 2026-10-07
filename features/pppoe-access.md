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
| Interop | pppoe/01-pppoe-chap-ipv4, pppoe/02-ze-ac-pppd-client, pppoe/pppoe-empty-service-name, pppoe/pppoe-padr-replay |
| RFCs | rfc2516 |
| Docs | docs/guide/pppoe.md |
| Doc review | 2026-10-07: checked auth-method default chap-md5 and allow-no-auth in internal/component/l2tp/yang/ze-l2tp-conf.yang; every source anchor resolves |
| Defect review | 2026-10-07: open: spec-pppoe-padt-ends-session, spec-pppoe-lcp-option-reject, spec-pppoe-subscribers-produce-no-accounting-or-telemetry; journal rows naming a Component, not each re-verified here: blanket-mechanism-hid-missing-cases.md:8, false-synchronization-claim.md:35, feature-test-missing-build-tag.md:13, gate-excludes-part-of-its-population.md:12, gate-excludes-part-of-its-population.md:140, plugin-startup-barrier-deadlock.md:3, registry-read-outruns-its-lazy-creation.md:11, unwired-feature.md:9, unwired-feature.md:80 |
| Extra criteria | supported: test/pppoe runs under a declared functional suite = test/pppoe/pppoe-basic.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

RFC 2516 PPPoE access concentrator: discovery state machine (PADI/PADO/PADR/PADS/PADT), HMAC-SHA256 AC-Cookie for DoS protection, per-interface session tables with bitmap SID allocation (1-65535), per-source-MAC PADI rate limiting, Service-Name filtering, kernel PPPoE sessions via AF_PPPOX + PX_PROTO_OE, and integration with the transport-agnostic PPP Driver (same auth/pool/shaper plugins as L2TP). Subscriber authentication is per-AC: `auth-method` selects `none`, `pap`, `chap-md5` (the default) or `ms-chap-v2`, and the AC advertises it in its own LCP Configure-Request; `allow-no-auth` is the explicit opt-in that `auth-method none` requires, so an unauthenticated concentrator is never the default. The credential is verified by the same `l2tp-auth-local` or `l2tp-auth-radius` handler L2TP uses. <!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- leaf auth-method, leaf allow-no-auth --> YANG config (`pppoe {}`) with per-interface settings. CLI commands: `show pppoe`, `show pppoe sessions`, `show pppoe statistics`, `show pppoe interfaces`. Runs concurrently with L2TP on the same daemon. Functional tests (`test/pppoe/`, `./le test qemu pppoe-test`) drive a real client over a veth pair inside a per-test network namespace on ze's runtime kernel: PADI to PADO carrying AC-Name and AC-Cookie, PADR to PADS with a kernel AF_PPPOX session and a non-zero session id, a forged AC-Cookie earning no PADS, the same exchange on an 802.1Q sub-interface, and an L2TP SCCRP answered while the AC is bound. Both roles are now interop-tested in `test/interop-pppoe/`: scenario 01 runs Ze as a client against accel-ppp, and scenario 02 runs Ze as the access concentrator with pppd 2.5.1 and the rp-pppoe plugin as the client, asserting discovery, LCP, CHAP-MD5 accept and reject, IPCP address assignment from the pool, ICMP across the session, and a PADT teardown that empties Ze's session table read over its own REST API. `Partial` because PADI rate limiting and Service-Name filtering still have unit coverage only. <!-- source: internal/le/interoplab/pppoe/pppoe.go -- checkZeAccessConcentrator --> <!-- source: internal/component/l2tp/pppoe/subsystem.go -- PPPoE subsystem lifecycle --> <!-- source: internal/component/l2tp/pppoe/server.go -- per-interface discovery dispatch --> <!-- source: internal/component/l2tp/pppoe/discovery.go -- RFC 2516 wire format --> <!-- source: test/pppoe/pppoe-basic.ci -- discovery, cookie rejection --> PPPoE ships inside the `ze_l2tp` build tag (the BNG gate); a stripped build rejects a `pppoe {}` block as unknown. <!-- source: feature-gates.txt -- ze_l2tp -->
