# BGP Protocol

### Address Families

| Family | Config Name | AFI/SAFI | Encode | Decode | Route Config |
|--------|-------------|----------|--------|--------|--------------|
| IPv4 Unicast | `ipv4/unicast` | 1/1 | Yes | Yes | Yes |
| IPv6 Unicast | `ipv6/unicast` | 2/1 | Yes | Yes | Yes |
| IPv4 Multicast | `ipv4/multicast` | 1/2 | Yes | Yes | Yes |
| IPv6 Multicast | `ipv6/multicast` | 2/2 | Yes | Yes | Yes |
| IPv4 VPN | `ipv4/mpls-vpn` | 1/128 | Yes | Yes | Yes |
| IPv6 VPN | `ipv6/mpls-vpn` | 2/128 | Yes | Yes | Yes |
| IPv4 FlowSpec | `ipv4/flow` | 1/133 | Yes | Yes | Yes |
| IPv6 FlowSpec | `ipv6/flow` | 2/133 | Yes | Yes | Yes |
| IPv4 FlowSpec VPN | `ipv4/flow-vpn` | 1/134 | Yes | Yes | Yes |
| IPv6 FlowSpec VPN | `ipv6/flow-vpn` | 2/134 | Yes | Yes | Yes |
| IPv4 MPLS Label | `ipv4/mpls-label` | 1/4 | Yes | Yes | Yes |
| IPv6 MPLS Label | `ipv6/mpls-label` | 2/4 | Yes | Yes | Yes |
| L2VPN EVPN | `l2vpn/evpn` | 25/70 | Yes | Yes | Yes |
| L2VPN VPLS | `l2vpn/vpls` | 25/65 | Yes | Yes | Yes |
| BGP-LS | `bgp-ls/bgp-ls` | 16388/71 | No | Yes | No |
| BGP-LS VPN | `bgp-ls/bgp-ls-vpn` | 16388/72 | No | Yes | No |
| IPv4 MVPN | `ipv4/mvpn` | 1/5 | Yes | Yes | Partial |
| IPv6 MVPN | `ipv6/mvpn` | 2/5 | Yes | Yes | Partial |
| IPv4 RTC | `ipv4/rtc` | 1/132 | No | Yes | No |
| IPv4 MUP | `ipv4/mup` | 1/85 | Yes | Yes | Yes |
| IPv6 MUP | `ipv6/mup` | 2/85 | Yes | Yes | Yes |
| IPv4 SR-Policy | `ipv4/sr-policy` | 1/73 | Yes | Yes | Yes |
| IPv6 SR-Policy | `ipv6/sr-policy` | 2/73 | Yes | Yes | Yes |

<!-- source: internal/component/bgp/plugins/nlri/evpn/register.go -- EVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/srpolicy/register.go -- SR-Policy family registration -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/register.go -- FlowSpec family registration -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/register.go -- VPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/mup/register.go -- MUP family registration -->
<!-- source: internal/component/bgp/plugins/nlri/ls/register.go -- BGP-LS family registration -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/register.go -- MPLS label family registration -->
<!-- source: internal/component/bgp/plugins/nlri/vpls/register.go -- VPLS family registration -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/register.go -- MVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/rtc/register.go -- RTC family registration -->

### Capabilities

| Capability | Code | RFC | Description |
|------------|------|-----|-------------|
| Multiprotocol Extensions | 1 | RFC 4760 | Multi-protocol BGP (AFI/SAFI negotiation) |
| 4-byte ASN | 65 | RFC 6793 | 32-bit AS numbers |
| Route Refresh | 2 | RFC 2918 | Request full route re-advertisement |
| Enhanced Route Refresh | 70 | RFC 7313 | Bounded clear and re-send |
| ADD-PATH | 69 | RFC 7911 | Multiple paths per prefix |
| Extended Message | 6 | RFC 8654 | 65535-byte messages |
| Extended Next Hop | 5 | RFC 8950 | IPv6 next-hop for IPv4 NLRI |
| Graceful Restart | 64 | RFC 4724 | Session preservation across restarts (Restarting Speaker: R-bit via zefs marker on `ze signal restart`) |
| Long-Lived GR | 71 | RFC 9494 | Extended stale route retention with LLGR_STALE community and depreference |
| BGP Role | 9 | RFC 9234 | Peer relationship role |
| Hostname | 73 | RFC 8516 | FQDN capability |
| Software Version | 75 | draft | Software version advertisement |
| Link-Local Next Hop | 77 | RFC 2545 + draft | IPv6 link-local as next-hop |
| PATHS-LIMIT | 76 | draft-abraitis-idr-addpath-paths-limit | Per-family path count limit for ADD-PATH |

<!-- source: internal/core/bgp/capability/capability.go -- capability code constants -->
<!-- source: internal/core/bgp/capability/encoding.go -- EncodingCaps fields ASN4, ExtendedMessage, AddPathMode, ExtendedNextHop, PathsLimitSend, PathsLimitRecv -->
<!-- source: internal/core/bgp/capability/session.go -- SessionCaps fields RouteRefresh, EnhancedRouteRefresh, GracefulRestart -->
<!-- source: internal/component/bgp/plugins/role/register.go -- BGP Role capability plugin -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- Hostname capability plugin -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- Software Version capability plugin -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- Link-Local NH capability plugin -->

### AS number notation

An AS number is one 4-octet number on the wire (RFC 6793). RFC 5396 Section 2
names three ways to write that number in text. Ze READS all three wherever an
AS number is configured or typed at a command.

That includes a route distinguisher in every form Ze parses one, and the
administrator of an extended community such as `target:1.10:5`. It also
includes the `as-path` of `show bgp encode route`, the MVPN `source-as`, the
`show bgp rib path` filter, the `AS<n>` peer selector, and `rpki validate`. The
`ze-analyse` binary reads them on `--peer-asn` and `--local-as`.

One reader answers each of those forms. `selector.ParseASNSelector` is the
`AS<n>` selector, for every peer command and for the policy filter.
`attribute.ParseExtCommunityAdmin` is the extended-community administrator, for
all four of its parsers. A private copy of either is what `./le repository
check` now reports.

The `L` suffix of an extended community forces the four-octet encoding, and it
is orthogonal to the spelling: `65000L`, `0.100L` and `1.10L` each say it. For
a value of more than 65535 the value forces that encoding by itself, so the
suffix is redundant there and is accepted.

AS 65546 is written `65546` in asplain, and `1.10` in both asdot and asdot+.
Each field of the dotted form is a 16-bit value, so `0.65546` is not a spelling
of it. The tree stores the decimal form, so the notation an operator types
changes nothing downstream.

These configured fields hold an AS number and still read decimal only:

- the AS half of a standard community. RFC 1997 page 3 encodes it in the first
  two octets. Two octets hold no four-byte AS number. The only dotted spelling
  that fits is `0.Y`, and that names the AS number `Y` already names.
- the Global Administrator of a large community. RFC 8092 Section 5 pins the
  canonical representation to three decimal integers.
- the 2-octet administrator of a route origin (`origin:ASN:IP`) and of a
  filter's extended-community match. Two octets again, as above.

`bgp { as-notation }` selects the notation Ze WRITES. `asplain` is the default,
and RFC 5396 Section 3 recommends it. `asdot` writes an AS number of 65536 or
more as `X.Y`, and leaves a lower one as a decimal integer. `asdot+` writes
every AS number as `X.Y`.

The leaf reaches every surface an operator reads an AS number on:

- the route rows of `show bgp rib`
- the peer rows of `show bgp` and `show bgp peer detail`
- the ROA and ASPA rows of `show bgp rpki`
- the `show bgp irr` rows
- the CLI dashboard
- the looking glass tables and topology graph
- the BGP pages of the web interface

Under a dotted notation those AS numbers are JSON STRINGS rather than JSON
numbers, because `1.10` is not a JSON number. Every Ze reader of such a payload
accepts both forms (`asn.Number`, `asn.FromJSON`), so a client renders what the
producer rendered.

Five surfaces stay asplain whatever the leaf says, because their text is a
contract rather than a display:

- the AS path of the plugin event stream
- the `update text` command Ze replays to itself
- the filter text a filter plugin matches
- the text form of the plugin process protocol
- the `as_path` of the birdwatcher-compatible looking glass API

A dotted AS number there would stop a configured filter matching, or fail a
client that parses an integer. The ExaBGP bridge writes ExaBGP's own text
format, and that program fixes the spelling.

<!-- source: internal/core/bgp/asn/asn.go -- Parse, Append, Number, Configured -->
<!-- source: internal/core/bgp/attribute/text.go -- ParseASPathText, ParseCommunity, ParseLargeCommunity -->
<!-- source: internal/component/bgp/config/routeattr.go -- ParseRouteDistinguisher, ParseASPath, ParseAggregator -->
<!-- source: internal/core/bgp/nlri/rd.go -- ParseRDString, the RD reader every command word reaches -->
<!-- source: internal/core/bgp/attribute/text.go -- ParseExtCommunityAdmin, the one extended-community administrator reader -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- as-notation leaf -->
<!-- source: internal/component/bgp/plugins/rib/rib_attr_format.go -- asPathList -->
<!-- source: internal/core/bgp/attribute/text_append.go -- AppendText, which stays asplain -->

### Multipath installation

When several BGP candidates tie under multipath selection, Ze carries the
winner and its equal-cost sibling next hops into the shared Loc-RIB. The system
RIB then emits one ECMP group to the active FIB backend. Membership-only
changes, such as one equal-cost peer disappearing, update the installed group
without requiring the winning peer to change.

`show rib` reports the primary `next-hop` and the additional `ecmp-paths`.
Together they are the complete installed next-hop set.

<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- SelectMultipath, mirrorToLocRIB -->
<!-- source: internal/core/rib/locrib/candidate.go -- Path.ECMP -->
<!-- source: internal/core/rib/locrib/manager.go -- siblingNextHops -->

### Startup convergence hold (`update-delay`)

`bgp update-delay max-delay <seconds>` holds the first advertisement of a
speaker that has just started, so a neighbor receives one settled route set
instead of an initial set followed by corrections. The optional
`establish-wait <seconds>` ends the hold early, on the peers that have reached
Established by then. It must not be more than `max-delay`, and Ze refuses a
configuration where it is, at `ze config validate` as well as at startup.

While Ze holds, sessions negotiate, reach Established and receive UPDATE
messages as usual. Ze sends no UPDATE and no End-of-RIB marker, because the
initial routing update has not completed and RFC 4724 Section 4.1 makes the
marker a claim that it has. The hold ends on the first of three conditions:
every configured peer sends its own End-of-RIB marker, `establish-wait` expires
with one or more peers established, or `max-delay` expires. A peer that never
comes up cannot extend the wait past `max-delay`.

Ze waits for the marker rather than for Established, because Established says
only that the neighbor answered. RFC 4724 Section 4.1 excludes two kinds of peer
from that wait by name, and Ze excludes the same two: a peer that advertised no
graceful-restart capability, which has promised no marker, and a peer whose
capability carries the Restart State bit, which is deferring its own initial
update. A dynamic-group member is held like every other peer and is never counted
toward the release.

`show bgp update-delay` reports the hold: whether it is running, which condition
ended it, and how many expected peers have converged.

The hold applies at startup alone. A reload validates a changed `update-delay`
and Ze keeps the hold it already armed, or none: re-arming would withhold routes
from a neighbor that already has them.

An absent `max-delay`, or `max-delay 0`, is the default and holds nothing.

<!-- source: internal/component/bgp/reactor/update_delay.go -- updateDelayHold, Peer.startInitialRoutes -->
<!-- source: internal/component/bgp/config/update_delay.go -- ParseUpdateDelay -->

### Long-Lived Graceful Restart readvertisement

RFC 9494 stale-route handling is applied per destination on both normal
forwarding and RIB readvertisement. An LLGR-capable peer receives the stale
route unchanged. A non-LLGR eBGP peer receives a withdrawal. A non-LLGR iBGP
peer receives the route with `NO_EXPORT` and `LOCAL_PREF=0`, allowing partial
deployments to retain reachability without preferring stale information.

Only the LLGR filter runs during stale readvertisement. Other export policy has
already run on the original announcement and is not applied twice.

<!-- source: internal/component/bgp/plugins/gr/gr_egress.go -- LLGREgressFilter -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- sendStaleReadvertise -->

### Egress attribute rules

Several rails write an UPDATE: the announce rails that encode a route Ze
produced, the forward rail that relays another speaker's UPDATE, and the
route-server rail. Each rule below is answered at one site that every rail asks,
rather than re-derived per rail. The rails that re-derived a rule disagreed about
it: the announce rail stripped LOCAL_PREF toward an external peer and the forward
rail did not.

| Rule | RFC | Behavior |
|------|-----|----------|
| LOCAL_PREF off an external session | 4271 Section 5.1.5 | Removed from every UPDATE toward an external peer, relayed UPDATEs included. Recorded after the export filter pass, so a filter that sets LOCAL_PREF does not override the prohibition. A session is internal when `LocalAS == PeerAS`. Ze names no confederation member-AS, so the RFC 3065 exception is never active. |
| A received MULTI_EXIT_DISC is not relayed | 4271 Section 5.1.4 | Removed when the value about to be written is the value that arrived. A metric an announce rail or an export filter sets is kept, because that is Ze originating one. A route-server client keeps the received metric (RFC 7947 Section 2.2.3). |
| Configured MULTI_EXIT_DISC removal | 4271 Section 5.1.4 | `bgp/policy/modify/<name>/del/med`, on an import chain only. An export chain refuses the directive and logs why (Section 9.1.2.2). |
| No peer receives its own address as next hop | 4271 Section 5.1.3 | Asked of the built body, after the export chain, for a relayed route and for an originated one. The route is withheld from that peer and logged; it is never rewritten, which keeps RFC 7947 Section 2.2.2 transparency. Withdrawals in the same UPDATE still reach the peer. |
| A route naming this speaker is not installed | 4271 Section 5.1.3 | Excluded from best-path candidacy rather than refused at install, so a sound alternative wins. "Itself" is the set of session-local addresses. |
| Partial bit on an unrecognized transitive optional attribute | 4271 Sections 5 and 9 | Stamped at ingest. Recognition comes from Ze's own attribute registry, so removing a plugin makes that plugin's attribute unrecognized again. Session-reset and treat-as-withdraw skip the stamp, since they propagate nothing. |
| Withdrawals before announces | 4271 Section 4.3 | All withdrawals run before all announces, in the legacy sections and the multiprotocol ones, so one UPDATE naming a prefix in both leaves it reachable. |
| A relayed withdrawal carries no path attributes | 4271 Sections 4.3 and 6.3 | An UPDATE advertising no reachable NLRI gets no attribute created on it: no next-hop rewrite, no RFC 4456 reflection stamp, no community tag, no policy delta, no AS_PATH prepend. Rewriting an attribute the source already carries stays allowed, which keeps the RFC 6793 Section 4.2.2 width transcode. |
| One attribute order for every builder | 4271 Section 5 | Path attributes are inserted by type code in ascending order on every rail. MP_UNREACH_NLRI stays first, out of type-code order, so a withdrawal precedes an announcement in one message. |
| The next-hop wire form matches its length octet | 4760, 2545 Section 3 | The MP_REACH Next Hop length and the bytes written derive from one value. The IPv6 link-local address is appended after the global one only when this speaker shares a locally connected subnet with the peer and with the entity the global next hop names. The `link-local` leaf supplies the address; it does not decide that the address is sent. |
| A modification that cannot be applied suppresses the route | none | The route is withheld from that destination rather than forwarded unmodified. Counted on `ze_bgp_update_modify_failed_total{reason}`. |

<!-- source: internal/component/bgp/reactor/forward_local_pref.go -- localPrefAllowedTo, applyFactsLocalPref -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- medPropagationAllowedTo, applyFactsMED -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- egressNextHopIsPeerOwn, originatedNextHopIsPeerOwn -->
<!-- source: internal/component/bgp/plugins/rib/rib_self_nexthop.go -- refreshSelfNextHopsLocked, isSelfNextHop -->
<!-- source: internal/component/bgp/reactor/forward_build.go -- planAttr, buildModifiedPayload -->
<!-- source: internal/component/bgp/reactor/forward_modify_failure.go -- modifyFailure -->
<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- appendMEDRemove -->

### Unrecognized NLRI types in a typed family

RFC 7606 Section 5.4 requires a speaker advertising a typed address family to
discard routes carrying an NLRI type it does not implement, "unless the relevant
specification for that address family specifies otherwise". The ruling is
registered per family by the plugin that owns the family, so compiling that
plugin out removes both the advertisement and the obligation.

| Family | Ruling |
|--------|--------|
| `l2vpn/evpn` | Discard an unrecognized route type |
| `ipv4/mvpn`, `ipv6/mvpn` | Discard an unrecognized route type (RFC 6514 Section 4 framing) |
| `ipv4/mup`, `ipv6/mup` | Discard an unrecognized route type (draft-ietf-bess-mup-safi Section 3.1 framing) |
| `bgp-ls/bgp-ls`, `bgp-ls/bgp-ls-vpn` | No discard. RFC 9552 Section 5.2 requires an unknown Link-State NLRI type to be preserved and propagated |
| Every other family | No discard. A family nobody has ruled on discards nothing |

The recognizer answers about the route TYPE only. A well-typed but malformed
NLRI is a Section 5.3 concern, handled before this. When the length fields do
not agree with the section the NLRI boundaries are unknowable, so no discard
decision is made rather than one being guessed.

<!-- source: internal/core/bgp/nlri/nlritype/nlritype.go -- Register, Retain -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/register.go -- EVPN recognizer -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/register.go -- MCAST-VPN recognizer -->
<!-- source: internal/component/bgp/plugins/nlri/mup/register.go -- MUP recognizer -->

### RFC 7911 Path Identifier regeneration

RFC 7911 Section 2 requires a speaker that re-advertises a route to generate its
own Path Identifier, assigned so that (prefix, identifier) uniquely names a path
advertised to a neighbor. Both forward rails do that instead of relaying the
identifier the source chose.

The key is the path at ingress, and how much of the path it holds follows what
the source framed. A source that negotiated no ADD-PATH names a path by its
prefix alone and sends every one of them under identifier 0, so ze holds one
identifier for that source's whole session. A source that negotiated ADD-PATH
names a path by (prefix, identifier), so ze holds one entry per family, received
identifier and prefix. A withdrawn route carries no path attributes, so an
attribute-derived identifier could not be recomputed when the path leaves; the
ingress key can. The same key makes a re-announcement with changed attributes
replace rather than duplicate.

An identifier of the first kind is released only when the peer is removed. One of
the second kind is released when ze has relayed that pair's withdraw, at the
point the recent-update cache evicts the UPDATE that carried it. Neither is
released at session down, so a reconnecting peer re-announces under the
identifiers its destinations already hold. Zero is minted and accepted like any
other value (RFC 7911 Section 3).

Regeneration runs whenever either side of the forward frames identifiers. A
session where neither side negotiated ADD-PATH keeps its zero-copy forward.

<!-- source: internal/component/bgp/reactor/forward_path_id.go -- fwdPathIDs, fwdPathIDTable.generatePath, fwdReleaseWithdrawnPathIDs -->

### Protocol event capture and replay

A peer writes every message it receives to a bounded JSONL file, together with
the config operations applied while the capture runs. `ze-test replay <file>`
feeds the file back through the same read path with an injected clock.

The tee sits on the complete wire message in both read paths, before message
processing. RFC 7606 enforcement rewrites attributes and synthesizes
withdrawals, and the family and prefix-limit checks can return before anything
is dispatched, so the plugin message-observer hook cannot see what a bug capture
needs.

Off by default. The tee costs one nil comparison and no allocation per received
message. Config payloads pass a redactor, and TCP-MD5 keys never appear on the
wire, so the file holds routing data and no local secret. `ze doctor` reports
whether an enabled peer's capture directory is usable
(`doctor-bgp-capture-directory`).

```bash
ze-test replay [--json] [--local-as N] [--peer-as N] [--router-id N] <capture-file|->
```

<!-- source: internal/core/capture/capture.go -- the bounded JSONL writer -->
<!-- source: internal/component/bgp/reactor/capture_replay.go -- the session tee and the replay driver -->
<!-- source: internal/test/cli/cmd_replay.go -- cmdReplay -->
<!-- source: internal/component/doctor/checks_bgp_capture.go -- capture directory readiness -->

### Path Attributes

| Attribute | Code | JSON Key | Description |
|-----------|------|----------|-------------|
| ORIGIN | 1 | `origin` | igp / egp / incomplete |
| AS_PATH | 2 | `as-path` | AS path segments |
| NEXT_HOP | 3 | `next-hop` | Next hop IP address |
| MED | 4 | `med` | Multi-Exit Discriminator |
| LOCAL_PREF | 5 | `local-preference` | Local preference |
| ATOMIC_AGGREGATE | 6 | `atomic-aggregate` | Atomic aggregate flag |
| AGGREGATOR | 7 | `aggregator` | Aggregator ASN:IP |
| COMMUNITY | 8 | `community` | Standard communities |
| ORIGINATOR_ID | 9 | `originator-id` | Route reflector originator |
| CLUSTER_LIST | 10 | `cluster-list` | Route reflector cluster list |
| MP_REACH_NLRI | 14 | none | Multiprotocol reachable NLRI |
| MP_UNREACH_NLRI | 15 | none | Multiprotocol unreachable NLRI |
| EXTENDED_COMMUNITY | 16 | `extended-community` | Extended communities |
| LARGE_COMMUNITY | 32 | `large-community` | Large communities (RFC 8092) |
| PREFIX_SID | 40 | `prefix-sid` | Segment Routing prefix SID |

<!-- source: internal/core/bgp/attribute/attribute.go -- attribute code constants -->
<!-- source: internal/core/bgp/attribute/origin.go -- ORIGIN -->
<!-- source: internal/core/bgp/attribute/aspath.go -- AS_PATH -->
<!-- source: internal/core/bgp/attribute/community.go -- Communities, ExtendedCommunities, LargeCommunities -->
