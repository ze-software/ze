# Configuration Reference

The complete Ze configuration as one tree: 36 top-level sections (27 provided by plugins, the rest core), generated live from the YANG schema with `ze yang tree`. This is about the structure of the configuration -- every section, searchable and inspectable. See [the Configuration guide](https://ze-software.net/features/bgp-configuration/) for a narrative walkthrough of BGP peer config specifically.

## anomaly

Behavioral anomaly detection and response subsystem.

- **detect** `container`
  *Provided by `anomaly-detect` ([ze-anomaly-detect-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang))*
  Report-only behavioral anomaly detector (emits incidents, takes no action).
  - **baseline-window** `uint32`
    Per-entity baseline horizon in ticks (the EWMA smoothing factor is derived from it).
  - **clear-consecutive** `uint8`
    Consecutive below-threshold ticks before an active incident clears.
  - **cohort-prefix-len-v4** `uint8`
    Source-prefix length that buckets IPv4 entities into peer-group cohorts.
  - **cohort-prefix-len-v6** `uint8`
    Source-prefix length that buckets IPv6 entities into peer-group cohorts.
  - **confirm-duration** `uint16`
    Consecutive above-threshold ticks before an incident is confirmed and emitted.
  - **corroboration-weight** `decimal-2`
    Discount applied to a corroborating feature when Ze combines scores.
  - **deviation-threshold** `decimal-2`
    Sigma at or above which a per-entity feature deviation fires.
  - **enabled** `boolean`
    Enable the behavioral anomaly detector (report-only; emits incidents, takes no action).
  - **min-cohort-size** `uint16`
    Minimum members in a cohort before Ze scores peer-group rarity.
  - **min-features-to-correlate** `uint8`
    Minimum distinct features that must fire before Ze scores an incident.
- **observe** `container`
  *Provided by `anomaly-observe` ([ze-anomaly-observe-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/anomaly/observe/yang/ze-anomaly-observe-conf.yang))*
  Incident lifecycle store for confirmed anomaly incidents.
  - **incident-ring-size** `uint32`
    Maximum number of incidents the ring keeps in memory.
  - **stale-incident-timeout** `uint32`
    Seconds before an open incident with no clear event is finalized.
- **shape** `container`
  *Provided by `anomaly-shape` ([ze-anomaly-shape-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/anomaly/shape/yang/ze-anomaly-shape-conf.yang))*
  Autonomous responder: shadow (log-only) or armed (live per-source firewall actions).
  - **action** `enumeration`
    Armed action: rate-limit the source (surgical) or drop it (fallback).
  - **allowlist** `ip-prefix[]`
    Protected source prefixes that ze never arms an action against.
  - **auto-revert-ttl** `uint16`
    Seconds after the last signal before an armed action reverts by itself.
  - **blast-radius-cap** `uint16`
    Maximum concurrently-armed live actions; further arm attempts are refused.
  - **kill-switch** `boolean`
    When true, revert every armed action and force the responder to shadow.
  - **limit-burst** `uint32`
    Burst allowance for the limit action.
  - **limit-rate** `uint64`
    Rate for the limit action, in packets per limit-unit.
  - **limit-unit** `enumeration`
    Time unit for limit-rate.
  - **mode** `enumeration`
    Responder mode: shadow or armed.

## bfd

*Provided by `bfd` ([ze-bfd-api.yang](https://github.com/ze-software/ze/blob/main/internal/component/bfd/yang/ze-bfd-api.yang), [ze-bfd-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/bfd/yang/ze-bfd-cmd.yang), [ze-bfd-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/bfd/yang/ze-bfd-conf.yang))*

BFD control configuration for Ze.

- **bind-v6** `boolean`
  Bind an IPv6 socket beside the IPv4 socket on every loop.
- **enabled** `boolean`
  Master switch for the BFD plugin.
- **multi-hop-session <peer local vrf>** `list`
  Multi-hop BFD session (RFC 5883). Port 4784, no GTSM.
  - **local** `string`
    Local source address. Required for multi-hop.
  - **min-ttl** `uint8`
    Minimum acceptable TTL on receive. Weak replacement for GTSM on multi-hop paths.
  - **peer** `string`
    Address of the remote system this session runs to.
  - **profile** `string`
    Name of the timer profile this session takes its intervals from.
  - **shutdown** `boolean`
    Hold the session administratively down and keep its configuration.
  - **vrf** `string`
    Routing instance the session runs in. The default instance is named default.
- **persist-dir** `string`
  DEPRECATED. A non-empty value opts a session into sequence persistence.
- **profile <name>** `list`
  Reusable timer and feature profile.
  - **auth** `container`
    Authentication parameters (RFC 5880 Section 6.7).
    - **key-id** `uint8`
      Auth Key ID (RFC 5880 Section 6.7.1).
    - **secret** `string`
      Shared secret, redacted from `ze config show` output.
    - **type** `enumeration`
      Authentication type, which the operator must name.
  - **desired-min-tx-us** `uint32`
    Local target transmit rate in microseconds.
  - **detect-multiplier** `uint8`
    Consecutive missed Control packets that trigger a Down transition.
  - **echo** `container`
    BFD Echo mode (RFC 5880 Section 6.4, RFC 5881 Section 5).
    - **desired-min-echo-tx-us** `uint32`
      Local target echo transmit rate in microseconds.
  - **passive** `boolean`
    Wait for a peer Control packet before this session transmits.
  - **required-min-rx-us** `uint32`
    Minimum inter-packet gap the local end can handle, in microseconds.
- **single-hop-session <peer vrf interface>** `list`
  Per-link BFD session on port 3784 with TTL 255 (RFC 5881).
  - **interface** `string`
    Egress interface name.
  - **local** `string`
    Local source address (optional).
  - **peer** `string`
    Peer IPv4 or IPv6 address.
  - **profile** `string`
    Named profile to inherit timer parameters from.
  - **shutdown** `boolean`
    When true, the session stays in AdminDown state. RFC 5880 Section 6.8.16.
  - **vrf** `string`
    Routing instance the session runs in. The default instance is named default.

## bgp

*Provided by `bgp` ([ze-bgp-api.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/yang/ze-bgp-api.yang), [ze-bgp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/yang/ze-bgp-conf.yang)); `bgp-bmp` ([ze-bmp-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-cmd.yang), [ze-bmp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang)); `bgp-filter-aspath` ([ze-filter-aspath.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_aspath/yang/ze-filter-aspath.yang)); `bgp-filter-aspath-length` ([ze-filter-aspath-length.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_aspath_length/yang/ze-filter-aspath-length.yang)); `bgp-filter-community` ([ze-filter-community.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang)); `bgp-filter-community-match` ([ze-filter-community-match.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/yang/ze-filter-community-match.yang)); `bgp-filter-family` ([ze-filter-family.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_family/yang/ze-filter-family.yang)); `bgp-filter-irr` ([ze-filter-irr-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang), [ze-filter-irr.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang)); `bgp-filter-modify` ([ze-filter-modify.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang)); `bgp-filter-path-asn` ([ze-filter-path-asn-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn-cmd.yang), [ze-filter-path-asn.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_path_asn/yang/ze-filter-path-asn.yang)); `bgp-filter-prefix` ([ze-filter-prefix.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/yang/ze-filter-prefix.yang)); `bgp-filter-remove-private-as` ([ze-filter-remove-private-as.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/yang/ze-filter-remove-private-as.yang)); `bgp-gr` ([ze-graceful-restart.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang)); `bgp-healthcheck` ([ze-healthcheck-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/healthcheck/yang/ze-healthcheck-conf.yang)); `bgp-hostname` ([ze-hostname.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/hostname/yang/ze-hostname.yang)); `bgp-llnh` ([ze-link-local-nexthop.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang)); `bgp-rib` ([ze-rib-api.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/yang/ze-rib-api.yang), [ze-rib.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/yang/ze-rib.yang)); `bgp-role` ([ze-role.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/yang/ze-role.yang)); `bgp-route-refresh` ([ze-refresh-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/yang/ze-refresh-cmd.yang), [ze-route-refresh-api.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/yang/ze-route-refresh-api.yang), [ze-route-refresh.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/route_refresh/yang/ze-route-refresh.yang)); `bgp-rpki` ([ze-rpki.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/yang/ze-rpki.yang)); `bgp-rs` ([ze-rs-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang)); `bgp-softver` ([ze-softver.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/yang/ze-softver.yang)); `bgp-watchdog`*

Border Gateway Protocol routing configuration.

- **as-notation** `enumeration`
  How Ze writes an AS number in the output an operator reads.
- **blackhole** `container`
  RFC 7999 blackhole agreement on this BGP session, in both directions.
  - **communities** `string[]`
    Communities that make Ze discard traffic toward a prefix this peer announces.
  - **prefixes** `string[]`
    The prefixes this peer is authorized to advertise.
- **bmp** `container`
  BMP sender and protocol options
  - **sender** `container`
    Stream BMP data to external collectors
    - **collector <name>** `list`
      BMP collector endpoints to connect to
      - **address** `ip-address`
        Collector IP address
      - **port** `port`
        Collector TCP port
      - **source-address** `ip-address`
        Source IP address for outbound BMP connections
    - **loc-rib** `boolean`
      Enable Loc-RIB monitoring (RFC 9069).
    - **route-mirroring** `boolean`
      Enable Route Mirroring (RFC 7854 S4.7): stream verbatim copies of all BGP messages to collectors
    - **route-monitoring-policy** `enumeration`
      Which routes to stream: pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out, RFC 8671), or all
    - **statistics-timeout** `uint16`
      Interval for periodic statistics reports (0 = disabled)
- **community** `container`
  Named community definitions referenced by filter rules.
  - **extended <name>** `list`
    Named sets of extended communities a tag or a strip list names.
    - **value** `string[]`
      Extended community values.
  - **large <name>** `list`
    Named sets of large communities a tag or a strip list names.
    - **value** `string[]`
      Large community values, in GA:LD1:LD2 format.
  - **standard <name>** `list`
    Named sets of standard communities a tag or a strip list names.
    - **value** `string[]`
      Standard community values, in ASN:value format.
- **defaults** `container`
  Values ze supplies when a route carries no such attribute.
  - **attribute** `container`
    The value policy arithmetic starts from when the route carries no such attribute.
    - **local-preference** `uint32`
      The LOCAL_PREF value policy arithmetic starts from when the route carries none.
    - **med** `uint32`
      The MULTI_EXIT_DISC value policy arithmetic starts from when the route carries none.
- **filter** `container`
  Global route filter chains for import and export.
  - **egress** `container`
    Egress direction filters.
    - **community** `container`
      Community filter for egress.
      - **strip** `string[]`
        Named communities to remove on egress.
      - **tag** `string[]`
        Named communities to add on egress.
  - **export** `string[]`
    Global export filter chain
  - **import** `string[]`
    Global import filter chain
  - **ingress** `container`
    Ingress direction filters.
    - **community** `container`
      Community filter for ingress.
      - **blackhole-propagation** `enumeration`
        Community to add to a route that carries BLACKHOLE (65535:666).
      - **relation-function** `uint32`
        Function number of the relation tag.
      - **relation-tag** `boolean`
        Write the relation-to-origin large community of RFC 8195 on each route from this peer.
      - **scrub-keep-function** `uint32[]`
        Function numbers a peer can send with the local AS as the Global Administrator.
      - **scrub-own-ga** `boolean`
        Remove inbound communities whose Global Administrator is the local AS.
      - **strip** `string[]`
        Named communities to remove on ingress.
      - **tag** `string[]`
        Named communities to add on ingress.
- **group <name>** `list`
  Peer group - defines shared defaults for member peers
  - **attach** `container`
    Programs this peer attaches.
    - **process <name>** `list`
      External process that receives BGP events and can inject a message.
      - **content** `container`
        Controls the encoding and filtering of events sent to this process.
        - **attribute** `string`
          Filter expression to select which BGP attributes are included in events.
        - **encoding** `string`
          Wire encoding for events sent to the process (e.g., json, text).
        - **format** `string`
          Output format template for event rendering.
      - **neighbor-changes** `container`
        Send session state change events to this process.
      - **processes** `string[]`
        Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.
      - **processes-match** `string[]`
        Legacy ExaBGP process match patterns for event filtering.
      - **receive** `string[]`
        Event types this peer feeds the program.
      - **run** `string`
        Shell command that starts the external process.
      - **send** `string[]`
        Message types this program can send toward the peer.
  - **behavior** `container`
    Controls how the reactor processes and forwards an UPDATE for this peer.
    - **auto-flush** `boolean`
      Withdraw every route advertised to this peer when the session goes down.
    - **group-updates** `boolean`
      Pack several NLRI into one UPDATE message when they share the same path attributes.
    - **manual-eor** `boolean`
      Do not send End-of-RIB automatically after the initial route advertisement.
    - **rs-fast-path** `boolean`
      Forward a received UPDATE inside the reactor for an RS-client peer.
  - **blackhole** `container`
    RFC 7999 blackhole agreement on this BGP session, in both directions.
    - **communities** `string[]`
      Communities that make Ze discard traffic toward a prefix this peer announces.
    - **prefixes** `string[]`
      The prefixes this peer is authorized to advertise.
  - **capture** `container`
    Protocol event capture for this peer.
    - **directory** `string`
      Directory that holds the capture files of this peer.
    - **enabled** `boolean`
      Capture this peer's inbound protocol events to a file. Off by default.
    - **maximum-size** `uint32`
      Hard cap on the size of one capture file.
    - **on-limit** `enumeration`
      What happens when a capture file reaches maximum-size.
  - **connection** `container`
    Transport-level connection settings
    - **bfd** `container`
      Bidirectional Forwarding Detection options for this peer (RFC 5880).
      - **enabled** `boolean`
        Master switch for the BFD session of this peer.
      - **hold-down** `uint32`
        BFD hold-down interval for strict mode (draft-ietf-idr-bgp-bfd-strict-mode Section 10).
      - **hold-time** `uint16`
        BfdHoldTime for strict mode (draft-ietf-idr-bgp-bfd-strict-mode Section 3).
      - **interface** `string`
        Single-hop egress interface for the BFD session.
      - **min-ttl** `uint8`
        Multi-hop minimum acceptable TTL (RFC 5883 Section 5).
      - **mode** `enumeration`
        BFD hop mode for this peer: single-hop or multi-hop.
      - **profile** `string`
        Name of a profile defined under the top-level bfd { profile ... } block.
      - **strict** `boolean`
        BFD strict mode (draft-ietf-idr-bgp-bfd-strict-mode).
    - **link-local** `boolean`
      Auto-discover IPv6 link-local address for TCP connection
    - **local** `container`
      Local endpoint for the TCP session.
      - **accept** `boolean`
        Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Local address for connection (use IP address or 'auto')
      - **port** `port`
        Local listen port
    - **md5** `container`
      TCP MD5 authentication (RFC 2385)
      - **ip** `ip-address`
        MD5 authentication IP
      - **password** `string`
        MD5 authentication password
    - **remote** `container`
      Remote endpoint for the TCP session.
      - **connect** `boolean`
        Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Peer IP address or 'dynamic' for dynamic peer groups
      - **max-peers** `uint32`
        Maximum number of dynamic peers for this group
      - **port** `port`
        Remote connection port
      - **range** `ip-prefix[]`
        IP prefix ranges for a dynamic peer group.
    - **ttl** `container`
      TTL settings for BGP sessions
      - **max** `uint8`
        TTL security / GTSM (RFC 5082)
      - **min** `uint8`
        Minimum incoming TTL
      - **set** `uint8`
        Outgoing TTL value
  - **description** `string`
    Free-text label for this peer.
  - **filter** `container`
    Route filter chains for import and export.
    - **egress** `container`
      Egress direction filters.
      - **community** `container`
        Community filter for egress.
        - **strip** `string[]`
          Named communities to remove on egress.
        - **tag** `string[]`
          Named communities to add on egress.
    - **export** `string[]`
      Export filter chain (filter instance names)
    - **import** `string[]`
      Import filter chain (filter instance names)
    - **ingress** `container`
      Ingress direction filters.
      - **community** `container`
        Community filter for ingress.
        - **blackhole-propagation** `enumeration`
          Community to add to a route that carries BLACKHOLE (65535:666).
        - **relation-function** `uint32`
          Function number of the relation tag.
        - **relation-tag** `boolean`
          Write the relation-to-origin large community of RFC 8195 on each route from this peer.
        - **scrub-keep-function** `uint32[]`
          Function numbers a peer can send with the local AS as the Global Administrator.
        - **scrub-own-ga** `boolean`
          Remove inbound communities whose Global Administrator is the local AS.
        - **strip** `string[]`
          Named communities to remove on ingress.
        - **tag** `string[]`
          Named communities to add on ingress.
  - **peer <name>** `list`
    BGP peer in this group
    - **attach** `container`
      Programs this peer attaches.
      - **process <name>** `list`
        External process that receives BGP events and can inject a message.
        - **content** `container`
          Controls the encoding and filtering of events sent to this process.
          - **attribute** `string`
            Filter expression to select which BGP attributes are included in events.
          - **encoding** `string`
            Wire encoding for events sent to the process (e.g., json, text).
          - **format** `string`
            Output format template for event rendering.
        - **neighbor-changes** `container`
          Send session state change events to this process.
        - **processes** `string[]`
          Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.
        - **processes-match** `string[]`
          Legacy ExaBGP process match patterns for event filtering.
        - **receive** `string[]`
          Event types this peer feeds the program.
        - **run** `string`
          Shell command that starts the external process.
        - **send** `string[]`
          Message types this program can send toward the peer.
    - **behavior** `container`
      Controls how the reactor processes and forwards an UPDATE for this peer.
      - **auto-flush** `boolean`
        Withdraw every route advertised to this peer when the session goes down.
      - **group-updates** `boolean`
        Pack several NLRI into one UPDATE message when they share the same path attributes.
      - **manual-eor** `boolean`
        Do not send End-of-RIB automatically after the initial route advertisement.
      - **rs-fast-path** `boolean`
        Forward a received UPDATE inside the reactor for an RS-client peer.
    - **blackhole** `container`
      RFC 7999 blackhole agreement on this BGP session, in both directions.
      - **communities** `string[]`
        Communities that make Ze discard traffic toward a prefix this peer announces.
      - **prefixes** `string[]`
        The prefixes this peer is authorized to advertise.
    - **capture** `container`
      Protocol event capture for this peer.
      - **directory** `string`
        Directory that holds the capture files of this peer.
      - **enabled** `boolean`
        Capture this peer's inbound protocol events to a file. Off by default.
      - **maximum-size** `uint32`
        Hard cap on the size of one capture file.
      - **on-limit** `enumeration`
        What happens when a capture file reaches maximum-size.
    - **connection** `container`
      Transport-level connection settings
      - **bfd** `container`
        Bidirectional Forwarding Detection options for this peer (RFC 5880).
        - **enabled** `boolean`
          Master switch for the BFD session of this peer.
        - **hold-down** `uint32`
          BFD hold-down interval for strict mode (draft-ietf-idr-bgp-bfd-strict-mode Section 10).
        - **hold-time** `uint16`
          BfdHoldTime for strict mode (draft-ietf-idr-bgp-bfd-strict-mode Section 3).
        - **interface** `string`
          Single-hop egress interface for the BFD session.
        - **min-ttl** `uint8`
          Multi-hop minimum acceptable TTL (RFC 5883 Section 5).
        - **mode** `enumeration`
          BFD hop mode for this peer: single-hop or multi-hop.
        - **profile** `string`
          Name of a profile defined under the top-level bfd { profile ... } block.
        - **strict** `boolean`
          BFD strict mode (draft-ietf-idr-bgp-bfd-strict-mode).
      - **link-local** `boolean`
        Auto-discover IPv6 link-local address for TCP connection
      - **local** `container`
        Local endpoint for the TCP session.
        - **accept** `boolean`
          Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)
        - **ip** `union`
          Local address for connection (use IP address or 'auto')
        - **port** `port`
          Local listen port
      - **md5** `container`
        TCP MD5 authentication (RFC 2385)
        - **ip** `ip-address`
          MD5 authentication IP
        - **password** `string`
          MD5 authentication password
      - **remote** `container`
        Remote endpoint for the TCP session.
        - **connect** `boolean`
          Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)
        - **ip** `union`
          Peer IP address or 'dynamic' for dynamic peer groups
        - **max-peers** `uint32`
          Maximum number of dynamic peers for this group
        - **port** `port`
          Remote connection port
        - **range** `ip-prefix[]`
          IP prefix ranges for a dynamic peer group.
      - **ttl** `container`
        TTL settings for BGP sessions
        - **max** `uint8`
          TTL security / GTSM (RFC 5082)
        - **min** `uint8`
          Minimum incoming TTL
        - **set** `uint8`
          Outgoing TTL value
    - **description** `string`
      Free-text label for this peer.
    - **filter** `container`
      Route filter chains for import and export.
      - **egress** `container`
        Egress direction filters.
        - **community** `container`
          Community filter for egress.
          - **strip** `string[]`
            Named communities to remove on egress.
          - **tag** `string[]`
            Named communities to add on egress.
      - **export** `string[]`
        Export filter chain (filter instance names)
      - **import** `string[]`
        Import filter chain (filter instance names)
      - **ingress** `container`
        Ingress direction filters.
        - **community** `container`
          Community filter for ingress.
          - **blackhole-propagation** `enumeration`
            Community to add to a route that carries BLACKHOLE (65535:666).
          - **relation-function** `uint32`
            Function number of the relation tag.
          - **relation-tag** `boolean`
            Write the relation-to-origin large community of RFC 8195 on each route from this peer.
          - **scrub-keep-function** `uint32[]`
            Function numbers a peer can send with the local AS as the Global Administrator.
          - **scrub-own-ga** `boolean`
            Remove inbound communities whose Global Administrator is the local AS.
          - **strip** `string[]`
            Named communities to remove on ingress.
          - **tag** `string[]`
            Named communities to add on ingress.
    - **rib** `container`
      Route Information Base settings for this peer.
      - **adj** `container`
        Adjacency RIB storage for this peer.
        - **in** `boolean`
          Store received routes in Adj-RIB-In, before policy.
        - **out** `boolean`
          Store advertised routes in Adj-RIB-Out, after policy.
      - **out** `container`
        Outbound RIB batching: how route changes are grouped and flushed to the peer.
        - **auto-commit-delay** `uint32`
          Milliseconds to wait before Ze flushes pending route changes to the peer.
        - **group-updates** `boolean`
          Pack routes with identical attributes into one UPDATE.
        - **max-batch-size** `int32`
          Maximum number of route changes in one flush cycle.
    - **role** `container`
      RFC 9234 BGP Role configuration
      - **export** `export-token[]`
        Controls which destination peer roles may receive routes from this peer
      - **import** `role-type`
        Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)
      - **strict** `boolean`
        Require peer to send Role capability
    - **rpki** `container`
      RPKI action overrides of this peer or group.
      - **action** `container`
        Origin-validation action overrides of this peer or group.
        - **invalid** `validation-action`
          Action Ze applies to a route in the Invalid validation state.
        - **not-found** `validation-action`
          Action Ze applies to a route in the NotFound validation state.
      - **aspa** `container`
        ASPA action overrides for this peer or group.
        - **action** `container`
          ASPA path-state action overrides of this peer or group.
          - **invalid** `validation-action`
            Action Ze applies to a route in the ASPA Invalid path state.
          - **unknown** `validation-action`
            Action Ze applies to a route in the ASPA Unknown path state.
      - **blackhole-exempt** `boolean`
        Keep a BLACKHOLE route when its only origin-validation fault is the prefix length.
    - **session** `container`
      BGP session parameters for this peer or group.
      - **accept-srv6-prefix-sid** `boolean`
        Accept the BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer.
      - **as-override** `boolean`
        Replace the ASN of the peer with the local ASN in the outbound AS_PATH.
      - **asn** `container`
        AS number configuration
        - **local** `asn`
          Local AS (overrides global)
        - **local-options** `enumeration[]`
          Modifiers for the local-as behavior of this peer.
        - **migration** `asn`
          Second local AS this iBGP session accepts and sends during an AS migration.
        - **remote** `asn`
          Peer Autonomous System Number
      - **capability** `container`
        BGP capability negotiation
        - **add-path** `container`
          ADD-PATH capability (RFC 7911), with an optional PATHS-LIMIT.
          - **direction** `enumeration`
            Default ADD-PATH direction for all negotiated families.
          - **family <name>** `list`
            Per-family ADD-PATH overrides with optional path count limit.
            - **direction** `enumeration`
              ADD-PATH direction override for this family.
            - **limit** `uint16`
              Maximum paths per prefix to receive (PATHS-LIMIT capability).
            - **mode** `enumeration`
              ADD-PATH negotiation mode for this family.
          - **limit** `uint16`
            Default maximum paths per prefix (PATHS-LIMIT).
        - **asn4** `boolean`
          Advertise the 4-byte ASN capability (RFC 6793).
        - **extended-message** `container`
          Extended Message capability (RFC 8654).
        - **graceful-restart** `container`
          Graceful Restart capability configuration.
          - **long-lived-stale-time** `uint32`
            Long-Lived Stale Time for the peer, in seconds (RFC 9494 Section 3).
          - **restart-time** `uint16`
            Restart Time in seconds (RFC 4724 Section 3). Maximum value is 4095 (12-bit field).
        - **hostname** `container`
          FQDN capability configuration.
          - **domain** `string`
            Domain name (max 255 bytes).
          - **host** `string`
            System hostname (max 255 bytes).
        - **link-local-nexthop** `container`
          Link-local next-hop capability.
        - **nexthop <family>** `list`
          Extended Next Hop capability (RFC 8950)
          - **mode** `enumeration`
            Extended next hop negotiation mode
          - **nhafi** `afi`
            Next-hop AFI (e.g. ipv6)
        - **route-refresh** `container`
          Route Refresh capability (RFC 2918).
        - **software-version** `container`
          Software Version capability (code 75).
          - **mode** `enumeration`
            Capability negotiation mode.
      - **cluster-id** `ipv4-address`
        Override the cluster ID for route reflection (RFC 4456 Section 7).
      - **community** `container`
        Community attribute control for this session
        - **send** `enumeration[]`
          Community types Ze includes in an outbound UPDATE.
      - **domain-name** `string`
        Legacy: Domain name for FQDN capability.
      - **family <name>** `list`
        Address families to negotiate
        - **default-originate** `boolean`
          Originate the default route to this peer for this address family.
        - **default-originate-filter** `string`
          Named filter that must accept before Ze originates the default route.
        - **mode** `enumeration`
          Address family negotiation mode
        - **prefix** `container`
          Prefix limit configuration for this address family
          - **count** `enumeration`
            Which prefixes the count compared against maximum holds.
          - **idle-timeout** `uint16`
            Seconds to wait before reconnect after this family stopped the session.
          - **maximum** `uint32`
            Hard maximum number of prefixes accepted
          - **reconnect** `enumeration`
            What the peer does after this family stopped the session.
          - **teardown** `boolean`
            Tear down session when prefix maximum exceeded (false = warn only)
          - **updated** `string`
            ISO date (YYYY-MM-DD) when prefix maximum was last updated from PeeringDB. Hidden leaf.
          - **warning** `uint32`
            Warning threshold. Defaults to 90% of maximum when not set.
      - **host-name** `string`
        Legacy: Host name for FQDN capability.
      - **irr** `container`
        Per-peer IRR filter settings.
        - **as-set** `string`
          Explicit AS-SET name for the IRR prefix lookup.
        - **enable** `enumeration`
          Enable or disable IRR filtering for this peer.
      - **link-local** `ipv6-address`
        IPv6 link-local address advertised after the global next hop (RFC 2545 Section 3).
      - **next-hop** `union`
        Next-hop rewriting policy for a forwarded UPDATE (RFC 4271 Section 5.1.3).
      - **propagate-srv6-prefix-sid** `boolean`
        Advertise the BGP Prefix-SID attribute (code 40) to this EBGP peer.
      - **route-reflector-client** `boolean`
        Mark this peer as a route reflector client (RFC 4456).
      - **router-id** `ipv4-address`
        Override the router ID for this peer.
      - **rs-client** `boolean`
        Mark this peer as an RS-client for transparent AS-path forwarding.
    - **timer** `container`
      BGP session timers: hold time, keepalive interval, and connect retry delay.
      - **connect-retry** `uint16`
        Connect retry interval in seconds. Ze reads it for a dynamic peer alone.
      - **keepalive** `uint16`
        Keepalive interval in seconds (RFC 4271 Section 10). 0 = auto: hold-time/3.
      - **receive-hold-time** `uint16`
        Receive hold time in seconds (RFC 4271: 0 or >= 3). Proposed in OPEN.
      - **send-hold-time** `uint16`
        Send hold time in seconds (RFC 9687). 0 = auto: max(480, 2x receive-hold-time).
    - **update** `list`
      Native Ze route announcements
      - **attribute** `container`
        Path attributes for routes in this block
        - **aggregator** `string`
          AGGREGATOR attribute (RFC 4271 Section 5.1.7). The format is AS:IP.
        - **as-path** `string[]`
          AS_PATH segments prepended to the route, as space-separated ASNs.
        - **atomic-aggregate** `boolean`
          ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6).
        - **attribute** `string[]`
          Raw BGP attributes in hex encoding, as flags:type:hex-value.
        - **bgp-prefix-sid** `container`
          Prefix-SID attribute (RFC 8669).
        - **bgp-prefix-sid-srv6** `container`
          SRv6 Prefix-SID attribute (RFC 9252).
        - **cluster-list** `ipv4-address[]`
          CLUSTER_LIST for route reflection (RFC 4456).
        - **community** `string[]`
          Standard BGP communities (RFC 1997).
        - **extended-community** `string[]`
          Extended communities (RFC 4360). The format is type:admin:value.
        - **label** `string`
          MPLS label value for a labeled unicast route (RFC 3107). The value is a number.
        - **labels** `string[]`
          MPLS label stack for a multi-label route (RFC 8277).
        - **large-community** `string[]`
          Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
        - **local-preference** `uint32`
          LOCAL_PREF value for iBGP best-path selection. The higher value wins.
        - **med** `uint32`
          Multi-Exit Discriminator (MED). The lower value is preferred.
        - **next-hop** `union`
          Next hop address or 'self'
        - **origin** `enumeration`
          ORIGIN attribute (RFC 4271 Section 4.3)
        - **originator-id** `ipv4-address`
          ORIGINATOR_ID for route reflection (RFC 4456).
        - **path-information** `string`
          ADD-PATH path identifier (RFC 7911).
        - **rd** `route-distinguisher`
          Route Distinguisher (RFC 4364). The format is ASN:nn or IP:nn.
        - **split** `string`
          Split one prefix into more-specific prefixes for the announcement.
      - **name** `string`
        Optional label for this update block (display only)
      - **nlri <name>** `list`
        Routes this update block announces, one entry for each address family.
        - **content** `string`
          Operation, qualifiers, and payload
      - **watchdog** `container`
        Watchdog-controlled route - held until 'bgp watchdog announce <name>'
        - **name** `string`
          Watchdog group name
        - **withdraw** `boolean`
          Start in withdrawn state (default true)
  - **rib** `container`
    Route Information Base settings for this peer.
    - **adj** `container`
      Adjacency RIB storage for this peer.
      - **in** `boolean`
        Store received routes in Adj-RIB-In, before policy.
      - **out** `boolean`
        Store advertised routes in Adj-RIB-Out, after policy.
    - **out** `container`
      Outbound RIB batching: how route changes are grouped and flushed to the peer.
      - **auto-commit-delay** `uint32`
        Milliseconds to wait before Ze flushes pending route changes to the peer.
      - **group-updates** `boolean`
        Pack routes with identical attributes into one UPDATE.
      - **max-batch-size** `int32`
        Maximum number of route changes in one flush cycle.
  - **role** `container`
    RFC 9234 BGP Role configuration
    - **export** `export-token[]`
      Controls which destination peer roles may receive routes from this peer
    - **import** `role-type`
      Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)
    - **strict** `boolean`
      Require peer to send Role capability
  - **rpki** `container`
    RPKI action overrides of this peer or group.
    - **action** `container`
      Origin-validation action overrides of this peer or group.
      - **invalid** `validation-action`
        Action Ze applies to a route in the Invalid validation state.
      - **not-found** `validation-action`
        Action Ze applies to a route in the NotFound validation state.
    - **aspa** `container`
      ASPA action overrides for this peer or group.
      - **action** `container`
        ASPA path-state action overrides of this peer or group.
        - **invalid** `validation-action`
          Action Ze applies to a route in the ASPA Invalid path state.
        - **unknown** `validation-action`
          Action Ze applies to a route in the ASPA Unknown path state.
    - **blackhole-exempt** `boolean`
      Keep a BLACKHOLE route when its only origin-validation fault is the prefix length.
  - **session** `container`
    BGP session parameters for this peer or group.
    - **accept-srv6-prefix-sid** `boolean`
      Accept the BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer.
    - **as-override** `boolean`
      Replace the ASN of the peer with the local ASN in the outbound AS_PATH.
    - **asn** `container`
      AS number configuration
      - **local** `asn`
        Local AS (overrides global)
      - **local-options** `enumeration[]`
        Modifiers for the local-as behavior of this peer.
      - **migration** `asn`
        Second local AS this iBGP session accepts and sends during an AS migration.
      - **remote** `asn`
        Peer Autonomous System Number
    - **capability** `container`
      BGP capability negotiation
      - **add-path** `container`
        ADD-PATH capability (RFC 7911), with an optional PATHS-LIMIT.
        - **direction** `enumeration`
          Default ADD-PATH direction for all negotiated families.
        - **family <name>** `list`
          Per-family ADD-PATH overrides with optional path count limit.
          - **direction** `enumeration`
            ADD-PATH direction override for this family.
          - **limit** `uint16`
            Maximum paths per prefix to receive (PATHS-LIMIT capability).
          - **mode** `enumeration`
            ADD-PATH negotiation mode for this family.
        - **limit** `uint16`
          Default maximum paths per prefix (PATHS-LIMIT).
      - **asn4** `boolean`
        Advertise the 4-byte ASN capability (RFC 6793).
      - **extended-message** `container`
        Extended Message capability (RFC 8654).
      - **graceful-restart** `container`
        Graceful Restart capability configuration.
        - **long-lived-stale-time** `uint32`
          Long-Lived Stale Time for the peer, in seconds (RFC 9494 Section 3).
        - **restart-time** `uint16`
          Restart Time in seconds (RFC 4724 Section 3). Maximum value is 4095 (12-bit field).
      - **hostname** `container`
        FQDN capability configuration.
        - **domain** `string`
          Domain name (max 255 bytes).
        - **host** `string`
          System hostname (max 255 bytes).
      - **link-local-nexthop** `container`
        Link-local next-hop capability.
      - **nexthop <family>** `list`
        Extended Next Hop capability (RFC 8950)
        - **mode** `enumeration`
          Extended next hop negotiation mode
        - **nhafi** `afi`
          Next-hop AFI (e.g. ipv6)
      - **route-refresh** `container`
        Route Refresh capability (RFC 2918).
      - **software-version** `container`
        Software Version capability (code 75).
        - **mode** `enumeration`
          Capability negotiation mode.
    - **cluster-id** `ipv4-address`
      Override the cluster ID for route reflection (RFC 4456 Section 7).
    - **community** `container`
      Community attribute control for this session
      - **send** `enumeration[]`
        Community types Ze includes in an outbound UPDATE.
    - **domain-name** `string`
      Legacy: Domain name for FQDN capability (group default).
    - **family <name>** `list`
      Address families to negotiate
      - **default-originate** `boolean`
        Originate the default route to this peer for this address family.
      - **default-originate-filter** `string`
        Named filter that must accept before Ze originates the default route.
      - **mode** `enumeration`
        Address family negotiation mode
      - **prefix** `container`
        Prefix limit configuration for this address family
        - **count** `enumeration`
          Which prefixes the count compared against maximum holds.
        - **idle-timeout** `uint16`
          Seconds to wait before reconnect after this family stopped the session.
        - **maximum** `uint32`
          Hard maximum number of prefixes accepted
        - **reconnect** `enumeration`
          What the peer does after this family stopped the session.
        - **teardown** `boolean`
          Tear down session when prefix maximum exceeded (false = warn only)
        - **updated** `string`
          ISO date (YYYY-MM-DD) when prefix maximum was last updated from PeeringDB. Hidden leaf.
        - **warning** `uint32`
          Warning threshold. Defaults to 90% of maximum when not set.
    - **host-name** `string`
      Legacy: Host name for FQDN capability (group default).
    - **irr** `container`
      Per-peer IRR filter settings.
      - **as-set** `string`
        Explicit AS-SET name for the IRR prefix lookup.
      - **enable** `enumeration`
        Enable or disable IRR filtering for this peer.
    - **link-local** `ipv6-address`
      IPv6 link-local address advertised after the global next hop (RFC 2545 Section 3).
    - **next-hop** `union`
      Next-hop rewriting policy for a forwarded UPDATE (RFC 4271 Section 5.1.3).
    - **propagate-srv6-prefix-sid** `boolean`
      Advertise the BGP Prefix-SID attribute (code 40) to this EBGP peer.
    - **route-reflector-client** `boolean`
      Mark this peer as a route reflector client (RFC 4456).
    - **router-id** `ipv4-address`
      Override the router ID for this peer.
    - **rs-client** `boolean`
      Mark this peer as an RS-client for transparent AS-path forwarding.
  - **timer** `container`
    BGP session timers: hold time, keepalive interval, and connect retry delay.
    - **connect-retry** `uint16`
      Connect retry interval in seconds. Ze reads it for a dynamic peer alone.
    - **keepalive** `uint16`
      Keepalive interval in seconds (RFC 4271 Section 10). 0 = auto: hold-time/3.
    - **receive-hold-time** `uint16`
      Receive hold time in seconds (RFC 4271: 0 or >= 3). Proposed in OPEN.
    - **send-hold-time** `uint16`
      Send hold time in seconds (RFC 9687). 0 = auto: max(480, 2x receive-hold-time).
  - **update** `list`
    Native Ze route announcements
    - **attribute** `container`
      Path attributes for routes in this block
      - **aggregator** `string`
        AGGREGATOR attribute (RFC 4271 Section 5.1.7). The format is AS:IP.
      - **as-path** `string[]`
        AS_PATH segments prepended to the route, as space-separated ASNs.
      - **atomic-aggregate** `boolean`
        ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6).
      - **attribute** `string[]`
        Raw BGP attributes in hex encoding, as flags:type:hex-value.
      - **bgp-prefix-sid** `container`
        Prefix-SID attribute (RFC 8669).
      - **bgp-prefix-sid-srv6** `container`
        SRv6 Prefix-SID attribute (RFC 9252).
      - **cluster-list** `ipv4-address[]`
        CLUSTER_LIST for route reflection (RFC 4456).
      - **community** `string[]`
        Standard BGP communities (RFC 1997).
      - **extended-community** `string[]`
        Extended communities (RFC 4360). The format is type:admin:value.
      - **label** `string`
        MPLS label value for a labeled unicast route (RFC 3107). The value is a number.
      - **labels** `string[]`
        MPLS label stack for a multi-label route (RFC 8277).
      - **large-community** `string[]`
        Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
      - **local-preference** `uint32`
        LOCAL_PREF value for iBGP best-path selection. The higher value wins.
      - **med** `uint32`
        Multi-Exit Discriminator (MED). The lower value is preferred.
      - **next-hop** `union`
        Next hop address or 'self'
      - **origin** `enumeration`
        ORIGIN attribute (RFC 4271 Section 4.3)
      - **originator-id** `ipv4-address`
        ORIGINATOR_ID for route reflection (RFC 4456).
      - **path-information** `string`
        ADD-PATH path identifier (RFC 7911).
      - **rd** `route-distinguisher`
        Route Distinguisher (RFC 4364). The format is ASN:nn or IP:nn.
      - **split** `string`
        Split one prefix into more-specific prefixes for the announcement.
    - **name** `string`
      Optional label for this update block (display only)
    - **nlri <name>** `list`
      Routes this update block announces, one entry for each address family.
      - **content** `string`
        Operation, qualifiers, and payload
    - **watchdog** `container`
      Watchdog-controlled route - held until 'bgp watchdog announce <name>'
      - **name** `string`
        Watchdog group name
      - **withdraw** `boolean`
        Start in withdrawn state (default true)
- **healthcheck** `container`
  Healthcheck probes for service-aware BGP route management
  - **probe <name>** `list`
    Healthcheck probe definition
    - **command** `string`
      Shell command to execute for health check (exit 0 = success)
    - **debounce** `boolean`
      When true, only dispatch watchdog commands on state changes
    - **disable** `boolean`
      Admin disable: probe enters DISABLED state immediately
    - **disabled-metric** `uint32`
      MED value when DISABLED (used when withdraw-on-down is false)
    - **down-metric** `uint32`
      MED value when DOWN (used when withdraw-on-down is false)
    - **fall** `uint32`
      Consecutive failures before DOWN
    - **fast-interval** `uint32`
      Seconds between checks during RISING/FALLING states
    - **group** `string`
      Watchdog group name (exclusive: one probe per group)
    - **interval** `uint32`
      Seconds between checks (0 = single check then dormant)
    - **ip-setup** `container`
      VIP management on local interface (internal plugin mode only)
      - **dynamic** `boolean`
        When true, remove IPs on DOWN/DISABLED, restore on UP
      - **interface** `string`
        Target interface for VIPs (e.g., lo, dummy0)
      - **ip** `string[]`
        VIP addresses in CIDR notation (e.g., 10.0.0.1/32)
    - **on-change** `string[]`
      Shell commands to execute on any state transition (30s timeout, runs after state-specific hooks)
    - **on-disabled** `string[]`
      Shell commands to execute on transition to DISABLED (30s timeout)
    - **on-down** `string[]`
      Shell commands to execute on transition to DOWN (30s timeout)
    - **on-up** `string[]`
      Shell commands to execute on transition to UP (30s timeout)
    - **rise** `uint32`
      Consecutive successes before UP
    - **timeout** `uint32`
      Command timeout in seconds
    - **up-metric** `uint32`
      MED value when UP
    - **withdraw-on-down** `boolean`
      Withdraw the route when the probe is DOWN or DISABLED.
- **multipath** `container`
  BGP multipath / ECMP configuration
  - **maximum-paths** `uint16`
    Maximum number of equal-cost paths Ze installs per prefix.
  - **relax-as-path** `boolean`
    Treat paths with different AS-paths as equal-cost.
- **peer <name>** `list`
  BGP peer configuration (standalone, no group)
  - **attach** `container`
    Programs this peer attaches.
    - **process <name>** `list`
      External process that receives BGP events and can inject a message.
      - **content** `container`
        Controls the encoding and filtering of events sent to this process.
        - **attribute** `string`
          Filter expression to select which BGP attributes are included in events.
        - **encoding** `string`
          Wire encoding for events sent to the process (e.g., json, text).
        - **format** `string`
          Output format template for event rendering.
      - **neighbor-changes** `container`
        Send session state change events to this process.
      - **processes** `string[]`
        Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.
      - **processes-match** `string[]`
        Legacy ExaBGP process match patterns for event filtering.
      - **receive** `string[]`
        Event types this peer feeds the program.
      - **run** `string`
        Shell command that starts the external process.
      - **send** `string[]`
        Message types this program can send toward the peer.
  - **behavior** `container`
    Controls how the reactor processes and forwards an UPDATE for this peer.
    - **auto-flush** `boolean`
      Withdraw every route advertised to this peer when the session goes down.
    - **group-updates** `boolean`
      Pack several NLRI into one UPDATE message when they share the same path attributes.
    - **manual-eor** `boolean`
      Do not send End-of-RIB automatically after the initial route advertisement.
    - **rs-fast-path** `boolean`
      Forward a received UPDATE inside the reactor for an RS-client peer.
  - **blackhole** `container`
    RFC 7999 blackhole agreement on this BGP session, in both directions.
    - **communities** `string[]`
      Communities that make Ze discard traffic toward a prefix this peer announces.
    - **prefixes** `string[]`
      The prefixes this peer is authorized to advertise.
  - **capture** `container`
    Protocol event capture for this peer.
    - **directory** `string`
      Directory that holds the capture files of this peer.
    - **enabled** `boolean`
      Capture this peer's inbound protocol events to a file. Off by default.
    - **maximum-size** `uint32`
      Hard cap on the size of one capture file.
    - **on-limit** `enumeration`
      What happens when a capture file reaches maximum-size.
  - **connection** `container`
    Transport-level connection settings
    - **bfd** `container`
      Bidirectional Forwarding Detection options for this peer (RFC 5880).
      - **enabled** `boolean`
        Master switch for the BFD session of this peer.
      - **hold-down** `uint32`
        BFD hold-down interval for strict mode (draft-ietf-idr-bgp-bfd-strict-mode Section 10).
      - **hold-time** `uint16`
        BfdHoldTime for strict mode (draft-ietf-idr-bgp-bfd-strict-mode Section 3).
      - **interface** `string`
        Single-hop egress interface for the BFD session.
      - **min-ttl** `uint8`
        Multi-hop minimum acceptable TTL (RFC 5883 Section 5).
      - **mode** `enumeration`
        BFD hop mode for this peer: single-hop or multi-hop.
      - **profile** `string`
        Name of a profile defined under the top-level bfd { profile ... } block.
      - **strict** `boolean`
        BFD strict mode (draft-ietf-idr-bgp-bfd-strict-mode).
    - **link-local** `boolean`
      Auto-discover IPv6 link-local address for TCP connection
    - **local** `container`
      Local endpoint for the TCP session.
      - **accept** `boolean`
        Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Local address for connection (use IP address or 'auto')
      - **port** `port`
        Local listen port
    - **md5** `container`
      TCP MD5 authentication (RFC 2385)
      - **ip** `ip-address`
        MD5 authentication IP
      - **password** `string`
        MD5 authentication password
    - **remote** `container`
      Remote endpoint for the TCP session.
      - **connect** `boolean`
        Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Peer IP address or 'dynamic' for dynamic peer groups
      - **max-peers** `uint32`
        Maximum number of dynamic peers for this group
      - **port** `port`
        Remote connection port
      - **range** `ip-prefix[]`
        IP prefix ranges for a dynamic peer group.
    - **ttl** `container`
      TTL settings for BGP sessions
      - **max** `uint8`
        TTL security / GTSM (RFC 5082)
      - **min** `uint8`
        Minimum incoming TTL
      - **set** `uint8`
        Outgoing TTL value
  - **description** `string`
    Free-text label for this peer.
  - **filter** `container`
    Route filter chains for import and export.
    - **egress** `container`
      Egress direction filters.
      - **community** `container`
        Community filter for egress.
        - **strip** `string[]`
          Named communities to remove on egress.
        - **tag** `string[]`
          Named communities to add on egress.
    - **export** `string[]`
      Export filter chain (filter instance names)
    - **import** `string[]`
      Import filter chain (filter instance names)
    - **ingress** `container`
      Ingress direction filters.
      - **community** `container`
        Community filter for ingress.
        - **blackhole-propagation** `enumeration`
          Community to add to a route that carries BLACKHOLE (65535:666).
        - **relation-function** `uint32`
          Function number of the relation tag.
        - **relation-tag** `boolean`
          Write the relation-to-origin large community of RFC 8195 on each route from this peer.
        - **scrub-keep-function** `uint32[]`
          Function numbers a peer can send with the local AS as the Global Administrator.
        - **scrub-own-ga** `boolean`
          Remove inbound communities whose Global Administrator is the local AS.
        - **strip** `string[]`
          Named communities to remove on ingress.
        - **tag** `string[]`
          Named communities to add on ingress.
  - **rib** `container`
    Route Information Base settings for this peer.
    - **adj** `container`
      Adjacency RIB storage for this peer.
      - **in** `boolean`
        Store received routes in Adj-RIB-In, before policy.
      - **out** `boolean`
        Store advertised routes in Adj-RIB-Out, after policy.
    - **out** `container`
      Outbound RIB batching: how route changes are grouped and flushed to the peer.
      - **auto-commit-delay** `uint32`
        Milliseconds to wait before Ze flushes pending route changes to the peer.
      - **group-updates** `boolean`
        Pack routes with identical attributes into one UPDATE.
      - **max-batch-size** `int32`
        Maximum number of route changes in one flush cycle.
  - **role** `container`
    RFC 9234 BGP Role configuration
    - **export** `export-token[]`
      Controls which destination peer roles may receive routes from this peer
    - **import** `role-type`
      Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)
    - **strict** `boolean`
      Require peer to send Role capability
  - **rpki** `container`
    RPKI action overrides of this peer or group.
    - **action** `container`
      Origin-validation action overrides of this peer or group.
      - **invalid** `validation-action`
        Action Ze applies to a route in the Invalid validation state.
      - **not-found** `validation-action`
        Action Ze applies to a route in the NotFound validation state.
    - **aspa** `container`
      ASPA action overrides for this peer or group.
      - **action** `container`
        ASPA path-state action overrides of this peer or group.
        - **invalid** `validation-action`
          Action Ze applies to a route in the ASPA Invalid path state.
        - **unknown** `validation-action`
          Action Ze applies to a route in the ASPA Unknown path state.
    - **blackhole-exempt** `boolean`
      Keep a BLACKHOLE route when its only origin-validation fault is the prefix length.
  - **session** `container`
    BGP session parameters for this peer or group.
    - **accept-srv6-prefix-sid** `boolean`
      Accept the BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer.
    - **as-override** `boolean`
      Replace the ASN of the peer with the local ASN in the outbound AS_PATH.
    - **asn** `container`
      AS number configuration
      - **local** `asn`
        Local AS (overrides global)
      - **local-options** `enumeration[]`
        Modifiers for the local-as behavior of this peer.
      - **migration** `asn`
        Second local AS this iBGP session accepts and sends during an AS migration.
      - **remote** `asn`
        Peer Autonomous System Number
    - **capability** `container`
      BGP capability negotiation
      - **add-path** `container`
        ADD-PATH capability (RFC 7911), with an optional PATHS-LIMIT.
        - **direction** `enumeration`
          Default ADD-PATH direction for all negotiated families.
        - **family <name>** `list`
          Per-family ADD-PATH overrides with optional path count limit.
          - **direction** `enumeration`
            ADD-PATH direction override for this family.
          - **limit** `uint16`
            Maximum paths per prefix to receive (PATHS-LIMIT capability).
          - **mode** `enumeration`
            ADD-PATH negotiation mode for this family.
        - **limit** `uint16`
          Default maximum paths per prefix (PATHS-LIMIT).
      - **asn4** `boolean`
        Advertise the 4-byte ASN capability (RFC 6793).
      - **extended-message** `container`
        Extended Message capability (RFC 8654).
      - **graceful-restart** `container`
        Graceful Restart capability configuration.
        - **long-lived-stale-time** `uint32`
          Long-Lived Stale Time for the peer, in seconds (RFC 9494 Section 3).
        - **restart-time** `uint16`
          Restart Time in seconds (RFC 4724 Section 3). Maximum value is 4095 (12-bit field).
      - **hostname** `container`
        FQDN capability configuration.
        - **domain** `string`
          Domain name (max 255 bytes).
        - **host** `string`
          System hostname (max 255 bytes).
      - **link-local-nexthop** `container`
        Link-local next-hop capability.
      - **nexthop <family>** `list`
        Extended Next Hop capability (RFC 8950)
        - **mode** `enumeration`
          Extended next hop negotiation mode
        - **nhafi** `afi`
          Next-hop AFI (e.g. ipv6)
      - **route-refresh** `container`
        Route Refresh capability (RFC 2918).
      - **software-version** `container`
        Software Version capability (code 75).
        - **mode** `enumeration`
          Capability negotiation mode.
    - **cluster-id** `ipv4-address`
      Override the cluster ID for route reflection (RFC 4456 Section 7).
    - **community** `container`
      Community attribute control for this session
      - **send** `enumeration[]`
        Community types Ze includes in an outbound UPDATE.
    - **domain-name** `string`
      Legacy: Domain name for FQDN capability.
    - **family <name>** `list`
      Address families to negotiate
      - **default-originate** `boolean`
        Originate the default route to this peer for this address family.
      - **default-originate-filter** `string`
        Named filter that must accept before Ze originates the default route.
      - **mode** `enumeration`
        Address family negotiation mode
      - **prefix** `container`
        Prefix limit configuration for this address family
        - **count** `enumeration`
          Which prefixes the count compared against maximum holds.
        - **idle-timeout** `uint16`
          Seconds to wait before reconnect after this family stopped the session.
        - **maximum** `uint32`
          Hard maximum number of prefixes accepted
        - **reconnect** `enumeration`
          What the peer does after this family stopped the session.
        - **teardown** `boolean`
          Tear down session when prefix maximum exceeded (false = warn only)
        - **updated** `string`
          ISO date (YYYY-MM-DD) when prefix maximum was last updated from PeeringDB. Hidden leaf.
        - **warning** `uint32`
          Warning threshold. Defaults to 90% of maximum when not set.
    - **host-name** `string`
      Legacy: Host name for FQDN capability.
    - **irr** `container`
      Per-peer IRR filter settings.
      - **as-set** `string`
        Explicit AS-SET name for the IRR prefix lookup.
      - **enable** `enumeration`
        Enable or disable IRR filtering for this peer.
    - **link-local** `ipv6-address`
      IPv6 link-local address advertised after the global next hop (RFC 2545 Section 3).
    - **next-hop** `union`
      Next-hop rewriting policy for a forwarded UPDATE (RFC 4271 Section 5.1.3).
    - **propagate-srv6-prefix-sid** `boolean`
      Advertise the BGP Prefix-SID attribute (code 40) to this EBGP peer.
    - **route-reflector-client** `boolean`
      Mark this peer as a route reflector client (RFC 4456).
    - **router-id** `ipv4-address`
      Override the router ID for this peer.
    - **rs-client** `boolean`
      Mark this peer as an RS-client for transparent AS-path forwarding.
  - **timer** `container`
    BGP session timers: hold time, keepalive interval, and connect retry delay.
    - **connect-retry** `uint16`
      Connect retry interval in seconds. Ze reads it for a dynamic peer alone.
    - **keepalive** `uint16`
      Keepalive interval in seconds (RFC 4271 Section 10). 0 = auto: hold-time/3.
    - **receive-hold-time** `uint16`
      Receive hold time in seconds (RFC 4271: 0 or >= 3). Proposed in OPEN.
    - **send-hold-time** `uint16`
      Send hold time in seconds (RFC 9687). 0 = auto: max(480, 2x receive-hold-time).
  - **update** `list`
    Native Ze route announcements
    - **attribute** `container`
      Path attributes for routes in this block
      - **aggregator** `string`
        AGGREGATOR attribute (RFC 4271 Section 5.1.7). The format is AS:IP.
      - **as-path** `string[]`
        AS_PATH segments prepended to the route, as space-separated ASNs.
      - **atomic-aggregate** `boolean`
        ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6).
      - **attribute** `string[]`
        Raw BGP attributes in hex encoding, as flags:type:hex-value.
      - **bgp-prefix-sid** `container`
        Prefix-SID attribute (RFC 8669).
      - **bgp-prefix-sid-srv6** `container`
        SRv6 Prefix-SID attribute (RFC 9252).
      - **cluster-list** `ipv4-address[]`
        CLUSTER_LIST for route reflection (RFC 4456).
      - **community** `string[]`
        Standard BGP communities (RFC 1997).
      - **extended-community** `string[]`
        Extended communities (RFC 4360). The format is type:admin:value.
      - **label** `string`
        MPLS label value for a labeled unicast route (RFC 3107). The value is a number.
      - **labels** `string[]`
        MPLS label stack for a multi-label route (RFC 8277).
      - **large-community** `string[]`
        Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
      - **local-preference** `uint32`
        LOCAL_PREF value for iBGP best-path selection. The higher value wins.
      - **med** `uint32`
        Multi-Exit Discriminator (MED). The lower value is preferred.
      - **next-hop** `union`
        Next hop address or 'self'
      - **origin** `enumeration`
        ORIGIN attribute (RFC 4271 Section 4.3)
      - **originator-id** `ipv4-address`
        ORIGINATOR_ID for route reflection (RFC 4456).
      - **path-information** `string`
        ADD-PATH path identifier (RFC 7911).
      - **rd** `route-distinguisher`
        Route Distinguisher (RFC 4364). The format is ASN:nn or IP:nn.
      - **split** `string`
        Split one prefix into more-specific prefixes for the announcement.
    - **name** `string`
      Optional label for this update block (display only)
    - **nlri <name>** `list`
      Routes this update block announces, one entry for each address family.
      - **content** `string`
        Operation, qualifiers, and payload
    - **watchdog** `container`
      Watchdog-controlled route - held until 'bgp watchdog announce <name>'
      - **name** `string`
        Watchdog group name
      - **withdraw** `boolean`
        Start in withdrawn state (default true)
- **policy** `container`
  Named filter definitions for the route policy framework.
  - **as-path-length <name>** `list`
    Named AS-path length filter instance.
    - **max** `uint16`
      Maximum allowed AS-path length (inclusive). Routes with longer paths are rejected.
    - **min** `uint16`
      Minimum required AS-path length (inclusive). Routes with shorter paths are rejected.
  - **as-path-list <name>** `list`
    Named AS-path regex filter instance.
    - **entry <regex>** `list`
      One regex entry of the ordered list.
      - **action** `enumeration`
        Action applied when this entry's regex matches.
  - **community-match <name>** `list`
    Named community match filter instance.
    - **entry <community>** `list`
      Ordered match entry, checked against the route's community attributes.
      - **action** `enumeration`
        Action applied when this community is found in the route.
      - **type** `enumeration`
        Which community attribute type to check.
  - **family-filter <name>** `list`
    Named address-family filter instance.
    - **action** `enumeration`
      Action to apply when the family matches.
    - **family** `address-family`
      Address family to match, for example ipv4/flowspec.
  - **irr** `container`
    Global IRR filtering settings.
    - **peeringdb-url** `string`
      Base URL of the PeeringDB API that Ze queries.
    - **refresh-interval** `uint32`
      Seconds between automatic IRR re-queries.
    - **server** `string`
      IRR whois server hostname or host:port.
    - **source-address** `ip-address`
      Source IP address for outbound IRR whois connections.
  - **loop-detection <name>** `list`
    Named loop detection filter instance.
    - **allow-own-as** `uint8`
      Own-AS occurrences permitted in AS_PATH before ze rejects the route.
    - **cluster-id** `ipv4-address`
      Override Router ID for CLUSTER_LIST loop detection.
  - **modify <name>** `list`
    Named route attribute modifier instance.
    - **decrement** `container`
      Attributes to decrement on a matching route.
      - **aigp** `uint32`
        Decrement AIGP metric by this value.
      - **local-preference** `uint32`
        Decrement LOCAL_PREF by this value.
      - **med** `uint32`
        Decrement MED by this value.
    - **del** `container`
      Attributes Ze deletes from a matching route.
      - **med** `empty`
        Remove the MULTI_EXIT_DISC attribute (RFC 4271 type 4) from the route.
    - **increment** `container`
      Attributes to increment on a matching route.
      - **aigp** `uint32`
        Increment AIGP metric by this value.
      - **local-preference** `uint32`
        Increment LOCAL_PREF by this value.
      - **med** `uint32`
        Increment MED by this value.
    - **match** `container`
      Condition a route meets before the operations apply.
      - **community** `string[]`
        Apply only to a route whose COMMUNITIES attribute carries this value.
      - **extended-community** `string[]`
        Apply only to a route whose EXTENDED COMMUNITIES attribute carries this value.
      - **large-community** `string[]`
        Apply only to a route whose LARGE COMMUNITIES attribute carries this value.
    - **set** `container`
      Attributes Ze sets on a matching route.
      - **as-path-prepend** `uint8`
        Prepend the local AS to AS_PATH this many times.
      - **community-add** `string[]`
        Standard community values to add (ASN:VAL format).
      - **community-remove** `string[]`
        Standard community values to remove (ASN:VAL format).
      - **extended-community-add** `string[]`
        Extended community values to add (target:ASN:NN or hex).
      - **extended-community-remove** `string[]`
        Extended community values to remove (target:ASN:NN or hex).
      - **large-community-add** `string[]`
        Large community values to add (GA:LD1:LD2 format).
      - **large-community-remove** `string[]`
        Large community values to remove (GA:LD1:LD2 format).
      - **local-preference** `uint32`
        Set LOCAL_PREF attribute (RFC 4271 type 5).
      - **med** `uint32`
        Set MULTI_EXIT_DISC attribute (RFC 4271 type 4).
      - **next-hop** `ip-address`
        Set the NEXT_HOP attribute of the route.
      - **origin** `enumeration`
        Set ORIGIN attribute (RFC 4271 type 1).
  - **prefix-list <name>** `list`
    Named prefix-list filter instance.
    - **entry <prefix>** `list`
      Ordered match entry. First match wins per route prefix.
      - **action** `enumeration`
        Action applied when this entry matches a route prefix.
      - **ge** `uint8`
        Minimum match length (greater-than-or-equal). Defaults to the prefix length of this entry.
      - **le** `uint8`
        Maximum match length (less-than-or-equal). Defaults to 32 for IPv4 or 128 for IPv6.
  - **reject-asn <name>** `list`
    Named list of ASNs rejected by their position in the AS_PATH.
    - **anywhere** `asn[]`
      Reject these ASNs at any position: direct, transit and origin together.
    - **direct** `asn[]`
      Reject these ASNs where they are the peer we are talking to, prepends collapsed.
    - **indirect** `asn[]`
      Reject these ASNs anywhere they are NOT the peer we are talking to.
    - **nth <index>** `list`
      Reject the listed ASNs at one collapsed position of the AS_PATH.
      - **asn** `asn[]`
        The ASNs this position rejects.
    - **origin** `asn[]`
      Reject these ASNs where they are the last ASN of the AS_PATH.
    - **regex** `string[]`
      Go RE2 patterns matched against the whole space-separated AS_PATH string.
    - **transit** `asn[]`
      Reject these ASNs where they sit past the peer and are not the last ASN of the AS_PATH.
  - **remove-private-as <name>** `list`
    Named action filter that removes RFC 6996 Private Use ASNs.
    - **replace-with** `enumeration`
      Replacement mode. When absent, Private Use ASNs are stripped.
- **rib** `container`
  RIB plugin state and operations.
  - **adj-rib-in** `container`
    Routes received from peers.
    - **peer <address>** `list`
      Per-peer Adj-RIB-In.
      - **route-count** `uint32`
        Number of routes from this peer.
  - **adj-rib-out** `container`
    Routes sent to peers.
    - **peer <address>** `list`
      Per-peer Adj-RIB-Out.
      - **route-count** `uint32`
        Number of routes to this peer.
- **route-server** `container`
  Route server plugin tuning parameters
  - **worker-queue-size** `uint32`
    Worker channel capacity per source peer, overridden by ze.bgp.route-server.worker-queue-size.
- **router-id** `ipv4-address`
  BGP Router ID for this speaker. The leaf is required.
- **rpki** `container`
  RPKI origin validation of a received route (RFC 6811).
  - **action** `container`
    Global origin-validation actions.
    - **invalid** `validation-action`
      Action Ze applies to a route in the Invalid validation state.
    - **not-found** `validation-action`
      Action Ze applies to a route in the NotFound validation state.
  - **aspa** `container`
    AS_PATH verification against the ASPA records, and the action of each result.
    - **action** `container`
      Global action Ze applies for each ASPA path state.
      - **invalid** `validation-action`
        Action Ze applies to a route in the ASPA Invalid path state.
      - **unknown** `validation-action`
        Action Ze applies to a route in the ASPA Unknown path state.
    - **validation** `boolean`
      Verify the AS path of a route against the ASPA records.
  - **cache-server <address>** `list`
    RTR cache servers Ze reads the validated ROA and ASPA records from.
    - **port** `uint16`
      TCP port of the RTR cache server.
    - **preference** `uint8`
      Preference of this cache server.
    - **source-address** `ip-address`
      Source IP address of an outbound RTR connection.
  - **validation-timeout** `uint16`
    Fail-open timeout of a route whose validation is pending.
- **session** `container`
  Global BGP session defaults
  - **allow-shared-router-id** `boolean`
    Accept a peer whose BGP Identifier duplicates another established peer in the same AS.
  - **asn** `container`
    AS number configuration
    - **local** `asn`
      Local Autonomous System Number (required)
- **update** `list`
  Native Ze route announcements
  - **attribute** `container`
    Path attributes for routes in this block
    - **aggregator** `string`
      AGGREGATOR attribute (RFC 4271 Section 5.1.7). The format is AS:IP.
    - **as-path** `string[]`
      AS_PATH segments prepended to the route, as space-separated ASNs.
    - **atomic-aggregate** `boolean`
      ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6).
    - **attribute** `string[]`
      Raw BGP attributes in hex encoding, as flags:type:hex-value.
    - **bgp-prefix-sid** `container`
      Prefix-SID attribute (RFC 8669).
    - **bgp-prefix-sid-srv6** `container`
      SRv6 Prefix-SID attribute (RFC 9252).
    - **cluster-list** `ipv4-address[]`
      CLUSTER_LIST for route reflection (RFC 4456).
    - **community** `string[]`
      Standard BGP communities (RFC 1997).
    - **extended-community** `string[]`
      Extended communities (RFC 4360). The format is type:admin:value.
    - **label** `string`
      MPLS label value for a labeled unicast route (RFC 3107). The value is a number.
    - **labels** `string[]`
      MPLS label stack for a multi-label route (RFC 8277).
    - **large-community** `string[]`
      Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
    - **local-preference** `uint32`
      LOCAL_PREF value for iBGP best-path selection. The higher value wins.
    - **med** `uint32`
      Multi-Exit Discriminator (MED). The lower value is preferred.
    - **next-hop** `union`
      Next hop address or 'self'
    - **origin** `enumeration`
      ORIGIN attribute (RFC 4271 Section 4.3)
    - **originator-id** `ipv4-address`
      ORIGINATOR_ID for route reflection (RFC 4456).
    - **path-information** `string`
      ADD-PATH path identifier (RFC 7911).
    - **rd** `route-distinguisher`
      Route Distinguisher (RFC 4364). The format is ASN:nn or IP:nn.
    - **split** `string`
      Split one prefix into more-specific prefixes for the announcement.
  - **name** `string`
    Optional label for this update block (display only)
  - **nlri <name>** `list`
    Routes this update block announces, one entry for each address family.
    - **content** `string`
      Operation, qualifiers, and payload
  - **watchdog** `container`
    Watchdog-controlled route - held until 'bgp watchdog announce <name>'
    - **name** `string`
      Watchdog group name
    - **withdraw** `boolean`
      Start in withdrawn state (default true)
- **update-delay** `container`
  Hold the first advertisement at startup until the RIB settles.
  - **establish-wait** `uint16`
    Seconds after which the peers that are held end the hold. Must not exceed max-delay.
  - **max-delay** `uint16`
    Seconds to hold the first advertisement. 0 disables the hold.

## class-of-service

*Provided by `cos` ([ze-cos-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/cos/yang/ze-cos-conf.yang))*

Named class-of-service profiles.

- **ieee-802.1p <name>** `list`
  An 802.1p QoS profile.
  - **egress** `container`
    Priority-to-PCP mapping of a transmitted tagged frame.
    - **priority <value>** `list`
      Map one internal priority to a transmitted PCP value.
      - **pcp** `uint8`
        PCP value Ze stamps in the 802.1Q header.
  - **ingress** `container`
    PCP-to-priority mapping of a received tagged frame.
    - **pcp <value>** `list`
      Map one received PCP value to an internal priority.
      - **priority** `uint8`
        Internal priority Ze gives to a matching frame.

## connected

*Provided by `connected` ([ze-connected-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/connected/yang/ze-connected-conf.yang))*

Connected route redistribution. Presence enables the plugin.


## control-plane-protection

*Provided by `copp` ([ze-copp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/copp/yang/ze-copp-conf.yang))*

Control-plane policing configuration.

- **bgp** `container`
  Rate-limit new TCP connections to the BGP listen port.
  - **burst** `uint32`
    Burst size (packets or bytes matching the rate unit).
  - **over-limit-policy** `enumeration`
    What Ze does with the packets that exceed the rate limit.
  - **protected-port** `uint16[]`
    TCP port(s) the control-plane policing rules protect, 179 (BGP) by default.
  - **rate** `rate-spec`
    Rate limit for new connections (e.g., 100/second).
  - **trusted-source** `string[]`
    Source prefixes that bypass the rate limit. Typically the addresses of configured BGP peers.

## ddos

Distributed denial-of-service detection and mitigation subsystem.

- **detect** `container`
  *Provided by `ddos-detect` ([ze-ddos-detect-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ddos/detect/yang/ze-ddos-detect-conf.yang))*
  Detect a volumetric attack from the traffic Ze already sees.
  - **absolute-floor** `uint32`
    Minimum threshold in PPS regardless of baseline.
  - **baseline-window** `uint32`
    Rolling baseline window size in samples.
  - **bps-floor** `uint64`
    Minimum bandwidth in bits/sec below which the BPS trigger is inert.
  - **bps-threshold-multiplier** `decimal-2`
    Baseline p99 multiplier for the bandwidth (BPS) trigger.
  - **bps-trigger-enable** `boolean`
    Enable the bandwidth (BPS) trigger beside the PPS threshold.
  - **characterize-enable** `boolean`
    Run Stage-2 flow characterization on a detected attack.
  - **characterize-timeout** `uint16`
    Milliseconds budget for the on-trigger queries.
  - **characterize-window** `uint16`
    Seconds of recent flows to consider when characterizing an attack.
  - **check-interval** `uint16`
    Seconds between detection evaluations.
  - **clear-consecutive-checks** `uint16`
    Consecutive evaluations below threshold before clearing.
  - **confirm-duration** `uint16`
    Consecutive evaluations above threshold before triggering.
  - **enabled** `boolean`
    Enable the DDoS detector.
  - **entropy-threshold** `decimal-2`
    Source-address Shannon entropy in bits that marks an attack distributed.
  - **policy** `container`
    Allow and deny traffic policy applied to a detected attack.
    - **default-action** `enumeration`
      Disposition when no rule matches the attack.
    - **rule <prefix>** `list`
      One allow or deny rule matched against an attack, keyed by prefix.
      - **action** `enumeration`
        allow exempts matching traffic and deny subjects it to DDoS handling.
      - **match** `enumeration`
        Whether the prefix matches the attack source, the destination, or either.
      - **scope** `enumeration`
        Stage the action governs, mitigation or detection.
  - **startup-grace** `uint16`
    Seconds after startup where only extreme spikes trigger.
  - **threshold-multiplier** `decimal-2`
    Baseline p99 multiplier for the dynamic threshold.
  - **top-n-sources** `uint16`
    Maximum number of attacker source addresses ranked into TopSources by packet volume.
- **flowspec** `container`
  *Provided by `ddos-flowspec` ([ze-ddos-flowspec-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang))*
  Answer a detected attack by announcing a BGP FlowSpec rule.
  - **action** `enumeration`
    FlowSpec traffic-action: discard or rate-limit. Mandatory, with no default.
  - **announce-rate-limit** `uint16`
    Maximum FlowSpec announcements per minute.
  - **backoff-cap** `uint32`
    Maximum hold-down after exponential backoff.
  - **blackhole-fallback** `boolean`
    Announce an immediate upstream discard on a critical-severity AttackDetected.
  - **confidence-min** `uint8`
    Minimum incident confidence (0-100) to announce an upstream rule from a characterized attack.
  - **hold-down** `uint32`
    Minimum seconds before the first leak-probe after announcement.
  - **max-mitigation-duration** `uint32`
    Maximum seconds a FlowSpec rule stays announced (0 = no cap).
  - **probe-interval** `uint16`
    Seconds between leak-probe attempts after hold-down.
  - **probe-rate** `uint32`
    Bits per second to allow during a leak-probe.
  - **probe-window** `uint16`
    Seconds to observe leaked traffic during a probe.
  - **rate-limit-bytes** `uint64`
    Bytes per second announced in the RFC 8955 traffic-rate extended community.
  - **response-level** `enumeration`
    Action on attack detection: alert (log only) or enforce (announce FlowSpec).
- **flowtriq** `container`
  *Provided by `ddos-flowtriq` ([ze-ddos-flowtriq-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ddos/flowtriq/yang/ze-ddos-flowtriq-conf.yang))*
  Feed detected attacks to the FlowTriq reporting service.
  - **api-base** `string`
    Flowtriq API base URL.
  - **api-key** `string`
    Flowtriq API bearer token.
  - **enabled** `boolean`
    Enable reporting DDoS incidents to the Flowtriq cloud API.
  - **node-uuid** `string`
    Node UUID for Flowtriq agent identification.
- **local** `container`
  *Provided by `ddos-local` ([ze-ddos-local-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang))*
  Answer a detected attack with a firewall rule on this router.
  - **confidence-min** `uint8`
    Minimum incident confidence, from 0 to 100, to install a drop rule.
  - **forward-mitigation** `boolean`
    Drop the traffic of a remote transit victim on the netfilter FORWARD hook.
  - **max-mitigation-duration** `uint32`
    Maximum seconds a drop rule stays installed (0 = no cap).
  - **response-level** `enumeration`
    Action on attack detection: alert (log only) or enforce (install drop rule).
- **observe** `container`
  *Provided by `ddos-observe` ([ze-ddos-observe-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ddos/observe/yang/ze-ddos-observe-conf.yang))*
  Keep a ring of recent incidents for an operator to read back.
  - **incident-ring-size** `uint32`
    Maximum number of incidents to retain in memory.
  - **stale-incident-timeout** `uint32`
    Seconds before an open incident without a clear event is auto-finalized.

## environment

*Provided by `bgp-bmp` ([ze-bmp-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-cmd.yang), [ze-bmp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang)); `ntp` ([ze-ntp-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ntp/yang/ze-ntp-cmd.yang), [ze-ntp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ntp/yang/ze-ntp-conf.yang))*

Environment settings for API transports

- **api-server** `container`
  API engine settings (shared parent for the REST/gRPC transports)
  - **grpc** `container`
    gRPC API transport. Authenticated non-loopback listeners require tls-cert and tls-key.
    - **enabled** `boolean`
      Enable gRPC API server
    - **server <name>** `list`
      gRPC API listen endpoints
      - **ip** `ip-address`
        Listen IP address
      - **port** `listener-port`
        Listen TCP port; 0 means OS-assigned
    - **tls-cert** `string`
      Path to TLS certificate file for gRPC
    - **tls-key** `string`
      Path to TLS private key file for gRPC
  - **rest** `container`
    REST and HTTP transport for the API server.
    - **cors-origin** `string`
      CORS allowed origin (empty disables CORS headers)
    - **enabled** `boolean`
      Enable REST API server
    - **server <name>** `list`
      REST API listen endpoints
      - **ip** `ip-address`
        Listen IP address
      - **port** `listener-port`
        Listen TCP port; 0 means OS-assigned
  - **token** `string`
    Bearer token for API authentication.
- **bgp** `container`
  BGP protocol environment settings, applied before any peer-level config.
  - **announce-delay** `string`
    Delay between reactor Ready and first UPDATE (duration, 0s-1h)
  - **openwait** `int32`
    Seconds to wait for peer OPEN after TCP connect
- **bmp** `container`
  BMP receiver settings
  - **enabled** `boolean`
    Enable BMP receiver
  - **max-sessions** `uint16`
    Maximum concurrent BMP sessions
  - **route-action** `enumeration`
    Action for received BMP Route Monitoring messages
  - **server <name>** `list`
    BMP receiver listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
- **chaos** `container`
  Chaos fault injection settings
  - **rate** `string`
    Fault probability per operation (0.0-1.0)
  - **seed** `int64`
    PRNG seed (0 = disabled)
- **cli** `container`
  CLI session settings
  - **format** `container`
    Default output format settings
    - **default** `enumeration`
      Default output format when no pipe operator is specified
  - **transcript** `enumeration`
    CLI session transcript recording to $XDG_DATA_HOME/ze/transcripts/
- **daemon** `container`
  Daemon settings
  - **pid** `string`
    PID file path
  - **user** `string`
    System user for privilege drop
- **exabgp** `container`
  ExaBGP compatibility settings for the migration bridge
  - **api** `container`
    ExaBGP bridge API settings
    - **ack** `boolean`
      Emit done/error ack lines on plugin stdin after each dispatched command
- **gnmi** `container`
  Settings of the gNMI server.
  - **enabled** `boolean`
    Start the gNMI server.
  - **server <name>** `list`
    Listen endpoints of the gNMI server.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **tls** `container`
    TLS certificate of the gNMI gRPC transport.
    - **cert** `string`
      Path to the PEM-encoded certificate file.
    - **key** `string`
      Path to the PEM-encoded private-key file.
  - **token** `string`
    Bearer token that authenticates a gNMI client.
- **hide-version** `boolean`
  Hide the X-Ze-Version banner on every HTTP response (ze.hide-version)
- **l2tp** `container`
  L2TP listener endpoints
  - **server <name>** `list`
    L2TP server listen endpoints (UDP)
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
- **log** `container`
  Logging settings
  - **backend** `enumeration`
    Log output backend (default: stderr)
  - **color** `boolean`
    Force color log output on (true) or off (false). When unset, auto-detected from terminal.
  - **destination** `string`
    Syslog address used when backend is syslog: empty, a socket path, or host:port
  - **level** `string`
    Base log level for all subsystems: disabled, debug, info, warn, or err (case-insensitive)
  - **relay** `string`
    Plugin stderr relay level: disabled, debug, info, warn, or err (case-insensitive)
- **looking-glass** `container`
  Looking glass HTTP server settings.
  - **certificate** `string`
    PKI store certificate served on the looking-glass listener (ze.looking-glass.certificate).
  - **enabled** `boolean`
    Enable the looking glass.
  - **server <name>** `list`
    Looking glass listen endpoints.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **tls** `boolean`
    Enable TLS. Override with the ze.looking-glass.tls variable.
  - **token** `string`
    Bearer token checked on every /api/ and /lg/ route (ze.looking-glass.token).
- **mcp** `container`
  Settings of the MCP server.
  - **auth-mode** `enumeration`
    Authentication strategy for the MCP server.
  - **bind-remote** `boolean`
    Allow the MCP server to bind to a non-loopback address.
  - **enabled** `boolean`
    Start the MCP server.
  - **identity <name>** `list`
    Per-identity bearer entries, for auth-mode=bearer-list.
    - **scope** `string[]`
      Scopes Ze carries on a request this identity authenticates.
    - **token** `string`
      Bearer token value of this identity.
  - **oauth** `container`
    OAuth 2.1 resource-server settings, for auth-mode=oauth.
    - **audience** `string`
      Canonical URL that identifies this MCP endpoint (RFC 8707).
    - **authorization-server** `string`
      HTTPS URL of the authorization server.
    - **required-scopes** `string[]`
      Scopes an accepted token MUST carry.
  - **server <name>** `list`
    Listen endpoints of the MCP server.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **tls** `container`
    TLS certificate that serves the MCP endpoint over HTTPS.
    - **cert** `string`
      Path to PEM-encoded certificate file (or a chain).
    - **key** `string`
      Path to PEM-encoded private-key file.
  - **token** `string`
    Bearer token for auth-mode=bearer.
- **ntp** `container`
  Settings of the NTP client.
  - **enabled** `boolean`
    Synchronize the system clock with an NTP server.
  - **interval** `uint32`
    Seconds between two clock synchronizations.
  - **max-step** `uint32`
    Largest clock step in seconds that Ze accepts.
  - **persist-path** `string`
    Path of the file that holds the time across a restart.
  - **server <name>** `list`
    The NTP servers Ze queries.
    - **address** `string`
      Hostname or IP address of the NTP server.
  - **slew-threshold** `uint32`
    Maximum offset in milliseconds for a gradual clock slew.
- **pprof** `string`
  Address of the pprof HTTP server, which must be a loopback address
- **reactor** `container`
  BGP reactor engine tuning.
  - **cache-max** `uint32`
    Maximum number of cached UPDATE wire messages.
  - **cache-ttl** `uint32`
    Seconds Ze keeps a built UPDATE wire message in the encoding cache.
  - **forward-batch-limit** `uint32`
    Max items per drain batch. Bounds writeMu hold time during forward dispatch. 0 means unlimited.
  - **forward-pool-headroom** `uint32`
    Extra bytes beyond the auto-sized pool baseline.
  - **forward-pool-max-bytes** `uint32`
    Combined byte budget for the 4K and 64K buffer pools, in bytes.
  - **forward-queue-size** `uint32`
    Per-destination forward channel capacity.
  - **forward-teardown-grace** `string`
    Grace period before forced teardown on congestion (duration string, e.g. 5s, 1m).
  - **read-buffer-size** `uint32`
    Per-session TCP read buffer size in bytes.
  - **speed** `string`
    Reactor loop cycle time multiplier, from 0.1 to 10.0.
  - **update-groups** `boolean`
    Build each UPDATE once for every peer with the same encoding context.
  - **write-buffer-size** `uint32`
    Per-session TCP write buffer size in bytes.
- **ssh** `container`
  SSH server settings.
  - **enabled** `boolean`
    Enable the SSH server.
  - **host-certificate** `string`
    Path to the SSH host certificate file, signed by a CA.
  - **host-key** `string`
    Path to the SSH host key file.
  - **idle-timeout** `uint32`
    Idle timeout in seconds. The default is 600.
  - **max-sessions** `uint16`
    Maximum number of concurrent SSH sessions.
  - **server <name>** `list`
    SSH server listen endpoints.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
- **web** `container`
  Web interface settings.
  - **certificate** `string`
    Name of the PKI store certificate served on the HTTPS listener (ze.web.certificate).
  - **enabled** `boolean`
    Enable the web interface.
  - **insecure** `boolean`
    Disable authentication, which forces the host to 127.0.0.1.
  - **server <name>** `list`
    Web server listen endpoints.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **ui-mode** `enumeration`
    Web UI mode.

## exabgp

*Provided by `exabgp-bridge` ([ze-exabgp-bridge-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/exabgp/bridgeplugin/yang/ze-exabgp-bridge-conf.yang))*

Top-level container for ExaBGP-compatibility configuration.

- **bridge** `container`
  In-process ExaBGP bridge settings.
  - **add-path** `enumeration`
    ADD-PATH mode for the families the bridge negotiates.
  - **family** `address-family[]`
    Address families the bridge negotiates for the script.
  - **process <name>** `list`
    One ExaBGP-format script the bridge runs as a subprocess.
    - **encoder** `enumeration`
      Format the script's events are written in.
    - **feed <peer>** `list`
      One peer that feeds this script, and the events it feeds.
      - **event** `string[]`
        Events this peer feeds this script.
    - **respawn** `boolean`
      Start the script again when it exits.
    - **run** `string`
      Command line of the script.
  - **route-refresh** `boolean`
    Advertise the BGP route-refresh capability.

## fib

Forwarding Information Base configuration.

- **kernel** `container`
  *Provided by `fib-kernel` ([ze-fib-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/yang/ze-fib-conf.yang))*
  OS kernel route programming via netlink (Linux).
  - **flush-on-stop** `boolean`
    Remove every ze-installed route when the plugin stops.
  - **sweep-delay** `uint16`
    Time to wait after startup before Ze sweeps stale routes.
- **p4** `container`
  *Provided by `fib-p4` ([ze-fib-p4-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/p4/yang/ze-fib-p4-conf.yang))*
  P4 switch route programming via gRPC/P4Runtime.
  - **device-id** `uint64`
    P4Runtime device ID.
  - **flush-on-stop** `boolean`
    Remove all forwarding entries when the plugin stops.
  - **target** `string`
    P4Runtime gRPC target address (host:port). Example: 127.0.0.1:9559
- **vpp** `container`
  *Provided by `fib-vpp` ([ze-fib-vpp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/vpp/yang/ze-fib-vpp-conf.yang))*
  VPP FIB programming via GoVPP binary API.
  - **batch-interval-ms** `uint16`
    Maximum time in milliseconds before dispatching a partial batch.
  - **batch-size** `uint16`
    Maximum number of routes per batch dispatch.
  - **enabled** `boolean`
    Enable VPP FIB programming. Routes from system RIB are programmed directly into VPP's FIB.
  - **table-id** `uint32`
    VRF table ID for route programming. 0 is the default VRF.

## firewall

*Provided by `firewall` ([ze-firewall-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/firewall/yang/ze-firewall-cmd.yang), [ze-firewall-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/firewall/yang/ze-firewall-conf.yang)); `firewall-domain` ([ze-firewall-domain-group-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/firewall/plugins/domain/yang/ze-firewall-domain-group-cmd.yang), [ze-firewall-domain-group.yang](https://github.com/ze-software/ze/blob/main/internal/component/firewall/plugins/domain/yang/ze-firewall-domain-group.yang)); `firewall-irr` ([ze-firewall-irr-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang), [ze-firewall-irr.yang](https://github.com/ze-software/ze/blob/main/internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang))*

Ze-managed nftables firewall tables.

- **backend** `string`
  Firewall backend implementation, nft by default.
- **domain-group <name>** `list`
  Address group whose members are what its DNS names resolve to.
  - **domain-names** `string[]`
    DNS names whose addresses are the members of this group.
  - **ttl-floor** `uint32`
    Shortest interval Ze waits before it resolves a name again.
- **flush-on-shutdown** `boolean`
  Remove ze-owned nftables tables when the ze process stops in an orderly way.
- **global-options** `container`
  Network security defaults mapped to kernel sysctls.
  - **all-ping** `enumeration`
    Control ICMP echo (ping) responses.
  - **broadcast-ping** `enumeration`
    Control broadcast ICMP echo responses.
  - **ipv6-receive-redirects** `enumeration`
    Control acceptance of IPv6 ICMP redirect messages.
  - **ipv6-src-route** `enumeration`
    Control acceptance of IPv6 packets with source routing headers.
  - **log-martians** `enumeration`
    Log packets with impossible source addresses.
  - **receive-redirects** `enumeration`
    Control acceptance of IPv4 ICMP redirect messages.
  - **send-redirects** `enumeration`
    Control sending of IPv4 ICMP redirect messages.
  - **source-validation** `enumeration`
    Reverse path filtering mode for source address validation.
  - **syn-cookies** `enumeration`
    TCP SYN cookie protection against SYN flood attacks.
- **irr** `container`
  IRR policy settings for firewall prefix-list resolution.
  - **interface <name>** `list`
    Per-interface AS-SET binding for ingress source validation.
    - **source-as-set** `string`
      AS-SET whose IRR-resolved prefixes are the allowed source addresses.
  - **peeringdb-url** `string`
    Base URL for PeeringDB API queries. Override for testing with a mock server.
  - **refresh-interval** `uint32`
    Seconds between automatic IRR re-queries.
  - **server** `string`
    IRR whois server hostname or host:port.
- **table <name>** `list`
  Named firewall table
  - **chain <name>** `list`
    Named chain within table
    - **hook** `chain-hook`
      Netfilter hook point (base chains only)
    - **policy** `chain-policy`
      Default policy (base chains only)
    - **priority** `int32`
      Chain priority (base chains only)
    - **term <name>** `list`
      Named rule (from/then structure)
      - **from** `container`
        Match criteria
        - **connection-mark** `mark-value`
          Connection mark value with an optional mask.
        - **connection-state** `connection-state`
          Connection tracking state(s) the rule matches.
        - **destination-address** `ip-prefix`
          Destination IP prefix or @set-name
        - **destination-as-set** `string`
          Match the destination address against the IRR-resolved prefixes of this AS-SET.
        - **destination-asn** `asn`
          Match the destination address against the IRR-resolved prefixes of this ASN.
        - **destination-domain-group** `string`
          Match the destination address against the addresses of this domain group.
        - **destination-port** `port-spec`
          Destination port, range, or list
        - **dscp** `dscp-value`
          DSCP value
        - **icmp-type** `string`
          ICMPv4 type, as an nftables symbolic name or a byte value 0..255.
        - **icmpv6-type** `string`
          ICMPv6 type, as an nftables symbolic name or a byte value 0..255.
        - **input-interface** `string`
          Input interface name, with a trailing asterisk for a prefix match.
        - **mark** `mark-value`
          Packet mark value/mask
        - **output-interface** `string`
          Output interface name, with a trailing asterisk for a prefix match.
        - **protocol** `protocol-name`
          L4 protocol
        - **source-address** `ip-prefix`
          Source IP prefix or @set-name
        - **source-as-set** `string`
          Match the source address against the IRR-resolved prefixes of this AS-SET.
        - **source-asn** `asn`
          Match the source address against the IRR-resolved prefixes of this ASN.
        - **source-domain-group** `string`
          Match the source address against the addresses of this domain group.
        - **source-port** `port-spec`
          Source port, range, or list
      - **then** `container`
        Actions and modifiers
        - **accept** `container`
          Accept the packet
        - **connection-mark-set** `container`
          Write the conntrack mark of the packet's flow.
          - **value** `mark-value`
            Mark value with optional mask
        - **counter** `container`
          Anonymous per-rule counter.
          - **name** `string`
            Counter name, reserved for a future named-counter feature.
        - **dnat** `container`
          Rewrite the destination address of a packet arriving at this router.
          - **to** `nat-spec`
            NAT target address:port
        - **drop** `container`
          Drop the packet
        - **dscp-set** `dscp-value`
          Set DSCP field value
        - **exclude** `container`
          In a NAT chain term, skip NAT for the matching traffic.
        - **flow-offload** `container`
          Hardware or software flow offload through an nftables flowtable.
          - **flowtable** `string`
            Flowtable name declared in this table.
        - **goto** `string`
          Goto target chain (no return)
        - **jump** `string`
          Jump to target chain
        - **limit-rate** `container`
          Match only while the packet rate stays under a stated limit.
          - **burst** `uint32`
            Burst size
          - **rate** `rate-spec`
            Rate limit, as a count and a time unit, such as 10/second.
        - **log** `container`
          Record the packet and go on evaluating the term.
          - **level** `uint32`
            Log level (syslog severity)
          - **prefix** `string`
            Log prefix string
        - **mark-set** `container`
          Write the kernel skb mark.
          - **value** `mark-value`
            Mark value with optional mask
        - **masquerade** `container`
          Masquerade, which is source NAT with the outgoing interface address.
          - **persistent** `empty`
            Program the masquerade with the NF_NAT_RANGE_PERSISTENT flag.
          - **port-range** `string`
            Source port or port range for the translated source, such as 1024-65535.
          - **random** `container`
            Randomize the translated source port.
            - **full** `empty`
              Use full randomization
        - **notrack** `container`
          Disable connection tracking for the packet.
        - **redirect** `container`
          Send the packet to a port on this router instead of its destination.
          - **to** `port`
            Local port number
        - **reject** `container`
          Drop the packet and answer the sender with an ICMP or TCP reset.
          - **code** `uint8`
            ICMP code
          - **with** `reject-type`
            Reject response type
        - **return** `container`
          Return to caller chain
        - **snat** `container`
          Rewrite the source address of a packet leaving this router.
          - **to** `nat-spec`
            NAT target address:port
        - **tcp-mss-set** `uint16`
          Clamp TCP Maximum Segment Size option to the given value
    - **type** `chain-type`
      Chain type (base chains only)
  - **family** `table-family`
    Address family
  - **flowtable <name>** `list`
    Hardware offload flowtable, which is nftables-specific.
    - **device** `string[]`
      Devices for offload
    - **hook** `chain-hook`
      Hook point, which MUST be ingress.
    - **priority** `int32`
      Flowtable priority at the ingress hook.
  - **set <name>** `list`
    Named set
    - **element <value>** `list`
      Static set elements, whose value form follows the set type.
      - **timeout** `uint32`
        Per-element timeout in seconds, where 0 means no timeout.
    - **flags-constant** `container`
      Immutable after creation
    - **flags-dynamic** `container`
      Dynamically populated
    - **flags-interval** `container`
      Enable interval ranges
    - **flags-timeout** `container`
      Enable per-element timeouts
    - **type** `set-type`
      Element data type

## flow-export

*Provided by `flow-export` ([ze-flowexport-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/yang/ze-flowexport-conf.yang))*

Flow export (sFlow, NetFlow v9, IPFIX) configuration

- **collector <name>** `list`
  A flow export collector endpoint
  - **address** `ip-address`
    Collector IP address
  - **agent-address** `ip-address`
    sFlow agent address (device's own stable IP, e.g. loopback)
  - **observation-domain** `uint32`
    IPFIX/NetFlow v9 observation domain ID
  - **polling-interval** `uint16`
    Counter polling interval in seconds
  - **port** `uint16`
    Collector UDP port
  - **protocol** `enumeration`
    Export protocol
  - **source-address** `ip-address`
    Source IP address for outbound flow export datagrams
  - **sub-agent-id** `uint32`
    sFlow sub-agent identifier
  - **template-refresh** `uint32`
    Template refresh interval in seconds (NetFlow v9, IPFIX)
- **conntrack** `container`
  Per-flow record export from conntrack (NetFlow v9, IPFIX)
  - **active-timeout** `uint16`
    Seconds between conntrack table dumps
  - **enabled** `boolean`
    Export per-flow records from the conntrack table
  - **recent-flow-ring** `uint32`
    Capacity of the recent-flow ring, in flow records.
- **enrichment** `container`
  Flow record enrichment from the BGP RIB
  - **bgp** `boolean`
    Enrich flow records with BGP next-hop (and AS where available)
- **sampling** `container`
  Packet sampling (tc sample + psample) exported as sFlow flow samples
  - **interface <name>** `list`
    Per-interface packet sampling configuration
    - **group** `uint32`
      psample group ID
    - **rate** `uint32`
      Sampling rate (1-in-N packets)
    - **trunc-size** `uint16`
      Bytes of each sampled packet header to capture

## interface

*Provided by `interface` ([ze-iface-api.yang](https://github.com/ze-software/ze/blob/main/internal/component/iface/yang/ze-iface-api.yang), [ze-iface-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/iface/yang/ze-iface-cmd.yang), [ze-iface-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/iface/yang/ze-iface-conf.yang), [ze-iface-interface-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/iface/yang/ze-iface-interface-cmd.yang), [ze-iface-monitor-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/iface/yang/ze-iface-monitor-cmd.yang), [ze-iface-show-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/component/iface/yang/ze-iface-show-cmd.yang)); `vrrp` ([ze-vrrp-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/yang/ze-vrrp-cmd.yang), [ze-vrrp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/yang/ze-vrrp-conf.yang))*

Interface-level class-of-service bindings and inline QoS maps.

- **backend** `string`
  Interface management backend: netlink or vpp.
- **bridge <name>** `list`
  Linux bridge interface, which forwards Ethernet frames between member ports at L2.
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none'.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings for this interface.
    - **address** `string`
      Override the kernel-assigned MAC with a colon-separated hex value.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC.
  - **member** `string[]`
    Member port interface names
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features.
    - **gro** `boolean`
      Generic Receive Offload: aggregate small incoming packets before the network stack.
    - **gso** `boolean`
      Generic Segmentation Offload: delay segmentation until the NIC driver transmit path.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload: run TC filter rules in the NIC.
    - **lro** `boolean`
      Large Receive Offload: the NIC coalesces incoming TCP segments before DMA.
    - **rfs** `boolean`
      Receive Flow Steering: steer a flow to the CPU that runs its application.
    - **rps** `boolean`
      Receive Packet Steering: spread incoming packets across CPUs in software.
    - **sg** `boolean`
      Scatter-Gather I/O: build one frame from several memory buffers.
    - **tso** `boolean`
      TCP Segmentation Offload: the NIC splits large TCP segments into wire frames.
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **stp** `boolean`
    Enable Spanning Tree Protocol
  - **unit <name>** `list`
    Logical interface unit.
    - **class-of-service** `string`
      Class-of-service profile of this unit.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of an outgoing packet to the 802.1p PCP value.
      - **pcp** `uint8`
        PCP value Ze stamps in the 802.1Q header.
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of a received tagged frame to an internal priority.
      - **priority** `uint8`
        Internal priority Ze gives to a matching frame.
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Sections 6.1 and 6.4.3).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **version** `enumeration`
            Protocol version.
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **dhcp-auto** `boolean`
  Auto-discover the first ethernet interface and run DHCP on it.
- **dummy <name>** `list`
  Dummy interface, which behaves like a loopback.
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none'.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings for this interface.
    - **address** `string`
      Override the kernel-assigned MAC with a colon-separated hex value.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC.
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features.
    - **gro** `boolean`
      Generic Receive Offload: aggregate small incoming packets before the network stack.
    - **gso** `boolean`
      Generic Segmentation Offload: delay segmentation until the NIC driver transmit path.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload: run TC filter rules in the NIC.
    - **lro** `boolean`
      Large Receive Offload: the NIC coalesces incoming TCP segments before DMA.
    - **rfs** `boolean`
      Receive Flow Steering: steer a flow to the CPU that runs its application.
    - **rps** `boolean`
      Receive Packet Steering: spread incoming packets across CPUs in software.
    - **sg** `boolean`
      Scatter-Gather I/O: build one frame from several memory buffers.
    - **tso** `boolean`
      TCP Segmentation Offload: the NIC splits large TCP segments into wire frames.
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **unit <name>** `list`
    Logical interface unit.
    - **class-of-service** `string`
      Class-of-service profile of this unit.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of an outgoing packet to the 802.1p PCP value.
      - **pcp** `uint8`
        PCP value Ze stamps in the 802.1Q header.
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of a received tagged frame to an internal priority.
      - **priority** `uint8`
        Internal priority Ze gives to a matching frame.
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Sections 6.1 and 6.4.3).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **version** `enumeration`
            Protocol version.
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **ethernet <name>** `list`
  Physical or virtual Ethernet interface.
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none'.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings for this interface.
    - **address** `string`
      Override the kernel-assigned MAC with a colon-separated hex value.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC.
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features.
    - **gro** `boolean`
      Generic Receive Offload: aggregate small incoming packets before the network stack.
    - **gso** `boolean`
      Generic Segmentation Offload: delay segmentation until the NIC driver transmit path.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload: run TC filter rules in the NIC.
    - **lro** `boolean`
      Large Receive Offload: the NIC coalesces incoming TCP segments before DMA.
    - **rfs** `boolean`
      Receive Flow Steering: steer a flow to the CPU that runs its application.
    - **rps** `boolean`
      Receive Packet Steering: spread incoming packets across CPUs in software.
    - **sg** `boolean`
      Scatter-Gather I/O: build one frame from several memory buffers.
    - **tso** `boolean`
      TCP Segmentation Offload: the NIC splits large TCP segments into wire frames.
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **unit <name>** `list`
    Logical interface unit.
    - **class-of-service** `string`
      Class-of-service profile of this unit.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of an outgoing packet to the 802.1p PCP value.
      - **pcp** `uint8`
        PCP value Ze stamps in the 802.1Q header.
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of a received tagged frame to an internal priority.
      - **priority** `uint8`
        Internal priority Ze gives to a matching frame.
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Sections 6.1 and 6.4.3).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **version** `enumeration`
            Protocol version.
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **loopback** `container`
  The system loopback interface (lo).
  - **unit <name>** `list`
    Logical interface unit
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **monitor** `container`
  Interface monitoring settings
  - **loopback** `boolean`
    Monitor loopback interface
- **pppoe-client <name>** `list`
  PPPoE client interface (RFC 2516).
  - **ac-name** `string`
    Desired access concentrator name.
  - **authentication** `container`
    PPPoE authentication credentials (PAP or CHAP)
    - **password** `string`
      Authentication password. Stored $9$-encoded on disk via the standard ze sensitive-leaf pattern.
    - **username** `string`
      Authentication username sent to the AC
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mtu** `uint16`
    Maximum transmission unit
  - **no-default-route** `empty`
    Do not install a default route through the PPP interface.
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **route-priority** `uint32`
    Route metric of the default route installed after IPCP completes.
  - **service-name** `string`
    Desired PPPoE service name. Empty or absent means accept any service (RFC 2516 Section 5.1).
  - **source-interface** `string`
    Physical Ethernet interface for PPPoE discovery (e.g. eth2). Must exist and be admin-up.
- **tunnel <name>** `list`
  Tunnel interface, in the GRE, GRETAP, IPIP, SIT or IP6TNL family.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **encapsulation** `container`
    Tunnel encapsulation kind and per-kind parameters
    - **kind** `choice`
      Encapsulation kind of this tunnel.
      - **gre** `case`
        GRE over IPv4 (RFC 2784, key extension RFC 2890). L3.
        - **gre** `container`
          GRE-over-IPv4 parameters
          - **key** `uint32`
            32-bit GRE key (RFC 2890). Symmetric: same value used for input and output
          - **local** `container`
            Local IPv4 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv4-address`
              Local IPv4 endpoint
          - **no-pmtu-discovery** `empty`
            Disable Path MTU Discovery on the outer header
          - **remote** `container`
            Remote IPv4 endpoint
            - **ip** `ipv4-address`
              Remote IPv4 endpoint
          - **tos** `uint8`
            Outer-header Type of Service. 0 = inherit
          - **ttl** `uint8`
            Outer-header TTL. 0 = inherit from the inner packet
      - **gretap** `case`
        GRE over IPv4, L2 (Ethernet over GRE, bridgeable). RFC 2784.
        - **gretap** `container`
          GRETAP-over-IPv4 parameters
          - **key** `uint32`
            32-bit GRE key (RFC 2890)
          - **local** `container`
            Local IPv4 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv4-address`
              Local IPv4 endpoint
          - **mac** `container`
            Hardware MAC settings (L2 tunnel kinds only)
            - **address** `string`
              Hardware MAC address (L2 tunnel kinds only)
          - **no-pmtu-discovery** `empty`
            Disable Path MTU Discovery on the outer header
          - **remote** `container`
            Remote IPv4 endpoint
            - **ip** `ipv4-address`
              Remote IPv4 endpoint
          - **tos** `uint8`
            Outer-header Type of Service. 0 = inherit
          - **ttl** `uint8`
            Outer-header TTL. 0 = inherit from the inner packet
      - **ip6gre** `case`
        GRE over IPv6 (RFC 2784, key extension RFC 2890). L3.
        - **ip6gre** `container`
          GRE-over-IPv6 parameters
          - **hoplimit** `uint8`
            Outer IPv6 hop limit (RFC 2473 Section 6.3)
          - **key** `uint32`
            32-bit GRE key (RFC 2890)
          - **local** `container`
            Local IPv6 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv6-address`
              Local IPv6 endpoint
          - **remote** `container`
            Remote IPv6 endpoint
            - **ip** `ipv6-address`
              Remote IPv6 endpoint
          - **tclass** `uint8`
            Outer IPv6 traffic class (RFC 2473 Section 6.4)
      - **ip6gretap** `case`
        GRE over IPv6, L2 (Ethernet over GRE bridgeable). RFC 2784.
        - **ip6gretap** `container`
          GRETAP-over-IPv6 parameters
          - **hoplimit** `uint8`
            Outer IPv6 hop limit
          - **key** `uint32`
            32-bit GRE key (RFC 2890)
          - **local** `container`
            Local IPv6 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv6-address`
              Local IPv6 endpoint
          - **mac** `container`
            Hardware MAC settings (L2 tunnel kinds only)
            - **address** `string`
              Hardware MAC address (L2 tunnel kinds only)
          - **remote** `container`
            Remote IPv6 endpoint
            - **ip** `ipv6-address`
              Remote IPv6 endpoint
          - **tclass** `uint8`
            Outer IPv6 traffic class
      - **ip6tnl** `case`
        IPv6 in IPv6 (RFC 2473). Also covers ip6ip6. L3.
        - **ip6tnl** `container`
          IPv6-in-IPv6 tunnel parameters (RFC 2473). Also handles ip6ip6 encapsulation.
          - **encaplimit** `uint8`
            Tunnel encapsulation limit (RFC 2473 Section 4.1.1)
          - **hoplimit** `uint8`
            Outer IPv6 hop limit (RFC 2473 Section 6.3)
          - **local** `container`
            Local IPv6 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv6-address`
              Local IPv6 endpoint
          - **remote** `container`
            Remote IPv6 endpoint
            - **ip** `ipv6-address`
              Remote IPv6 endpoint
          - **tclass** `uint8`
            Outer IPv6 traffic class (RFC 2473 Section 6.4)
      - **ipip** `case`
        IPv4 in IPv4 (RFC 2003). No GRE header, no key. L3.
        - **ipip** `container`
          IPv4-in-IPv4 tunnel parameters (RFC 2003). Minimal overhead, no key support.
          - **local** `container`
            Local IPv4 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv4-address`
              Local IPv4 endpoint
          - **no-pmtu-discovery** `empty`
            Disable Path MTU Discovery on the outer header
          - **remote** `container`
            Remote IPv4 endpoint
            - **ip** `ipv4-address`
              Remote IPv4 endpoint
          - **tos** `uint8`
            Outer-header Type of Service. 0 = inherit
          - **ttl** `uint8`
            Outer-header TTL. 0 = inherit from the inner packet
      - **ipip6** `case`
        IPv4 in IPv6 (RFC 2473 with Next Header = 4). L3.
        - **ipip6** `container`
          IPv4-in-IPv6 tunnel parameters (RFC 2473, Next Header = 4). Uses the ip6tnl kernel kind.
          - **encaplimit** `uint8`
            Tunnel encapsulation limit
          - **hoplimit** `uint8`
            Outer IPv6 hop limit
          - **local** `container`
            Local IPv6 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv6-address`
              Local IPv6 endpoint
          - **remote** `container`
            Remote IPv6 endpoint
            - **ip** `ipv6-address`
              Remote IPv6 endpoint
          - **tclass** `uint8`
            Outer IPv6 traffic class
      - **sit** `case`
        IPv6 in IPv4 (6in4, RFC 4213 Section 3). L3.
        - **sit** `container`
          SIT (6in4) parameters
          - **local** `container`
            Local IPv4 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv4-address`
              Local IPv4 endpoint
          - **no-pmtu-discovery** `empty`
            Disable Path MTU Discovery on the outer header
          - **remote** `container`
            Remote IPv4 endpoint
            - **ip** `ipv4-address`
              Remote IPv4 endpoint
          - **tos** `uint8`
            Outer-header Type of Service. 0 = inherit
          - **ttl** `uint8`
            Outer-header TTL. 0 = inherit from the inner packet
      - **vxlan** `case`
        VXLAN overlay: L2 Ethernet frames over a UDP/IPv4 underlay, keyed by a 24-bit VNI.
        - **vxlan** `container`
          VXLAN parameters. local ip is the tunnel source, remote ip the (unicast) VTEP destination.
          - **local** `container`
            Local IPv4 endpoint or source interface (one of ip or interface)
            - **interface** `string`
              Local interface to take the source address from
            - **ip** `ipv4-address`
              Local IPv4 endpoint
          - **port** `port`
            UDP destination port (IANA-assigned default 4789)
          - **remote** `container`
            Remote IPv4 endpoint
            - **ip** `ipv4-address`
              Remote IPv4 endpoint
          - **vni** `uint32`
            VXLAN Network Identifier (24-bit, 1..16777215)
  - **mtu** `uint16`
    Maximum transmission unit
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **unit <name>** `list`
    Logical interface unit
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **veth <name>** `list`
  Virtual ethernet pair interface
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none'.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings for this interface.
    - **address** `string`
      Override the kernel-assigned MAC with a colon-separated hex value.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC.
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features.
    - **gro** `boolean`
      Generic Receive Offload: aggregate small incoming packets before the network stack.
    - **gso** `boolean`
      Generic Segmentation Offload: delay segmentation until the NIC driver transmit path.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload: run TC filter rules in the NIC.
    - **lro** `boolean`
      Large Receive Offload: the NIC coalesces incoming TCP segments before DMA.
    - **rfs** `boolean`
      Receive Flow Steering: steer a flow to the CPU that runs its application.
    - **rps** `boolean`
      Receive Packet Steering: spread incoming packets across CPUs in software.
    - **sg** `boolean`
      Scatter-Gather I/O: build one frame from several memory buffers.
    - **tso** `boolean`
      TCP Segmentation Offload: the NIC splits large TCP segments into wire frames.
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **peer** `string`
    Veth peer name
  - **unit <name>** `list`
    Logical interface unit.
    - **class-of-service** `string`
      Class-of-service profile of this unit.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of an outgoing packet to the 802.1p PCP value.
      - **pcp** `uint8`
        PCP value Ze stamps in the 802.1Q header.
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of a received tagged frame to an internal priority.
      - **priority** `uint8`
        Internal priority Ze gives to a matching frame.
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Sections 6.1 and 6.4.3).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **version** `enumeration`
            Protocol version.
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds.
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time, in Junos semantics.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4).
          - **track** `container`
            Interfaces whose loss lowers the priority this group advertises.
            - **interface <name>** `list`
              One tracked interface and the priority its loss costs.
              - **priority-decrement** `uint8`
                Priority subtracted from this group while this interface is down.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3).
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **wireguard <name>** `list`
  WireGuard interface, configured declaratively.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **fwmark** `uint32`
    Firewall mark applied to outgoing encapsulated packets for policy routing. 0 means unset.
  - **listen-port** `port`
    UDP port WireGuard binds on 0.0.0.0 and :: for handshake and data traffic.
  - **mtu** `uint16`
    Maximum transmission unit
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **peer <name>** `list`
    WireGuard peer, identified by its public key.
    - **allowed-ips** `string[]`
      CIDR prefixes routed into the tunnel for this peer.
    - **disable** `empty`
      Administratively disable this peer.
    - **endpoint** `container`
      Remote UDP endpoint for this peer.
      - **ip** `ip-address`
        Remote IPv4 or IPv6 address (numeric, not a hostname -- DNS resolution is not performed)
      - **port** `port`
        Remote UDP port
    - **persistent-keepalive** `uint16`
      Seconds between unsolicited keepalive packets that maintain NAT state.
    - **preshared-key** `string`
      Optional base64-encoded 32-byte symmetric preshared key.
    - **public-key** `string`
      Base64-encoded 32-byte Curve25519 peer public key
  - **private-key** `string`
    Base64-encoded 32-byte Curve25519 private key.
  - **unit <name>** `list`
    Logical interface unit
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.
- **xfrm <name>** `list`
  XFRM interface, for route-based IPsec (Linux 4.19 and later).
  - **description** `string`
    Interface description
  - **dev** `string`
    Optional parent device that this XFRM interface binds to.
  - **disable** `empty`
    Administratively disable this interface
  - **if-id** `uint32`
    XFRM identifier that binds security associations to this interface.
  - **mtu** `uint16`
    Maximum transmission unit
  - **os-name** `string`
    OS device this logical interface name binds to.
  - **unit <name>** `list`
    Logical interface unit
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only answer ARP requests for addresses on the receiving interface.
      - **arp-ignore** `uint8`
        ARP ignore level, which selects the ARP requests this interface answers.
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier, sent in DISCOVER and REQUEST messages.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces.
      - **proxy-arp** `boolean`
        Answer ARP requests for other hosts that are reachable through this router.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0 disables, 1 accepts when not forwarding, 2 accepts even when forwarding.
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration.
      - **dhcpv6** `container`
        DHCPv6 stateful client (RFC 8415).
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces.
      - **router-advertisement** `container`
        Router Advertisement sender (RFC 4861).
        - **enabled** `boolean`
          Send Router Advertisements on this unit.
        - **hop-limit** `uint8`
          Value a host places in the Hop Limit field of its outgoing packets.
        - **managed** `boolean`
          Set the M flag: hosts get their addresses from DHCPv6 (RFC 4861 Section 4.2).
        - **maximum-interval** `uint16`
          Longest time between unsolicited Router Advertisements (MaxRtrAdvInterval).
        - **minimum-interval** `uint16`
          Shortest time between unsolicited Router Advertisements (MinRtrAdvInterval).
        - **other-config** `boolean`
          Set the O flag: hosts get other configuration, such as DNS, from DHCPv6 (RFC 4861 Section 4.2).
        - **prefix <prefix>** `list`
          Prefixes advertised in Prefix Information options (RFC 4861 Section 4.6.2).
          - **autonomous** `boolean`
            Set the A flag in the Prefix Information option.
          - **on-link** `boolean`
            Set the L flag: hosts treat addresses in this prefix as on-link (RFC 4861 Section 4.6.2).
          - **preferred-lifetime** `uint32`
            How long an address built from the prefix stays preferred.
          - **valid-lifetime** `uint32`
            How long the prefix stays valid (RFC 4861 Section 4.6.2).
        - **rdnss** `container`
          Recursive DNS servers advertised to hosts (RFC 8106 Section 5.1).
          - **lifetime** `uint32`
            How long a host can use these resolvers (RFC 8106 Section 5.1).
          - **server** `ipv6-address[]`
            Resolver addresses. All of them share lifetime.
        - **reachable-time** `uint32`
          How long a host treats a neighbor as reachable after a reachability confirmation.
        - **retransmit-timer** `uint32`
          Time between retransmitted Neighbor Solicitations on this link.
        - **router-lifetime** `uint16`
          How long a host keeps Ze in its default router list (AdvDefaultLifetime).
      - **rpf-check** `enumeration`
        Reverse path filtering mode, on the VPP data plane only for IPv6.
    - **mirror** `container`
      Traffic mirroring for this interface.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface.
    - **route-priority** `uint32`
      Route metric for default routes learned on this unit through DHCP or router advertisement.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Not implemented. Ze refuses a config that sets this leaf.

## isis

*Provided by `isis` ([ze-isis-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/yang/ze-isis-cmd.yang), [ze-isis-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/yang/ze-isis-conf.yang))*

IS-IS routing instance configuration.

- **hostname** `string`
  Dynamic hostname this IS-IS instance advertises (RFC 5301 section 3).
- **interfaces** `container`
  IS-IS-enabled interfaces.
  - **interface <name>** `list`
    Per-interface IS-IS configuration.
    - **address-family <af>** `list`
      Per-interface address families on this circuit (single-topology; both ride the shared SPF tree).
    - **circuit-type** `enumeration`
      Circuit type.
    - **enabled** `boolean`
      IS-IS enabled on this interface.
    - **hello-interval** `uint16`
      Hello interval.
    - **hold-multiplier** `uint8`
      Hold time = hello-interval * hold-multiplier.
    - **level** `enumeration`
      Per-interface level override.
    - **level-1** `container`
      Level-1 per-interface overrides.
      - **auth-key-chain** `string`
        L1 per-interface (IIH) key-chain reference.
      - **hello-interval** `uint16`
        L1 hello-interval override.
      - **hold-multiplier** `uint8`
        L1 hold-multiplier override.
      - **metric** `uint32`
        L1 wide metric override.
      - **priority** `uint8`
        L1 DIS priority override.
    - **level-2** `container`
      Level-2 per-interface overrides.
      - **auth-key-chain** `string`
        L2 per-interface (IIH) key-chain reference.
      - **hello-interval** `uint16`
        L2 hello-interval override.
      - **hold-multiplier** `uint8`
        L2 hold-multiplier override.
      - **metric** `uint32`
        L2 wide metric override.
      - **priority** `uint8`
        L2 DIS priority override.
    - **metric** `uint32`
      Wide metric (RFC 5305).
    - **passive** `boolean`
      Advertise the interface but form no adjacencies.
    - **priority** `uint8`
      DIS election priority (broadcast circuits).
- **key-chains <name>** `list`
  Named authentication key chains for hitless key rotation.
  - **key <key-id>** `list`
    Keys in this chain.
    - **accept-lifetime** `container`
      When this key is accepted on receive (hitless rotation).
      - **end** `string`
        RFC3339 end timestamp.
      - **start** `string`
        RFC3339 start timestamp.
    - **algorithm** `enumeration`
      Authentication algorithm.
    - **secret** `string`
      Shared secret, masked and $9$-encoded at rest.
    - **send-lifetime** `container`
      When this key may be used to sign (hitless rotation).
      - **end** `string`
        RFC3339 end timestamp.
      - **start** `string`
        RFC3339 start timestamp.
- **level** `enumeration`
  Routing level of this Intermediate System.
- **level-1** `container`
  Level-1 (area) per-level configuration.
  - **auth-key-chain** `string`
    L1 per-level (LSP/SNP, area key) key-chain reference.
- **level-2** `container`
  Level-2 (domain) per-level configuration.
  - **auth-key-chain** `string`
    L2 per-level (LSP/SNP, domain key) key-chain reference.
- **lsp-lifetime** `uint16`
  Maximum LSP remaining lifetime.
- **lsp-refresh-interval** `uint16`
  LSP refresh interval.
- **net** `string[]`
  Network Entity Title(s) of this IS-IS instance, at least one.
- **overload** `boolean`
  Set the overload bit (RFC 3787).
- **system-id** `string`
  6-byte System ID (xxxx.xxxx.xxxx); derived from NET if unset.

## kernel

*Provided by `kernel` ([ze-kernel-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/kernel/yang/ze-kernel-conf.yang))*

Kernel route redistribution. Presence enables the plugin.


## l2tp

*Provided by `l2tp-auth-local` ([ze-l2tp-auth-local-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/yang/ze-l2tp-auth-local-conf.yang)); `l2tp-auth-radius` ([ze-l2tp-auth-radius-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang)); `l2tp-pool` ([ze-l2tp-pool-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang)); `l2tp-shaper` ([ze-l2tp-shaper-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang))*

L2TPv2 tunnel subsystem settings (RFC 2661).

- **allow-no-auth** `boolean`
  Allow PPP LCP to finish with no Auth-Protocol, false by default.
- **auth** `container`
  PPP authentication for L2TP subscriber sessions.
  - **local** `container`
    Authenticate a subscriber against users declared in this config.
    - **user <name>** `list`
      One subscriber account, keyed by its PPP username.
      - **password** `string`
        Shared secret for PAP cleartext and CHAP-MD5/MS-CHAPv2 challenge-response.
  - **radius** `container`
    Authenticate and account subscriber sessions against a RADIUS server.
    - **acct-interval** `uint16`
      Accounting interim-update interval in seconds.
    - **attributes** `container`
      Which optional RADIUS attributes Ze puts in the packets it sends.
      - **exclude** `container`
        Attributes Ze does not send, each with the record types it is held back from.
        - **acct-delay-time** `container`
          Do not send Acct-Delay-Time (attribute 41).
          - **packet-type** `enumeration[]`
            Record types to hold the attribute back from. Name none for all of them.
        - **acct-terminate-cause** `container`
          Do not send Acct-Terminate-Cause (attribute 49).
          - **packet-type** `enumeration[]`
            Record types to hold the attribute back from. Name none for all of them.
        - **calling-station-id** `container`
          Do not send Calling-Station-Id (attribute 31).
          - **packet-type** `enumeration[]`
            Record types to hold the attribute back from. Name none for all of them.
        - **event-timestamp** `container`
          Do not send Event-Timestamp (attribute 55).
          - **packet-type** `enumeration[]`
            Record types to hold the attribute back from. Name none for all of them.
        - **framed-ip-address** `container`
          Do not send Framed-IP-Address (attribute 8).
          - **packet-type** `enumeration[]`
            Record types to hold the attribute back from. Name none for all of them.
        - **nas-port-id** `container`
          Do not send NAS-Port-Id (attribute 87).
          - **packet-type** `enumeration[]`
            Record types to hold the attribute back from. Name none for all of them.
    - **coa-port** `uint16`
      UDP port of the RADIUS CoA and Disconnect listener, usually 3799.
    - **nas-identifier** `string`
      NAS-Identifier sent in RADIUS requests.
    - **nas-port-id-format** `string`
      Template for the NAS-Port-Id attribute sent in RADIUS requests.
    - **require-message-authenticator** `boolean`
      Discard a CoA-Request or a Disconnect-Request that carries no Message-Authenticator.
    - **retries** `uint8`
      Number of retransmit attempts per server.
    - **server <name>** `list`
      One RADIUS server Ze sends requests to, tried in the order written.
      - **address** `string`
        RADIUS server IP address or hostname.
      - **port** `uint16`
        RADIUS server UDP port.
      - **shared-key** `string`
        RADIUS shared secret.
    - **source-address** `ipv4-address`
      Source IPv4 address for outbound RADIUS packets.
    - **timeout** `uint8`
      Per-request timeout in seconds.
- **auth-method** `enumeration`
  PPP Auth-Protocol method first advertised in LCP Configure-Request.
- **authentication** `container`
  PPP authentication phase settings for L2TP sessions.
  - **reauth-interval** `uint32`
    Periodic re-authentication interval in seconds, 0 by default.
  - **timeout** `uint16`
    PPP auth-phase timeout in seconds, 30 by default.
- **cqm-enabled** `boolean`
  Enable the CQM (Customer Quality Monitor) observer.
- **enabled** `boolean`
  Enable L2TP subsystem. Defaults to true when the l2tp block is present; set to false to disable.
- **event-ring-size-per-session** `uint16`
  Number of events in each per-session event ring. When full, oldest events are overwritten.
- **hello-interval** `uint16`
  Seconds of peer silence before Ze sends HELLO.
- **hello-retries** `uint8`
  Unanswered HELLO intervals tolerated before the peer is declared dead.
- **max-logins** `uint32`
  Maximum concurrent PPP logins the CQM observer tracks.
- **max-sessions** `uint16`
  Maximum concurrent sessions per tunnel.
- **max-tunnels** `uint16`
  Maximum concurrent L2TP tunnels.
- **ncp** `container`
  NCP (Network Control Protocol) settings for L2TP sessions.
  - **enable-ipcp** `boolean`
    Enable the IPCP NCP (RFC 1332) for new L2TP sessions. When false, no IPv4 address is negotiated.
  - **enable-ipv6cp** `boolean`
    Enable the IPv6CP NCP (RFC 5072) for a new L2TP session.
  - **timeout** `uint16`
    NCP negotiation timeout in seconds, 30 by default.
- **pool** `container`
  Address and prefix pools handed to L2TP PPP sessions.
  - **ipv4** `container`
    The IPv4 range and the DNS servers a PPP session receives.
    - **dns-primary** `ipv4-address`
      Primary DNS server pushed to subscribers.
    - **dns-secondary** `ipv4-address`
      Secondary DNS server pushed to subscribers.
    - **end** `ipv4-address`
      Last address in the pool range (inclusive).
    - **gateway** `ipv4-address`
      NAS-side IP for all PPP sessions (IPCP local address). Must not overlap the pool range.
    - **start** `ipv4-address`
      First address in the pool range.
  - **ipv6-pd** `container`
    IPv6 prefix delegation pool for subscribers.
    - **block** `string`
      Prefix block to allocate from, for example 2001:db8::/32.
    - **delegation-length** `uint8`
      Prefix length delegated to each subscriber (e.g. 56 for /56 prefixes).
  - **named-ipv6-pool <name>** `list`
    Named IPv6 prefix pools selected by RADIUS Framed-IPv6-Pool attribute (RFC 6911 attr 100).
    - **block** `string`
      Prefix block to allocate from, for example 2001:db8::/32.
    - **delegation-length** `uint8`
      Prefix length delegated to each subscriber.
  - **named-pool <name>** `list`
    Named IPv4 pools selected by RADIUS Framed-Pool attribute.
    - **dns-primary** `ipv4-address`
      Primary DNS server pushed to subscribers.
    - **dns-secondary** `ipv4-address`
      Secondary DNS server pushed to subscribers.
    - **end** `ipv4-address`
      Last address in the pool range (inclusive).
    - **gateway** `ipv4-address`
      NAS-side IP for sessions using this pool.
    - **start** `ipv4-address`
      First address in the pool range.
- **relay <service>** `list`
  PPPoE-to-L2TP relay bindings for the LAC role.
  - **remote** `string`
    Name of the L2TP remote that matching subscribers are relayed to.
- **remote <name>** `list`
  L2TP dial targets, the remote LNS or LAC endpoints Ze dials.
  - **address** `ip-address`
    Remote control-plane IP address to dial. The SCCRQ destination (RFC 2661 Section 6.1).
  - **outgoing-calls** `boolean`
    Permit LNS-side outgoing calls toward this remote, false by default.
  - **port** `uint16`
    Remote UDP port for the control channel, 1701 by default.
  - **shared-secret** `string`
    Per-remote CHAP-MD5 tunnel-authentication secret (RFC 2661).
- **sample-retention-seconds** `uint32`
  Duration of CQM sample retention per login, in seconds.
- **shaper** `container`
  Traffic shaping applied to each L2TP subscriber interface.
  - **default-rate** `rate`
    Default download rate applied to a new session.
  - **qdisc-type** `enumeration`
    Queueing discipline type for subscriber interfaces.
  - **upload-rate** `rate`
    Default upload rate. When omitted, defaults to default-rate.
- **shared-secret** `string`
  Shared secret that computes CHAP-MD5 Challenge Responses (RFC 2661).

## ldp

*Provided by `ldp` ([ze-ldp-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/yang/ze-ldp-cmd.yang), [ze-ldp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/yang/ze-ldp-conf.yang))*

Label Distribution Protocol configuration

- **hello-hold-time** `uint16`
  Hello adjacency hold time
- **hello-interval** `uint16`
  Hello message interval
- **interfaces** `string[]`
  Interfaces on which LDP discovery is enabled
- **keepalive-time** `uint16`
  Session keepalive interval.
- **lsr-id** `string`
  LSR identifier (IPv4 address format)
- **transport-address** `string`
  Transport address for TCP sessions

## mrt

*Provided by `mrt` ([ze-mrt-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/yang/ze-mrt-conf.yang))*

MRT dump configuration.

- **add-path** `boolean`
  Force add-path subtypes even when not negotiated with the peer.
- **all** `container`
  All BGP messages and state changes, written as BGP4MP records.
  - **file** `string`
    Output file path, with strftime patterns for rotation.
  - **interval** `uint32`
    File rotation interval in seconds. Zero disables rotation.
- **direction** `enumeration`
  Which direction of BGP messages to record.
- **extended-timestamp** `boolean`
  Use BGP4MP_ET (type 17) with microsecond resolution instead of BGP4MP (type 16).
- **peer-filter** `string[]`
  If set, only record messages from/to these peer addresses. Empty list means record all peers.
- **routes** `container`
  Periodic RIB snapshots, written as TABLE_DUMP_V2 records.
  - **file** `string`
    Output file path with strftime patterns.
  - **interval** `uint32`
    RIB dump interval in seconds. Minimum 60.
- **updates** `container`
  BGP UPDATE message stream (BGP4MP records).
  - **file** `string`
    Output file path, with strftime patterns for rotation.
  - **interval** `uint32`
    File rotation interval in seconds. Zero disables rotation.

## ospf

*Provided by `ospf` ([ze-ospf-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/yang/ze-ospf-cmd.yang), [ze-ospf-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/yang/ze-ospf-conf.yang))*

OSPFv2 routing instance configuration.

- **address-family** `container`
  More OSPF address families.
  - **ipv4-multicast** `container`
    The IPv4-multicast address family over OSPFv3.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link across this transit area to a backbone area-border router.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID of this address family.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          Single-hop BFD failure detection for this OSPFv3 interface.
          - **enabled** `boolean`
            Enable single-hop BFD failure detection on this interface.
          - **min-rx** `uint32`
            Required minimum BFD receive interval (RFC 5880 Required Min RX Interval).
          - **min-tx** `uint32`
            Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval).
          - **multiplier** `uint8`
            BFD detection multiplier (RFC 5880 Detect Mult).
        - **cost** `uint16`
          Interface output cost.
        - **dead-interval** `uint16`
          Router-dead interval.
        - **enabled** `boolean`
          OSPFv3 enabled on this interface.
        - **hello-interval** `uint16`
          Hello interval.
        - **ipsec** `container`
          Manual IPsec that protects the OSPFv3 packets of this interface.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm.
          - **encryption-key** `string`
            ESP encryption key, written in hex characters.
          - **key** `string`
            Integrity key, written in hex characters.
          - **protocol** `enumeration`
            IPsec protocol that protects the OSPFv3 packets.
          - **replay-window** `uint8`
            Anti-replay window, counted in packets.
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization of the IPv6 family.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds Ze waits before it declares the link synchronized.
        - **nbma-neighbor <router-id>** `list`
          A statically configured NBMA neighbor.
          - **link-local** `string`
            IPv6 link-local address of the neighbor.
          - **priority** `uint8`
            Eligibility of this neighbor in the DR election and the BDR election.
        - **network-type** `enumeration`
          Network type of this interface.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval.
        - **priority** `uint8`
          Priority of this interface in the DR election and the BDR election.
    - **segment-routing** `container`
      Segment Routing over the MPLS dataplane.
      - **enable** `boolean`
        Enable Segment Routing for this address family.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs Ze advertises for the local prefixes.
        - **explicit-null** `boolean`
          Set the E-Flag on the Prefix-SID.
        - **index** `uint32`
          SID index into the SRGB.
        - **no-php** `boolean`
          Set the NP-Flag on the Prefix-SID.
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB, inclusive.
      - **srlb** `container`
        Segment Routing Local Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB, inclusive.
      - **srms-preference** `uint8`
        SR Mapping-Server preference of this node.
  - **ipv4-unicast** `container`
    The IPv4-unicast address family over OSPFv3.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link across this transit area to a backbone area-border router.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID of this address family.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          Single-hop BFD failure detection for this OSPFv3 interface.
          - **enabled** `boolean`
            Enable single-hop BFD failure detection on this interface.
          - **min-rx** `uint32`
            Required minimum BFD receive interval (RFC 5880 Required Min RX Interval).
          - **min-tx** `uint32`
            Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval).
          - **multiplier** `uint8`
            BFD detection multiplier (RFC 5880 Detect Mult).
        - **cost** `uint16`
          Interface output cost.
        - **dead-interval** `uint16`
          Router-dead interval.
        - **enabled** `boolean`
          OSPFv3 enabled on this interface.
        - **hello-interval** `uint16`
          Hello interval.
        - **ipsec** `container`
          Manual IPsec that protects the OSPFv3 packets of this interface.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm.
          - **encryption-key** `string`
            ESP encryption key, written in hex characters.
          - **key** `string`
            Integrity key, written in hex characters.
          - **protocol** `enumeration`
            IPsec protocol that protects the OSPFv3 packets.
          - **replay-window** `uint8`
            Anti-replay window, counted in packets.
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization of the IPv6 family.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds Ze waits before it declares the link synchronized.
        - **nbma-neighbor <router-id>** `list`
          A statically configured NBMA neighbor.
          - **link-local** `string`
            IPv6 link-local address of the neighbor.
          - **priority** `uint8`
            Eligibility of this neighbor in the DR election and the BDR election.
        - **network-type** `enumeration`
          Network type of this interface.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval.
        - **priority** `uint8`
          Priority of this interface in the DR election and the BDR election.
    - **segment-routing** `container`
      Segment Routing over the MPLS dataplane.
      - **enable** `boolean`
        Enable Segment Routing for this address family.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs Ze advertises for the local prefixes.
        - **explicit-null** `boolean`
          Set the E-Flag on the Prefix-SID.
        - **index** `uint32`
          SID index into the SRGB.
        - **no-php** `boolean`
          Set the NP-Flag on the Prefix-SID.
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB, inclusive.
      - **srlb** `container`
        Segment Routing Local Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB, inclusive.
      - **srms-preference** `uint8`
        SR Mapping-Server preference of this node.
  - **ipv6** `container`
    The default IPv6-unicast OSPFv3 address family.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link across this transit area to a backbone area-border router.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID of this address family.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          Single-hop BFD failure detection for this OSPFv3 interface.
          - **enabled** `boolean`
            Enable single-hop BFD failure detection on this interface.
          - **min-rx** `uint32`
            Required minimum BFD receive interval (RFC 5880 Required Min RX Interval).
          - **min-tx** `uint32`
            Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval).
          - **multiplier** `uint8`
            BFD detection multiplier (RFC 5880 Detect Mult).
        - **cost** `uint16`
          Interface output cost.
        - **dead-interval** `uint16`
          Router-dead interval.
        - **enabled** `boolean`
          OSPFv3 enabled on this interface.
        - **hello-interval** `uint16`
          Hello interval.
        - **ipsec** `container`
          Manual IPsec that protects the OSPFv3 packets of this interface.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm.
          - **encryption-key** `string`
            ESP encryption key, written in hex characters.
          - **key** `string`
            Integrity key, written in hex characters.
          - **protocol** `enumeration`
            IPsec protocol that protects the OSPFv3 packets.
          - **replay-window** `uint8`
            Anti-replay window, counted in packets.
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization of the IPv6 family.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds Ze waits before it declares the link synchronized.
        - **nbma-neighbor <router-id>** `list`
          A statically configured NBMA neighbor.
          - **link-local** `string`
            IPv6 link-local address of the neighbor.
          - **priority** `uint8`
            Eligibility of this neighbor in the DR election and the BDR election.
        - **network-type** `enumeration`
          Network type of this interface.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval.
        - **priority** `uint8`
          Priority of this interface in the DR election and the BDR election.
    - **segment-routing** `container`
      Segment Routing over the MPLS dataplane.
      - **enable** `boolean`
        Enable Segment Routing for this address family.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs Ze advertises for the local prefixes.
        - **explicit-null** `boolean`
          Set the E-Flag on the Prefix-SID.
        - **index** `uint32`
          SID index into the SRGB.
        - **no-php** `boolean`
          Set the NP-Flag on the Prefix-SID.
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB, inclusive.
      - **srlb** `container`
        Segment Routing Local Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB, inclusive.
      - **srms-preference** `uint8`
        SR Mapping-Server preference of this node.
  - **ipv6-multicast** `container`
    The IPv6-multicast OSPFv3 address family.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link across this transit area to a backbone area-border router.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID of this address family.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          Single-hop BFD failure detection for this OSPFv3 interface.
          - **enabled** `boolean`
            Enable single-hop BFD failure detection on this interface.
          - **min-rx** `uint32`
            Required minimum BFD receive interval (RFC 5880 Required Min RX Interval).
          - **min-tx** `uint32`
            Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval).
          - **multiplier** `uint8`
            BFD detection multiplier (RFC 5880 Detect Mult).
        - **cost** `uint16`
          Interface output cost.
        - **dead-interval** `uint16`
          Router-dead interval.
        - **enabled** `boolean`
          OSPFv3 enabled on this interface.
        - **hello-interval** `uint16`
          Hello interval.
        - **ipsec** `container`
          Manual IPsec that protects the OSPFv3 packets of this interface.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm.
          - **encryption-key** `string`
            ESP encryption key, written in hex characters.
          - **key** `string`
            Integrity key, written in hex characters.
          - **protocol** `enumeration`
            IPsec protocol that protects the OSPFv3 packets.
          - **replay-window** `uint8`
            Anti-replay window, counted in packets.
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization of the IPv6 family.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds Ze waits before it declares the link synchronized.
        - **nbma-neighbor <router-id>** `list`
          A statically configured NBMA neighbor.
          - **link-local** `string`
            IPv6 link-local address of the neighbor.
          - **priority** `uint8`
            Eligibility of this neighbor in the DR election and the BDR election.
        - **network-type** `enumeration`
          Network type of this interface.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval.
        - **priority** `uint8`
          Priority of this interface in the DR election and the BDR election.
    - **segment-routing** `container`
      Segment Routing over the MPLS dataplane.
      - **enable** `boolean`
        Enable Segment Routing for this address family.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs Ze advertises for the local prefixes.
        - **explicit-null** `boolean`
          Set the E-Flag on the Prefix-SID.
        - **index** `uint32`
          SID index into the SRGB.
        - **no-php** `boolean`
          Set the NP-Flag on the Prefix-SID.
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB, inclusive.
      - **srlb** `container`
        Segment Routing Local Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB, inclusive.
      - **srms-preference** `uint8`
        SR Mapping-Server preference of this node.
  - **ipv6-unicast** `container`
    IPv6-unicast OSPFv3 address family (RFC 5838 §2.1: Instance ID 0-31).
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link across this transit area to a backbone area-border router.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID of this address family.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          Single-hop BFD failure detection for this OSPFv3 interface.
          - **enabled** `boolean`
            Enable single-hop BFD failure detection on this interface.
          - **min-rx** `uint32`
            Required minimum BFD receive interval (RFC 5880 Required Min RX Interval).
          - **min-tx** `uint32`
            Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval).
          - **multiplier** `uint8`
            BFD detection multiplier (RFC 5880 Detect Mult).
        - **cost** `uint16`
          Interface output cost.
        - **dead-interval** `uint16`
          Router-dead interval.
        - **enabled** `boolean`
          OSPFv3 enabled on this interface.
        - **hello-interval** `uint16`
          Hello interval.
        - **ipsec** `container`
          Manual IPsec that protects the OSPFv3 packets of this interface.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm.
          - **encryption-key** `string`
            ESP encryption key, written in hex characters.
          - **key** `string`
            Integrity key, written in hex characters.
          - **protocol** `enumeration`
            IPsec protocol that protects the OSPFv3 packets.
          - **replay-window** `uint8`
            Anti-replay window, counted in packets.
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization of the IPv6 family.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds Ze waits before it declares the link synchronized.
        - **nbma-neighbor <router-id>** `list`
          A statically configured NBMA neighbor.
          - **link-local** `string`
            IPv6 link-local address of the neighbor.
          - **priority** `uint8`
            Eligibility of this neighbor in the DR election and the BDR election.
        - **network-type** `enumeration`
          Network type of this interface.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval.
        - **priority** `uint8`
          Priority of this interface in the DR election and the BDR election.
    - **segment-routing** `container`
      Segment Routing over the MPLS dataplane.
      - **enable** `boolean`
        Enable Segment Routing for this address family.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs Ze advertises for the local prefixes.
        - **explicit-null** `boolean`
          Set the E-Flag on the Prefix-SID.
        - **index** `uint32`
          SID index into the SRGB.
        - **no-php** `boolean`
          Set the NP-Flag on the Prefix-SID.
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB, inclusive.
      - **srlb** `container`
        Segment Routing Local Block of this node.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB, inclusive.
      - **srms-preference** `uint8`
        SR Mapping-Server preference of this node.
- **areas** `container`
  OSPF areas.
  - **area <area-id>** `list`
    Per-area configuration.
    - **area-type** `enumeration`
      Type of this area.
    - **authentication** `container`
      Area-level authentication defaults.
      - **key-chain** `string`
        Default key chain inherited by interfaces.
    - **default-cost** `uint32`
      Default summary metric for stub/NSSA areas.
    - **no-summary** `boolean`
      Suppress the Type 3 summaries of a totally-stubby or totally-NSSA area.
    - **nssa** `container`
      NSSA-specific configuration (applies when area-type is nssa).
      - **default-originate** `boolean`
        Originate a Type 7 default route into the NSSA.
      - **stability-interval** `uint16`
        Hysteresis before a translator that lost the role stops translating.
      - **translate-role** `enumeration`
        Role of this router in the Type 7 to Type 5 translation.
    - **ranges** `container`
      Inter-area summary ranges.
      - **range <prefix>** `list`
        One area summary range.
        - **advertise** `enumeration`
          Advertise the aggregate or suppress specifics.
        - **cost** `uint32`
          Override cost for the aggregate Type 3 LSA.
    - **virtual-link <remote-router-id>** `list`
      Virtual link across this transit area to a backbone area-border router.
      - **dead-interval** `uint16`
        Router-dead interval on the virtual link.
      - **hello-interval** `uint16`
        Hello interval on the virtual link.
      - **retransmit-interval** `uint16`
        LSA retransmit interval on the virtual link.
      - **transmit-delay** `uint16`
        Estimated LSA transmission delay (RFC 2328 InfTransDelay must be > 0).
- **default-information** `container`
  Default-route origination as an AS-external LSA.
  - **always** `boolean`
    Originate even without a default in the RIB.
  - **metric** `uint32`
    Default LSA metric.
  - **metric-type** `enumeration`
    External metric type for the default route.
  - **originate** `boolean`
    Originate a default Type 5 LSA.
- **extended-link** `boolean`
  Originate Extended Link Opaque LSAs for the local links.
- **extended-prefix** `boolean`
  Originate Extended Prefix Opaque LSAs for the local prefixes.
- **fast-reroute** `container`
  Loop-Free Alternate and TI-LFA IP fast reroute.
  - **enable** `boolean`
    Enable LFA / TI-LFA fast-reroute backup computation and install.
  - **mode** `enumeration`
    Backup computation mode: base LFA, or LFA with a TI-LFA SR-repair fallback.
  - **node-protection** `boolean`
    Prefer node-protecting alternates over link-only alternates (RFC 5286 Section 3.6).
- **graceful-restart** `container`
  Graceful Restart, which keeps forwarding across a control-plane restart.
  - **helper** `container`
    Helper role of this router.
    - **strict-lsa-checking** `boolean`
      Stop helper mode when a changed LSA would flood to the restarting router.
    - **support** `boolean`
      RFC 3623 Appendix B.2 RestartHelperSupport: act as a helper for a restarting neighbor.
  - **restarter** `container`
    Restarting-router role of this router.
    - **restart-interval** `uint16`
      Grace period during which a neighbor advertises this router as fully adjacent.
    - **support** `enumeration`
      RFC 3623 Appendix B.1 RestartSupport.
- **interfaces** `container`
  OSPF-enabled interfaces.
  - **interface <name>** `list`
    Per-interface OSPF configuration.
    - **area** `string`
      Declared area this interface belongs to.
    - **authentication** `container`
      Per-interface authentication settings.
      - **key-chain** `string`
        Key chain reference.
      - **mode** `enumeration`
        Authentication mode of this interface.
    - **bfd** `container`
      Single-hop BFD failure detection for this OSPF interface.
      - **enabled** `boolean`
        Enable single-hop BFD failure detection on this interface.
      - **min-rx** `uint32`
        Required minimum BFD receive interval (RFC 5880 Required Min RX Interval).
      - **min-tx** `uint32`
        Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval).
      - **multiplier** `uint8`
        BFD detection multiplier (RFC 5880 Detect Mult).
    - **cost** `uint16`
      Interface output cost.
    - **dead-interval** `uint16`
      Router-dead interval.
    - **enabled** `boolean`
      OSPF enabled on this interface.
    - **hello-interval** `uint16`
      Hello interval.
    - **instance-id** `uint8[]`
      Multi-Instance Instance IDs this interface takes part in.
    - **ldp-sync** `container`
      LDP-IGP synchronization of this interface.
      - **enable** `boolean`
        Enable LDP-IGP synchronization on this interface.
      - **holddown** `uint16`
        Seconds Ze waits before it declares the link synchronized.
    - **mtu-ignore** `boolean`
      Skip DD MTU mismatch rejection.
    - **nbma-neighbor <address>** `list`
      A statically configured NBMA neighbor.
      - **priority** `uint8`
        Eligibility of this neighbor in the DR election and the BDR election.
    - **network-type** `enumeration`
      Network type of this interface.
    - **passive** `boolean`
      Advertise the interface but form no adjacency.
    - **poll-interval** `uint16`
      NBMA poll interval.
    - **priority** `uint8`
      Priority of this interface in the DR election and the BDR election.
    - **retransmit-interval** `uint16`
      LSA retransmit interval.
    - **traffic-engineering** `container`
      Traffic Engineering link attributes of this interface.
      - **admin-group** `uint32`
        Administrative Group mask of this link.
      - **enable** `boolean`
        Advertise a TE Link LSA for this interface.
      - **inter-as** `container`
        Inter-AS Traffic Engineering link.
        - **remote-as** `asn`
          RFC 5392 sec 3.3.1 Remote AS Number of the neighboring AS the link connects to.
        - **remote-asbr-ipv4** `ipv4-address`
          IPv4 Remote ASBR ID of the inter-AS link.
        - **remote-asbr-ipv6** `ipv6-address`
          IPv6 Remote ASBR ID of the inter-AS link.
        - **scope** `enumeration`
          Flooding scope of the inter-AS TE LSA.
      - **max-bandwidth** `uint64`
        RFC 3630 sec 2.5.6 Maximum Bandwidth (true link capacity) in bytes/second.
      - **max-reservable-bandwidth** `uint64`
        Maximum Reservable Bandwidth of this link.
      - **te-metric** `uint32`
        Traffic Engineering metric of this link.
    - **transmit-delay** `uint16`
      Estimated LSA transmission delay (RFC 2328 InfTransDelay must be > 0).
- **key-chains <name>** `list`
  Named authentication key chains for hitless rotation.
  - **extended-sequence** `boolean`
    Use AuType 3 rather than AuType 2 for this key chain.
  - **key <key-id>** `list`
    Keys in this chain.
    - **accept-lifetime** `container`
      When this key is accepted on receive.
      - **end** `string`
        RFC3339 end timestamp.
      - **start** `string`
        RFC3339 start timestamp.
    - **algorithm** `enumeration`
      Authentication algorithm.
    - **secret** `string`
      Shared secret, masked and $9$-encoded at rest.
    - **send-lifetime** `container`
      Window in which Ze can use this key to sign a packet.
      - **end** `string`
        RFC3339 end timestamp.
      - **start** `string`
        RFC3339 start timestamp.
- **max-metric** `container`
  Stub-router advertisement of this router.
  - **router-lsa** `container`
    Stub-router control of the Router-LSA link costs.
    - **always** `boolean`
      Always advertise as a stub router.
    - **on-shutdown** `uint32`
      Advertise as a stub router for N seconds during a graceful shutdown (0 disables).
    - **on-startup** `uint32`
      Advertise as a stub router for N seconds after startup (0 disables).
- **maximum-paths** `uint8`
  Maximum ECMP paths per prefix.
- **opaque** `boolean`
  Enable the opaque-LSA capability.
- **redistribute <source>** `list`
  Per-source metric, metric type and tag for redistribution into OSPF.
  - **metric** `uint32`
    Injected metric.
  - **metric-type** `enumeration`
    External metric type.
  - **tag** `uint32`
    External route tag.
- **reference-bandwidth** `uint32`
  Auto-cost reference bandwidth in Mbps.
- **router-address** `ipv4-address`
  Traffic Engineering Router Address of this router.
- **router-id** `string`
  OSPF Router ID of this instance, in dotted-quad form.
- **router-information** `container`
  Router Information LSA of this router.
  - **enabled** `boolean`
    Originate the Router Information LSA.
  - **scope** `enumeration[]`
    Flooding scopes at which Ze advertises the RI LSA.
- **segment-routing** `container`
  Segment Routing over the MPLS dataplane.
  - **enable** `boolean`
    Enable Segment Routing for this address family.
  - **prefix-sid <prefix>** `list`
    Prefix-SIDs Ze advertises for the local prefixes.
    - **explicit-null** `boolean`
      Set the E-Flag on the Prefix-SID.
    - **index** `uint32`
      SID index into the SRGB.
    - **no-php** `boolean`
      Set the NP-Flag on the Prefix-SID.
    - **node-sid** `boolean`
      Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
  - **srgb** `container`
    Segment Routing Global Block of this node.
    - **lower-bound** `uint32`
      First MPLS label of the SRGB (inclusive).
    - **upper-bound** `uint32`
      Last MPLS label of the SRGB, inclusive.
  - **srlb** `container`
    Segment Routing Local Block of this node.
    - **lower-bound** `uint32`
      First MPLS label of the SRLB (inclusive).
    - **upper-bound** `uint32`
      Last MPLS label of the SRLB, inclusive.
  - **srms-preference** `uint8`
    SR Mapping-Server preference of this node.
- **timers** `container`
  SPF and LSA throttle timers.
  - **min-ls-arrival-ms** `uint32`
    Minimum arrival interval for accepting a new LSA instance.
  - **min-ls-interval-ms** `uint32`
    Minimum interval between LSA reoriginations.
  - **spf-delay-ms** `uint32`
    Initial SPF delay.
  - **spf-hold-ms** `uint32`
    SPF hold floor.
  - **spf-max-hold-ms** `uint32`
    SPF max hold.

## pki

*Provided by `ike`*

PKI certificate and key store.

- **ca <name>** `list`
  Trusted CA certificate.
  - **certificate** `string`
    X.509 CA certificate, as a PEM document or as base64-encoded DER.
  - **crl** `string[]`
    Certificate revocation lists this CA issued, each a PEM document or base64-encoded DER.
- **certificate <name>** `list`
  Device certificate with optional private key and intermediate certificates.
  - **certificate** `string`
    X.509 device certificate, as a PEM document or as base64-encoded DER.
  - **intermediate** `string[]`
    Intermediate CA certificates, from the issuer toward the trust anchor.
  - **ocsp-response** `string`
    OCSP response about this certificate, stapled to a peer that asks for it.
  - **private** `container`
    Private key associated with this certificate.
    - **key** `string`
      Private key, as a PEM document or as base64-encoded DER.

## plugin

Plugin configuration

- **external <name>** `list`
  External plugin process
  - **encoder** `enumeration`
    Event encoding format
  - **respawn** `boolean`
    Respawn on exit
  - **run** `string`
    Command to execute plugin as external process (with args)
  - **timeout** `string`
    Startup timeout (e.g., 10s, 1m)
  - **use** `string`
    Name of a built-in plugin to run in-process (mutually exclusive with run)
- **hub** `container`
  Plugin transport and auth configuration
  - **client <name>** `list`
    Outbound hub connections (managed client mode)
    - **ca** `string`
      Name of the pki ca entry holding the hub certificate authority root.
    - **host** `string`
      Remote hub address
    - **port** `uint16`
      Remote hub port
    - **secret** `string`
      Auth token (min 32 chars)
    - **source-address** `ip-address`
      Source IP address for outbound hub connections.
  - **server <name>** `list`
    Named hub server instances (TLS listeners)
    - **certificate** `string`
      Name of the pki certificate the managed-client listener on this block serves.
    - **client <name>** `list`
      Accepted remote managed clients
      - **secret** `string`
        Per-client auth token (min 32 chars)
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
    - **secret** `string`
      Auth token for plugin connections (min 32 chars)
- **internal <name>** `list`
  Built-in plugin running in-process
  - **use** `string`
    Name of a built-in plugin to run in-process

## policy

*Provided by `policy-routes` ([ze-policyroute-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/policyroute/yang/ze-policyroute-cmd.yang), [ze-policyroute-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/policyroute/yang/ze-policyroute-conf.yang))*

Policy routing configuration.

- **route <name>** `list`
  A named policy route applied to ingress interfaces.
  - **interface** `string[]`
    Ingress interfaces to match. The policy matches a packet that arrives on any one of them. A trailing '*' enables prefix (wildcard) matching (e.g. 'l2tp*').
  - **rule <name>** `list`
    Ordered list of match/action rules.
    - **from** `container`
      Packet match criteria.
      - **destination-address** `string`
        Destination IP prefix or @set reference.
      - **destination-port** `string`
        Destination port or port range.
      - **protocol** `protocol-name`
        L4 protocol to match.
      - **source-address** `string`
        Source IP prefix or @set reference.
      - **source-port** `string`
        Source port or port range (e.g. '80,443').
      - **tcp-flags** `string`
        Comma-separated TCP flags to match (fin, syn, rst, psh, ack, urg).
    - **order** `uint32`
      Evaluation order of this rule, lower values first.
    - **then** `container`
      Action for matching packets.
      - **accept** `empty`
        Skip this policy (packet routes normally).
      - **drop** `empty`
        Drop matching packets.
      - **next-hop** `string`
        Redirect matching packets to this next-hop.
      - **table** `uint32`
        Route matching packets through this kernel routing table.
      - **tcp-mss** `uint16`
        Rewrite the TCP MSS option to this value in bytes.

## pppoe

PPPoE access concentrator settings (RFC 2516).

- **ac-name** `string`
  Access Concentrator Name advertised in PADO (RFC 2516 Section 5.2).
- **allow-no-auth** `boolean`
  Accept a subscriber whose LCP negotiation ends with no Auth-Protocol option.
- **auth-method** `enumeration`
  PPP Auth-Protocol method the access concentrator advertises in LCP.
- **cookie-timeout** `uint16`
  AC-Cookie validity duration in seconds. Cookies older than this are rejected in PADR validation.
- **enabled** `boolean`
  Enable the PPPoE subsystem.
- **interface <name>** `list`
  Access interfaces on which PPPoE discovery listens.
  - **max-sessions** `uint16`
    Per-interface session limit. Defaults to the global max-sessions when unset.
  - **max-sessions-per-mac** `uint16`
    Per-interface override for max-sessions-per-mac. Defaults to the global value when unset.
  - **service-name** `string[]`
    Per-interface Service-Name filter. Overrides the global service-name list when set.
- **max-sessions** `uint16`
  Maximum concurrent PPPoE sessions per interface.
- **max-sessions-per-mac** `uint16`
  Maximum concurrent sessions one subscriber MAC address may hold.
- **padi-rate-limit** `uint16`
  Maximum PADI packets accepted per second on one interface.
- **service-name** `string[]`
  Accepted Service-Name values (RFC 2516 Section 5.1).

## redistribute

*Provided by `redistribute-orchestrator`*

Route redistribution between protocols.

- **destination <protocol>** `list`
  Destination protocol for redistributed routes.
  - **import <source>** `list`
    Import routes from a source into this destination.
    - **family** `string[]`
      Address families Ze imports from this source.
    - **tag** `uint32`
      Import only the routes that carry this route tag.

## rib

*Provided by `rib` ([ze-rib-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/yang/ze-rib-conf.yang))*

System RIB configuration.

- **distance** `container`
  Administrative distance per protocol.
  - **connected** `uint8`
    Directly connected networks.
  - **ebgp** `uint8`
    External BGP routes.
  - **ibgp** `uint8`
    Internal BGP routes.
  - **isis** `uint8`
    IS-IS routes.
  - **ospf** `uint8`
    OSPF routes.
  - **static** `uint8`
    Static routes.
- **fib-withhold** `string[]`
  Protocols whose selected routes are not programmed into the FIB. An empty list programs all.

## routing-table

*Provided by `routing-table` ([ze-routing-table-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/routingtable/yang/ze-routing-table-conf.yang))*

Named routing table definitions. Each entry maps a name to a kernel routing table ID.

- **table <name>** `list`
  A named routing table.
  - **id** `uint32`
    Kernel routing table ID. Reserved IDs excluded: 0 (use 'default'), 253-255 (kernel reserved).

## rsvp-te

*Provided by `rsvp-te` ([ze-rsvp-te-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/yang/ze-rsvp-te-cmd.yang), [ze-rsvp-te-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang))*

RSVP-TE traffic engineering configuration

- **bypass <name>** `list`
  Facility-backup bypass LSP from this PLR to a merge point (RFC 4090 Section 3.2).
  - **explicit-route <index>** `list`
    Bypass path hops (must avoid the protected resource)
    - **address** `string`
      Hop address (IPv4 prefix)
    - **type** `enumeration`
      Hop type (strict or loose)
  - **merge-point** `string`
    Merge point address (IPv4) where this bypass rejoins the protected LSPs.
  - **node-protection** `boolean`
    This bypass merges at a next-next hop, so it gives node protection
- **interface <name>** `list`
  Per-interface RSVP-TE configuration
  - **address** `string`
    Local link prefix in IPv4 CIDR form, for example 10.0.0.4/30.
  - **max-bandwidth** `string`
    Maximum link bandwidth (bps)
  - **max-reservable-bandwidth** `string`
    Maximum reservable bandwidth (bps)
- **refresh-multiplier** `uint8`
  Number of missed refreshes before state cleanup
- **refresh-period** `uint16`
  PATH/RESV refresh interval (RFC 2205 soft-state)
- **router-id** `string`
  Router identifier (IPv4 address format)
- **tunnel <name>** `list`
  RSVP-TE LSP tunnel definition
  - **bandwidth** `string`
    Requested bandwidth (bps)
  - **destination** `string`
    Tunnel endpoint (IPv4 address)
  - **explicit-route <index>** `list`
    Explicit route hops
    - **address** `string`
      Hop address (IPv4 prefix)
    - **type** `enumeration`
      Hop type (strict or loose)
  - **fast-reroute** `container`
    Fast Reroute (RFC 4090) local protection for this LSP
    - **backup** `enumeration`
      Backup method: facility (one bypass protects many LSPs) or one-to-one (a detour per LSP)
    - **bandwidth-protection** `boolean`
      Request a backup that guarantees the reserved bandwidth
    - **hop-limit** `uint8`
      Maximum number of hops the backup path can take
    - **node-protection** `boolean`
      Request NNHOP (node) protection rather than NHOP (link) protection
  - **hold-priority** `uint8`
    Hold priority (0 = highest)
  - **setup-priority** `uint8`
    Setup priority (0 = highest)
  - **tunnel-id** `uint16`
    Tunnel identifier

## service

*Provided by `as112` ([ze-as112-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/yang/ze-as112-cmd.yang), [ze-as112-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/yang/ze-as112-conf.yang)); `dhcpserver` ([ze-dhcp-server-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang)); `geodns` ([ze-geodns-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/yang/ze-geodns-cmd.yang), [ze-geodns-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/yang/ze-geodns-conf.yang)); `imageserver` ([ze-image-server-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/imageserver/yang/ze-image-server-conf.yang)); `tftpserver` ([ze-tftp-server-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang))*

Service settings.

- **as112** `container`
  AS112 anycast DNS node.
  - **address-family** `enumeration`
    Restrict the service to one address family.
  - **allow-from** `ip-prefix[]`
    Client-source access list of the queries the node answers.
  - **asn** `asn`
    Origin AS the AS112 covering prefixes carry when redistributed into BGP.
  - **community** `string[]`
    BGP communities attached to the redistributed AS112 covering prefixes.
  - **doh** `container`
    DNS-over-HTTPS listener (RFC 8484).
    - **enabled** `boolean`
      Enable the DNS-over-HTTPS listener (RFC 8484).
    - **listen-port** `port`
      TCP port for the DoH HTTPS listener (RFC 8484).
    - **path** `string`
      HTTP request path the DoH endpoint answers on.
  - **enabled** `boolean`
    Enable the AS112 anycast DNS node.
  - **facility** `string`
    Facility name in the HOSTNAME.AS112.NET/ARPA TXT answers.
  - **hostname** `string`
    Node identification string in the HOSTNAME.AS112.NET/ARPA TXT answers.
  - **ipv4-anycast-listener <name>** `list`
    Presence-only anchor for port 53 on the fixed IPv4 anycast addresses.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **ipv6-anycast-listener <name>** `list`
    Presence-only anchor for port 53 on the fixed IPv6 anycast addresses.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **location** `string`
    City and country in the HOSTNAME.AS112.NET/ARPA TXT answers.
  - **tls** `container`
    DNS-over-TLS listener, and the certificate material it shares with DoH.
    - **cert-file** `string`
      Path to the PEM server certificate presented on DoT and DoH.
    - **certificate** `string`
      Name of the PKI store certificate served on DoT and DoH.
    - **enabled** `boolean`
      Enable the DNS-over-TLS listener (RFC 7858).
    - **key-file** `string`
      Path to the PEM private key matching cert-file. Must be set together with cert-file.
    - **listen-port** `port`
      TCP port for the DoT listener.
  - **watchdog** `boolean`
    Announce the covering prefixes only while the node serves DNS.
- **dhcp-server** `container`
  DHCP server settings
  - **enabled** `boolean`
    Enable DHCP server
  - **listen-interface** `string[]`
    Interfaces to serve DHCP on
  - **pxe** `container`
    PXE boot server settings (RFC 4578)
    - **boot-script-url** `string`
      HTTP URL for iPXE boot script (sent as option 67 to iPXE clients)
    - **bootfile-bios** `string`
      Boot file path for BIOS clients (option 67)
    - **bootfile-uefi** `string`
      Boot file path for UEFI clients (option 67)
    - **enabled** `boolean`
      Enable PXE boot option injection
    - **tftp-server** `string`
      TFTP server IP address for PXE boot (option 66, siaddr)
  - **shared-network <name>** `list`
    Named network grouping of subnets
    - **subnet <prefix>** `list`
      Subnet with address pool and options
      - **default-router** `string`
        Default gateway address (option 3)
      - **dns-server** `string[]`
        DNS server addresses (option 6)
      - **domain-name** `string`
        Domain name for clients (option 15)
      - **lease-time** `uint32`
        Lease duration in seconds
      - **range <name>** `list`
        Named dynamic address pool range
        - **start** `string`
          First allocatable address
        - **stop** `string`
          Last allocatable address
      - **static-mapping <name>** `list`
        Static MAC-to-IP binding
        - **ip-address** `string`
          Fixed IP address for this client
        - **mac-address** `string`
          Client MAC address (xx:xx:xx:xx:xx:xx)
- **geodns** `container`
  GeoDNS server: per-source-IP DNS answers
  - **client-ip-source** `enumeration`
    Where the client IP used for source selection is read from
  - **default-ttl** `uint32`
    Default record TTL in seconds when a host omits its own.
  - **doh** `container`
    DNS-over-HTTPS listener (DoH, RFC 8484).
    - **enabled** `boolean`
      Enable the DNS-over-HTTPS listener (RFC 8484).
    - **listen-port** `port`
      TCP port for the DoH HTTPS listener (RFC 8484).
    - **path** `string`
      HTTP request path the DoH endpoint answers on (RFC 8484 URI Template); defaults to /dns-query.
  - **enabled** `boolean`
    Enable the GeoDNS server
  - **host-set <name>** `list`
    Named, reusable set of host records; referenced by source entries
    - **host <name>** `list`
      A hostname and its records
      - **address** `ip-address[]`
        One or more A/AAAA addresses (v4 => A, v6 => AAAA when type omitted)
      - **srv** `container`
        SRV record fields (when type is SRV)
        - **port** `uint16`
          SRV port
        - **priority** `uint16`
          SRV priority
        - **target** `string`
          SRV target hostname
        - **weight** `uint16`
          SRV weight
      - **ttl** `uint32`
        Record TTL; defaults to default-ttl when unset
      - **type** `enumeration`
        Record type; omit to auto-detect A vs AAAA per address
  - **listener <name>** `list`
    UDP and TCP listen endpoints for geodns.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **nameserver** `ipv4-address[]`
    Nameserver IPv4 addresses; ns1..nsN.<zone> A glue is synthesised (max 9)
  - **soa** `container`
    SOA record fields (synthesised; geodns serves no AXFR)
    - **contact** `string`
      Responsible-party mailbox label (SOA RNAME), e.g. hostmaster
    - **expire** `uint32`
      SOA expire seconds
    - **minimum** `uint32`
      SOA minimum, the negative-cache TTL in seconds.
    - **mname** `string`
      Primary nameserver (SOA MNAME). Defaults to ns1.<first-zone> when unset.
    - **refresh** `uint32`
      SOA refresh seconds
    - **retry** `uint32`
      SOA retry seconds
    - **serial** `uint32`
      SOA serial used when serial-mode is fixed
    - **serial-mode** `enumeration`
      How the 32-bit SOA serial is generated
  - **source <prefix>** `list`
    Maps a client-IP prefix to a host-set.
    - **host-set** `string`
      Name of the host-set to answer for clients matching this prefix
  - **tls** `container`
    DNS-over-TLS listener and the certificate material shared with DoH.
    - **cert-file** `string`
      Path to the PEM server certificate presented on DoT and DoH.
    - **certificate** `string`
      Name of the PKI store certificate to serve on DoT and DoH.
    - **enabled** `boolean`
      Enable the DNS-over-TLS listener (RFC 7858).
    - **key-file** `string`
      Path to the PEM private key matching cert-file. Must be set together with cert-file.
    - **listen-port** `port`
      TCP port for the DoT listener; the RFC 7858 well-known 'domain-s' port is 853.
  - **zone** `string[]`
    Zones served, as fully qualified domain names.
- **image-server** `container`
  HTTP image server for PXE provisioning
  - **boot-directory** `string`
    Directory containing installer kernel, initrd, iPXE config
  - **enabled** `boolean`
    Enable image server
  - **image-directory** `string`
    Directory containing gokrazy disk images
  - **listen-interface** `string[]`
    Interfaces to serve on
  - **listen-port** `uint16`
    HTTP listen port
  - **rescue-auth** `string`
    Salted argon2id of the installer rescue token, as '<saltHex>:<digestHex>'.
  - **ssh-password-hash** `string`
    Bcrypt hash of admin password (written to served zefs)
  - **ssh-username** `string`
    Admin username for installed target (written to served zefs)
- **tftp-server** `container`
  Read-only TFTP server (RFC 1350)
  - **enabled** `boolean`
    Enable TFTP server
  - **listen-interface** `string[]`
    Interfaces to serve TFTP on
  - **max-transfers** `uint16`
    Maximum concurrent TFTP transfers
  - **root-directory** `string`
    Directory to serve files from

## static

*Provided by `static` ([ze-static-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/static/yang/ze-static-cmd.yang), [ze-static-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/static/yang/ze-static-conf.yang))*

Static route configuration.

- **table <name>** `list`
  A named routing table that holds static routes.
  - **route <prefix>** `list`
    A static route entry.
    - **action** `choice`
      What to do with matching packets.
      - **discard** `case`
        Drop the packet and tell the sender nothing.
        - **blackhole** `container`
          Drop the packet and send no ICMP message to the sender.
      - **forward** `case`
        Send the packet on toward a next hop.
        - **next** `container`
          Forward action. Contains gateway and interface next-hops that may be combined for ECMP.
          - **hop <address>** `list`
            Forward via gateway. Multiple entries produce ECMP with traffic distributed by weight.
            - **bfd-profile** `string`
              BFD profile name for this next-hop, from the bfd/profile list.
            - **interface** `string`
              Outgoing interface. Required when the next-hop is a link-local IPv6 address.
            - **weight** `uint16`
              ECMP weight of this next-hop, 1 by default.
          - **interface <name>** `list`
            Forward through the interface only, with no gateway address.
            - **weight** `uint16`
              ECMP weight. Higher values receive proportionally more traffic.
      - **unreachable** `case`
        Drop the packet and answer the sender.
        - **reject** `container`
          Drop the packet and send an ICMP unreachable to the sender.
    - **description** `string`
      Operator note for this route.
    - **metric** `uint32`
      Route metric. Used as kernel route priority (lower is preferred) and carried in redistribute.
    - **tag** `uint32`
      Opaque tag Ze attaches to this route for policy and redistribution.

## storage

Storage device management

- **smart** `container`
  SMART disk health monitoring and self-test scheduling
  - **check-interval** `uint32`
    Health poll interval in seconds
  - **enabled** `boolean`
    Enable SMART monitoring on all detected ATA/NVMe devices
  - **self-test** `container`
    Periodic SMART self-test scheduling
    - **long** `container`
      Extended self-test schedule
      - **day** `enumeration`
        Preferred day of week for extended self-tests
      - **interval** `string`
        Interval between extended self-tests, for example 7d
      - **time** `string`
        Preferred time of day for extended self-tests (HH:MM)
    - **short** `container`
      Short self-test schedule
      - **interval** `string`
        Interval between short self-tests, for example 24h
      - **time** `string`
        Preferred time of day for short self-tests (HH:MM)
  - **temperature** `container`
    Temperature alert thresholds in degrees Celsius
    - **critical** `uint8`
      Temperature above which a critical error is raised
    - **difference** `uint8`
      Temperature change threshold for rate-of-change alerts
    - **informational** `uint8`
      Temperature above which an informational warning is raised

## sysctl

*Provided by `sysctl` ([ze-sysctl-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/sysctl/yang/ze-sysctl-conf.yang))*

Kernel tunable management.

- **profile <name>** `list`
  A named collection of kernel tunables that Ze applies together to interface units.
  - **setting <name>** `list`
    Kernel tunables in this profile.
    - **value** `string`
      Value to set for this key.
- **setting <name>** `list`
  A kernel tunable that Ze sets persistently.
  - **value** `string`
    Value to set for this key.

## system

System-level settings

- **archive <name>** `list`
  Named config archive destinations.
  - **filename** `string`
    Filename format with token substitution.
  - **location** `string`
    Archive destination URL (file://, http://, https://).
  - **on-change** `boolean`
    Time-based only: skip if config unchanged since last archive.
  - **timeout** `string`
    HTTP upload timeout, in Go duration format, for example 30s.
  - **trigger** `enumeration`
    When to archive.
- **authentication** `container`
  System authentication settings
  - **radius** `container`
    RADIUS servers for operator login over SSH, web and MCP (RFC 2865).
    - **auth-method** `enumeration`
      Credential the Access-Request carries for an operator login.
    - **default-profile** `string[]`
      Ze authorization profiles used when the Access-Accept carries no profile-attribute.
    - **profile-attribute** `enumeration`
      Access-Accept reply attribute that names the user's ze authorization profiles.
    - **retries** `uint8`
      Transmit attempts per server before failover
    - **server <address>** `list`
      RADIUS servers, tried in configured order
      - **key** `string`
        RADIUS shared secret (RFC 2865).
      - **port** `uint16`
        UDP authentication port (default 1812)
    - **source-address** `ip-address`
      Source IP for outbound RADIUS requests
    - **timeout** `uint16`
      Per-server request timeout in seconds
  - **tacacs** `container`
    TACACS+ server configuration (RFC 8907)
    - **accounting** `boolean`
      Enable command execution accounting
    - **authorization** `boolean`
      Enable per-command TACACS+ authorization
    - **server <address>** `list`
      TACACS+ servers, tried in configured order
      - **key** `string`
        Shared secret this server obfuscates its packets with
      - **port** `uint16`
        TCP port (default 49)
    - **source-address** `ip-address`
      Source IP for outbound TACACS+ connections
    - **strict-fallback** `boolean`
      Deny authorization when TACACS+ is unavailable instead of falling back to local RBAC
    - **timeout** `uint16`
      Per-server connection timeout in seconds
  - **tacacs-profile <level>** `list`
    Maps TACACS+ privilege level to ze authorization profile
    - **profile** `string[]`
      Ze authorization profiles for this privilege level. At least one is required.
  - **user <name>** `list`
    Authenticated user (local)
    - **password** `string`
      Bcrypt-hashed password, in canonical form.
    - **plaintext-password** `string`
      Write-only plaintext password.
    - **profile** `string[]`
      Authorization profile name(s) assigned to this user
    - **public-keys <name>** `list`
      SSH public keys for key-based authentication. Each entry is a named key (e.g. user@host).
      - **key** `string`
        Base64-encoded public key data.
      - **type** `enumeration`
        SSH public key algorithm.
- **authorization** `container`
  Profile-based command authorization
  - **profile <name>** `list`
    Named authorization profile with run and edit sections
    - **edit** `container`
      Configuration command authorization (write commands)
      - **default-action** `enumeration`
        Action when no entry matches
      - **entry <number>** `list`
        Ordered authorization entry
        - **action** `enumeration`
          Authorization action
        - **match** `string`
          Command path prefix or regex pattern
        - **regex** `boolean`
          If true, match is a regular expression
    - **run** `container`
      Operational command authorization (read-only commands)
      - **default-action** `enumeration`
        Action when no entry matches
      - **entry <number>** `list`
        Ordered authorization entry
        - **action** `enumeration`
          Authorization action
        - **match** `string`
          Command path prefix or regex pattern
        - **regex** `boolean`
          If true, match is a regular expression
- **commit-revisions** `uint16`
  Maximum number of config archive files kept per file:// location.
- **conntrack** `container`
  Connection tracking (conntrack) module loading, table tuning, and timeout configuration.
  - **accounting** `empty`
    Enable per-connection byte/packet counters (nf_conntrack_acct).
  - **checksum** `empty`
    Enable conntrack checksum verification (nf_conntrack_checksum).
  - **expect-max** `uint16`
    Maximum expected connections (nf_conntrack_expect_max).
  - **hash-size** `uint32`
    Conntrack hash table buckets (nf_conntrack_buckets).
  - **log-invalid** `enumeration`
    Log invalid packets for the specified protocol (nf_conntrack_log_invalid).
  - **module** `enumeration[]`
    Conntrack helper modules to load.
  - **table-size** `uint32`
    Maximum conntrack table entries (nf_conntrack_max).
  - **tcp** `container`
    TCP connection tracking behavior flags.
    - **be-liberal** `boolean`
      Accept out-of-window packets for established connections.
    - **ignore-invalid-rst** `boolean`
      Ignore RST packets with invalid sequence numbers.
    - **loose** `boolean`
      Allow tracking connections started before conntrack loaded.
    - **max-retrans** `uint8`
      Maximum retransmissions before marking connection invalid.
  - **timeout** `container`
    Connection tracking timeouts per protocol.
    - **dccp** `container`
      DCCP connection tracking timeouts.
      - **closereq** `uint32`
        Close-request timeout (seconds).
      - **closing** `uint32`
        Closing timeout (seconds).
      - **open** `uint32`
        Open timeout (seconds).
      - **partopen** `uint32`
        Partopen timeout (seconds).
      - **request** `uint32`
        Request timeout (seconds).
      - **respond** `uint32`
        Respond timeout (seconds).
      - **timewait** `uint32`
        Time-wait timeout (seconds).
    - **generic** `uint32`
      Generic timeout in seconds (nf_conntrack_generic_timeout).
    - **gre** `container`
      GRE connection tracking timeouts.
      - **stream** `uint32`
        Stream timeout (seconds).
      - **timeout** `uint32`
        Default timeout (seconds).
    - **icmp** `container`
      ICMP connection tracking timeouts.
      - **timeout** `uint32`
        Timeout (seconds).
    - **icmpv6** `container`
      ICMPv6 connection tracking timeouts.
      - **timeout** `uint32`
        Timeout (seconds).
    - **sctp** `container`
      SCTP connection tracking timeouts.
      - **closed** `uint32`
        Closed timeout (seconds).
      - **cookie-echoed** `uint32`
        Cookie-echoed timeout (seconds).
      - **cookie-wait** `uint32`
        Cookie-wait timeout (seconds).
      - **established** `uint32`
        Established timeout (seconds).
      - **heartbeat-sent** `uint32`
        Heartbeat-sent timeout (seconds).
      - **shutdown-ack-sent** `uint32`
        Shutdown-ACK-sent timeout (seconds).
      - **shutdown-recd** `uint32`
        Shutdown-received timeout (seconds).
      - **shutdown-sent** `uint32`
        Shutdown-sent timeout (seconds).
    - **tcp** `container`
      TCP connection tracking timeouts.
      - **close** `uint32`
        Close timeout (seconds).
      - **close-wait** `uint32`
        Close-wait timeout (seconds).
      - **established** `uint32`
        Established timeout (seconds).
      - **fin-wait** `uint32`
        FIN-wait timeout (seconds).
      - **last-ack** `uint32`
        Last-ACK timeout (seconds).
      - **max-retrans** `uint32`
        Max retransmission timeout (seconds).
      - **syn-recv** `uint32`
        SYN-received timeout (seconds).
      - **syn-sent** `uint32`
        SYN-sent timeout (seconds).
      - **time-wait** `uint32`
        Time-wait timeout (seconds).
      - **unacknowledged** `uint32`
        Unacknowledged timeout (seconds).
    - **udp** `container`
      UDP connection tracking timeouts.
      - **stream** `uint32`
        Stream timeout (seconds).
      - **timeout** `uint32`
        Default timeout (seconds).
  - **timestamp** `empty`
    Enable per-connection timestamps (nf_conntrack_timestamp).
- **console** `container`
  Serial console configuration for headless CPE devices.
  - **device <name>** `list`
    Serial device to configure as console.
    - **speed** `enumeration`
      Baud rate (default 115200).
- **crash-dump** `container`
  Kernel crash capture, kept across a reboot and listed by show crashes.
  - **enabled** `boolean`
    Store the kernel panic record as a crash report at the next boot.
  - **memory-image** `container`
    Full memory image capture, for a fault a backtrace does not explain.
    - **enabled** `boolean`
      Capture a full memory image as well as the panic backtrace.
    - **reserve** `uint16`
      Memory reserved for the capture kernel that writes the image.
  - **reserve** `uint16`
    Memory the kernel reserves to write its panic record into.
- **dns** `container`
  DNS resolver tuning and resolv.conf settings.
  - **cache-size** `uint32`
    Maximum cached entries (0 disables caching).
  - **cache-ttl** `uint32`
    Maximum cache TTL in seconds (0 uses response TTL only).
  - **dnssec-validation** `enumeration`
    Upstream-answer DNSSEC validation for the stub resolver.
  - **resolv-conf-path** `string`
    Path Ze writes the DNS servers to, as a resolv.conf file.
  - **timeout** `uint16`
    Query timeout in seconds.
- **domain** `string`
  System domain, supports $ENV_VAR expansion.
- **host** `string`
  System hostname, supports $ENV_VAR expansion.
- **name-server** `ip-address[]`
  Static DNS name servers, written to resolv.conf and used by ze internal resolver.
- **peeringdb** `container`
  PeeringDB API configuration for prefix data lookups.
  - **margin** `uint8`
    Percentage margin above PeeringDB count for prefix maximum (0-100).
  - **url** `string`
    PeeringDB-compatible API base URL.
- **rir** `container`
  Regional Internet Registry delegation table settings.
  - **delegation-source <registry>** `list`
    Per-registry source of the delegation file.
    - **url** `string`
      URL Ze reads this registry's delegation file from.
- **tuning** `container`
  Runtime hardware tuning applied at startup and on config commit.
  - **cpu** `container`
    CPU frequency scaling tuning.
    - **governor** `enumeration`
      CPU scaling governor applied to all cores.
  - **ethtool <interface>** `list`
    Per-interface ethtool settings.
    - **ring** `container`
      Ring buffer sizes.
      - **rx** `uint16`
        Receive ring buffer size.
      - **tx** `uint16`
        Transmit ring buffer size.
  - **irq-affinity <interface>** `list`
    Pin NIC IRQs to specific CPUs.
    - **cpus** `string`
      CPU list, for example 0,2,4-7.
- **update-check** `container`
  System version check and platform update settings.
  - **auto-apply** `boolean`
    Download, verify, and stage a Ze binary update without an operator.
  - **interval** `uint32`
    Check interval in seconds (default 86400 = daily, minimum 60).
  - **maintenance-window** `container`
    Time window when binary replacement may occur. Download and verification proceed at any time.
    - **end** `string`
      End time HH:MM in local time.
    - **start** `string`
      Start time HH:MM in local time.
  - **restart** `container`
    What Ze does after it stages an update.
    - **immediate** `empty`
      Restart automatically after staging (5s drain delay). Mutually exclusive with time.
    - **time** `string`
      Daily restart time, HH:MM in local time. Mutually exclusive with immediate.
  - **spread** `uint32`
    Maximum random delay in seconds before the download starts.
  - **url** `string`
    HTTPS URL that serves a JSON object with a 'version' field.

## telemetry

Telemetry export configuration.

- **prometheus** `container`
  Prometheus metrics HTTP export.
  - **basic-auth** `container`
    HTTP Basic Authentication for the Prometheus service.
    - **enabled** `boolean`
      Require HTTP Basic Authentication on the metrics and health endpoints.
    - **password** `string`
      Bcrypt-hashed HTTP Basic Authentication password. Write plaintext via plaintext-password.
    - **plaintext-password** `string`
      Write-only password.
    - **realm** `string`
      HTTP Basic Authentication realm.
    - **username** `string`
      HTTP Basic Authentication username.
  - **collector <name>** `list`
    Deprecated: use netdata/collector. Per-Netdata-collector overrides (enable/disable, interval).
    - **enabled** `boolean`
      Enable or disable this collector.
    - **interval** `uint16`
      Override sampling interval for this collector (seconds). Inherits global interval if not set.
  - **enabled** `boolean`
    Enable the Prometheus metrics endpoint.
  - **interval** `uint16`
    Deprecated: use netdata/interval. Netdata-compatible OS collector sampling interval in seconds.
  - **netdata** `container`
    Netdata-compatible OS collector metrics. These settings do not affect Ze-native metrics.
    - **collector <name>** `list`
      Per-Netdata-collector overrides of the enable state and the interval.
      - **enabled** `boolean`
        Enable or disable this Netdata-compatible OS collector.
      - **interval** `uint16`
        Override the sampling interval of this collector, in seconds.
    - **enabled** `boolean`
      Enable the Netdata-compatible OS collectors.
    - **interval** `uint16`
      Netdata-compatible OS collector sampling interval in seconds.
    - **prefix** `string`
      Metric name prefix for Netdata-compatible OS collector metrics only.
  - **path** `string`
    HTTP path of the metrics endpoint.
  - **prefix** `string`
    Deprecated: use netdata/prefix.
  - **server <name>** `list`
    Prometheus listen endpoints.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned

## traffic

Traffic subsystem: QoS control and byte-usage accounting.

- **control** `container`
  *Provided by `traffic` ([ze-traffic-control-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/traffic/yang/ze-traffic-control-conf.yang))*
  Traffic control configuration of each interface.
  - **backend** `string`
    Traffic control backend that programs the queueing disciplines.
  - **interface <name>** `list`
    Traffic control of one named interface.
    - **qdisc** `container`
      The root queueing discipline of the interface.
      - **class <name>** `list`
        One traffic class of the queueing discipline.
        - **ceil** `rate-bps`
          Largest rate this class can reach.
        - **match <type>** `list`
          A filter that classifies a packet into this class.
          - **value** `string`
            Value the filter matches.
        - **priority** `uint8`
          Scheduling priority of this class.
        - **rate** `rate-bps`
          Rate this class is guaranteed.
      - **default-class** `string`
        Name of the class that takes the unclassified traffic.
      - **type** `qdisc-type`
        Type of the queueing discipline.
- **usage** `container`
  *Provided by `traffic-usage` ([ze-traffic-usage-cmd.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/trafficusage/yang/ze-traffic-usage-cmd.yang), [ze-traffic-usage-conf.yang](https://github.com/ze-software/ze/blob/main/internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang))*
  Per-interface eBPF TCX byte accounting (port/protocol always; per-IP via track-ip).
  - **enabled** `boolean`
    Enable traffic-usage accounting.
  - **interfaces** `container`
    Interfaces to account on.
    - **interface <name>** `list`
      Per-interface traffic-usage accounting.
      - **enabled** `boolean`
        Account traffic on this interface.
      - **max-entries** `uint32`
        Override the global max-entries (per-map LRU capacity) for this interface (inherits when unset).
      - **stale-timeout** `uint32`
        Override the global stale-timeout for this interface, in milliseconds.
      - **track-ip** `boolean`
        Override the global track-ip for this interface (inherits the global value when unset).
  - **interval** `uint32`
    Map poll interval in milliseconds (100ms..1h).
  - **max-entries** `uint32`
    Per-map LRU capacity; least-recently-used entries are evicted beyond this, keeping top talkers.
  - **stale-timeout** `uint32`
    Remove a metric series unseen within this many milliseconds; 0 disables cleanup.
  - **track-ip** `boolean`
    Account bytes per IPv4 address as well as per interface.

## vpn

*Provided by `ike`*

VPN subsystems.

- **ipsec** `container`
  IPsec VPN configuration.
  - **cookie-threshold** `uint32`
    Number of half-open IKE SAs tolerated before a COOKIE challenge.
  - **esp-group <name>** `list`
    ESP (Encapsulating Security Payload) group.
    - **lifetime** `uint32`
      SA lifetime in seconds. 0 disables expiry.
    - **pfs** `enumeration`
      Perfect Forward Secrecy for Child SA rekeying.
    - **proposal <number>** `list`
      ESP proposal, where a lower number is a higher priority.
      - **encryption** `encryption-algo`
        Encryption algorithm.
      - **hash** `hash-algo`
        Integrity algorithm.
  - **ike-group <name>** `list`
    IKE (Internet Key Exchange) group.
    - **close-action** `enumeration`
      Action when peer closes the IKE SA.
    - **dead-peer-detection** `container`
      Dead Peer Detection (DPD) settings (RFC 3706).
      - **action** `enumeration`
        Action when DPD detects a dead peer.
      - **interval** `uint16`
        DPD probe interval in seconds.
      - **timeout** `uint16`
        DPD timeout in seconds before declaring peer dead.
    - **key-exchange** `enumeration`
      IKE protocol version.
    - **lifetime** `uint32`
      IKE SA lifetime in seconds, where 0 starts no rekey.
    - **proposal <number>** `list`
      IKE proposal, where a lower number is a higher priority.
      - **dh-group** `uint8`
        Diffie-Hellman group number (RFC 7296 Section 3.3.2).
      - **encryption** `encryption-algo`
        Encryption algorithm.
      - **hash** `hash-algo`
        PRF and integrity algorithm.
  - **interface** `string`
    WAN interface for IPsec traffic.
  - **policy <name>** `list`
    Operator-authored Security Policy Database entry.
    - **action** `enumeration`
      What the entry does with the traffic its selector matches.
    - **direction** `enumeration`
      Which side of the boundary the entry acts on.
    - **local** `container`
      The local side of the selector.
      - **port** `union`
        Local port of the selector.
      - **prefix** `ip-prefix`
        Local traffic the entry matches.
    - **order** `uint32`
      Rank of this entry, lowest searched first.
    - **protocol** `uint8`
      IP protocol number the selector matches, 0 for every protocol.
    - **remote** `container`
      The remote side of the selector.
      - **port** `union`
        Remote port of the selector.
      - **prefix** `ip-prefix`
        Remote traffic the entry matches.
  - **remote-access** `container`
    Remote access VPN for road warrior clients (EAP).
    - **authentication** `container`
      EAP authentication settings for remote access.
      - **mode** `enumeration`
        EAP authentication method.
      - **x509** `container`
        Server certificate references for EAP.
        - **ca-certificate** `string`
          Name of the CA certificate in the PKI store. It has no effect today.
        - **certificate** `string`
          Name of the server certificate in the PKI store. It has no effect today.
    - **eap-user <name>** `list`
      EAP user entry for remote access authentication.
      - **certificate** `string`
        Client certificate name in PKI store. Required for eap-tls.
      - **password** `string`
        User password ($9$-encoded). Required for eap-mschapv2.
    - **esp-group** `string`
      Reference to an ESP group name.
    - **ike-group** `string`
      Reference to an IKE group name.
    - **pool <name>** `list`
      Virtual IP pool for client address assignment.
      - **dns** `string`
        DNS server pushed to clients via IKEv2 Configuration Payload.
      - **domain** `string`
        Search domain pushed to clients.
      - **range** `string`
        IPv4 CIDR for client addresses (e.g. 10.10.0.0/24). Prefix length /8-/30.
      - **range6** `string`
        IPv6 CIDR for client addresses (e.g. fd00::/64). Prefix length /48-/126.
  - **site-to-site** `container`
    Site-to-site VPN peers.
    - **peer <name>** `list`
      Remote VPN peer.
      - **authentication** `container`
        Peer authentication settings.
        - **ca-certificate** `string`
          Name of the CA certificate in the PKI store.
        - **certificate** `string`
          Name of the device certificate in the PKI store.
        - **certificate-count** `uint8`
          The most X.509 certificates Ze sends to this peer and accepts from it.
        - **certificate-status-request** `boolean`
          Check this peer's EAP-TLS certificate chain by the OCSP response it staples.
        - **certificate-url** `string`
          The http URL at which this device publishes its own certificate.
        - **certificate-url-allow** `ip-prefix[]`
          Extra destinations a certificate URL from this peer CAN resolve to.
        - **hash-and-url** `boolean`
          Use the Hash and URL certificate encodings of RFC 7296 Section 3.6.
        - **local-id** `string`
          Local identity for IKE negotiation.
        - **mode** `enumeration`
          Authentication mode.
        - **pre-shared-secret** `string`
          Pre-shared key ($9$-encoded).
        - **pre-shared-secret-encoding** `enumeration`
          How to read the pre-shared-secret value.
        - **remote-id** `string`
          Identity the remote endpoint must assert.
        - **remote-id-type** `enumeration`
          The one IKE ID type the remote endpoint CAN assert.
        - **session-resumption** `boolean`
          Let an EAP-TLS exchange with this peer resume an earlier one.
        - **x509** `container`
          X.509 certificate references, deprecated in favor of the direct leaves.
          - **ca-certificate** `string`
            Name of the CA certificate in the PKI store.
          - **certificate** `string`
            Name of the device certificate in the PKI store.
      - **connection-type** `enumeration`
        Whether to initiate or wait for the remote peer.
      - **esp-group** `string`
        Reference to an ESP group name.
      - **ike-group** `string`
        Reference to an IKE group name.
      - **local-address** `string`
        Local endpoint IP address.
      - **mode** `enumeration`
        Encapsulation mode for this peer's Child SAs.
      - **policy-priority** `uint32`
        Rank of this peer's Security Policy Database entries, lowest searched first.
      - **remote-address** `string`
        Remote endpoint IPv4 address or DNS hostname.
      - **traffic-selector <number>** `list`
        The traffic this peer's Child SAs carry.
        - **local** `container`
          The local side of the selector pair (TSi when Ze initiates, TSr when Ze responds).
          - **port** `union`
            Local port of the selector.
          - **prefix** `ip-prefix`
            Local traffic the Child SA carries.
        - **protocol** `uint8`
          IP protocol number the selector matches.
        - **remote** `container`
          The remote side of the selector pair (TSr when Ze initiates, TSi when Ze responds).
          - **port** `union`
            Remote port of the selector.
          - **prefix** `ip-prefix`
            Remote traffic the Child SA carries.
      - **transport-required** `boolean`
        Delete the SA when the peer declines transport mode.
      - **vti** `container`
        Virtual Tunnel Interface binding.
        - **bind** `string`
          VTI interface name to bind this peer's traffic to.

## vpp

*Provided by `vpp` ([ze-vpp-conf.yang](https://github.com/ze-software/ze/blob/main/internal/component/vpp/yang/ze-vpp-conf.yang))*

VPP data plane configuration.

- **api-socket** `string`
  GoVPP binary API Unix socket path.
- **cpu** `container`
  VPP CPU pinning configuration.
  - **main-core** `uint8`
    CPU core for VPP main thread. Omit for automatic assignment.
  - **poll-sleep** `string`
    Fixed sleep between VPP main-loop polls, in whole milliseconds.
  - **worker-cores** `string`
    Explicit CPU core list for the VPP worker threads.
  - **workers** `uint8`
    Number of VPP worker threads. Omit for automatic (one per available core).
- **dpdk** `container`
  DPDK NIC configuration.
  - **interface <pci-address>** `list`
    DPDK-managed network interface.
    - **name** `string`
      Short interface name (e.g. xe0, e1). Used in ze CLI and config.
    - **rx-queues** `uint8`
      Number of receive queues. Omit for VPP default.
    - **tx-queues** `uint8`
      Number of transmit queues. Omit for VPP default.
- **enabled** `boolean`
  Enable VPP integration, so Ze manages the VPP lifecycle.
- **external** `boolean`
  Let an external supervisor own the VPP process, false by default.
- **lcp** `container`
  Linux Control Plane plugin settings.
  - **auto-subint** `boolean`
    Auto-create sub-TAPs for dot1q/QinQ sub-interfaces.
  - **enabled** `boolean`
    Enable LCP plugin in VPP.
  - **netns** `string`
    Network namespace for LCP TAP interfaces. Routing daemons run in this namespace.
  - **sync** `boolean`
    Sync VPP state changes (link, MTU, IP) to Linux TAP mirrors.
- **memory** `container`
  VPP memory and buffer configuration.
  - **buffers** `uint32`
    Number of packet buffers per NUMA node. 128000 is proven for full DFZ at 10G.
  - **hugepage-size** `enumeration`
    Hugepage size for VPP buffers.
  - **main-heap** `string`
    VPP main heap size (e.g. 512M, 1G, 1536M). Production with full DFZ: 1536M.
- **plugins** `container`
  Optional VPP plugins to load.
  - **wireguard** `boolean`
    Load wireguard_plugin.so for the vpp interface backend.
- **stats** `container`
  VPP stats segment configuration.
  - **poll-interval** `uint16`
    Stats polling interval in seconds.
  - **segment-size** `string`
    Stats segment shared memory size.
  - **socket-path** `string`
    Stats segment Unix socket path.
