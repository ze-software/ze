# Configuration Reference

The complete Ze configuration as one tree: 36 top-level sections (27 provided by plugins, the rest core), generated live from the YANG schema with `ze yang tree`. This is about the structure of the configuration -- every section, searchable and inspectable. See [the Configuration guide](https://ze-software.net/docs/features/configuration/) for a narrative walkthrough of BGP peer config specifically.

## anomaly

Behavioral anomaly detection and response subsystem.

- **detect** `container`
  *Provided by `anomaly-detect-feature-source` ([ze-anomaly-detect-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang))*
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
    Discount applied to corroborating features when combining scores, so correlated features do not double-count.
  - **deviation-threshold** `decimal-2`
    Sigma at or above which a per-entity feature deviation fires.
  - **enabled** `boolean`
    Enable the behavioral anomaly detector (report-only; emits incidents, takes no action).
  - **min-cohort-size** `uint16`
    Minimum cohort (source-prefix bucket) members before peer-group rarity is scored; smaller cohorts fall back to self-deviation only.
  - **min-features-to-correlate** `uint8`
    Minimum distinct features that must fire on one entity/window before an incident is scored (weak-signal correlation gate).
- **shape** `container`
  *Provided by `anomaly-shape-firewall` ([ze-anomaly-shape-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/anomaly/shape/yang/ze-anomaly-shape-conf.yang))*
  Autonomous responder: shadow (log-only) or armed (live per-source firewall actions).
  - **action** `enumeration`
    Armed action: rate-limit the source (surgical) or drop it (fallback).
  - **allowlist** `ip-prefix[]`
    Protected source prefixes that are never armed (self-lockout guard for management / control-plane sources).
  - **auto-revert-ttl** `uint16`
    Seconds after the last signal before an armed action auto-reverts, regardless of any clear event (safety ceiling).
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
    shadow (default): log the would-be action, install nothing. armed: install live per-source firewall actions.

## bfd

*Provided by `bfd` ([ze-bfd-api.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bfd/yang/ze-bfd-api.yang), [ze-bfd-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bfd/yang/ze-bfd-cmd.yang), [ze-bfd-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bfd/yang/ze-bfd-conf.yang))*

BFD control configuration for Ze.

- **bind-v6** `boolean`
  When true, the BFD plugin also binds an IPv6 socket (::0) alongside the IPv4 socket for every (vrf, mode) loop. The paired sockets share the same RX channel via a transport.Dual wrapper and send routes by destination address family. Stage 2b (spec-bfd-2b-ipv6-transport) ships this leaf; pinned IPv6 sessions require it to be set explicitly so operators who do not need v6 do not open a second socket per loop.
- **enabled** `boolean`
  Master switch for the BFD plugin.
- **multi-hop-session <peer local vrf>** `list`
  Multi-hop BFD session (RFC 5883). Port 4784, no GTSM.
  - **local** `string`
    Local source address. Required for multi-hop.
  - **min-ttl** `uint8`
    Minimum acceptable TTL on receive. Weak replacement for GTSM on multi-hop paths.
  - **peer** `string`
  - **profile** `string`
  - **shutdown** `boolean`
  - **vrf** `string`
- **persist-dir** `string`
  DEPRECATED (retained for back-compat; scheduled for removal). The value is no longer interpreted as a directory: TX sequence numbers for Meticulous Keyed authentication now persist to the shared managed state store (database.zefs) instead of a loose file. A non-empty value still opts a session into persistence. RFC 5880 Section 6.7.3 requires the sequence to survive process restarts; without this leaf set, Meticulous sessions still work at runtime but a fresh process loses its sequence floor until the peer's replay window slides forward.
- **profile <name>** `list`
  Reusable timer and feature profile. Sessions reference a profile by name and inherit every field below.
  - **auth** `container`
    RFC 5880 Section 6.7 authentication parameters. Sessions inheriting this profile sign every outgoing Control packet with the configured type and verify incoming packets using the same key. Simple Password (type 1) is rejected at parse time -- it provides no cryptographic protection and RFC 5880 warns against using it.
    - **key-id** `uint8`
      Auth Key ID (RFC 5880 Section 6.7.1).
    - **secret** `string`
      Shared secret for the keyed digest. Redacted from `ze config show` output. MD5 variants use the first 16 bytes of the secret, SHA1 variants the first 20.
    - **type** `enumeration`
      Authentication type. Simple Password is intentionally absent from the enum.
  - **desired-min-tx-us** `uint32`
    Local target transmit rate in microseconds. The slow-start floor of 1 000 000 us applies while the session is not Up, per RFC 5880 Section 6.8.3.
  - **detect-multiplier** `uint8`
    Number of consecutive missed Control packets that trigger a Down transition. RFC 5880 Section 6.8.4.
  - **echo** `container`
    RFC 5880 Section 6.4 / RFC 5881 Section 5 Echo mode. Single-hop only -- RFC 5883 Section 4 explicitly prohibits multi-hop echo, and the parser rejects echo on a multi-hop session. When active, the engine sends echo packets on UDP port 3785 at the peer's advertised RequiredMinEchoRxInterval and slows its async Control TX to the peer's RequiredMinRxInterval.
    - **desired-min-echo-tx-us** `uint32`
      Local target echo transmit rate in microseconds. The effective echo rate is max(local desired, peer RequiredMinEchoRx).
  - **passive** `boolean`
    Active (default) transmits from session creation. Passive transmits nothing until a Control packet arrives from the peer. RFC 5883 Section 4.3.
  - **required-min-rx-us** `uint32`
    Minimum inter-packet gap the local end can handle, in microseconds.
- **single-hop-session <peer vrf interface>** `list`
  Per-link BFD session (RFC 5881). Port 3784, TTL=255. Protocol clients should NOT use this list; they call the plugin Service interface at runtime.
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

## bgp

*Provided by `bgp` ([ze-bgp-api.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/yang/ze-bgp-api.yang), [ze-bgp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/yang/ze-bgp-conf.yang)); `bgp-bmp` ([ze-bmp-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-cmd.yang), [ze-bmp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang)); `bgp-filter-aspath` ([ze-filter-aspath.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_aspath/yang/ze-filter-aspath.yang)); `bgp-filter-aspath-length` ([ze-filter-aspath-length.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_aspath_length/yang/ze-filter-aspath-length.yang)); `bgp-filter-community` ([ze-filter-community.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang)); `bgp-filter-community-match` ([ze-filter-community-match.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_community_match/yang/ze-filter-community-match.yang)); `bgp-filter-family` ([ze-filter-family.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_family/yang/ze-filter-family.yang)); `bgp-filter-irr` ([ze-filter-irr-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang), [ze-filter-irr.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang)); `bgp-filter-modify` ([ze-filter-modify.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang)); `bgp-filter-prefix` ([ze-filter-prefix.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_prefix/yang/ze-filter-prefix.yang)); `bgp-filter-remove-private-as` ([ze-filter-remove-private-as.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/filter_remove_private_as/yang/ze-filter-remove-private-as.yang)); `bgp-gr` ([ze-graceful-restart.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang)); `bgp-healthcheck` ([ze-healthcheck-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/healthcheck/yang/ze-healthcheck-conf.yang)); `bgp-hostname` ([ze-hostname.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/hostname/yang/ze-hostname.yang)); `bgp-llnh` ([ze-link-local-nexthop.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang)); `bgp-rib` ([ze-rib-api.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/rib/yang/ze-rib-api.yang), [ze-rib.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/rib/yang/ze-rib.yang)); `bgp-role` ([ze-role.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/role/yang/ze-role.yang)); `bgp-route-refresh` ([ze-refresh-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/route_refresh/yang/ze-refresh-cmd.yang), [ze-route-refresh-api.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/route_refresh/yang/ze-route-refresh-api.yang), [ze-route-refresh.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/route_refresh/yang/ze-route-refresh.yang)); `bgp-rpki` ([ze-rpki.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/rpki/yang/ze-rpki.yang)); `bgp-rs` ([ze-rs-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang)); `bgp-softver` ([ze-softver.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/softver/yang/ze-softver.yang)); `bgp-watchdog`*

Border Gateway Protocol routing configuration. Peers inherit from group defaults; groups inherit from this global level.

- **admin-distance** `container`
  Classical admin distance stamped on BGP best-paths when they are mirrored into the shared Loc-RIB. Lower distance wins against other protocols for the same prefix. Defaults follow the Cisco/Juniper convention; RFC 4271 does not mandate values.
  - **ebgp** `uint8`
    Admin distance for routes learned from external BGP peers.
  - **ibgp** `uint8`
    Admin distance for routes learned from internal BGP peers.
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
      Enable Loc-RIB monitoring (RFC 9069, PeerType=3): stream local RIB best-path changes to collectors as Route Monitoring messages with a Loc-RIB peer header
    - **route-mirroring** `boolean`
      Enable Route Mirroring (RFC 7854 S4.7): stream verbatim copies of all BGP messages to collectors
    - **route-monitoring-policy** `enumeration`
      Which routes to stream: pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out, RFC 8671), or all
    - **statistics-timeout** `uint16`
      Interval for periodic statistics reports (0 = disabled)
- **community** `container`
  Named community definitions referenced by filter rules
  - **extended <name>** `list`
    - **value** `string[]`
      Extended community values
  - **large <name>** `list`
    - **value** `string[]`
      Large community values (GA:LD1:LD2 format)
  - **standard <name>** `list`
    - **value** `string[]`
      Standard community values (ASN:value format)
- **filter** `container`
  Global route filter chains for import and export. Group and peer levels append to this base chain. Names reference filter instances defined in bgp/policy.
  - **egress** `container`
    Egress direction filters
    - **community** `container`
      Community filter for egress
      - **strip** `string[]`
        Named communities to remove on egress
      - **tag** `string[]`
        Named communities to add on egress
  - **export** `string[]`
    Global export filter chain
  - **import** `string[]`
    Global import filter chain
  - **ingress** `container`
    Ingress direction filters
    - **community** `container`
      Community filter for ingress
      - **strip** `string[]`
        Named communities to remove on ingress
      - **tag** `string[]`
        Named communities to add on ingress
- **group <name>** `list`
  Peer group - defines shared defaults for member peers
  - **behavior** `container`
    Operational knobs that control how the reactor processes and forwards UPDATE messages for this peer. Most users can leave these at defaults.
    - **auto-flush** `boolean`
      Automatically withdraw all routes advertised to this peer when the session goes down. When disabled, routes remain until explicitly withdrawn or the hold timer expires. Legacy ExaBGP option, retained for migration.
    - **group-updates** `boolean`
      Pack multiple NLRI into a single UPDATE message when they share the same path attributes. Reduces the number of UPDATE messages from O(routes) to O(unique-attribute-sets). Disable for peers that require one prefix per UPDATE.
    - **manual-eor** `boolean`
      Do not send End-of-RIB automatically after initial route advertisement. When enabled, End-of-RIB must be triggered externally via the process API. Used when an external controller manages convergence signaling.
    - **rs-fast-path** `boolean`
      Forward received UPDATEs directly inside the reactor for RS-client peers, bypassing the plugin dispatch chain. Lower latency for route server forwarding. Peers with export filters are excluded and use the normal path.
  - **connection** `container`
    Transport-level connection settings
    - **bfd** `container`
      Bidirectional Forwarding Detection (RFC 5880) options for this peer. When the container is present, the BGP reactor calls the BFD plugin's Service interface on session establishment and tears the BGP session down when BFD reports Down. The BFD plugin must be loaded (a top-level bfd { ... } block); if it is not, the BGP peer starts without BFD and logs a warning.
      - **enabled** `boolean`
        Master switch for this peer's BFD session. Set false to keep the config in place but suspend the BFD client (useful for maintenance).
      - **interface** `string`
        Single-hop egress interface. Optional: when omitted, the BFD plugin derives the interface from the peer's local address. Ignored for multi-hop.
      - **min-ttl** `uint8`
        Multi-hop minimum acceptable TTL (RFC 5883 §5). Ignored for single-hop. Zero means use the plugin default (254).
      - **mode** `enumeration`
        Hop mode. Single-hop is the common case for an eBGP peer on a direct link; multi-hop is for iBGP over an IGP or any peering that crosses more than one IP hop.
      - **profile** `string`
        Name of a profile defined under the top-level bfd { profile ... } block. The referenced profile supplies detect-multiplier, desired-min-tx-us, and required-min-rx-us. Empty means use the BFD plugin defaults.
    - **link-local** `boolean`
      Auto-discover IPv6 link-local address for TCP connection
    - **local** `container`
      Local endpoint for the TCP session. Controls the bind address, port, and whether to accept inbound connections.
      - **accept** `boolean`
        Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Local address for connection (use IP address or 'auto')
      - **port** `port`
        Local bind port
    - **md5** `container`
      TCP MD5 authentication (RFC 2385)
      - **ip** `ip-address`
        MD5 authentication IP
      - **password** `string`
        MD5 authentication password
    - **remote** `container`
      Remote endpoint for the TCP session. Controls the peer address, port, outbound connection initiation, and dynamic peer ranges.
      - **connect** `boolean`
        Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Peer IP address or 'dynamic' for dynamic peer groups
      - **max-peers** `uint32`
        Maximum number of dynamic peers for this group
      - **port** `port`
        Remote connection port
      - **range** `ip-prefix[]`
        IP prefix ranges for dynamic peer groups. Only meaningful when ip is 'dynamic'. Connections from IPs within these ranges create dynamic peers.
    - **ttl** `container`
      TTL settings for BGP sessions
      - **max** `uint8`
        TTL security / GTSM (RFC 5082)
      - **min** `uint8`
        Minimum incoming TTL
      - **set** `uint8`
        Outgoing TTL value
  - **description** `string`
    Free-text label for this peer. Shown in the CLI, web UI, and logs. Typically the peer's role or location (e.g., 'upstream-transit-provider').
  - **filter** `container`
    Route filter chains for import and export. Names reference filter instances defined in bgp/policy.
    - **egress** `container`
      Egress direction filters
      - **community** `container`
        Community filter for egress
        - **strip** `string[]`
          Named communities to remove on egress
        - **tag** `string[]`
          Named communities to add on egress
    - **export** `string[]`
      Export filter chain (filter instance names)
    - **import** `string[]`
      Import filter chain (filter instance names)
    - **ingress** `container`
      Ingress direction filters
      - **community** `container`
        Community filter for ingress
        - **strip** `string[]`
          Named communities to remove on ingress
        - **tag** `string[]`
          Named communities to add on ingress
  - **peer <name>** `list`
    BGP peer in this group
    - **behavior** `container`
      Operational knobs that control how the reactor processes and forwards UPDATE messages for this peer. Most users can leave these at defaults.
      - **auto-flush** `boolean`
        Automatically withdraw all routes advertised to this peer when the session goes down. When disabled, routes remain until explicitly withdrawn or the hold timer expires. Legacy ExaBGP option, retained for migration.
      - **group-updates** `boolean`
        Pack multiple NLRI into a single UPDATE message when they share the same path attributes. Reduces the number of UPDATE messages from O(routes) to O(unique-attribute-sets). Disable for peers that require one prefix per UPDATE.
      - **manual-eor** `boolean`
        Do not send End-of-RIB automatically after initial route advertisement. When enabled, End-of-RIB must be triggered externally via the process API. Used when an external controller manages convergence signaling.
      - **rs-fast-path** `boolean`
        Forward received UPDATEs directly inside the reactor for RS-client peers, bypassing the plugin dispatch chain. Lower latency for route server forwarding. Peers with export filters are excluded and use the normal path.
    - **connection** `container`
      Transport-level connection settings
      - **bfd** `container`
        Bidirectional Forwarding Detection (RFC 5880) options for this peer. When the container is present, the BGP reactor calls the BFD plugin's Service interface on session establishment and tears the BGP session down when BFD reports Down. The BFD plugin must be loaded (a top-level bfd { ... } block); if it is not, the BGP peer starts without BFD and logs a warning.
        - **enabled** `boolean`
          Master switch for this peer's BFD session. Set false to keep the config in place but suspend the BFD client (useful for maintenance).
        - **interface** `string`
          Single-hop egress interface. Optional: when omitted, the BFD plugin derives the interface from the peer's local address. Ignored for multi-hop.
        - **min-ttl** `uint8`
          Multi-hop minimum acceptable TTL (RFC 5883 §5). Ignored for single-hop. Zero means use the plugin default (254).
        - **mode** `enumeration`
          Hop mode. Single-hop is the common case for an eBGP peer on a direct link; multi-hop is for iBGP over an IGP or any peering that crosses more than one IP hop.
        - **profile** `string`
          Name of a profile defined under the top-level bfd { profile ... } block. The referenced profile supplies detect-multiplier, desired-min-tx-us, and required-min-rx-us. Empty means use the BFD plugin defaults.
      - **link-local** `boolean`
        Auto-discover IPv6 link-local address for TCP connection
      - **local** `container`
        Local endpoint for the TCP session. Controls the bind address, port, and whether to accept inbound connections.
        - **accept** `boolean`
          Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)
        - **ip** `union`
          Local address for connection (use IP address or 'auto')
        - **port** `port`
          Local bind port
      - **md5** `container`
        TCP MD5 authentication (RFC 2385)
        - **ip** `ip-address`
          MD5 authentication IP
        - **password** `string`
          MD5 authentication password
      - **remote** `container`
        Remote endpoint for the TCP session. Controls the peer address, port, outbound connection initiation, and dynamic peer ranges.
        - **connect** `boolean`
          Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)
        - **ip** `union`
          Peer IP address or 'dynamic' for dynamic peer groups
        - **max-peers** `uint32`
          Maximum number of dynamic peers for this group
        - **port** `port`
          Remote connection port
        - **range** `ip-prefix[]`
          IP prefix ranges for dynamic peer groups. Only meaningful when ip is 'dynamic'. Connections from IPs within these ranges create dynamic peers.
      - **ttl** `container`
        TTL settings for BGP sessions
        - **max** `uint8`
          TTL security / GTSM (RFC 5082)
        - **min** `uint8`
          Minimum incoming TTL
        - **set** `uint8`
          Outgoing TTL value
    - **description** `string`
      Free-text label for this peer. Shown in the CLI, web UI, and logs. Typically the peer's role or location (e.g., 'upstream-transit-provider').
    - **filter** `container`
      Route filter chains for import and export. Names reference filter instances defined in bgp/policy.
      - **egress** `container`
        Egress direction filters
        - **community** `container`
          Community filter for egress
          - **strip** `string[]`
            Named communities to remove on egress
          - **tag** `string[]`
            Named communities to add on egress
      - **export** `string[]`
        Export filter chain (filter instance names)
      - **import** `string[]`
        Import filter chain (filter instance names)
      - **ingress** `container`
        Ingress direction filters
        - **community** `container`
          Community filter for ingress
          - **strip** `string[]`
            Named communities to remove on ingress
          - **tag** `string[]`
            Named communities to add on ingress
    - **process <name>** `list`
      External process that receives BGP events and can inject messages. Ze spawns the process and communicates via stdin/stdout JSON.
      - **content** `container`
        Controls the encoding and filtering of events sent to this process.
        - **attribute** `string`
          Filter expression to select which BGP attributes are included in events.
        - **encoding** `string`
          Wire encoding for events sent to the process (e.g., json, text).
        - **format** `string`
          Output format template for event rendering.
      - **neighbor-changes** `container`
        Send session state change events (up/down/reset) to this process. Enables the process to react to peer lifecycle transitions.
      - **processes** `string[]`
        Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.
      - **processes-match** `string[]`
        Legacy ExaBGP process match patterns for event filtering.
      - **receive** `string[]`
        Event types to receive. Base types: update, open, notification, keepalive, refresh, state, sent, negotiated. Plugins may register additional types (e.g., update-rpki). List types explicitly; 'all' is not accepted. Validated at runtime against registered event types.
      - **run** `string`
        Shell command to spawn the external process. Ze pipes BGP events to its stdin and reads commands from its stdout.
      - **send** `string[]`
        Message types to send. Valid: update, refresh, enhanced-refresh (BORR/EORR markers, RFC 7313). Validated at runtime against known send types.
    - **rib** `container`
      Route Information Base settings for this peer. Controls which RIB tables are maintained and how outbound route batching works.
      - **adj** `container`
        Adjacency RIB storage. These tables hold the raw routes before and after policy processing, enabling route-refresh and soft-reconfiguration.
        - **in** `boolean`
          Store received routes in Adj-RIB-In before policy. Enables soft reconfiguration inbound (re-apply import filters without session reset). Uses additional memory proportional to received route count.
        - **out** `boolean`
          Store advertised routes in Adj-RIB-Out after policy. Required for route-refresh (RFC 2918) to re-send routes on peer request. Uses additional memory proportional to advertised route count.
      - **out** `container`
        Outbound RIB batching settings. Control how route changes are grouped and flushed to the peer.
        - **auto-commit-delay** `uint32`
          Milliseconds to wait before flushing pending route changes to the peer. Allows batching rapid successive changes into fewer UPDATEs. 0 means flush immediately on each change.
        - **group-updates** `boolean`
          Pack routes with identical attributes into a single UPDATE. Same as the behavior-level group-updates but scoped to this peer's Adj-RIB-Out.
        - **max-batch-size** `int32`
          Maximum number of route changes to include in a single flush cycle. 0 means no limit (flush all pending changes at once). Limits per-cycle CPU cost at the expense of convergence latency.
    - **role** `container`
      RFC 9234 BGP Role configuration
      - **export** `export-token[]`
        Controls which destination peer roles may receive routes from this peer
      - **import** `role-type`
        Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)
      - **strict** `boolean`
        Require peer to send Role capability
    - **rpki** `container`
      Per-peer/group RPKI action overrides (peer > group > global, resolved per leaf)
      - **action** `container`
        Origin-validation action overrides for this peer/group
        - **invalid** `validation-action`
          Action for routes with Invalid validation state (unset: inherit group then global)
        - **not-found** `validation-action`
          Action for routes with NotFound validation state (unset: inherit group then global)
      - **aspa** `container`
        ASPA action overrides for this peer/group (only applies when ASPA validation is enabled globally)
        - **action** `container`
          - **invalid** `validation-action`
            Action for routes with ASPA Invalid path state (unset: inherit group then global)
          - **unknown** `validation-action`
            Action for routes with ASPA Unknown path state (unset: inherit group then global)
    - **session** `container`
      BGP session parameters: ASN, capabilities, address families, next-hop policy, and community control. Inherited from group to peer; peer values override.
      - **accept-srv6-prefix-sid** `boolean`
        Accept BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer. RFC 8669 Section 4: PrefixSID from EBGP outside the SR domain MUST be discarded unless configured to accept. Has no effect on IBGP (always accepted).
      - **as-override** `boolean`
        Replace peer's ASN with local ASN in outbound AS_PATH. Used in VPN/multi-site where the same customer ASN appears at multiple sites.
      - **asn** `container`
        AS number configuration
        - **local** `asn`
          Local AS (overrides global)
        - **local-options** `enumeration[]`
          Modifiers for local-as behavior. no-prepend: real ASN not prepended before local-as. replace-as: local-as replaces real ASN entirely. Both together: full replacement with no prepend.
        - **remote** `asn`
          Peer Autonomous System Number
      - **capability** `container`
        BGP capability negotiation
        - **add-path** `container`
          ADD-PATH capability (RFC 7911) with optional PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit). Default direction applies to all negotiated multiprotocol families. Per-family entries override.
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
            Default maximum paths per prefix (PATHS-LIMIT). Inherited by all families unless overridden per-family.
        - **asn4** `boolean`
          Advertise 4-byte ASN capability (RFC 6793). Required for ASNs above 65535. Disable only for peers running legacy 2-byte-only implementations.
        - **extended-message** `container`
          Extended Message capability (RFC 8654). Raises the BGP message size limit from 4096 to 65535 bytes. Required for large UPDATE messages with many attributes.
        - **graceful-restart** `container`
          Graceful Restart capability configuration.
          - **long-lived-stale-time** `uint32`
            Long-Lived Stale Time in seconds (RFC 9494 Section 3). When set, LLGR capability (code 71) is advertised. Maximum value is 16777215 (24-bit field, ~194 days). Applied to all negotiated address families for the peer.
          - **restart-time** `uint16`
            Restart Time in seconds (RFC 4724 Section 3). Maximum value is 4095 (12-bit field).
        - **hostname** `container`
          FQDN capability configuration.
          - **domain** `string`
            Domain name (max 255 bytes).
          - **host** `string`
            System hostname (max 255 bytes).
        - **link-local-nexthop** `container`
          Link-local next-hop capability. Advertises willingness to receive IPv6 link-local next-hops in MP_REACH_NLRI (RFC 2545 Section 3).
        - **nexthop <family>** `list`
          Extended Next Hop capability (RFC 8950)
          - **mode** `enumeration`
            Extended next hop negotiation mode
          - **nhafi** `afi`
            Next-hop AFI (e.g. ipv6)
        - **route-refresh** `container`
          Route Refresh capability (RFC 2918). Allows the peer to request a full re-advertisement of routes without tearing down the session.
        - **software-version** `container`
          Software Version capability (code 75).
          - **mode** `enumeration`
            Capability negotiation mode.
      - **cluster-id** `ipv4-address`
        Override cluster ID for route reflection (RFC 4456 Section 7). Defaults to router-id when not set. Prepended to CLUSTER_LIST on reflected routes.
      - **community** `container`
        Community attribute control for this session
        - **send** `enumeration[]`
          Community types to include in outbound UPDATEs. Default is all (send every community type). Specify individual types for granular control. none suppresses all community attributes.
      - **domain-name** `string`
        Legacy: Domain name for FQDN capability.
      - **family <name>** `list`
        Address families to negotiate
        - **default-originate** `boolean`
          Originate the default route (0.0.0.0/0 for IPv4, ::/0 for IPv6) to this peer for this address family.
        - **default-originate-filter** `string`
          Named filter that must accept for the default route to be originated. If the filter rejects, the default route is not sent. Empty or absent means always originate (unconditional).
        - **mode** `enumeration`
          Address family negotiation mode
        - **prefix** `container`
          Prefix limit configuration for this address family
          - **idle-timeout** `uint16`
            Seconds before auto-reconnect after prefix teardown (0 = no reconnect)
          - **maximum** `uint32`
            Hard maximum number of prefixes accepted
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
          Explicit AS-SET name for IRR prefix lookup. When omitted, the AS-SET is auto-discovered from PeeringDB using the peer's remote ASN.
        - **enable** `enumeration`
          Enable or disable IRR filtering for this peer.
      - **link-local** `ipv6-address`
        IPv6 link-local address for next-hop (RFC 2545 Section 3)
      - **next-hop** `union`
        Next-hop rewriting policy for forwarded UPDATEs (RFC 4271 Section 5.1.3). auto: rewrite for eBGP peers, preserve for iBGP peers. self: always rewrite to local address. unchanged: never rewrite (preserves original next-hop). IP address: set next-hop to explicit address.
      - **route-reflector-client** `boolean`
        Mark this peer as a route reflector client (RFC 4456). Routes from clients are forwarded to all clients and non-clients. Routes from non-clients are forwarded to clients only.
      - **router-id** `ipv4-address`
        Override router ID for this peer
      - **rs-client** `boolean`
        Mark this peer as an RS-client for transparent AS-path forwarding. RFC 7947 Section 2.2.2: the route server MUST NOT modify AS_PATH or any other transitive attribute. When true, the reactor skips AS-path prepending for this peer on the forwarding path.
    - **timer** `container`
      BGP session timers: hold time, keepalive interval, and connect retry delay. Hold time is proposed in OPEN and negotiated to the lower of both peers' values.
      - **connect-retry** `uint16`
        Connect retry interval in seconds (RFC 4271 Section 8)
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
          AGGREGATOR attribute (RFC 4271 Section 5.1.7). Format: AS:IP. Identifies the AS and router that formed the aggregate.
        - **as-path** `string[]`
          AS_PATH segments prepended to the route. Space-separated ASNs. Affects loop detection and path selection (shorter preferred).
        - **atomic-aggregate** `boolean`
          ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6). Signals that the route was formed by aggregation and AS_PATH information was lost.
        - **attribute** `string[]`
          Raw BGP attributes in hex encoding. For attributes not covered by named fields. Format: flags:type:hex-value.
        - **bgp-prefix-sid** `container`
          Prefix-SID attribute (RFC 8669). Carries an MPLS label index for Segment Routing, allowing a node to advertise its SID without per-prefix label allocation.
        - **bgp-prefix-sid-srv6** `container`
          SRv6 Prefix-SID attribute (RFC 9252). Carries SRv6 SID information for IPv6 Segment Routing, including SRv6 L3 Service TLVs.
        - **cluster-list** `ipv4-address[]`
          CLUSTER_LIST for route reflection (RFC 4456). Each reflector prepends its cluster ID. Used for loop detection between reflectors.
        - **community** `string[]`
          Standard BGP communities (RFC 1997). Format: AS:value (e.g., 65000:100) or well-known names (no-export, no-advertise).
        - **extended-community** `string[]`
          Extended communities (RFC 4360). Used for route targets, site-of-origin, and VPN semantics. Format: type:admin:value.
        - **label** `string`
          MPLS label value for labeled unicast routes (RFC 3107). Numeric or keyword.
        - **labels** `string[]`
          MPLS label stack for multi-label routes (RFC 8277). Ordered bottom-to-top.
        - **large-community** `string[]`
          Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
        - **local-preference** `uint32`
          LOCAL_PREF value for iBGP best-path selection. Higher wins. Only meaningful within a single AS (RFC 4271 Section 5.1.5). Default: 100.
        - **med** `uint32`
          Multi-Exit Discriminator (MED). Lower values are preferred. Used to influence inbound path selection by an external peer (RFC 4271 Section 5.1.4).
        - **next-hop** `union`
          Next hop address or 'self'
        - **origin** `enumeration`
          ORIGIN attribute (RFC 4271 Section 4.3)
        - **originator-id** `ipv4-address`
          ORIGINATOR_ID for route reflection (RFC 4456). Set by the reflector to the originating router's ID. Used for loop prevention.
        - **path-information** `string`
          ADD-PATH path identifier (RFC 7911). Distinguishes multiple paths for the same prefix from the same peer.
        - **rd** `route-distinguisher`
          Route Distinguisher (RFC 4364). Prepended to the prefix to create a unique VPN route. Format: ASN:nn or IP:nn.
        - **split** `string`
          Split a single prefix into multiple more-specifics for announcement. Format: /length (e.g., /24 splits a /16 into 256 /24s).
      - **name** `string`
        Optional label for this update block (display only)
      - **nlri <name>** `list`
        - **content** `string`
          Operation, qualifiers, and payload
      - **watchdog** `container`
        Watchdog-controlled route - held until 'bgp watchdog announce <name>'
        - **name** `string`
          Watchdog group name
        - **withdraw** `boolean`
          Start in withdrawn state (default true)
  - **process <name>** `list`
    External process that receives BGP events and can inject messages. Ze spawns the process and communicates via stdin/stdout JSON.
    - **content** `container`
      Controls the encoding and filtering of events sent to this process.
      - **attribute** `string`
        Filter expression to select which BGP attributes are included in events.
      - **encoding** `string`
        Wire encoding for events sent to the process (e.g., json, text).
      - **format** `string`
        Output format template for event rendering.
    - **neighbor-changes** `container`
      Send session state change events (up/down/reset) to this process. Enables the process to react to peer lifecycle transitions.
    - **processes** `string[]`
      Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.
    - **processes-match** `string[]`
      Legacy ExaBGP process match patterns for event filtering.
    - **receive** `string[]`
      Event types to receive. Base types: update, open, notification, keepalive, refresh, state, sent, negotiated. Plugins may register additional types (e.g., update-rpki). List types explicitly; 'all' is not accepted. Validated at runtime against registered event types.
    - **run** `string`
      Shell command to spawn the external process. Ze pipes BGP events to its stdin and reads commands from its stdout.
    - **send** `string[]`
      Message types to send. Valid: update, refresh, enhanced-refresh (BORR/EORR markers, RFC 7313). Validated at runtime against known send types.
  - **rib** `container`
    Route Information Base settings for this peer. Controls which RIB tables are maintained and how outbound route batching works.
    - **adj** `container`
      Adjacency RIB storage. These tables hold the raw routes before and after policy processing, enabling route-refresh and soft-reconfiguration.
      - **in** `boolean`
        Store received routes in Adj-RIB-In before policy. Enables soft reconfiguration inbound (re-apply import filters without session reset). Uses additional memory proportional to received route count.
      - **out** `boolean`
        Store advertised routes in Adj-RIB-Out after policy. Required for route-refresh (RFC 2918) to re-send routes on peer request. Uses additional memory proportional to advertised route count.
    - **out** `container`
      Outbound RIB batching settings. Control how route changes are grouped and flushed to the peer.
      - **auto-commit-delay** `uint32`
        Milliseconds to wait before flushing pending route changes to the peer. Allows batching rapid successive changes into fewer UPDATEs. 0 means flush immediately on each change.
      - **group-updates** `boolean`
        Pack routes with identical attributes into a single UPDATE. Same as the behavior-level group-updates but scoped to this peer's Adj-RIB-Out.
      - **max-batch-size** `int32`
        Maximum number of route changes to include in a single flush cycle. 0 means no limit (flush all pending changes at once). Limits per-cycle CPU cost at the expense of convergence latency.
  - **role** `container`
    RFC 9234 BGP Role configuration
    - **export** `export-token[]`
      Controls which destination peer roles may receive routes from this peer
    - **import** `role-type`
      Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)
    - **strict** `boolean`
      Require peer to send Role capability
  - **rpki** `container`
    Per-peer/group RPKI action overrides (peer > group > global, resolved per leaf)
    - **action** `container`
      Origin-validation action overrides for this peer/group
      - **invalid** `validation-action`
        Action for routes with Invalid validation state (unset: inherit group then global)
      - **not-found** `validation-action`
        Action for routes with NotFound validation state (unset: inherit group then global)
    - **aspa** `container`
      ASPA action overrides for this peer/group (only applies when ASPA validation is enabled globally)
      - **action** `container`
        - **invalid** `validation-action`
          Action for routes with ASPA Invalid path state (unset: inherit group then global)
        - **unknown** `validation-action`
          Action for routes with ASPA Unknown path state (unset: inherit group then global)
  - **session** `container`
    BGP session parameters: ASN, capabilities, address families, next-hop policy, and community control. Inherited from group to peer; peer values override.
    - **accept-srv6-prefix-sid** `boolean`
      Accept BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer. RFC 8669 Section 4: PrefixSID from EBGP outside the SR domain MUST be discarded unless configured to accept. Has no effect on IBGP (always accepted).
    - **as-override** `boolean`
      Replace peer's ASN with local ASN in outbound AS_PATH. Used in VPN/multi-site where the same customer ASN appears at multiple sites.
    - **asn** `container`
      AS number configuration
      - **local** `asn`
        Local AS (overrides global)
      - **local-options** `enumeration[]`
        Modifiers for local-as behavior. no-prepend: real ASN not prepended before local-as. replace-as: local-as replaces real ASN entirely. Both together: full replacement with no prepend.
      - **remote** `asn`
        Peer Autonomous System Number
    - **capability** `container`
      BGP capability negotiation
      - **add-path** `container`
        ADD-PATH capability (RFC 7911) with optional PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit). Default direction applies to all negotiated multiprotocol families. Per-family entries override.
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
          Default maximum paths per prefix (PATHS-LIMIT). Inherited by all families unless overridden per-family.
      - **asn4** `boolean`
        Advertise 4-byte ASN capability (RFC 6793). Required for ASNs above 65535. Disable only for peers running legacy 2-byte-only implementations.
      - **extended-message** `container`
        Extended Message capability (RFC 8654). Raises the BGP message size limit from 4096 to 65535 bytes. Required for large UPDATE messages with many attributes.
      - **graceful-restart** `container`
        Graceful Restart capability configuration.
        - **long-lived-stale-time** `uint32`
          Long-Lived Stale Time in seconds (RFC 9494 Section 3). When set, LLGR capability (code 71) is advertised. Maximum value is 16777215 (24-bit field, ~194 days). Applied to all negotiated address families for the peer.
        - **restart-time** `uint16`
          Restart Time in seconds (RFC 4724 Section 3). Maximum value is 4095 (12-bit field).
      - **hostname** `container`
        FQDN capability configuration.
        - **domain** `string`
          Domain name (max 255 bytes).
        - **host** `string`
          System hostname (max 255 bytes).
      - **link-local-nexthop** `container`
        Link-local next-hop capability. Advertises willingness to receive IPv6 link-local next-hops in MP_REACH_NLRI (RFC 2545 Section 3).
      - **nexthop <family>** `list`
        Extended Next Hop capability (RFC 8950)
        - **mode** `enumeration`
          Extended next hop negotiation mode
        - **nhafi** `afi`
          Next-hop AFI (e.g. ipv6)
      - **route-refresh** `container`
        Route Refresh capability (RFC 2918). Allows the peer to request a full re-advertisement of routes without tearing down the session.
      - **software-version** `container`
        Software Version capability (code 75).
        - **mode** `enumeration`
          Capability negotiation mode.
    - **cluster-id** `ipv4-address`
      Override cluster ID for route reflection (RFC 4456 Section 7). Defaults to router-id when not set. Prepended to CLUSTER_LIST on reflected routes.
    - **community** `container`
      Community attribute control for this session
      - **send** `enumeration[]`
        Community types to include in outbound UPDATEs. Default is all (send every community type). Specify individual types for granular control. none suppresses all community attributes.
    - **domain-name** `string`
      Legacy: Domain name for FQDN capability (group default).
    - **family <name>** `list`
      Address families to negotiate
      - **default-originate** `boolean`
        Originate the default route (0.0.0.0/0 for IPv4, ::/0 for IPv6) to this peer for this address family.
      - **default-originate-filter** `string`
        Named filter that must accept for the default route to be originated. If the filter rejects, the default route is not sent. Empty or absent means always originate (unconditional).
      - **mode** `enumeration`
        Address family negotiation mode
      - **prefix** `container`
        Prefix limit configuration for this address family
        - **idle-timeout** `uint16`
          Seconds before auto-reconnect after prefix teardown (0 = no reconnect)
        - **maximum** `uint32`
          Hard maximum number of prefixes accepted
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
        Explicit AS-SET name for IRR prefix lookup. When omitted, the AS-SET is auto-discovered from PeeringDB using the peer's remote ASN.
      - **enable** `enumeration`
        Enable or disable IRR filtering for this peer.
    - **link-local** `ipv6-address`
      IPv6 link-local address for next-hop (RFC 2545 Section 3)
    - **next-hop** `union`
      Next-hop rewriting policy for forwarded UPDATEs (RFC 4271 Section 5.1.3). auto: rewrite for eBGP peers, preserve for iBGP peers. self: always rewrite to local address. unchanged: never rewrite (preserves original next-hop). IP address: set next-hop to explicit address.
    - **route-reflector-client** `boolean`
      Mark this peer as a route reflector client (RFC 4456). Routes from clients are forwarded to all clients and non-clients. Routes from non-clients are forwarded to clients only.
    - **router-id** `ipv4-address`
      Override router ID for this peer
    - **rs-client** `boolean`
      Mark this peer as an RS-client for transparent AS-path forwarding. RFC 7947 Section 2.2.2: the route server MUST NOT modify AS_PATH or any other transitive attribute. When true, the reactor skips AS-path prepending for this peer on the forwarding path.
  - **timer** `container`
    BGP session timers: hold time, keepalive interval, and connect retry delay. Hold time is proposed in OPEN and negotiated to the lower of both peers' values.
    - **connect-retry** `uint16`
      Connect retry interval in seconds (RFC 4271 Section 8)
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
        AGGREGATOR attribute (RFC 4271 Section 5.1.7). Format: AS:IP. Identifies the AS and router that formed the aggregate.
      - **as-path** `string[]`
        AS_PATH segments prepended to the route. Space-separated ASNs. Affects loop detection and path selection (shorter preferred).
      - **atomic-aggregate** `boolean`
        ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6). Signals that the route was formed by aggregation and AS_PATH information was lost.
      - **attribute** `string[]`
        Raw BGP attributes in hex encoding. For attributes not covered by named fields. Format: flags:type:hex-value.
      - **bgp-prefix-sid** `container`
        Prefix-SID attribute (RFC 8669). Carries an MPLS label index for Segment Routing, allowing a node to advertise its SID without per-prefix label allocation.
      - **bgp-prefix-sid-srv6** `container`
        SRv6 Prefix-SID attribute (RFC 9252). Carries SRv6 SID information for IPv6 Segment Routing, including SRv6 L3 Service TLVs.
      - **cluster-list** `ipv4-address[]`
        CLUSTER_LIST for route reflection (RFC 4456). Each reflector prepends its cluster ID. Used for loop detection between reflectors.
      - **community** `string[]`
        Standard BGP communities (RFC 1997). Format: AS:value (e.g., 65000:100) or well-known names (no-export, no-advertise).
      - **extended-community** `string[]`
        Extended communities (RFC 4360). Used for route targets, site-of-origin, and VPN semantics. Format: type:admin:value.
      - **label** `string`
        MPLS label value for labeled unicast routes (RFC 3107). Numeric or keyword.
      - **labels** `string[]`
        MPLS label stack for multi-label routes (RFC 8277). Ordered bottom-to-top.
      - **large-community** `string[]`
        Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
      - **local-preference** `uint32`
        LOCAL_PREF value for iBGP best-path selection. Higher wins. Only meaningful within a single AS (RFC 4271 Section 5.1.5). Default: 100.
      - **med** `uint32`
        Multi-Exit Discriminator (MED). Lower values are preferred. Used to influence inbound path selection by an external peer (RFC 4271 Section 5.1.4).
      - **next-hop** `union`
        Next hop address or 'self'
      - **origin** `enumeration`
        ORIGIN attribute (RFC 4271 Section 4.3)
      - **originator-id** `ipv4-address`
        ORIGINATOR_ID for route reflection (RFC 4456). Set by the reflector to the originating router's ID. Used for loop prevention.
      - **path-information** `string`
        ADD-PATH path identifier (RFC 7911). Distinguishes multiple paths for the same prefix from the same peer.
      - **rd** `route-distinguisher`
        Route Distinguisher (RFC 4364). Prepended to the prefix to create a unique VPN route. Format: ASN:nn or IP:nn.
      - **split** `string`
        Split a single prefix into multiple more-specifics for announcement. Format: /length (e.g., /24 splits a /16 into 256 /24s).
    - **name** `string`
      Optional label for this update block (display only)
    - **nlri <name>** `list`
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
      When true, withdraw route on DOWN/DISABLED. When false (default), re-announce with down-metric/disabled-metric.
- **multipath** `container`
  BGP multipath / ECMP configuration
  - **maximum-paths** `uint16`
    Maximum number of equal-cost paths to install per prefix. 1 means single best path (default, RFC 4271 Section 9.1.2). Values > 1 enable ECMP with N-way load balancing.
  - **relax-as-path** `boolean`
    Allow paths with different AS-paths to be considered equal-cost. When false, multipath requires identical AS-path length and content. When true, only AS-path length must match (not content). Equivalent to 'bgp bestpath as-path multipath-relax' on other vendors.
- **peer <name>** `list`
  BGP peer configuration (standalone, no group)
  - **behavior** `container`
    Operational knobs that control how the reactor processes and forwards UPDATE messages for this peer. Most users can leave these at defaults.
    - **auto-flush** `boolean`
      Automatically withdraw all routes advertised to this peer when the session goes down. When disabled, routes remain until explicitly withdrawn or the hold timer expires. Legacy ExaBGP option, retained for migration.
    - **group-updates** `boolean`
      Pack multiple NLRI into a single UPDATE message when they share the same path attributes. Reduces the number of UPDATE messages from O(routes) to O(unique-attribute-sets). Disable for peers that require one prefix per UPDATE.
    - **manual-eor** `boolean`
      Do not send End-of-RIB automatically after initial route advertisement. When enabled, End-of-RIB must be triggered externally via the process API. Used when an external controller manages convergence signaling.
    - **rs-fast-path** `boolean`
      Forward received UPDATEs directly inside the reactor for RS-client peers, bypassing the plugin dispatch chain. Lower latency for route server forwarding. Peers with export filters are excluded and use the normal path.
  - **connection** `container`
    Transport-level connection settings
    - **bfd** `container`
      Bidirectional Forwarding Detection (RFC 5880) options for this peer. When the container is present, the BGP reactor calls the BFD plugin's Service interface on session establishment and tears the BGP session down when BFD reports Down. The BFD plugin must be loaded (a top-level bfd { ... } block); if it is not, the BGP peer starts without BFD and logs a warning.
      - **enabled** `boolean`
        Master switch for this peer's BFD session. Set false to keep the config in place but suspend the BFD client (useful for maintenance).
      - **interface** `string`
        Single-hop egress interface. Optional: when omitted, the BFD plugin derives the interface from the peer's local address. Ignored for multi-hop.
      - **min-ttl** `uint8`
        Multi-hop minimum acceptable TTL (RFC 5883 §5). Ignored for single-hop. Zero means use the plugin default (254).
      - **mode** `enumeration`
        Hop mode. Single-hop is the common case for an eBGP peer on a direct link; multi-hop is for iBGP over an IGP or any peering that crosses more than one IP hop.
      - **profile** `string`
        Name of a profile defined under the top-level bfd { profile ... } block. The referenced profile supplies detect-multiplier, desired-min-tx-us, and required-min-rx-us. Empty means use the BFD plugin defaults.
    - **link-local** `boolean`
      Auto-discover IPv6 link-local address for TCP connection
    - **local** `container`
      Local endpoint for the TCP session. Controls the bind address, port, and whether to accept inbound connections.
      - **accept** `boolean`
        Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Local address for connection (use IP address or 'auto')
      - **port** `port`
        Local bind port
    - **md5** `container`
      TCP MD5 authentication (RFC 2385)
      - **ip** `ip-address`
        MD5 authentication IP
      - **password** `string`
        MD5 authentication password
    - **remote** `container`
      Remote endpoint for the TCP session. Controls the peer address, port, outbound connection initiation, and dynamic peer ranges.
      - **connect** `boolean`
        Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)
      - **ip** `union`
        Peer IP address or 'dynamic' for dynamic peer groups
      - **max-peers** `uint32`
        Maximum number of dynamic peers for this group
      - **port** `port`
        Remote connection port
      - **range** `ip-prefix[]`
        IP prefix ranges for dynamic peer groups. Only meaningful when ip is 'dynamic'. Connections from IPs within these ranges create dynamic peers.
    - **ttl** `container`
      TTL settings for BGP sessions
      - **max** `uint8`
        TTL security / GTSM (RFC 5082)
      - **min** `uint8`
        Minimum incoming TTL
      - **set** `uint8`
        Outgoing TTL value
  - **description** `string`
    Free-text label for this peer. Shown in the CLI, web UI, and logs. Typically the peer's role or location (e.g., 'upstream-transit-provider').
  - **filter** `container`
    Route filter chains for import and export. Names reference filter instances defined in bgp/policy.
    - **egress** `container`
      Egress direction filters
      - **community** `container`
        Community filter for egress
        - **strip** `string[]`
          Named communities to remove on egress
        - **tag** `string[]`
          Named communities to add on egress
    - **export** `string[]`
      Export filter chain (filter instance names)
    - **import** `string[]`
      Import filter chain (filter instance names)
    - **ingress** `container`
      Ingress direction filters
      - **community** `container`
        Community filter for ingress
        - **strip** `string[]`
          Named communities to remove on ingress
        - **tag** `string[]`
          Named communities to add on ingress
  - **process <name>** `list`
    External process that receives BGP events and can inject messages. Ze spawns the process and communicates via stdin/stdout JSON.
    - **content** `container`
      Controls the encoding and filtering of events sent to this process.
      - **attribute** `string`
        Filter expression to select which BGP attributes are included in events.
      - **encoding** `string`
        Wire encoding for events sent to the process (e.g., json, text).
      - **format** `string`
        Output format template for event rendering.
    - **neighbor-changes** `container`
      Send session state change events (up/down/reset) to this process. Enables the process to react to peer lifecycle transitions.
    - **processes** `string[]`
      Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.
    - **processes-match** `string[]`
      Legacy ExaBGP process match patterns for event filtering.
    - **receive** `string[]`
      Event types to receive. Base types: update, open, notification, keepalive, refresh, state, sent, negotiated. Plugins may register additional types (e.g., update-rpki). List types explicitly; 'all' is not accepted. Validated at runtime against registered event types.
    - **run** `string`
      Shell command to spawn the external process. Ze pipes BGP events to its stdin and reads commands from its stdout.
    - **send** `string[]`
      Message types to send. Valid: update, refresh, enhanced-refresh (BORR/EORR markers, RFC 7313). Validated at runtime against known send types.
  - **rib** `container`
    Route Information Base settings for this peer. Controls which RIB tables are maintained and how outbound route batching works.
    - **adj** `container`
      Adjacency RIB storage. These tables hold the raw routes before and after policy processing, enabling route-refresh and soft-reconfiguration.
      - **in** `boolean`
        Store received routes in Adj-RIB-In before policy. Enables soft reconfiguration inbound (re-apply import filters without session reset). Uses additional memory proportional to received route count.
      - **out** `boolean`
        Store advertised routes in Adj-RIB-Out after policy. Required for route-refresh (RFC 2918) to re-send routes on peer request. Uses additional memory proportional to advertised route count.
    - **out** `container`
      Outbound RIB batching settings. Control how route changes are grouped and flushed to the peer.
      - **auto-commit-delay** `uint32`
        Milliseconds to wait before flushing pending route changes to the peer. Allows batching rapid successive changes into fewer UPDATEs. 0 means flush immediately on each change.
      - **group-updates** `boolean`
        Pack routes with identical attributes into a single UPDATE. Same as the behavior-level group-updates but scoped to this peer's Adj-RIB-Out.
      - **max-batch-size** `int32`
        Maximum number of route changes to include in a single flush cycle. 0 means no limit (flush all pending changes at once). Limits per-cycle CPU cost at the expense of convergence latency.
  - **role** `container`
    RFC 9234 BGP Role configuration
    - **export** `export-token[]`
      Controls which destination peer roles may receive routes from this peer
    - **import** `role-type`
      Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)
    - **strict** `boolean`
      Require peer to send Role capability
  - **rpki** `container`
    Per-peer/group RPKI action overrides (peer > group > global, resolved per leaf)
    - **action** `container`
      Origin-validation action overrides for this peer/group
      - **invalid** `validation-action`
        Action for routes with Invalid validation state (unset: inherit group then global)
      - **not-found** `validation-action`
        Action for routes with NotFound validation state (unset: inherit group then global)
    - **aspa** `container`
      ASPA action overrides for this peer/group (only applies when ASPA validation is enabled globally)
      - **action** `container`
        - **invalid** `validation-action`
          Action for routes with ASPA Invalid path state (unset: inherit group then global)
        - **unknown** `validation-action`
          Action for routes with ASPA Unknown path state (unset: inherit group then global)
  - **session** `container`
    BGP session parameters: ASN, capabilities, address families, next-hop policy, and community control. Inherited from group to peer; peer values override.
    - **accept-srv6-prefix-sid** `boolean`
      Accept BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer. RFC 8669 Section 4: PrefixSID from EBGP outside the SR domain MUST be discarded unless configured to accept. Has no effect on IBGP (always accepted).
    - **as-override** `boolean`
      Replace peer's ASN with local ASN in outbound AS_PATH. Used in VPN/multi-site where the same customer ASN appears at multiple sites.
    - **asn** `container`
      AS number configuration
      - **local** `asn`
        Local AS (overrides global)
      - **local-options** `enumeration[]`
        Modifiers for local-as behavior. no-prepend: real ASN not prepended before local-as. replace-as: local-as replaces real ASN entirely. Both together: full replacement with no prepend.
      - **remote** `asn`
        Peer Autonomous System Number
    - **capability** `container`
      BGP capability negotiation
      - **add-path** `container`
        ADD-PATH capability (RFC 7911) with optional PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit). Default direction applies to all negotiated multiprotocol families. Per-family entries override.
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
          Default maximum paths per prefix (PATHS-LIMIT). Inherited by all families unless overridden per-family.
      - **asn4** `boolean`
        Advertise 4-byte ASN capability (RFC 6793). Required for ASNs above 65535. Disable only for peers running legacy 2-byte-only implementations.
      - **extended-message** `container`
        Extended Message capability (RFC 8654). Raises the BGP message size limit from 4096 to 65535 bytes. Required for large UPDATE messages with many attributes.
      - **graceful-restart** `container`
        Graceful Restart capability configuration.
        - **long-lived-stale-time** `uint32`
          Long-Lived Stale Time in seconds (RFC 9494 Section 3). When set, LLGR capability (code 71) is advertised. Maximum value is 16777215 (24-bit field, ~194 days). Applied to all negotiated address families for the peer.
        - **restart-time** `uint16`
          Restart Time in seconds (RFC 4724 Section 3). Maximum value is 4095 (12-bit field).
      - **hostname** `container`
        FQDN capability configuration.
        - **domain** `string`
          Domain name (max 255 bytes).
        - **host** `string`
          System hostname (max 255 bytes).
      - **link-local-nexthop** `container`
        Link-local next-hop capability. Advertises willingness to receive IPv6 link-local next-hops in MP_REACH_NLRI (RFC 2545 Section 3).
      - **nexthop <family>** `list`
        Extended Next Hop capability (RFC 8950)
        - **mode** `enumeration`
          Extended next hop negotiation mode
        - **nhafi** `afi`
          Next-hop AFI (e.g. ipv6)
      - **route-refresh** `container`
        Route Refresh capability (RFC 2918). Allows the peer to request a full re-advertisement of routes without tearing down the session.
      - **software-version** `container`
        Software Version capability (code 75).
        - **mode** `enumeration`
          Capability negotiation mode.
    - **cluster-id** `ipv4-address`
      Override cluster ID for route reflection (RFC 4456 Section 7). Defaults to router-id when not set. Prepended to CLUSTER_LIST on reflected routes.
    - **community** `container`
      Community attribute control for this session
      - **send** `enumeration[]`
        Community types to include in outbound UPDATEs. Default is all (send every community type). Specify individual types for granular control. none suppresses all community attributes.
    - **domain-name** `string`
      Legacy: Domain name for FQDN capability.
    - **family <name>** `list`
      Address families to negotiate
      - **default-originate** `boolean`
        Originate the default route (0.0.0.0/0 for IPv4, ::/0 for IPv6) to this peer for this address family.
      - **default-originate-filter** `string`
        Named filter that must accept for the default route to be originated. If the filter rejects, the default route is not sent. Empty or absent means always originate (unconditional).
      - **mode** `enumeration`
        Address family negotiation mode
      - **prefix** `container`
        Prefix limit configuration for this address family
        - **idle-timeout** `uint16`
          Seconds before auto-reconnect after prefix teardown (0 = no reconnect)
        - **maximum** `uint32`
          Hard maximum number of prefixes accepted
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
        Explicit AS-SET name for IRR prefix lookup. When omitted, the AS-SET is auto-discovered from PeeringDB using the peer's remote ASN.
      - **enable** `enumeration`
        Enable or disable IRR filtering for this peer.
    - **link-local** `ipv6-address`
      IPv6 link-local address for next-hop (RFC 2545 Section 3)
    - **next-hop** `union`
      Next-hop rewriting policy for forwarded UPDATEs (RFC 4271 Section 5.1.3). auto: rewrite for eBGP peers, preserve for iBGP peers. self: always rewrite to local address. unchanged: never rewrite (preserves original next-hop). IP address: set next-hop to explicit address.
    - **route-reflector-client** `boolean`
      Mark this peer as a route reflector client (RFC 4456). Routes from clients are forwarded to all clients and non-clients. Routes from non-clients are forwarded to clients only.
    - **router-id** `ipv4-address`
      Override router ID for this peer
    - **rs-client** `boolean`
      Mark this peer as an RS-client for transparent AS-path forwarding. RFC 7947 Section 2.2.2: the route server MUST NOT modify AS_PATH or any other transitive attribute. When true, the reactor skips AS-path prepending for this peer on the forwarding path.
  - **timer** `container`
    BGP session timers: hold time, keepalive interval, and connect retry delay. Hold time is proposed in OPEN and negotiated to the lower of both peers' values.
    - **connect-retry** `uint16`
      Connect retry interval in seconds (RFC 4271 Section 8)
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
        AGGREGATOR attribute (RFC 4271 Section 5.1.7). Format: AS:IP. Identifies the AS and router that formed the aggregate.
      - **as-path** `string[]`
        AS_PATH segments prepended to the route. Space-separated ASNs. Affects loop detection and path selection (shorter preferred).
      - **atomic-aggregate** `boolean`
        ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6). Signals that the route was formed by aggregation and AS_PATH information was lost.
      - **attribute** `string[]`
        Raw BGP attributes in hex encoding. For attributes not covered by named fields. Format: flags:type:hex-value.
      - **bgp-prefix-sid** `container`
        Prefix-SID attribute (RFC 8669). Carries an MPLS label index for Segment Routing, allowing a node to advertise its SID without per-prefix label allocation.
      - **bgp-prefix-sid-srv6** `container`
        SRv6 Prefix-SID attribute (RFC 9252). Carries SRv6 SID information for IPv6 Segment Routing, including SRv6 L3 Service TLVs.
      - **cluster-list** `ipv4-address[]`
        CLUSTER_LIST for route reflection (RFC 4456). Each reflector prepends its cluster ID. Used for loop detection between reflectors.
      - **community** `string[]`
        Standard BGP communities (RFC 1997). Format: AS:value (e.g., 65000:100) or well-known names (no-export, no-advertise).
      - **extended-community** `string[]`
        Extended communities (RFC 4360). Used for route targets, site-of-origin, and VPN semantics. Format: type:admin:value.
      - **label** `string`
        MPLS label value for labeled unicast routes (RFC 3107). Numeric or keyword.
      - **labels** `string[]`
        MPLS label stack for multi-label routes (RFC 8277). Ordered bottom-to-top.
      - **large-community** `string[]`
        Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
      - **local-preference** `uint32`
        LOCAL_PREF value for iBGP best-path selection. Higher wins. Only meaningful within a single AS (RFC 4271 Section 5.1.5). Default: 100.
      - **med** `uint32`
        Multi-Exit Discriminator (MED). Lower values are preferred. Used to influence inbound path selection by an external peer (RFC 4271 Section 5.1.4).
      - **next-hop** `union`
        Next hop address or 'self'
      - **origin** `enumeration`
        ORIGIN attribute (RFC 4271 Section 4.3)
      - **originator-id** `ipv4-address`
        ORIGINATOR_ID for route reflection (RFC 4456). Set by the reflector to the originating router's ID. Used for loop prevention.
      - **path-information** `string`
        ADD-PATH path identifier (RFC 7911). Distinguishes multiple paths for the same prefix from the same peer.
      - **rd** `route-distinguisher`
        Route Distinguisher (RFC 4364). Prepended to the prefix to create a unique VPN route. Format: ASN:nn or IP:nn.
      - **split** `string`
        Split a single prefix into multiple more-specifics for announcement. Format: /length (e.g., /24 splits a /16 into 256 /24s).
    - **name** `string`
      Optional label for this update block (display only)
    - **nlri <name>** `list`
      - **content** `string`
        Operation, qualifiers, and payload
    - **watchdog** `container`
      Watchdog-controlled route - held until 'bgp watchdog announce <name>'
      - **name** `string`
        Watchdog group name
      - **withdraw** `boolean`
        Start in withdrawn state (default true)
- **policy** `container`
  Named filter definitions for the route policy framework. Each filter type is a list added by its plugin via augment. Filter instances are referenced by name in peer filter chains.
  - **as-path-length <name>** `list`
    Named AS-path length filter instance. Routes with path length outside the configured min..max range are rejected. Both min and max are optional; omitting one makes that bound open.
    - **max** `uint16`
      Maximum allowed AS-path length (inclusive). Routes with longer paths are rejected.
    - **min** `uint16`
      Minimum required AS-path length (inclusive). Routes with shorter paths are rejected.
  - **as-path-list <name>** `list`
    Named AS-path regex filter instance. Each list contains an ordered set of regex entries. Entries are evaluated in order; first match wins. No match = implicit deny (reject).
    - **entry <regex>** `list`
      Ordered regex entry. First match wins against the space-separated AS-path string.
      - **action** `enumeration`
        Action applied when this entry's regex matches.
  - **community-match <name>** `list`
    Named community match filter instance. Each list contains an ordered set of match entries. Entries are evaluated in order; first match wins. No match = implicit deny (reject).
    - **entry <community>** `list`
      Ordered match entry. First match wins. Checks whether the specified community value is present in the route's community attributes.
      - **action** `enumeration`
        Action applied when this community is found in the route.
      - **type** `enumeration`
        Which community attribute type to check.
  - **family-filter <name>** `list`
    Named address-family filter instance.
    - **action** `enumeration`
      Action to apply when the family matches.
    - **family** `address-family`
      Address family to match, e.g. ipv4/flowspec. An UPDATE with no MP_REACH/MP_UNREACH attribute is treated as ipv4/unicast.
  - **irr** `container`
    Global IRR filtering settings.
    - **peeringdb-url** `string`
      Base URL for PeeringDB API queries. Override for testing with a mock server.
    - **refresh-interval** `uint32`
      Seconds between automatic IRR re-queries.
    - **server** `string`
      IRR whois server hostname or host:port.
    - **source-address** `ip-address`
      Source IP address for outbound IRR whois connections.
  - **loop-detection <name>** `list`
    Named loop detection filter instance. Configures AS loop and cluster-list loop detection per-peer.
    - **allow-own-as** `uint8`
      Number of own-AS occurrences to allow in AS_PATH before rejecting. 0 means reject on first occurrence (default, RFC 4271 Section 9).
    - **cluster-id** `ipv4-address`
      Override Router ID for CLUSTER_LIST loop detection. If not set, uses the BGP Router ID (RFC 4456 Section 7).
  - **modify <name>** `list`
    Named route attribute modifier instance. Only declared leaves are modified; undeclared attributes are preserved unchanged. Multiple attributes can be set in a single modifier.
    - **decrement** `container`
      Attributes to decrement on matching routes. Subtracts the specified value from the current attribute value. Floors at 0 (no underflow). Mutually exclusive with set for the same attribute.
      - **aigp** `uint32`
        Decrement AIGP metric by this value.
      - **local-preference** `uint32`
        Decrement LOCAL_PREF by this value.
      - **med** `uint32`
        Decrement MED by this value.
    - **increment** `container`
      Attributes to increment on matching routes. Adds the specified value to the current attribute value. Saturates at uint32 max (4294967295). Mutually exclusive with set for the same attribute.
      - **aigp** `uint32`
        Increment AIGP metric by this value.
      - **local-preference** `uint32`
        Increment LOCAL_PREF by this value.
      - **med** `uint32`
        Increment MED by this value.
    - **set** `container`
      Attributes to set on matching routes. Only present leaves are applied.
      - **as-path-prepend** `uint8`
        Prepend local AS to AS_PATH this many times. The actual ASN prepended is the peer's local-as (from session config). Range 1-32 prevents excessive path inflation.
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
        Set NEXT_HOP attribute (RFC 4271 type 3). IPv4 address only (IPv6 next-hop is in MP_REACH).
      - **origin** `enumeration`
        Set ORIGIN attribute (RFC 4271 type 1).
  - **prefix-list <name>** `list`
    Named prefix-list filter instance. Each list contains an ordered set of match entries. Entries are evaluated in order; first match wins. No match = implicit deny.
    - **entry <prefix>** `list`
      Ordered match entry. First match wins per route prefix.
      - **action** `enumeration`
        Action applied when this entry matches a route prefix.
      - **ge** `uint8`
        Minimum match length (greater-than-or-equal). Defaults to the prefix length of this entry.
      - **le** `uint8`
        Maximum match length (less-than-or-equal). Defaults to 32 for IPv4 or 128 for IPv6.
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
    Per-source-peer worker channel capacity. Env var ze.bgp.route-server.worker-queue-size overrides this value.
- **router-id** `ipv4-address`
  BGP Router ID (required)
- **rpki** `container`
  - **action** `container`
    Global origin-validation actions (RFC 6811 Section 3)
    - **invalid** `validation-action`
      Action for routes with Invalid validation state
    - **not-found** `validation-action`
      Action for routes with NotFound validation state
  - **aspa** `container`
    - **action** `container`
      - **invalid** `validation-action`
        Action for routes with ASPA Invalid path verification state
      - **unknown** `validation-action`
        Action for routes with ASPA Unknown path verification state
    - **validation** `boolean`
      Enable ASPA path verification using RTR v2 ASPA records
  - **cache-server <address>** `list`
    - **port** `uint16`
      RTR TCP port
    - **preference** `uint8`
      Server preference (lower = preferred)
    - **source-address** `ip-address`
      Source IP address for outbound RTR connections
  - **validation-timeout** `uint16`
    Fail-open timeout for pending routes
- **session** `container`
  Global BGP session defaults
  - **asn** `container`
    AS number configuration
    - **local** `asn`
      Local Autonomous System Number (required)
- **update** `list`
  Native Ze route announcements
  - **attribute** `container`
    Path attributes for routes in this block
    - **aggregator** `string`
      AGGREGATOR attribute (RFC 4271 Section 5.1.7). Format: AS:IP. Identifies the AS and router that formed the aggregate.
    - **as-path** `string[]`
      AS_PATH segments prepended to the route. Space-separated ASNs. Affects loop detection and path selection (shorter preferred).
    - **atomic-aggregate** `boolean`
      ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6). Signals that the route was formed by aggregation and AS_PATH information was lost.
    - **attribute** `string[]`
      Raw BGP attributes in hex encoding. For attributes not covered by named fields. Format: flags:type:hex-value.
    - **bgp-prefix-sid** `container`
      Prefix-SID attribute (RFC 8669). Carries an MPLS label index for Segment Routing, allowing a node to advertise its SID without per-prefix label allocation.
    - **bgp-prefix-sid-srv6** `container`
      SRv6 Prefix-SID attribute (RFC 9252). Carries SRv6 SID information for IPv6 Segment Routing, including SRv6 L3 Service TLVs.
    - **cluster-list** `ipv4-address[]`
      CLUSTER_LIST for route reflection (RFC 4456). Each reflector prepends its cluster ID. Used for loop detection between reflectors.
    - **community** `string[]`
      Standard BGP communities (RFC 1997). Format: AS:value (e.g., 65000:100) or well-known names (no-export, no-advertise).
    - **extended-community** `string[]`
      Extended communities (RFC 4360). Used for route targets, site-of-origin, and VPN semantics. Format: type:admin:value.
    - **label** `string`
      MPLS label value for labeled unicast routes (RFC 3107). Numeric or keyword.
    - **labels** `string[]`
      MPLS label stack for multi-label routes (RFC 8277). Ordered bottom-to-top.
    - **large-community** `string[]`
      Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively.
    - **local-preference** `uint32`
      LOCAL_PREF value for iBGP best-path selection. Higher wins. Only meaningful within a single AS (RFC 4271 Section 5.1.5). Default: 100.
    - **med** `uint32`
      Multi-Exit Discriminator (MED). Lower values are preferred. Used to influence inbound path selection by an external peer (RFC 4271 Section 5.1.4).
    - **next-hop** `union`
      Next hop address or 'self'
    - **origin** `enumeration`
      ORIGIN attribute (RFC 4271 Section 4.3)
    - **originator-id** `ipv4-address`
      ORIGINATOR_ID for route reflection (RFC 4456). Set by the reflector to the originating router's ID. Used for loop prevention.
    - **path-information** `string`
      ADD-PATH path identifier (RFC 7911). Distinguishes multiple paths for the same prefix from the same peer.
    - **rd** `route-distinguisher`
      Route Distinguisher (RFC 4364). Prepended to the prefix to create a unique VPN route. Format: ASN:nn or IP:nn.
    - **split** `string`
      Split a single prefix into multiple more-specifics for announcement. Format: /length (e.g., /24 splits a /16 into 256 /24s).
  - **name** `string`
    Optional label for this update block (display only)
  - **nlri <name>** `list`
    - **content** `string`
      Operation, qualifiers, and payload
  - **watchdog** `container`
    Watchdog-controlled route - held until 'bgp watchdog announce <name>'
    - **name** `string`
      Watchdog group name
    - **withdraw** `boolean`
      Start in withdrawn state (default true)

## class-of-service

*Provided by `cos` ([ze-cos-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/cos/yang/ze-cos-conf.yang))*

Named class-of-service profiles

- **ieee-802.1p <name>** `list`
  An 802.1p QoS profile. Defines the mapping between the 3-bit PCP field in the 802.1Q header and internal priorities, for both ingress and egress directions.
  - **egress** `container`
    Priority-to-PCP mapping for transmitted tagged frames
    - **priority <value>** `list`
      Map an internal priority to a transmitted PCP value
      - **pcp** `uint8`
        PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)
  - **ingress** `container`
    PCP-to-priority mapping for received tagged frames
    - **pcp <value>** `list`
      Map a received PCP value to an internal priority
      - **priority** `uint8`
        Internal priority assigned to matching frames

## connected

*Provided by `connected` ([ze-connected-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/connected/yang/ze-connected-conf.yang))*

Connected route redistribution. Presence enables the plugin.


## control-plane-protection

*Provided by `copp-input-chain` ([ze-copp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/copp/yang/ze-copp-conf.yang))*

Control-plane policing configuration.

- **bgp** `container`
  Rate-limit new TCP connections to the BGP listen port.
  - **burst** `uint32`
    Burst size (packets or bytes matching the rate unit).
  - **over-limit-policy** `enumeration`
    Action for packets exceeding the rate limit. Default is accept to avoid lock-out risk.
  - **protected-port** `uint16[]`
    TCP port(s) to protect. Defaults to 179 (BGP) when omitted. Set to a non-default value when the BGP listener uses an alternate port.
  - **rate** `rate-spec`
    Rate limit for new connections (e.g., 100/second).
  - **trusted-source** `string[]`
    Source prefixes that bypass the rate limit. Typically the addresses of configured BGP peers.

## ddos

Distributed denial-of-service detection and mitigation subsystem.

- **detect** `container`
  *Provided by `ddos-detect-flow-source` ([ze-ddos-detect-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ddos/detect/yang/ze-ddos-detect-conf.yang))*
  - **absolute-floor** `uint32`
    Minimum threshold in PPS regardless of baseline.
  - **baseline-window** `uint32`
    Rolling baseline window size in samples.
  - **bps-floor** `uint64`
    Minimum bandwidth in bits/sec below which the BPS trigger is inert (default 50 Mbps). Expressed in bits/sec (operator unit); the detector's internal rate is bytes/sec and converts.
  - **bps-threshold-multiplier** `decimal-2`
    Baseline p99 multiplier for the bandwidth (BPS) trigger. Catches low-PPS/high-bandwidth amplification (NTP/memcached/CLDAP) that the packet-rate threshold misses.
  - **bps-trigger-enable** `boolean`
    Enable the bandwidth (BPS) trigger alongside the PPS threshold. When false only the packet-rate threshold can trigger detection.
  - **characterize-enable** `boolean`
    Run Stage-2 flow characterization (classify family + narrowest vector from the flow-export recent-flow ring, emit AttackCharacterized). When false the detector still emits the coarse AttackDetected target from traffic-usage.
  - **characterize-timeout** `uint16`
    Milliseconds budget for the on-trigger traffic-usage and flow-recent queries; on timeout the detector falls back to the best available target.
  - **characterize-window** `uint16`
    Seconds of recent flows to consider when characterizing; flows last seen before this window are ignored. Flows without a timestamp are always kept.
  - **check-interval** `uint16`
    Seconds between detection evaluations.
  - **clear-consecutive-checks** `uint16`
    Consecutive ticks below threshold before clearing.
  - **confirm-duration** `uint16`
    Consecutive ticks above threshold before triggering.
  - **enabled** `boolean`
    Enable the DDoS detector.
  - **entropy-threshold** `decimal-2`
    Source-address Shannon entropy (bits) at or above which an attack is logged as distributed/spoofed. 0 = a single source; higher = more sources.
  - **policy** `container`
    Allow/deny traffic policy applied to detected attacks, indexed by prefix and evaluated longest-prefix-match (most specific wins), NOT config order. Replaces the removed per-responder allowlists: the detector enforces it once and encodes the exempt/mitigate decision on the emitted event so the responders honor it without reading config.
    - **default-action** `enumeration`
      Disposition when no rule matches: deny = defend (detection + mitigation); allow = exempt from detection entirely.
    - **rule <prefix>** `list`
      One allow/deny rule matched against an attack, keyed by prefix. Precedence is longest-prefix-match, so a /24 rule beats a covering /16 without any ordering.
      - **action** `enumeration`
        allow = exempt matching traffic; deny = subject it to DDoS handling.
      - **match** `enumeration`
        Whether the prefix matches the attack source, the victim destination, or either. Source rules take effect once the attack sources are characterized.
      - **scope** `enumeration`
        Stage the action governs: mitigation = detect and record but (for allow) do not block; detection = (for allow) suppress the incident entirely.
  - **startup-grace** `uint16`
    Seconds after startup where only extreme spikes trigger.
  - **threshold-multiplier** `decimal-2`
    Baseline p99 multiplier for the dynamic threshold.
  - **top-n-sources** `uint16`
    Maximum number of attacker source addresses ranked into TopSources by packet volume.
- **flowspec** `container`
  *Provided by `ddos-flowspec` ([ze-ddos-flowspec-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang))*
  - **action** `enumeration`
    FlowSpec traffic-action. Mandatory, with no default because neither drop nor rate-limit is universally safe: discard announces an RFC 8955 traffic-rate of 0 (drops the characterized attack flow); rate-limit announces a byte rate and requires an explicit rate-limit-bytes. A rate-limit-bytes of 0 is a valid choice and is equivalent to discard on the wire.
  - **announce-rate-limit** `uint16`
    Maximum FlowSpec announcements per minute.
  - **backoff-cap** `uint32`
    Maximum hold-down after exponential backoff.
  - **blackhole-fallback** `boolean`
    When true, a critical-severity AttackDetected (peak >= 5x threshold) auto-engages an immediate upstream discard (RTBH-style) without waiting for characterization. When false (default) the upstream rule is announced only from AttackCharacterized, so it is precise before the box goes blind behind the filter.
  - **confidence-min** `uint8`
    Minimum incident confidence (0-100) required to announce an upstream rule from a characterized attack. 0 (default) disables the gate. The blackhole-fallback fast path is never gated (AttackDetected carries no confidence).
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
    Bytes per second announced in the RFC 8955 traffic-rate extended community. Required (and validated) when action is rate-limit; there is no default because no rate is universally safe. A value of 0 is valid and encodes the same traffic-rate-0 as discard. Not used when action is discard. Set it to the per-flow rate you are willing to pass to the victim during mitigation.
  - **response-level** `enumeration`
    Action on attack detection: alert (log only) or enforce (announce FlowSpec).
- **flowtriq** `container`
  *Provided by `ddos-flowtriq` ([ze-ddos-flowtriq-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ddos/flowtriq/yang/ze-ddos-flowtriq-conf.yang))*
  - **api-base** `string`
    Flowtriq API base URL.
  - **api-key** `string`
    Flowtriq API bearer token.
  - **enabled** `boolean`
    Enable reporting DDoS incidents to the Flowtriq cloud API.
  - **node-uuid** `string`
    Node UUID for Flowtriq agent identification.
- **local** `container`
  *Provided by `ddos-local` ([ze-ddos-local-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang))*
  - **confidence-min** `uint8`
    Minimum incident confidence (0-100) required to install a drop rule from a characterized attack. 0 (default) disables the gate. Higher values suppress mitigation on borderline detections.
  - **forward-mitigation** `boolean`
    When true, also drop a remote (transit) victim's traffic on the netfilter FORWARD hook to protect a downstream host. Default false: the responder guards only local (box-owned) victims on INPUT and leaves remote victims to the flowspec upstream announce. The exempt/mitigate decision itself comes from the ddos/detect policy.
  - **max-mitigation-duration** `uint32`
    Maximum seconds a drop rule stays installed (0 = no cap).
  - **response-level** `enumeration`
    Action on attack detection: alert (log only) or enforce (install drop rule).
- **observe** `container`
  *Provided by `ddos-observe` ([ze-ddos-observe-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ddos/observe/yang/ze-ddos-observe-conf.yang))*
  - **incident-ring-size** `uint32`
    Maximum number of incidents to retain in memory.
  - **stale-incident-timeout** `uint32`
    Seconds before an open incident without a clear event is auto-finalized.

## environment

*Provided by `bgp-bmp` ([ze-bmp-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-cmd.yang), [ze-bmp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang)); `ntp` ([ze-ntp-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ntp/yang/ze-ntp-cmd.yang), [ze-ntp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ntp/yang/ze-ntp-conf.yang))*

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
    REST/HTTP API transport. Plain HTTP listeners are restricted to loopback; expose remotely through a TLS terminator.
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
    Bearer token for API authentication. When set, clients must send Authorization: Bearer <token> header. Leave empty to allow unauthenticated access.
- **bgp** `container`
  BGP protocol environment settings: global defaults that apply before any peer-level config. Controls OPEN wait timeout and initial announce delay.
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
  gNMI (gRPC Network Management Interface) server settings
  - **enabled** `boolean`
    Enable gNMI server
  - **server <name>** `list`
    gNMI server listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **tls** `container`
    TLS certificate for gNMI gRPC transport
    - **cert** `string`
      Path to PEM-encoded certificate file (or chain)
    - **key** `string`
      Path to PEM-encoded private-key file
  - **token** `string`
    Bearer token for gNMI authentication. Compared constant-time (sha256 + subtle.ConstantTimeCompare). Empty means no authentication.
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
    Log destination: stdout, stderr, syslog, or filename
  - **level** `string`
    Base log level: DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL (case-insensitive)
  - **relay** `string`
    Plugin stderr relay level: DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL (case-insensitive)
- **looking-glass** `container`
  Looking glass HTTP server settings
  - **enabled** `boolean`
    Enable looking glass
  - **server <name>** `list`
    Looking glass listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **tls** `boolean`
    Enable TLS (requires blob storage for certificates)
- **mcp** `container`
  MCP server settings (AI assistant control interface)
  - **auth-mode** `enumeration`
    Authentication strategy. Defaults: if token is set and auth-mode is absent, auth-mode is inferred as 'bearer' for back-compat; otherwise 'none'.
  - **bind-remote** `boolean`
    Allow binding to non-loopback addresses. Default (false) keeps the Phase 1 clamp that forces every server entry to 127.0.0.1. When true, the operator configured ip is honored, and auth-mode MUST be set to a non-'none' value (verify-time reject).
  - **enabled** `boolean`
    Enable MCP server
  - **identity <name>** `list`
    Per-identity bearer entries (auth-mode=bearer-list). Each entry's token grants access to the named principal. Scopes are attached to the session and visible to later phases (tasks, elicitation).
    - **scope** `string[]`
      Optional scopes attached to the session when this identity authenticates.
    - **token** `string`
      Bearer token value. Compared constant-time.
  - **oauth** `container`
    OAuth 2.1 resource-server settings (auth-mode=oauth). Ze does NOT run an authorization server; it validates bearer tokens issued by the external AS.
    - **audience** `string`
      Canonical URL identifying this MCP endpoint (RFC 8707). Tokens whose 'aud' claim does not match are rejected. Must be set explicitly; never derived from the request Host header.
    - **authorization-server** `string`
      HTTPS URL of the authorization server. The resource server reads AS metadata from <url>/.well-known/oauth-authorization-server (RFC 8414) to discover the jwks_uri.
    - **required-scopes** `string[]`
      Scopes every accepted token must carry. Empty list means any valid token is accepted.
  - **server <name>** `list`
    MCP server listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **tls** `container`
    TLS certificate for HTTPS. Required when auth-mode=oauth on a non-loopback bind.
    - **cert** `string`
      Path to PEM-encoded certificate file (or a chain).
    - **key** `string`
      Path to PEM-encoded private-key file.
  - **token** `string`
    Bearer token for auth-mode=bearer. When set without an explicit auth-mode leaf, auth-mode is inferred as 'bearer'. Leave empty when using bearer-list or oauth.
- **ntp** `container`
  NTP client settings
  - **enabled** `boolean`
    Enable NTP time synchronization
  - **interval** `uint32`
    Sync interval in seconds
  - **max-step** `uint32`
    Maximum accepted NTP clock step in seconds. A value of 0 explicitly allows unlimited steps.
  - **persist-path** `string`
    Path to save time on shutdown for recovery
  - **server <name>** `list`
    NTP server pool entries
    - **address** `string`
      NTP server hostname or IP address
  - **slew-threshold** `uint32`
    Maximum offset in milliseconds for gradual clock slew via Adjtimex. Offsets above this threshold use Settimeofday (step). A value of 0 disables slew (always step).
- **pprof** `string`
  pprof HTTP server address (e.g. :6060). Empty disables pprof.
- **reactor** `container`
  BGP reactor engine tuning. The reactor is the core event loop that processes incoming messages, runs peer FSMs, and dispatches outbound UPDATEs.
  - **cache-max** `uint32`
    Maximum number of cached UPDATE wire messages. Bounds memory usage of the encoding cache. 0 means unlimited (cache grows with the route table).
  - **cache-ttl** `uint32`
    Seconds to keep recently-built UPDATE wire messages in the encoding cache. Cached UPDATEs are reused for peers with the same encoding context, avoiding redundant serialization. 0 disables caching (every UPDATE is built fresh).
  - **forward-batch-limit** `uint32`
    Max items per drain batch. Bounds writeMu hold time during forward dispatch. 0 means unlimited.
  - **forward-pool-headroom** `uint32`
    Extra bytes beyond auto-sized pool baseline (max ~4GB). Ignored when forward-pool-max-bytes is explicitly set.
  - **forward-pool-max-bytes** `uint32`
    Combined byte budget for 4K+64K buffer pools in bytes (max ~4GB). 0 means unlimited (auto-sized from peer prefix maximums).
  - **forward-queue-size** `uint32`
    Per-destination forward channel capacity. Controls how many UPDATE items can be buffered per destination peer before backpressure kicks in.
  - **forward-teardown-grace** `string`
    Grace period before forced teardown on congestion (duration string, e.g. 5s, 1m).
  - **read-buffer-size** `uint32`
    Per-session TCP read buffer size in bytes.
  - **speed** `string`
    Reactor loop cycle time multiplier. Values below 1.0 run faster (lower latency, higher CPU). Values above 1.0 run slower (higher latency, lower CPU). Range: 0.1 to 10.0.
  - **update-groups** `boolean`
    Build each UPDATE once and send to all peers with the same encoding context (same capabilities, same address families). Reduces CPU from O(peers x routes) to O(groups x routes). Disable only for debugging.
  - **write-buffer-size** `uint32`
    Per-session TCP write buffer size in bytes.
- **ssh** `container`
  SSH server settings
  - **enabled** `boolean`
    Enable SSH server
  - **host-certificate** `string`
    Path to SSH host certificate file (signed by a CA). Eliminates trust-on-first-use for clients that trust the CA.
  - **host-key** `string`
    Path to SSH host key file (default: config dir + ssh_host_ed25519_key, auto-generated if missing)
  - **idle-timeout** `uint32`
    Idle timeout in seconds (default 600)
  - **max-sessions** `uint16`
    Maximum concurrent SSH sessions
  - **server <name>** `list`
    SSH server listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
- **web** `container`
  Web interface settings
  - **enabled** `boolean`
    Enable web interface
  - **insecure** `boolean`
    Disable authentication (forces host to 127.0.0.1)
  - **server <name>** `list`
    Web server listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **ui-mode** `enumeration`
    Web UI mode

## exabgp

*Provided by `exabgp-bridge` ([ze-exabgp-bridge-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/exabgp/bridgeplugin/yang/ze-exabgp-bridge-conf.yang))*

Top-level container for ExaBGP-compatibility configuration.

- **bridge** `container`
  In-process ExaBGP bridge settings. Present this container to configure the internal exabgp-bridge plugin.
  - **add-path** `enumeration`
    ADD-PATH mode (RFC 7911) for the negotiated families: none (disabled), receive, send, or both.
  - **family** `address-family[]`
    Address families the bridge negotiates on behalf of the script (e.g. ipv4/unicast). Defaults to ipv4/unicast when no entry is configured.
  - **route-refresh** `boolean`
    Advertise the BGP route-refresh capability (RFC 2918) on the sessions the bridge feeds.
  - **run** `string`
    Command line of the ExaBGP-format script to run as a subprocess (e.g. './plugin.py' or 'python3 /opt/plugin.py'). Whitespace-separated; the first token is the executable and the rest are its arguments. Required for the bridge to do anything.

## fib

Forwarding Information Base configuration.

- **kernel** `container`
  *Provided by `fib-kernel` ([ze-fib-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/fib/kernel/yang/ze-fib-conf.yang))*
  OS kernel route programming via netlink (Linux).
  - **flush-on-stop** `boolean`
    Remove all ze-installed routes when the plugin stops. When false (default), routes persist in the kernel after shutdown for graceful restart support.
  - **sweep-delay** `uint16`
    Time to wait after startup before sweeping stale routes. Allows BGP reconvergence to refresh matching routes.
- **p4** `container`
  *Provided by `fib-p4` ([ze-fib-p4-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/fib/p4/yang/ze-fib-p4-conf.yang))*
  P4 switch route programming via gRPC/P4Runtime.
  - **device-id** `uint64`
    P4Runtime device ID.
  - **flush-on-stop** `boolean`
    Remove all forwarding entries when the plugin stops.
  - **target** `string`
    P4Runtime gRPC target address (host:port). Example: 127.0.0.1:9559
- **vpp** `container`
  *Provided by `fib-vpp` ([ze-fib-vpp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/fib/vpp/yang/ze-fib-vpp-conf.yang))*
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

*Provided by `firewall-irr` ([ze-firewall-irr-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang), [ze-firewall-irr.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang)); `firewall` ([ze-firewall-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/firewall/yang/ze-firewall-cmd.yang), [ze-firewall-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/firewall/yang/ze-firewall-conf.yang))*

Ze-managed nftables firewall tables. Table names are bare in config; ze_ prefix added by component.

- **backend** `string`
  Firewall backend implementation. Default is nft (nftables via libnftnl). Future backends can declare themselves via firewall.RegisterBackend. The ze:backend YANG extension on feature nodes declares per-feature backend support so the commit-time gate rejects configs that try to use unsupported primitives.
- **flush-on-shutdown** `boolean`
  Remove ze-owned nftables tables from the kernel when the ze process stops in an orderly way (SIGTERM), default true, so a stopped daemon leaves no rules behind. Set false to use ze as a one-shot provisioner: program the rules, let the process exit, and leave them running in the kernel (like nft -f). Keys off how the process exits; unrelated to BGP graceful restart. A crash never runs the shutdown path, so tables always persist across a crash regardless of this setting. Governs every ze table producer (firewall, control-plane-protection, policy-routes, ddos-local), which share one backend.
- **global-options** `container`
  Network security defaults mapped to kernel sysctls. Provides VyOS-compatible keyword toggles for common settings. At apply time each keyword translates to a sysctl key/value pair emitted to the sysctl plugin. Explicit sysctl settings always override these.
  - **all-ping** `enumeration`
    Control ICMP echo (ping) responses. Inverted sysctl: enable sets icmp_echo_ignore_all to 0.
  - **broadcast-ping** `enumeration`
    Control broadcast ICMP echo responses. Inverted sysctl: enable sets icmp_echo_ignore_broadcasts to 0.
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
    Per-interface AS-SET binding for ingress source validation. Packets arriving on this interface with source addresses not in the AS-SET's IRR-resolved prefixes are dropped.
    - **source-as-set** `string`
      AS-SET whose IRR-resolved prefixes form the allowed source-address set for this interface. Requires cached prefix data; config commit rejects if no cached data exists.
  - **peeringdb-url** `string`
    Base URL for PeeringDB API queries. Override for testing with a mock server.
  - **refresh-interval** `uint32`
    Seconds between automatic IRR re-queries. 0 (default) disables automatic refresh; the operator must use 'update firewall irr' manually. A value in 60..86400 enables periodic refresh with fail-closed semantics (failed refresh preserves last-good cache).
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
          Connection mark value/mask. Conntrack-driven; see connection-state for backend notes.
        - **connection-state** `connection-state`
          Connection tracking state(s). Conntrack-driven; requires a backend that integrates with the kernel nf_conntrack module.
        - **destination-address** `ip-prefix`
          Destination IP prefix or @set-name
        - **destination-as-set** `string`
          Match destination address against IRR-resolved prefixes for this AS-SET. Same caching semantics as source-as-set.
        - **destination-asn** `uint32`
          Match destination address against IRR-resolved prefixes for this ASN. Same caching semantics as source-asn.
        - **destination-port** `port-spec`
          Destination port, range, or list
        - **dscp** `dscp-value`
          DSCP value
        - **icmp-type** `string`
          ICMPv4 type. Accepts symbolic names from nftables (echo-request, echo-reply, destination-unreachable, time-exceeded, ...) or a numeric byte value 0..255.
        - **icmpv6-type** `string`
          ICMPv6 type. Accepts symbolic names from nftables (echo-request, nd-neighbor-solicit, packet-too-big, ...) or a numeric byte value 0..255.
        - **input-interface** `string`
          Input interface name. A trailing asterisk (e.g. 'l2tp*') matches any interface whose name begins with the given prefix.
        - **mark** `mark-value`
          Packet mark value/mask
        - **output-interface** `string`
          Output interface name. A trailing asterisk (e.g. 'veth*') matches any interface whose name begins with the given prefix.
        - **protocol** `protocol-name`
          L4 protocol
        - **source-address** `ip-prefix`
          Source IP prefix or @set-name
        - **source-as-set** `string`
          Match source address against IRR-resolved prefixes for this AS-SET. Requires cached prefix data (run 'update firewall irr as-set <name>' first). Config commit rejects if no cached data exists.
        - **source-asn** `uint32`
          Match source address against IRR-resolved prefixes for this ASN. Requires cached prefix data (run 'update firewall irr asn <N>' first). Config commit rejects if no cached data exists.
        - **source-port** `port-spec`
          Source port, range, or list
      - **then** `container`
        Actions and modifiers
        - **accept** `container`
          Accept the packet
        - **connection-mark-set** `container`
          Conntrack-driven; see mark-set for backend notes.
          - **value** `mark-value`
            Mark value with optional mask
        - **counter** `container`
          Anonymous per-rule counter. A non-empty `name` leaf is reserved for a future named-counter feature and is rejected at verify today.
          - **name** `string`
            Counter name (reserved; non-empty values reject at verify)
        - **dnat** `container`
          - **to** `nat-spec`
            NAT target address:port
        - **drop** `container`
          Drop the packet
        - **dscp-set** `dscp-value`
          Set DSCP field value
        - **exclude** `container`
          In a NAT chain term, skip NAT for matching traffic. Lowers to the same `return` verdict the operator could have written directly; the keyword exists for VyOS-config parity.
        - **flow-offload** `container`
          Hardware / software flow offload via an nftables flowtable. The VPP dataplane is itself the fast path and does not expose a flowtable surface.
          - **flowtable** `string`
            Flowtable name (prefixed with @)
        - **goto** `string`
          Goto target chain (no return)
        - **jump** `string`
          Jump to target chain
        - **limit-rate** `container`
          - **burst** `uint32`
            Burst size
          - **rate** `rate-spec`
            Rate limit (e.g., 10/second)
        - **log** `container`
          - **level** `uint32`
            Log level (syslog severity)
          - **prefix** `string`
            Log prefix string
        - **mark-set** `container`
          Write the kernel skb mark. Different backends carry per-packet metadata differently; the VPP classifier's equivalent uses opaque keys via ClassifyAddDelSession.
          - **value** `mark-value`
            Mark value with optional mask
        - **masquerade** `container`
          Masquerade (source NAT with outgoing interface address). Optionally restrict source port range with port-range, or set randomization/persistence flags. Port mapping and flags are mutually exclusive.
          - **persistent** `empty`
            Use consistent source port mapping per connection
          - **port-range** `string`
            Source port or port range (e.g. 1024-65535)
          - **random** `container`
            Randomize source port selection. With 'full', use full randomization.
            - **full** `empty`
              Use full randomization
        - **notrack** `container`
          Disable connection tracking. Conntrack-specific.
        - **redirect** `container`
          - **to** `port`
            Local port number
        - **reject** `container`
          - **code** `uint8`
            ICMP code
          - **with** `reject-type`
            Reject response type
        - **return** `container`
          Return to caller chain
        - **snat** `container`
          - **to** `nat-spec`
            NAT target address:port
        - **tcp-mss-set** `uint16`
          Clamp TCP Maximum Segment Size option to the given value
    - **type** `chain-type`
      Chain type (base chains only)
  - **family** `table-family`
    Address family
  - **flowtable <name>** `list`
    Hardware offload flowtable. nftables-specific; the VPP dataplane is itself the fast path and does not expose a flowtable surface.
    - **device** `string[]`
      Devices for offload
    - **hook** `chain-hook`
      Hook point (typically ingress)
    - **priority** `int32`
      Priority (typically negative)
  - **set <name>** `list`
    Named set
    - **element <value>** `list`
      Static set elements. The value form is interpreted per set type: ipv4 / ipv6 accepts addresses or prefixes (when flags-interval is set); inet-service accepts port numbers; mark accepts a numeric or 0x-prefixed value; ether accepts aa:bb:cc:dd:ee:ff; ifname accepts an interface name.
      - **timeout** `uint32`
        Per-element timeout in seconds. 0 means no timeout. Requires flags-timeout on the enclosing set.
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

*Provided by `flow-export-conntrack-tracking` ([ze-flowexport-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/flowexport/yang/ze-flowexport-conf.yang))*

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
    Capacity (in flow records) of the recent-flow ring queried by 'show flow recent'. The ring feeds on-box DDoS characterization; larger values retain more history at higher memory cost. Only allocated when conntrack export is enabled.
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

*Provided by `interface` ([ze-iface-api.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/iface/yang/ze-iface-api.yang), [ze-iface-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/iface/yang/ze-iface-cmd.yang), [ze-iface-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/iface/yang/ze-iface-conf.yang), [ze-iface-interface-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/iface/yang/ze-iface-interface-cmd.yang), [ze-iface-monitor-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/iface/yang/ze-iface-monitor-cmd.yang), [ze-iface-show-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/iface/yang/ze-iface-show-cmd.yang)); `vrrp` ([ze-vrrp-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/vrrp/yang/ze-vrrp-cmd.yang), [ze-vrrp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/vrrp/yang/ze-vrrp-conf.yang))*

Interface-level class-of-service bindings and inline QoS maps (container-merge with ze-iface-conf). Removing the cos plugin removes all QoS surface from interfaces.

- **backend** `string`
  Interface management backend (e.g., netlink, networkd)
- **bridge <name>** `list`
  Linux bridge interface. Forwards Ethernet frames between member ports at L2. Created by ze if it does not exist. Member interfaces are enslaved at apply time.
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none' to explicitly disable inheritance. When set on the interface, all VLAN units inherit the profile unless overridden per-unit.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings: override the operational address (address) and/or select the kernel device by its MAC (match). The two are independent: a NIC can be matched by its permanent (factory) MAC AND have its operational MAC overridden at the same time.
    - **address** `string`
      Override the kernel-assigned MAC address with this explicit colon-separated hex value (e.g., 02:42:ac:11:00:02). Applied at interface creation time. When omitted, the kernel assigns a MAC automatically.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC, instead of binding by name. The resolver matches the device's PERMANENT (factory) address (IFLA_PERM_ADDRESS) when it reports one, so the binding survives an operational MAC override; for virtual devices that report no permanent address it matches the current address. Takes precedence over os-name. Applies to ethernet (the matched physical kind) only; the Ze-created kinds (dummy/veth/bridge/...) are identified by the name Ze assigns. When the matching device is absent the binding defers until it appears.
  - **member** `string[]`
    Member port interface names
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features. Applied directly via kernel ioctl (the ethtool SIOCETHTOOL interface) or sysfs writes after the interface is created. The ethtool(8) CLI program is NOT required; ze talks to the kernel directly. Boolean leaves: true = explicitly enable the feature, false = explicitly disable, absent = preserve whatever the OS default is (no kernel call is made). Offload availability depends on the NIC driver and kernel version; unsupported features are logged as warnings and do not block the config commit.
    - **gro** `boolean`
      Generic Receive Offload. Aggregates small incoming packets into larger ones before passing them up the network stack, reducing per-packet CPU overhead. Software-based (works on all NIC types including veth and dummy). Applied via kernel ioctl (ETHTOOL_SGRO). Equivalent to: ethtool -K <dev> gro on|off.
    - **gso** `boolean`
      Generic Segmentation Offload. Delays segmentation of large outgoing packets until just before the NIC driver transmit path, reducing CPU work in the upper stack. Software-based counterpart of TSO. Applied via kernel ioctl (ETHTOOL_SGSO). Equivalent to: ethtool -K <dev> gso on|off.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload. Enables the NIC to execute TC flower / u32 filter rules in hardware, bypassing kernel processing for matched flows. Requires NIC firmware support (common on mlx5, bnxt, nfp). Applied via kernel ioctl (ETHTOOL_SFEATURES). Equivalent to: ethtool -K <dev> hw-tc-offload on|off.
    - **lro** `boolean`
      Large Receive Offload. Hardware-based counterpart of GRO: the NIC coalesces incoming TCP segments before DMA. Can break forwarding and bridging because the coalesced frame no longer matches the original wire format. Disable on routers or bridges that forward traffic between interfaces. Applied via kernel ioctl (ETHTOOL_SLRO). Equivalent to: ethtool -K <dev> lro on|off.
    - **rfs** `boolean`
      Receive Flow Steering. Extension of RPS that steers incoming packets to the CPU where the application consuming that flow is running, improving cache locality and reducing cross-CPU cache bounces. When enabled, ze sets /proc/sys/net/core/ rps_sock_flow_entries to 32768 and distributes per-queue rps_flow_cnt evenly across rx queues. RPS should also be enabled for RFS to be effective. Not an ethtool feature; uses sysfs directly.
    - **rps** `boolean`
      Receive Packet Steering. Software-based distribution of incoming packets across multiple CPUs by hashing the packet header and steering to a target CPU receive queue. Useful when the NIC has fewer hardware RSS queues than available CPUs. When enabled, ze writes a bitmask covering all online CPUs to each rx queue sysfs entry (/sys/class/net/<dev>/queues/rx-*/ rps_cpus). When disabled, ze writes 0 to each entry. Not an ethtool feature; uses sysfs directly.
    - **sg** `boolean`
      Scatter-Gather I/O. Allows the NIC to assemble a single frame from multiple non-contiguous memory buffers, avoiding data copies. Required by TSO and GSO on most drivers. Applied via kernel ioctl (ETHTOOL_SSG). Equivalent to: ethtool -K <dev> sg on|off.
    - **tso** `boolean`
      TCP Segmentation Offload. Offloads TCP segmentation to the NIC hardware, allowing the kernel to hand off large (up to 64 KB) TCP segments that the NIC splits into MTU-sized frames on the wire. Requires NIC hardware support. Disable when passing traffic to VPP or virtual switches that cannot handle oversized frames. Applied via kernel ioctl (ETHTOOL_STSO). Equivalent to: ethtool -K <dev> tso on|off.
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
  - **stp** `boolean`
    Enable Spanning Tree Protocol
  - **unit <name>** `list`
    Logical interface unit
    - **class-of-service** `string`
      Per-unit override: profile name or 'none' to opt out of inheritance from the parent interface.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of outgoing packets to the 802.1p PCP value stamped in the 802.1Q header. Requires vlan-id. Priorities with no entry are sent with PCP 0.
      - **pcp** `uint8`
        PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of received tagged frames to an internal priority. Requires vlan-id. Frames whose PCP has no entry keep priority 0.
      - **priority** `uint8`
        Internal priority assigned to matching frames
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3): a non-owner Active accepts packets addressed to the virtual addresses. v3 semantics only; combining true with version 2 is rejected by the plugin verifier. Drives FSM/owner semantics only (not dataplane-enforced this pass).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds. The native range spans both versions; the plugin verifier narrows it per version. v3: multiples of 10 ms within 10..40950 (wire = centiseconds, RFC 9568 erratum 8301). v2: whole seconds within 1000..255000 (wire = seconds).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2): a higher-priority Backup takes over from a live lower-priority Active router.
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568). Armed on the first losing advert while a higher-priority local router waits; never delays dead-master failover.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically; it is never operator-set.
          - **version** `enumeration`
            Protocol version. Present only for IPv4 groups; IPv6 is always VRRPv3 (RFC 9568).
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order (RFC 9568 Section 5.2.9). An address equal to a real address on this unit makes this router the owner (priority forced to 255, accept-mode forced true).
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds, multiples of 10 ms (wire = centiseconds, RFC 9568 erratum 8301).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568).
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order. The FIRST address MUST be an IPv6 link-local (fe80::/10) address: it is the advertisement source identity (RFC 9568 Section 5.2.9, erratum 8300), enforced by the plugin verifier.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **dhcp-auto** `boolean`
  Auto-discover first ethernet interface and run DHCP on it. Used when the interface name is not known at config time (e.g., gokrazy appliance). Ignored if any explicit DHCP config exists.
- **dummy <name>** `list`
  Dummy (loopback-like) interface. Created by ze if it does not exist. Used for hosting service addresses that are not tied to a physical link (e.g., router-id, anycast VIPs).
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none' to explicitly disable inheritance. When set on the interface, all VLAN units inherit the profile unless overridden per-unit.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings: override the operational address (address) and/or select the kernel device by its MAC (match). The two are independent: a NIC can be matched by its permanent (factory) MAC AND have its operational MAC overridden at the same time.
    - **address** `string`
      Override the kernel-assigned MAC address with this explicit colon-separated hex value (e.g., 02:42:ac:11:00:02). Applied at interface creation time. When omitted, the kernel assigns a MAC automatically.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC, instead of binding by name. The resolver matches the device's PERMANENT (factory) address (IFLA_PERM_ADDRESS) when it reports one, so the binding survives an operational MAC override; for virtual devices that report no permanent address it matches the current address. Takes precedence over os-name. Applies to ethernet (the matched physical kind) only; the Ze-created kinds (dummy/veth/bridge/...) are identified by the name Ze assigns. When the matching device is absent the binding defers until it appears.
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features. Applied directly via kernel ioctl (the ethtool SIOCETHTOOL interface) or sysfs writes after the interface is created. The ethtool(8) CLI program is NOT required; ze talks to the kernel directly. Boolean leaves: true = explicitly enable the feature, false = explicitly disable, absent = preserve whatever the OS default is (no kernel call is made). Offload availability depends on the NIC driver and kernel version; unsupported features are logged as warnings and do not block the config commit.
    - **gro** `boolean`
      Generic Receive Offload. Aggregates small incoming packets into larger ones before passing them up the network stack, reducing per-packet CPU overhead. Software-based (works on all NIC types including veth and dummy). Applied via kernel ioctl (ETHTOOL_SGRO). Equivalent to: ethtool -K <dev> gro on|off.
    - **gso** `boolean`
      Generic Segmentation Offload. Delays segmentation of large outgoing packets until just before the NIC driver transmit path, reducing CPU work in the upper stack. Software-based counterpart of TSO. Applied via kernel ioctl (ETHTOOL_SGSO). Equivalent to: ethtool -K <dev> gso on|off.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload. Enables the NIC to execute TC flower / u32 filter rules in hardware, bypassing kernel processing for matched flows. Requires NIC firmware support (common on mlx5, bnxt, nfp). Applied via kernel ioctl (ETHTOOL_SFEATURES). Equivalent to: ethtool -K <dev> hw-tc-offload on|off.
    - **lro** `boolean`
      Large Receive Offload. Hardware-based counterpart of GRO: the NIC coalesces incoming TCP segments before DMA. Can break forwarding and bridging because the coalesced frame no longer matches the original wire format. Disable on routers or bridges that forward traffic between interfaces. Applied via kernel ioctl (ETHTOOL_SLRO). Equivalent to: ethtool -K <dev> lro on|off.
    - **rfs** `boolean`
      Receive Flow Steering. Extension of RPS that steers incoming packets to the CPU where the application consuming that flow is running, improving cache locality and reducing cross-CPU cache bounces. When enabled, ze sets /proc/sys/net/core/ rps_sock_flow_entries to 32768 and distributes per-queue rps_flow_cnt evenly across rx queues. RPS should also be enabled for RFS to be effective. Not an ethtool feature; uses sysfs directly.
    - **rps** `boolean`
      Receive Packet Steering. Software-based distribution of incoming packets across multiple CPUs by hashing the packet header and steering to a target CPU receive queue. Useful when the NIC has fewer hardware RSS queues than available CPUs. When enabled, ze writes a bitmask covering all online CPUs to each rx queue sysfs entry (/sys/class/net/<dev>/queues/rx-*/ rps_cpus). When disabled, ze writes 0 to each entry. Not an ethtool feature; uses sysfs directly.
    - **sg** `boolean`
      Scatter-Gather I/O. Allows the NIC to assemble a single frame from multiple non-contiguous memory buffers, avoiding data copies. Required by TSO and GSO on most drivers. Applied via kernel ioctl (ETHTOOL_SSG). Equivalent to: ethtool -K <dev> sg on|off.
    - **tso** `boolean`
      TCP Segmentation Offload. Offloads TCP segmentation to the NIC hardware, allowing the kernel to hand off large (up to 64 KB) TCP segments that the NIC splits into MTU-sized frames on the wire. Requires NIC hardware support. Disable when passing traffic to VPP or virtual switches that cannot handle oversized frames. Applied via kernel ioctl (ETHTOOL_STSO). Equivalent to: ethtool -K <dev> tso on|off.
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
  - **unit <name>** `list`
    Logical interface unit
    - **class-of-service** `string`
      Per-unit override: profile name or 'none' to opt out of inheritance from the parent interface.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of outgoing packets to the 802.1p PCP value stamped in the 802.1Q header. Requires vlan-id. Priorities with no entry are sent with PCP 0.
      - **pcp** `uint8`
        PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of received tagged frames to an internal priority. Requires vlan-id. Frames whose PCP has no entry keep priority 0.
      - **priority** `uint8`
        Internal priority assigned to matching frames
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3): a non-owner Active accepts packets addressed to the virtual addresses. v3 semantics only; combining true with version 2 is rejected by the plugin verifier. Drives FSM/owner semantics only (not dataplane-enforced this pass).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds. The native range spans both versions; the plugin verifier narrows it per version. v3: multiples of 10 ms within 10..40950 (wire = centiseconds, RFC 9568 erratum 8301). v2: whole seconds within 1000..255000 (wire = seconds).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2): a higher-priority Backup takes over from a live lower-priority Active router.
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568). Armed on the first losing advert while a higher-priority local router waits; never delays dead-master failover.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically; it is never operator-set.
          - **version** `enumeration`
            Protocol version. Present only for IPv4 groups; IPv6 is always VRRPv3 (RFC 9568).
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order (RFC 9568 Section 5.2.9). An address equal to a real address on this unit makes this router the owner (priority forced to 255, accept-mode forced true).
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds, multiples of 10 ms (wire = centiseconds, RFC 9568 erratum 8301).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568).
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order. The FIRST address MUST be an IPv6 link-local (fe80::/10) address: it is the advertisement source identity (RFC 9568 Section 5.2.9, erratum 8300), enforced by the plugin verifier.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **ethernet <name>** `list`
  Physical or virtual Ethernet interface. Ze manages the interface's addresses, MTU, offload settings, and units. The interface must already exist in the OS (ze does not create physical interfaces).
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none' to explicitly disable inheritance. When set on the interface, all VLAN units inherit the profile unless overridden per-unit.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings: override the operational address (address) and/or select the kernel device by its MAC (match). The two are independent: a NIC can be matched by its permanent (factory) MAC AND have its operational MAC overridden at the same time.
    - **address** `string`
      Override the kernel-assigned MAC address with this explicit colon-separated hex value (e.g., 02:42:ac:11:00:02). Applied at interface creation time. When omitted, the kernel assigns a MAC automatically.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC, instead of binding by name. The resolver matches the device's PERMANENT (factory) address (IFLA_PERM_ADDRESS) when it reports one, so the binding survives an operational MAC override; for virtual devices that report no permanent address it matches the current address. Takes precedence over os-name. Applies to ethernet (the matched physical kind) only; the Ze-created kinds (dummy/veth/bridge/...) are identified by the name Ze assigns. When the matching device is absent the binding defers until it appears.
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features. Applied directly via kernel ioctl (the ethtool SIOCETHTOOL interface) or sysfs writes after the interface is created. The ethtool(8) CLI program is NOT required; ze talks to the kernel directly. Boolean leaves: true = explicitly enable the feature, false = explicitly disable, absent = preserve whatever the OS default is (no kernel call is made). Offload availability depends on the NIC driver and kernel version; unsupported features are logged as warnings and do not block the config commit.
    - **gro** `boolean`
      Generic Receive Offload. Aggregates small incoming packets into larger ones before passing them up the network stack, reducing per-packet CPU overhead. Software-based (works on all NIC types including veth and dummy). Applied via kernel ioctl (ETHTOOL_SGRO). Equivalent to: ethtool -K <dev> gro on|off.
    - **gso** `boolean`
      Generic Segmentation Offload. Delays segmentation of large outgoing packets until just before the NIC driver transmit path, reducing CPU work in the upper stack. Software-based counterpart of TSO. Applied via kernel ioctl (ETHTOOL_SGSO). Equivalent to: ethtool -K <dev> gso on|off.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload. Enables the NIC to execute TC flower / u32 filter rules in hardware, bypassing kernel processing for matched flows. Requires NIC firmware support (common on mlx5, bnxt, nfp). Applied via kernel ioctl (ETHTOOL_SFEATURES). Equivalent to: ethtool -K <dev> hw-tc-offload on|off.
    - **lro** `boolean`
      Large Receive Offload. Hardware-based counterpart of GRO: the NIC coalesces incoming TCP segments before DMA. Can break forwarding and bridging because the coalesced frame no longer matches the original wire format. Disable on routers or bridges that forward traffic between interfaces. Applied via kernel ioctl (ETHTOOL_SLRO). Equivalent to: ethtool -K <dev> lro on|off.
    - **rfs** `boolean`
      Receive Flow Steering. Extension of RPS that steers incoming packets to the CPU where the application consuming that flow is running, improving cache locality and reducing cross-CPU cache bounces. When enabled, ze sets /proc/sys/net/core/ rps_sock_flow_entries to 32768 and distributes per-queue rps_flow_cnt evenly across rx queues. RPS should also be enabled for RFS to be effective. Not an ethtool feature; uses sysfs directly.
    - **rps** `boolean`
      Receive Packet Steering. Software-based distribution of incoming packets across multiple CPUs by hashing the packet header and steering to a target CPU receive queue. Useful when the NIC has fewer hardware RSS queues than available CPUs. When enabled, ze writes a bitmask covering all online CPUs to each rx queue sysfs entry (/sys/class/net/<dev>/queues/rx-*/ rps_cpus). When disabled, ze writes 0 to each entry. Not an ethtool feature; uses sysfs directly.
    - **sg** `boolean`
      Scatter-Gather I/O. Allows the NIC to assemble a single frame from multiple non-contiguous memory buffers, avoiding data copies. Required by TSO and GSO on most drivers. Applied via kernel ioctl (ETHTOOL_SSG). Equivalent to: ethtool -K <dev> sg on|off.
    - **tso** `boolean`
      TCP Segmentation Offload. Offloads TCP segmentation to the NIC hardware, allowing the kernel to hand off large (up to 64 KB) TCP segments that the NIC splits into MTU-sized frames on the wire. Requires NIC hardware support. Disable when passing traffic to VPP or virtual switches that cannot handle oversized frames. Applied via kernel ioctl (ETHTOOL_STSO). Equivalent to: ethtool -K <dev> tso on|off.
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
  - **unit <name>** `list`
    Logical interface unit
    - **class-of-service** `string`
      Per-unit override: profile name or 'none' to opt out of inheritance from the parent interface.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of outgoing packets to the 802.1p PCP value stamped in the 802.1Q header. Requires vlan-id. Priorities with no entry are sent with PCP 0.
      - **pcp** `uint8`
        PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of received tagged frames to an internal priority. Requires vlan-id. Frames whose PCP has no entry keep priority 0.
      - **priority** `uint8`
        Internal priority assigned to matching frames
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3): a non-owner Active accepts packets addressed to the virtual addresses. v3 semantics only; combining true with version 2 is rejected by the plugin verifier. Drives FSM/owner semantics only (not dataplane-enforced this pass).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds. The native range spans both versions; the plugin verifier narrows it per version. v3: multiples of 10 ms within 10..40950 (wire = centiseconds, RFC 9568 erratum 8301). v2: whole seconds within 1000..255000 (wire = seconds).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2): a higher-priority Backup takes over from a live lower-priority Active router.
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568). Armed on the first losing advert while a higher-priority local router waits; never delays dead-master failover.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically; it is never operator-set.
          - **version** `enumeration`
            Protocol version. Present only for IPv4 groups; IPv6 is always VRRPv3 (RFC 9568).
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order (RFC 9568 Section 5.2.9). An address equal to a real address on this unit makes this router the owner (priority forced to 255, accept-mode forced true).
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds, multiples of 10 ms (wire = centiseconds, RFC 9568 erratum 8301).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568).
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order. The FIRST address MUST be an IPv6 link-local (fe80::/10) address: it is the advertisement source identity (RFC 9568 Section 5.2.9, erratum 8300), enforced by the plugin verifier.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **loopback** `container`
  The system loopback interface (lo). Always present; ze manages its addresses and units. Used for hosting router-id addresses accessible from any interface.
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
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **monitor** `container`
  Interface monitoring settings
  - **loopback** `boolean`
    Monitor loopback interface
- **pppoe-client <name>** `list`
  PPPoE client interface (RFC 2516). Dials an access concentrator over a physical Ethernet interface, negotiates LCP/auth/IPCP, and presents the resulting PPP session as a routable interface with server- assigned addresses. The kernel pppN interface is created dynamically; the name leaf is a config key only (the OS interface name is pppN).
  - **ac-name** `string`
    Desired access concentrator name. When set, only PADO frames from this AC are accepted. Empty or absent means accept any AC.
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
    Do not install a default route via the PPP interface. When absent (default), a default route is installed after IPCP completes.
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
  - **service-name** `string`
    Desired PPPoE service name. Empty or absent means accept any service (RFC 2516 Section 5.1).
  - **source-interface** `string`
    Physical Ethernet interface for PPPoE discovery (e.g. eth2). Must exist and be admin-up.
- **tunnel <name>** `list`
  Tunnel interface (GRE/GRETAP/IPIP/SIT/IP6TNL families). Encapsulation kind is selected by the choice below; each case carries only the leaves valid for that kind. The local/remote endpoint pattern matches bgp peer connection { local { ip ... } remote { ip ... } }.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **encapsulation** `container`
    Tunnel encapsulation kind and per-kind parameters
    - **kind** `choice`
      Encapsulation discriminator. Invalid combinations (e.g. key on ipip) are unrepresentable in the schema.
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
            Outer-header TTL. 0 = inherit from inner
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
            Outer-header TTL. 0 = inherit
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
            Outer-header TTL. 0 = inherit
      - **ipip6** `case`
        IPv4 in IPv6 (RFC 2473 with Next Header = 4). L3. Implemented via the ip6tnl Linux kind with Proto=IPPROTO_IPIP.
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
            Outer-header TTL. 0 = inherit
      - **vxlan** `case`
        VXLAN overlay: L2 Ethernet frames over a UDP/IPv4 underlay, keyed by a 24-bit VNI. Landed in both the netlink and VPP backends.
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
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
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
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **veth <name>** `list`
  Virtual ethernet pair interface
  - **class-of-service** `string`
    Name of a class-of-service ieee-802.1p profile, or 'none' to explicitly disable inheritance. When set on the interface, all VLAN units inherit the profile unless overridden per-unit.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **mac** `container`
    Hardware MAC settings: override the operational address (address) and/or select the kernel device by its MAC (match). The two are independent: a NIC can be matched by its permanent (factory) MAC AND have its operational MAC overridden at the same time.
    - **address** `string`
      Override the kernel-assigned MAC address with this explicit colon-separated hex value (e.g., 02:42:ac:11:00:02). Applied at interface creation time. When omitted, the kernel assigns a MAC automatically.
    - **match** `string`
      Bind this logical interface to the kernel device that carries this hardware MAC, instead of binding by name. The resolver matches the device's PERMANENT (factory) address (IFLA_PERM_ADDRESS) when it reports one, so the binding survives an operational MAC override; for virtual devices that report no permanent address it matches the current address. Takes precedence over os-name. Applies to ethernet (the matched physical kind) only; the Ze-created kinds (dummy/veth/bridge/...) are identified by the name Ze assigns. When the matching device is absent the binding defers until it appears.
  - **mtu** `uint16`
    Maximum transmission unit
  - **offload** `container`
    Network offload and packet steering features. Applied directly via kernel ioctl (the ethtool SIOCETHTOOL interface) or sysfs writes after the interface is created. The ethtool(8) CLI program is NOT required; ze talks to the kernel directly. Boolean leaves: true = explicitly enable the feature, false = explicitly disable, absent = preserve whatever the OS default is (no kernel call is made). Offload availability depends on the NIC driver and kernel version; unsupported features are logged as warnings and do not block the config commit.
    - **gro** `boolean`
      Generic Receive Offload. Aggregates small incoming packets into larger ones before passing them up the network stack, reducing per-packet CPU overhead. Software-based (works on all NIC types including veth and dummy). Applied via kernel ioctl (ETHTOOL_SGRO). Equivalent to: ethtool -K <dev> gro on|off.
    - **gso** `boolean`
      Generic Segmentation Offload. Delays segmentation of large outgoing packets until just before the NIC driver transmit path, reducing CPU work in the upper stack. Software-based counterpart of TSO. Applied via kernel ioctl (ETHTOOL_SGSO). Equivalent to: ethtool -K <dev> gso on|off.
    - **hw-tc-offload** `boolean`
      Hardware Traffic Control Offload. Enables the NIC to execute TC flower / u32 filter rules in hardware, bypassing kernel processing for matched flows. Requires NIC firmware support (common on mlx5, bnxt, nfp). Applied via kernel ioctl (ETHTOOL_SFEATURES). Equivalent to: ethtool -K <dev> hw-tc-offload on|off.
    - **lro** `boolean`
      Large Receive Offload. Hardware-based counterpart of GRO: the NIC coalesces incoming TCP segments before DMA. Can break forwarding and bridging because the coalesced frame no longer matches the original wire format. Disable on routers or bridges that forward traffic between interfaces. Applied via kernel ioctl (ETHTOOL_SLRO). Equivalent to: ethtool -K <dev> lro on|off.
    - **rfs** `boolean`
      Receive Flow Steering. Extension of RPS that steers incoming packets to the CPU where the application consuming that flow is running, improving cache locality and reducing cross-CPU cache bounces. When enabled, ze sets /proc/sys/net/core/ rps_sock_flow_entries to 32768 and distributes per-queue rps_flow_cnt evenly across rx queues. RPS should also be enabled for RFS to be effective. Not an ethtool feature; uses sysfs directly.
    - **rps** `boolean`
      Receive Packet Steering. Software-based distribution of incoming packets across multiple CPUs by hashing the packet header and steering to a target CPU receive queue. Useful when the NIC has fewer hardware RSS queues than available CPUs. When enabled, ze writes a bitmask covering all online CPUs to each rx queue sysfs entry (/sys/class/net/<dev>/queues/rx-*/ rps_cpus). When disabled, ze writes 0 to each entry. Not an ethtool feature; uses sysfs directly.
    - **sg** `boolean`
      Scatter-Gather I/O. Allows the NIC to assemble a single frame from multiple non-contiguous memory buffers, avoiding data copies. Required by TSO and GSO on most drivers. Applied via kernel ioctl (ETHTOOL_SSG). Equivalent to: ethtool -K <dev> sg on|off.
    - **tso** `boolean`
      TCP Segmentation Offload. Offloads TCP segmentation to the NIC hardware, allowing the kernel to hand off large (up to 64 KB) TCP segments that the NIC splits into MTU-sized frames on the wire. Requires NIC hardware support. Disable when passing traffic to VPP or virtual switches that cannot handle oversized frames. Applied via kernel ioctl (ETHTOOL_STSO). Equivalent to: ethtool -K <dev> tso on|off.
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
  - **peer** `string`
    Veth peer name
  - **unit <name>** `list`
    Logical interface unit
    - **class-of-service** `string`
      Per-unit override: profile name or 'none' to opt out of inheritance from the parent interface.
    - **description** `string`
      Unit description
    - **disable** `empty`
      Administratively disable this unit
    - **egress-qos-map <priority>** `list`
      Map the internal priority of outgoing packets to the 802.1p PCP value stamped in the 802.1Q header. Requires vlan-id. Priorities with no entry are sent with PCP 0.
      - **pcp** `uint8`
        PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)
    - **ingress-qos-map <pcp>** `list`
      Map the 802.1p PCP value of received tagged frames to an internal priority. Requires vlan-id. Frames whose PCP has no entry keep priority 0.
      - **priority** `uint8`
        Internal priority assigned to matching frames
    - **ipv4** `container`
      IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.
      - **address** `string[]`
        IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)
      - **arp-accept** `boolean`
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv4 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3): a non-owner Active accepts packets addressed to the virtual addresses. v3 semantics only; combining true with version 2 is rejected by the plugin verifier. Drives FSM/owner semantics only (not dataplane-enforced this pass).
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds. The native range spans both versions; the plugin verifier narrows it per version. v3: multiples of 10 ms within 10..40950 (wire = centiseconds, RFC 9568 erratum 8301). v2: whole seconds within 1000..255000 (wire = seconds).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2): a higher-priority Backup takes over from a live lower-priority Active router.
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568). Armed on the first losing advert while a higher-priority local router waits; never delays dead-master failover.
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically; it is never operator-set.
          - **version** `enumeration`
            Protocol version. Present only for IPv4 groups; IPv6 is always VRRPv3 (RFC 9568).
          - **virtual-address** `ipv4-address[]`
            Virtual IPv4 addresses, encoded on the wire in configuration order (RFC 9568 Section 5.2.9). An address equal to a real address on this unit makes this router the owner (priority forced to 255, accept-mode forced true).
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
      - **vrrp** `container`
        VRRP virtual routers hosted on this IPv6 unit.
        - **group <name>** `list`
          One virtual router, identified by an operator-assigned name. The name is a config label only and is never sent on the wire; the VRID leaf carries the protocol identity.
          - **accept-mode** `boolean`
            Accept_Mode (RFC 9568 Section 6.1/6.4.3), v3 semantics.
          - **advertise-interval-milliseconds** `uint32`
            Advertisement interval in milliseconds, multiples of 10 ms (wire = centiseconds, RFC 9568 erratum 8301).
          - **preempt** `boolean`
            Preempt_Mode (RFC 9568 Section 6.4.2).
          - **preempt-delay-seconds** `uint16`
            Preemption hold-time (Junos semantics; not in RFC 9568).
          - **priority** `uint8`
            Election priority (RFC 9568 Section 5.2.4). 255 is reserved for the address owner and assigned automatically.
          - **virtual-address** `ipv6-address[]`
            Virtual IPv6 addresses, encoded on the wire in configuration order. The FIRST address MUST be an IPv6 link-local (fe80::/10) address: it is the advertisement source identity (RFC 9568 Section 5.2.9, erratum 8300), enforced by the plugin verifier.
          - **vrid** `uint8`
            Virtual Router Identifier (RFC 9568 Section 5.2.3). Unique per interface unit and address family; the IPv4 and IPv6 VRID spaces are independent.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **wireguard <name>** `list`
  WireGuard interface. Declarative config: interface-level listen-port, fwmark, private-key, and a nested peer list with public-key, endpoint, allowed-ips, preshared-key, and persistent-keepalive. L3 kind with no MAC address (uses interface-common rather than interface-l2). Reconciliation is in-place per peer via wgctrl ConfigureDevice; adding, removing, or rekeying a peer does not disturb the netdev or any other peer.
  - **description** `string`
    Interface description
  - **disable** `empty`
    Administratively disable this interface
  - **fwmark** `uint32`
    Firewall mark applied to outgoing encapsulated packets for policy routing. 0 means unset.
  - **listen-port** `port`
    UDP port WireGuard binds on 0.0.0.0 and :: for peer handshake and data traffic. If unset the kernel chooses an ephemeral port on the first outbound handshake.
  - **mtu** `uint16`
    Maximum transmission unit
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
  - **peer <name>** `list`
    WireGuard peer. Each peer is identified by its public key and has its own set of allowed-ips that define which traffic is routed through the tunnel.
    - **allowed-ips** `string[]`
      CIDR prefixes routed into the tunnel for this peer. Inbound packets from this peer must have a source address inside one of these prefixes (cryptokey routing).
    - **disable** `empty`
      Administratively disable this peer. The peer is removed from the kernel peer set on reload but stays in the config.
    - **endpoint** `container`
      Remote UDP endpoint for this peer. Both leaves are required together; the kernel uses this address on the first outbound handshake and updates it on every inbound handshake from a new source address.
      - **ip** `ip-address`
        Remote IPv4 or IPv6 address (numeric, not a hostname -- DNS resolution is not performed)
      - **port** `port`
        Remote UDP port
    - **persistent-keepalive** `uint16`
      Seconds between unsolicited keepalive packets to maintain NAT state. 0 disables keepalives. Typical value is 25.
    - **preshared-key** `string`
      Optional base64-encoded 32-byte symmetric preshared key mixed into the handshake for post-quantum resistance. Stored $9$-encoded on disk, same pattern as private-key.
    - **public-key** `string`
      Base64-encoded 32-byte Curve25519 peer public key
  - **private-key** `string`
    Base64-encoded 32-byte Curve25519 private key. Stored $9$-encoded on disk via the standard ze sensitive-leaf pattern; decoded to plaintext base64 at parse time. ze config show / dump always emits the $9$ form, never the plaintext. Obfuscation only (not encryption) -- protect the config file at filesystem level (chmod 600) just like BGP MD5 passwords.
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
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.
- **xfrm <name>** `list`
  XFRM interface (Linux 4.19+). Route-based IPsec: traffic routed into this interface is encrypted/decrypted by the kernel XFRM subsystem. The if-id binds security associations to the interface. L3 kind with no MAC address.
  - **description** `string`
    Interface description
  - **dev** `string`
    Optional parent device name. When set, the XFRM interface is bound to this physical device. When omitted, the interface is unbound and uses the routing table to select the underlay.
  - **disable** `empty`
    Administratively disable this interface
  - **if-id** `uint32`
    XFRM interface identifier. Binds this interface to XFRM security associations that carry the same if_id. Must be non-zero (0 means unset in the kernel).
  - **mtu** `uint16`
    Maximum transmission unit
  - **os-name** `string`
    OS/kernel device this logical interface name binds to. When omitted, the logical name is used as the OS device name, so every interface whose name already matches its kernel device resolves unchanged. Set it to alias a human-readable interface name to a different kernel device; the iface resolver maps the logical name to this OS device (ze init records the discovered OS name here).
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
        Accept gratuitous ARP frames and add their entries to the ARP cache (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios where a peer announces a new MAC for an existing IP.
      - **arp-announce** `uint8`
        ARP announce level: 0=any, 1=prefer subnet, 2=best only
      - **arp-filter** `boolean`
        Only respond to ARP requests for addresses on the receiving interface (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses assigned to other interfaces. Recommended for multi-homed hosts.
      - **arp-ignore** `uint8`
        ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check
      - **dhcp** `container`
        DHCPv4 client configuration
        - **client-id** `string`
          DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages. When omitted, the MAC address is used. Override for environments where the DHCP server keys leases on client-id rather than MAC.
        - **enabled** `boolean`
          Enable DHCPv4 client
        - **hostname** `string`
          Hostname in DHCP requests
      - **forwarding** `boolean`
        Allow this interface to forward IPv4 packets between interfaces. Sets net.ipv4.conf.<iface>.forwarding. Required for routing. When disabled, packets not destined for a local address are dropped.
      - **proxy-arp** `boolean`
        Answer ARP requests on behalf of other hosts that are reachable via this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging subnets without L2 connectivity or for unnumbered interfaces.
      - **rpf-check** `enumeration`
        Reverse path filtering mode
    - **ipv6** `container`
      IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.
      - **accept-ra** `uint8`
        Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding
      - **address** `string[]`
        IPv6 addresses in CIDR notation (e.g., fd00::1/64)
      - **autoconf** `boolean`
        Enable IPv6 stateless autoconfiguration
      - **dhcpv6** `container`
        DHCPv6 stateful client. Requests addresses and/or delegated prefixes from a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.
        - **duid** `string`
          Override the DHCPv6 Unique Identifier (RFC 8415 Section 11). When omitted, a DUID-LL based on the interface MAC is generated. Set this when the DHCP server binds leases to a specific DUID.
        - **enabled** `boolean`
          Enable DHCPv6 client
        - **pd** `container`
          Prefix delegation
          - **length** `uint8`
            Requested prefix length
      - **forwarding** `boolean`
        Allow this interface to forward IPv6 packets between interfaces. Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA acceptance unless accept-ra is set to 2.
      - **rpf-check** `enumeration`
        Reverse path filtering mode (VPP data plane only on IPv6). strict: drop packets whose source would not be routed back via this interface. loose: drop packets whose source has no route at all. disable: no source address validation.
    - **mirror** `container`
      Traffic mirroring. netlink mirrors via tc mirred; the vpp backend programs SPAN (sw_interface_span_ enable_disable), mapping ingress/egress onto the RX/TX SPAN state as a device-level port mirror.
      - **egress** `string`
        Mirror egress traffic to this interface
      - **ingress** `string`
        Mirror ingress traffic to this interface
    - **mpls** `container`
      MPLS forwarding on this interface (RFC 3031 LSR).
      - **enable** `boolean`
        Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.
    - **route-priority** `uint32`
      Route metric for default routes installed via DHCP on this unit. Lower values are preferred. On link-down, the metric is increased by 1024 to deprioritize the interface. 0 = kernel default.
    - **sysctl-profile** `node-name[]`
      Named sysctl profiles to apply to this unit. Built-in: dsr, router, hardened, multihomed, proxy. User-defined profiles from sysctl { profile ... } config. Applied in order; last wins on key overlap.
    - **vlan-id** `uint16`
      VLAN identifier
    - **vrf** `string`
      Assign this unit to a VRF (Virtual Routing and Forwarding) instance. The VRF must be defined separately. Traffic on this unit uses the VRF's routing table instead of the main table.

## isis

*Provided by `isis` ([ze-isis-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/isis/yang/ze-isis-cmd.yang), [ze-isis-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/isis/yang/ze-isis-conf.yang))*

IS-IS routing instance configuration.

- **hostname** `string`
  Dynamic hostname to advertise (RFC 5301).
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
  Network Entity Title(s), e.g. 49.0001.0000.0000.0001.00. At least one is required; the System ID is the 6 octets before the NSEL.
- **overload** `boolean`
  Set the overload bit (RFC 3787).
- **system-id** `string`
  6-byte System ID (xxxx.xxxx.xxxx); derived from NET if unset.

## kernel

*Provided by `kernel` ([ze-kernel-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/kernel/yang/ze-kernel-conf.yang))*

Kernel route redistribution. Presence enables the plugin.


## l2tp

*Provided by `l2tp-auth-local` ([ze-l2tp-auth-local-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/l2tp/plugins/authlocal/yang/ze-l2tp-auth-local-conf.yang)); `l2tp-auth-radius-servers` ([ze-l2tp-auth-radius-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang)); `l2tp-pool` ([ze-l2tp-pool-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang)); `l2tp-shaper` ([ze-l2tp-shaper-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang))*

L2TPv2 tunnel subsystem settings (RFC 2661). Presence of this block with any content implies the subsystem is enabled. Use 'enabled false' to disable explicitly, or 'enabled true' as a filler when no other settings are needed.

- **allow-no-auth** `boolean`
  Allow PPP LCP authentication negotiation to proceed without an Auth-Protocol. The default is false, so a peer that rejects all configured authentication methods is disconnected instead of being accepted as no-auth.
- **auth** `container`
  - **local** `container`
    - **user <name>** `list`
      - **password** `string`
        Shared secret for PAP cleartext and CHAP-MD5/MS-CHAPv2 challenge-response.
  - **radius** `container`
    - **acct-interval** `uint16`
      Accounting interim-update interval in seconds.
    - **coa-port** `uint16`
      UDP port for the RADIUS CoA/Disconnect listener (RFC 5176), commonly 3799. Deliberately has no default: leaving it unset keeps the listener off, so an existing deployment does not start accepting CoA on upgrade. Requests are accepted only from the configured RADIUS server addresses.
    - **nas-identifier** `string`
      NAS-Identifier sent in RADIUS requests.
    - **retries** `uint8`
      Number of retransmit attempts per server.
    - **server <name>** `list`
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
  PPP Auth-Protocol method first advertised in LCP Configure-Request. The default requires CHAP-MD5. Set to none only together with allow-no-auth true.
- **authentication** `container`
  PPP authentication phase settings for L2TP sessions.
  - **reauth-interval** `uint32`
    Periodic re-authentication interval in seconds (CHAP/MS-CHAPv2 only). Zero disables re-auth. Values 1-4 are rejected to prevent re-auth storms. Default 0 (disabled).
  - **timeout** `uint16`
    PPP auth-phase timeout in seconds. Sessions that do not complete authentication within this window are torn down. Default 30 seconds.
- **cqm-enabled** `boolean`
  Enable the CQM (Customer Quality Monitor) observer. When true, pre-allocates per-session event rings and per-login sample rings at subsystem start, and overrides the PPP echo interval to 1s for RTT measurement.
- **enabled** `boolean`
  Enable L2TP subsystem. Defaults to true when the l2tp block is present; set to false to disable.
- **event-ring-size-per-session** `uint16`
  Number of events in each per-session event ring. When full, oldest events are overwritten.
- **hello-interval** `uint16`
  Seconds of peer silence before sending HELLO. RFC 2661 recommends 60s; values above 3600 (one hour) are rejected because they effectively disable keepalive.
- **hello-retries** `uint8`
  Dead-peer detection threshold: consecutive unanswered HELLO keepalive intervals tolerated before an established tunnel's peer is declared dead and the tunnel is torn down. Effective detection time is hello-retries x hello-interval, measured from the last proof of peer liveness (a delivered control message or an acknowledgement of one of our messages, including a ZLB ACK of a HELLO). 0 disables dead-peer detection, leaving reliable-transport retransmit exhaustion (~31s) as the only teardown signal. Default 2.
- **max-logins** `uint32`
  Maximum concurrent PPP logins tracked by the CQM observer. Controls the size of the pre-allocated sample ring pool. When exceeded, the least recently used login's data is evicted.
- **max-sessions** `uint16`
  Maximum concurrent sessions per tunnel. The default is a finite deployment safety cap. A value of 0 explicitly requests unbounded admission on each tunnel. New ICRQs and OCRQs beyond a positive limit are rejected with CDN Result Code 4 (no resources).
- **max-tunnels** `uint16`
  Maximum concurrent L2TP tunnels. The default is a finite deployment safety cap. A value of 0 explicitly requests unbounded admission by this knob; the 65535 ceiling imposed by RFC 2661's 16-bit Tunnel ID field still applies. New SCCRQs beyond a positive limit are rejected with StopCCN Result Code 2.
- **ncp** `container`
  NCP (Network Control Protocol) settings for L2TP sessions.
  - **enable-ipcp** `boolean`
    Enable the IPCP NCP (RFC 1332) for new L2TP sessions. When false, no IPv4 address is negotiated.
  - **enable-ipv6cp** `boolean`
    Enable the IPv6CP NCP (RFC 5072) for new L2TP sessions. When false, no IPv6 interface identifier is negotiated.
  - **timeout** `uint16`
    NCP negotiation timeout in seconds. Sessions that do not complete NCP within this window are torn down. Default 30 seconds.
- **pool** `container`
  - **ipv4** `container`
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
    IPv6 prefix delegation pool. Allocates /N prefixes from a configured block for DHCPv6-PD. RFC 3633.
    - **block** `string`
      Prefix block to allocate from (e.g. 2001:db8::/32).
    - **delegation-length** `uint8`
      Prefix length delegated to each subscriber (e.g. 56 for /56 prefixes).
  - **named-ipv6-pool <name>** `list`
    Named IPv6 prefix pools selected by RADIUS Framed-IPv6-Pool attribute (RFC 6911 attr 100).
    - **block** `string`
      Prefix block to allocate from (e.g. 2001:db8::/32).
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
  PPPoE-to-L2TP relay bindings (LAC role). A subscriber whose PADI/PADR carries a matching PPPoE Service-Name is relayed into an L2TP incoming call toward the named remote instead of terminating PPP locally on this box. Removing a binding restores local termination for that service.
  - **remote** `string`
    Name of the L2TP remote (dial target) that matching subscribers are relayed to. MUST reference a remote declared under l2tp/remote; an unknown name is rejected at config validation.
- **remote <name>** `list`
  L2TP dial targets: remote LNS/LAC endpoints ze initiates tunnels toward (sends SCCRQ to). Referenced by name from 'request l2tp outgoing-call remote <name>' and from PPPoE relay bindings. Declaring a remote grants no dial by itself; an operator action (RPC) or a relay binding drives the dial.
  - **address** `ip-address`
    Remote control-plane IP address to dial. The SCCRQ destination (RFC 2661 Section 6.1).
  - **outgoing-calls** `boolean`
    Permit LNS-side outgoing calls ('request l2tp outgoing-call') toward this remote. Default false: the remote must be explicitly enabled for dial-out before an operator can place a call, so a mistyped remote name is rejected rather than dialed.
  - **port** `uint16`
    Remote UDP port for the control channel. RFC 2661 assigns 1701; overridable for non-standard peers.
  - **shared-secret** `string`
    Per-remote CHAP-MD5 tunnel-authentication secret (RFC 2661 Section 4.2). Overrides the subsystem-level shared-secret for tunnels dialed to this remote. When empty, this dial carries no Challenge AVP. Masked in CLI output.
- **sample-retention-seconds** `uint32`
  Duration of CQM sample retention per login, in seconds. Divided by the 100-second bucket interval to determine ring capacity. Default 86400 (24 hours) = 864 buckets.
- **shaper** `container`
  - **default-rate** `rate`
    Default download rate applied to new sessions. Format: number followed by suffix (e.g. 10mbit, 100kbit).
  - **qdisc-type** `enumeration`
    Queueing discipline type for subscriber interfaces.
  - **upload-rate** `rate`
    Default upload rate. When omitted, defaults to default-rate.
- **shared-secret** `string`
  Shared secret used to compute CHAP-MD5 Challenge Responses (RFC 2661 Section 4.2). When unset and a peer sends a Challenge AVP, the subsystem rejects the tunnel with StopCCN Result Code 4 (Not Authorized). Value is masked in CLI output and cleared from the process environment after first read.

## ldp

*Provided by `ldp-port` ([ze-ldp-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ldp/yang/ze-ldp-cmd.yang), [ze-ldp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ldp/yang/ze-ldp-conf.yang))*

Label Distribution Protocol configuration

- **hello-hold-time** `uint16`
  Hello adjacency hold time
- **hello-interval** `uint16`
  Hello message interval
- **interfaces** `string[]`
  Interfaces on which LDP discovery is enabled
- **keepalive-time** `uint16`
  Session keepalive interval
- **lsr-id** `string`
  LSR identifier (IPv4 address format)
- **transport-address** `string`
  Transport address for TCP sessions

## mrt

*Provided by `mrt` ([ze-mrt-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/mrt/yang/ze-mrt-conf.yang))*

MRT dump configuration

- **add-path** `boolean`
  Force add-path subtypes even when not negotiated with the peer.
- **all** `container`
  All BGP messages plus state changes (BGP4MP records). Includes OPEN, UPDATE, NOTIFICATION, KEEPALIVE, ROUTE_REFRESH, and FSM state transitions.
  - **file** `string`
    Output file path with strftime patterns for rotation. Supported codes: %Y (year), %m (month), %d (day), %H (hour), %M (minute), %S (second), %s (unix timestamp), %N (table name).
  - **interval** `uint32`
    File rotation interval in seconds. Zero disables rotation.
- **direction** `enumeration`
  Which direction of BGP messages to record.
- **extended-timestamp** `boolean`
  Use BGP4MP_ET (type 17) with microsecond resolution instead of BGP4MP (type 16).
- **peer-filter** `string[]`
  If set, only record messages from/to these peer addresses. Empty list means record all peers.
- **routes** `container`
  Periodic RIB snapshots (TABLE_DUMP_V2 records). Each dump writes a PEER_INDEX_TABLE followed by RIB entries for all address families.
  - **file** `string`
    Output file path with strftime patterns.
  - **interval** `uint32`
    RIB dump interval in seconds. Minimum 60.
- **updates** `container`
  BGP UPDATE message stream (BGP4MP records)
  - **file** `string`
    Output file path with strftime patterns for rotation. Supported codes: %Y (year), %m (month), %d (day), %H (hour), %M (minute), %S (second), %s (unix timestamp), %N (table name).
  - **interval** `uint32`
    File rotation interval in seconds. Zero disables rotation.

## ospf

*Provided by `ospf` ([ze-ospf-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ospf/yang/ze-ospf-cmd.yang), [ze-ospf-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/ospf/yang/ze-ospf-conf.yang))*

OSPFv2 routing instance configuration.

- **address-family** `container`
  Additional OSPF address families. The IPv6 (OSPFv3, RFC 5340) family runs as a second engine instance over the ospfv3 transport (ff02::5/6), sharing the FSM/flooding/SPF machinery with the IPv4 family.
  - **ipv4-multicast** `container`
    IPv4-multicast over OSPFv3 (RFC 5838 §2.1: Instance ID 96-127). Reachability is computed unicast-shaped; MOSPF tree computation is not implemented.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link through this (transit) area to a backbone area-border router (RFC 5340 section 4.2). It belongs to the backbone; its cost is the transit-area SPF path cost, never configured. The transit area must not be the backbone, a stub, or an NSSA.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID; the RFC 5838 §2.1 IPv4-multicast range is 96-127.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          RFC 5880 / RFC 5881 single-hop BFD for this OSPFv3 interface. When enabled, a Full adjacency opens an IPv6 single-hop BFD session (link-local pair); a BFD-detected failure declares the neighbor down far faster than the router-dead interval.
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
          RFC 4552 manual IPsec (AH/ESP) for this OSPFv3 interface. A transport-mode kernel Security Association plus a proto-89 policy is installed on interface up; the kernel applies AH/ESP below the socket and silently discards unprotected or failed-integrity OSPF packets (RFC 4552 §3/§4). This is a DISTINCT auth path from the RFC 7166 authentication trailer and is mutually exclusive with a per-interface authentication key-chain. Manual keying only (RFC 4552 §7: IKE cannot key the multicast group); the key is shared by the inbound and outbound SA.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm (RFC 4552 §4: only ESP, never AH). Omit or null for authentication-only ESP.
          - **encryption-key** `string`
            Hex ESP encryption key; length must match encryption-algorithm (aes128=32, aes256=64 hex characters).
          - **key** `string`
            Hex integrity key; length must match the algorithm (sha1=40, sha256=64, sha384=96, sha512=128 hex characters).
          - **protocol** `enumeration`
            IPsec protocol: esp (RFC 4303; authentication MUST be supported) or ah (RFC 4302; MAY).
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization (RFC 5443, RFC 6138) for the IPv6 family; reuses the AF-neutral state machine on the shared interface model.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2).
        - **nbma-neighbor <router-id>** `list`
          Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant. The IPv6 neighbor is keyed by Router ID; the link-local (the unicast Hello destination) may be configured or learned from the first Hello.
          - **link-local** `string`
            Neighbor IPv6 link-local address (the unicast Hello destination); learned from the neighbor's first Hello when omitted.
          - **priority** `uint8`
            Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected).
        - **network-type** `enumeration`
          Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 5340); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor /128 host routes and no DR.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5).
        - **priority** `uint8`
          DR/BDR election priority; 0 means ineligible.
    - **segment-routing** `container`
      RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.
      - **enable** `boolean`
        Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).
        - **explicit-null** `boolean`
          Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php.
        - **index** `uint32`
          The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size.
        - **no-php** `boolean`
          Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping).
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB (inclusive). Must be >= lower-bound.
      - **srlb** `container`
        Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB (inclusive). Must be >= lower-bound.
      - **srms-preference** `uint8`
        SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset.
  - **ipv4-unicast** `container`
    IPv4-unicast over OSPFv3 (RFC 5838 §2.1/§2.7: Instance ID 64-95). Carries IPv4 prefixes in the address-free OSPFv3 LSA model; routes install into the IPv4 RIB.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link through this (transit) area to a backbone area-border router (RFC 5340 section 4.2). It belongs to the backbone; its cost is the transit-area SPF path cost, never configured. The transit area must not be the backbone, a stub, or an NSSA.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID; the RFC 5838 §2.1 IPv4-unicast range is 64-95.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          RFC 5880 / RFC 5881 single-hop BFD for this OSPFv3 interface. When enabled, a Full adjacency opens an IPv6 single-hop BFD session (link-local pair); a BFD-detected failure declares the neighbor down far faster than the router-dead interval.
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
          RFC 4552 manual IPsec (AH/ESP) for this OSPFv3 interface. A transport-mode kernel Security Association plus a proto-89 policy is installed on interface up; the kernel applies AH/ESP below the socket and silently discards unprotected or failed-integrity OSPF packets (RFC 4552 §3/§4). This is a DISTINCT auth path from the RFC 7166 authentication trailer and is mutually exclusive with a per-interface authentication key-chain. Manual keying only (RFC 4552 §7: IKE cannot key the multicast group); the key is shared by the inbound and outbound SA.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm (RFC 4552 §4: only ESP, never AH). Omit or null for authentication-only ESP.
          - **encryption-key** `string`
            Hex ESP encryption key; length must match encryption-algorithm (aes128=32, aes256=64 hex characters).
          - **key** `string`
            Hex integrity key; length must match the algorithm (sha1=40, sha256=64, sha384=96, sha512=128 hex characters).
          - **protocol** `enumeration`
            IPsec protocol: esp (RFC 4303; authentication MUST be supported) or ah (RFC 4302; MAY).
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization (RFC 5443, RFC 6138) for the IPv6 family; reuses the AF-neutral state machine on the shared interface model.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2).
        - **nbma-neighbor <router-id>** `list`
          Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant. The IPv6 neighbor is keyed by Router ID; the link-local (the unicast Hello destination) may be configured or learned from the first Hello.
          - **link-local** `string`
            Neighbor IPv6 link-local address (the unicast Hello destination); learned from the neighbor's first Hello when omitted.
          - **priority** `uint8`
            Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected).
        - **network-type** `enumeration`
          Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 5340); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor /128 host routes and no DR.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5).
        - **priority** `uint8`
          DR/BDR election priority; 0 means ineligible.
    - **segment-routing** `container`
      RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.
      - **enable** `boolean`
        Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).
        - **explicit-null** `boolean`
          Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php.
        - **index** `uint32`
          The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size.
        - **no-php** `boolean`
          Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping).
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB (inclusive). Must be >= lower-bound.
      - **srlb** `container`
        Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB (inclusive). Must be >= lower-bound.
      - **srms-preference** `uint8`
        SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset.
  - **ipv6** `container`
    Default IPv6-unicast OSPFv3 address family (RFC 5340); the bare `ipv6` spelling is the IPv6-unicast AF. Presence enables the v6 instance.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link through this (transit) area to a backbone area-border router (RFC 5340 section 4.2). It belongs to the backbone; its cost is the transit-area SPF path cost, never configured. The transit area must not be the backbone, a stub, or an NSSA.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID (RFC 5340 §2.5); the RFC 5838 §2.1 IPv6-unicast range is 0-31.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          RFC 5880 / RFC 5881 single-hop BFD for this OSPFv3 interface. When enabled, a Full adjacency opens an IPv6 single-hop BFD session (link-local pair); a BFD-detected failure declares the neighbor down far faster than the router-dead interval.
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
          RFC 4552 manual IPsec (AH/ESP) for this OSPFv3 interface. A transport-mode kernel Security Association plus a proto-89 policy is installed on interface up; the kernel applies AH/ESP below the socket and silently discards unprotected or failed-integrity OSPF packets (RFC 4552 §3/§4). This is a DISTINCT auth path from the RFC 7166 authentication trailer and is mutually exclusive with a per-interface authentication key-chain. Manual keying only (RFC 4552 §7: IKE cannot key the multicast group); the key is shared by the inbound and outbound SA.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm (RFC 4552 §4: only ESP, never AH). Omit or null for authentication-only ESP.
          - **encryption-key** `string`
            Hex ESP encryption key; length must match encryption-algorithm (aes128=32, aes256=64 hex characters).
          - **key** `string`
            Hex integrity key; length must match the algorithm (sha1=40, sha256=64, sha384=96, sha512=128 hex characters).
          - **protocol** `enumeration`
            IPsec protocol: esp (RFC 4303; authentication MUST be supported) or ah (RFC 4302; MAY).
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization (RFC 5443, RFC 6138) for the IPv6 family; reuses the AF-neutral state machine on the shared interface model.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2).
        - **nbma-neighbor <router-id>** `list`
          Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant. The IPv6 neighbor is keyed by Router ID; the link-local (the unicast Hello destination) may be configured or learned from the first Hello.
          - **link-local** `string`
            Neighbor IPv6 link-local address (the unicast Hello destination); learned from the neighbor's first Hello when omitted.
          - **priority** `uint8`
            Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected).
        - **network-type** `enumeration`
          Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 5340); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor /128 host routes and no DR.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5).
        - **priority** `uint8`
          DR/BDR election priority; 0 means ineligible.
    - **segment-routing** `container`
      RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.
      - **enable** `boolean`
        Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).
        - **explicit-null** `boolean`
          Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php.
        - **index** `uint32`
          The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size.
        - **no-php** `boolean`
          Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping).
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB (inclusive). Must be >= lower-bound.
      - **srlb** `container`
        Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB (inclusive). Must be >= lower-bound.
      - **srms-preference** `uint8`
        SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset.
  - **ipv6-multicast** `container`
    IPv6-multicast OSPFv3 address family (RFC 5838 §2.1: Instance ID 32-63). Reachability is computed unicast-shaped; MOSPF tree computation is not implemented.
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link through this (transit) area to a backbone area-border router (RFC 5340 section 4.2). It belongs to the backbone; its cost is the transit-area SPF path cost, never configured. The transit area must not be the backbone, a stub, or an NSSA.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID; the RFC 5838 §2.1 IPv6-multicast range is 32-63.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          RFC 5880 / RFC 5881 single-hop BFD for this OSPFv3 interface. When enabled, a Full adjacency opens an IPv6 single-hop BFD session (link-local pair); a BFD-detected failure declares the neighbor down far faster than the router-dead interval.
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
          RFC 4552 manual IPsec (AH/ESP) for this OSPFv3 interface. A transport-mode kernel Security Association plus a proto-89 policy is installed on interface up; the kernel applies AH/ESP below the socket and silently discards unprotected or failed-integrity OSPF packets (RFC 4552 §3/§4). This is a DISTINCT auth path from the RFC 7166 authentication trailer and is mutually exclusive with a per-interface authentication key-chain. Manual keying only (RFC 4552 §7: IKE cannot key the multicast group); the key is shared by the inbound and outbound SA.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm (RFC 4552 §4: only ESP, never AH). Omit or null for authentication-only ESP.
          - **encryption-key** `string`
            Hex ESP encryption key; length must match encryption-algorithm (aes128=32, aes256=64 hex characters).
          - **key** `string`
            Hex integrity key; length must match the algorithm (sha1=40, sha256=64, sha384=96, sha512=128 hex characters).
          - **protocol** `enumeration`
            IPsec protocol: esp (RFC 4303; authentication MUST be supported) or ah (RFC 4302; MAY).
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization (RFC 5443, RFC 6138) for the IPv6 family; reuses the AF-neutral state machine on the shared interface model.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2).
        - **nbma-neighbor <router-id>** `list`
          Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant. The IPv6 neighbor is keyed by Router ID; the link-local (the unicast Hello destination) may be configured or learned from the first Hello.
          - **link-local** `string`
            Neighbor IPv6 link-local address (the unicast Hello destination); learned from the neighbor's first Hello when omitted.
          - **priority** `uint8`
            Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected).
        - **network-type** `enumeration`
          Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 5340); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor /128 host routes and no DR.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5).
        - **priority** `uint8`
          DR/BDR election priority; 0 means ineligible.
    - **segment-routing** `container`
      RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.
      - **enable** `boolean`
        Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).
        - **explicit-null** `boolean`
          Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php.
        - **index** `uint32`
          The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size.
        - **no-php** `boolean`
          Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping).
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB (inclusive). Must be >= lower-bound.
      - **srlb** `container`
        Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB (inclusive). Must be >= lower-bound.
      - **srms-preference** `uint8`
        SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset.
  - **ipv6-unicast** `container`
    IPv6-unicast OSPFv3 address family (RFC 5838 §2.1: Instance ID 0-31).
    - **areas** `container`
      OSPFv3 areas for this address family.
      - **area <area-id>** `list`
        Per-area OSPFv3 configuration.
        - **area-type** `enumeration`
          Area type.
        - **virtual-link <remote-router-id>** `list`
          Virtual link through this (transit) area to a backbone area-border router (RFC 5340 section 4.2). It belongs to the backbone; its cost is the transit-area SPF path cost, never configured. The transit area must not be the backbone, a stub, or an NSSA.
          - **dead-interval** `uint16`
            Router-dead interval on the virtual link.
          - **hello-interval** `uint16`
            Hello interval on the virtual link.
          - **retransmit-interval** `uint16`
            LSA retransmit interval on the virtual link.
          - **transmit-delay** `uint16`
            Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0).
    - **instance-id** `uint8`
      OSPFv3 Instance ID; the RFC 5838 §2.1 IPv6-unicast range is 0-31.
    - **interfaces** `container`
      OSPFv3-enabled interfaces for this address family.
      - **interface <name>** `list`
        Per-interface OSPFv3 configuration.
        - **area** `string`
          Declared area this interface belongs to.
        - **bfd** `container`
          RFC 5880 / RFC 5881 single-hop BFD for this OSPFv3 interface. When enabled, a Full adjacency opens an IPv6 single-hop BFD session (link-local pair); a BFD-detected failure declares the neighbor down far faster than the router-dead interval.
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
          RFC 4552 manual IPsec (AH/ESP) for this OSPFv3 interface. A transport-mode kernel Security Association plus a proto-89 policy is installed on interface up; the kernel applies AH/ESP below the socket and silently discards unprotected or failed-integrity OSPF packets (RFC 4552 §3/§4). This is a DISTINCT auth path from the RFC 7166 authentication trailer and is mutually exclusive with a per-interface authentication key-chain. Manual keying only (RFC 4552 §7: IKE cannot key the multicast group); the key is shared by the inbound and outbound SA.
          - **algorithm** `enumeration`
            HMAC-SHA integrity algorithm.
          - **encryption-algorithm** `enumeration`
            ESP confidentiality algorithm (RFC 4552 §4: only ESP, never AH). Omit or null for authentication-only ESP.
          - **encryption-key** `string`
            Hex ESP encryption key; length must match encryption-algorithm (aes128=32, aes256=64 hex characters).
          - **key** `string`
            Hex integrity key; length must match the algorithm (sha1=40, sha256=64, sha384=96, sha512=128 hex characters).
          - **protocol** `enumeration`
            IPsec protocol: esp (RFC 4303; authentication MUST be supported) or ah (RFC 4302; MAY).
          - **spi** `uint32`
            Security Parameters Index (RFC 4303 §2.1 reserves 0..255).
        - **ldp-sync** `container`
          LDP-IGP synchronization (RFC 5443, RFC 6138) for the IPv6 family; reuses the AF-neutral state machine on the shared interface model.
          - **enable** `boolean`
            Enable LDP-IGP synchronization on this interface.
          - **holddown** `uint16`
            Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2).
        - **nbma-neighbor <router-id>** `list`
          Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant. The IPv6 neighbor is keyed by Router ID; the link-local (the unicast Hello destination) may be configured or learned from the first Hello.
          - **link-local** `string`
            Neighbor IPv6 link-local address (the unicast Hello destination); learned from the neighbor's first Hello when omitted.
          - **priority** `uint8`
            Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected).
        - **network-type** `enumeration`
          Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 5340); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor /128 host routes and no DR.
        - **passive** `boolean`
          Advertise the interface but form no adjacency.
        - **poll-interval** `uint16`
          NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5).
        - **priority** `uint8`
          DR/BDR election priority; 0 means ineligible.
    - **segment-routing** `container`
      RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.
      - **enable** `boolean`
        Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB.
      - **prefix-sid <prefix>** `list`
        Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).
        - **explicit-null** `boolean`
          Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php.
        - **index** `uint32`
          The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size.
        - **no-php** `boolean`
          Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping).
        - **node-sid** `boolean`
          Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
      - **srgb** `container`
        Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRGB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRGB (inclusive). Must be >= lower-bound.
      - **srlb** `container`
        Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.
        - **lower-bound** `uint32`
          First MPLS label of the SRLB (inclusive).
        - **upper-bound** `uint32`
          Last MPLS label of the SRLB (inclusive). Must be >= lower-bound.
      - **srms-preference** `uint8`
        SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset.
- **areas** `container`
  OSPF areas.
  - **area <area-id>** `list`
    Per-area configuration.
    - **area-type** `enumeration`
      Area type; semantics land in the stub/NSSA spec.
    - **authentication** `container`
      Area-level authentication defaults.
      - **key-chain** `string`
        Default key chain inherited by interfaces.
    - **default-cost** `uint32`
      Default summary metric for stub/NSSA areas.
    - **no-summary** `boolean`
      Suppress Type 3 summaries in totally-stubby or totally-NSSA areas.
    - **nssa** `container`
      NSSA-specific configuration (applies when area-type is nssa).
      - **default-originate** `boolean`
        Originate a Type 7 default route (0.0.0.0/0) into the NSSA.
      - **stability-interval** `uint16`
        Hysteresis before a newly elected translator stops translating after losing the role (RFC 3101 section 3.5).
      - **translate-role** `enumeration`
        Type 7 to Type 5 translator role: candidate (elect by highest router-id), always (force translate), never (do not translate).
    - **ranges** `container`
      Inter-area summary ranges.
      - **range <prefix>** `list`
        One area summary range.
        - **advertise** `enumeration`
          Advertise the aggregate or suppress specifics.
        - **cost** `uint32`
          Override cost for the aggregate Type 3 LSA.
    - **virtual-link <remote-router-id>** `list`
      Virtual link through this (transit) area to a backbone area-border router (RFC 2328 section 15). The virtual link belongs to the backbone; its output cost is computed from the transit-area SPF, never configured. The transit area must not be the backbone, a stub, or an NSSA.
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
  Originate RFC 7684 Extended Link Opaque LSAs (Opaque Type 8) that associate attributes with this router's links (one LSA per point-to-point/transit link, mirroring the Router-LSA link). Requires opaque true. The LSA is a container a link-attribute application (e.g. Segment Routing) fills with sub-TLVs; it is off by default until such a producer needs it. Received Extended Link LSAs are decoded and shown regardless of this leaf.
- **extended-prefix** `boolean`
  Originate RFC 7684 Extended Prefix Opaque LSAs (Opaque Type 7) that associate attributes with this router's advertised prefixes. Requires opaque true. The LSA is a container a prefix-attribute application (e.g. Segment Routing) fills with sub-TLVs; it is off by default until such a producer needs it. Received Extended Prefix LSAs are decoded and shown regardless of this leaf.
- **fast-reroute** `container`
  RFC 5286 Loop-Free Alternate (LFA) and TI-LFA IP fast reroute. When enabled, SPF pre-computes a loop-free backup next-hop alongside each primary and programs it into the FIB, so a single local link or node failure is repaired locally before the IGP reconverges. TI-LFA mode adds a Segment-Routing repair-list fallback where no directly-connected LFA exists (requires segment-routing). Applies to OSPFv2 and OSPFv3; OSPFv3 gets base-LFA next-hop selection (SR repair labels are IPv4 only).
  - **enable** `boolean`
    Enable LFA / TI-LFA fast-reroute backup computation and install.
  - **mode** `enumeration`
    Backup computation mode: base LFA, or LFA with a TI-LFA SR-repair fallback.
  - **node-protection** `boolean`
    Prefer node-protecting alternates over link-only alternates (RFC 5286 Section 3.6).
- **graceful-restart** `container`
  RFC 3623 (OSPFv2) / RFC 5187 (OSPFv3) Graceful Restart: keep forwarding across a control-plane restart. Family-neutral: this container drives both address families (the OSPFv3 family inherits it unless it configures its own).
  - **helper** `container`
    This router's helper role: hold an adjacency to a restarting neighbor and suppress LSDB churn for its grace period.
    - **strict-lsa-checking** `boolean`
      RFC 3623 Appendix B.2 RestartHelperStrictLSAChecking (sec 3.2): terminate helper mode when a changed LSA that would flood to the restarting router is installed.
    - **support** `boolean`
      RFC 3623 Appendix B.2 RestartHelperSupport: act as a helper for a restarting neighbor.
  - **restarter** `container`
    This router's restarting-router role: originate Grace-LSAs and preserve the FIB across a restart.
    - **restart-interval** `uint16`
      RFC 3623 Appendix B.1 RestartInterval: the grace period neighbors keep advertising this router as fully adjacent. Should not exceed LSRefreshTime (1800 s) or this router's own LSAs age out mid-restart.
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
        Authentication mode; inherit uses the area default.
    - **bfd** `container`
      RFC 5880 / RFC 5881 single-hop BFD for this OSPF interface. When enabled, a Full adjacency opens a single-hop BFD session; a BFD-detected failure declares the neighbor down far faster than the router-dead interval.
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
      OSPFv2 Multi-Instance Instance ID(s) this interface participates in (RFC 6549 section 3). Absent means the base instance 0 only, which is bit-for-bit compatible with base OSPFv2. Each listed value runs a separate OSPFv2 instance demultiplexed on this subnet; a received packet whose Instance ID matches none of them is discarded (section 2/3.1).
    - **ldp-sync** `container`
      LDP-IGP synchronization (RFC 5443, RFC 6138): hold the link at maximum cost (point-to-point) or withhold its transit link (broadcast, non-cut-edge) until LDP is synchronized, so transit traffic is not black-holed before the label bindings exist.
      - **enable** `boolean`
        Enable LDP-IGP synchronization on this interface.
      - **holddown** `uint16`
        Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2 estimation that all label bindings are exchanged; the RFC defines no universal default). 0 restores the cost immediately once the session is up (allowed but discouraged).
    - **mtu-ignore** `boolean`
      Skip DD MTU mismatch rejection.
    - **nbma-neighbor <address>** `list`
      Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant.
      - **priority** `uint8`
        Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected).
    - **network-type** `enumeration`
      Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 2328); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor host routes and no DR.
    - **passive** `boolean`
      Advertise the interface but form no adjacency.
    - **poll-interval** `uint16`
      NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5).
    - **priority** `uint8`
      DR/BDR election priority; 0 means ineligible.
    - **retransmit-interval** `uint16`
      LSA retransmit interval.
    - **traffic-engineering** `container`
      RFC 3630 / RFC 5392 Traffic Engineering link attributes advertised in a Type 10 (or, for inter-as, Type 10/11) opaque TE LSA. Requires the top-level opaque leaf. The TE metric is independent of cost.
      - **admin-group** `uint32`
        RFC 3630 sec 2.5.9 Administrative Group (Resource Class/Color) 32-bit mask; LSB is group 0.
      - **enable** `boolean`
        Advertise a TE Link LSA for this interface.
      - **inter-as** `container`
        RFC 5392 inter-AS TE link. No OSPF adjacency is formed on the link (sec 4); the ASBR proxies the link into its own AS. Requires remote-as and at least one remote-asbr address.
        - **remote-as** `asn`
          RFC 5392 sec 3.3.1 Remote AS Number of the neighboring AS the link connects to.
        - **remote-asbr-ipv4** `ipv4-address`
          RFC 5392 sec 3.3.2 IPv4 Remote ASBR ID (sub-TLV 22); recommended to be the remote TE Router ID.
        - **remote-asbr-ipv6** `ipv6-address`
          RFC 5392 sec 3.3.3 IPv6 Remote ASBR ID (sub-TLV 24, not 23); a stable global IPv6 address.
        - **scope** `enumeration`
          RFC 5392 sec 3.1.1 flooding scope policy: area (Type 10, limited to the ASBR's area) or as (Type 11, AS-wide).
      - **max-bandwidth** `uint64`
        RFC 3630 sec 2.5.6 Maximum Bandwidth (true link capacity) in bytes/second.
      - **max-reservable-bandwidth** `uint64`
        RFC 3630 sec 2.5.7 Maximum Reservable Bandwidth in bytes/second; defaults to max-bandwidth.
      - **te-metric** `uint32`
        RFC 3630 sec 2.5.5 Traffic Engineering metric; independent of the OSPF cost, defaulting to it when unset.
    - **transmit-delay** `uint16`
      Estimated LSA transmission delay (RFC 2328 InfTransDelay must be > 0).
- **key-chains <name>** `list`
  Named authentication key chains for hitless rotation.
  - **extended-sequence** `boolean`
    Use RFC 7474 AuType 3 (extended 64-bit cryptographic sequence numbers) instead of AuType 2; applies to the HMAC-SHA algorithms.
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
      When this key may be used to sign.
      - **end** `string`
        RFC3339 end timestamp.
      - **start** `string`
        RFC3339 start timestamp.
- **max-metric** `container`
  RFC 6987 stub router: originate the Router-LSA with MaxLinkMetric so transit traffic avoids this router.
  - **router-lsa** `container`
    - **always** `boolean`
      Always advertise as a stub router.
    - **on-shutdown** `uint32`
      Advertise as a stub router for N seconds during a graceful shutdown (0 disables).
    - **on-startup** `uint32`
      Advertise as a stub router for N seconds after startup (0 disables).
- **maximum-paths** `uint8`
  Maximum ECMP paths per prefix.
- **opaque** `boolean`
  Enable the RFC 5250 opaque-LSA capability. When true, this router sets the O-bit in its Database Description packets (advertising it will receive and forward opaque LSAs) and originates opaque LSAs for any registered consumer. Received opaque LSAs are stored and reflooded by their scope regardless of this leaf; disabling it only stops advertising opaque capability and originating opaque LSAs.
- **redistribute <source>** `list`
  Route sources redistributed as AS-external LSAs.
  - **metric** `uint32`
    Injected metric.
  - **metric-type** `enumeration`
    External metric type.
  - **tag** `uint32`
    External route tag.
- **reference-bandwidth** `uint32`
  Auto-cost reference bandwidth in Mbps.
- **router-address** `ipv4-address`
  RFC 3630 sec 2.4.1 Traffic Engineering Router Address: a stable, always-reachable IPv4 address (typically a loopback) advertised once per router in its Router-Address TE LSA. Defaults to the Router ID when unset. Requires opaque and at least one traffic-engineering interface to take effect.
- **router-id** `string`
  OSPF Router ID in dotted-quad form; derived when omitted.
- **router-information** `container`
  RFC 7770 Router Information (RI) LSA: advertise this router's optional capabilities (OSPFv2 opaque type 4, OSPFv3 function code 12). Applies to both address families; a top-level container drives the OSPFv3 family unless it configures its own.
  - **enabled** `boolean`
    Originate the Router Information LSA advertising this router's informational capabilities. OSPFv2 also requires 'opaque true' (the RI LSA is an opaque LSA).
  - **scope** `enumeration[]`
    Flooding scope(s) at which the RI LSA is advertised (RFC 7770 sec 2.7). When enabled with no scope listed, defaults to area + as.
- **segment-routing** `container`
  RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.
  - **enable** `boolean`
    Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB.
  - **prefix-sid <prefix>** `list`
    Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).
    - **explicit-null** `boolean`
      Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php.
    - **index** `uint32`
      The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size.
    - **no-php** `boolean`
      Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping).
    - **node-sid** `boolean`
      Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV).
  - **srgb** `container`
    Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.
    - **lower-bound** `uint32`
      First MPLS label of the SRGB (inclusive).
    - **upper-bound** `uint32`
      Last MPLS label of the SRGB (inclusive). Must be >= lower-bound.
  - **srlb** `container`
    Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.
    - **lower-bound** `uint32`
      First MPLS label of the SRLB (inclusive).
    - **upper-bound** `uint32`
      Last MPLS label of the SRLB (inclusive). Must be >= lower-bound.
  - **srms-preference** `uint8`
    SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset.
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

PKI certificate and key store. Presence of this block enables certificate-based authentication for IPsec VPN, TLS, and other subsystems.

- **ca <name>** `list`
  Trusted CA certificate. The certificate leaf holds a base64-encoded DER X.509 certificate (no PEM headers).
  - **certificate** `string`
    Base64-encoded DER X.509 CA certificate.
- **certificate <name>** `list`
  Device certificate with optional private key and intermediate certificates.
  - **certificate** `string`
    Base64-encoded DER X.509 device certificate.
  - **intermediate** `string`
    Base64-encoded DER intermediate CA certificate. Stored alongside device cert for chain building.
  - **private** `container`
    Private key associated with this certificate.
    - **key** `string`
      Private key in base64-encoded DER format. Stored as $9$ in config file, auto-decoded on load. Supports PKCS8, SEC1 (ECDSA), and PKCS1 (RSA) encodings.

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

*Provided by `policy-routes` ([ze-policyroute-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/policyroute/yang/ze-policyroute-cmd.yang), [ze-policyroute-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/policyroute/yang/ze-policyroute-conf.yang))*

Policy routing configuration.

- **route <name>** `list`
  A named policy route applied to ingress interfaces.
  - **interface** `string[]`
    Ingress interface(s) to match. A trailing '*' enables prefix (wildcard) matching (e.g. 'l2tp*').
  - **rule <name>** `list`
    Ordered list of match/action rules.
    - **from** `container`
      Packet match criteria.
      - **destination-address** `string`
        Destination IP prefix or @set reference.
      - **destination-port** `string`
        Destination port or port range.
      - **protocol** `string`
        L4 protocol name (tcp, udp, icmp).
      - **source-address** `string`
        Source IP prefix or @set reference.
      - **source-port** `string`
        Source port or port range (e.g. '80,443').
      - **tcp-flags** `string`
        Comma-separated TCP flags to match (fin, syn, rst, psh, ack, urg).
    - **order** `uint32`
      Evaluation order (lower values first). Rules with equal order are sorted by name. If omitted, defaults to 0.
    - **then** `container`
      Action for matching packets.
      - **accept** `empty`
        Skip this policy (packet routes normally).
      - **drop** `empty`
        Drop matching packets.
      - **next-hop** `string`
        Redirect matching packets to this next-hop. Ze auto-allocates a kernel routing table from range 2000-2999 and manages the default route, fwmark, and ip rule internally.
      - **table** `uint32`
        Route matching packets via this kernel routing table. Range 1000-2999 is reserved for ze internal use (VRF and policy-routing auto tables).
      - **tcp-mss** `uint16`
        Clamp TCP MSS to this value (bytes).

## pppoe

PPPoE access concentrator settings (RFC 2516). Presence of this block with any content implies the subsystem is enabled. Use 'enabled false' to disable explicitly, or 'enabled true' as a filler when no other settings are needed.

- **ac-name** `string`
  Access Concentrator Name advertised in PADO (RFC 2516 Section 5.2).
- **cookie-timeout** `uint16`
  AC-Cookie validity duration in seconds. Cookies older than this are rejected in PADR validation.
- **enabled** `boolean`
  Enable PPPoE subsystem. Defaults to true when the pppoe block is present; set to false to disable.
- **interface <name>** `list`
  Access interfaces on which PPPoE discovery listens. Each interface gets an independent session ID space.
  - **max-sessions** `uint16`
    Per-interface session limit. Defaults to the global max-sessions when unset.
  - **service-name** `string[]`
    Per-interface Service-Name filter. Overrides the global service-name list when set.
- **max-sessions** `uint16`
  Maximum concurrent PPPoE sessions per interface. Limited by the 16-bit session ID range (1-65535).
- **padi-rate-limit** `uint16`
  Maximum PADI packets accepted per second per source MAC. Excess PADIs are silently discarded.
- **service-name** `string[]`
  Accepted Service-Name values. Empty list means accept any Service-Name (RFC 2516 Section 5.1). Per-interface service-name overrides this global list.

## redistribute

*Provided by `redistribute-orchestrator`*

Route redistribution between protocols. Each destination protocol has its own container with import rules. Adding a new destination protocol means adding a new key here.

- **destination <protocol>** `list`
  Destination protocol for redistributed routes. Each entry names a registered consumer (bgp, ospf, ...).
  - **import <source>** `list`
    Import routes from a source into this destination. When family is omitted, all families are imported.
    - **family** `string[]`
      Address families to import from this source. Empty means all families.

## rib

*Provided by `rib` ([ze-rib-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/sysrib/yang/ze-rib-conf.yang))*

System RIB configuration.

- **admin-distance** `container`
  Administrative distance per protocol. Lower value wins. Used by the system RIB to select the best route across protocols.
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

## routing-table

*Provided by `routing-table` ([ze-routing-table-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/routingtable/yang/ze-routing-table-conf.yang))*

Named routing table definitions. Each entry maps a name to a kernel routing table ID.

- **table <name>** `list`
  A named routing table.
  - **id** `uint32`
    Kernel routing table ID. Reserved IDs excluded: 0 (use 'default'), 253-255 (kernel reserved).

## rsvp-te

*Provided by `rsvp-te-rawsock` ([ze-rsvp-te-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/rsvpte/yang/ze-rsvp-te-cmd.yang), [ze-rsvp-te-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang))*

RSVP-TE traffic engineering configuration

- **bypass <name>** `list`
  Facility-backup bypass LSP (RFC 4090 Section 3.2): an LSP from this PLR to a merge point, explicitly routed to avoid the protected resource. A protected transit LSP whose next hop (link protection) or next-next hop (node protection) is this bypass's merge point is redirected onto it on a local failure. Explicit because ze has no IGP/CSPF to auto-compute a backup path.
  - **explicit-route <index>** `list`
    Bypass path hops (must avoid the protected resource)
    - **address** `string`
      Hop address (IPv4 prefix)
    - **type** `enumeration`
      Hop type (strict or loose)
  - **merge-point** `string`
    Merge point address (IPv4): the protected LSPs' next hop (link protection) or next-next hop (node protection) where this bypass rejoins them
  - **node-protection** `boolean`
    This bypass merges at a next-next hop, providing node protection
- **interface <name>** `list`
  Per-interface RSVP-TE configuration
  - **address** `string`
    Local link prefix (IPv4 CIDR, e.g. 10.0.0.4/30). When more than one interface is configured, admission control maps an LSP to this interface when the neighbor address falls within the prefix.
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
      Maximum number of hops the backup path may take
    - **node-protection** `boolean`
      Request NNHOP (node) protection rather than NHOP (link) protection
  - **hold-priority** `uint8`
    Hold priority (0 = highest)
  - **setup-priority** `uint8`
    Setup priority (0 = highest)
  - **tunnel-id** `uint16`
    Tunnel identifier

## service

*Provided by `as112` ([ze-as112-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/as112/yang/ze-as112-cmd.yang), [ze-as112-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/as112/yang/ze-as112-conf.yang)); `dhcpserver` ([ze-dhcp-server-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang)); `geodns` ([ze-geodns-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/geodns/yang/ze-geodns-cmd.yang), [ze-geodns-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/geodns/yang/ze-geodns-conf.yang)); `imageserver` ([ze-image-server-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/imageserver/yang/ze-image-server-conf.yang)); `tftpserver` ([ze-tftp-server-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang))*

Service settings

- **as112** `container`
  AS112 anycast DNS node
  - **address-family** `enumeration`
    Restrict the service to one address family (RFC 7534 Section 3.4 / RFC 7535 Section 3.1 single-stack option). The anycast addresses themselves are fixed constants, never operator-typed.
  - **allow-from** `ip-prefix[]`
    Optional client-source access list. Empty/unset (default) answers every source, matching standard AS112 public-sink behavior. When non-empty, only queries whose source IP is contained in one of these prefixes are answered; all others are silently dropped (no response). Loopback/ on-box sources are always implicitly permitted regardless of this list, so the 'request as112 healthcheck' probe is never blocked. Setting this makes the node non-public -- correct for a local-use mirror, wrong for a globally-reachable AS112 contributor.
  - **asn** `asn`
    Origin AS the AS112 covering prefixes carry when redistributed into BGP (redistribute { destination bgp { import as112 } }). Defaults to the well-known AS112 number 112 (RFC 7534 Section 3.2): the redistribute source models an AS112 virtual router. Set an operator ASN or an RFC 6996 private-use ASN to originate under a coordinated or local-use AS instead. Ignored unless 'import as112' is configured under redistribute.
  - **community** `string[]`
    Optional BGP communities attached to the redistributed AS112 covering prefixes. Accepts AA:NN standard communities (RFC 1997) and well-known names such as no-export or nopeer (RFC 3765) -- the values RFC 7534 Section 3.4 recommends for restricting AS112 route propagation. Typed 'string' rather than the AA:NN-only ze-types community pattern so well-known names are accepted; every value is validated by the canonical community parser at config time. Ignored unless 'import as112' is configured under redistribute.
  - **doh** `container`
    DNS-over-HTTPS (DoH, RFC 8484) listener. It reuses the tls container's certificate material and binds the fixed anycast addresses (and loopback) on 'listen-port' at 'path'.
    - **enabled** `boolean`
      Enable the DNS-over-HTTPS listener (RFC 8484).
    - **listen-port** `port`
      TCP port for the DoH HTTPS listener (RFC 8484).
    - **path** `string`
      HTTP request path the DoH endpoint answers on (RFC 8484 URI Template); defaults to /dns-query.
  - **enabled** `boolean`
    Enable the AS112 anycast DNS node
  - **facility** `string`
    Facility/site name surfaced alongside location in the HOSTNAME.AS112.NET/ARPA TXT answers, e.g. 'Example Datacenter'.
  - **hostname** `string`
    Node identification string surfaced in the HOSTNAME.AS112.NET/ARPA TXT answers (RFC 7534 Section 3.5), so operators can tell which anycast instance answered a given query. Empty omits the TXT string.
  - **ipv4-anycast-listener <name>** `list`
    Presence-only anchor: as112 always occupies port 53 on its fixed IPv4 anycast addresses while enabled. The Go-level default (RegisterListenerDefault) fills in the representative address, since this list is never populated by any operator-facing command.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **ipv6-anycast-listener <name>** `list`
    Presence-only anchor: as112 always occupies port 53 on its fixed IPv6 anycast addresses while enabled.
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned
  - **location** `string`
    City/country surfaced alongside facility in the HOSTNAME.AS112.NET/ARPA TXT answers, e.g. 'London, UK'.
  - **tls** `container`
    DNS-over-TLS (DoT, RFC 7858) listener and the certificate material shared with DoH. When enabled the DoT listener binds the fixed anycast addresses (and loopback) on 'listen-port'. When cert-file/key-file are unset an ephemeral self-signed certificate is used, which strict clients cannot validate -- supply operator PEM for a publicly trusted node.
    - **cert-file** `string`
      Path to the PEM server certificate presented on DoT and DoH. Unset (together with key-file) selects an ephemeral self-signed certificate.
    - **enabled** `boolean`
      Enable the DNS-over-TLS listener (RFC 7858).
    - **key-file** `string`
      Path to the PEM private key matching cert-file. Must be set together with cert-file.
    - **listen-port** `port`
      TCP port for the DoT listener; the RFC 7858 well-known 'domain-s' port is 853.
  - **watchdog** `boolean`
    Health-gate the BGP announcement on DNS serving state (RFC 7534 Section 3.3: do not advertise the service prefix while the name server is not running). When true (default) the covering prefixes are announced only while the as112 node is serving, and withdrawn when serving is lost, the service is disabled, or the node shuts down. false announces as soon as enabled and imported, without the serving-state gate. Ignored unless 'import as112' is configured under redistribute.
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
    Default record TTL (seconds) when a host omits its own. 1..2147483647 (RFC 2181 section 8); a zero default is not allowed.
  - **doh** `container`
    DNS-over-HTTPS (DoH, RFC 8484) listener. It reuses the tls container's certificate material and binds the configured listener IPs on 'listen-port' at 'path'.
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
    UDP+TCP listen endpoints. Each named entry binds one IP (IPv4 or IPv6) and port; repeat with the same port for a dual-stack pair. Defaults to 127.0.0.1:5300 and ::1:5300 when no entry is configured. The ze:listener marking enables config-time port-conflict detection across services.
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
      SOA minimum / negative-cache TTL seconds
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
    Maps a client-IP prefix to a host-set. Longest prefix wins; 0.0.0.0/0 and ::/0 are the catch-all (external) default.
    - **host-set** `string`
      Name of the host-set to answer for clients matching this prefix
  - **tls** `container`
    DNS-over-TLS (DoT, RFC 7858) listener and the certificate material shared with DoH. When enabled the DoT listener binds the configured listener IPs on 'listen-port'. When cert-file/key-file are unset an ephemeral self-signed certificate is used, which strict clients cannot validate.
    - **cert-file** `string`
      Path to the PEM server certificate presented on DoT and DoH. Unset (together with key-file) selects an ephemeral self-signed certificate.
    - **enabled** `boolean`
      Enable the DNS-over-TLS listener (RFC 7858).
    - **key-file** `string`
      Path to the PEM private key matching cert-file. Must be set together with cert-file.
    - **listen-port** `port`
      TCP port for the DoT listener; the RFC 7858 well-known 'domain-s' port is 853.
  - **zone** `string[]`
    Zones served (FQDN). A query name must end in one of these.
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
  - **shell-auth-sha256** `string`
    Lowercase hex sha256 of the admin password. Emitted on the installer kernel cmdline (ze.shell-auth) to gate the rescue shell; empty means the installer fails closed (no shell on fatal).
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

*Provided by `static` ([ze-static-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/static/yang/ze-static-cmd.yang), [ze-static-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/static/yang/ze-static-conf.yang))*

Static route configuration.

- **table <name>** `list`
  A named routing table grouping. Table name is resolved to a kernel table ID via the routing-table registry. Use 'default' for the main routing table.
  - **route <prefix>** `list`
    A static route entry.
    - **action** `choice`
      What to do with matching packets.
      - **discard** `case`
        - **blackhole** `container`
      - **forward** `case`
        - **next** `container`
          Forward action. Contains gateway and interface next-hops that may be combined for ECMP.
          - **hop <address>** `list`
            Forward via gateway. Multiple entries produce ECMP with traffic distributed by weight.
            - **bfd-profile** `string`
              BFD profile name (from bfd/profile list). When the BFD session to this next-hop goes down, the next-hop is removed from the ECMP group and the route is reprogrammed with remaining active next-hops.
            - **interface** `string`
              Outgoing interface. Required when the next-hop is a link-local IPv6 address.
            - **weight** `uint16`
              ECMP weight. Higher values receive proportionally more traffic. Default 1 gives equal distribution.
          - **interface <name>** `list`
            Forward via interface only (no gateway address). Used for point-to-point links (PPPoE, GRE tunnels) where the next-hop is implicit. Multiple entries can coexist with gateway next-hops for mixed ECMP.
            - **weight** `uint16`
              ECMP weight. Higher values receive proportionally more traffic.
      - **unreachable** `case`
        - **reject** `container`
    - **description** `string`
      Operator note for this route.
    - **metric** `uint32`
      Route metric. Used as kernel route priority (lower is preferred) and carried in redistribute.
    - **tag** `uint32`
      Opaque tag for route policy matching. Carried in redistribute events.

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
        Interval between extended self-tests (e.g. 7d)
      - **time** `string`
        Preferred time of day for extended self-tests (HH:MM)
    - **short** `container`
      Short self-test schedule
      - **interval** `string`
        Interval between short self-tests (e.g. 24h)
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

*Provided by `sysctl` ([ze-sysctl-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/sysctl/yang/ze-sysctl-conf.yang))*

Kernel tunable management.

- **profile <name>** `list`
  User-defined sysctl profile: a named collection of kernel tunables applied together to interface units. Overrides built-in profiles with the same name.
  - **setting <name>** `list`
    Kernel tunables in this profile.
    - **value** `string`
      Value to set for this key.
- **setting <name>** `list`
  A kernel tunable to set persistently. Uses kernel-native naming (e.g., net.ipv4.conf.all.forwarding).
  - **value** `string`
    Value to set for this key.

## system

System-level settings

- **archive <name>** `list`
  Named config archive destinations
  - **filename** `string`
    Filename format with token substitution
  - **location** `string`
    Archive destination URL (file://, http://, https://)
  - **on-change** `boolean`
    Time-based only: skip if config unchanged since last archive
  - **timeout** `string`
    HTTP upload timeout (Go duration format, e.g., 30s)
  - **trigger** `enumeration`
    When to archive
- **authentication** `container`
  System authentication settings
  - **radius** `container`
    RADIUS server configuration for operator/admin login (SSH, web, MCP), RFC 2865. Separate from the L2TP subscriber RADIUS path, which lives under the l2tp root.
    - **default-profile** `string[]`
      Ze authorization profile name(s) assigned when the Access-Accept carries no profile-attribute. Optional: leave it unset when the RADIUS server always names a profile via profile-attribute. When the server does not, and no default is set here, the login names no profile and is denied -- ze never authorizes a user it cannot attach a profile to.
    - **profile-attribute** `enumeration`
      Access-Accept reply attribute whose value(s) name the ze authorization profile(s) for the user
    - **retries** `uint8`
      Retransmit count per server before failover
    - **server <address>** `list`
      RADIUS servers, tried in configured order
      - **key** `string`
        RADIUS shared secret (RFC 2865)
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
        Shared encryption key
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
      Ze authorization profile name(s). At least one is required: a level mapped to no profiles would authenticate the user while granting nothing, and authorization reads an empty profile set as 'no opinion' rather than 'deny'. To deny a privilege level, leave it out of the mapping entirely.
  - **user <name>** `list`
    Authenticated user (local)
    - **password** `string`
      Bcrypt-hashed password (canonical form, e.g. $2a$10$...). Write plaintext via plaintext-password -- it is hashed into this leaf on commit and then discarded.
    - **plaintext-password** `string`
      Write-only plaintext password. On commit, the config system bcrypt-hashes this value into the canonical password leaf and removes this one from the tree. Never persisted to the config file.
    - **profile** `string[]`
      Authorization profile name(s) assigned to this user
    - **public-keys <name>** `list`
      SSH public keys for key-based authentication. Each entry is a named key (e.g. user@host).
      - **key** `string`
        Base64-encoded public key data (the middle field of an SSH public key line, without type prefix or comment suffix)
      - **type** `enumeration`
        SSH public key algorithm
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
  Maximum number of config archive files to keep per file:// archive location. After each archive write, files beyond this count are pruned (oldest first). 0 disables pruning (keep all). Only applies to file:// scheme archives; HTTP archives are not pruned.
- **conntrack** `container`
  Connection tracking (conntrack) module loading, table tuning, and timeout configuration
  - **accounting** `empty`
    Enable per-connection byte/packet counters (nf_conntrack_acct)
  - **checksum** `empty`
    Enable conntrack checksum verification (nf_conntrack_checksum)
  - **expect-max** `uint16`
    Maximum expected connections (nf_conntrack_expect_max)
  - **hash-size** `uint32`
    Conntrack hash table buckets (nf_conntrack_buckets)
  - **log-invalid** `enumeration`
    Log invalid packets for the specified protocol (nf_conntrack_log_invalid)
  - **module** `enumeration[]`
    Conntrack helper modules to load. Load-only: removing a module stops loading on next boot but does not unload at runtime.
  - **table-size** `uint32`
    Maximum conntrack table entries (nf_conntrack_max)
  - **tcp** `container`
    TCP connection tracking behavior flags
    - **be-liberal** `boolean`
      Accept out-of-window packets for established connections
    - **ignore-invalid-rst** `boolean`
      Ignore RST packets with invalid sequence numbers
    - **loose** `boolean`
      Allow tracking connections started before conntrack loaded
    - **max-retrans** `uint8`
      Maximum retransmissions before marking connection invalid
  - **timeout** `container`
    Connection tracking timeouts per protocol
    - **dccp** `container`
      DCCP connection tracking timeouts
      - **closereq** `uint32`
        Close-request timeout (seconds)
      - **closing** `uint32`
        Closing timeout (seconds)
      - **open** `uint32`
        Open timeout (seconds)
      - **partopen** `uint32`
        Partopen timeout (seconds)
      - **request** `uint32`
        Request timeout (seconds)
      - **respond** `uint32`
        Respond timeout (seconds)
      - **timewait** `uint32`
        Time-wait timeout (seconds)
    - **generic** `uint32`
      Generic timeout in seconds (nf_conntrack_generic_timeout)
    - **gre** `container`
      GRE connection tracking timeouts
      - **stream** `uint32`
        Stream timeout (seconds)
      - **timeout** `uint32`
        Default timeout (seconds)
    - **icmp** `container`
      ICMP connection tracking timeouts
      - **timeout** `uint32`
        Timeout (seconds)
    - **icmpv6** `container`
      ICMPv6 connection tracking timeouts
      - **timeout** `uint32`
        Timeout (seconds)
    - **sctp** `container`
      SCTP connection tracking timeouts
      - **closed** `uint32`
        Closed timeout (seconds)
      - **cookie-echoed** `uint32`
        Cookie-echoed timeout (seconds)
      - **cookie-wait** `uint32`
        Cookie-wait timeout (seconds)
      - **established** `uint32`
        Established timeout (seconds)
      - **heartbeat-sent** `uint32`
        Heartbeat-sent timeout (seconds)
      - **shutdown-ack-sent** `uint32`
        Shutdown-ACK-sent timeout (seconds)
      - **shutdown-recd** `uint32`
        Shutdown-received timeout (seconds)
      - **shutdown-sent** `uint32`
        Shutdown-sent timeout (seconds)
    - **tcp** `container`
      TCP connection tracking timeouts
      - **close** `uint32`
        Close timeout (seconds)
      - **close-wait** `uint32`
        Close-wait timeout (seconds)
      - **established** `uint32`
        Established timeout (seconds)
      - **fin-wait** `uint32`
        FIN-wait timeout (seconds)
      - **last-ack** `uint32`
        Last-ACK timeout (seconds)
      - **max-retrans** `uint32`
        Max retransmission timeout (seconds)
      - **syn-recv** `uint32`
        SYN-received timeout (seconds)
      - **syn-sent** `uint32`
        SYN-sent timeout (seconds)
      - **time-wait** `uint32`
        Time-wait timeout (seconds)
      - **unacknowledged** `uint32`
        Unacknowledged timeout (seconds)
    - **udp** `container`
      UDP connection tracking timeouts
      - **stream** `uint32`
        Stream timeout (seconds)
      - **timeout** `uint32`
        Default timeout (seconds)
  - **timestamp** `empty`
    Enable per-connection timestamps (nf_conntrack_timestamp)
- **console** `container`
  Serial console configuration for headless CPE devices
  - **device <name>** `list`
    Serial device to configure as console
    - **speed** `enumeration`
      Baud rate (default 115200)
- **dns** `container`
  DNS resolver tuning and resolv.conf settings
  - **cache-size** `uint32`
    Maximum cached entries (0 disables caching)
  - **cache-ttl** `uint32`
    Maximum cache TTL in seconds (0 uses response TTL only)
  - **dnssec-validation** `enumeration`
    Upstream-answer DNSSEC validation for the stub resolver (RFC 4035 stub model). off leaves resolution unchanged; permissive and strict set the EDNS0 DO bit and rely on a validating upstream (CD=0) to SERVFAIL a broken chain. strict rejects such answers as an error; permissive logs and returns the empty result. Insecure/unsigned zones (NOERROR, AD=0) are always accepted.
  - **resolv-conf-path** `string`
    Path where DNS servers are written as resolv.conf. Default /tmp/resolv.conf suits gokrazy (read-only rootfs). Set to /etc/resolv.conf on standard Linux. Empty string disables resolv.conf writing.
  - **timeout** `uint16`
    Query timeout in seconds
- **domain** `string`
  System domain, supports $ENV_VAR expansion
- **host** `string`
  System hostname, supports $ENV_VAR expansion
- **name-server** `ip-address[]`
  Static DNS name servers, written to resolv.conf and used by ze internal resolver
- **peeringdb** `container`
  PeeringDB API configuration for prefix data lookups
  - **margin** `uint8`
    Percentage margin above PeeringDB count for prefix maximum (0-100)
  - **url** `string`
    PeeringDB-compatible API base URL
- **tuning** `container`
  Runtime hardware tuning applied at startup and on config commit
  - **cpu** `container`
    CPU frequency scaling tuning
    - **governor** `enumeration`
      CPU scaling governor applied to all cores
  - **ethtool <interface>** `list`
    Per-interface ethtool settings
    - **ring** `container`
      Ring buffer sizes
      - **rx** `uint16`
        Receive ring buffer size
      - **tx** `uint16`
        Transmit ring buffer size
  - **irq-affinity <interface>** `list`
    Pin NIC IRQs to specific CPUs
    - **cpus** `string`
      CPU list (e.g. 0,2,4-7)
- **update-check** `container`
  System version check and platform update orchestration. On gokrazy, image updates are managed by gokrazy.
  - **auto-apply** `boolean`
    When true, automatically download, verify, and stage Ze binary updates on platforms managed by Ze. When false, only check and report.
  - **interval** `uint32`
    Check interval in seconds (default 86400 = daily, minimum 60)
  - **maintenance-window** `container`
    Time window when binary replacement may occur. Download and verification proceed at any time.
    - **end** `string`
      End time HH:MM in local time.
    - **start** `string`
      Start time HH:MM in local time.
  - **restart** `container`
    What to do after an update is staged. If omitted: manual. If present with immediate: restart after brief drain. If present with time: restart at that daily time.
    - **immediate** `empty`
      Restart automatically after staging (5s drain delay). Mutually exclusive with time.
    - **time** `string`
      Daily restart time, HH:MM in local time. Mutually exclusive with immediate.
  - **spread** `uint32`
    Maximum random delay in seconds before downloading after a new version is detected. 0 disables spread.
  - **url** `string`
    HTTPS URL serving a JSON object with a 'version' field, e.g. {"version":"26.05.17"}. Ze compares this value lexicographically against its own release version.

## telemetry

Telemetry export configuration

- **prometheus** `container`
  Prometheus metrics HTTP export
  - **basic-auth** `container`
    HTTP Basic Authentication for the Prometheus service
    - **enabled** `boolean`
      Require HTTP Basic Authentication for Prometheus metrics and health endpoints
    - **password** `string`
      Bcrypt-hashed HTTP Basic Authentication password. Write plaintext via plaintext-password.
    - **plaintext-password** `string`
      Write-only password. On commit, the config system hashes this value into password and removes this leaf.
    - **realm** `string`
      HTTP Basic Authentication realm
    - **username** `string`
      HTTP Basic Authentication username
  - **collector <name>** `list`
    Deprecated: use netdata/collector. Per-Netdata-collector overrides (enable/disable, interval).
    - **enabled** `boolean`
      Enable or disable this collector
    - **interval** `uint16`
      Override sampling interval for this collector (seconds). Inherits global interval if not set.
  - **enabled** `boolean`
    Enable Prometheus metrics endpoint
  - **interval** `uint16`
    Deprecated: use netdata/interval. Netdata-compatible OS collector sampling interval in seconds.
  - **netdata** `container`
    Netdata-compatible OS collector metrics. These settings do not affect Ze-native metrics.
    - **collector <name>** `list`
      Per-Netdata-collector overrides (enable/disable, interval)
      - **enabled** `boolean`
        Enable or disable this Netdata-compatible OS collector
      - **interval** `uint16`
        Override sampling interval for this collector (seconds). Inherits global Netdata interval if not set.
    - **enabled** `boolean`
      Enable Netdata-compatible OS collectors
    - **interval** `uint16`
      Netdata-compatible OS collector sampling interval in seconds
    - **prefix** `string`
      Metric name prefix for Netdata-compatible OS collector metrics only
  - **path** `string`
    HTTP path for metrics endpoint
  - **prefix** `string`
    Deprecated: use netdata/prefix. Metric name prefix for Netdata-compatible OS collector metrics only.
  - **server <name>** `list`
    Prometheus listen endpoints
    - **ip** `ip-address`
      Listen IP address
    - **port** `listener-port`
      Listen TCP port; 0 means OS-assigned

## traffic

Traffic subsystem: QoS control and byte-usage accounting.

- **control** `container`
  *Provided by `traffic` ([ze-traffic-control-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/traffic/yang/ze-traffic-control-conf.yang))*
  Per-interface traffic control configuration
  - **backend** `string`
    Traffic control backend implementation. Default is tc (Linux iproute2 queueing disciplines). Future backends can declare themselves via traffic.RegisterBackend. The ze:backend YANG extension on feature nodes declares per-feature backend support so the commit-time gate rejects configs that try to use unsupported qdiscs or filter types.
  - **interface <name>** `list`
    Traffic control for a named interface
    - **qdisc** `container`
      Root queueing discipline
      - **class <name>** `list`
        Traffic class within the qdisc
        - **ceil** `rate-bps`
          Maximum rate (defaults to rate if omitted)
        - **match <type>** `list`
          Filter to classify packets into this class
          - **value** `string`
            Match value (mark hex, dscp name, protocol name)
        - **priority** `uint8`
          Scheduling priority (0 = highest)
        - **rate** `rate-bps`
          Guaranteed rate
      - **default-class** `string`
        Name of the default class for unclassified traffic
      - **type** `qdisc-type`
        Qdisc type
- **usage** `container`
  *Provided by `traffic-usage` ([ze-traffic-usage-cmd.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/trafficusage/yang/ze-traffic-usage-cmd.yang), [ze-traffic-usage-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang))*
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
        Override the global stale-timeout (milliseconds; 0 disables) for this interface (inherits when unset).
      - **track-ip** `boolean`
        Override the global track-ip for this interface (inherits the global value when unset).
  - **interval** `uint32`
    Map poll interval in milliseconds (100ms..1h).
  - **max-entries** `uint32`
    Per-map LRU capacity; least-recently-used entries are evicted beyond this, keeping top talkers.
  - **stale-timeout** `uint32`
    Remove a metric series unseen within this many milliseconds; 0 disables cleanup.
  - **track-ip** `boolean`
    Also account bytes per source (ingress) / destination (egress) IPv4. Off by default to bound metric cardinality.

## vpn

*Provided by `ike`*

VPN subsystems.

- **ipsec** `container`
  IPsec site-to-site VPN configuration.
  - **esp-group <name>** `list`
    ESP (Encapsulating Security Payload) group.
    - **lifetime** `uint32`
      SA lifetime in seconds. 0 disables expiry.
    - **pfs** `enumeration`
      Perfect Forward Secrecy for Child SA rekeying.
    - **proposal <number>** `list`
      ESP proposal. Lower number = higher priority.
      - **encryption** `encryption-algo`
        Encryption algorithm.
      - **hash** `hash-algo`
        Integrity algorithm. Optional for AEAD ciphers.
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
      IKE SA lifetime in seconds. 0 disables reauth.
    - **proposal <number>** `list`
      IKE proposal. Lower number = higher priority.
      - **dh-group** `uint8`
        Diffie-Hellman group number (RFC 7296 Section 3.3.2).
      - **encryption** `encryption-algo`
        Encryption algorithm.
      - **hash** `hash-algo`
        PRF/integrity algorithm.
  - **interface** `string`
    WAN interface for IPsec traffic.
  - **remote-access** `container`
    Remote access VPN for road warrior clients (EAP).
    - **authentication** `container`
      EAP authentication settings for remote access.
      - **mode** `enumeration`
        EAP authentication method.
      - **x509** `container`
        Server certificate references for EAP.
        - **ca-certificate** `string`
          Name of the CA certificate in the PKI store.
        - **certificate** `string`
          Name of the server certificate in the PKI store.
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
      Virtual IP pool for client address assignment (IKEv2 Configuration Payload, RFC 7296 Section 2.19).
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
          Name of the CA certificate in the PKI store (EAP and X.509 modes).
        - **certificate** `string`
          Name of the device certificate in the PKI store (EAP-TLS and X.509 modes).
        - **local-id** `string`
          Local identity for IKE negotiation.
        - **mode** `enumeration`
          Authentication mode.
        - **pre-shared-secret** `string`
          Pre-shared key ($9$-encoded).
        - **remote-id** `string`
          Remote identity for IKE negotiation.
        - **x509** `container`
          X.509 certificate references (deprecated, use direct leaves).
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
        Local endpoint address or interface name.
      - **remote-address** `string`
        Remote endpoint address or DNS hostname.
      - **vti** `container`
        Virtual Tunnel Interface binding.
        - **bind** `string`
          VTI interface name to bind this peer's traffic to.

## vpp

*Provided by `vpp` ([ze-vpp-conf.yang](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/vpp/yang/ze-vpp-conf.yang))*

VPP data plane configuration.

- **api-socket** `string`
  GoVPP binary API Unix socket path.
- **cpu** `container`
  VPP CPU pinning configuration.
  - **main-core** `uint8`
    CPU core for VPP main thread. Omit for automatic assignment.
  - **poll-sleep** `string`
    Fixed sleep between VPP main-loop polls, expressed in whole milliseconds (the only accepted unit), e.g. 10ms. Emitted as unix { poll-sleep-usec N } in startup.conf (1ms = 1000 microseconds). Omit for VPP's default (no sleep: workers busy-poll at 100% CPU) for lowest latency. Set a non-zero value up to 100ms on shared or dev hosts to trade latency for idle CPU. An explicit 0ms is emitted and equals the default.
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
  Enable VPP integration. When true, ze manages VPP lifecycle: generates startup.conf, binds DPDK NICs, starts VPP process, connects via GoVPP.
- **external** `boolean`
  When true, ze does NOT exec or supervise the VPP process, generate startup.conf, or bind DPDK NICs; the external supervisor owns all of that. Ze only connects via GoVPP to api-socket. Use this for systemd-managed VPP, container sidecar deployments, or the `ze-test vpp` stub harness. Default: false (ze owns VPP lifecycle).
- **lcp** `container`
  Linux Control Plane plugin. Creates TAP mirrors in Linux for VPP interfaces, enabling routing daemons (ze BGP) to use Linux TCP on VPP-managed NICs.
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
  Optional VPP plugin enablement. startup.conf uses 'plugin default { disable }', so only the always-on plugins (dpdk, plus linux-cp when lcp is enabled) and the plugins toggled on here are loaded.
  - **wireguard** `boolean`
    Load wireguard_plugin.so so the vpp interface backend can program WireGuard tunnels (interface wireguard under backend vpp).
- **stats** `container`
  VPP stats segment configuration.
  - **poll-interval** `uint16`
    Stats polling interval in seconds. Controls how often ze reads VPP's stats segment for Prometheus metrics.
  - **segment-size** `string`
    Stats segment shared memory size.
  - **socket-path** `string`
    Stats segment Unix socket path.
