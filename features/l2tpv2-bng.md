# L2TPv2 BNG

## Meta

| Field | Value |
|-------|-------|
| Name | L2TPv2 BNG |
| Page | docs/guide/l2tp.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 2661 MUST gaps open in the ledger, BGP announce and withdraw for subscriber routes are proven only with a synthetic producer, subscriber IPv6 remains a post-release extension |
| Level | experimental |
| Components | internal/component/l2tp |
| Real-path tests | test/l2tp/handshake-full.ci, test/l2tp/session-incoming-lns.ci, test/l2tp/auth-radius-basic.ci, test/l2tp/radius-acct-wire.ci, test/l2tp/radius-nas-port-id.ci, test/l2tp/rfc2661-sccrq-tunnel-id-zero.ci, test/l2tp/session-stopccn-cascade.ci, test/l2tp/shaper-upload-rate.ci |
| Interop | l2tp/01-ppp-ipv4, l2tp/02-ppp-bgp-redistribute-frr, l2tp/03-ze-lac-xl2tpd-lns, l2tp/04-radius-acct-attrs, topology/access-up, topology/access-pool, topology/access-accounting, topology/access-wholesale |
| RFCs | rfc2661, rfc2865, rfc2866, rfc2869, rfc1334, rfc1661, rfc1877, rfc5072, rfc5176 |
| Docs | docs/guide/l2tp.md |
| Doc review | 2026-11-09: the IP address pool paragraph of docs/guide/l2tp.md (IPCP DNS follows the pool's dns-source list, default radius configured received first-usable) matches poolDNSDefault and selectSubscriberDNS in internal/component/l2tp/plugins/pool/dns_source.go and extractMSDNSServers in internal/component/l2tp/plugins/authradius/extract_dns.go; the Configure-Reject of a DNS option the pool holds no server for matches buildNakOrReject in internal/component/l2tp/ppp/ncp.go; the realm relay paragraph now names checkRealmRelayAccel and checkRealmRelayXL2TPD in internal/le/interoplab/topology/check_realm_relay.go and TestLACRealmBridgeAfterLocalLCP and TestLACRealmHandoffRetransmission in internal/component/l2tp/bridge_realm_integration_linux_test.go, which resolve. Earlier, 2026-11-08: the LAC realm relay paragraph checked against LookupRealmRelay and validateRelayRealms (exact, case-folded match, refused bindings), RelayRealm and newICCNProxy (AVPs 26 to 33, no-shared-secret withholds PAP proxy auth), realmFromUsername and decide (last @, no realm or no binding authenticates locally, a matched failed call is refused), sessionJSON (the AC-12 keys) and checkAccessWholesale; every new source anchor resolves. Earlier the same day: the access-chain paragraph checked against checkAccessUp, checkAccessPool and checkAccessAccounting, and the three topology Interop entries resolve; the guide's relay paragraph names access-up as the relay data-path proof; every source anchor resolves (handleTick, nasPortIDFormat, answerRefusedSCCRQ, stopCCNSlot, bngRegister); the tunnel-id-zero StopCCN and NAS-Port-Id each have a listed test |
| Defect review | 2026-10-07: open: spec-radius-rfc-defects; l2tp journal rows name internal/component/l2tp; journal rows naming a Component, not each re-verified here: assertion-reads-state-the-scheduler-owns.md:10, assertion-reads-state-the-scheduler-owns.md:11, blanket-mechanism-hid-missing-cases.md:8, bulk-rename-corruption.md:13, claim-outlives-the-evidence-it-cites.md:21, claim-outlives-the-evidence-it-cites.md:24, claim-outlives-the-evidence-it-cites.md:32, closure-deletes-a-cited-document.md:6, concurrent-session-corruption.md:26, crypto-rand-read-error-branch-is-dead-code.md:3, declared-format-contradicts-payload.md:21, design-claim-never-measured.md:17, false-synchronization-claim.md:35, feature-test-missing-build-tag.md:13, field-carries-two-meanings.md:31, full-disk-false-red.md:27, gate-excludes-part-of-its-population.md:12, gate-excludes-part-of-its-population.md:47, gate-excludes-part-of-its-population.md:96, gate-excludes-part-of-its-population.md:140, gate-excludes-part-of-its-population.md:143, gate-excludes-part-of-its-population.md:146, grace-bound-hides-a-missing-signal.md:3, grace-bound-hides-a-missing-signal.md:4, green-that-could-not-have-been-red.md:179, green-that-could-not-have-been-red.md:206, green-that-could-not-have-been-red.md:207, guard-added-to-one-half-of-a-pair.md:34, guard-added-to-one-half-of-a-pair.md:36, guard-added-to-one-half-of-a-pair.md:38, guard-added-to-one-half-of-a-pair.md:86, guard-added-to-one-half-of-a-pair.md:91, guard-added-to-one-half-of-a-pair.md:92, helper-bypassed-by-an-open-coded-copy.md:14, helper-bypassed-by-an-open-coded-copy.md:36, index-key-derived-from-mutable-field.md:10, lint-contract-not-applied.md:21, mutating-a-guard-runs-the-unguarded-path.md:3, plugin-startup-barrier-deadlock.md:3, reference-checked-claim-unchecked.md:16, reference-checked-claim-unchecked.md:18, registry-contamination.md:7, registry-contamination.md:14, registry-read-outruns-its-lazy-creation.md:11, reply-sent-per-request-with-no-rate-limit.md:16, reply-sized-by-the-peer-it-answers.md:18, rollback-forgets-partial-apply.md:3, rollback-forgets-partial-apply.md:5, silent-fall-through.md:5, silent-fall-through.md:21, silent-fall-through.md:41, silent-fall-through.md:59, silent-fall-through.md:60, silent-fall-through.md:61, stale-spec-claims-done.md:4, fixture-encodes-an-impossible-state.md:45, transient-failure-treated-as-fatal.md:13, tree-state-claim-published-unverified.md:54, unwired-feature.md:7, unwired-feature.md:8, unwired-feature.md:9, unwired-feature.md:21, unwired-feature.md:25, unwired-feature.md:45, unwired-feature.md:54, unwired-feature.md:72, unwired-feature.md:73, unwired-feature.md:76, unwired-feature.md:80, unwired-feature.md:81, unwired-feature.md:121, validated-value-discarded-by-its-caller.md:26, zero-value-as-valid-answer.md:15, zero-value-as-valid-answer.md:68, zero-value-as-valid-answer.md:70 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

New-session PPP policy includes `mru` (68..1500, default 1500) and the ordered
`authentication/allow` set. IPv6CP defaults enabled and requires an MRU of at
least 1280. Disable IPv6CP to configure 68 through 1279 for IPv4-only service.
MRU and authentication policy hot-apply to new sessions; established sessions
retain their policy. `show l2tp config` reports the effective values.
<!-- source: internal/component/l2tp/config.go -- ExtractParameters -->
<!-- source: internal/component/l2tp/config_policy.go -- validateMRUPolicy, validateMRUNCP -->
<!-- source: internal/component/l2tp/subsystem_reload.go -- Reload -->
<!-- source: internal/component/l2tp/cmd/l2tp.go -- handleConfig -->

Native L2TP interop proves the 1492 transmit cap with omitted peer MRU and
accepted proxy MRUs of 1500 and 9000. Those IPv4 sessions authenticate,
carry traffic and release their interfaces. The refusal scenario verifies
required authentication closes before LCP opens.
<!-- source: internal/le/interoplab/l2tp/lns_omitted_mru.go -- checkLNSOmittedMRU -->
<!-- source: internal/le/interoplab/l2tp/proxy_mru.go -- checkProxyMRU -->
<!-- source: internal/le/interoplab/l2tp/lns_policy.go -- checkLNSCHAPOnly -->

The topology scenarios prove the LAC relay data path with Ze in every PPP
role: a Ze CPE dials with no Service-Name, a Ze LAC relays it (`relay ""`)
over L2TP to a Ze LNS, and the LNS checks CHAP against FreeRADIUS.
`access-up` asserts the Framed-IP-Address on the CPE, MRU 1492 and CHAP-MD5
on the LNS session, and a ping across the LAC's kernel channel bridge;
`access-pool` asserts the pool fallback, and `access-accounting` the Start,
Interim-Update and Stop records with the session's octets. Each goes red
when its producer is broken.
<!-- source: internal/le/interoplab/topology/check_access.go -- checkAccessUp, checkAccessPool, checkAccessAccounting -->

The LNS policy for a LAC's proxy LCP (RFC 2661 Section 4.4.5) is the
`proxy-lcp` leaf: `accept`, the default, opens LCP on valid proxy LCP;
`renegotiate` ignores it and negotiates LCP with the subscriber afresh,
then authenticates with the configured method. `show l2tp session id <id>`
names the outcome (`accepted`, `renegotiated`, `none`). Unit and wiring
tests prove it; the pppd interop scenario `l2tp-lns-lcp-renegotiate-pppd`
is not yet run.
<!-- source: internal/component/l2tp/ppp/proxy.go -- admitProxyLCP -->

As a LAC, Ze can choose the LNS from the realm of the subscriber's PPP
username, the way a wholesale access network does: an `l2tp relay-realm
<realm> { remote <name>; }` binding names a declared remote. The realm is the
text after the last `@` of the username, matched exactly once case is
ignored, so `cpe@Example.NET` matches `example.net` and `cpe@x.example.net`
does not. For a matched realm the LAC runs LCP and starts authentication with
the subscriber itself, then places an incoming call to the bound remote whose
ICCN carries the RFC 2661 Section 4.4.5 proxy AVPs: the three LCP
Configure-Requests (AVPs 26 to 28) and the proxy authentication type, name,
challenge, Identifier and response (AVPs 29 to 33), each with M=0. A PAP
password travels only hidden, so with no `shared-secret` on the remote a PAP
subscriber's proxy authentication is left out and the LNS restarts it. Once
the session is up the LAC bridges the subscriber's PPP channel to it and the
LNS carries on the PPP session. A username with no realm, or whose realm
names no binding, is authenticated locally as for any PPPoE session, and a
Service-Name `relay` that matched at discovery wins over the realm. A matched
realm whose call cannot start, and an MS-CHAPv2 subscriber of a matched
realm, are refused, never authenticated locally. `show l2tp session id <id>`
on the LAC adds `relay-realm`, `relay-remote`, `proxy-lcp-sent`,
`proxy-auth-sent` and, when no proxy authentication was sent,
`proxy-auth-reason`. The `access-wholesale` topology scenario proves the relay
with a Ze LNS; the accel-ppp and xl2tpd LNS scenarios are written and not yet
run.
<!-- source: internal/component/l2tp/config.go -- validateRelayRealms, LookupRealmRelay -->
<!-- source: internal/component/l2tp/relay_sink.go -- RelayRealm -->
<!-- source: internal/component/l2tp/iccn_proxy.go -- newICCNProxy -->
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- realmFromUsername, decide -->
<!-- source: internal/component/l2tp/cmd/l2tp.go -- sessionJSON -->
<!-- source: internal/le/interoplab/topology/check_wholesale.go -- checkAccessWholesale -->

The PPP engine's IPv6CP boundary tests negotiate at MRU 1280 with a test
address provider. Shipped subscriber service remains IPv4-only: the static
pool declines IPv6 requests. Subscriber IPv6 traffic and prefix delegation
belong to the post-release IPv6 subscriber spec under the owner's existing
release decision.
<!-- source: internal/component/l2tp/ppp/mru_protocol_floor_test.go -- TestIPv6MinimumMRUCompletesNCP -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- poolPlugin.handle -->
<!-- source: plan/post-release/features/spec-l2tp-ipv6-subscriber.md -- owner ruling, 2026-08-30 -->

RFC 2661 L2TPv2 LNS/LAC with full BNG stack: tunnel lifecycle (SCCRQ/SCCRP/SCCCN, CHAP-MD5 challenge, HELLO keepalive, StopCCN teardown, tie-breaker), configurable dead-peer detection (`hello-retries` x `hello-interval` ACK-driven keepalive timeout, separate from the ~31s reliable-transport retransmit exhaustion, so a peer that dies without StopCCN is torn down fast while an idle-but-alive peer that only ZLB-ACKs HELLOs is not falsely dropped) <!-- source: internal/component/l2tp/reactor.go -- handleTick dead-peer check -->, reliable delivery with sliding window and congestion avoidance, PPP negotiation (LCP 10-state FSM, PAP/CHAP-MD5/MS-CHAPv2 auth, IPCP/IPv6CP address assignment, proxy LCP), kernel data plane via `l2tp_ppp` netlink and PPPoL2TP sockets. Four plugins: auth-local (static users), auth-radius (RADIUS auth/acct/CoA/DM with failover, real per-subscriber traffic counters in Interim/Stop including RFC 2869 Gigawords), pool (bitmap-backed IPv4 ranges from Ze config), shaper (TC TBF/HTB on pppN interfaces with configured defaults and CoA rate updates). Subscriber route redistribution via `redistribute { import l2tp }`: the real RouteObserver emits add/remove route-change batches, and BGP announce/withdraw plumbing is covered with a synthetic producer. CQM monitoring (100s echo RTT/loss buckets, per-login sample rings, 24h retention). Web UI at `/l2tp` with session list, detail page, uPlot CQM graph with SSE live updates, event timeline, and disconnect with audit trail. `ze_l2tp_*` Prometheus metrics (session/tunnel gauges, per-session byte/packet counters, CQM echo RTT histogram, loss ratio, bucket state). `ze_radius_*` metrics (auth/acct/interim counters, server reachability). RADIUS Access-Accept subscriber profile attributes are consumed: Framed-IP-Address bypasses pool (direct IP assignment), Framed-Pool selects a named pool, Session-Timeout/Idle-Timeout enforce session lifetime, Filter-Id sets initial shaping rate at session establishment, Vendor-Specific attributes (RFC 2865 S5.26) extract CoS profiles from Cisco-AVPair/Juniper ERX/Nokia/Huawei VSAs and shaper rates from MikroTik Mikrotik-Rate-Limit (Ze "cos:" Filter-Id takes priority; unknown vendors silently ignored), Acct-Interim-Interval sets the per-session accounting interval for a deployment that left `acct-interval` unset, because RFC 2869 Section 2.1 gives a locally configured value precedence over the Access-Accept, and Framed-Route/Framed-IPv6-Route (RFC 2865 Section 5.22, RFC 6911 Section 3.2) inject per-subscriber static routes into BGP alongside the subscriber /32 or /128. Named pools are configured via YANG `named-pool` list under `l2tp > pool`. PPP auth now defaults to mandatory CHAP-MD5 with finite tunnel/session caps; no-auth requires explicit opt-in, hidden mandatory AVPs are rejected fail-closed. Every Accounting-Request carries Framed-IP-Address, the address IPCP actually assigned (RFC 2866 Section 4.1), and a NAS-Port-Id composed once per session from a `{nas-id}` / `{tunnel-id}` / `{session-id}` template (`nas-port-id-format`, RFC 2869 Section 5.17), so an LNS with no physical port still joins its accounting records. An SCCRQ whose Assigned Tunnel ID AVP carries 0 is a protocol error under RFC 2661 Section 4.4.3: Ze answers it with a StopCCN sent outside any tunnel, allocates no tunnel, and bounds the reply to one per source-address slot per second so a spoofed flood costs nothing. <!-- source: internal/component/l2tp/plugins/authradius/handler.go -- nasPortIDFormat --> <!-- source: internal/component/l2tp/reactor.go -- answerRefusedSCCRQ, stopCCNSlot --> Docker-backed `xl2tpd` evidence covers external LAC control tunnel and incoming-call session setup. A peer-isolated Docker lab (`./le test deployment docker-l2tp-ppp`) proves full PPP LCP/IPCP, kernel pppN creation, dataplane ping, and BGP route redistribution from a live PPP session with Ze LNS, real xl2tpd/pppd LAC, and FRR in separate containers; requires host kernel PPPoL2TP support. Compile-out-able with the `ze_l2tp` build tag (default-on in `ZE_FEATURES`): a stripped build drops the whole BNG subtree (L2TP, PPPoE, subscriber management, local-auth/pool/shaper plugins) and rejects `l2tp {}` and `pppoe {}` blocks; the hub builds the subsystems through the gated `bngRegister` seam, the web session pages and diag l2tp captures answer not-in-this-build, and the cos dynamic RADIUS-CoS handler drops with it. <!-- source: feature-gates.txt -- ze_l2tp --> <!-- source: cmd/ze/hub/bng_infra.go -- bngRegister seam -->
