# Configuration Reference

86 plugins across 38 groups, 92 YANG modules total. Generated from every real `registry.Registration{}` in `../main/internal/` and the YANG modules each one actually imports -- grouped by the config root each plugin's own registration declares, not a hand-picked category list. Every subsystem here, not only BGP: see [the Configuration guide](https://ze-software.github.io/ze/docs/features/configuration/) for a narrative walkthrough of BGP peer config specifically.

## Anomaly Detect (`anomaly-detect`, 1 plugins)

### anomaly-detect-feature-source

source: `internal/plugins/anomaly/detect` -- config root: `anomaly-detect` -- depends on: `config-loaded`

Behavioral anomaly detector (report-only): per-entity pattern-of-life over trafficfeature

`ze-anomaly-detect-conf.yang`

```yang
module ze-anomaly-detect-conf {
  namespace "urn:ze:anomaly-detect-conf";
  prefix anomaly-detect;

  import ze-types { prefix zt; }

  container anomaly-detect {
    leaf enabled {
      type boolean;
      default false;
      description "Enable the behavioral anomaly detector (report-only; emits incidents, takes no action).";
    }
    leaf deviation-threshold {
      type zt:decimal-2 { range "1.00..100.00"; }
      default "3.00";
      description "Sigma at or above which a per-entity feature deviation fires.";
    }
    leaf min-features-to-correlate {
      type uint8 { range "1..6"; }
      default 2;
      description "Minimum distinct features that must fire on one entity/window before an incident is scored (weak-signal correlation gate).";
    }
    leaf min-cohort-size {
      type uint16 { range "2..1024"; }
      default 4;
      description "Minimum cohort (source-prefix bucket) members before peer-group rarity is scored; smaller cohorts fall back to self-deviation only.";
    }
    leaf corroboration-weight {
      type zt:decimal-2 { range "0.00..1.00"; }
      default "0.50";
      description "Discount applied to corroborating features when combining scores, so correlated features do not double-count.";
    }
    leaf confirm-duration {
      type uint16 { range "1..3600"; }
      default 3;
      description "Consecutive above-threshold ticks before an incident is confirmed and emitted.";
    }
    leaf clear-consecutive {
      type uint8 { range "1..100"; }
      default 10;
      description "Consecutive below-threshold ticks before an active incident clears.";
    }
    leaf baseline-window {
      type uint32 { range "10..86400"; }
      default 300;
      description "Per-entity baseline horizon in ticks (the EWMA smoothing factor is derived from it).";
    }
    leaf cohort-prefix-len-v4 {
      type uint8 { range "8..32"; }
      default 24;
      description "Source-prefix length that buckets IPv4 entities into peer-group cohorts.";
    }
    leaf cohort-prefix-len-v6 {
      type uint8 { range "16..64"; }
      default 48;
      description "Source-prefix length that buckets IPv6 entities into peer-group cohorts.";
    }
  }
}
```

## Anomaly Shape (`anomaly-shape`, 1 plugins)

### anomaly-shape-firewall

source: `internal/plugins/anomaly/shape` -- config root: `anomaly-shape` -- depends on: `config-loaded`

Shadow-first autonomous anomaly responder: per-source rate-limit with arm/auto-revert/kill-switch

`ze-anomaly-shape-conf.yang`

```yang
module ze-anomaly-shape-conf {
  namespace "urn:ze:anomaly-shape-conf";
  prefix anomaly-shape;

  import ze-types { prefix zt; }

  container anomaly-shape {
    leaf mode {
      type enumeration { enum shadow; enum armed; }
      default shadow;
      description "shadow (default): log the would-be action, install nothing. armed: install live per-source firewall actions.";
    }
    leaf action {
      type enumeration { enum limit; enum drop; }
      default limit;
      description "Armed action: rate-limit the source (surgical) or drop it (fallback).";
    }
    leaf limit-rate {
      type uint64 { range "1..max"; }
      default 1000;
      description "Rate for the limit action, in packets per limit-unit.";
    }
    leaf limit-unit {
      type enumeration { enum second; enum minute; enum hour; enum day; }
      default second;
      description "Time unit for limit-rate.";
    }
    leaf limit-burst {
      type uint32;
      default 0;
      description "Burst allowance for the limit action.";
    }
    leaf auto-revert-ttl {
      type uint16 { range "5..3600"; }
      default 300;
      description "Seconds after the last signal before an armed action auto-reverts, regardless of any clear event (safety ceiling).";
    }
    leaf blast-radius-cap {
      type uint16 { range "1..1024"; }
      default 16;
      description "Maximum concurrently-armed live actions; further arm attempts are refused.";
    }
    leaf kill-switch {
      type boolean;
      default false;
      description "When true, revert every armed action and force the responder to shadow.";
    }
    leaf-list allowlist {
      type zt:ip-prefix;
      description "Protected source prefixes that are never armed (self-lockout guard for management / control-plane sources).";
    }
  }
}
```

## Bfd (`bfd`, 1 plugins)

### bfd

source: `internal/component/bfd` -- config root: `bfd`

Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)

`ze-bfd-api.yang`

```yang
module ze-bfd-api {
    namespace "urn:ze:bfd:api";
    prefix bfdapi;

    description
        "BFD API operations for Ze. RPCs for observing session state
         and resolved profiles. RFC 5880 sections 6.8.1 (state
         variables) and 6.8.4 (detection time) supply the field
         semantics.";

    revision 2026-04-11 {
        description "Initial BFD observability API (spec-bfd-4).";
    }

    rpc show-sessions {
        description "Return every live BFD session.";
        output {
            leaf sessions {
                type string;
                description "JSON array of session objects.";
            }
        }
    }

    rpc show-session {
        description "Return one BFD session by peer address.";
        input {
            leaf peer {
                type string;
                mandatory true;
                description "Peer IPv4 or IPv6 address.";
            }
        }
        output {
            leaf session {
                type string;
                description "JSON object with session detail.";
            }
        }
    }

    rpc show-profile {
        description "Return resolved BFD profile parameters.";
        input {
            leaf name {
                type string;
                description "Profile name (empty returns every profile).";
            }
        }
        output {
            leaf profiles {
                type string;
                description "JSON array of profile objects.";
            }
        }
    }
}
```

`ze-bfd-cmd.yang`

```yang
module ze-bfd-cmd {
    namespace "urn:ze:bfd:cmd";
    prefix bfdcmd;
    import ze-extensions    { prefix ze; }
    import ze-cli-show-cmd  { prefix clishowcmd; }
    description
        "BFD session state, timers, and profile configuration.";
    revision 2026-04-11 {
        description "Initial revision (spec-bfd-4).";
    }

    augment "/clishowcmd:show" {
        container bfd {
            config false;
            description "BFD session state and timer configuration";

            container sessions {
                config false;
                ze:command "ze-bfd-api:show-sessions";
                description "List all active BFD sessions.
One line per session: peer address, state, negotiated tx/rx
intervals, and detect multiplier.";
            }

            container session {
                config false;
                description "Show one BFD session selected by peer address.";

                container address {
                    config false;
                    ze:command "ze-bfd-api:show-session";
                    description "Show full detail for one BFD session.
Pass the peer address. Returns local/remote discriminators,
negotiated timers, detection time, and packet counters.";
                    leaf address {
                        type string;
                        mandatory true;
                        description "Peer address";
                    }
                }
            }

            container profile {
                config false;
                ze:command "ze-bfd-api:show-profile";
                description "Show BFD timer profiles with effective values.
Returns min-tx, min-rx, and detect-multiplier after inheritance.
Use 'show bfd profile' for every profile or 'show bfd profile name <name>'
for one profile.";

                container name {
                    config false;
                    ze:command "ze-bfd-api:show-profile";
                    description "Show one BFD profile by name.";
                    leaf name {
                        type string;
                        mandatory true;
                        description "Profile name";
                    }
                }
            }
        }
    }
}
```

`ze-bfd-conf.yang`

```yang
module ze-bfd-conf {
    namespace "urn:ze:bfd:conf";
    prefix bfd;

    import ze-extensions { prefix ze; }

    description
        "Bidirectional Forwarding Detection (BFD) configuration for Ze.

         Implements RFC 5880 (base), RFC 5881 (single-hop), and
         RFC 5883 (multi-hop). Profiles hold reusable timer and feature
         bundles. Sessions explicitly pinned in configuration exist even
         without a protocol client; protocol-driven sessions (from BGP,
         OSPF, static routes) are created at runtime via the plugin
         Service interface and do not appear here.";

    revision 2026-04-11 {
        description "Initial BFD config skeleton.";
    }

    container bfd {
        description "BFD control configuration for Ze.";

        leaf enabled {
            type boolean;
            default true;
            description "Master switch for the BFD plugin.";
        }

        leaf persist-dir {
            type string;
            description
                "Absolute path to the directory where the plugin
                 persists TX sequence numbers for Meticulous Keyed
                 authentication. RFC 5880 Section 6.7.3 requires the
                 sequence to survive process restarts; without this
                 leaf set, Meticulous sessions still work at runtime
                 but a fresh process loses its sequence floor until
                 the peer's replay window slides forward.";
        }

        leaf bind-v6 {
            type boolean;
            default false;
            description
                "When true, the BFD plugin also binds an IPv6 socket
                 (::0) alongside the IPv4 socket for every (vrf, mode)
                 loop. The paired sockets share the same RX channel
                 via a transport.Dual wrapper and send routes by
                 destination address family. Stage 2b
                 (spec-bfd-2b-ipv6-transport) ships this leaf; pinned
                 IPv6 sessions require it to be set explicitly so
                 operators who do not need v6 do not open a second
                 socket per loop.";
        }

        list profile {
            key "name";
            description
                "Reusable timer and feature profile. Sessions reference
                 a profile by name and inherit every field below.";

            leaf name {
                type string;
                description "Profile name used as the reference key.";
            }

            leaf detect-multiplier {
                type uint8 {
                    range "1..255";
                }
                default 3;
                description
                    "Number of consecutive missed Control packets that
                     trigger a Down transition. RFC 5880 Section 6.8.4.";
            }

            leaf desired-min-tx-us {
                type uint32;
                default 300000;
                description
                    "Local target transmit rate in microseconds. The
                     slow-start floor of 1 000 000 us applies while the
                     session is not Up, per RFC 5880 Section 6.8.3.";
            }

            leaf required-min-rx-us {
                type uint32;
                default 300000;
                description
                    "Minimum inter-packet gap the local end can handle,
                     in microseconds.";
            }

            leaf passive {
                type boolean;
                default false;
                description
                    "Active (default) transmits from session creation.
                     Passive transmits nothing until a Control packet
                     arrives from the peer. RFC 5883 Section 4.3.";
            }

            container echo {
                presence "enables BFD Echo mode on sessions using this profile";
                description
                    "RFC 5880 Section 6.4 / RFC 5881 Section 5 Echo
                     mode. Single-hop only -- RFC 5883 Section 4
                     explicitly prohibits multi-hop echo, and the
                     parser rejects echo on a multi-hop session. When
                     active, the engine sends echo packets on UDP
                     port 3785 at the peer's advertised
                     RequiredMinEchoRxInterval and slows its async
                     Control TX to the peer's RequiredMinRxInterval.";

                leaf desired-min-echo-tx-us {
                    type uint32;
                    default 50000;
                    description
                        "Local target echo transmit rate in
                         microseconds. The effective echo rate is
                         max(local desired, peer RequiredMinEchoRx).";
                }
            }

            container auth {
                presence "enables authentication on sessions using this profile";
                description
                    "RFC 5880 Section 6.7 authentication parameters.
                     Sessions inheriting this profile sign every
                     outgoing Control packet with the configured type
                     and verify incoming packets using the same key.
                     Simple Password (type 1) is rejected at parse
                     time -- it provides no cryptographic protection
                     and RFC 5880 warns against using it.";

                leaf type {
                    type enumeration {
                        enum keyed-md5 {
                            description "RFC 5880 Section 6.7.3 Keyed MD5.";
                        }
                        enum meticulous-keyed-md5 {
                            description "RFC 5880 Section 6.7.3 Meticulous Keyed MD5 (strict sequence increase).";
                        }
                        enum keyed-sha1 {
                            description "RFC 5880 Section 6.7.4 Keyed SHA1.";
                        }
                        enum meticulous-keyed-sha1 {
                            description "RFC 5880 Section 6.7.4 Meticulous Keyed SHA1 (strict sequence increase).";
                        }
                    }
                    mandatory true;
                    description
                        "Authentication type. Simple Password is
                         intentionally absent from the enum.";
                }

                leaf key-id {
                    type uint8;
                    mandatory true;
                    description "Auth Key ID (RFC 5880 Section 6.7.1).";
                }

                leaf secret {
                    type string;
                    mandatory true;
                    ze:sensitive;
                    description
                        "Shared secret for the keyed digest. Redacted
                         from `ze config show` output. MD5 variants
                         use the first 16 bytes of the secret, SHA1
                         variants the first 20.";
                }
            }
        }

        list single-hop-session {
            key "peer vrf interface";
            description
                "Per-link BFD session (RFC 5881). Port 3784, TTL=255.
                 Protocol clients should NOT use this list; they call
                 the plugin Service interface at runtime.";

            leaf peer {
                type string;
                description "Peer IPv4 or IPv6 address.";
            }
            leaf local {
                type string;
                description "Local source address (optional).";
            }
            leaf interface {
                type string;
                description "Egress interface name.";
            }
            leaf vrf {
                type string;
                default "default";
            }
            leaf profile {
                type string;
                description "Named profile to inherit timer parameters from.";
            }
            leaf shutdown {
                type boolean;
                default false;
                description
                    "When true, the session stays in AdminDown state.
                     RFC 5880 Section 6.8.16.";
            }
        }

        list multi-hop-session {
            key "peer local vrf";
            description
                "Multi-hop BFD session (RFC 5883). Port 4784, no GTSM.";

            leaf peer {
                type string;
            }
            leaf local {
                type string;
                description "Local source address. Required for multi-hop.";
            }
            leaf vrf {
                type string;
                default "default";
            }
            leaf min-ttl {
                type uint8;
                default 254;
                description
                    "Minimum acceptable TTL on receive. Weak replacement
                     for GTSM on multi-hop paths.";
            }
            leaf profile {
                type string;
            }
            leaf shutdown {
                type boolean;
                default false;
            }
        }
    }
}
```

## Bgp (`bgp`, 39 plugins)

### bgp

source: `internal/component/bgp/plugin` -- config root: `bgp`

BGP routing daemon

`ze-bgp-api.yang`

```yang
module ze-bgp-api {
    namespace "urn:ze:bgp:api";
    prefix bgpapi;

    import ze-types { prefix zt; }

    description
        "BGP API operations for Ze.
         RPCs for peer management, route operations, cache control,
         and event subscriptions. Notifications for BGP events.";

    revision 2026-02-01 {
        description "Initial revision";
    }

    // Introspection

    rpc help {
        description "List BGP subcommands";
        output {
            leaf-list subcommands { type string; description "Available subcommands"; }
        }
    }

    rpc command-list {
        description "List BGP commands";
        output {
            list command {
                uses zt:command-info;
            }
        }
    }

    rpc command-help {
        description "Show BGP command details";
        input {
            leaf name { type string; mandatory true; description "Command name"; }
        }
        output {
            leaf help { type string; description "Detailed help text"; }
        }
    }

    rpc command-complete {
        description "Complete BGP command/args";
        input {
            leaf partial { type string; mandatory true; description "Partial command string"; }
        }
        output {
            leaf-list completions { type string; description "Completion candidates"; }
        }
    }

    // Plugin configuration

    rpc plugin-encoding {
        description "Set event encoding (json|text)";
        input {
            leaf encoding { type zt:encoding-mode; mandatory true; description "Encoding format"; }
        }
    }

    rpc plugin-format {
        description "Set wire format (hex|base64|parsed|full)";
        input {
            leaf format { type zt:wire-format; mandatory true; description "Wire format"; }
        }
    }

    rpc plugin-ack {
        description "Set ACK timing (sync|async)";
        input {
            leaf mode { type zt:ack-mode; mandatory true; description "ACK mode"; }
        }
    }

    // Peer operations

    rpc peer-list {
        description "List configured peers";
        input {
            leaf selector { type zt:peer-selector; description "Peer filter"; }
        }
        output {
            list peer {
                uses zt:peer-info;
            }
        }
    }

    rpc peer-show {
        description "Show detailed peer information";
        input {
            leaf selector { type zt:peer-selector; description "Peer filter"; }
        }
        output {
            list peer {
                uses zt:peer-info;
            }
        }
    }

    rpc summary {
        description "Show BGP summary (peer table with statistics)";
        output {
            leaf uptime { type string; description "Daemon uptime"; }
            leaf peers-configured { type uint32; description "Total configured peers"; }
            leaf peers-established { type uint32; description "Peers in Established state"; }
            list peer {
                uses zt:peer-info;
            }
        }
    }

    rpc peer-show-capabilities {
        description "Show negotiated capabilities for peer";
        input {
            leaf selector { type zt:peer-selector; mandatory true; description "Peer filter"; }
        }
        output {
            leaf peer { type zt:ip-address; description "Peer address"; }
            leaf state { type string; description "FSM state"; }
            leaf negotiation-complete { type boolean; description "True if OPEN exchange completed"; }
        }
    }

    rpc peer-show-statistics {
        description "Show per-peer update statistics with rates";
        input {
            leaf selector { type zt:peer-selector; description "Peer filter"; }
        }
        output {
            leaf address { type zt:ip-address; description "Peer address"; }
            leaf peer-as { type zt:asn; description "Peer ASN"; }
            leaf state { type string; description "FSM state"; }
            leaf uptime { type string; description "Session uptime"; }
            leaf updates-received { type uint32; description "UPDATE messages received"; }
            leaf updates-sent { type uint32; description "UPDATE messages sent"; }
            leaf keepalives-received { type uint32; description "KEEPALIVE messages received"; }
            leaf keepalives-sent { type uint32; description "KEEPALIVE messages sent"; }
            leaf eor-received { type uint32; description "End-of-RIB markers received"; }
            leaf eor-sent { type uint32; description "End-of-RIB markers sent"; }
            leaf rate-updates-received { type string; description "Updates received per second"; }
            leaf rate-updates-sent { type string; description "Updates sent per second"; }
            leaf rate-keepalives-received { type string; description "Keepalives received per second"; }
            leaf rate-keepalives-sent { type string; description "Keepalives sent per second"; }
        }
    }

    rpc peer-clear-soft {
        description "Soft-clear peer (send ROUTE-REFRESH per negotiated family)";
        input {
            leaf selector { type zt:peer-selector; mandatory true; description "Peer address"; }
        }
        output {
            leaf peer { type zt:ip-address; description "Peer address"; }
            leaf action { type string; description "Always soft-clear"; }
            leaf-list families-refreshed { type zt:address-family; description "Families refreshed"; }
        }
    }

    rpc peer-add {
        description "Add a new peer";
        input {
            leaf address { type zt:ip-address; mandatory true; description "Peer address"; }
            leaf asn { type zt:asn; mandatory true; description "Peer ASN"; }
            leaf local-as { type zt:asn; description "Local ASN override"; }
            leaf local-address { type zt:ip-address; description "Local bind address"; }
            leaf router-id { type zt:ipv4-address; description "Router ID override"; }
            leaf receive-hold-time { type uint16; description "Receive hold time (RFC 4271)"; }
            leaf send-hold-time { type uint16; description "Send hold time (RFC 9687, 0=auto)"; }
            leaf connect-retry { type uint16; description "Connect retry interval"; }
            container local {
                description "Local connection settings";
                leaf connect { type boolean; description "Initiate outbound connections (default true)"; }
            }
            container remote {
                description "Remote connection settings";
                leaf accept { type boolean; description "Accept inbound connections (default true)"; }
            }
        }
    }

    rpc peer-remove {
        description "Remove a peer";
        input {
            leaf address { type zt:ip-address; mandatory true; description "Peer address"; }
        }
    }

    rpc peer-teardown {
        description "Teardown peer session with CEASE notification";
        input {
            leaf selector { type zt:peer-selector; mandatory true; description "Peer filter"; }
            leaf subcode { type uint8; description "CEASE subcode"; }
        }
    }

    rpc peer-flush {
        description "Wait for forward pool to drain (barrier)";
        input {
            leaf selector { type zt:peer-selector; mandatory true; description "Peer filter (* for all)"; }
        }
        output {
            leaf peer { type string; description "Peer selector that was flushed"; }
            leaf action { type string; description "Always flush"; }
        }
    }

    // Route operations

    rpc peer-update {
        description "Batch UPDATE with text/hex/b64 encoding";
        input {
            leaf peer-selector { type zt:peer-selector; mandatory true; description "Target peers"; }
            leaf encoding { type zt:encoding-mode; mandatory true; description "Command encoding"; }
            leaf command { type string; mandatory true; description "Route DSL command"; }
        }
        output {
            leaf announced { type uint32; description "Number of NLRIs announced"; }
            leaf withdrawn { type uint32; description "Number of NLRIs withdrawn"; }
        }
    }

    // Route refresh (RFC 7313)

    rpc peer-borr {
        description "Send Beginning of Route Refresh";
        input {
            leaf peer-selector { type zt:peer-selector; mandatory true; description "Target peers"; }
            leaf family { type zt:address-family; mandatory true; description "Address family"; }
        }
    }

    rpc peer-eorr {
        description "Send End of Route Refresh";
        input {
            leaf peer-selector { type zt:peer-selector; mandatory true; description "Target peers"; }
            leaf family { type zt:address-family; mandatory true; description "Address family"; }
        }
    }

    // Raw message

    rpc peer-raw {
        description "Send raw bytes to peer (no validation)";
        input {
            leaf peer-selector { type zt:peer-selector; mandatory true; description "Target peers"; }
            leaf type { type string; description "Message type hint"; }
            leaf encoding { type zt:encoding-mode; mandatory true; description "Data encoding"; }
            leaf data { type string; mandatory true; description "Raw data"; }
        }
    }

    // Cache operations

    rpc cache {
        description "BGP message cache operations";
        input {
            leaf action { type string; mandatory true; description "Cache action (list|retain|release|expire|forward)"; }
            leaf message-id { type uint64; description "Cache message ID"; }
            leaf peer-selector { type zt:peer-selector; description "Target peers (for forward)"; }
        }
    }

    // Commit operations

    rpc commit {
        description "Named commit operations";
        input {
            leaf name { type string; description "Commit name"; }
        }
    }

    // Subscription

    rpc subscribe {
        description "Subscribe to BGP events (streaming)";
        input {
            leaf-list events { type string; description "Event types to subscribe to"; }
        }
    }

    rpc unsubscribe {
        description "Unsubscribe from BGP events";
    }

    rpc event-list {
        description "List available event types";
        output {
            list event {
                uses zt:event-type-info;
            }
        }
    }

    // Notifications

    notification peer-state-change {
        description "Peer FSM state changed";
        leaf address { type zt:ip-address; description "Peer address"; }
        leaf old-state { type string; description "Previous FSM state"; }
        leaf new-state { type string; description "New FSM state"; }
    }

    notification route-received {
        description "BGP UPDATE received from peer";
        leaf peer { type zt:ip-address; description "Source peer"; }
        leaf family { type zt:address-family; description "Address family"; }
        leaf count { type uint32; description "Number of NLRI in update"; }
    }

    notification route-update-sent {
        description "BGP UPDATE sent to peer";
        leaf peer { type zt:ip-address; description "Target peer"; }
        leaf family { type zt:address-family; description "Address family"; }
    }

    notification eor-received {
        description "End-of-RIB marker received";
        leaf peer { type zt:ip-address; description "Source peer"; }
        leaf family { type zt:address-family; description "Address family"; }
    }

    notification graceful-restart-state {
        description "Graceful restart state changed";
        leaf peer { type zt:ip-address; description "Peer address"; }
        leaf state { type string; description "GR state"; }
    }

    notification session-established {
        description "BGP session established";
        leaf peer { type zt:ip-address; description "Peer address"; }
        leaf asn { type zt:asn; description "Peer ASN"; }
    }

    notification session-closed {
        description "BGP session closed";
        leaf peer { type zt:ip-address; description "Peer address"; }
        leaf reason { type string; description "Close reason"; }
    }
}
```

`ze-bgp-conf.yang`

```yang
module ze-bgp-conf {
    namespace "urn:ze:bgp:conf";
    prefix bgp;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }
    import ze-hub-conf { prefix hub; }

    description "BGP configuration for ZeBGP";

    revision 2026-01-01 {
        description "Initial revision";
    }

    // BGP block - main BGP configuration
    container bgp {
        description "Border Gateway Protocol routing configuration.
                     Peers inherit from group defaults; groups inherit from this global level.";

        // Global BGP related tools (workbench V2). Spec D7/D12.
        // Command paths target the registered command tree: `show bgp-health`
        // is one container under show; `show warnings` and `show errors` are
        // cross-subsystem report bus snapshots.
        ze:related 'id=bgp-health; label="BGP Health"; command="show bgp-health"; placement=global; presentation=modal; class=inspect';
        ze:related 'id=bgp-warnings; label="BGP Warnings"; command="show warnings"; placement=global; presentation=modal; class=diagnose';
        ze:related 'id=bgp-errors; label="BGP Errors"; command="show errors"; placement=global; presentation=modal; class=diagnose';

        leaf router-id {
            type zt:ipv4-address;
            mandatory true;
            description "BGP Router ID (required)";
        }

        container multipath {
            description "BGP multipath / ECMP configuration";

            leaf maximum-paths {
                type uint16 {
                    range "1..256";
                }
                default 1;
                description "Maximum number of equal-cost paths to install per prefix.
                             1 means single best path (default, RFC 4271 Section 9.1.2).
                             Values > 1 enable ECMP with N-way load balancing.";
            }

            leaf relax-as-path {
                type boolean;
                default false;
                description "Allow paths with different AS-paths to be considered equal-cost.
                             When false, multipath requires identical AS-path length and content.
                             When true, only AS-path length must match (not content).
                             Equivalent to 'bgp bestpath as-path multipath-relax' on other vendors.";
            }
        }

        container admin-distance {
            description "Classical admin distance stamped on BGP best-paths when
                         they are mirrored into the shared Loc-RIB. Lower distance
                         wins against other protocols for the same prefix.
                         Defaults follow the Cisco/Juniper convention; RFC 4271
                         does not mandate values.";

            leaf ebgp {
                type uint8 {
                    range "1..255";
                }
                default 20;
                description "Admin distance for routes learned from external BGP peers.";
            }

            leaf ibgp {
                type uint8 {
                    range "1..255";
                }
                default 200;
                description "Admin distance for routes learned from internal BGP peers.";
            }
        }

        container session {
            description "Global BGP session defaults";
            container asn {
                description "AS number configuration";
                leaf local {
                    type zt:asn;
                    mandatory true;
                    ze:decorate "asn-name";
                    description "Local Autonomous System Number (required)";
                }
            }
        }

        // Named filter policy definitions. Filter type plugins augment this
        // container with their own lists marked ze:filter. Each list entry
        // is a named filter instance referenced in peer filter { import/export }
        // chains by its unique name.
        container policy {
            description
                "Named filter definitions for the route policy framework.
                 Each filter type is a list added by its plugin via augment.
                 Filter instances are referenced by name in peer filter chains.";
        }

        // Global filter chains (apply to all peers as base chain)
        container filter {
            description
                "Global route filter chains for import and export.
                 Group and peer levels append to this base chain.
                 Names reference filter instances defined in bgp/policy.";
            leaf-list import {
                type string;
                description "Global import filter chain";
            }
            leaf-list export {
                type string;
                description "Global export filter chain";
            }
        }

        // Global update blocks - default routes applied to all peers.
        // Group and peer levels accumulate additional routes.
        uses update-block;

        // Peer groups - Junos-style: peers nested inside named groups
        list group {
            key "name";
            description "Peer group - defines shared defaults for member peers";

            leaf name {
                type string;
                description "Group name (indicative, for readability)";
            }

            uses peer-fields;

            list peer {
                key "name";
                unique "connection/remote/ip";
                ze:required "connection/remote/ip";
                ze:required "session/asn/local";
                ze:required "session/asn/remote";
                ze:suggest "connection/local/ip";
                description "BGP peer in this group";

                // Same workbench tools as standalone bgp/peer, but the
                // selector walks to the parent group's remote IP when the
                // peer omits its own (Spec D7).
                ze:related 'id=peer-detail; label="Peer Detail"; command="peer ${path-inherit:connection/remote/ip|key} detail"; placement=row; presentation=drawer; class=inspect';
                ze:related 'id=peer-capabilities; label="Capabilities"; command="peer ${path-inherit:connection/remote/ip|key} capabilities"; placement=row; presentation=modal; class=inspect';
                ze:related 'id=peer-statistics; label="Statistics"; command="peer ${path-inherit:connection/remote/ip|key} statistics"; placement=row; presentation=modal; class=inspect';
                ze:related 'id=peer-flush; label="Flush"; command="request peer ${path-inherit:connection/remote/ip|key} flush"; placement=row; presentation=modal; class=refresh';
                ze:related 'id=peer-teardown; label="Teardown"; command="request peer ${path-inherit:connection/remote/ip|key} teardown"; placement=row; presentation=modal; confirm="Tear down BGP session?"; class=danger';

                leaf name {
                    type string {
                        pattern '[a-zA-Z0-9_][a-zA-Z0-9_.\-]*';
                    }
                    description "Peer name (alphanumeric/underscore start, then alphanumeric/underscore/hyphen/dot)";
                }

                uses peer-fields;
            }
        }

        // Standalone peers (not in a group)
        list peer {
            key "name";
            unique "connection/remote/ip";
            ze:required "connection/remote/ip";
            ze:required "session/asn/local";
            ze:required "session/asn/remote";
            ze:suggest "connection/local/ip";
            description "BGP peer configuration (standalone, no group)";

            // Per-row workbench tools (spec Day-One BGP Related Tools).
            // The dispatcher extracts the `peer <selector>` pair and dispatches
            // the remainder against the registered command tree, so commands
            // are written in the form `peer <selector> <verb>` to match
            // `peer/detail`, `peer/capabilities`, etc. from ze-peer-cmd.yang.
            // Selector resolves the peer's remote IP, falling back to the
            // peer's list key when the IP is unset.
            ze:related 'id=peer-detail; label="Peer Detail"; command="peer ${path:connection/remote/ip|key} detail"; placement=row; presentation=drawer; class=inspect';
            ze:related 'id=peer-capabilities; label="Capabilities"; command="peer ${path:connection/remote/ip|key} capabilities"; placement=row; presentation=modal; class=inspect';
            ze:related 'id=peer-statistics; label="Statistics"; command="peer ${path:connection/remote/ip|key} statistics"; placement=row; presentation=modal; class=inspect';
            ze:related 'id=peer-flush; label="Flush"; command="request peer ${path:connection/remote/ip|key} flush"; placement=row; presentation=modal; class=refresh';
            ze:related 'id=peer-teardown; label="Teardown"; command="request peer ${path:connection/remote/ip|key} teardown"; placement=row; presentation=modal; confirm="Tear down BGP session?"; class=danger';

            leaf name {
                type string {
                    pattern '[a-zA-Z_][a-zA-Z0-9_.\-]*';
                }
                description "Peer name (must start with a letter, not a digit)";
            }

            uses peer-fields;
        }
    }

    // Update block grouping - reused at bgp, group, and peer levels.
    grouping update-block {
        // Native update syntax - Ze-native route announcements
        // update { attribute { origin igp; next-hop 10.0.0.1; } nlri { ipv4/unicast 1.0.0.0/24; } }
        list update {
            description "Native Ze route announcements";

            leaf name {
                type string {
                    pattern '[a-zA-Z0-9_][a-zA-Z0-9_.\-]*';
                }
                ze:display-key;
                description "Optional label for this update block (display only)";
            }

            container attribute {
                description "Path attributes for routes in this block";
                leaf origin {
                    type enumeration {
                        enum igp { description "Interior Gateway Protocol"; }
                        enum egp { description "Exterior Gateway Protocol"; }
                        enum incomplete { description "Incomplete origin"; }
                    }
                    description "ORIGIN attribute (RFC 4271 Section 4.3)";
                }
                leaf next-hop {
                    type union {
                        type zt:ip-address;
                        type enumeration {
                            enum self;
                        }
                    }
                    description "Next hop address or 'self'";
                }
                leaf med { type uint32; description "Multi-Exit Discriminator (MED). Lower values are preferred. Used to influence inbound path selection by an external peer (RFC 4271 Section 5.1.4)."; }
                leaf local-preference { type uint32; description "LOCAL_PREF value for iBGP best-path selection. Higher wins. Only meaningful within a single AS (RFC 4271 Section 5.1.5). Default: 100."; }
                leaf-list as-path { type string; description "AS_PATH segments prepended to the route. Space-separated ASNs. Affects loop detection and path selection (shorter preferred)."; }
                leaf-list community { type string; description "Standard BGP communities (RFC 1997). Format: AS:value (e.g., 65000:100) or well-known names (no-export, no-advertise)."; }
                leaf-list large-community { type string; description "Large BGP communities (RFC 8092). Format: global:local1:local2. Supports 4-byte ASNs natively."; }
                leaf-list extended-community { type string; description "Extended communities (RFC 4360). Used for route targets, site-of-origin, and VPN semantics. Format: type:admin:value."; }
                leaf label { type string; description "MPLS label value for labeled unicast routes (RFC 3107). Numeric or keyword."; }
                leaf-list labels { type string; description "MPLS label stack for multi-label routes (RFC 8277). Ordered bottom-to-top."; }
                leaf path-information { type string; description "ADD-PATH path identifier (RFC 7911). Distinguishes multiple paths for the same prefix from the same peer."; }
                leaf rd { type zt:route-distinguisher; description "Route Distinguisher (RFC 4364). Prepended to the prefix to create a unique VPN route. Format: ASN:nn or IP:nn."; }
                leaf aggregator { type string; description "AGGREGATOR attribute (RFC 4271 Section 5.1.7). Format: AS:IP. Identifies the AS and router that formed the aggregate."; }
                leaf atomic-aggregate { type boolean; description "ATOMIC_AGGREGATE flag (RFC 4271 Section 5.1.6). Signals that the route was formed by aggregation and AS_PATH information was lost."; }
                leaf originator-id { type zt:ipv4-address; description "ORIGINATOR_ID for route reflection (RFC 4456). Set by the reflector to the originating router's ID. Used for loop prevention."; }
                leaf-list cluster-list { type zt:ipv4-address; description "CLUSTER_LIST for route reflection (RFC 4456). Each reflector prepends its cluster ID. Used for loop detection between reflectors."; }
                leaf-list attribute { type string; description "Raw BGP attributes in hex encoding. For attributes not covered by named fields. Format: flags:type:hex-value."; }
                container bgp-prefix-sid { presence "Prefix-SID enabled"; description "Prefix-SID attribute (RFC 8669). Carries an MPLS label index for Segment Routing, allowing a node to advertise its SID without per-prefix label allocation."; }
                container bgp-prefix-sid-srv6 { presence "SRv6 Prefix-SID enabled"; description "SRv6 Prefix-SID attribute (RFC 9252). Carries SRv6 SID information for IPv6 Segment Routing, including SRv6 L3 Service TLVs."; }
                leaf split { type string; description "Split a single prefix into multiple more-specifics for announcement. Format: /length (e.g., /24 splits a /16 into 256 /24s)."; }
            }

            list nlri {
                key "name";
                leaf name { type zt:address-family; description "Address family (e.g., ipv4/unicast)"; }
                leaf content { type string; description "Operation, qualifiers, and payload"; }
            }

            container watchdog {
                description "Watchdog-controlled route - held until 'bgp watchdog announce <name>'";
                leaf name { type string; description "Watchdog group name"; }
                leaf withdraw { type boolean; description "Start in withdrawn state (default true)"; }
            }
        }
    }

    // Shared fields for both groups and peers (no IP addresses).
    grouping peer-fields {
        description "Common peer/group configuration fields";

        // Connection container - transport-level settings
        container connection {
            description "Transport-level connection settings";

            container local {
                description "Local endpoint for the TCP session. Controls the bind address, port,
                             and whether to accept inbound connections.";
                leaf ip {
                    type union {
                        type zt:ip-address;
                        type enumeration {
                            enum auto;
                        }
                    }
                    description "Local address for connection (use IP address or 'auto')";
                }
                leaf port {
                    type zt:port;
                    description "Local bind port";
                }
                leaf accept {
                    type boolean;
                    default true;
                    description "Accept inbound TCP connections at this local endpoint (RFC 4271 Section 8.1.1)";
                }
            }

            container remote {
                description "Remote endpoint for the TCP session. Controls the peer address, port,
                             outbound connection initiation, and dynamic peer ranges.";
                leaf ip {
                    type union {
                        type zt:ip-address;
                        type enumeration {
                            enum dynamic {
                                description "Accept BGP sessions from any IP in the group's range.
                                             Valid at group level only; rejected at peer level.";
                            }
                        }
                    }
                    description "Peer IP address or 'dynamic' for dynamic peer groups";
                }
                leaf port {
                    type zt:port;
                    description "Remote connection port";
                }
                leaf connect {
                    type boolean;
                    default true;
                    description "Initiate outbound TCP connections to this remote endpoint (RFC 4271 Section 8.1.1)";
                }
                leaf-list range {
                    type zt:ip-prefix;
                    description "IP prefix ranges for dynamic peer groups.
                                 Only meaningful when ip is 'dynamic'.
                                 Connections from IPs within these ranges create dynamic peers.";
                }
                leaf max-peers {
                    type uint32 {
                        range "1..100000";
                    }
                    default 1000;
                    description "Maximum number of dynamic peers for this group";
                }
            }

            container md5 {
                description "TCP MD5 authentication (RFC 2385)";
                leaf password {
                    type string;
                    ze:sensitive;
                    description "MD5 authentication password";
                }
                leaf ip {
                    type zt:ip-address;
                    description "MD5 authentication IP";
                }
            }

            container ttl {
                description "TTL settings for BGP sessions";
                leaf max {
                    type uint8;
                    description "TTL security / GTSM (RFC 5082)";
                }
                leaf set {
                    type uint8;
                    description "Outgoing TTL value";
                }
                leaf min {
                    type uint8;
                    description "Minimum incoming TTL";
                }
            }

            leaf link-local {
                type boolean;
                description "Auto-discover IPv6 link-local address for TCP connection";
            }

            container bfd {
                presence "Enable BFD liveness detection for this peer";
                description
                    "Bidirectional Forwarding Detection (RFC 5880) options
                     for this peer. When the container is present, the BGP
                     reactor calls the BFD plugin's Service interface on
                     session establishment and tears the BGP session down
                     when BFD reports Down. The BFD plugin must be loaded
                     (a top-level bfd { ... } block); if it is not, the
                     BGP peer starts without BFD and logs a warning.";

                leaf enabled {
                    type boolean;
                    default true;
                    description
                        "Master switch for this peer's BFD session. Set
                         false to keep the config in place but suspend
                         the BFD client (useful for maintenance).";
                }

                leaf mode {
                    type enumeration {
                        enum single-hop {
                            description "RFC 5881 single-hop BFD on UDP 3784 with GTSM.";
                        }
                        enum multi-hop {
                            description "RFC 5883 multi-hop BFD on UDP 4784 with min-TTL.";
                        }
                    }
                    default single-hop;
                    description
                        "Hop mode. Single-hop is the common case for an
                         eBGP peer on a direct link; multi-hop is for
                         iBGP over an IGP or any peering that crosses
                         more than one IP hop.";
                }

                leaf profile {
                    type string;
                    description
                        "Name of a profile defined under the top-level
                         bfd { profile ... } block. The referenced
                         profile supplies detect-multiplier,
                         desired-min-tx-us, and required-min-rx-us.
                         Empty means use the BFD plugin defaults.";
                }

                leaf min-ttl {
                    type uint8;
                    description
                        "Multi-hop minimum acceptable TTL (RFC 5883 §5).
                         Ignored for single-hop. Zero means use the
                         plugin default (254).";
                }

                leaf interface {
                    type string;
                    description
                        "Single-hop egress interface. Optional: when
                         omitted, the BFD plugin derives the interface
                         from the peer's local address. Ignored for
                         multi-hop.";
                }
            }
        }

        // Session container - BGP session settings
        container session {
            description "BGP session parameters: ASN, capabilities, address families, next-hop policy,
                         and community control. Inherited from group to peer; peer values override.";

            container asn {
                description "AS number configuration";
                leaf local {
                    type zt:asn;
                    ze:decorate "asn-name";
                    description "Local AS (overrides global)";
                }
                leaf remote {
                    type zt:asn;
                    ze:decorate "asn-name";
                    description "Peer Autonomous System Number";
                }
                leaf-list local-options {
                    type enumeration {
                        enum no-prepend {
                            description "Do not prepend real ASN before local-as in AS_PATH";
                        }
                        enum replace-as {
                            description "Replace real ASN entirely with local-as in AS_PATH";
                        }
                    }
                    description "Modifiers for local-as behavior.
                                 no-prepend: real ASN not prepended before local-as.
                                 replace-as: local-as replaces real ASN entirely.
                                 Both together: full replacement with no prepend.";
                }
            }

            leaf as-override {
                type boolean;
                default false;
                description "Replace peer's ASN with local ASN in outbound AS_PATH.
                             Used in VPN/multi-site where the same customer ASN appears at multiple sites.";
            }

            leaf accept-srv6-prefix-sid {
                type boolean;
                default false;
                description "Accept BGP Prefix-SID attribute (code 40) with SRv6 TLVs from this EBGP peer.
                             RFC 8669 Section 4: PrefixSID from EBGP outside the SR domain MUST be
                             discarded unless configured to accept. Has no effect on IBGP (always accepted).";
            }

            container community {
                description "Community attribute control for this session";
                leaf-list send {
                    type enumeration {
                        enum standard {
                            description "Send standard communities (type 8, RFC 1997)";
                        }
                        enum large {
                            description "Send large communities (type 32, RFC 8092)";
                        }
                        enum extended {
                            description "Send extended communities (type 16, RFC 4360)";
                        }
                        enum all {
                            description "Send all community types (default)";
                        }
                        enum none {
                            description "Suppress all community attributes";
                        }
                    }
                    description "Community types to include in outbound UPDATEs.
                                 Default is all (send every community type).
                                 Specify individual types for granular control.
                                 none suppresses all community attributes.";
                }
            }

            leaf router-id {
                type zt:ipv4-address;
                description "Override router ID for this peer";
            }

            leaf rs-client {
                type boolean;
                default false;
                description "Mark this peer as an RS-client for transparent AS-path forwarding.
                             RFC 7947 Section 2.2.2: the route server MUST NOT modify AS_PATH
                             or any other transitive attribute. When true, the reactor skips
                             AS-path prepending for this peer on the forwarding path.";
            }

            leaf route-reflector-client {
                type boolean;
                default false;
                description "Mark this peer as a route reflector client (RFC 4456).
                             Routes from clients are forwarded to all clients and non-clients.
                             Routes from non-clients are forwarded to clients only.";
            }

            leaf cluster-id {
                type zt:ipv4-address;
                description "Override cluster ID for route reflection (RFC 4456 Section 7).
                             Defaults to router-id when not set.
                             Prepended to CLUSTER_LIST on reflected routes.";
            }

            leaf next-hop {
                type union {
                    type zt:ip-address;
                    type enumeration {
                        enum self {
                            description "Rewrite next-hop to local address (common for iBGP)";
                        }
                        enum unchanged {
                            description "Never rewrite next-hop (route-server, third-party NH)";
                        }
                        enum auto {
                            description "RFC 4271 default: rewrite for eBGP, preserve for iBGP";
                        }
                    }
                }
                default "auto";
                description "Next-hop rewriting policy for forwarded UPDATEs (RFC 4271 Section 5.1.3).
                             auto: rewrite for eBGP peers, preserve for iBGP peers.
                             self: always rewrite to local address.
                             unchanged: never rewrite (preserves original next-hop).
                             IP address: set next-hop to explicit address.";
            }

            leaf link-local {
                type zt:ipv6-address;
                description "IPv6 link-local address for next-hop (RFC 2545 Section 3)";
            }

            // Address families
            list family {
                key "name";
                description "Address families to negotiate";
                leaf name {
                    type zt:address-family;
                    ze:validate "registered-address-family";
                    description "AFI/SAFI (e.g. ipv4/unicast)";
                }
                leaf mode {
                    type enumeration {
                        enum enable { description "Enable this address family"; }
                        enum disable { description "Disable this address family"; }
                        enum require { description "Require this address family"; }
                        enum ignore { description "Ignore this address family"; }
                    }
                    description "Address family negotiation mode";
                }

                // Per-family prefix limits (RFC 4486)
                container prefix {
                    description "Prefix limit configuration for this address family";
                    leaf maximum {
                        type uint32 {
                            range "1..max";
                        }
                        description "Hard maximum number of prefixes accepted";
                    }
                    leaf warning {
                        type uint32;
                        description "Warning threshold. Defaults to 90% of maximum when not set.";
                    }
                    leaf teardown {
                        type boolean;
                        default true;
                        description "Tear down session when prefix maximum exceeded (false = warn only)";
                    }
                    leaf idle-timeout {
                        type uint16;
                        default 0;
                        description "Seconds before auto-reconnect after prefix teardown (0 = no reconnect)";
                    }
                    leaf updated {
                        type string;
                        ze:hidden true;
                        description "ISO date (YYYY-MM-DD) when prefix maximum was last updated from PeeringDB. Hidden leaf.";
                    }
                }

                leaf default-originate {
                    type boolean;
                    default false;
                    description "Originate the default route (0.0.0.0/0 for IPv4, ::/0 for IPv6)
                                 to this peer for this address family.";
                }

                leaf default-originate-filter {
                    type string;
                    description "Named filter that must accept for the default route to be originated.
                                 If the filter rejects, the default route is not sent.
                                 Empty or absent means always originate (unconditional).";
                }
            }

            // Capabilities
            container capability {
                description "BGP capability negotiation";

                leaf asn4 {
                    type boolean;
                    default true;
                    description "Advertise 4-byte ASN capability (RFC 6793). Required for ASNs above 65535.
                                 Disable only for peers running legacy 2-byte-only implementations.";
                }

                container route-refresh {
                    presence "Route Refresh capability enabled";
                    description "Route Refresh capability (RFC 2918). Allows the peer to request a full
                                 re-advertisement of routes without tearing down the session.";
                }

                // NOTE: graceful-restart moved to GR plugin (ze-graceful-restart.yang)

                container add-path {
                    presence "ADD-PATH capability enabled";
                    description "ADD-PATH capability (RFC 7911) with optional PATHS-LIMIT
                                 (draft-abraitis-idr-addpath-paths-limit). Default direction applies
                                 to all negotiated multiprotocol families. Per-family entries override.";
                    leaf direction {
                        type enumeration {
                            enum send { description "Send multiple paths"; }
                            enum receive { description "Receive multiple paths"; }
                            enum send/receive { description "Both send and receive multiple paths"; }
                        }
                        description "Default ADD-PATH direction for all negotiated families.";
                    }
                    leaf limit {
                        type uint16 { range "1..65535"; }
                        description "Default maximum paths per prefix (PATHS-LIMIT). Inherited by all
                                     families unless overridden per-family.";
                    }
                    list family {
                        key "name";
                        description "Per-family ADD-PATH overrides with optional path count limit.";
                        leaf name { type zt:address-family; description "Address family (e.g. ipv4/unicast)"; }
                        leaf direction {
                            type enumeration {
                                enum send { description "Send ADD-PATH"; }
                                enum receive { description "Receive ADD-PATH"; }
                                enum send/receive { description "Both send and receive ADD-PATH"; }
                            }
                            description "ADD-PATH direction override for this family.";
                        }
                        leaf limit {
                            type uint16 { range "1..65535"; }
                            description "Maximum paths per prefix to receive (PATHS-LIMIT capability).";
                        }
                        leaf mode {
                            type enumeration {
                                enum enable { description "Enable ADD-PATH for this family"; }
                                enum disable { description "Disable ADD-PATH for this family"; }
                                enum require { description "Require ADD-PATH for this family"; }
                                enum refuse { description "Refuse ADD-PATH for this family"; }
                            }
                            description "ADD-PATH negotiation mode for this family.";
                        }
                    }
                }

                container extended-message {
                    presence "Extended Message capability enabled";
                    description "Extended Message capability (RFC 8654). Raises the BGP message size limit
                                 from 4096 to 65535 bytes. Required for large UPDATE messages with many attributes.";
                }

                list nexthop {
                    key "family";
                    description "Extended Next Hop capability (RFC 8950)";
                    leaf family { type zt:address-family; description "NLRI family (e.g. ipv4/unicast)"; }
                    leaf nhafi { type zt:afi; description "Next-hop AFI (e.g. ipv6)"; }
                    leaf mode {
                        type enumeration {
                            enum enable { description "Enable extended next hop"; }
                            enum disable { description "Disable extended next hop"; }
                            enum require { description "Require extended next hop"; }
                            enum refuse { description "Refuse extended next hop"; }
                        }
                        description "Extended next hop negotiation mode";
                    }
                }
            }

        }

        // Behaviour container - operational knobs
        container behavior {
            description "Operational knobs that control how the reactor processes and forwards
                         UPDATE messages for this peer. Most users can leave these at defaults.";

            leaf group-updates {
                type boolean;
                default true;
                description "Pack multiple NLRI into a single UPDATE message when they share the same
                             path attributes. Reduces the number of UPDATE messages from O(routes) to
                             O(unique-attribute-sets). Disable for peers that require one prefix per UPDATE.";
            }

            leaf manual-eor {
                type boolean;
                description "Do not send End-of-RIB automatically after initial route advertisement.
                             When enabled, End-of-RIB must be triggered externally via the process API.
                             Used when an external controller manages convergence signaling.";
            }

            leaf auto-flush {
                type boolean;
                description "Automatically withdraw all routes advertised to this peer when the session
                             goes down. When disabled, routes remain until explicitly withdrawn or the
                             hold timer expires. Legacy ExaBGP option, retained for migration.";
            }

            leaf rs-fast-path {
                type boolean;
                default false;
                description "Forward received UPDATEs directly inside the reactor for RS-client peers,
                             bypassing the plugin dispatch chain. Lower latency for route server
                             forwarding. Peers with export filters are excluded and use the normal path.";
            }
        }

        // Basic peer identification
        leaf description {
            type string;
            description "Free-text label for this peer. Shown in the CLI, web UI, and logs.
                         Typically the peer's role or location (e.g., 'upstream-transit-provider').";
        }

        container timer {
            description "BGP session timers: hold time, keepalive interval, and connect retry delay.
                         Hold time is proposed in OPEN and negotiated to the lower of both peers' values.";

            leaf receive-hold-time {
                type uint16 {
                    range "0 | 3..65535";
                }
                default 90;
                description "Receive hold time in seconds (RFC 4271: 0 or >= 3). Proposed in OPEN.";
            }

            leaf send-hold-time {
                type uint16 {
                    range "0 | 480..65535";
                }
                default 0;
                description "Send hold time in seconds (RFC 9687). 0 = auto: max(480, 2x receive-hold-time).";
            }

            leaf keepalive {
                type uint16;
                default 0;
                description "Keepalive interval in seconds (RFC 4271 Section 10). 0 = auto: hold-time/3.";
            }

            leaf connect-retry {
                type uint16;
                default 120;
                description "Connect retry interval in seconds (RFC 4271 Section 8)";
            }
        }

        uses update-block;

        // Process bindings
        list process {
            key "name";
            description "External process that receives BGP events and can inject messages.
                         Ze spawns the process and communicates via stdin/stdout JSON.";

            leaf name {
                type string;
                description "Unique identifier for this process binding. Used in logs and status output.";
            }

            leaf run {
                type string;
                description "Shell command to spawn the external process. Ze pipes BGP events to its
                             stdin and reads commands from its stdout.";
            }

            // Old syntax fields
            leaf-list processes {
                type string;
                description "Legacy ExaBGP process reference list. Use 'receive' and 'send' instead.";
            }

            leaf-list processes-match {
                type string;
                description "Legacy ExaBGP process match patterns for event filtering.";
            }

            container neighbor-changes {
                presence "Neighbor change notifications enabled";
                description "Send session state change events (up/down/reset) to this process.
                             Enables the process to react to peer lifecycle transitions.";
            }

            // New syntax fields
            container content {
                description "Controls the encoding and filtering of events sent to this process.";
                leaf encoding { type string; description "Wire encoding for events sent to the process (e.g., json, text)."; }
                leaf format { type string; description "Output format template for event rendering."; }
                leaf attribute { type string; description "Filter expression to select which BGP attributes are included in events."; }
            }

            leaf-list receive {
                type string;
                ze:validate "receive-event-type";
                description "Event types to receive. Base types: update, open,
                    notification, keepalive, refresh, state, sent, negotiated.
                    Plugins may register additional types (e.g., update-rpki).
                    List types explicitly; 'all' is not accepted.
                    Validated at runtime against registered event types.";
            }

            leaf-list send {
                type string;
                ze:validate "send-message-type";
                description "Message types to send. Valid: update, refresh,
                    enhanced-refresh (BORR/EORR markers, RFC 7313).
                    Validated at runtime against known send types.";
            }
        }

        // Filter chains
        container filter {
            description
                "Route filter chains for import and export.
                 Names reference filter instances defined in bgp/policy.";
            leaf-list import {
                type string;
                description "Import filter chain (filter instance names)";
            }
            leaf-list export {
                type string;
                description "Export filter chain (filter instance names)";
            }
        }

        // RIB configuration
        container rib {
            description "Route Information Base settings for this peer. Controls which RIB tables
                         are maintained and how outbound route batching works.";
            container adj {
                description "Adjacency RIB storage. These tables hold the raw routes before and after
                             policy processing, enabling route-refresh and soft-reconfiguration.";
                leaf in {
                    type boolean;
                    description "Store received routes in Adj-RIB-In before policy. Enables soft
                                 reconfiguration inbound (re-apply import filters without session reset).
                                 Uses additional memory proportional to received route count.";
                }
                leaf out {
                    type boolean;
                    description "Store advertised routes in Adj-RIB-Out after policy. Required for
                                 route-refresh (RFC 2918) to re-send routes on peer request.
                                 Uses additional memory proportional to advertised route count.";
                }
            }
            container out {
                description "Outbound RIB batching settings. Control how route changes are grouped
                             and flushed to the peer.";
                leaf group-updates {
                    type boolean;
                    default true;
                    description "Pack routes with identical attributes into a single UPDATE. Same as
                                 the behavior-level group-updates but scoped to this peer's Adj-RIB-Out.";
                }
                leaf auto-commit-delay {
                    type uint32;
                    default 0;
                    description "Milliseconds to wait before flushing pending route changes to the peer.
                                 Allows batching rapid successive changes into fewer UPDATEs.
                                 0 means flush immediately on each change.";
                }
                leaf max-batch-size {
                    type int32;
                    default 0;
                    description "Maximum number of route changes to include in a single flush cycle.
                                 0 means no limit (flush all pending changes at once).
                                 Limits per-cycle CPU cost at the expense of convergence latency.";
                }
            }
        }

    }

    // BGP-owned environment settings — augment hub's environment container
    augment "/hub:environment" {
        container bgp {
            description "BGP protocol environment settings: global defaults that apply before
                         any peer-level config. Controls OPEN wait timeout and initial announce delay.";
            leaf openwait {
                type int32 { range "1..3600"; }
                default "120";
                description "Seconds to wait for peer OPEN after TCP connect";
            }
            leaf announce-delay {
                type string;
                default "0s";
                description "Delay between reactor Ready and first UPDATE (duration, 0s-1h)";
            }
        }

        container reactor {
            description "BGP reactor engine tuning. The reactor is the core event loop that processes
                         incoming messages, runs peer FSMs, and dispatches outbound UPDATEs.";
            leaf speed { type string; default "1.0"; description "Reactor loop cycle time multiplier. Values below 1.0 run faster (lower latency, higher CPU). Values above 1.0 run slower (higher latency, lower CPU). Range: 0.1 to 10.0."; }
            leaf cache-ttl { type uint32; default 60; description "Seconds to keep recently-built UPDATE wire messages in the encoding cache. Cached UPDATEs are reused for peers with the same encoding context, avoiding redundant serialization. 0 disables caching (every UPDATE is built fresh)."; }
            leaf cache-max { type uint32; default 1000000; description "Maximum number of cached UPDATE wire messages. Bounds memory usage of the encoding cache. 0 means unlimited (cache grows with the route table)."; }
            leaf update-groups { type boolean; default true; description "Build each UPDATE once and send to all peers with the same encoding context (same capabilities, same address families). Reduces CPU from O(peers x routes) to O(groups x routes). Disable only for debugging."; }
            leaf forward-queue-size { type uint32 { range "1..1000000"; } default 256; description "Per-destination forward channel capacity. Controls how many UPDATE items can be buffered per destination peer before backpressure kicks in."; }
            leaf forward-batch-limit { type uint32 { range "0..1000000"; } default 1024; description "Max items per drain batch. Bounds writeMu hold time during forward dispatch. 0 means unlimited."; }
            leaf forward-pool-max-bytes { type uint32; default 0; description "Combined byte budget for 4K+64K buffer pools in bytes (max ~4GB). 0 means unlimited (auto-sized from peer prefix maximums)."; }
            leaf forward-pool-headroom { type uint32; default 0; description "Extra bytes beyond auto-sized pool baseline (max ~4GB). Ignored when forward-pool-max-bytes is explicitly set."; }
            leaf forward-teardown-grace { type string; default "5s"; description "Grace period before forced teardown on congestion (duration string, e.g. 5s, 1m)."; }
            leaf read-buffer-size { type uint32 { range "4096..16777216"; } default 65536; description "Per-session TCP read buffer size in bytes."; }
            leaf write-buffer-size { type uint32 { range "4096..16777216"; } default 16384; description "Per-session TCP write buffer size in bytes."; }
        }
    }
}
```

### bgp-adj-rib-in

source: `internal/component/bgp/plugins/adj_rib_in`

Adj-RIB-In storage (raw hex replay)

`ze-adj-rib-in-api.yang`

```yang
module ze-adj-rib-in-api {
  namespace "urn:ze:adj-rib-in";
  prefix adj-rib-in;

  description
    "Adj-RIB-In storage plugin for ze.
     Stores received routes per source peer as raw hex wire bytes
     for efficient replay via update hex commands.

     RFC 4271 Section 3.2: Adj-RIBs-In contains unprocessed routing
     information advertised to the local BGP speaker by its peers.";

  revision 2025-01-01 {
    description "Initial revision";
  }
}
```

### bgp-aigp

source: `internal/component/bgp/plugins/aigp`

Accumulated IGP Metric (RFC 7311)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-bmp

source: `internal/component/bgp/plugins/bmp` -- config root: `bgp, environment`

BMP receiver and sender (RFC 7854, 8671)

`ze-bmp-cmd.yang`

```yang
module ze-bmp-cmd {
    namespace "urn:ze:bmp:cmd";
    prefix bmpcmd;
    import ze-extensions { prefix ze; }
    description "show bmp ... command tree. Owned by the bgp-bmp plugin so that removing the BMP surface removes these command nodes together with the handlers and config schema. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated show bmp ... out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container bmp {
            config false;
            description "BMP monitoring sessions, peers, and routes";

            container sessions {
                config false;
                ze:command "ze-show:bmp-sessions";
                description "Show active BMP receiver sessions.
Lists each session with connection state and message counters.
Check here to confirm your BMP collector is receiving data.";
            }

            container peers {
                config false;
                ze:command "ze-show:bmp-peers";
                description "Show BGP peers as seen through BMP monitoring.
Lists peers reported via BMP with their state and route statistics.";
            }

            container collectors {
                config false;
                ze:command "ze-show:bmp-collectors";
                description "Show BMP collector connection status.
Lists configured collectors with connection state, sent message
counts, and error statistics. Check here if your collector is
not receiving data.";
            }

            container rib {
                config false;
                ze:command "ze-show:bmp-rib";
                description "Show routes received via BMP monitoring sessions.
Returns the BMP RIB content. Use this to verify what your
collector is seeing from remote peers.";
            }
        }
    }
}
```

`ze-bmp-conf.yang`

```yang
module ze-bmp-conf {
    namespace "urn:ze:bmp:conf";
    prefix bmp;

    import ze-bgp-conf { prefix bgp; }
    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description "BMP (BGP Monitoring Protocol, RFC 7854) configuration for Ze.
                 Receiver listener lives under environment (like web, ssh, lg).
                 Sender and protocol options live under bgp.";

    revision 2026-04-12 {
        description "Initial revision";
    }

    // --- Receiver: environment { bmp { ... } } ---

    container environment {
        description "Environment settings for BMP receiver";

        container bmp {
            description "BMP receiver settings";

            leaf enabled {
                type boolean;
                default false;
                description "Enable BMP receiver";
            }

            list server {
                key "name";
                ze:listener;
                description "BMP receiver listen endpoints";

                leaf name {
                    type string;
                    description "Listener instance name";
                }

                uses zt:listener {
                    refine ip {
                        default "0.0.0.0";
                    }
                    refine port {
                        default 11019;
                    }
                }
            }

            leaf max-sessions {
                type uint16 {
                    range "1..1000";
                }
                default 100;
                description "Maximum concurrent BMP sessions";
            }

            leaf route-action {
                type enumeration {
                    enum monitor {
                        description "Store in BMP RIB for visibility (show bmp rib), isolated from BGP best-path and FIB";
                    }
                    enum redistribute {
                        description "Store in BMP RIB and redistribute into BGP best-path selection";
                    }
                }
                default monitor;
                description "Action for received BMP Route Monitoring messages";
            }
        }
    }

    // --- Sender + protocol options: bgp { bmp { ... } } ---

    grouping bmp-bgp-config {
        container bmp {
            description "BMP sender and protocol options";

            container sender {
                presence "BMP sender enabled";
                description "Stream BMP data to external collectors";

                list collector {
                    key "name";
                    description "BMP collector endpoints to connect to";

                    leaf name {
                        type string;
                        description "Collector instance name";
                    }

                    leaf address {
                        type zt:ip-address;
                        mandatory true;
                        description "Collector IP address";
                    }

                    leaf port {
                        type zt:port;
                        default 11019;
                        description "Collector TCP port";
                    }
                }

                leaf route-monitoring-policy {
                    type enumeration {
                        enum pre-policy;
                        enum post-policy;
                        enum all;
                    }
                    default "all";
                    description "Which routes to stream: pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out, RFC 8671), or all";
                }

                leaf route-mirroring {
                    type boolean;
                    default false;
                    description "Enable Route Mirroring (RFC 7854 S4.7): stream verbatim copies of all BGP messages to collectors";
                }

                leaf statistics-timeout {
                    type uint16 {
                        range "0..65535";
                    }
                    default 0;
                    units "seconds";
                    description "Interval for periodic statistics reports (0 = disabled)";
                }
            }
        }
    }

    augment "/bgp:bgp" {
        uses bmp-bgp-config;
    }
}
```

### bgp-capa

source: `internal/component/bgp/plugins/capa`

Core BGP capability decoding (multiprotocol, asn4, add-path, paths-limit, extended-nexthop, extended-message)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-filter-aspath

source: `internal/component/bgp/plugins/filter_aspath` -- config root: `bgp` -- depends on: `bgp`

Named AS-path regex filter (ordered entries, first match wins, accept/reject)

`ze-filter-aspath.yang`

```yang
module ze-filter-aspath {
    namespace "urn:ze:filter-aspath";
    prefix fa;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }

    description "AS-path regex filter type for the BGP policy framework.
                 Named as-path-lists with ordered regex entries (first match wins)
                 and accept/reject actions. The AS-path is converted to a
                 space-separated decimal string (e.g. '65001 65002 65003') and
                 each entry's regex is matched against it using Go RE2 semantics
                 (linear time, no backtracking -- inherently ReDoS-safe).
                 Referenced from peer filter chains as bgp-filter-aspath:NAME.";

    revision 2026-04-12 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list as-path-list {
            ze:filter;
            key "name";
            description "Named AS-path regex filter instance.
                         Each list contains an ordered set of regex entries.
                         Entries are evaluated in order; first match wins.
                         No match = implicit deny (reject).";

            leaf name {
                type string;
                description "Filter instance name (referenced in peer filter chains
                             as bgp-filter-aspath:NAME or as-path-list:NAME).";
            }

            list entry {
                key "regex";
                ordered-by user;
                description "Ordered regex entry. First match wins against the
                             space-separated AS-path string.";

                leaf regex {
                    type string;
                    description "Regular expression matched against the space-separated
                                 decimal AS-path string representation.
                                 Examples: '^65001$' (exact single-AS match),
                                 '^65001 ' (paths starting with AS 65001),
                                 '^$' (empty AS-path, locally originated).
                                 Uses Go RE2 syntax (linear time guarantee).
                                 Maximum length: 512 characters.";
                }

                leaf action {
                    type enumeration {
                        enum accept {
                            description "Accept routes with matching AS-path";
                        }
                        enum reject {
                            description "Reject routes with matching AS-path";
                        }
                    }
                    default accept;
                    description "Action applied when this entry's regex matches.";
                }
            }
        }
    }
}
```

### bgp-filter-aspath-length

source: `internal/component/bgp/plugins/filter_aspath_length` -- config root: `bgp` -- depends on: `bgp`

Named AS-path length filter (accept/reject based on hop count)

`ze-filter-aspath-length.yang`

```yang
module ze-filter-aspath-length {
    namespace "urn:ze:filter-aspath-length";
    prefix fal;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }

    description "AS-path length filter type for the BGP policy framework.
                 Named filters that accept or reject routes based on AS_PATH
                 hop count. Path length follows RFC 4271 Section 9.1.2.2:
                 AS_SEQUENCE entries counted individually, AS_SET counted as 1,
                 confederation segments not counted.
                 Referenced from peer filter chains as as-path-length:NAME.";

    revision 2026-05-25 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list as-path-length {
            ze:filter;
            key "name";
            description "Named AS-path length filter instance.
                         Routes with path length outside the configured
                         min..max range are rejected. Both min and max
                         are optional; omitting one makes that bound open.";

            leaf name {
                type string;
                description "Filter instance name (referenced in peer filter
                             chains as as-path-length:NAME).";
            }

            leaf max {
                type uint16 {
                    range "1..65535";
                }
                description "Maximum allowed AS-path length (inclusive).
                             Routes with longer paths are rejected.";
            }

            leaf min {
                type uint16 {
                    range "0..65535";
                }
                description "Minimum required AS-path length (inclusive).
                             Routes with shorter paths are rejected.";
            }
        }
    }
}
```

### bgp-filter-community

source: `internal/component/bgp/plugins/filter_community` -- config root: `bgp` -- depends on: `bgp`

Community tag/strip filter (standard, large, extended)

`ze-filter-community.yang`

```yang
module ze-filter-community {
    namespace "urn:ze:filter-community";
    prefix fc;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }

    description "Community filter plugin for ZeBGP — tag/strip communities on ingress/egress";

    revision 2026-03-27 {
        description "Initial revision";
    }

    // Named community definitions under bgp { community { } }
    augment "/bgp:bgp" {
        container community {
            description "Named community definitions referenced by filter rules";

            list standard {
                key "name";
                leaf name { type string; description "Community set name"; }
                leaf-list value {
                    type string;
                    description "Standard community values (ASN:value format)";
                }
            }

            list large {
                key "name";
                leaf name { type string; description "Community set name"; }
                leaf-list value {
                    type string;
                    description "Large community values (GA:LD1:LD2 format)";
                }
            }

            list extended {
                key "name";
                leaf name { type string; description "Community set name"; }
                leaf-list value {
                    type string;
                    description "Extended community values";
                }
            }
        }
    }

    // Community filter config grouping — ingress/egress community tag/strip.
    // Augmented into the existing filter container at bgp, group, and peer levels.
    grouping community-filter-fields {
        container ingress {
            description "Ingress direction filters";

            container community {
                description "Community filter for ingress";

                leaf-list tag {
                    type string;
                    ze:cumulative;
                    description "Named communities to add on ingress";
                }

                leaf-list strip {
                    type string;
                    ze:cumulative;
                    description "Named communities to remove on ingress";
                }
            }
        }

        container egress {
            description "Egress direction filters";

            container community {
                description "Community filter for egress";

                leaf-list tag {
                    type string;
                    ze:cumulative;
                    description "Named communities to add on egress";
                }

                leaf-list strip {
                    type string;
                    ze:cumulative;
                    description "Named communities to remove on egress";
                }
            }
        }
    }

    // Standalone peer — augment existing filter container
    augment "/bgp:bgp/bgp:peer/bgp:filter" {
        uses community-filter-fields;
    }

    // Peer inside group — augment existing filter container
    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:filter" {
        uses community-filter-fields;
    }

    // Group-level — augment existing filter container
    augment "/bgp:bgp/bgp:group/bgp:filter" {
        uses community-filter-fields;
    }

    // BGP-level (global defaults) — augment existing filter container
    augment "/bgp:bgp/bgp:filter" {
        uses community-filter-fields;
    }
}
```

### bgp-filter-community-match

source: `internal/component/bgp/plugins/filter_community_match` -- config root: `bgp` -- depends on: `bgp`

Named community match filter (ordered entries, first match wins, accept/reject)

`ze-filter-community-match.yang`

```yang
module ze-filter-community-match {
    namespace "urn:ze:filter-community-match";
    prefix cm;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }

    description "Community match filter type for the BGP policy framework.
                 Named community-lists with ordered match entries (first match wins)
                 and accept/reject actions. Checks for presence of a community value
                 in the route's standard, large, or extended community attributes.
                 Referenced from peer filter chains as community-match:NAME.
                 Separate from the tag/strip community plugin (bgp-filter-community)
                 because intent differs: filtering vs modification.";

    revision 2026-04-12 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list community-match {
            ze:filter;
            key "name";
            description "Named community match filter instance.
                         Each list contains an ordered set of match entries.
                         Entries are evaluated in order; first match wins.
                         No match = implicit deny (reject).";

            leaf name {
                type string;
                description "Filter instance name (referenced in peer filter chains
                             as community-match:NAME).";
            }

            list entry {
                key "community";
                ordered-by user;
                description "Ordered match entry. First match wins.
                             Checks whether the specified community value is present
                             in the route's community attributes.";

                leaf community {
                    type string;
                    description "Community value to match.
                                 Standard: ASN:VAL (e.g. 65001:100) or well-known name
                                   (no-export, no-advertise, no-export-subconfed, nopeer, blackhole).
                                 Large: GA:LD1:LD2 (e.g. 65001:100:200).
                                 Extended: hex string (e.g. 000200010000000a) or
                                   target:ASN:NN / origin:ASN:NN.";
                }

                leaf type {
                    type enumeration {
                        enum standard {
                            description "Match in standard community attribute (type 8)";
                        }
                        enum large {
                            description "Match in large community attribute (type 32)";
                        }
                        enum extended {
                            description "Match in extended community attribute (type 16)";
                        }
                    }
                    default standard;
                    description "Which community attribute type to check.";
                }

                leaf action {
                    type enumeration {
                        enum accept {
                            description "Accept routes containing this community";
                        }
                        enum reject {
                            description "Reject routes containing this community";
                        }
                    }
                    default accept;
                    description "Action applied when this community is found in the route.";
                }
            }
        }
    }
}
```

### bgp-filter-family

source: `internal/component/bgp/plugins/filter_family` -- config root: `bgp` -- depends on: `bgp`

Named address-family policy filter: remove a family's NLRI or tear down the session

`ze-filter-family.yang`

```yang
module ze-filter-family {
    namespace "urn:ze:filter-family";
    prefix ff;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }
    import ze-types { prefix zt; }

    description "Address-family policy filter for ZeBGP. Matches an AFI/SAFI in a
                 BGP UPDATE and either removes that family's NLRI (import + export)
                 or tears down the session (import only). Referenced from peer
                 filter chains as bgp-filter-family:NAME.";

    revision 2026-06-26 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list family-filter {
            ze:filter;
            key "name";
            description "Named address-family filter instance.";

            leaf name {
                type string;
                description "Filter instance name (referenced in peer filter chains
                             as bgp-filter-family:NAME).";
            }

            leaf family {
                type zt:address-family;
                mandatory true;
                description "Address family to match, e.g. ipv4/flowspec. An UPDATE
                             with no MP_REACH/MP_UNREACH attribute is treated as
                             ipv4/unicast.";
            }

            leaf action {
                type enumeration {
                    enum remove {
                        description "Strip the matched family's MP_REACH/MP_UNREACH
                                     NLRI from the UPDATE (import and export). If the
                                     removal empties the UPDATE, the whole UPDATE is
                                     dropped for that peer.";
                    }
                    enum tear-down {
                        description "Send a BGP NOTIFICATION (Cease / Connection
                                     Rejected) and close the session when the family
                                     is present in a received UPDATE. Import only.";
                    }
                }
                mandatory true;
                description "Action to apply when the family matches.";
            }
        }
    }
}
```

### bgp-filter-irr

source: `internal/component/bgp/plugins/filter_irr` -- config root: `bgp` -- depends on: `bgp`

IRR-based prefix-list filter for eBGP peers

`ze-filter-irr-cmd.yang`

```yang
module ze-filter-irr-cmd {
    namespace "urn:ze:filter-irr:cmd";
    prefix firrcmd;
    import ze-extensions { prefix ze; }
    description "show bgp irr ... and update bgp irr ... command tree. Owned by the bgp-filter-irr plugin so that removing the IRR filter surface removes these command nodes together with the handlers and config schema. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-14 { description "Initial revision."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container bgp {
            config false;
            description "BGP peers, sessions, RIB, and protocol tools";

            container irr {
                config false;
                ze:command "ze-show:irr-status";
                description "Show IRR filter status per ASN.
Lists each enrolled ASN with its resolved AS-SET, prefix counts,
last refresh time, and error status. Use this to confirm that IRR
prefix-lists are loaded and current.";

                container prefix {
                    config false;
                    ze:command "ze-show:irr-prefix";
                    description "Show IRR-resolved prefixes for a peer.
Usage: show bgp irr prefix <peer>. Lists all IPv4 and IPv6 prefixes
in the IRR-resolved prefix-list for the given peer address.";
                    leaf peer { type string; mandatory true; description "Peer address"; }
                }

                container check {
                    config false;
                    ze:command "ze-show:irr-check";
                    description "Check if a prefix is accepted by the IRR filter.
Usage: show bgp irr check <peer> <prefix>. Reports whether the
prefix would be accepted or rejected, and which entry matches.";
                    leaf peer { type string; mandatory true; description "Peer address"; }
                    leaf prefix { type string; mandatory true; description "Prefix to check (CIDR)"; }
                }
            }
        }
    }

    container update {
        config false;

        container bgp {
            config false;
            description "BGP operational updates";

            container irr {
                config false;
                description "IRR prefix-list refresh commands";

                container all {
                    config false;
                    ze:command "ze-update:irr-all";
                    description "Refresh all IRR prefix-lists immediately.
Re-queries the IRR server for every enrolled ASN and atomically
swaps prefix-lists on success. Failed refreshes preserve the
existing prefix-list and report an error.";
                }

                container asn {
                    config false;
                    ze:command "ze-update:irr-asn";
                    description "Refresh IRR prefix-list for a specific ASN.
Usage: update bgp irr asn <asn>. Re-queries the IRR server for
the given ASN only.";
                    leaf asn { type string; mandatory true; description "ASN number"; }
                }

                container as-set {
                    config false;
                    ze:command "ze-update:irr-as-set";
                    description "Refresh IRR prefix-list for a specific AS-SET.
Usage: update bgp irr as-set <as-set>. Re-queries the IRR server
for all peers using the given AS-SET name.";
                    leaf as-set { type string; mandatory true; description "AS-SET name"; }
                }
            }
        }
    }
}
```

`ze-filter-irr.yang`

```yang
module ze-filter-irr {
    namespace "urn:ze:filter-irr";
    prefix firr;

    import ze-bgp-conf { prefix bgp; }

    description "IRR-based prefix-list filter for eBGP peers.
                 Queries IRR databases for AS-SET prefixes and applies them
                 as import filters. Referenced from peer filter chains as
                 bgp-filter-irr:$remote_as.";

    revision 2026-06-13 {
        description "Initial revision";
    }

    grouping irr-peer-config {
        container irr {
            description "Per-peer IRR filter settings.";

            leaf as-set {
                type string;
                description "Explicit AS-SET name for IRR prefix lookup.
                             When omitted, the AS-SET is auto-discovered
                             from PeeringDB using the peer's remote ASN.";
            }

            leaf enable {
                type enumeration {
                    enum enable {
                        description "Enable IRR filtering for this peer (default).";
                    }
                    enum disable {
                        description "Disable IRR filtering for this peer.";
                    }
                }
                default "enable";
                description "Enable or disable IRR filtering for this peer.";
            }
        }
    }

    augment "/bgp:bgp/bgp:policy" {
        container irr {
            description "Global IRR filtering settings.";

            leaf server {
                type string;
                default "whois.radb.net";
                description "IRR whois server hostname or host:port.";
            }

            leaf peeringdb-url {
                type string {
                    pattern 'https?://.*';
                }
                default "https://www.peeringdb.com";
                description "Base URL for PeeringDB API queries.
                             Override for testing with a mock server.";
            }

            leaf refresh-interval {
                type uint32 {
                    range "60..86400";
                }
                default "3600";
                units "seconds";
                description "Seconds between automatic IRR re-queries.";
            }
        }
    }

    augment "/bgp:bgp/bgp:peer/bgp:session" {
        uses irr-peer-config;
    }

    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:session" {
        uses irr-peer-config;
    }

    augment "/bgp:bgp/bgp:group/bgp:session" {
        uses irr-peer-config;
    }
}
```

### bgp-filter-modify

source: `internal/component/bgp/plugins/filter_modify` -- config root: `bgp` -- depends on: `bgp`

Named route attribute modifier (set local-preference, med, origin, next-hop)

`ze-filter-modify.yang`

```yang
module ze-filter-modify {
    namespace "urn:ze:filter-modify";
    prefix fm;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }
    import ze-types { prefix zt; }

    description "Route attribute modifier for the BGP policy framework.
                 Named modifier definitions with attribute setters.
                 Each modifier unconditionally sets the declared attributes
                 on routes that reach it in the filter chain. For conditional
                 modification, compose with match filters (prefix-list,
                 as-path-list, community-match) earlier in the chain.
                 Referenced from peer filter chains as modify:NAME.";

    revision 2026-05-25 {
        description "Add increment/decrement containers and community add/remove leaf-lists";
    }

    augment "/bgp:bgp/bgp:policy" {
        list modify {
            ze:filter;
            key "name";
            description "Named route attribute modifier instance.
                         Only declared leaves are modified; undeclared
                         attributes are preserved unchanged. Multiple
                         attributes can be set in a single modifier.";

            leaf name {
                type string;
                description "Modifier instance name (referenced in peer filter
                             chains as modify:NAME).";
            }

            container set {
                description "Attributes to set on matching routes.
                             Only present leaves are applied.";

                leaf local-preference {
                    type uint32;
                    description "Set LOCAL_PREF attribute (RFC 4271 type 5).";
                }

                leaf med {
                    type uint32;
                    description "Set MULTI_EXIT_DISC attribute (RFC 4271 type 4).";
                }

                leaf origin {
                    type enumeration {
                        enum igp {
                            description "IGP origin";
                        }
                        enum egp {
                            description "EGP origin";
                        }
                        enum incomplete {
                            description "Incomplete origin";
                        }
                    }
                    description "Set ORIGIN attribute (RFC 4271 type 1).";
                }

                leaf next-hop {
                    type zt:ip-address;
                    description "Set NEXT_HOP attribute (RFC 4271 type 3).
                                 IPv4 address only (IPv6 next-hop is in MP_REACH).";
                }

                leaf as-path-prepend {
                    type uint8 {
                        range "1..32";
                    }
                    description "Prepend local AS to AS_PATH this many times.
                                 The actual ASN prepended is the peer's local-as
                                 (from session config). Range 1-32 prevents
                                 excessive path inflation.";
                }

                leaf-list community-add {
                    type string;
                    description "Standard community values to add (ASN:VAL format).";
                }

                leaf-list community-remove {
                    type string;
                    description "Standard community values to remove (ASN:VAL format).";
                }

                leaf-list large-community-add {
                    type string;
                    description "Large community values to add (GA:LD1:LD2 format).";
                }

                leaf-list large-community-remove {
                    type string;
                    description "Large community values to remove (GA:LD1:LD2 format).";
                }

                leaf-list extended-community-add {
                    type string;
                    description "Extended community values to add (target:ASN:NN or hex).";
                }

                leaf-list extended-community-remove {
                    type string;
                    description "Extended community values to remove (target:ASN:NN or hex).";
                }
            }

            container increment {
                description "Attributes to increment on matching routes.
                             Adds the specified value to the current attribute
                             value. Saturates at uint32 max (4294967295).
                             Mutually exclusive with set for the same attribute.";

                leaf local-preference {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Increment LOCAL_PREF by this value.";
                }

                leaf med {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Increment MED by this value.";
                }

                leaf aigp {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Increment AIGP metric by this value.";
                }
            }

            container decrement {
                description "Attributes to decrement on matching routes.
                             Subtracts the specified value from the current
                             attribute value. Floors at 0 (no underflow).
                             Mutually exclusive with set for the same attribute.";

                leaf local-preference {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Decrement LOCAL_PREF by this value.";
                }

                leaf med {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Decrement MED by this value.";
                }

                leaf aigp {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Decrement AIGP metric by this value.";
                }
            }
        }
    }
}
```

### bgp-filter-prefix

source: `internal/component/bgp/plugins/filter_prefix` -- config root: `bgp` -- depends on: `bgp`

Named prefix-list filter (CIDR + ge/le + accept/reject)

`ze-filter-prefix.yang`

```yang
module ze-filter-prefix {
    namespace "urn:ze:filter-prefix";
    prefix fp;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }
    import ze-types { prefix zt; }

    description "Prefix-list filter type for the BGP policy framework.
                 Named prefix-lists with ordered match entries (first match wins),
                 ge/le length ranges, and accept/reject actions.
                 Referenced from peer filter chains as bgp-filter-prefix:NAME.";

    revision 2026-04-11 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list prefix-list {
            ze:filter;
            key "name";
            description "Named prefix-list filter instance.
                         Each list contains an ordered set of match entries.
                         Entries are evaluated in order; first match wins.
                         No match = implicit deny.";

            leaf name {
                type string;
                description "Filter instance name (referenced in peer filter chains
                             as bgp-filter-prefix:NAME).";
            }

            list entry {
                key "prefix";
                ordered-by user;
                description "Ordered match entry. First match wins per route prefix.";

                leaf prefix {
                    type union {
                        type zt:prefix-ipv4;
                        type zt:prefix-ipv6;
                    }
                    description "IPv4 or IPv6 CIDR prefix to match.
                                 Match requires the route prefix be a subnet of this prefix.";
                }

                leaf ge {
                    type uint8 {
                        range "0..128";
                    }
                    description "Minimum match length (greater-than-or-equal).
                                 Defaults to the prefix length of this entry.";
                }

                leaf le {
                    type uint8 {
                        range "0..128";
                    }
                    description "Maximum match length (less-than-or-equal).
                                 Defaults to 32 for IPv4 or 128 for IPv6.";
                }

                leaf action {
                    type enumeration {
                        enum accept {
                            description "Accept route prefixes matching this entry";
                        }
                        enum reject {
                            description "Reject route prefixes matching this entry";
                        }
                    }
                    default accept;
                    description "Action applied when this entry matches a route prefix.";
                }
            }
        }
    }
}
```

### bgp-filter-remove-private-as

source: `internal/component/bgp/plugins/filter_remove_private_as` -- config root: `bgp` -- depends on: `bgp`

Named AS-path action filter that removes RFC 6996 Private Use ASNs

`ze-filter-remove-private-as.yang`

```yang
module ze-filter-remove-private-as {
    namespace "urn:ze:filter-remove-private-as";
    prefix frpa;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }

    description "Remove Private Use ASNs from AS_PATH and AS4_PATH in the BGP policy framework.
                 Referenced from peer filter chains as remove-private-as:NAME.";

    revision 2026-05-24 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list remove-private-as {
            ze:filter;
            key "name";
            description "Named action filter that removes RFC 6996 Private Use ASNs.";

            leaf name {
                type string;
                description "Filter instance name.";
            }

            leaf replace-with {
                type enumeration {
                    enum peer-as {
                        description "Replace Private Use ASNs with the neighbor peer ASN.";
                    }
                }
                description "Replacement mode. When absent, Private Use ASNs are stripped.";
            }
        }
    }
}
```

### bgp-gr

source: `internal/component/bgp/plugins/gr` -- config root: `bgp` -- depends on: `bgp, bgp-rib`

Graceful Restart capability and mechanism plugin

`ze-graceful-restart.yang`

```yang
module ze-graceful-restart {
    namespace "urn:ze:graceful-restart";
    prefix gr;

    import ze-bgp-conf { prefix bgp; }

    description
        "Graceful Restart capability plugin for Ze (RFC 4724, code 64;
         RFC 9494, code 71). Configures per-peer restart-time for BGP
         graceful restart and long-lived-stale-time for LLGR.";

    revision 2025-01-31 {
        description "Initial revision.";
    }

    // Grouping for reuse across all augment paths
    grouping graceful-restart-config {
        container graceful-restart {
            presence "Graceful Restart capability enabled";
            description "Graceful Restart capability configuration.";

            leaf restart-time {
                type uint16 {
                    range "0..4095";
                }
                units "seconds";
                default "120";
                description
                    "Restart Time in seconds (RFC 4724 Section 3).
                     Maximum value is 4095 (12-bit field).";
            }

            leaf long-lived-stale-time {
                type uint32 {
                    range "0..16777215";
                }
                units "seconds";
                description
                    "Long-Lived Stale Time in seconds (RFC 9494 Section 3).
                     When set, LLGR capability (code 71) is advertised.
                     Maximum value is 16777215 (24-bit field, ~194 days).
                     Applied to all negotiated address families for the peer.";
            }
        }
    }

    // Standalone peer capability
    augment "/bgp:bgp/bgp:peer/bgp:session/bgp:capability" {
        uses graceful-restart-config;
    }

    // Peer inside group capability
    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:session/bgp:capability" {
        uses graceful-restart-config;
    }

    // Group-level capability
    augment "/bgp:bgp/bgp:group/bgp:session/bgp:capability" {
        uses graceful-restart-config;
    }
}
```

### bgp-healthcheck

source: `internal/component/bgp/plugins/healthcheck` -- config root: `bgp` -- depends on: `bgp, bgp-watchdog`

Service healthcheck plugin with watchdog route control

`ze-healthcheck-conf.yang`

```yang
module ze-healthcheck-conf {
    namespace "urn:ze:healthcheck-conf";
    prefix hc;

    import ze-bgp-conf { prefix bgp; }

    description "BGP healthcheck plugin configuration for Ze";

    revision 2026-04-03 {
        description "Initial revision";
    }

    grouping probe-config {
        list probe {
            key "name";
            unique "group";
            description "Healthcheck probe definition";

            leaf name {
                type string;
                description "Probe identifier";
            }

            leaf command {
                type string;
                mandatory true;
                description "Shell command to execute for health check (exit 0 = success)";
            }

            leaf group {
                type string;
                mandatory true;
                description "Watchdog group name (exclusive: one probe per group)";
            }

            leaf interval {
                type uint32 {
                    range "0..86400";
                }
                default 5;
                description "Seconds between checks (0 = single check then dormant)";
            }

            leaf fast-interval {
                type uint32 {
                    range "1..3600";
                }
                default 1;
                description "Seconds between checks during RISING/FALLING states";
            }

            leaf timeout {
                type uint32 {
                    range "1..3600";
                }
                default 5;
                description "Command timeout in seconds";
            }

            leaf rise {
                type uint32 {
                    range "1..1000";
                }
                default 3;
                description "Consecutive successes before UP";
            }

            leaf fall {
                type uint32 {
                    range "1..1000";
                }
                default 3;
                description "Consecutive failures before DOWN";
            }

            leaf withdraw-on-down {
                type boolean;
                default false;
                description "When true, withdraw route on DOWN/DISABLED. When false (default), re-announce with down-metric/disabled-metric.";
            }

            leaf disable {
                type boolean;
                default false;
                description "Admin disable: probe enters DISABLED state immediately";
            }

            leaf debounce {
                type boolean;
                default false;
                description "When true, only dispatch watchdog commands on state changes";
            }

            leaf up-metric {
                type uint32;
                default 100;
                description "MED value when UP";
            }

            leaf down-metric {
                type uint32;
                default 1000;
                description "MED value when DOWN (used when withdraw-on-down is false)";
            }

            leaf disabled-metric {
                type uint32;
                default 500;
                description "MED value when DISABLED (used when withdraw-on-down is false)";
            }

            container ip-setup {
                description "VIP management on local interface (internal plugin mode only)";

                leaf interface {
                    type string;
                    description "Target interface for VIPs (e.g., lo, dummy0)";
                }

                leaf dynamic {
                    type boolean;
                    default false;
                    description "When true, remove IPs on DOWN/DISABLED, restore on UP";
                }

                leaf-list ip {
                    type string;
                    description "VIP addresses in CIDR notation (e.g., 10.0.0.1/32)";
                }
            }

            leaf-list on-up {
                type string;
                description "Shell commands to execute on transition to UP (30s timeout)";
            }

            leaf-list on-down {
                type string;
                description "Shell commands to execute on transition to DOWN (30s timeout)";
            }

            leaf-list on-disabled {
                type string;
                description "Shell commands to execute on transition to DISABLED (30s timeout)";
            }

            leaf-list on-change {
                type string;
                description "Shell commands to execute on any state transition (30s timeout, runs after state-specific hooks)";
            }
        }
    }

    augment "/bgp:bgp" {
        container healthcheck {
            description "Healthcheck probes for service-aware BGP route management";
            uses probe-config;
        }
    }
}
```

### bgp-hostname

source: `internal/component/bgp/plugins/hostname` -- config root: `bgp` -- depends on: `bgp`

FQDN capability decoding

`ze-hostname.yang`

```yang
module ze-hostname {
    namespace "urn:ze:hostname";
    prefix hostname;

    import ze-bgp-conf { prefix bgp; }

    description
        "FQDN capability plugin for ZeBGP (draft-walton-bgp-hostname, code 73).
         Advertises hostname and domain name of the BGP speaker.";

    revision 2025-01-29 {
        description "Initial revision.";
    }

    // Standalone peers: augment capability container
    augment "/bgp:bgp/bgp:peer/bgp:session/bgp:capability" {
        container hostname {
            description "FQDN capability configuration.";

            leaf host {
                type string {
                    length "0..255";
                }
                description "System hostname (max 255 bytes).";
            }

            leaf domain {
                type string {
                    length "0..255";
                }
                description "Domain name (max 255 bytes).";
            }
        }
    }

    // Group-level capability
    augment "/bgp:bgp/bgp:group/bgp:session/bgp:capability" {
        container hostname {
            description "FQDN capability configuration.";

            leaf host {
                type string {
                    length "0..255";
                }
                description "System hostname (max 255 bytes).";
            }

            leaf domain {
                type string {
                    length "0..255";
                }
                description "Domain name (max 255 bytes).";
            }
        }
    }

    // Grouped peers: augment capability container
    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:session/bgp:capability" {
        container hostname {
            description "FQDN capability configuration.";

            leaf host {
                type string {
                    length "0..255";
                }
                description "System hostname (max 255 bytes).";
            }

            leaf domain {
                type string {
                    length "0..255";
                }
                description "Domain name (max 255 bytes).";
            }
        }
    }

    // Standalone peers: legacy syntax (in session container)
    augment "/bgp:bgp/bgp:peer/bgp:session" {
        leaf host-name {
            type string {
                length "0..255";
            }
            description "Legacy: Host name for FQDN capability.";
        }

        leaf domain-name {
            type string {
                length "0..255";
            }
            description "Legacy: Domain name for FQDN capability.";
        }
    }

    // Grouped peers: legacy syntax (in session container)
    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:session" {
        leaf host-name {
            type string {
                length "0..255";
            }
            description "Legacy: Host name for FQDN capability.";
        }

        leaf domain-name {
            type string {
                length "0..255";
            }
            description "Legacy: Domain name for FQDN capability.";
        }
    }

    // Group-level: legacy syntax (in session container)
    augment "/bgp:bgp/bgp:group/bgp:session" {
        leaf host-name {
            type string {
                length "0..255";
            }
            description "Legacy: Host name for FQDN capability (group default).";
        }

        leaf domain-name {
            type string {
                length "0..255";
            }
            description "Legacy: Domain name for FQDN capability (group default).";
        }
    }
}
```

### bgp-llnh

source: `internal/component/bgp/plugins/llnh` -- config root: `bgp` -- depends on: `bgp`

Link-Local Next-Hop capability plugin

`ze-link-local-nexthop.yang`

```yang
module ze-link-local-nexthop {
    namespace "urn:ze:link-local-nexthop";
    prefix llnh;

    import ze-bgp-conf { prefix bgp; }

    description
        "Link-local next-hop capability plugin for Ze
         (draft-ietf-idr-linklocal-capability, code 77).
         Signals peer support for IPv6 link-local addresses as BGP next-hops.";

    revision 2025-02-08 {
        description "Initial revision.";
    }

    grouping link-local-nexthop-config {
        container link-local-nexthop {
            presence "Link-local next-hop capability enabled";
            description
                "Link-local next-hop capability.
                 Advertises willingness to receive IPv6 link-local next-hops
                 in MP_REACH_NLRI (RFC 2545 Section 3).";
        }
    }

    // Standalone peer capability
    augment "/bgp:bgp/bgp:peer/bgp:session/bgp:capability" {
        uses link-local-nexthop-config;
    }

    // Peer inside group capability
    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:session/bgp:capability" {
        uses link-local-nexthop-config;
    }

    // Group-level capability
    augment "/bgp:bgp/bgp:group/bgp:session/bgp:capability" {
        uses link-local-nexthop-config;
    }
}
```

### bgp-nlri-evpn

source: `internal/component/bgp/plugins/nlri/evpn`

EVPN family plugin

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-flowspec

source: `internal/component/bgp/plugins/nlri/flowspec`

FlowSpec NLRI encoding/decoding

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-labeled

source: `internal/component/bgp/plugins/nlri/labeled`

Labeled Unicast family plugin (RFC 8277)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-ls

source: `internal/component/bgp/plugins/nlri/ls`

BGP-LS family plugin

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-mup

source: `internal/component/bgp/plugins/nlri/mup`

Mobile User Plane family plugin (draft-mpmz-bess-mup-safi)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-mvpn

source: `internal/component/bgp/plugins/nlri/mvpn`

Multicast VPN family plugin (RFC 6514)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-rtc

source: `internal/component/bgp/plugins/nlri/rtc`

Route Target Constraint family plugin (RFC 4684)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-srpolicy

source: `internal/component/bgp/plugins/nlri/srpolicy`

SR-Policy family plugin (RFC 9830, SAFI 73)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-vpls

source: `internal/component/bgp/plugins/nlri/vpls`

VPLS family plugin (RFC 4761)

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-nlri-vpn

source: `internal/component/bgp/plugins/nlri/vpn`

VPN family plugin

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-persist

source: `internal/component/bgp/plugins/persist`

Route Persistence

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-redistribute

source: `internal/component/bgp/plugins/redistribute_ingress` -- depends on: `bgp`

Route redistribution ingress filter with loop prevention and family filtering

No YANG module of its own (reads config defined by another plugin, or has none).

### bgp-rib

source: `internal/component/bgp/plugins/rib` -- config root: `bgp`

Route Information Base storage

`ze-rib-api.yang`

```yang
module ze-rib-api {
    namespace "urn:ze:rib:api";
    prefix ribapi;

    import ze-types { prefix zt; }

    description
        "RIB API operations for Ze.
         RPCs for querying and clearing RIB state.
         Notifications for RIB changes.";

    revision 2026-02-01 {
        description "Initial revision";
    }

    // Introspection

    rpc help {
        description "Show RIB subcommands";
        output {
            leaf-list subcommands { type string; description "Available subcommands"; }
        }
    }

    rpc command-list {
        description "List RIB commands";
        output {
            list command {
                leaf name { type string; description "Command name"; }
                leaf description { type string; description "Command description"; }
            }
        }
    }

    rpc command-help {
        description "Show RIB command details";
        input {
            leaf name { type string; mandatory true; description "Command name"; }
        }
        output {
            leaf help { type string; description "Detailed help text"; }
        }
    }

    rpc command-complete {
        description "Complete RIB command/args";
        input {
            leaf partial { type string; mandatory true; description "Partial command string"; }
        }
        output {
            leaf-list completions { type string; description "Completion candidates"; }
        }
    }

    rpc event-list {
        description "List RIB event types";
        output {
            list event {
                leaf name { type string; description "Event type name"; }
                leaf description { type string; description "Event description"; }
            }
        }
    }

    // RIB operations

    rpc status {
        description "RIB summary: peer count, route counts, GR state.";
        output {
            leaf running { type boolean; description "RIB plugin is running"; }
            leaf peers { type uint32; description "Number of peers tracked"; }
            leaf routes-in { type uint32; description "Total Adj-RIB-In routes"; }
            leaf routes-out { type uint32; description "Total Adj-RIB-Out routes"; }
            leaf stale-routes { type uint32; description "Stale routes (GR)"; }
        }
    }

    rpc show {
        description "Show routes with pipeline filters.
                     Syntax: show bgp rib [scope] [filters...] [terminal]
                     Scope: sent | received | sent-received (default)
                     Filters: path <pattern>, prefix <pattern>, community <value>,
                              family <afi/safi>, match <text>
                     Terminals: count (metadata only), json (serialized output),
                                prefix-summary (per-family prefix-length distribution),
                                graph (AS-path topology as box-drawing text)";
        input {
            leaf peer { type zt:ip-address; description "Filter by peer"; }
            leaf-list args { type string; description "Pipeline args: scope, filters, terminal"; }
        }
        output {
            leaf route-count { type uint32; description "Number of routes (count terminal)"; }
        }
    }

    rpc best {
        description "Show best-path per prefix (RFC 4271 §9.1.2).
                     Syntax: show bgp rib best [filters...] [terminal]
                     Filters: path <pattern>, prefix <pattern>, community <value>,
                              family <afi/safi>, match <text>
                     Terminals: count (metadata only), json (serialized output),
                                prefix-summary (per-family prefix-length distribution),
                                graph (AS-path topology as box-drawing text)";
        input {
            leaf peer { type zt:ip-address; description "Filter by peer"; }
            leaf-list args { type string; description "Pipeline args: filters, terminal"; }
        }
        output {
            leaf route-count { type uint32; description "Number of best paths"; }
        }
    }

    rpc best-status {
        description "Show best-path computation status";
        output {
            leaf peers-with-rib { type uint32; description "Peers with RIB entries"; }
            leaf total-routes { type uint32; description "Total route count across peers"; }
        }
    }

    rpc clear-in {
        description "Clear Adj-RIB-In entries";
        input {
            leaf peer { type zt:ip-address; description "Filter by peer"; }
            leaf family { type zt:address-family; description "Filter by address family"; }
        }
    }

    rpc clear-out {
        description "Clear Adj-RIB-Out entries";
        input {
            leaf peer { type zt:ip-address; description "Filter by peer"; }
            leaf family { type zt:address-family; description "Filter by address family"; }
        }
    }

    rpc inject {
        description "Insert route into Adj-RIB-In as if received from a peer.
                     The peer address is a label; no live BGP session required.
                     Syntax: request bgp rib inject <peer> <family> <prefix> [origin <val>]
                             [nhop|nexthop <ip>] [aspath <asn,...>] [localpref <n>] [med <n>]";
        input {
            leaf peer { type zt:ip-address; mandatory true; description "Peer address label"; }
            leaf family { type zt:address-family; mandatory true; description "Address family (e.g. ipv4/unicast)"; }
            leaf prefix { type string; mandatory true; description "CIDR prefix (e.g. 10.0.0.0/24)"; }
            leaf origin { type string; description "Origin: igp, egp, incomplete (default: igp)"; }
            leaf nhop { type zt:ip-address; description "Next-hop IP address"; }
            leaf nexthop { type zt:ip-address; description "Alias for nhop next-hop IP address"; }
            leaf aspath { type string; description "AS path as comma-separated ASNs (e.g. 64500,64501)"; }
            leaf localpref { type uint32; description "LOCAL_PREF value"; }
            leaf med { type uint32; description "MULTI_EXIT_DISC value"; }
        }
        output {
            leaf injected { type string; description "Prefix that was injected"; }
            leaf peer { type zt:ip-address; description "Peer the route was injected for"; }
            leaf family { type zt:address-family; description "Address family"; }
        }
    }

    rpc withdraw {
        description "Remove route from Adj-RIB-In.
                     Syntax: request bgp rib withdraw <peer> <family> <prefix>";
        input {
            leaf peer { type zt:ip-address; mandatory true; description "Peer address label"; }
            leaf family { type zt:address-family; mandatory true; description "Address family"; }
            leaf prefix { type string; mandatory true; description "CIDR prefix to withdraw"; }
        }
        output {
            leaf withdrawn { type string; description "Prefix that was withdrawn"; }
            leaf peer { type zt:ip-address; description "Peer the route was withdrawn from"; }
            leaf family { type zt:address-family; description "Address family"; }
            leaf existed { type boolean; description "Whether the route existed before withdrawal"; }
        }
    }

    // Notifications

    notification rib-change {
        description "RIB content changed";
        leaf peer { type zt:ip-address; description "Affected peer"; }
        leaf family { type zt:address-family; description "Address family"; }
        leaf added { type uint32; description "Routes added"; }
        leaf removed { type uint32; description "Routes removed"; }
    }
}
```

`ze-rib.yang`

```yang
module ze-rib {
    namespace "urn:ze:rib";
    prefix rib;

    import ze-bgp-conf { prefix bgp; }

    description
        "RIB (Routing Information Base) plugin for Ze.
         Tracks Adj-RIB-In (routes from peers) and Adj-RIB-Out (routes to peers).
         Supports ADD-PATH (RFC 7911) with per-path-id storage.";

    revision 2025-01-31 {
        description "Initial revision.";
    }

    augment "/bgp:bgp" {
        container rib {
            description "RIB plugin state and operations.";
            config false;

            container adj-rib-in {
                description "Routes received from peers.";

                list peer {
                    key "address";
                    description "Per-peer Adj-RIB-In.";

                    leaf address {
                        type string;
                        description "Peer IP address.";
                    }

                    leaf route-count {
                        type uint32;
                        description "Number of routes from this peer.";
                    }
                }
            }

            container adj-rib-out {
                description "Routes sent to peers.";

                list peer {
                    key "address";
                    description "Per-peer Adj-RIB-Out.";

                    leaf address {
                        type string;
                        description "Peer IP address.";
                    }

                    leaf route-count {
                        type uint32;
                        description "Number of routes to this peer.";
                    }
                }
            }
        }
    }
}
```

### bgp-role

source: `internal/component/bgp/plugins/role` -- config root: `bgp` -- depends on: `bgp`

RFC 9234 BGP Role capability

`ze-role.yang`

```yang
module ze-role {
    namespace "urn:ze:role";
    prefix role;

    import ze-bgp-conf { prefix bgp; }

    description "RFC 9234 BGP Role plugin for ZeBGP";

    revision 2026-01-01 {
        description "Initial revision";
    }

    typedef role-type {
        type enumeration {
            enum provider { value 0; description "RFC 9234: Provider (0)"; }
            enum rs { value 1; description "RFC 9234: Route Server (1)"; }
            enum rs-client { value 2; description "RFC 9234: RS-Client (2)"; }
            enum customer { value 3; description "RFC 9234: Customer (3)"; }
            enum peer { value 4; description "RFC 9234: Peer (4)"; }
        }
        description "RFC 9234 BGP Role values";
    }

    typedef export-token {
        type union {
            type enumeration {
                enum default { description "RFC 9234 Section 5 default egress rules for the declared role"; }
                enum unknown { description "Also send to peers with no role configured"; }
            }
            type role-type;
        }
        description "Export filter token: default expands to RFC rules, unknown sends to untagged peers, or explicit role name";
    }

    grouping role-config {
        container role {
            description "RFC 9234 BGP Role configuration";

            leaf import {
                type role-type;
                description "Declares local role and enables RFC 9234 ingress rules (replaces Phase 1 name keyword)";
            }

            leaf-list export {
                type export-token;
                description "Controls which destination peer roles may receive routes from this peer";
            }

            leaf strict {
                type boolean;
                default false;
                description "Require peer to send Role capability";
            }
        }
    }

    // Standalone peer
    augment "/bgp:bgp/bgp:peer" {
        uses role-config;
    }

    // Peer inside group
    augment "/bgp:bgp/bgp:group/bgp:peer" {
        uses role-config;
    }

    // Group-level
    augment "/bgp:bgp/bgp:group" {
        uses role-config;
    }
}
```

### bgp-route-refresh

source: `internal/component/bgp/plugins/route_refresh` -- config root: `bgp` -- depends on: `bgp`

Route Refresh capability decoding

`ze-refresh-cmd.yang`

```yang
module ze-refresh-cmd {
    namespace "urn:ze:refresh:cmd";
    prefix refreshcmd;
    import ze-extensions { prefix ze; }
    description "Request peers to re-send routes (RFC 2918, RFC 7313)";
    revision 2026-03-16 { description "Initial revision"; }

    container request {
        config false;

        container peer {
            config false;

            container refresh {
                config false;
                ze:command "ze-bgp:peer-refresh";
                description "Ask a peer to re-send all routes (RFC 2918).
Sends a ROUTE-REFRESH message for the specified AFI/SAFI. The
peer will re-advertise its entire Adj-RIB-Out.";
                leaf selector { type string; mandatory true; description "Peer selector"; }
            }

            container borr {
                config false;
                ze:command "ze-bgp:peer-borr";
                description "Start an Enhanced Route Refresh cycle (RFC 7313).
Tells the peer to mark existing routes as stale. After re-sending,
send EORR to purge anything not refreshed.";
                leaf selector { type string; mandatory true; description "Peer selector"; }
            }

            container eorr {
                config false;
                ze:command "ze-bgp:peer-eorr";
                description "Finish an Enhanced Route Refresh cycle (RFC 7313).
The peer purges any routes not re-advertised since the matching
BORR. Only send this after the peer has finished re-advertising.";
                leaf selector { type string; mandatory true; description "Peer selector"; }
            }

            container clear {
                config false;
                description "Non-disruptive peer refresh";

                container soft {
                    config false;
                    ze:command "ze-bgp:peer-clear-soft";
                    description "Soft-clear a peer without dropping the session.
Sends ROUTE-REFRESH for every negotiated AFI/SAFI, causing the peer
to re-send all routes. No session bounce, no traffic impact.";
                    leaf selector { type string; mandatory true; description "Peer selector"; }
                }
            }
        }
    }
}
```

`ze-route-refresh-api.yang`

```yang
module ze-route-refresh-api {
    namespace "urn:ze:route-refresh:api";
    prefix "rr-api";

    organization "Ze Project";
    description "Route refresh command RPCs (RFC 2918, RFC 7313).";
    revision 2026-03-08 {
        description "Initial revision.";
    }

    rpc peer-refresh {
        description "Send ROUTE-REFRESH to peer for specified family.";
    }
    rpc peer-borr {
        description "Send Beginning of Route Refresh marker (RFC 7313).";
    }
    rpc peer-eorr {
        description "Send End of Route Refresh marker (RFC 7313).";
    }
    rpc peer-clear-soft {
        description "Soft-clear peer by sending ROUTE-REFRESH for all negotiated families.";
    }
}
```

`ze-route-refresh.yang`

```yang
module ze-route-refresh {
    namespace "urn:ze:route-refresh";
    prefix rr;

    import ze-bgp-conf { prefix bgp; }

    description
        "Route Refresh capability plugin for ZeBGP (RFC 2918, code 2)
         and Enhanced Route Refresh (RFC 7313, code 70).";

    revision 2026-02-22 {
        description "Initial revision.";
    }

    // NOTE: route-refresh container is defined in peer-fields grouping (ze-bgp-conf.yang).
    // These augments are no-ops since the container already exists via uses peer-fields,
    // but they're kept for documentation and forward compatibility.
}
```

### bgp-rpki

source: `internal/component/bgp/plugins/rpki` -- config root: `bgp` -- depends on: `bgp, bgp-adj-rib-in`

RPKI origin validation via RTR protocol

`ze-rpki.yang`

```yang
module ze-rpki {
  namespace "urn:ze:rpki";
  prefix rpki;

  import ze-bgp-conf {
    prefix bgp;
  }

  grouping rpki-config {
    container rpki {
      presence "RPKI origin validation enabled";

      list cache-server {
        key "address";
        leaf address {
          type string;
          description "Cache server IP or hostname";
        }
        leaf port {
          type uint16;
          default "323";
          description "RTR TCP port";
        }
        leaf preference {
          type uint8;
          default "100";
          description "Server preference (lower = preferred)";
        }
      }

      leaf validation-timeout {
        type uint16;
        default "30";
        units "seconds";
        description "Fail-open timeout for pending routes";
      }

      container policy {
        leaf invalid-action {
          type enumeration {
            enum reject;
            enum log-only;
            enum accept;
          }
          default "reject";
          description "Action for routes with Invalid validation state";
        }
        leaf not-found-action {
          type enumeration {
            enum accept;
            enum reject;
            enum log-only;
          }
          default "accept";
          description "Action for routes with NotFound validation state";
        }
      }

      container aspa {
        leaf validation {
          type boolean;
          default "false";
          description "Enable ASPA path verification using RTR v2 ASPA records";
        }
        container policy {
          leaf invalid-action {
            type enumeration {
              enum reject;
              enum log-only;
              enum accept;
            }
            default "log-only";
            description "Action for routes with ASPA Invalid path verification state";
          }
          leaf unknown-action {
            type enumeration {
              enum accept;
              enum reject;
              enum log-only;
            }
            default "accept";
            description "Action for routes with ASPA Unknown path verification state";
          }
        }
      }
    }
  }

  augment "/bgp:bgp" {
    uses rpki-config;
  }
}
```

### bgp-rpki-decorator

source: `internal/component/bgp/plugins/rpki_decorator` -- depends on: `bgp, bgp-rpki`

Correlates UPDATE + RPKI events into merged update-rpki events

`ze-rpki-decorator.yang`

```yang
module ze-rpki-decorator {
  namespace "urn:ze:rpki:decorator";
  prefix rpkidec;

  description
    "RPKI decorator plugin: correlates UPDATE and RPKI events,
     emits merged update-rpki events for downstream consumers.";

  revision 2026-03-21 {
    description "Initial revision";
  }
}
```

### bgp-rr

source: `internal/component/bgp/plugins/rr` -- depends on: `bgp-adj-rib-in`

Route Reflector

`ze-rr-cmd.yang`

```yang
module ze-rr-cmd {
    namespace "urn:ze:rr:cmd";
    prefix rrcmd;
    import ze-extensions { prefix ze; }
    description "show rr ... command tree. Owned by the bgp-rr plugin so that removing the route-reflector surface removes these command nodes together with the handlers. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated show rr ... out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container rr {
            config false;
            description "Route reflector status and client peers";

            container status {
                config false;
                ze:command "ze-show:rr-status";
                description "Show whether the route reflector is active.
Returns cluster ID, running state, and summary statistics
(reflected routes, client count).";
            }

            container peers {
                config false;
                ze:command "ze-show:rr-peers";
                description "Show route reflector client peers.
Lists each RR client with session state and reflected route counts.";
            }
        }
    }
}
```

### bgp-rs

source: `internal/component/bgp/plugins/rs` -- config root: `bgp` -- optional: `bgp-adj-rib-in`

Route Server

`ze-rs-conf.yang`

```yang
module ze-rs-conf {
    namespace "urn:ze:rs:conf";
    prefix rs;

    import ze-bgp-conf { prefix bgp; }

    description "BGP route server plugin configuration for Ze";

    revision 2026-06-12 {
        description "Initial revision";
    }

    augment "/bgp:bgp" {
        container route-server {
            description "Route server plugin tuning parameters";

            leaf worker-queue-size {
                type uint32 {
                    range "1..1000000";
                }
                default 4096;
                description "Per-source-peer worker channel capacity.
                             Env var ze.bgp.route-server.worker-queue-size overrides this value.";
            }
        }
    }
}
```

### bgp-softver

source: `internal/component/bgp/plugins/softver` -- config root: `bgp` -- depends on: `bgp`

Software Version capability (code 75)

`ze-softver.yang`

```yang
module ze-softver {
    namespace "urn:ze:softver";
    prefix softver;

    import ze-bgp-conf { prefix bgp; }

    description
        "Software Version capability plugin for ZeBGP (draft-ietf-idr-software-version, code 75).
         Advertises the software version of the BGP speaker.";

    revision 2026-02-22 {
        description "Initial revision.";
    }

    grouping software-version-config {
        container software-version {
            presence "Software Version capability enabled";
            description "Software Version capability (code 75).";
            leaf mode {
                type enumeration {
                    enum enable { description "Advertise capability (default)."; }
                    enum disable { description "Do not advertise capability."; }
                    enum require { description "Advertise and require peer support."; }
                    enum refuse { description "Do not advertise, reject if peer has it."; }
                }
                default "enable";
                description "Capability negotiation mode.";
            }
        }
    }

    // Standalone peer capability
    augment "/bgp:bgp/bgp:peer/bgp:session/bgp:capability" {
        uses software-version-config;
    }

    // Peer inside group capability
    augment "/bgp:bgp/bgp:group/bgp:peer/bgp:session/bgp:capability" {
        uses software-version-config;
    }

    // Group-level capability
    augment "/bgp:bgp/bgp:group/bgp:session/bgp:capability" {
        uses software-version-config;
    }
}
```

### bgp-watchdog

source: `internal/component/bgp/plugins/watchdog` -- config root: `bgp` -- depends on: `bgp`

Watchdog route management plugin

No YANG module of its own (reads config defined by another plugin, or has none).

## Class Of Service (`class-of-service`, 1 plugins)

### cos

source: `internal/plugins/cos` -- config root: `class-of-service`

802.1p class-of-service profile definitions

`ze-cos-conf.yang`

```yang
module ze-cos-conf {
    namespace "urn:ze:cos:conf";
    prefix cos;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description
        "Class-of-service profile definitions and interface bindings.
         Profiles define named 802.1p PCP-to-priority mappings that can
         be referenced from interface configuration instead of inline
         ingress-qos-map / egress-qos-map lists.";

    revision 2026-06-12 {
        description "Initial revision";
    }

    container class-of-service {
        description "Named class-of-service profiles";

        list ieee-802.1p {
            key "name";
            description
                "An 802.1p QoS profile. Defines the mapping between the
                 3-bit PCP field in the 802.1Q header and internal
                 priorities, for both ingress and egress directions.";

            leaf name {
                type zt:node-name;
                description "Profile name (lowercase alphanumeric and hyphens)";
            }

            container ingress {
                description "PCP-to-priority mapping for received tagged frames";

                list pcp {
                    key "value";
                    description "Map a received PCP value to an internal priority";

                    leaf value {
                        type uint8 {
                            range "0..7";
                        }
                        description "PCP value in the received 802.1Q header (IEEE 802.1Q, 3 bits)";
                    }

                    leaf priority {
                        type uint8 {
                            range "0..7";
                        }
                        mandatory true;
                        description "Internal priority assigned to matching frames";
                    }
                }
            }

            container egress {
                description "Priority-to-PCP mapping for transmitted tagged frames";

                list priority {
                    key "value";
                    description "Map an internal priority to a transmitted PCP value";

                    leaf value {
                        type uint8 {
                            range "0..7";
                        }
                        description "Internal priority of the outgoing packet";
                    }

                    leaf pcp {
                        type uint8 {
                            range "0..7";
                        }
                        mandatory true;
                        description "PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)";
                    }
                }
            }
        }
    }

    grouping qos-maps {
        description "Inline 802.1p QoS maps for VLAN sub-interfaces.
                     Maps translate between the 3-bit PCP field of the
                     802.1Q tag header and kernel-internal priority.";

        list ingress-qos-map {
            key "pcp";
            description "Map the 802.1p PCP value of received tagged frames
                         to an internal priority. Requires vlan-id. Frames
                         whose PCP has no entry keep priority 0.";

            leaf pcp {
                type uint8 {
                    range "0..7";
                }
                description "PCP value in the received 802.1Q header (IEEE 802.1Q, 3 bits)";
            }

            leaf priority {
                type uint8 {
                    range "0..7";
                }
                mandatory true;
                description "Internal priority assigned to matching frames";
            }
        }

        list egress-qos-map {
            key "priority";
            description "Map the internal priority of outgoing packets to
                         the 802.1p PCP value stamped in the 802.1Q header.
                         Requires vlan-id. Priorities with no entry are
                         sent with PCP 0.";

            leaf priority {
                type uint8 {
                    range "0..7";
                }
                description "Internal priority of the outgoing packet";
            }

            leaf pcp {
                type uint8 {
                    range "0..7";
                }
                mandatory true;
                description "PCP value stamped in the 802.1Q header (IEEE 802.1Q, 3 bits)";
            }
        }
    }

    grouping cos-unit-leaves {
        description "Per-unit class-of-service leaves: profile reference,
                     inline QoS maps, merged into interface units via
                     container-merge.";

        leaf class-of-service {
            type string;
            description "Per-unit override: profile name or 'none' to
                         opt out of inheritance from the parent interface.";
        }

        uses qos-maps;
    }

    grouping cos-interface-leaves {
        description "Interface-level class-of-service leaf merged into L2
                     interface types via container-merge.";

        leaf class-of-service {
            type string;
            description "Name of a class-of-service ieee-802.1p profile, or
                         'none' to explicitly disable inheritance. When set
                         on the interface, all VLAN units inherit the profile
                         unless overridden per-unit.";
        }
    }

    container interface {
        description "Interface-level class-of-service bindings and inline
                     QoS maps (container-merge with ze-iface-conf).
                     Removing the cos plugin removes all QoS surface
                     from interfaces.";

        list ethernet {
            key "name";
            leaf name { type string; }
            uses cos-interface-leaves;
            list unit {
                key "name";
                leaf name { type zt:node-name; }
                uses cos-unit-leaves;
            }
        }

        list dummy {
            key "name";
            leaf name { type string; }
            uses cos-interface-leaves;
            list unit {
                key "name";
                leaf name { type zt:node-name; }
                uses cos-unit-leaves;
            }
        }

        list veth {
            key "name";
            leaf name { type string; }
            uses cos-interface-leaves;
            list unit {
                key "name";
                leaf name { type zt:node-name; }
                uses cos-unit-leaves;
            }
        }

        list bridge {
            key "name";
            leaf name { type string; }
            uses cos-interface-leaves;
            list unit {
                key "name";
                leaf name { type zt:node-name; }
                uses cos-unit-leaves;
            }
        }
    }
}
```

## Connected (`connected`, 1 plugins)

### connected

source: `internal/plugins/connected` -- config root: `connected`

Connected routes: redistribute directly connected interface prefixes

`ze-connected-conf.yang`

```yang
module ze-connected-conf {
    namespace "urn:ze:connected:conf";
    prefix connected;

    import ze-extensions { prefix ze; }

    description
        "Connected route redistribution for Ze.
         When present, directly connected interface prefixes are
         advertised into BGP via the redistribute event bus.";

    revision 2026-05-12 {
        description "Initial revision.";
    }

    container connected {
        description "Connected route redistribution. Presence enables the plugin.";
        ze:config-root "connected";
    }
}
```

## Control Plane Protection (`control-plane-protection`, 1 plugins)

### copp-input-chain

source: `internal/plugins/copp` -- config root: `control-plane-protection` -- depends on: `firewall`

Control-plane policing: rate-limit new TCP connections to BGP listen port

`ze-copp-conf.yang`

```yang
module ze-copp-conf {
    namespace "urn:ze:copp:conf";
    prefix copp;

    import ze-extensions { prefix ze; }

    description
        "Control-plane policing configuration for Ze.
         Protects host-bound traffic to the BGP listen port
         (TCP/179) from connection-flood DDoS by rate-limiting
         new connections while exempting established sessions
         and trusted sources.";

    revision 2026-06-26 {
        description "Initial revision.";
    }

    typedef rate-spec {
        type string {
            pattern '[0-9]+(bytes|kbytes|mbytes|gbytes)?/(second|minute|hour|day)';
        }
        description "Rate limit: packet-per-unit (10/second) or byte-per-unit (1mbytes/second).";
    }

    container control-plane-protection {
        description "Control-plane policing configuration.";

        container bgp {
            presence "BGP control-plane policing enabled";
            description "Rate-limit new TCP connections to the BGP listen port.";

            leaf rate {
                type rate-spec;
                mandatory true;
                description "Rate limit for new connections (e.g., 100/second).";
            }

            leaf burst {
                type uint32;
                description "Burst size (packets or bytes matching the rate unit).";
            }

            leaf-list protected-port {
                type uint16 {
                    range "1..65535";
                }
                description
                    "TCP port(s) to protect. Defaults to 179 (BGP) when
                     omitted. Set to a non-default value when the BGP
                     listener uses an alternate port.";
            }

            leaf-list trusted-source {
                type string;
                ze:validate "ipv4-prefix|ipv6-prefix";
                description
                    "Source prefixes that bypass the rate limit.
                     Typically the addresses of configured BGP peers.";
            }

            leaf over-limit-policy {
                type enumeration {
                    enum accept {
                        description "Accept over-limit packets (safe default).";
                    }
                    enum drop {
                        description "Drop over-limit packets.";
                    }
                }
                default "accept";
                description
                    "Action for packets exceeding the rate limit.
                     Default is accept to avoid lock-out risk.";
            }
        }
    }
}
```

## Ddos Detect (`ddos-detect`, 1 plugins)

### ddos-detect-flow-source

source: `internal/plugins/ddos/detect` -- config root: `ddos-detect` -- depends on: `config-loaded`

Automatic DDoS attack detector with two-stage detection

`ze-ddos-detect-conf.yang`

```yang
module ze-ddos-detect-conf {
  namespace "urn:ze:ddos-detect-conf";
  prefix ddos-detect;

  import ze-types { prefix zt; }

  container ddos-detect {
    leaf enabled {
      type boolean;
      default false;
      description "Enable the DDoS detector.";
    }
    leaf check-interval {
      type uint16 { range "1..3600"; }
      default 1;
      description "Seconds between detection evaluations.";
    }
    leaf confirm-duration {
      type uint16 { range "0..3600"; }
      default 3;
      description "Consecutive ticks above threshold before triggering.";
    }
    leaf clear-consecutive-checks {
      type uint16 { range "1..100"; }
      default 10;
      description "Consecutive ticks below threshold before clearing.";
    }
    leaf baseline-window {
      type uint32 { range "10..86400"; }
      default 300;
      description "Rolling baseline window size in samples.";
    }
    leaf threshold-multiplier {
      type zt:decimal-2 { range "1.00..100.00"; }
      default "3.00";
      description "Baseline p99 multiplier for the dynamic threshold.";
    }
    leaf absolute-floor {
      type uint32 { range "1..max"; }
      default 5000;
      description "Minimum threshold in PPS regardless of baseline.";
    }
    leaf startup-grace {
      type uint16 { range "0..3600"; }
      default 90;
      description "Seconds after startup where only extreme spikes trigger.";
    }
    leaf characterize-enable {
      type boolean;
      default true;
      description "Run Stage-2 flow characterization (classify family + narrowest vector from the flow-export recent-flow ring, emit AttackCharacterized). When false the detector still emits the coarse AttackDetected target from traffic-usage.";
    }
    leaf top-n-sources {
      type uint16 { range "1..100"; }
      default 10;
      description "Maximum number of attacker source addresses ranked into TopSources by packet volume.";
    }
    leaf characterize-window {
      type uint16 { range "1..60"; }
      default 10;
      description "Seconds of recent flows to consider when characterizing; flows last seen before this window are ignored. Flows without a timestamp are always kept.";
    }
    leaf characterize-timeout {
      type uint16 { range "50..5000"; }
      default 2000;
      description "Milliseconds budget for the on-trigger traffic-usage and flow-recent queries; on timeout the detector falls back to the best available target.";
    }
    leaf entropy-threshold {
      type zt:decimal-2 { range "0.00..16.00"; }
      default "2.00";
      description "Source-address Shannon entropy (bits) at or above which an attack is logged as distributed/spoofed. 0 = a single source; higher = more sources.";
    }
  }
}
```

## Ddos Flowspec (`ddos-flowspec`, 1 plugins)

### ddos-flowspec

source: `internal/plugins/ddos/flowspec` -- config root: `ddos-flowspec`

DDoS FlowSpec/RTBH responder: upstream mitigation with leak-probe clear

`ze-ddos-flowspec-conf.yang`

```yang
module ze-ddos-flowspec-conf {
  namespace "urn:ze:ddos-flowspec-conf";
  prefix ddos-flowspec;

  container ddos-flowspec {
    leaf response-level {
      type enumeration {
        enum alert;
        enum enforce;
      }
      default alert;
      description "Action on attack detection: alert (log only) or enforce (announce FlowSpec).";
    }
    leaf action {
      type enumeration {
        enum rate-limit;
        enum discard;
      }
      default rate-limit;
      description "FlowSpec traffic-action: rate-limit (non-zero rate) or discard (rate 0).";
    }
    leaf hold-down {
      type uint32 { range "1..86400"; }
      default 300;
      description "Minimum seconds before the first leak-probe after announcement.";
    }
    leaf probe-interval {
      type uint16 { range "1..3600"; }
      default 60;
      description "Seconds between leak-probe attempts after hold-down.";
    }
    leaf probe-window {
      type uint16 { range "1..300"; }
      default 10;
      description "Seconds to observe leaked traffic during a probe.";
    }
    leaf probe-rate {
      type uint32 { range "1..max"; }
      default 1000000;
      description "Bits per second to allow during a leak-probe.";
    }
    leaf announce-rate-limit {
      type uint16 { range "1..600"; }
      default 10;
      description "Maximum FlowSpec announcements per minute.";
    }
    leaf max-mitigation-duration {
      type uint32 { range "0..604800"; }
      default 3600;
      description "Maximum seconds a FlowSpec rule stays announced (0 = no cap).";
    }
    leaf backoff-cap {
      type uint32 { range "1..604800"; }
      default 3600;
      description "Maximum hold-down after exponential backoff.";
    }
    leaf blackhole-fallback {
      type boolean;
      default false;
      description "When true, a critical-severity AttackDetected (peak >= 5x threshold) auto-engages an immediate upstream discard (RTBH-style) without waiting for characterization. When false (default) the upstream rule is announced only from AttackCharacterized, so it is precise before the box goes blind behind the filter.";
    }
    leaf-list allowlist {
      type string;
      description "Prefixes that must never be announced for mitigation.";
    }
  }
}
```

## Ddos Flowtriq (`ddos-flowtriq`, 1 plugins)

### ddos-flowtriq

source: `internal/plugins/ddos/flowtriq` -- config root: `ddos-flowtriq`

DDoS incident reporter for Flowtriq cloud API

`ze-ddos-flowtriq-conf.yang`

```yang
module ze-ddos-flowtriq-conf {
  namespace "urn:ze:ddos-flowtriq-conf";
  prefix ddos-flowtriq;

  container ddos-flowtriq {
    leaf enabled {
      type boolean;
      default false;
      description "Enable reporting DDoS incidents to the Flowtriq cloud API.";
    }
    leaf api-key {
      type string { length "1..512"; }
      description "Flowtriq API bearer token.";
    }
    leaf node-uuid {
      type string { length "1..128"; }
      description "Node UUID for Flowtriq agent identification.";
    }
    leaf api-base {
      type string;
      default "https://flowtriq.com/api/v1";
      description "Flowtriq API base URL.";
    }
  }
}
```

## Ddos Local (`ddos-local`, 1 plugins)

### ddos-local

source: `internal/plugins/ddos/local` -- config root: `ddos-local` -- depends on: `firewall`

DDoS local responder: on-host nft drop on attack detection

`ze-ddos-local-conf.yang`

```yang
module ze-ddos-local-conf {
  namespace "urn:ze:ddos-local-conf";
  prefix ddos-local;

  container ddos-local {
    leaf response-level {
      type enumeration {
        enum alert;
        enum enforce;
      }
      default alert;
      description "Action on attack detection: alert (log only) or enforce (install drop rule).";
    }
    leaf max-mitigation-duration {
      type uint32 { range "0..86400"; }
      default 3600;
      description "Maximum seconds a drop rule stays installed (0 = no cap).";
    }
    leaf-list allowlist {
      type string;
      description "Prefixes that must never be blocked.";
    }
  }
}
```

## Ddos Observe (`ddos-observe`, 1 plugins)

### ddos-observe

source: `internal/plugins/ddos/observe` -- config root: `ddos-observe`

DDoS observability: incident store, status CLI, doctor, metrics

`ze-ddos-observe-conf.yang`

```yang
module ze-ddos-observe-conf {
  namespace "urn:ze:ddos-observe-conf";
  prefix ddos-observe;

  container ddos-observe {
    leaf incident-ring-size {
      type uint32 { range "1..100000"; }
      default 1000;
      description "Maximum number of incidents to retain in memory.";
    }
    leaf stale-incident-timeout {
      type uint32 { range "1..86400"; }
      default 3600;
      description "Seconds before an open incident without a clear event is auto-finalized.";
    }
  }
}
```

## Environment (`environment`, 1 plugins)

### ntp

source: `internal/plugins/ntp` -- config root: `environment`

NTP client: system clock synchronization

`ze-ntp-cmd.yang`

```yang
module ze-ntp-cmd {
    namespace "urn:ze:ntp:cmd";
    prefix ntpcmd;
    import ze-extensions { prefix ze; }
    description "show system ntp command tree. Owned by the ntp plugin because the handlers read NTP peer and sync state. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-06 { description "Relocated show system ntp out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container system {
            config false;

            container ntp {
                config false;
                description "NTP clock synchronization status";

                ze:command "ze-show:system-ntp";

                container peers {
                    config false;
                    ze:command "ze-show:system-ntp-peers";
                    description "Show NTP peers with offset, RTT, stratum, and reachability.
Tells you whether your clock is synced and how far off each
NTP server thinks you are.";
                }
            }
        }
    }
}
```

`ze-ntp-conf.yang`

```yang
module ze-ntp-conf {
    namespace "urn:ze:ntp:conf";
    prefix ntp;

    description "NTP client configuration for Ze";

    revision 2026-01-01 {
        description "Initial revision";
    }

    container environment {
        description "Environment settings for NTP client";

        container ntp {
            description "NTP client settings";

            leaf enabled {
                type boolean;
                default false;
                description "Enable NTP time synchronization";
            }

            leaf interval {
                type uint32 {
                    range "60..86400";
                }
                default 3600;
                description "Sync interval in seconds";
            }

            leaf max-step {
                type uint32 {
                    range "0..86400";
                }
                default 3600;
                description
                    "Maximum accepted NTP clock step in seconds. A value
                     of 0 explicitly allows unlimited steps.";
            }

            leaf slew-threshold {
                type uint32 {
                    range "0..1000";
                }
                default 128;
                description
                    "Maximum offset in milliseconds for gradual clock slew
                     via Adjtimex. Offsets above this threshold use Settimeofday
                     (step). A value of 0 disables slew (always step).";
            }

            leaf persist-path {
                type string;
                default "/perm/ze/timefile";
                description "Path to save time on shutdown for recovery";
            }

            list server {
                key "name";
                description "NTP server pool entries";

                leaf name {
                    type string;
                    description "Server entry name (used as key)";
                }

                leaf address {
                    type string;
                    description "NTP server hostname or IP address";
                }
            }
        }
    }
}
```

## Fib / Kernel (`fib/kernel`, 1 plugins)

### fib-kernel

source: `internal/plugins/fib/kernel` -- config root: `fib/kernel` -- depends on: `rib, sysctl`

FIB kernel: programs OS routes from system RIB via netlink/route socket

`ze-fib-conf.yang`

```yang
module ze-fib-conf {
    namespace "urn:ze:fib:conf";
    prefix fib;

    description
        "FIB configuration for Ze.
         Contains the kernel backend configuration for route programming.
         Admin distance settings are in ze-rib-conf (RIB concept).";

    revision 2026-04-04 {
        description "Move admin-distance to ze-rib-conf.";
    }

    container fib {
        description "Forwarding Information Base configuration.";

        container kernel {
            description "OS kernel route programming via netlink (Linux).";

            leaf flush-on-stop {
                type boolean;
                default false;
                description
                    "Remove all ze-installed routes when the plugin stops.
                     When false (default), routes persist in the kernel
                     after shutdown for graceful restart support.";
            }

            leaf sweep-delay {
                type uint16;
                default 30;
                units seconds;
                description
                    "Time to wait after startup before sweeping stale routes.
                     Allows BGP reconvergence to refresh matching routes.";
            }
        }
    }
}
```

## Fib / P4 (`fib/p4`, 1 plugins)

### fib-p4

source: `internal/plugins/fib/p4` -- config root: `fib/p4` -- depends on: `rib`

FIB P4: programs P4 switch forwarding entries from system RIB via gRPC/P4Runtime

`ze-fib-p4-conf.yang`

```yang
module ze-fib-p4-conf {
    namespace "urn:ze:fib:p4:conf";
    prefix fibp4;

    import ze-fib-conf { prefix fib; }

    description
        "FIB P4 backend configuration for Ze.
         Augments the fib container with P4 switch programming
         via gRPC/P4Runtime.";

    revision 2026-04-04 {
        description "Initial revision.";
    }

    augment "/fib:fib" {
        container p4 {
            description "P4 switch route programming via gRPC/P4Runtime.";

            leaf target {
                type string;
                description
                    "P4Runtime gRPC target address (host:port).
                     Example: 127.0.0.1:9559";
            }

            leaf device-id {
                type uint64;
                default 1;
                description "P4Runtime device ID.";
            }

            leaf flush-on-stop {
                type boolean;
                default false;
                description
                    "Remove all forwarding entries when the plugin stops.";
            }
        }
    }
}
```

## Fib / Vpp (`fib/vpp`, 1 plugins)

### fib-vpp

source: `internal/plugins/fib/vpp` -- config root: `fib/vpp` -- depends on: `rib, vpp`

FIB VPP: programs VPP FIB entries from system RIB via GoVPP binary API

`ze-fib-vpp-conf.yang`

```yang
module ze-fib-vpp-conf {
    namespace "urn:ze:fib:vpp:conf";
    prefix fibvpp;

    import ze-fib-conf { prefix fib; }

    description
        "FIB VPP backend configuration for Ze.
         Augments the fib container with VPP FIB programming
         via GoVPP binary API (IPRouteAddDel).";

    revision 2026-04-14 {
        description "Initial revision.";
    }

    augment "/fib:fib" {
        container vpp {
            description "VPP FIB programming via GoVPP binary API.";

            leaf enabled {
                type boolean;
                default false;
                description
                    "Enable VPP FIB programming. Routes from system RIB
                     are programmed directly into VPP's FIB.";
            }

            leaf table-id {
                type uint32;
                default 0;
                description
                    "VRF table ID for route programming.
                     0 is the default VRF.";
            }

            leaf batch-size {
                type uint16;
                default 256;
                description
                    "Maximum number of routes per batch dispatch.";
            }

            leaf batch-interval-ms {
                type uint16;
                default 10;
                description
                    "Maximum time in milliseconds before dispatching
                     a partial batch.";
            }
        }
    }
}
```

## Firewall (`firewall`, 3 plugins)

### firewall

source: `internal/component/firewall` -- config root: `firewall`

Packet filter and NAT rules (nftables on Linux)

`ze-firewall-cmd.yang`

```yang
module ze-firewall-cmd {
    namespace "urn:ze:firewall:cmd";
    prefix firewallcmd;
    import ze-extensions { prefix ze; }
    description "Firewall show commands: ruleset inspection and address/port group listing.
         Relocated from the central show schema for plugin self-containment.";
    revision 2026-06-03 { description "Initial revision: relocated from ze-cli-show-cmd."; }
    revision 2026-06-06 { description "Standalone containers instead of augment (avoids duplicate system container)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container firewall {
            config false;
            description "Firewall rules and groups (nftables)";
            container ruleset {
                config false;
                ze:command "ze-show:firewall-ruleset";
                ze:backend "nft";
                description "Show the live firewall ruleset with per-term counters.
Usage: show firewall ruleset <name>. Joins applied desired state with
kernel counters from the nft backend.";
            }
            container group {
                config false;
                ze:command "ze-show:firewall-group";
                ze:backend "nft";
                description "Show members of a firewall address/port group.
Without arguments, lists all known groups. With a name, shows the
set elements. Reads from the last applied config, not the kernel.";
            }
        }

        container system {
            config false;
            container conntrack {
                config false;
                ze:command "ze-show:system-conntrack";
                ze:backend "nft";
                description "Show the kernel connection tracking table.
Returns conntrack entry count, table size, timeouts, and loaded
modules. Requires the nft backend. Check this when you suspect
conntrack table exhaustion is dropping traffic.";
            }
        }
    }
}
```

`ze-firewall-conf.yang`

```yang
module ze-firewall-conf {
    namespace "urn:ze:firewall:conf";
    prefix fw;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description "Firewall configuration for Ze.
                 Tables use nftables structural concepts (table, chain, hook,
                 priority, policy, set, flowtable) with readable keyword names.
                 Table names in config are bare; ze_ prefix added by component.";

    revision 2026-04-13 {
        description "Initial revision";
    }

    typedef table-family {
        type enumeration {
            enum inet { description "Dual-stack IPv4+IPv6"; }
            enum ip { description "IPv4 only"; }
            enum ip6 { description "IPv6 only"; }
            enum arp { description "ARP"; }
            enum bridge { description "Bridge"; }
            enum netdev { description "Netdev"; }
        }
        description "nftables address family";
    }

    typedef chain-type {
        type enumeration {
            enum filter { description "Packet filtering"; }
            enum nat { description "Network address translation"; }
            enum route { description "Routing decision override"; }
        }
        description "nftables chain type";
    }

    typedef chain-hook {
        type enumeration {
            enum input { description "Input hook"; }
            enum output { description "Output hook"; }
            enum forward { description "Forward hook"; }
            enum prerouting { description "Prerouting hook"; }
            enum postrouting { description "Postrouting hook"; }
            enum ingress { description "Ingress hook (netdev/inet)"; }
            enum egress { description "Egress hook (netdev)"; }
        }
        description "Netfilter hook point";
    }

    typedef chain-policy {
        type enumeration {
            enum accept { description "Accept by default"; }
            enum drop { description "Drop by default"; }
        }
        description "Default policy for base chains";
    }

    typedef set-type {
        type enumeration {
            enum ipv4 { description "IPv4 addresses"; }
            enum ipv6 { description "IPv6 addresses"; }
            enum ether { description "Ethernet addresses"; }
            enum inet-service { description "TCP/UDP ports"; }
            enum mark { description "Packet marks"; }
            enum ifname { description "Interface names"; }
        }
        description "Named set element data type";
    }

    typedef protocol-name {
        type enumeration {
            enum tcp;
            enum udp;
            enum icmp;
            enum icmpv6;
            enum sctp;
            enum gre;
            enum esp;
            enum ah;
            enum ospf;
            enum vrrp;
        }
        description "L4 protocol name";
    }

    typedef connection-state {
        type string {
            pattern '(new|established|related|invalid)'
                  + '(,(new|established|related|invalid))*';
        }
        description "Comma-separated connection tracking states";
    }

    typedef reject-type {
        type enumeration {
            enum icmp;
            enum icmpv6;
            enum tcp-reset;
        }
        description "Reject response type";
    }

    typedef rate-unit {
        type enumeration {
            enum second;
            enum minute;
            enum hour;
            enum day;
        }
        description "Rate limit time unit";
    }

    typedef dscp-value {
        type string {
            pattern '[a-z0-9]+';
        }
        description "DSCP value: numeric (0-63) or symbolic (ef, af41, cs6, etc.)";
    }

    typedef ip-prefix {
        type string;
        description "IPv4 or IPv6 prefix in CIDR notation, or @set-name reference";
    }

    typedef port-spec {
        type string {
            pattern '[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*|@[a-zA-Z0-9][a-zA-Z0-9_-]*';
        }
        description "Port number, range (80-90), comma-separated list (80,443,8080-8090), or named-set reference (@set-name)";
    }

    typedef mark-value {
        type string {
            pattern '0x[0-9a-fA-F]+(/0x[0-9a-fA-F]+)?|[0-9]+(/[0-9]+)?';
        }
        description "Mark value with optional mask (0x10/0xff or 16/255)";
    }

    typedef rate-spec {
        type string {
            pattern '[0-9]+(bytes|kbytes|mbytes|gbytes)?/(second|minute|hour|day)';
        }
        description "Rate limit specification: packet-per-unit (10/second)
                     or byte-per-unit (1mbytes/second, 500kbytes/minute). Byte
                     prefixes scale: kbytes=1024, mbytes=1024^2, gbytes=1024^3.";
    }

    typedef nat-spec {
        type string;
        description "NAT target: address with optional port (1.2.3.4:80 or 1.2.3.4:1024-2048)";
    }

    grouping from-block {
        description "Match criteria (from block in a term)";

        leaf source-address {
            type ip-prefix;
            description "Source IP prefix or @set-name";
        }
        leaf destination-address {
            type ip-prefix;
            description "Destination IP prefix or @set-name";
        }
        leaf source-port {
            type port-spec;
            description "Source port, range, or list";
        }
        leaf destination-port {
            type port-spec;
            description "Destination port, range, or list";
        }
        leaf protocol {
            type protocol-name;
            description "L4 protocol";
        }
        leaf input-interface {
            type string;
            description "Input interface name. A trailing asterisk
                         (e.g. 'l2tp*') matches any interface whose
                         name begins with the given prefix.";
        }
        leaf output-interface {
            type string;
            description "Output interface name. A trailing asterisk
                         (e.g. 'veth*') matches any interface whose
                         name begins with the given prefix.";
        }
        leaf connection-state {
            type connection-state;
            ze:backend "nft";
            description "Connection tracking state(s). Conntrack-driven;
                         requires a backend that integrates with the kernel
                         nf_conntrack module.";
        }
        leaf connection-mark {
            type mark-value;
            ze:backend "nft";
            description "Connection mark value/mask. Conntrack-driven; see
                         connection-state for backend notes.";
        }
        leaf mark {
            type mark-value;
            description "Packet mark value/mask";
        }
        leaf dscp {
            type dscp-value;
            description "DSCP value";
        }
        leaf icmp-type {
            type string;
            description "ICMPv4 type. Accepts symbolic names from
                         nftables (echo-request, echo-reply,
                         destination-unreachable, time-exceeded, ...)
                         or a numeric byte value 0..255.";
        }
        leaf icmpv6-type {
            type string;
            description "ICMPv6 type. Accepts symbolic names from
                         nftables (echo-request, nd-neighbor-solicit,
                         packet-too-big, ...) or a numeric byte value
                         0..255.";
        }
    }

    grouping then-block {
        description "Action and modifier expressions (then block in a term)";

        container accept {
            presence "Accept the packet";
            description "Accept the packet";
        }
        container drop {
            presence "Drop the packet";
            description "Drop the packet";
        }
        container reject {
            presence "Reject the packet";
            leaf with {
                type reject-type;
                description "Reject response type";
            }
            leaf code {
                type uint8;
                description "ICMP code";
            }
        }
        container return {
            presence "Return to caller chain";
            description "Return to caller chain";
        }
        container exclude {
            presence "Skip NAT for matching traffic";
            description "In a NAT chain term, skip NAT for matching
                         traffic. Lowers to the same `return` verdict
                         the operator could have written directly;
                         the keyword exists for VyOS-config parity.";
        }
        leaf jump {
            type string;
            description "Jump to target chain";
        }
        leaf goto {
            type string;
            description "Goto target chain (no return)";
        }
        container snat {
            presence "Source NAT";
            leaf to {
                type nat-spec;
                mandatory true;
                description "NAT target address:port";
            }
        }
        container dnat {
            presence "Destination NAT";
            leaf to {
                type nat-spec;
                mandatory true;
                description "NAT target address:port";
            }
        }
        container masquerade {
            presence "Masquerade outgoing traffic";
            description "Masquerade (source NAT with outgoing interface address).
                         Optionally restrict source port range with port-range,
                         or set randomization/persistence flags. Port mapping
                         and flags are mutually exclusive.";
            leaf port-range {
                type string;
                description "Source port or port range (e.g. 1024-65535)";
            }
            container random {
                presence "Randomize source port mapping";
                description "Randomize source port selection.
                             With 'full', use full randomization.";
                leaf full {
                    type empty;
                    description "Use full randomization";
                }
            }
            leaf persistent {
                type empty;
                description "Use consistent source port mapping per connection";
            }
        }
        container redirect {
            presence "Redirect to local port";
            leaf to {
                type zt:port;
                mandatory true;
                description "Local port number";
            }
        }
        container notrack {
            presence "Disable connection tracking";
            ze:backend "nft";
            description "Disable connection tracking. Conntrack-specific.";
        }
        container flow-offload {
            presence "Hardware flow offload";
            ze:backend "nft";
            description "Hardware / software flow offload via an nftables
                         flowtable. The VPP dataplane is itself the fast
                         path and does not expose a flowtable surface.";
            leaf flowtable {
                type string;
                mandatory true;
                description "Flowtable name (prefixed with @)";
            }
        }
        container mark-set {
            presence "Set packet mark";
            ze:backend "nft";
            description "Write the kernel skb mark. Different backends
                         carry per-packet metadata differently; the VPP
                         classifier's equivalent uses opaque keys via
                         ClassifyAddDelSession.";
            leaf value {
                type mark-value;
                mandatory true;
                description "Mark value with optional mask";
            }
        }
        container connection-mark-set {
            presence "Set connection mark";
            ze:backend "nft";
            description "Conntrack-driven; see mark-set for backend notes.";
            leaf value {
                type mark-value;
                mandatory true;
                description "Mark value with optional mask";
            }
        }
        leaf dscp-set {
            type dscp-value;
            description "Set DSCP field value";
        }
        leaf tcp-mss-set {
            type uint16 {
                range "1..65535";
            }
            description "Clamp TCP Maximum Segment Size option to the given value";
        }
        container counter {
            presence "Enable per-rule counter";
            description "Anonymous per-rule counter. A non-empty `name`
                         leaf is reserved for a future named-counter
                         feature and is rejected at verify today.";
            leaf name {
                type string;
                description "Counter name (reserved; non-empty values reject at verify)";
            }
        }
        container log {
            presence "Log the packet";
            leaf prefix {
                type string {
                    length "0..127";
                }
                description "Log prefix string";
            }
            leaf level {
                type uint32;
                description "Log level (syslog severity)";
            }
        }
        container limit-rate {
            presence "Rate limit";
            leaf rate {
                type rate-spec;
                mandatory true;
                description "Rate limit (e.g., 10/second)";
            }
            leaf burst {
                type uint32;
                description "Burst size";
            }
        }
    }

    container firewall {
        description "Ze-managed nftables firewall tables.
                     Table names are bare in config; ze_ prefix added by component.";

        leaf backend {
            type string;
            default "nft";
            description "Firewall backend implementation. Default is nft
                         (nftables via libnftnl). Future backends can declare
                         themselves via firewall.RegisterBackend. The
                         ze:backend YANG extension on feature nodes declares
                         per-feature backend support so the commit-time gate
                         rejects configs that try to use unsupported
                         primitives.";
        }

        container global-options {
            description "Network security defaults mapped to kernel sysctls.
                         Provides VyOS-compatible keyword toggles for common
                         settings. At apply time each keyword translates to a
                         sysctl key/value pair emitted to the sysctl plugin.
                         Explicit sysctl settings always override these.";

            leaf all-ping {
                type enumeration {
                    enum enable {
                        description "Allow ICMP echo replies (icmp_echo_ignore_all=0)";
                    }
                    enum disable {
                        description "Ignore all ICMP echo requests (icmp_echo_ignore_all=1)";
                    }
                }
                description "Control ICMP echo (ping) responses.
                             Inverted sysctl: enable sets icmp_echo_ignore_all to 0.";
            }

            leaf broadcast-ping {
                type enumeration {
                    enum enable {
                        description "Allow broadcast ICMP echo (icmp_echo_ignore_broadcasts=0)";
                    }
                    enum disable {
                        description "Ignore broadcast ICMP echo (icmp_echo_ignore_broadcasts=1)";
                    }
                }
                description "Control broadcast ICMP echo responses.
                             Inverted sysctl: enable sets icmp_echo_ignore_broadcasts to 0.";
            }

            leaf syn-cookies {
                type enumeration {
                    enum enable {
                        description "Enable TCP SYN cookies (tcp_syncookies=1)";
                    }
                    enum disable {
                        description "Disable TCP SYN cookies (tcp_syncookies=0)";
                    }
                }
                description "TCP SYN cookie protection against SYN flood attacks.";
            }

            leaf receive-redirects {
                type enumeration {
                    enum enable {
                        description "Accept ICMP redirects (accept_redirects=1)";
                    }
                    enum disable {
                        description "Reject ICMP redirects (accept_redirects=0)";
                    }
                }
                description "Control acceptance of IPv4 ICMP redirect messages.";
            }

            leaf send-redirects {
                type enumeration {
                    enum enable {
                        description "Send ICMP redirects (send_redirects=1)";
                    }
                    enum disable {
                        description "Do not send ICMP redirects (send_redirects=0)";
                    }
                }
                description "Control sending of IPv4 ICMP redirect messages.";
            }

            leaf source-validation {
                type enumeration {
                    enum disable {
                        description "No source address validation (rp_filter=0)";
                    }
                    enum strict {
                        description "Strict reverse path filtering (rp_filter=1)";
                    }
                    enum loose {
                        description "Loose reverse path filtering (rp_filter=2)";
                    }
                }
                description "Reverse path filtering mode for source address validation.";
            }

            leaf log-martians {
                type enumeration {
                    enum enable {
                        description "Log martian packets (log_martians=1)";
                    }
                    enum disable {
                        description "Do not log martian packets (log_martians=0)";
                    }
                }
                description "Log packets with impossible source addresses.";
            }

            leaf ipv6-receive-redirects {
                type enumeration {
                    enum enable {
                        description "Accept IPv6 ICMP redirects (accept_redirects=1)";
                    }
                    enum disable {
                        description "Reject IPv6 ICMP redirects (accept_redirects=0)";
                    }
                }
                description "Control acceptance of IPv6 ICMP redirect messages.";
            }

            leaf ipv6-src-route {
                type enumeration {
                    enum enable {
                        description "Accept IPv6 source-routed packets (accept_source_route=1)";
                    }
                    enum disable {
                        description "Reject IPv6 source-routed packets (accept_source_route=0)";
                    }
                }
                description "Control acceptance of IPv6 packets with source routing headers.";
            }
        }

        list table {
            key "name";
            description "Named firewall table";

            leaf name {
                type string {
                    length "1..255";
                    pattern '[a-zA-Z0-9][a-zA-Z0-9_-]*';
                }
                description "Table name (becomes ze_<name> in kernel)";
            }

            leaf family {
                type table-family;
                mandatory true;
                description "Address family";
            }

            list chain {
                key "name";
                description "Named chain within table";

                leaf name {
                    type string {
                        length "1..255";
                        pattern '[a-zA-Z0-9][a-zA-Z0-9_-]*';
                    }
                    description "Chain name";
                }

                leaf type {
                    type chain-type;
                    description "Chain type (base chains only)";
                }

                leaf hook {
                    type chain-hook;
                    description "Netfilter hook point (base chains only)";
                }

                leaf priority {
                    type int32;
                    description "Chain priority (base chains only)";
                }

                leaf policy {
                    type chain-policy;
                    description "Default policy (base chains only)";
                }

                list term {
                    key "name";
                    ordered-by user;
                    description "Named rule (from/then structure)";

                    leaf name {
                        type string {
                            length "1..255";
                            pattern '[a-zA-Z0-9][a-zA-Z0-9_-]*';
                        }
                        description "Term name (used in counters and logging)";
                    }

                    container from {
                        uses from-block;
                        description "Match criteria";
                    }

                    container then {
                        uses then-block;
                        description "Actions and modifiers";
                    }
                }
            }

            list set {
                key "name";
                description "Named set";

                leaf name {
                    type string {
                        length "1..255";
                        pattern '[a-zA-Z0-9][a-zA-Z0-9_-]*';
                    }
                    description "Set name";
                }

                leaf type {
                    type set-type;
                    mandatory true;
                    description "Element data type";
                }

                container flags-interval {
                    presence "Enable interval ranges";
                    description "Enable interval ranges";
                }

                container flags-timeout {
                    presence "Enable per-element timeouts";
                    description "Enable per-element timeouts";
                }

                container flags-constant {
                    presence "Immutable after creation";
                    description "Immutable after creation";
                }

                container flags-dynamic {
                    presence "Dynamically populated";
                    description "Dynamically populated";
                }

                list element {
                    key "value";
                    ordered-by user;
                    description "Static set elements. The value form is
                                 interpreted per set type: ipv4 / ipv6
                                 accepts addresses or prefixes (when
                                 flags-interval is set); inet-service
                                 accepts port numbers; mark accepts a
                                 numeric or 0x-prefixed value; ether
                                 accepts aa:bb:cc:dd:ee:ff; ifname
                                 accepts an interface name.";

                    leaf value {
                        type string;
                        description "Element value (see list description for format).";
                    }

                    leaf timeout {
                        type uint32;
                        description "Per-element timeout in seconds. 0 means no timeout.
                                     Requires flags-timeout on the enclosing set.";
                    }
                }
            }

            list flowtable {
                key "name";
                ze:backend "nft";
                description "Hardware offload flowtable. nftables-specific;
                             the VPP dataplane is itself the fast path and
                             does not expose a flowtable surface.";

                leaf name {
                    type string {
                        length "1..255";
                        pattern '[a-zA-Z0-9][a-zA-Z0-9_-]*';
                    }
                    description "Flowtable name";
                }

                leaf hook {
                    type chain-hook;
                    mandatory true;
                    description "Hook point (typically ingress)";
                }

                leaf priority {
                    type int32;
                    mandatory true;
                    description "Priority (typically negative)";
                }

                leaf-list device {
                    type string;
                    description "Devices for offload";
                }
            }
        }
    }
}
```

### firewall-irr

source: `internal/component/firewall/plugins/irr` -- config root: `firewall` -- depends on: `firewall`

IRR-based prefix-list filtering for firewall rules

`ze-firewall-irr-cmd.yang`

```yang
module ze-firewall-irr-cmd {
    namespace "urn:ze:firewall-irr:cmd";
    prefix fwirrcmd;
    import ze-extensions { prefix ze; }
    description "show firewall irr ... and update firewall irr ... command tree.
                 Owned by the firewall-irr plugin so that removing the plugin
                 directory removes these command nodes together with the handlers
                 and config schema. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-17 { description "Initial revision."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container firewall {
            config false;
            // Repeat the canonical description owned by ze-firewall-cmd.yang
            // verbatim so the command-tree merge does not warn on a mismatch.
            // This module only attaches the `irr` subtree to the shared node.
            description "Firewall rules and groups (nftables)";

            container irr {
                config false;
                ze:command "ze-show:firewall-irr-status";
                description "Show IRR filter status for all cached ASN/AS-SET entries.
Lists each cached entry with prefix counts, last refresh time, and
error status. Use this to confirm that IRR prefix-lists are loaded
and current before committing firewall config.";

                container prefix {
                    config false;
                    ze:command "ze-show:firewall-irr-prefix";
                    description "Show IRR-resolved prefixes for a cached entry.
Usage: show firewall irr prefix <asn-or-as-set>. Lists all IPv4 and
IPv6 prefixes in the cached prefix-list for the given ASN or AS-SET.";
                    leaf name { type string; mandatory true; description "ASN or AS-SET name"; }
                }
            }
        }
    }

    container update {
        config false;

        container firewall {
            config false;
            description "Firewall operational updates";

            container irr {
                config false;
                description "IRR prefix-list fetch and refresh commands";

                container all {
                    config false;
                    ze:command "ze-update:firewall-irr-all";
                    description "Refresh all cached IRR prefix-lists.
Re-queries the IRR server for every cached ASN/AS-SET entry and
updates the zefs cache on success. Failed refreshes preserve the
existing cache and report an error.";
                }

                container asn {
                    config false;
                    ze:command "ze-update:firewall-irr-asn";
                    description "Fetch or refresh IRR prefix-list for an ASN.
Usage: update firewall irr asn <asn>. Queries the IRR server and
saves resolved prefixes to the zefs cache. Creates the cache entry
if it does not exist.";
                    leaf asn { type string; mandatory true; description "ASN number"; }
                }

                container as-set {
                    config false;
                    ze:command "ze-update:firewall-irr-as-set";
                    description "Fetch or refresh IRR prefix-list for an AS-SET.
Usage: update firewall irr as-set <as-set>. Queries the IRR server
and saves resolved prefixes to the zefs cache.";
                    leaf as-set { type string; mandatory true; description "AS-SET name"; }
                }
            }
        }
    }
}
```

`ze-firewall-irr.yang`

```yang
module ze-firewall-irr {
    namespace "urn:ze:firewall-irr";
    prefix fwirr;

    import ze-firewall-conf { prefix fw; }

    description "IRR-based source/destination address filtering for the firewall.
                 Resolves ASN or AS-SET references to prefix lists via the IRR whois
                 client and populates nftables interval sets. Augments the firewall
                 from-block with source-asn, source-as-set, destination-asn, and
                 destination-as-set leaves, and adds a firewall irr policy container
                 for server, refresh-interval, and peeringdb-url settings.";

    revision 2026-06-17 {
        description "Initial revision";
    }

    augment "/fw:firewall" {
        container irr {
            description "IRR policy settings for firewall prefix-list resolution.";

            leaf server {
                type string;
                default "whois.radb.net";
                description "IRR whois server hostname or host:port.";
            }

            leaf peeringdb-url {
                type string {
                    pattern 'https?://.*';
                }
                default "https://www.peeringdb.com";
                description "Base URL for PeeringDB API queries.
                             Override for testing with a mock server.";
            }

            leaf refresh-interval {
                type uint32 {
                    range "0 | 60..86400";
                }
                default "0";
                units "seconds";
                description "Seconds between automatic IRR re-queries.
                             0 (default) disables automatic refresh; the operator
                             must use 'update firewall irr' manually. A value in
                             60..86400 enables periodic refresh with fail-closed
                             semantics (failed refresh preserves last-good cache).";
            }

            list interface {
                key "name";
                description "Per-interface AS-SET binding for ingress source
                             validation. Packets arriving on this interface with
                             source addresses not in the AS-SET's IRR-resolved
                             prefixes are dropped.";

                leaf name {
                    type string {
                        length "1..15";
                        pattern '[a-zA-Z0-9._-]+';
                    }
                    description "Interface name (e.g. eth1, bond0.100).
                                 Must match the kernel interface name exactly.";
                }

                leaf source-as-set {
                    type string {
                        length "1..255";
                        pattern '[A-Za-z0-9:._-]+';
                    }
                    mandatory true;
                    description "AS-SET whose IRR-resolved prefixes form the
                                 allowed source-address set for this interface.
                                 Requires cached prefix data; config commit
                                 rejects if no cached data exists.";
                }
            }
        }
    }

    augment "/fw:firewall/fw:table/fw:chain/fw:term/fw:from" {
        leaf source-asn {
            type uint32 {
                range "1..4294967294";
            }
            description "Match source address against IRR-resolved prefixes for
                         this ASN. Requires cached prefix data (run 'update
                         firewall irr asn <N>' first). Config commit rejects
                         if no cached data exists.";
        }

        leaf source-as-set {
            type string {
                length "1..255";
                pattern '[A-Za-z0-9:._-]+';
            }
            description "Match source address against IRR-resolved prefixes for
                         this AS-SET. Requires cached prefix data (run 'update
                         firewall irr as-set <name>' first). Config commit
                         rejects if no cached data exists.";
        }

        leaf destination-asn {
            type uint32 {
                range "1..4294967294";
            }
            description "Match destination address against IRR-resolved prefixes
                         for this ASN. Same caching semantics as source-asn.";
        }

        leaf destination-as-set {
            type string {
                length "1..255";
                pattern '[A-Za-z0-9:._-]+';
            }
            description "Match destination address against IRR-resolved prefixes
                         for this AS-SET. Same caching semantics as source-as-set.";
        }
    }
}
```

### flowspec-firewall

source: `internal/plugins/flowspec-firewall` -- depends on: `firewall`

Translates BGP FlowSpec routes into nftables firewall rules

No YANG module of its own (reads config defined by another plugin, or has none).

## Flow Export (`flow-export`, 1 plugins)

### flow-export

source: `internal/plugins/flowexport` -- config root: `flow-export` -- depends on: `interface`

sFlow, NetFlow v9, and IPFIX counter export

`ze-flowexport-conf.yang`

```yang
module ze-flowexport-conf {
    namespace "urn:ze:flowexport:conf";
    prefix flowexport;

    import ze-types { prefix zt; }

    container flow-export {
        description "Flow export (sFlow, NetFlow v9, IPFIX) configuration";

        list collector {
            key "name";
            description "A flow export collector endpoint";

            leaf name {
                type string;
                description "Collector name";
            }

            leaf address {
                type zt:ip-address;
                mandatory true;
                description "Collector IP address";
            }

            leaf port {
                type uint16 {
                    range "1..65535";
                }
                default 6343;
                description "Collector UDP port";
            }

            leaf protocol {
                type enumeration {
                    enum sflow {
                        description "sFlow v5";
                    }
                    enum netflow9 {
                        description "NetFlow v9 (RFC 3954)";
                    }
                    enum ipfix {
                        description "IPFIX (RFC 7011)";
                    }
                }
                mandatory true;
                description "Export protocol";
            }

            leaf polling-interval {
                type uint16 {
                    range "1..3600";
                }
                default 20;
                description "Counter polling interval in seconds";
            }

            leaf template-refresh {
                type uint32 {
                    range "1..86400";
                }
                default 600;
                description "Template refresh interval in seconds (NetFlow v9, IPFIX)";
            }

            leaf sub-agent-id {
                type uint32;
                default 0;
                description "sFlow sub-agent identifier";
            }

            leaf observation-domain {
                type uint32;
                default 0;
                description "IPFIX/NetFlow v9 observation domain ID";
            }

            leaf agent-address {
                type zt:ip-address;
                description "sFlow agent address (device's own stable IP, e.g. loopback)";
            }
        }

        container sampling {
            description "Packet sampling (tc sample + psample) exported as sFlow flow samples";

            list interface {
                key "name";
                description "Per-interface packet sampling configuration";

                leaf name {
                    type string;
                    description "Interface to sample";
                }

                leaf rate {
                    type uint32 {
                        range "1..1000000";
                    }
                    description "Sampling rate (1-in-N packets)";
                }

                leaf trunc-size {
                    type uint16 {
                        range "64..1500";
                    }
                    default 128;
                    description "Bytes of each sampled packet header to capture";
                }

                leaf group {
                    type uint32 {
                        range "1..2147483647";
                    }
                    default 1;
                    description "psample group ID";
                }
            }
        }

        container conntrack {
            description "Per-flow record export from conntrack (NetFlow v9, IPFIX)";

            leaf enabled {
                type boolean;
                default false;
                description "Export per-flow records from the conntrack table";
            }

            leaf active-timeout {
                type uint16 {
                    range "1..3600";
                }
                default 60;
                description "Seconds between conntrack table dumps";
            }

            leaf recent-flow-ring {
                type uint32 {
                    range "64..65536";
                }
                default 4096;
                description "Capacity (in flow records) of the recent-flow ring queried by 'show flow-recent'. The ring feeds on-box DDoS characterization; larger values retain more history at higher memory cost. Only allocated when conntrack export is enabled.";
            }
        }

        container enrichment {
            description "Flow record enrichment from the BGP RIB";

            leaf bgp {
                type boolean;
                default false;
                description "Enrich flow records with BGP next-hop (and AS where available)";
            }
        }
    }
}
```

## Interface (`interface`, 2 plugins)

### interface

source: `internal/component/iface` -- config root: `interface` -- depends on: `sysctl`

OS network interface monitoring and management

`ze-iface-api.yang`

```yang
module ze-iface-api {
    namespace "urn:ze:iface:api";
    prefix ifaceapi;
    description "Interface management RPC definitions with typed parameters";
    revision 2026-04-04 { description "Initial revision"; }

    rpc interface-create-dummy {
        description "Create a dummy (loopback-like) interface";
        input { leaf name { type string; mandatory true; description "Interface name"; } }
    }
    rpc interface-create-veth {
        description "Create a veth pair";
        input {
            leaf name { type string; mandatory true; description "Interface name"; }
            leaf peer { type string; mandatory true; description "Peer interface name"; }
        }
    }
    rpc interface-delete {
        description "Delete an interface";
        input { leaf name { type string; mandatory true; description "Interface name"; } }
    }
    rpc interface-addr-add {
        description "Add an IP address to an interface";
        input {
            leaf name { type string; mandatory true; description "Interface name"; }
            leaf address { type string; mandatory true; description "IP address in CIDR notation (e.g. 10.0.0.1/24)"; }
        }
    }
    rpc interface-addr-del {
        description "Remove an IP address from an interface";
        input {
            leaf name { type string; mandatory true; description "Interface name"; }
            leaf address { type string; mandatory true; description "IP address in CIDR notation"; }
        }
    }
    rpc interface-unit-add {
        description "Add a VLAN logical unit to an interface";
        input {
            leaf name { type string; mandatory true; description "Parent interface name"; }
            leaf vlan-id { type uint16; mandatory true; description "VLAN ID (1-4094)"; }
        }
    }
    rpc interface-unit-del {
        description "Delete a logical unit from an interface";
        input { leaf name { type string; mandatory true; description "Interface name (e.g. eth0.100)"; } }
    }
    rpc interface-migrate {
        description "Make-before-break IP migration between interfaces";
        input {
            leaf from { type string; mandatory true; description "Source interface.unit (e.g. eth0.0)"; }
            leaf to { type string; mandatory true; description "Destination interface.unit (e.g. eth1.0)"; }
            leaf address { type string; mandatory true; description "IP address to migrate (CIDR)"; }
        }
    }
}
```

`ze-iface-cmd.yang`

```yang
module ze-iface-cmd {
    namespace "urn:ze:iface:cmd";
    prefix ifacecmd;
    import ze-extensions { prefix ze; }
    description "Create, configure, and manage network interfaces";
    revision 2026-06-04 { description "Verb-first grammar: create/delete/request interface"; }

    container create {
        config false;
        description "Create new resources";

        container interface {
            config false;
            description "Create interfaces, units, and addresses";

            container dummy {
                config false;
                ze:command "ze-iface:interface-create-dummy";
                ze:backend "netlink";
                ze:ensure-exists "ze-iface:interface-delete";
                description "Create a dummy (loopback-style) interface.
Usage: create interface dummy <name>.";
                leaf name { type string; mandatory true; description "Interface name"; }

                container unit {
                    config false;
                    ze:command "ze-iface:interface-unit-add";
                    ze:backend "netlink";
                    description "Create a dummy interface (if needed) and add a VLAN sub-interface.
Usage: create interface dummy <name> unit <vid>.";
                    leaf name { type string; mandatory true; description "Parent interface name"; }
                }

                container address {
                    config false;
                    ze:command "ze-iface:interface-addr-add";
                    ze:backend "netlink";
                    description "Create a dummy interface (if needed) and add an IP address.
Usage: create interface dummy <name> address <prefix>.";
                    leaf name { type string; mandatory true; description "Interface name"; }
                }
            }

            container veth {
                config false;
                ze:command "ze-iface:interface-create-veth";
                ze:backend "netlink";
                description "Create a veth pair (two linked virtual Ethernet interfaces).
Usage: create interface veth <name> <peer>.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }

            container bridge {
                config false;
                ze:command "ze-iface:interface-create-bridge";
                ze:backend "netlink";
                ze:ensure-exists "ze-iface:interface-delete";
                description "Create a Linux bridge for L2 forwarding.
Usage: create interface bridge <name>.";
                leaf name { type string; mandatory true; description "Interface name"; }

                container unit {
                    config false;
                    ze:command "ze-iface:interface-unit-add";
                    ze:backend "netlink";
                    description "Create a bridge (if needed) and add a VLAN sub-interface.
Usage: create interface bridge <name> unit <vid>.";
                    leaf name { type string; mandatory true; description "Parent interface name"; }
                }

                container address {
                    config false;
                    ze:command "ze-iface:interface-addr-add";
                    ze:backend "netlink";
                    description "Create a bridge (if needed) and add an IP address.
Usage: create interface bridge <name> address <prefix>.";
                    leaf name { type string; mandatory true; description "Interface name"; }
                }
            }

            container unit {
                config false;
                ze:command "ze-iface:interface-unit-add";
                description "Add a VLAN sub-interface (802.1Q tagged).
Usage: create interface <parent> unit <vid>. Parent must already exist.";
                leaf name { type string; mandatory true; description "Parent interface name"; }
            }

            container address {
                config false;
                ze:command "ze-iface:interface-addr-add";
                description "Add an IP address to an interface.
Usage: create interface <name> address <prefix>. Interface must already exist.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }
        }
    }

    container delete {
        config false;

        container interface {
            config false;
            ze:command "ze-iface:interface-delete";
            description "Delete an interface from the kernel.
Usage: delete interface <name>.";
            leaf name { type string; mandatory true; description "Interface name"; }

            container unit {
                config false;
                ze:command "ze-iface:interface-unit-del";
                description "Remove a VLAN sub-interface.
Usage: delete interface <name> unit.";
                leaf name { type string; mandatory true; description "Sub-interface name (e.g. eth0.100)"; }
            }

            container address {
                config false;
                ze:command "ze-iface:interface-addr-del";
                description "Remove an IP address from an interface.
Usage: delete interface <name> address <prefix>.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }
        }
    }

    container request {
        config false;

        container interface {
            config false;
            description "Interface operational state";

            container up {
                config false;
                ze:command "ze-iface:interface-up";
                description "Bring an interface up.
Usage: request interface <name> up.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }

            container down {
                config false;
                ze:command "ze-iface:interface-down";
                description "Shut down an interface.
Usage: request interface <name> down.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }

            container mtu {
                config false;
                ze:command "ze-iface:interface-mtu";
                description "Set the MTU on an interface.
Usage: request interface <name> mtu <bytes>. Range: 68 to 65535.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }

            container mac {
                config false;
                ze:command "ze-iface:interface-mac";
                description "Set the MAC address on an interface.
Usage: request interface <name> mac <aa:bb:cc:dd:ee:ff>.";
                leaf name { type string; mandatory true; description "Interface name"; }
            }

            container migrate {
                config false;
                ze:command "ze-iface:interface-migrate";
                description "Move IP addresses between interfaces with minimal downtime.
Takes a source interface, a target interface, and the address to move.
Adds addresses to the target before removing them from the source
(make-before-break).";
            }
        }
    }
}
```

`ze-iface-conf.yang`

```yang
module ze-iface-conf {
    namespace "urn:ze:iface:conf";
    prefix iface;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description "Network interface configuration for Ze";

    revision 2026-01-01 {
        description "Initial revision";
    }

    grouping interface-common {
        description "Interface properties common to all kinds (L2 and L3)";

        leaf description {
            type string {
                length "0..255";
            }
            description "Interface description";
        }

        leaf mtu {
            type uint16 {
                range "68..16000";
            }
            default 1500;
            description "Maximum transmission unit";
        }

        leaf os-name {
            type string;
            description
                "OS/kernel device this logical interface name binds to. When
                 omitted, the logical name is used as the OS device name, so
                 every interface whose name already matches its kernel device
                 resolves unchanged. Set it to alias a human-readable interface
                 name to a different kernel device; the iface resolver maps the
                 logical name to this OS device (ze init records the discovered
                 OS name here).";
        }

        leaf disable {
            type empty;
            description "Administratively disable this interface";
        }
    }

    grouping interface-l2 {
        description "Interface properties for kinds that expose a MAC
                     address at the list level: ethernet, dummy, veth,
                     bridge. Tunnel L2 kinds (gretap, ip6gretap) carry
                     the mac/address leaf inside their per-case containers
                     instead. L3-only kinds with no MAC (e.g. wireguard,
                     L3 tunnels) use interface-common directly.";

        uses interface-common;

        container mac {
            description "Hardware MAC settings: override the operational address
                         (address) and/or select the kernel device by its MAC
                         (match). The two are independent: a NIC can be matched
                         by its permanent (factory) MAC AND have its operational
                         MAC overridden at the same time.";
            leaf address {
                type string {
                    pattern '[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}';
                }
                ze:validate "mac-address";
                description "Override the kernel-assigned MAC address with this
                             explicit colon-separated hex value (e.g.,
                             02:42:ac:11:00:02). Applied at interface creation
                             time. When omitted, the kernel assigns a MAC
                             automatically.";
            }
            leaf match {
                type string {
                    pattern '[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}';
                }
                ze:validate "mac-address";
                description "Bind this logical interface to the kernel device
                             that carries this hardware MAC, instead of binding
                             by name. The resolver matches the device's PERMANENT
                             (factory) address (IFLA_PERM_ADDRESS) when it reports
                             one, so the binding survives an operational MAC
                             override; for virtual devices that report no
                             permanent address it matches the current address.
                             Takes precedence over os-name. Applies to ethernet
                             (the matched physical kind) only; the Ze-created
                             kinds (dummy/veth/bridge/...) are identified by the
                             name Ze assigns. When the matching device is absent
                             the binding defers until it appears.";
            }
        }

        container offload {
            description "Network offload and packet steering features.
                         Applied directly via kernel ioctl (the ethtool
                         SIOCETHTOOL interface) or sysfs writes after the
                         interface is created. The ethtool(8) CLI program is
                         NOT required; ze talks to the kernel directly.
                         Boolean leaves: true = explicitly enable the feature,
                         false = explicitly disable, absent = preserve whatever
                         the OS default is (no kernel call is made).
                         Offload availability depends on the NIC driver and
                         kernel version; unsupported features are logged as
                         warnings and do not block the config commit.";

            leaf gro {
                type boolean;
                description "Generic Receive Offload. Aggregates small incoming
                             packets into larger ones before passing them up the
                             network stack, reducing per-packet CPU overhead.
                             Software-based (works on all NIC types including
                             veth and dummy). Applied via kernel ioctl
                             (ETHTOOL_SGRO). Equivalent to: ethtool -K <dev>
                             gro on|off.";
            }

            leaf gso {
                type boolean;
                description "Generic Segmentation Offload. Delays segmentation
                             of large outgoing packets until just before the NIC
                             driver transmit path, reducing CPU work in the upper
                             stack. Software-based counterpart of TSO. Applied
                             via kernel ioctl (ETHTOOL_SGSO). Equivalent to:
                             ethtool -K <dev> gso on|off.";
            }

            leaf sg {
                type boolean;
                description "Scatter-Gather I/O. Allows the NIC to assemble a
                             single frame from multiple non-contiguous memory
                             buffers, avoiding data copies. Required by TSO and
                             GSO on most drivers. Applied via kernel ioctl
                             (ETHTOOL_SSG). Equivalent to: ethtool -K <dev>
                             sg on|off.";
            }

            leaf tso {
                type boolean;
                description "TCP Segmentation Offload. Offloads TCP segmentation
                             to the NIC hardware, allowing the kernel to hand off
                             large (up to 64 KB) TCP segments that the NIC splits
                             into MTU-sized frames on the wire. Requires NIC
                             hardware support. Disable when passing traffic to
                             VPP or virtual switches that cannot handle oversized
                             frames. Applied via kernel ioctl (ETHTOOL_STSO).
                             Equivalent to: ethtool -K <dev> tso on|off.";
            }

            leaf lro {
                type boolean;
                description "Large Receive Offload. Hardware-based counterpart of
                             GRO: the NIC coalesces incoming TCP segments before
                             DMA. Can break forwarding and bridging because the
                             coalesced frame no longer matches the original wire
                             format. Disable on routers or bridges that forward
                             traffic between interfaces. Applied via kernel ioctl
                             (ETHTOOL_SLRO). Equivalent to: ethtool -K <dev>
                             lro on|off.";
            }

            leaf hw-tc-offload {
                type boolean;
                description "Hardware Traffic Control Offload. Enables the NIC to
                             execute TC flower / u32 filter rules in hardware,
                             bypassing kernel processing for matched flows.
                             Requires NIC firmware support (common on mlx5, bnxt,
                             nfp). Applied via kernel ioctl (ETHTOOL_SFEATURES).
                             Equivalent to: ethtool -K <dev> hw-tc-offload
                             on|off.";
            }

            leaf rps {
                type boolean;
                description "Receive Packet Steering. Software-based distribution
                             of incoming packets across multiple CPUs by hashing
                             the packet header and steering to a target CPU receive
                             queue. Useful when the NIC has fewer hardware RSS
                             queues than available CPUs. When enabled, ze writes
                             a bitmask covering all online CPUs to each rx queue
                             sysfs entry (/sys/class/net/<dev>/queues/rx-*/
                             rps_cpus). When disabled, ze writes 0 to each entry.
                             Not an ethtool feature; uses sysfs directly.";
            }

            leaf rfs {
                type boolean;
                description "Receive Flow Steering. Extension of RPS that steers
                             incoming packets to the CPU where the application
                             consuming that flow is running, improving cache
                             locality and reducing cross-CPU cache bounces. When
                             enabled, ze sets /proc/sys/net/core/
                             rps_sock_flow_entries to 32768 and distributes
                             per-queue rps_flow_cnt evenly across rx queues.
                             RPS should also be enabled for RFS to be effective.
                             Not an ethtool feature; uses sysfs directly.";
            }
        }
    }

    grouping interface-unit {
        description "Logical interface unit properties";

        list unit {
            key "name";
            description "Logical interface unit";

            leaf name {
                type zt:node-name;
                description "Unit name (lowercase alphanumeric and hyphens)";
            }

            leaf vlan-id {
                type uint16 {
                    range "1..4094";
                }
                description "VLAN identifier";
            }

            leaf description {
                type string {
                    length "0..255";
                }
                description "Unit description";
            }

            leaf disable {
                type empty;
                description "Administratively disable this unit";
            }

            leaf route-priority {
                type uint32 {
                    range "0..4294966271";
                }
                default 0;
                description "Route metric for default routes installed via DHCP on
                             this unit. Lower values are preferred. On link-down,
                             the metric is increased by 1024 to deprioritize the
                             interface. 0 = kernel default.";
            }

            leaf vrf {
                type string;
                description "Assign this unit to a VRF (Virtual Routing and Forwarding) instance.
                             The VRF must be defined separately. Traffic on this unit uses the VRF's
                             routing table instead of the main table.";
            }

            leaf-list sysctl-profile {
                type zt:node-name;
                max-elements 10;
                ze:syntax "bracket";
                description "Named sysctl profiles to apply to this unit.
                             Built-in: dsr, router, hardened, multihomed, proxy.
                             User-defined profiles from sysctl { profile ... } config.
                             Applied in order; last wins on key overlap.";
            }

            container ipv4 {
                description "IPv4 addressing, forwarding, ARP behavior, and DHCP client for this unit.";

                leaf-list address {
                    type string {
                        pattern '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/[0-9]{1,2}';
                    }
                    ze:syntax "bracket";
                    description "IPv4 addresses in CIDR notation (e.g., 10.0.0.1/24)";
                }

                leaf forwarding {
                    type boolean;
                    description "Allow this interface to forward IPv4 packets between interfaces.
                                 Sets net.ipv4.conf.<iface>.forwarding. Required for routing.
                                 When disabled, packets not destined for a local address are dropped.";
                }

                leaf arp-filter {
                    type boolean;
                    description "Only respond to ARP requests for addresses on the receiving interface
                                 (net.ipv4.conf.<iface>.arp_filter). Prevents answering for addresses
                                 assigned to other interfaces. Recommended for multi-homed hosts.";
                }

                leaf arp-accept {
                    type boolean;
                    description "Accept gratuitous ARP frames and add their entries to the ARP cache
                                 (net.ipv4.conf.<iface>.arp_accept). Useful for failover scenarios
                                 where a peer announces a new MAC for an existing IP.";
                }

                leaf proxy-arp {
                    type boolean;
                    description "Answer ARP requests on behalf of other hosts that are reachable via
                                 this router (net.ipv4.conf.<iface>.proxy_arp). Used for bridging
                                 subnets without L2 connectivity or for unnumbered interfaces.";
                }

                leaf arp-announce {
                    type uint8 {
                        range "0..2";
                    }
                    default 0;
                    description "ARP announce level: 0=any, 1=prefer subnet, 2=best only";
                }

                leaf arp-ignore {
                    type uint8 {
                        range "0..2";
                    }
                    default 0;
                    description "ARP ignore level: 0=reply any, 1=reply only if target on incoming iface, 2=plus sender subnet check";
                }

                leaf rpf-check {
                    type enumeration {
                        enum disable;
                        enum strict;
                        enum loose;
                    }
                    description "Reverse path filtering mode";
                }

                container dhcp {
                    ze:backend "netlink";
                    description "DHCPv4 client configuration";

                    leaf enabled {
                        type boolean;
                        default false;
                        description "Enable DHCPv4 client";
                    }

                    leaf client-id {
                        type string;
                        description "DHCP option 61 client identifier. Sent in DISCOVER/REQUEST messages.
                                     When omitted, the MAC address is used. Override for environments
                                     where the DHCP server keys leases on client-id rather than MAC.";
                    }

                    leaf hostname {
                        type string;
                        description "Hostname in DHCP requests";
                    }
                }
            }

            container ipv6 {
                description "IPv6 addressing, forwarding, autoconfiguration, and DHCPv6 client for this unit.";

                leaf-list address {
                    type string {
                        pattern '[0-9a-fA-F:]{2,39}/[0-9]{1,3}';
                    }
                    ze:syntax "bracket";
                    description "IPv6 addresses in CIDR notation (e.g., fd00::1/64)";
                }

                leaf autoconf {
                    type boolean;
                    description "Enable IPv6 stateless autoconfiguration";
                }

                leaf accept-ra {
                    type uint8 {
                        range "0..2";
                    }
                    default 0;
                    description "Accept RA level: 0=disable, 1=if not forwarding, 2=even if forwarding";
                }

                leaf forwarding {
                    type boolean;
                    description "Allow this interface to forward IPv6 packets between interfaces.
                                 Sets net.ipv6.conf.<iface>.forwarding. Implicitly disables RA
                                 acceptance unless accept-ra is set to 2.";
                }

                leaf rpf-check {
                    type enumeration {
                        enum disable;
                        enum strict;
                        enum loose;
                    }
                    description "Reverse path filtering mode (VPP data plane only on IPv6).
                                 strict: drop packets whose source would not be routed back via this interface.
                                 loose: drop packets whose source has no route at all.
                                 disable: no source address validation.";
                }

                container dhcpv6 {
                    ze:backend "netlink";
                    description "DHCPv6 stateful client. Requests addresses and/or delegated prefixes from
                                 a DHCPv6 server (RFC 8415). Runs alongside SLAAC when autoconf is also enabled.";

                    leaf enabled {
                        type boolean;
                        default false;
                        description "Enable DHCPv6 client";
                    }

                    container pd {
                        description "Prefix delegation";

                        leaf length {
                            type uint8 {
                                range "48..64";
                            }
                            description "Requested prefix length";
                        }
                    }

                    leaf duid {
                        type string;
                        description "Override the DHCPv6 Unique Identifier (RFC 8415 Section 11).
                                     When omitted, a DUID-LL based on the interface MAC is generated.
                                     Set this when the DHCP server binds leases to a specific DUID.";
                    }
                }
            }

            container mirror {
                ze:os "linux";
                ze:backend "netlink";
                description "Traffic mirroring";

                leaf ingress {
                    type string;
                    description "Mirror ingress traffic to this interface";
                }

                leaf egress {
                    type string;
                    description "Mirror egress traffic to this interface";
                }
            }

            container mpls {
                ze:os "linux";
                description "MPLS forwarding on this interface (RFC 3031 LSR).";

                leaf enable {
                    type boolean;
                    default false;
                    description "Enable MPLS label input on this interface (net.mpls.conf.<iface>.input). The global label table size is set via net.mpls.platform_labels.";
                }
            }
        }
    }

    grouping tunnel-v4-endpoints {
        description "IPv4 underlay endpoints for a tunnel.";

        container local {
            description "Local IPv4 endpoint or source interface (one of ip or interface)";

            leaf ip {
                type zt:ipv4-address;
                description "Local IPv4 endpoint";
            }

            leaf interface {
                type string;
                description "Local interface to take the source address from";
            }
        }

        container remote {
            description "Remote IPv4 endpoint";

            leaf ip {
                type zt:ipv4-address;
                mandatory true;
                description "Remote IPv4 endpoint";
            }
        }
    }

    grouping tunnel-v6-endpoints {
        description "IPv6 underlay endpoints for a tunnel.";

        container local {
            description "Local IPv6 endpoint or source interface (one of ip or interface)";

            leaf ip {
                type zt:ipv6-address;
                description "Local IPv6 endpoint";
            }

            leaf interface {
                type string;
                description "Local interface to take the source address from";
            }
        }

        container remote {
            description "Remote IPv6 endpoint";

            leaf ip {
                type zt:ipv6-address;
                mandatory true;
                description "Remote IPv6 endpoint";
            }
        }
    }

    container interface {
        description "Network interface configuration.

                     Interface names MUST NOT be one of the CLI reserved
                     keywords: brief, scan, type, errors, counters. These
                     collide with the `show interface` / `clear interface`
                     command grammar (e.g. `show interface counters` would
                     be ambiguous if an interface were literally named
                     `counters`). Reserved names are rejected by
                     iface.ValidateIfaceName at apply time with a clear
                     error; the kernel does not enforce this on its own.";

        leaf backend {
            type string;
            default "netlink";
            description "Interface management backend (e.g., netlink, networkd)";
        }

        leaf dhcp-auto {
            type boolean;
            default false;
            description "Auto-discover first ethernet interface and run DHCP on it. Used when the interface name is not known at config time (e.g., gokrazy appliance). Ignored if any explicit DHCP config exists.";
        }

        list ethernet {
            key "name";
            unique "mac/address";
            unique "mac/match";
            description "Physical or virtual Ethernet interface. Ze manages the interface's
                         addresses, MTU, offload settings, and units. The interface must already
                         exist in the OS (ze does not create physical interfaces).";

            leaf name {
                type string;
                description "OS interface name (e.g., eth0, ens3). Must match an existing kernel interface.";
            }

            uses interface-l2;
            uses interface-unit;
        }

        list dummy {
            key "name";
            description "Dummy (loopback-like) interface. Created by ze if it does not exist.
                         Used for hosting service addresses that are not tied to a physical link
                         (e.g., router-id, anycast VIPs).";

            leaf name {
                type string;
                description "Interface name (e.g., dummy0). Ze creates the interface at apply time.";
            }

            uses interface-l2;
            uses interface-unit;
        }

        list veth {
            key "name";
            unique "mac/address";
            ze:backend "netlink";
            description "Virtual ethernet pair interface";

            leaf name {
                type string;
                description "Veth interface name (e.g., veth0). Ze creates the pair at apply time.";
            }

            leaf peer {
                type string;
                description "Veth peer name";
            }

            uses interface-l2;
            uses interface-unit;
        }

        list bridge {
            key "name";
            unique "mac/address";
            ze:backend "netlink";
            description "Linux bridge interface. Forwards Ethernet frames between member ports at L2.
                         Created by ze if it does not exist. Member interfaces are enslaved at apply time.";

            leaf name {
                type string;
                description "Bridge interface name (e.g., br0). Ze creates the bridge at apply time.";
            }

            leaf stp {
                ze:os "linux";
                type boolean;
                default false;
                description "Enable Spanning Tree Protocol";
            }

            leaf-list member {
                ze:os "linux";
                type string;
                ze:syntax "bracket";
                description "Member port interface names";
            }

            uses interface-l2;
            uses interface-unit;
        }

        list tunnel {
            key "name";
            ze:backend "netlink";
            description "Tunnel interface (GRE/GRETAP/IPIP/SIT/IP6TNL families).
                         Encapsulation kind is selected by the choice below; each
                         case carries only the leaves valid for that kind.
                         The local/remote endpoint pattern matches bgp peer
                         connection { local { ip ... } remote { ip ... } }.";

            leaf name {
                type string;
                description "Interface name (free-form, validated by iface naming rules)";
            }

            container encapsulation {
                description "Tunnel encapsulation kind and per-kind parameters";

                choice kind {
                    mandatory true;
                    description "Encapsulation discriminator. Invalid combinations
                                 (e.g. key on ipip) are unrepresentable in the schema.";

                    case gre {
                        description "GRE over IPv4 (RFC 2784, key extension RFC 2890). L3.";
                        container gre {
                            description "GRE-over-IPv4 parameters";
                            uses tunnel-v4-endpoints;

                            leaf key {
                                type uint32;
                                description "32-bit GRE key (RFC 2890). Symmetric: same
                                             value used for input and output";
                            }

                            leaf ttl {
                                type uint8;
                                default 0;
                                description "Outer-header TTL. 0 = inherit from inner";
                            }

                            leaf tos {
                                type uint8;
                                default 0;
                                description "Outer-header Type of Service. 0 = inherit";
                            }

                            leaf no-pmtu-discovery {
                                type empty;
                                description "Disable Path MTU Discovery on the outer header";
                            }
                        }
                    }

                    case gretap {
                        description "GRE over IPv4, L2 (Ethernet over GRE, bridgeable). RFC 2784.";
                        container gretap {
                            description "GRETAP-over-IPv4 parameters";
                            uses tunnel-v4-endpoints;

                            leaf key {
                                type uint32;
                                description "32-bit GRE key (RFC 2890)";
                            }

                            leaf ttl {
                                type uint8;
                                default 0;
                                description "Outer-header TTL. 0 = inherit";
                            }

                            leaf tos {
                                type uint8;
                                default 0;
                                description "Outer-header Type of Service. 0 = inherit";
                            }

                            leaf no-pmtu-discovery {
                                type empty;
                                description "Disable Path MTU Discovery on the outer header";
                            }

                            container mac {
                                description "Hardware MAC settings (L2 tunnel kinds only)";
                                leaf address {
                                    type string {
                                        pattern '[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}';
                                    }
                                    ze:validate "mac-address";
                                    description "Hardware MAC address (L2 tunnel kinds only)";
                                }
                            }
                        }
                    }

                    case ip6gre {
                        description "GRE over IPv6 (RFC 2784, key extension RFC 2890). L3.";
                        container ip6gre {
                            description "GRE-over-IPv6 parameters";
                            uses tunnel-v6-endpoints;

                            leaf key {
                                type uint32;
                                description "32-bit GRE key (RFC 2890)";
                            }

                            leaf hoplimit {
                                type uint8;
                                default 64;
                                description "Outer IPv6 hop limit (RFC 2473 Section 6.3)";
                            }

                            leaf tclass {
                                type uint8;
                                default 0;
                                description "Outer IPv6 traffic class (RFC 2473 Section 6.4)";
                            }
                        }
                    }

                    case ip6gretap {
                        description "GRE over IPv6, L2 (Ethernet over GRE bridgeable). RFC 2784.";
                        container ip6gretap {
                            description "GRETAP-over-IPv6 parameters";
                            uses tunnel-v6-endpoints;

                            leaf key {
                                type uint32;
                                description "32-bit GRE key (RFC 2890)";
                            }

                            leaf hoplimit {
                                type uint8;
                                default 64;
                                description "Outer IPv6 hop limit";
                            }

                            leaf tclass {
                                type uint8;
                                default 0;
                                description "Outer IPv6 traffic class";
                            }

                            container mac {
                                description "Hardware MAC settings (L2 tunnel kinds only)";
                                leaf address {
                                    type string {
                                        pattern '[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}';
                                    }
                                    ze:validate "mac-address";
                                    description "Hardware MAC address (L2 tunnel kinds only)";
                                }
                            }
                        }
                    }

                    case ipip {
                        description "IPv4 in IPv4 (RFC 2003). No GRE header, no key. L3.";
                        container ipip {
                            description "IPv4-in-IPv4 tunnel parameters (RFC 2003). Minimal overhead, no key support.";
                            uses tunnel-v4-endpoints;

                            leaf ttl {
                                type uint8;
                                default 0;
                                description "Outer-header TTL. 0 = inherit";
                            }

                            leaf tos {
                                type uint8;
                                default 0;
                                description "Outer-header Type of Service. 0 = inherit";
                            }

                            leaf no-pmtu-discovery {
                                type empty;
                                description "Disable Path MTU Discovery on the outer header";
                            }
                        }
                    }

                    case sit {
                        description "IPv6 in IPv4 (6in4, RFC 4213 Section 3). L3.";
                        container sit {
                            description "SIT (6in4) parameters";
                            uses tunnel-v4-endpoints;

                            leaf ttl {
                                type uint8;
                                default 0;
                                description "Outer-header TTL. 0 = inherit";
                            }

                            leaf tos {
                                type uint8;
                                default 0;
                                description "Outer-header Type of Service. 0 = inherit";
                            }

                            leaf no-pmtu-discovery {
                                type empty;
                                description "Disable Path MTU Discovery on the outer header";
                            }
                        }
                    }

                    case ip6tnl {
                        description "IPv6 in IPv6 (RFC 2473). Also covers ip6ip6. L3.";
                        container ip6tnl {
                            description "IPv6-in-IPv6 tunnel parameters (RFC 2473). Also handles ip6ip6 encapsulation.";
                            uses tunnel-v6-endpoints;

                            leaf hoplimit {
                                type uint8;
                                default 64;
                                description "Outer IPv6 hop limit (RFC 2473 Section 6.3)";
                            }

                            leaf tclass {
                                type uint8;
                                default 0;
                                description "Outer IPv6 traffic class (RFC 2473 Section 6.4)";
                            }

                            leaf encaplimit {
                                type uint8;
                                default 4;
                                description "Tunnel encapsulation limit (RFC 2473 Section 4.1.1)";
                            }
                        }
                    }

                    case ipip6 {
                        description "IPv4 in IPv6 (RFC 2473 with Next Header = 4). L3.
                                     Implemented via the ip6tnl Linux kind with Proto=IPPROTO_IPIP.";
                        container ipip6 {
                            description "IPv4-in-IPv6 tunnel parameters (RFC 2473, Next Header = 4). Uses the ip6tnl kernel kind.";
                            uses tunnel-v6-endpoints;

                            leaf hoplimit {
                                type uint8;
                                default 64;
                                description "Outer IPv6 hop limit";
                            }

                            leaf tclass {
                                type uint8;
                                default 0;
                                description "Outer IPv6 traffic class";
                            }

                            leaf encaplimit {
                                type uint8;
                                default 4;
                                description "Tunnel encapsulation limit";
                            }
                        }
                    }
                }
            }

            uses interface-common;
            uses interface-unit;
        }

        list wireguard {
            key "name";
            ze:listener;
            ze:backend "netlink";
            description "WireGuard interface. Declarative config: interface-level
                         listen-port, fwmark, private-key, and a nested peer list
                         with public-key, endpoint, allowed-ips, preshared-key,
                         and persistent-keepalive. L3 kind with no MAC address
                         (uses interface-common rather than interface-l2).
                         Reconciliation is in-place per peer via wgctrl
                         ConfigureDevice; adding, removing, or rekeying a peer
                         does not disturb the netdev or any other peer.";

            leaf name {
                type string;
                description "Interface name (free-form, validated by iface naming rules)";
            }

            leaf listen-port {
                type zt:port;
                description "UDP port WireGuard binds on 0.0.0.0 and :: for
                             peer handshake and data traffic. If unset the
                             kernel chooses an ephemeral port on the first
                             outbound handshake.";
            }

            leaf fwmark {
                type uint32;
                default 0;
                description "Firewall mark applied to outgoing encapsulated
                             packets for policy routing. 0 means unset.";
            }

            leaf private-key {
                mandatory true;
                type string;
                ze:sensitive;
                description "Base64-encoded 32-byte Curve25519 private key.
                             Stored $9$-encoded on disk via the standard ze
                             sensitive-leaf pattern; decoded to plaintext
                             base64 at parse time. ze config show / dump
                             always emits the $9$ form, never the plaintext.
                             Obfuscation only (not encryption) -- protect the
                             config file at filesystem level (chmod 600) just
                             like BGP MD5 passwords.";
            }

            list peer {
                key "name";
                description "WireGuard peer. Each peer is identified by its public key and has its own
                             set of allowed-ips that define which traffic is routed through the tunnel.";

                leaf name {
                    type string;
                    description "Config-level peer name (free-form label, used as list key). Not sent on the wire.";
                }

                leaf public-key {
                    type string;
                    mandatory true;
                    description "Base64-encoded 32-byte Curve25519 peer public key";
                }

                leaf preshared-key {
                    type string;
                    ze:sensitive;
                    description "Optional base64-encoded 32-byte symmetric
                                 preshared key mixed into the handshake for
                                 post-quantum resistance. Stored $9$-encoded
                                 on disk, same pattern as private-key.";
                }

                container endpoint {
                    description "Remote UDP endpoint for this peer. Both leaves
                                 are required together; the kernel uses this
                                 address on the first outbound handshake and
                                 updates it on every inbound handshake from
                                 a new source address.";

                    leaf ip {
                        type zt:ip-address;
                        description "Remote IPv4 or IPv6 address (numeric, not
                                     a hostname -- DNS resolution is not performed)";
                    }

                    leaf port {
                        type zt:port;
                        description "Remote UDP port";
                    }
                }

                leaf-list allowed-ips {
                    type string {
                        pattern '[0-9a-fA-F:][0-9a-fA-F:.]*(/[0-9]{1,3})?';
                    }
                    ze:syntax "bracket";
                    description "CIDR prefixes routed into the tunnel for this
                                 peer. Inbound packets from this peer must have
                                 a source address inside one of these prefixes
                                 (cryptokey routing).";
                }

                leaf persistent-keepalive {
                    type uint16 {
                        range "0..65535";
                    }
                    default 0;
                    description "Seconds between unsolicited keepalive packets
                                 to maintain NAT state. 0 disables keepalives.
                                 Typical value is 25.";
                }

                leaf disable {
                    type empty;
                    description "Administratively disable this peer. The peer
                                 is removed from the kernel peer set on reload
                                 but stays in the config.";
                }
            }

            uses interface-common;
            uses interface-unit;
        }

        list xfrm {
            key "name";
            ze:os "linux";
            ze:backend "netlink";
            description "XFRM interface (Linux 4.19+). Route-based IPsec: traffic
                         routed into this interface is encrypted/decrypted by the
                         kernel XFRM subsystem. The if-id binds security associations
                         to the interface. L3 kind with no MAC address.";

            leaf name {
                type string;
                description "Interface name (free-form, validated by iface naming rules)";
            }

            leaf if-id {
                mandatory true;
                type uint32 {
                    range "1..4294967295";
                }
                description "XFRM interface identifier. Binds this interface to
                             XFRM security associations that carry the same if_id.
                             Must be non-zero (0 means unset in the kernel).";
            }

            leaf dev {
                type string;
                description "Optional parent device name. When set, the XFRM interface
                             is bound to this physical device. When omitted, the
                             interface is unbound and uses the routing table to select
                             the underlay.";
            }

            uses interface-common;
            uses interface-unit;
        }

        list pppoe-client {
            key "name";
            ze:os "linux";
            ze:backend "netlink";
            description "PPPoE client interface (RFC 2516). Dials an access
                         concentrator over a physical Ethernet interface,
                         negotiates LCP/auth/IPCP, and presents the resulting
                         PPP session as a routable interface with server-
                         assigned addresses. The kernel pppN interface is
                         created dynamically; the name leaf is a config key
                         only (the OS interface name is pppN).";

            leaf name {
                type string;
                description "Config-level interface name (e.g. pppoe0)";
            }

            leaf source-interface {
                mandatory true;
                type string;
                description "Physical Ethernet interface for PPPoE discovery
                             (e.g. eth2). Must exist and be admin-up.";
            }

            container authentication {
                description "PPPoE authentication credentials (PAP or CHAP)";

                leaf username {
                    mandatory true;
                    type string {
                        length "1..255";
                    }
                    description "Authentication username sent to the AC";
                }

                leaf password {
                    mandatory true;
                    type string {
                        length "1..255";
                    }
                    ze:sensitive;
                    description "Authentication password. Stored $9$-encoded
                                 on disk via the standard ze sensitive-leaf
                                 pattern.";
                }
            }

            leaf service-name {
                type string {
                    length "0..255";
                }
                description "Desired PPPoE service name. Empty or absent
                             means accept any service (RFC 2516 Section 5.1).";
            }

            leaf ac-name {
                type string {
                    length "0..255";
                }
                description "Desired access concentrator name. When set,
                             only PADO frames from this AC are accepted.
                             Empty or absent means accept any AC.";
            }

            leaf no-default-route {
                type empty;
                description "Do not install a default route via the PPP
                             interface. When absent (default), a default
                             route is installed after IPCP completes.";
            }

            uses interface-common;
        }

        container loopback {
            description "The system loopback interface (lo). Always present; ze manages its addresses
                         and units. Used for hosting router-id addresses accessible from any interface.";

            uses interface-unit;
        }

        container monitor {
            description "Interface monitoring settings";

            leaf loopback {
                type boolean;
                default false;
                description "Monitor loopback interface";
            }
        }
    }
}
```

`ze-iface-interface-cmd.yang`

```yang
module ze-iface-interface-cmd {
    namespace "urn:ze:iface:interface:cmd";
    prefix ifaceifcmd;
    import ze-extensions { prefix ze; }
    description "show interface family (interface, brief, scan, type, errors, rate, name detail/counters), monitor interface rate, and clear interface counters. Owned by the iface component because every handler reads or resets interface state through the iface backend (iface.ListInterfaces / GetInterface / DiscoverInterfaces / GetRate / ClearCounters). Container-merges onto the show, monitor, and clear verb roots. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated the show interface family, monitor interface rate, and clear interface counters out of the central show, monitor, and clear schemas into the iface component (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container interface {
            config false;
            ze:command "ze-show:interface";
            description "Show network interfaces on this box.
Without arguments, returns all interfaces with full detail.
Subcommands: brief, type <t>, errors, rate [<name>], name <name> detail,
name <name> counters.";

            container brief {
                config false;
                ze:command "ze-show:interface";
                description "One-line summary per interface: name, state, IP, and MTU.
Quick way to see what is up and what addresses are assigned.";
            }

            container scan {
                config false;
                ze:command "ze-show:interface-scan";
                description "Discover and classify all OS interfaces.
Returns name, Ze type (ethernet, bridge, vxlan, etc.), and MAC for
each interface found. Pipe to table, yaml, or json for different
views. Useful during initial setup to see what the box has.";
            }

            container type {
                config false;
                ze:command "ze-show:interface";
                description "Show only interfaces of a given type.
Usage: show interface type <type>. Types include ethernet, bridge,
vxlan, wireguard, tunnel, bond, and more. If you pick an invalid
type, the error lists all valid ones.";
            }

            container errors {
                config false;
                ze:command "ze-show:interface";
                description "Show interfaces that have errors or drops.
Filters to only interfaces with non-zero Rx/Tx error or drop
counters. Quick way to find troubled links.";
            }

            container rate {
                config false;
                ze:command "ze-show:interface";
                description "Show per-second traffic rates on your interfaces.
Returns rx/tx bytes and packets per second. Pass an interface name
to narrow the output. Requires the rate tracker. For continuous
monitoring, use 'monitor interface rate' instead.";
            }

            container name {
                config false;
                description "Select one interface by name.";

                container detail {
                    config false;
                    ze:command "ze-show:interface-detail";
                    description "Show full detail for one interface.
Usage: show interface name <name> detail.";
                    leaf name {
                        type string;
                        mandatory true;
                        description "Interface name";
                    }
                }

                container counters {
                    config false;
                    ze:command "ze-show:interface-counters";
                    description "Show counters for one interface.
Usage: show interface name <name> counters.";
                    leaf name {
                        type string;
                        mandatory true;
                        description "Interface name";
                    }
                }
            }
        }
    }

    container monitor {
        config false;
        description "Continuous monitoring commands (Ctrl-C to stop)";

        container interface {
            config false;
            description "Interface traffic monitoring";

            container rate {
                config false;
                ze:command "ze-monitor:interface-rate";
                ze:task-support required;
                description "Stream per-second traffic rates for your interfaces.
Shows rx/tx bytes and packets per second, updating every second.
Optionally pass an interface name to watch just one link.";
            }
        }
    }

    container clear {
        config false;

        container interface {
            config false;
            description "Reset interface traffic counters.";

            container counters {
                config false;
                ze:command "ze-clear:interface-counters";
                description "Zero the Rx/Tx counters for every managed interface.
Usage: clear interface counters.";
            }

            container name {
                config false;
                description "Select one interface by name.";

                container counters {
                    config false;
                    ze:command "ze-clear:interface-counters";
                    description "Zero the Rx/Tx counters for one interface.
Usage: clear interface name <name> counters.";
                    leaf name {
                        type string;
                        mandatory true;
                        description "Interface name";
                    }
                }
            }
        }
    }
}
```

`ze-iface-monitor-cmd.yang`

```yang
module ze-iface-monitor-cmd {
    namespace "urn:ze:iface:monitor:cmd";
    prefix ifacemonitorcmd;
    import ze-extensions { prefix ze; }
    description "monitor system netlink command tree. Owned by the iface component because the handler streams kernel netlink events (routes, links, addresses) through the iface backend. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-06 { description "Relocated monitor system netlink out of the central monitor schema (plugin self-containment)."; }

    container monitor {
        config false;
        description "Continuous monitoring commands (Ctrl-C to stop)";

        container system {
            config false;
            description "System-level event streams";

            container netlink {
                config false;
                ze:command "ze-monitor:system-netlink";
                ze:task-support required;
                description "Watch kernel networking changes in real time.
Streams netlink events: route adds/deletes, link state changes,
address assignments. Filter with route, link, address, or all.";
            }
        }
    }
}
```

`ze-iface-show-cmd.yang`

```yang
module ze-iface-show-cmd {
    namespace "urn:ze:iface:show:cmd";
    prefix ifaceshowcmd;
    import ze-extensions { prefix ze; }
    description "show route / show neighbor / show arp command tree. Owned by the iface component because these handlers read the kernel neighbor and routing tables through the iface backend. Commands are object-rooted (no shared 'ip' container); see ai/rules/plugin-self-containment.md and docs/architecture/cli/command-namespacing.md.";
    revision 2026-06-26 { description "Object-rooted the kernel-table reads: show ip route -> show route, show ip arp -> show neighbor (with show arp as an IPv4 alias); dropped the show ip container and the show neighbors / show kernel-routes aliases."; }
    revision 2026-06-03 { description "Relocated show ip / neighbors / kernel-routes out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container route {
            config false;
            ze:command "ze-show:route";
            description "Show the kernel routing table.
Lists installed routes with next-hop, interface, protocol, and metric.
Pass a CIDR prefix or 'default' to filter, or a route limit to cap the
output.";
            leaf prefix {
                type string;
                description "CIDR prefix filter";
            }
            leaf limit {
                type uint32;
                description "Maximum number of routes";
            }

            container lookup {
                config false;
                ze:command "ze-show:route-lookup";
                description "Look up which route the kernel would use for a given IP.
Performs a longest-prefix-match and returns the matching route with
gateway, interface, protocol, and metric. Usage: show route lookup
<ip>.";
            }
        }

        container neighbor {
            config false;
            ze:command "ze-show:neighbor";
            description "Show the ARP and neighbor discovery table.
Lists IPv4 ARP and IPv6 ND entries with MAC addresses and states.
Pass ipv4 or ipv6 to filter by address family; no argument shows both.
For the IPv4-only view, 'show arp' is a shortcut.";
            leaf family {
                type enumeration {
                    enum ipv4;
                    enum ipv6;
                    enum any;
                    enum all;
                }
                description "Address family filter";
            }
        }

        container arp {
            config false;
            ze:command "ze-show:arp";
            description "Show the IPv4 ARP table (shortcut for 'show neighbor ipv4').
Lists IPv4 ARP entries with MAC address and state. ARP is IPv4-only;
use 'show neighbor' for both families or 'show neighbor ipv6' for the
IPv6 ND table.";
        }
    }
}
```

### iface-dhcp

source: `internal/plugins/iface/dhcp` -- depends on: `interface`

DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal

No YANG module of its own (reads config defined by another plugin, or has none).

## Isis (`isis`, 1 plugins)

### isis

source: `internal/plugins/isis` -- config root: `isis` -- depends on: `fib-kernel, sysctl`

Intermediate System to Intermediate System (ISO/IEC 10589, RFC 1195): native link-state IGP

`ze-isis-cmd.yang`

```yang
module ze-isis-cmd {
    namespace "urn:ze:isis:cmd";
    prefix isiscmd;
    import ze-extensions { prefix ze; }
    description
        "show isis ... and clear isis ... command tree. Owned by the isis
         component so that removing it removes these command nodes together with
         the handlers in internal/plugins/isis/cmd_show.go. Both verbs bind into
         the CENTRAL ze-show / ze-clear command namespaces (Go-registered RPCs,
         the LDP/iface model); there is no per-component ze-isis-api module. The
         show and clear subtrees container-merge onto the generic show and clear
         verb roots. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-18 {
        description "Initial revision: show isis neighbor/database[ detail]/route/interface/hostname/spf-log and clear isis adjacency/counters (spec-isis-13).";
    }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container isis {
            config false;
            description "IS-IS neighbors, link-state database, routes, interfaces, hostnames, and SPF history (ISO/IEC 10589, RFC 1195).";

            container neighbor {
                config false;
                ze:command "ze-show:isis-neighbor";
                description "Show IS-IS adjacencies.
Returns the neighbor System ID, interface, level, adjacency state,
and hold time for each IS-IS neighbor.";
            }

            container database {
                config false;
                ze:command "ze-show:isis-database";
                description "Show the IS-IS link-state database.
Lists each LSP with its LSP ID, sequence number, remaining lifetime,
checksum, and overload bit, across Level-1 and Level-2.";

                container detail {
                    config false;
                    ze:command "ze-show:isis-database-detail";
                    description "Show the IS-IS link-state database with TLV detail.
Expands each LSP into its decoded TLVs (type, length, value) so you
can read exactly what each node advertises.";
                }
            }

            container route {
                config false;
                ze:command "ze-show:isis-route";
                description "Show IS-IS-computed routes.
Lists each prefix the SPF installed with its metric, level, up/down
bit, and next-hops (address and outgoing interface).";

                container ipv6 {
                    config false;
                    ze:command "ze-show:isis-route-ipv6";
                    description "Show IS-IS-computed IPv6 routes (RFC 5308).
Lists each IPv6 prefix the SPF installed with its metric, level,
and next-hops (link-local address and outgoing interface).";
                }
            }

            container interface {
                config false;
                ze:command "ze-show:isis-interface";
                description "Show IS-IS-enabled circuits.
Returns level, circuit type, metric, hello interval, hold multiplier,
passive flag, DIS state, and the count of Up adjacencies per circuit.";
            }

            container hostname {
                config false;
                ze:command "ze-show:isis-hostname";
                description "Show the IS-IS dynamic-hostname mapping (RFC 5301).
Maps each System ID to the hostname it advertises in TLV 137.";
            }

            container spf-log {
                config false;
                ze:command "ze-show:isis-spf-log";
                description "Show recent IS-IS SPF runs.
Returns the most recent SPF runs with their timestamp, level, trigger,
duration, and node count.";
            }
        }
    }

    container clear {
        config false;

        container isis {
            config false;
            description "Reset IS-IS runtime state without reconfiguring.";

            container adjacency {
                config false;
                ze:command "ze-clear:isis-adjacency";
                description "Tear down every IS-IS adjacency so neighbors re-form.
Usage: clear isis adjacency. Adjacencies re-learn from the next Hello;
the circuit is not closed and the configuration is unchanged.";
            }

            container counters {
                config false;
                ze:command "ze-clear:isis-counters";
                description "Reset IS-IS observational counters and the SPF log.
Usage: clear isis counters. Monotonic Prometheus series are not reset;
the SPF-run history is cleared.";
            }
        }
    }
}
```

`ze-isis-conf.yang`

```yang
module ze-isis-conf {
    namespace "urn:ze:isis:conf";
    prefix isisconf;
    import ze-extensions { prefix ze; }
    description
        "Native IS-IS link-state IGP configuration (ISO/IEC 10589, RFC 1195,
         RFC 5305 wide metrics, RFC 5301 dynamic hostname, RFC 5304/5310
         authentication). Presence of the isis container enables the component.";
    revision 2026-06-18 { description "Initial revision (spec-isis-4)."; }

    container isis {
        description "IS-IS routing instance configuration.";
        ze:config-root "isis";

        // ISO/IEC 10589 section 6.2: a NET is an Area Address (1..13 octets) +
        // the 6-octet System ID + a 1-octet NSEL of 0x00 for an IS. At least one
        // NET is required; the system-id is derived from the first NET's 6 bytes
        // before the NSEL. The custom validator enforces hex/length/NSEL.
        leaf-list net {
            type string;
            ze:validate "isis-net";
            description
                "Network Entity Title(s), e.g. 49.0001.0000.0000.0001.00. At least
                 one is required; the System ID is the 6 octets before the NSEL.";
        }

        // RFC 1195: the System ID is a fixed 6-octet field. The pattern enforces
        // the canonical dotted-hex form xxxx.xxxx.xxxx; when omitted it is derived
        // from the NET.
        leaf system-id {
            type string {
                pattern "[0-9a-fA-F]{4}\\.[0-9a-fA-F]{4}\\.[0-9a-fA-F]{4}";
            }
            ze:validate "isis-system-id";
            description "6-byte System ID (xxxx.xxxx.xxxx); derived from NET if unset.";
        }

        leaf level {
            type enumeration {
                enum l1 { description "Level-1 (intra-area) only."; }
                enum l2 { description "Level-2 (backbone) only."; }
                enum l1-l2 { description "Both Level-1 and Level-2."; }
            }
            default "l1-l2";
            description "Routing level of this Intermediate System.";
        }

        leaf lsp-lifetime {
            type uint16 { range "1..65535"; }
            default 1200;
            units seconds;
            description "Maximum LSP remaining lifetime.";
        }

        leaf lsp-refresh-interval {
            type uint16 { range "1..65535"; }
            default 900;
            units seconds;
            description "LSP refresh interval.";
        }

        leaf overload {
            type boolean;
            default false;
            description "Set the overload bit (RFC 3787).";
        }

        // RFC 5301: the dynamic hostname (TLV 137) advertised by this IS.
        leaf hostname {
            type string { length "1..255"; }
            description "Dynamic hostname to advertise (RFC 5301).";
        }

        container interfaces {
            description "IS-IS-enabled interfaces.";
            list interface {
                key "name";
                description "Per-interface IS-IS configuration.";

                leaf name {
                    type string;
                    description "Interface name.";
                }
                leaf enabled {
                    type boolean;
                    default true;
                    description "IS-IS enabled on this interface.";
                }
                leaf passive {
                    type boolean;
                    default false;
                    description "Advertise the interface but form no adjacencies.";
                }
                leaf circuit-type {
                    type enumeration {
                        enum broadcast { description "LAN broadcast circuit (DIS election)."; }
                        enum point-to-point { description "Point-to-point circuit."; }
                    }
                    default "broadcast";
                    description "Circuit type.";
                }
                leaf level {
                    type enumeration {
                        enum l1;
                        enum l2;
                        enum l1-l2;
                    }
                    default "l1-l2";
                    description "Per-interface level override.";
                }
                // RFC 5305: wide metric, 24-bit (TLV 22) / 32-bit (TLV 135). The
                // per-interface circuit metric is bounded to the 24-bit wide range.
                leaf metric {
                    type uint32 { range "1..16777215"; }
                    default 10;
                    description "Wide metric (RFC 5305).";
                }
                leaf hello-interval {
                    type uint16 { range "1..65535"; }
                    default 10;
                    units seconds;
                    description "Hello interval.";
                }
                leaf hold-multiplier {
                    type uint8 { range "1..255"; }
                    default 3;
                    description "Hold time = hello-interval * hold-multiplier.";
                }
                leaf priority {
                    type uint8 { range "0..127"; }
                    default 64;
                    description "DIS election priority (broadcast circuits).";
                }

                container level-1 {
                    description "Level-1 per-interface overrides.";
                    leaf metric { type uint32 { range "1..16777215"; } description "L1 wide metric override."; }
                    leaf hello-interval { type uint16 { range "1..65535"; } units seconds; description "L1 hello-interval override."; }
                    leaf hold-multiplier { type uint8 { range "1..255"; } description "L1 hold-multiplier override."; }
                    leaf priority { type uint8 { range "0..127"; } description "L1 DIS priority override."; }
                    leaf auth-key-chain { type string; description "L1 per-interface (IIH) key-chain reference."; }
                }
                container level-2 {
                    description "Level-2 per-interface overrides.";
                    leaf metric { type uint32 { range "1..16777215"; } description "L2 wide metric override."; }
                    leaf hello-interval { type uint16 { range "1..65535"; } units seconds; description "L2 hello-interval override."; }
                    leaf hold-multiplier { type uint8 { range "1..255"; } description "L2 hold-multiplier override."; }
                    leaf priority { type uint8 { range "0..127"; } description "L2 DIS priority override."; }
                    leaf auth-key-chain { type string; description "L2 per-interface (IIH) key-chain reference."; }
                }

                list address-family {
                    key "af";
                    description
                        "Per-interface address families on this circuit (single-topology;
                         both ride the shared SPF tree).";
                    leaf af {
                        type enumeration {
                            enum ipv4-unicast { description "IPv4 unicast."; }
                            enum ipv6-unicast { description "IPv6 unicast."; }
                        }
                        description "Address family key.";
                    }
                }
            }
        }

        list key-chains {
            key "name";
            description "Named authentication key chains for hitless key rotation.";
            leaf name {
                type string { length "1..63"; }
                description "Key-chain name; referenced by per-interface and per-level auth leaves.";
            }
            list key {
                key "key-id";
                description "Keys in this chain.";
                leaf key-id {
                    type uint16 { range "0..65535"; }
                    description "Key identifier carried in TLV 10 (RFC 5310).";
                }
                leaf algorithm {
                    type enumeration {
                        enum cleartext { description "Cleartext password (auth type 1)."; }
                        enum hmac-md5 { description "HMAC-MD5 (auth type 54, RFC 5304)."; }
                        enum hmac-sha-1 { description "HMAC-SHA-1 (auth type 3, RFC 5310)."; }
                        enum hmac-sha-224 { description "HMAC-SHA-224 (auth type 3, RFC 5310)."; }
                        enum hmac-sha-256 { description "HMAC-SHA-256 (auth type 3, RFC 5310)."; }
                        enum hmac-sha-384 { description "HMAC-SHA-384 (auth type 3, RFC 5310)."; }
                        enum hmac-sha-512 { description "HMAC-SHA-512 (auth type 3, RFC 5310)."; }
                    }
                    default "hmac-md5";
                    description "Authentication algorithm.";
                }
                leaf secret {
                    type string { length "1..255"; }
                    ze:sensitive;
                    description "Shared secret, masked and $9$-encoded at rest.";
                }
                container send-lifetime {
                    description "When this key may be used to sign (hitless rotation).";
                    leaf start { type string; description "RFC3339 start timestamp."; }
                    leaf end { type string; description "RFC3339 end timestamp."; }
                }
                container accept-lifetime {
                    description "When this key is accepted on receive (hitless rotation).";
                    leaf start { type string; description "RFC3339 start timestamp."; }
                    leaf end { type string; description "RFC3339 end timestamp."; }
                }
            }
        }

        container level-1 {
            description "Level-1 (area) per-level configuration.";
            leaf auth-key-chain { type string; description "L1 per-level (LSP/SNP, area key) key-chain reference."; }
        }
        container level-2 {
            description "Level-2 (domain) per-level configuration.";
            leaf auth-key-chain { type string; description "L2 per-level (LSP/SNP, domain key) key-chain reference."; }
        }
    }
}
```

## Kernel (`kernel`, 1 plugins)

### kernel

source: `internal/plugins/kernel` -- config root: `kernel`

Kernel routes: redistribute externally-installed kernel routes into BGP

`ze-kernel-conf.yang`

```yang
module ze-kernel-conf {
    namespace "urn:ze:kernel:conf";
    prefix kernel;

    import ze-extensions { prefix ze; }

    description
        "Kernel route redistribution for Ze.
         When present, externally-installed kernel routes (DHCP, PPP,
         manual) are advertised into BGP via the redistribute event bus.";

    revision 2026-05-15 {
        description "Initial revision.";
    }

    container kernel {
        description "Kernel route redistribution. Presence enables the plugin.";
        ze:config-root "kernel";
    }
}
```

## L2Tp (`l2tp`, 4 plugins)

### l2tp-auth-local

source: `internal/component/l2tp/plugins/authlocal` -- config root: `l2tp`

Static local user list for L2TP PPP authentication

`ze-l2tp-auth-local-conf.yang`

```yang
module ze-l2tp-auth-local-conf {
    namespace "urn:ze:l2tp:auth:local:conf";
    prefix l2tp-auth-local;
    import ze-extensions { prefix ze; }

    description
        "Static local user list for L2TP PPP authentication.
         When no users are configured, all sessions are accepted
         (permissive default for testing/development).";

    container l2tp {
        container auth {
            container local {
                list user {
                    key "name";
                    leaf name {
                        type string;
                        description "PPP username.";
                    }
                    leaf password {
                        type string;
                        ze:sensitive;
                        description
                            "Shared secret for PAP cleartext and
                             CHAP-MD5/MS-CHAPv2 challenge-response.";
                    }
                }
            }
        }
    }
}
```

### l2tp-auth-radius-servers

source: `internal/component/l2tp/plugins/authradius` -- config root: `l2tp` -- depends on: `radius-server`

RADIUS authentication and accounting for L2TP PPP sessions

`ze-l2tp-auth-radius-conf.yang`

```yang
module ze-l2tp-auth-radius-conf {
    namespace "urn:ze:l2tp:auth:radius:conf";
    prefix l2tp-auth-radius;
    import ze-extensions { prefix ze; }
    import ze-types { prefix zt; }

    description
        "RADIUS authentication and accounting for L2TP PPP sessions.";

    container l2tp {
        container auth {
            container radius {
                leaf source-address {
                    type zt:ipv4-address;
                    description "Source IPv4 address for outbound RADIUS packets.";
                }
                leaf nas-identifier {
                    type string;
                    description "NAS-Identifier sent in RADIUS requests.";
                }
                leaf timeout {
                    type uint8 { range "1..30"; }
                    default 3;
                    description "Per-request timeout in seconds.";
                }
                leaf retries {
                    type uint8 { range "1..10"; }
                    default 3;
                    description "Number of retransmit attempts per server.";
                }
                leaf acct-interval {
                    type uint16 { range "60..3600"; }
                    default 300;
                    description "Accounting interim-update interval in seconds.";
                }
                list server {
                    key "name";
                    ordered-by user;
                    leaf name {
                        type string;
                        description "Logical name for this RADIUS server entry.";
                    }
                    leaf address {
                        type string;
                        mandatory true;
                        description "RADIUS server IP address or hostname.";
                    }
                    leaf port {
                        type uint16 { range "1..65535"; }
                        default 1812;
                        description "RADIUS server UDP port.";
                    }
                    leaf shared-key {
                        type string;
                        ze:sensitive;
                        description "RADIUS shared secret.";
                    }
                }
            }
        }
    }
}
```

### l2tp-pool

source: `internal/component/l2tp/plugins/pool` -- config root: `l2tp`

IPv4 address and IPv6 prefix pool for L2TP PPP sessions

`ze-l2tp-pool-conf.yang`

```yang
module ze-l2tp-pool-conf {
    namespace "urn:ze:l2tp:pool:conf";
    prefix l2tp-pool;
    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description
        "IPv4 address and IPv6 prefix pool for L2TP PPP sessions.
         Allocates addresses/prefixes from configured ranges using
         bitmap-backed pools. Releases on session-down.";

    container l2tp {
        container pool {
            container ipv4 {
                leaf gateway {
                    type zt:ipv4-address;
                    mandatory true;
                    description
                        "NAS-side IP for all PPP sessions (IPCP local address).
                         Must not overlap the pool range.";
                }
                leaf start {
                    type zt:ipv4-address;
                    description "First address in the pool range.";
                }
                leaf end {
                    type zt:ipv4-address;
                    description "Last address in the pool range (inclusive).";
                }
                leaf dns-primary {
                    type zt:ipv4-address;
                    description "Primary DNS server pushed to subscribers.";
                }
                leaf dns-secondary {
                    type zt:ipv4-address;
                    description "Secondary DNS server pushed to subscribers.";
                }
            }
            list named-pool {
                key "name";
                description
                    "Named IPv4 pools selected by RADIUS Framed-Pool attribute.";
                leaf name {
                    type string;
                    description "Pool name matching RADIUS Framed-Pool value.";
                }
                leaf gateway {
                    type zt:ipv4-address;
                    mandatory true;
                    description "NAS-side IP for sessions using this pool.";
                }
                leaf start {
                    type zt:ipv4-address;
                    mandatory true;
                    description "First address in the pool range.";
                }
                leaf end {
                    type zt:ipv4-address;
                    mandatory true;
                    description "Last address in the pool range (inclusive).";
                }
                leaf dns-primary {
                    type zt:ipv4-address;
                    description "Primary DNS server pushed to subscribers.";
                }
                leaf dns-secondary {
                    type zt:ipv4-address;
                    description "Secondary DNS server pushed to subscribers.";
                }
            }
            container ipv6-pd {
                description
                    "IPv6 prefix delegation pool. Allocates /N prefixes from
                     a configured block for DHCPv6-PD. RFC 3633.";
                leaf block {
                    type string;
                    description
                        "Prefix block to allocate from (e.g. 2001:db8::/32).";
                }
                leaf delegation-length {
                    type uint8 {
                        range "48..64";
                    }
                    description
                        "Prefix length delegated to each subscriber
                         (e.g. 56 for /56 prefixes).";
                }
            }
            list named-ipv6-pool {
                key "name";
                description
                    "Named IPv6 prefix pools selected by RADIUS
                     Framed-IPv6-Pool attribute (RFC 6911 attr 100).";
                leaf name {
                    type string;
                    description
                        "Pool name matching RADIUS Framed-IPv6-Pool value.";
                }
                leaf block {
                    type string;
                    mandatory true;
                    description
                        "Prefix block to allocate from (e.g. 2001:db8::/32).";
                }
                leaf delegation-length {
                    type uint8 {
                        range "48..64";
                    }
                    mandatory true;
                    description
                        "Prefix length delegated to each subscriber.";
                }
            }
        }
    }
}
```

### l2tp-shaper

source: `internal/component/l2tp/plugins/shaper` -- config root: `l2tp`

Traffic shaping for L2TP subscriber sessions

`ze-l2tp-shaper-conf.yang`

```yang
module ze-l2tp-shaper-conf {
    namespace "urn:ze:l2tp:shaper:conf";
    prefix l2tp-shaper;
    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description
        "Traffic shaping for L2TP subscriber sessions.
         Applies TC (traffic control) rules on pppN interfaces
         when sessions come up. Supports TBF and HTB qdiscs.";

    container l2tp {
        container shaper {
            leaf qdisc-type {
                type enumeration {
                    enum tbf {
                        description "Token Bucket Filter (single rate limiter).";
                    }
                    enum htb {
                        description "Hierarchical Token Bucket (classful).";
                    }
                }
                default tbf;
                description "Queueing discipline type for subscriber interfaces.";
            }
            leaf default-rate {
                type zt:rate;
                mandatory true;
                description
                    "Default download rate applied to new sessions.
                     Format: number followed by suffix (e.g. 10mbit, 100kbit).";
            }
            leaf upload-rate {
                type zt:rate;
                description
                    "Default upload rate. When omitted, defaults to default-rate.";
            }
        }
    }
}
```

## Ldp (`ldp`, 1 plugins)

### ldp-port

source: `internal/plugins/ldp` -- config root: `ldp` -- depends on: `fib-kernel`

Label Distribution Protocol (RFC 5036): MPLS label distribution

`ze-ldp-cmd.yang`

```yang
module ze-ldp-cmd {
    namespace "urn:ze:ldp:cmd";
    prefix ldpcmd;
    import ze-extensions { prefix ze; }
    description "show ldp ... command tree. Owned by the ldp component so that removing it removes these command nodes together with the handlers. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated show ldp ... out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container ldp {
            config false;
            description "LDP neighbors and label bindings";

            container neighbor {
                config false;
                ze:command "ze-show:ldp-neighbor";
                description "Show LDP neighbors and their session state.
Returns peer address, transport address, session state, and
hold time for each LDP neighbor.";
            }

            container binding {
                config false;
                ze:command "ze-show:ldp-binding";
                description "Show LDP FEC-to-label bindings.
Lists local and remote label bindings for each FEC (prefix).
Use this to verify label distribution is working.";
            }
        }
    }
}
```

`ze-ldp-conf.yang`

```yang
module ze-ldp-conf {
    namespace "urn:ze:ldp:conf";
    prefix ldpconf;
    import ze-extensions { prefix ze; }
    description "LDP protocol configuration (RFC 5036)";
    revision 2026-05-28 { description "Initial revision"; }

    container ldp {
        description "Label Distribution Protocol configuration";

        leaf lsr-id {
            type string;
            description "LSR identifier (IPv4 address format)";
        }

        leaf transport-address {
            type string;
            description "Transport address for TCP sessions";
        }

        leaf hello-interval {
            type uint16 {
                range "1..65535";
            }
            default 5;
            units seconds;
            description "Hello message interval";
        }

        leaf hello-hold-time {
            type uint16 {
                range "1..65535";
            }
            default 15;
            units seconds;
            description "Hello adjacency hold time";
        }

        leaf keepalive-time {
            type uint16 {
                range "1..65535";
            }
            default 60;
            units seconds;
            description "Session keepalive interval";
        }

        leaf-list interfaces {
            type string;
            description "Interfaces on which LDP discovery is enabled";
        }
    }
}
```

## Mrt (`mrt`, 1 plugins)

### mrt

source: `internal/plugins/mrt` -- config root: `mrt`

MRT routing information export (RFC 6396)

`ze-mrt-conf.yang`

```yang
module ze-mrt-conf {
    namespace "urn:ze:mrt:conf";
    prefix mrt;

    import ze-extensions { prefix ze; }

    description "MRT routing information export configuration (RFC 6396)";

    revision 2026-06-07 {
        description "Initial revision";
    }

    grouping dump-stream-settings {
        description "Common settings for an MRT dump stream";

        leaf file {
            type string {
                length "1..255";
            }
            description
                "Output file path with strftime patterns for rotation.
                 Supported codes: %Y (year), %m (month), %d (day),
                 %H (hour), %M (minute), %S (second), %s (unix timestamp),
                 %N (table name).";
        }

        leaf interval {
            type uint32 {
                range "0..86400";
            }
            default 0;
            description
                "File rotation interval in seconds. Zero disables rotation.";
        }
    }

    container mrt {
        description "MRT dump configuration";

        leaf extended-timestamp {
            type boolean;
            default false;
            description
                "Use BGP4MP_ET (type 17) with microsecond resolution
                 instead of BGP4MP (type 16).";
        }

        leaf add-path {
            type boolean;
            default false;
            description
                "Force add-path subtypes even when not negotiated
                 with the peer.";
        }

        leaf-list peer-filter {
            type string;
            description
                "If set, only record messages from/to these peer addresses.
                 Empty list means record all peers.";
        }

        leaf direction {
            type enumeration {
                enum both;
                enum received;
                enum sent;
            }
            default both;
            description
                "Which direction of BGP messages to record.";
        }

        container updates {
            description "BGP UPDATE message stream (BGP4MP records)";
            uses dump-stream-settings;
        }

        container all {
            description
                "All BGP messages plus state changes (BGP4MP records).
                 Includes OPEN, UPDATE, NOTIFICATION, KEEPALIVE,
                 ROUTE_REFRESH, and FSM state transitions.";
            uses dump-stream-settings;
        }

        container routes {
            description
                "Periodic RIB snapshots (TABLE_DUMP_V2 records).
                 Each dump writes a PEER_INDEX_TABLE followed by
                 RIB entries for all address families.";

            leaf file {
                type string {
                    length "1..255";
                }
                description
                    "Output file path with strftime patterns.";
            }

            leaf interval {
                type uint32 {
                    range "60..86400";
                }
                default 3600;
                description
                    "RIB dump interval in seconds. Minimum 60.";
            }
        }
    }
}
```

## Ospf (`ospf`, 1 plugins)

### ospf

source: `internal/plugins/ospf` -- config root: `ospf` -- depends on: `interface, fib-kernel, sysctl`

Open Shortest Path First v2 (RFC 2328): native link-state IPv4 IGP

`ze-ospf-cmd.yang`

```yang
module ze-ospf-cmd {
    namespace "urn:ze:ospf:cmd";
    prefix ospfcmd;
    import ze-extensions { prefix ze; }
    description
        "show ospf ... command tree. Owned by the ospf component so removing it
         removes these command nodes together with the handlers in
         internal/plugins/ospf/cmd_show.go. The show subtree container-merges onto
         the generic show verb root; the wire methods bind into the CENTRAL ze-show
         namespace (Go-registered RPCs, the LDP/IS-IS model). Commands are
         object-rooted (no shared `ip` container); see
         docs/architecture/cli/command-namespacing.md and
         ai/rules/plugin-self-containment.md.";
    revision 2026-07-02 {
        description "Add the `ospf graceful-restart prepare` operator action (RFC 3623 planned
             restart trigger, spec-ospf-ext-9); add `show ospf instance` for OSPFv2
             Multi-Instance (RFC 6549, spec-ospf-ext-12).";
    }
    revision 2026-06-26 {
        description "Object-rooted: dropped the show/clear `ip` container so the tree is show ospf ... / clear ospf ... (was show ip ospf ...).";
    }
    revision 2026-06-21 {
        description "Initial revision: show ip ospf neighbor/interface/database/route/border-routers/spf (spec-ospf-13).";
    }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container ospf {
            config false;
            ze:command "ze-show:ospf";
            description "OSPFv2 process summary: router-id, areas, ABR/ASBR status, and stub-router (max-metric) state (RFC 2328).";

            container instance {
                config false;
                ze:command "ze-show:ospf-instance";
                description "Show the configured OSPFv2 instances (RFC 6549 Multi-Instance).
Lists each Instance ID with its router-id and the size of its isolated
area, interface, neighbor, and link-state database state.";
            }

            container graceful-restart {
                config false;
                ze:command "ze-show:ospf-graceful-restart";
                description "Show OSPFv2 (IPv4) Graceful Restart state (RFC 3623): the restarter
state (in-restart or not, grace end, reason) and the per-neighbor helper
sessions (which neighbors are being helped and their remaining grace).";
            }

            container segment-routing {
                config false;
                ze:command "ze-show:ospf-segment-routing";
                description "Show OSPFv2 (IPv4) Segment Routing state (RFC 8665): the configured
SRGB/SRLB label ranges, the advertised SR-Algorithm, this node's node
Prefix-SIDs, and the Adjacency-SIDs allocated per adjacency.";
            }

            container ipv6 {
                config false;
                ze:command "ze-show:ospf-ipv6";
                description "Show the OSPFv3 (IPv6) address-family instances (RFC 5838).
Lists each configured address family (ipv6-unicast, ipv6-multicast,
ipv4-unicast, ipv4-multicast) with its Instance ID, router-id, and
neighbor/interface counts, so multiple AF instances on a link are
distinguishable.";
                container interface {
                    config false;
                    ze:command "ze-show:ospf-ipv6-interface";
                    description "Show OSPFv3 (IPv6-family) interfaces and their RFC 4552 IPsec status.
Returns per interface whether IPsec is configured, the protocol (ah/esp) and
SPI, and whether the kernel SA is installed. The key is never shown.";
                }
                container graceful-restart {
                    config false;
                    ze:command "ze-show:ospf-ipv6-graceful-restart";
                    description "Show OSPFv3 (IPv6) Graceful Restart state (RFC 5187): the restarter
state (in-restart or not, grace end, reason) and the per-neighbor helper
sessions (which neighbors are being helped and their remaining grace).";
                }
                container segment-routing {
                    config false;
                    ze:command "ze-show:ospf-ipv6-segment-routing";
                    description "Show OSPFv3 (IPv6) Segment Routing state (RFC 8666): the configured
SRGB/SRLB label ranges, the advertised SR-Algorithm, this node's node
Prefix-SIDs, and the Adjacency-SIDs allocated per adjacency.";
                }
            }

            container neighbor {
                config false;
                ze:command "ze-show:ospf-neighbor";
                description "Show OSPF neighbors.
Returns each neighbor's router-id, interface, adjacency state, DR/BDR,
priority, dead time, and address.";
            }

            container interface {
                config false;
                ze:command "ze-show:ospf-interface";
                description "Show OSPF-enabled interfaces.
Returns area, network-type, cost, ISM state, DR/BDR, hello/dead
intervals, priority, and passive flag per interface.";
            }

            container database {
                config false;
                ze:command "ze-show:ospf-database";
                description "Show the OSPF link-state database.
Lists each LSA with its LS Type, Link State ID, Advertising Router,
sequence number, age, and checksum.";

                container router {
                    config false;
                    ze:command "ze-show:ospf-database-router";
                    description "Show only Router-LSAs (Type 1).";
                }
                container network {
                    config false;
                    ze:command "ze-show:ospf-database-network";
                    description "Show only Network-LSAs (Type 2).";
                }
                container summary {
                    config false;
                    ze:command "ze-show:ospf-database-summary";
                    description "Show only Summary-LSAs (Type 3, inter-area network).";
                }
                container asbr-summary {
                    config false;
                    ze:command "ze-show:ospf-database-asbr-summary";
                    description "Show only ASBR-Summary-LSAs (Type 4).";
                }
                container external {
                    config false;
                    ze:command "ze-show:ospf-database-external";
                    description "Show only AS-external-LSAs (Type 5).";
                }
                container nssa-external {
                    config false;
                    ze:command "ze-show:ospf-database-nssa-external";
                    description "Show only NSSA-external-LSAs (Type 7, RFC 3101).";
                }
                container opaque-link {
                    config false;
                    ze:command "ze-show:ospf-database-opaque-link";
                    description "Show only link-local opaque-LSAs (Type 9, RFC 5250).";
                }
                container opaque-area {
                    config false;
                    ze:command "ze-show:ospf-database-opaque-area";
                    description "Show only area-scope opaque-LSAs (Type 10, RFC 5250).";
                }
                container opaque-as {
                    config false;
                    ze:command "ze-show:ospf-database-opaque-as";
                    description "Show only AS-scope opaque-LSAs (Type 11, RFC 5250).";
                }
                container router-information {
                    config false;
                    ze:command "ze-show:ospf-database-router-information";
                    description "Show the Router Information LSAs (RFC 7770) for both address
families -- OSPFv2 opaque type 4 and OSPFv3 function code 12 -- decoded into the
advertised informational capability bits and the TLV list.";
                }
            }

            container te-database {
                config false;
                ze:command "ze-show:ospf-te-database";
                description "Show the OSPF Traffic Engineering Database (RFC 3630 / RFC 5392):
router addresses plus TE links with their Link ID, local/remote address, link
type, TE metric, bandwidths, admin group, and (for inter-AS links) remote AS and
remote ASBR.";
            }

            container route {
                config false;
                ze:command "ze-show:ospf-route";
                description "Show OSPF-computed routes.
Lists each prefix with its path type (intra/inter/external-1/2), cost,
next-hops, and area.";

                container fast-reroute {
                    config false;
                    ze:command "ze-show:ospf-route-fast-reroute";
                    description "Show OSPF fast-reroute (LFA / TI-LFA) backups (RFC 5286).
Lists each prefix's primary next-hops with their pre-computed loop-free
backup, protection class (node/link/downstream), and TI-LFA repair label
stack. Unprotected primaries are shown as unprotected.";
                }
            }

            container virtual-links {
                config false;
                ze:command "ze-show:ospf-virtual-links";
                description "Show OSPF virtual links (RFC 2328 section 15).
Lists each configured virtual link with its transit area, remote
router-id, adjacency state, computed cost, and transit next hop.";
            }

            container border-routers {
                config false;
                ze:command "ze-show:ospf-border-routers";
                description "Show routes to OSPF area-border and AS-boundary routers.
Lists each reachable ABR/ASBR with its router-id, cost, next-hops, and
area.";
            }

            container spf {
                config false;
                ze:command "ze-show:ospf-spf";
                description "Show recent OSPF SPF runs.
Returns the most recent per-area SPF runs with their timestamp,
duration, node count, and pending state.";
            }

            container ldp-sync {
                config false;
                ze:command "ze-show:ospf-ldp-sync";
                description "Show OSPF LDP-IGP synchronization state (RFC 5443, RFC 6138).
Lists each ldp-sync interface with its state (not-synchronized /
hold-down / synchronized), remaining hold-down, effective metric, and
whether it is stuck not-synchronized after having been synchronized.";
            }
        }
    }

    container clear {
        config false;

        container ospf {
            config false;
            description "Reset OSPF runtime state without reconfiguring.";

            container process {
                config false;
                ze:command "ze-clear:ospf-process";
                description "Full OSPF reset: tear down every adjacency and re-run SPF.
Usage: clear ospf process. Adjacencies re-form from the next Hello;
the configuration is unchanged.";
            }

            container neighbor {
                config false;
                ze:command "ze-clear:ospf-neighbor";
                description "Tear down every OSPF adjacency so neighbors re-form.
Usage: clear ospf neighbor. Adjacencies re-learn from the next Hello.";
            }

            container counters {
                config false;
                ze:command "ze-clear:ospf-counters";
                description "Reset the OSPF SPF-run history.
Usage: clear ospf counters. Monotonic Prometheus series are not reset;
the SPF-run log is cleared.";
            }
        }
    }

    container ospf {
        config false;
        description "OSPFv2 operational actions (RFC 2328).";

        container graceful-restart {
            config false;
            description "OSPFv2 (IPv4) Graceful Restart operator actions (RFC 3623).";

            container prepare {
                config false;
                ze:command "ze-ospf:graceful-restart-prepare";
                description "Trigger a planned OSPFv2 graceful restart (RFC 3623 section 2.1).
Usage: ospf graceful-restart prepare. The engine originates one Grace-LSA per
interface, persists the non-volatile restart fact, and suppresses route churn
so the FIB is retained across the ensuing control-plane restart. Refused when
graceful-restart is not configured.";
            }
        }
    }
}
```

`ze-ospf-conf.yang`

```yang
module ze-ospf-conf {
    namespace "urn:ze:ospf:conf";
    prefix ospfconf;
    import ze-extensions { prefix ze; }
    import ze-types { prefix zt; }

    description
        "Native OSPFv2 link-state IGP configuration (RFC 2328, RFC 9129 shape).
         Presence of the ospf container enables the component.";
    revision 2026-07-02 { description "Add per-interface instance-id leaf-list for OSPFv2 Multi-Instance (RFC 6549, spec-ospf-ext-12)."; }
    revision 2026-06-24 { description "RFC 5838 multiple address families (spec-ospf-ext-15)."; }
    revision 2026-06-20 { description "Initial revision (spec-ospf-4)."; }

    // ospf-af-topology is the per-address-family areas + interfaces shape (RFC 5838): every
    // OSPFv3 address family reuses the same area/interface model, distinguished only by its
    // Instance-ID range. The Instance-ID leaf is declared per AF container (its native range
    // is the AF's RFC 5838 §2.1 sub-range), so it is NOT part of this grouping.
    grouping ospf-af-topology {
        container areas {
            description "OSPFv3 areas for this address family.";
            list area {
                key "area-id";
                description "Per-area OSPFv3 configuration.";
                leaf area-id {
                    type string {
                        pattern "([0-9]{1,10}|([0-9]{1,3}\\.){3}[0-9]{1,3})";
                    }
                    ze:validate "ospf-area-id";
                    description "Area identifier as uint32 or dotted quad; 0.0.0.0 is the backbone.";
                }
                leaf area-type {
                    type enumeration {
                        enum normal;
                        enum stub;
                        enum nssa;
                    }
                    default "normal";
                    description "Area type.";
                }
                list virtual-link {
                    key "remote-router-id";
                    description
                        "Virtual link through this (transit) area to a backbone
                         area-border router (RFC 5340 section 4.2). It belongs to the
                         backbone; its cost is the transit-area SPF path cost, never
                         configured. The transit area must not be the backbone, a stub,
                         or an NSSA.";
                    leaf remote-router-id {
                        type string {
                            pattern "([0-9]{1,3}\\.){3}[0-9]{1,3}";
                        }
                        ze:validate "ospf-router-id";
                        description "Router ID of the far virtual-link endpoint (a 32-bit OSPFv3 Router ID, RFC 5340 section 2.11).";
                    }
                    leaf hello-interval { type uint16 { range "1..65535"; } default 10; units seconds; description "Hello interval on the virtual link."; }
                    leaf dead-interval { type uint16 { range "1..65535"; } default 40; units seconds; description "Router-dead interval on the virtual link."; }
                    leaf retransmit-interval { type uint16 { range "1..65535"; } default 5; units seconds; description "LSA retransmit interval on the virtual link."; }
                    leaf transmit-delay { type uint16 { range "1..3600"; } default 1; units seconds; description "Estimated LSA transmission delay (RFC 5340 / RFC 2328 InfTransDelay must be > 0)."; }
                }
            }
        }
        container interfaces {
            description "OSPFv3-enabled interfaces for this address family.";
            list interface {
                key "name";
                description "Per-interface OSPFv3 configuration.";
                leaf name { type string { length "1..255"; } description "Interface name."; }
                leaf area {
                    type string {
                        pattern "([0-9]{1,10}|([0-9]{1,3}\\.){3}[0-9]{1,3})";
                    }
                    ze:validate "ospf-area-id";
                    description "Declared area this interface belongs to.";
                }
                leaf enabled { type boolean; default true; description "OSPFv3 enabled on this interface."; }
                leaf network-type {
                    type enumeration {
                        enum broadcast;
                        enum point-to-point;
                        enum nbma;
                        enum point-to-multipoint;
                    }
                    default "broadcast";
                    description "Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 5340); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor /128 host routes and no DR.";
                }
                leaf cost { type uint16 { range "1..65535"; } description "Interface output cost."; }
                leaf hello-interval { type uint16 { range "1..65535"; } default 10; units seconds; description "Hello interval."; }
                leaf dead-interval { type uint16 { range "1..65535"; } default 40; units seconds; description "Router-dead interval."; }
                leaf poll-interval { type uint16 { range "1..65535"; } default 120; units seconds; description "NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5)."; }
                leaf priority { type uint8 { range "0..255"; } default 1; description "DR/BDR election priority; 0 means ineligible."; }
                list nbma-neighbor {
                    key "router-id";
                    description "Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant. The IPv6 neighbor is keyed by Router ID; the link-local (the unicast Hello destination) may be configured or learned from the first Hello.";
                    leaf router-id { type string { pattern "([0-9]{1,10}|([0-9]{1,3}\\.){3}[0-9]{1,3})"; } ze:validate "ospf-router-id"; description "Neighbor OSPFv3 Router ID."; }
                    leaf link-local { type string; ze:validate "ipv6-address"; description "Neighbor IPv6 link-local address (the unicast Hello destination); learned from the neighbor's first Hello when omitted."; }
                    leaf priority { type uint8 { range "0..255"; } default 0; description "Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected)."; }
                }
                leaf passive { type boolean; default false; description "Advertise the interface but form no adjacency."; }
                container ldp-sync {
                    description
                        "LDP-IGP synchronization (RFC 5443, RFC 6138) for the IPv6
                         family; reuses the AF-neutral state machine on the shared
                         interface model.";
                    leaf enable { type boolean; default false; description "Enable LDP-IGP synchronization on this interface."; }
                    leaf holddown {
                        type uint16 { range "0..65535"; }
                        default 0;
                        units seconds;
                        description "Seconds to wait after the LDP session is established before declaring the link synchronized (RFC 5443 section 2).";
                    }
                }
                container ipsec {
                    description
                        "RFC 4552 manual IPsec (AH/ESP) for this OSPFv3 interface. A transport-mode
                         kernel Security Association plus a proto-89 policy is installed on interface
                         up; the kernel applies AH/ESP below the socket and silently discards
                         unprotected or failed-integrity OSPF packets (RFC 4552 §3/§4). This is a
                         DISTINCT auth path from the RFC 7166 authentication trailer and is mutually
                         exclusive with a per-interface authentication key-chain. Manual keying only
                         (RFC 4552 §7: IKE cannot key the multicast group); the key is shared by the
                         inbound and outbound SA.";
                    leaf spi {
                        type uint32 { range "256..4294967295"; }
                        description "Security Parameters Index (RFC 4303 §2.1 reserves 0..255).";
                    }
                    leaf protocol {
                        type enumeration {
                            enum ah;
                            enum esp;
                        }
                        default "esp";
                        description "IPsec protocol: esp (RFC 4303; authentication MUST be supported) or ah (RFC 4302; MAY).";
                    }
                    leaf algorithm {
                        type enumeration {
                            enum sha1;
                            enum sha256;
                            enum sha384;
                            enum sha512;
                        }
                        default "sha256";
                        description "HMAC-SHA integrity algorithm.";
                    }
                    leaf key {
                        type string {
                            pattern "[0-9a-fA-F]+";
                            length "40..128";
                        }
                        ze:sensitive;
                        description "Hex integrity key; length must match the algorithm (sha1=40, sha256=64, sha384=96, sha512=128 hex characters).";
                    }
                    leaf encryption-algorithm {
                        type enumeration {
                            enum null;
                            enum aes128;
                            enum aes256;
                        }
                        description "ESP confidentiality algorithm (RFC 4552 §4: only ESP, never AH). Omit or null for authentication-only ESP.";
                    }
                    leaf encryption-key {
                        type string {
                            pattern "[0-9a-fA-F]+";
                            length "32..64";
                        }
                        ze:sensitive;
                        description "Hex ESP encryption key; length must match encryption-algorithm (aes128=32, aes256=64 hex characters).";
                    }
                }
                container bfd {
                    description "RFC 5880 / RFC 5881 single-hop BFD for this OSPFv3 interface. When enabled, a Full adjacency opens an IPv6 single-hop BFD session (link-local pair); a BFD-detected failure declares the neighbor down far faster than the router-dead interval.";
                    leaf enabled { type boolean; default false; description "Enable single-hop BFD failure detection on this interface."; }
                    leaf min-tx { type uint32 { range "1..10000"; } default 50; units milliseconds; description "Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval)."; }
                    leaf min-rx { type uint32 { range "1..10000"; } default 50; units milliseconds; description "Required minimum BFD receive interval (RFC 5880 Required Min RX Interval)."; }
                    leaf multiplier { type uint8 { range "1..255"; } default 3; description "BFD detection multiplier (RFC 5880 Detect Mult)."; }
                }
            }
        }
        uses ospf-segment-routing;
    }

    // ospf-segment-routing is the per-address-family Segment Routing config (RFC 8665
    // OSPFv2 / RFC 8666 OSPFv3): the SRGB/SRLB label ranges this node owns, the node
    // Prefix-SIDs it advertises, and the SR Mapping-Server preference. Used by the
    // top-level ospf container (IPv4) and every OSPFv3 address family (spec-ospf-ext-5).
    grouping ospf-segment-routing {
        container segment-routing {
            description "RFC 8665 (OSPFv2) / RFC 8666 (OSPFv3) Segment Routing over the MPLS data plane. Advertises SR-Algorithm 0, the SRGB/SRLB label ranges, and per-prefix Prefix-SIDs; installs label-switched forwarding through the shared mpls-fib. Off by default.";
            leaf enable { type boolean; default false; description "Enable Segment Routing for this address family. When true the RI LSA advertises SR-Algorithm 0 and the configured SRGB/SRLB."; }
            container srgb {
                description "Segment Routing Global Block: the contiguous global MPLS label range this node owns, mapped by Prefix-SID index (RFC 8665 sec 3.2). MUST NOT overlap the SRLB or the LDP/RSVP-TE label space.";
                leaf lower-bound { type uint32 { range "16..1048575"; } description "First MPLS label of the SRGB (inclusive)."; }
                leaf upper-bound { type uint32 { range "16..1048575"; } description "Last MPLS label of the SRGB (inclusive). Must be >= lower-bound."; }
            }
            container srlb {
                description "Segment Routing Local Block: the local MPLS label range Adjacency-SIDs are allocated from (RFC 8665 sec 3.3). MUST NOT overlap the SRGB or the LDP/RSVP-TE label space.";
                leaf lower-bound { type uint32 { range "16..1048575"; } description "First MPLS label of the SRLB (inclusive)."; }
                leaf upper-bound { type uint32 { range "16..1048575"; } description "Last MPLS label of the SRLB (inclusive). Must be >= lower-bound."; }
            }
            list prefix-sid {
                key "prefix";
                description "Prefix-SIDs advertised for local prefixes (typically the loopback). Each carries a SID index into this node's SRGB (RFC 8665 sec 5).";
                leaf prefix { type zt:ip-prefix; description "The local prefix (host loopback for a node-SID)."; }
                leaf index { type uint32; description "The SID index into the SRGB; the label is SRGB-base + index. Must be within the total SRGB size."; }
                leaf node-sid { type boolean; default true; description "Mark this as a node Prefix-SID (sets the N-Flag on the Extended Prefix TLV)."; }
                leaf no-php { type boolean; default false; description "Set the NP-Flag: the penultimate hop keeps the label rather than popping it (no penultimate-hop-popping)."; }
                leaf explicit-null { type boolean; default false; description "Set the E-Flag: the upstream neighbor uses the Explicit NULL label (0 IPv4 / 2 IPv6). Requires no-php."; }
            }
            leaf srms-preference { type uint8; description "SR Mapping-Server preference advertised in the SRMS-Preference TLV (RFC 8665 sec 3.4). Absent when unset."; }
        }
    }

    container ospf {
        description "OSPFv2 routing instance configuration.";
        ze:config-root "ospf";

        // RFC 2328 Appendix C.1: Router ID is a 32-bit AS-unique identifier.
        // When unset, Ze derives it from the highest loopback IPv4 address, then
        // the highest interface IPv4 address.
        leaf router-id {
            type string {
                pattern "([0-9]{1,3}\\.){3}[0-9]{1,3}";
            }
            ze:validate "ospf-router-id";
            description "OSPF Router ID in dotted-quad form; derived when omitted.";
        }

        leaf reference-bandwidth {
            type uint32 { range "1..4294967"; }
            default 100000;
            units Mbps;
            description "Auto-cost reference bandwidth in Mbps.";
        }

        leaf maximum-paths {
            type uint8 { range "1..32"; }
            default 8;
            description "Maximum ECMP paths per prefix.";
        }

        leaf opaque {
            type boolean;
            default false;
            description
                "Enable the RFC 5250 opaque-LSA capability. When true, this router sets
                 the O-bit in its Database Description packets (advertising it will
                 receive and forward opaque LSAs) and originates opaque LSAs for any
                 registered consumer. Received opaque LSAs are stored and reflooded by
                 their scope regardless of this leaf; disabling it only stops advertising
                 opaque capability and originating opaque LSAs.";
        }

        leaf extended-prefix {
            type boolean;
            default false;
            description
                "Originate RFC 7684 Extended Prefix Opaque LSAs (Opaque Type 7) that associate
                 attributes with this router's advertised prefixes. Requires opaque true. The
                 LSA is a container a prefix-attribute application (e.g. Segment Routing) fills
                 with sub-TLVs; it is off by default until such a producer needs it. Received
                 Extended Prefix LSAs are decoded and shown regardless of this leaf.";
        }

        leaf extended-link {
            type boolean;
            default false;
            description
                "Originate RFC 7684 Extended Link Opaque LSAs (Opaque Type 8) that associate
                 attributes with this router's links (one LSA per point-to-point/transit link,
                 mirroring the Router-LSA link). Requires opaque true. The LSA is a container a
                 link-attribute application (e.g. Segment Routing) fills with sub-TLVs; it is
                 off by default until such a producer needs it. Received Extended Link LSAs are
                 decoded and shown regardless of this leaf.";
        }

        leaf router-address {
            type zt:ipv4-address;
            description
                "RFC 3630 sec 2.4.1 Traffic Engineering Router Address: a stable,
                 always-reachable IPv4 address (typically a loopback) advertised once per
                 router in its Router-Address TE LSA. Defaults to the Router ID when unset.
                 Requires opaque and at least one traffic-engineering interface to take
                 effect.";
        }

        container fast-reroute {
            description
                "RFC 5286 Loop-Free Alternate (LFA) and TI-LFA IP fast reroute. When
                 enabled, SPF pre-computes a loop-free backup next-hop alongside each
                 primary and programs it into the FIB, so a single local link or node
                 failure is repaired locally before the IGP reconverges. TI-LFA mode adds
                 a Segment-Routing repair-list fallback where no directly-connected LFA
                 exists (requires segment-routing). Applies to OSPFv2 and OSPFv3; OSPFv3
                 gets base-LFA next-hop selection (SR repair labels are IPv4 only).";
            leaf enable {
                type boolean;
                default false;
                description "Enable LFA / TI-LFA fast-reroute backup computation and install.";
            }
            leaf mode {
                type enumeration {
                    enum lfa { description "Base loop-free alternates only (RFC 5286)."; }
                    enum ti-lfa { description "Add the Segment-Routing repair-list fallback (TI-LFA)."; }
                }
                default lfa;
                description "Backup computation mode: base LFA, or LFA with a TI-LFA SR-repair fallback.";
            }
            leaf node-protection {
                type boolean;
                default true;
                description "Prefer node-protecting alternates over link-only alternates (RFC 5286 Section 3.6).";
            }
        }

        container default-information {
            description "Default-route origination as an AS-external LSA.";
            leaf originate { type boolean; default false; description "Originate a default Type 5 LSA."; }
            leaf always { type boolean; default false; description "Originate even without a default in the RIB."; }
            leaf metric { type uint32 { range "0..16777215"; } default 1; description "Default LSA metric."; }
            leaf metric-type {
                type enumeration {
                    enum type-1;
                    enum type-2;
                }
                default "type-2";
                description "External metric type for the default route.";
            }
        }

        container timers {
            description "SPF and LSA throttle timers.";
            leaf spf-delay-ms { type uint32 { range "0..600000"; } default 50; units ms; description "Initial SPF delay."; }
            leaf spf-hold-ms { type uint32 { range "0..600000"; } default 200; units ms; description "SPF hold floor."; }
            leaf spf-max-hold-ms { type uint32 { range "0..600000"; } default 5000; units ms; description "SPF max hold."; }
            leaf min-ls-interval-ms { type uint32 { range "0..600000"; } default 5000; units ms; description "Minimum interval between LSA reoriginations."; }
            leaf min-ls-arrival-ms { type uint32 { range "0..600000"; } default 1000; units ms; description "Minimum arrival interval for accepting a new LSA instance."; }
        }

        container max-metric {
            description "RFC 6987 stub router: originate the Router-LSA with MaxLinkMetric so transit traffic avoids this router.";
            container router-lsa {
                leaf always { type boolean; default false; description "Always advertise as a stub router."; }
                leaf on-startup { type uint32 { range "0..86400"; } units seconds; description "Advertise as a stub router for N seconds after startup (0 disables)."; }
                leaf on-shutdown { type uint32 { range "0..86400"; } units seconds; description "Advertise as a stub router for N seconds during a graceful shutdown (0 disables)."; }
            }
        }

        container router-information {
            description "RFC 7770 Router Information (RI) LSA: advertise this router's optional capabilities (OSPFv2 opaque type 4, OSPFv3 function code 12). Applies to both address families; a top-level container drives the OSPFv3 family unless it configures its own.";
            leaf enabled { type boolean; default false; description "Originate the Router Information LSA advertising this router's informational capabilities. OSPFv2 also requires 'opaque true' (the RI LSA is an opaque LSA)."; }
            leaf-list scope {
                type enumeration {
                    enum link { description "Link-local (OSPFv2 opaque type 9)."; }
                    enum area { description "Area (OSPFv2 opaque type 10, OSPFv3 0xA00C)."; }
                    enum as { description "AS-wide (OSPFv2 opaque type 11, OSPFv3 0xC00C)."; }
                }
                description "Flooding scope(s) at which the RI LSA is advertised (RFC 7770 sec 2.7). When enabled with no scope listed, defaults to area + as.";
            }
        }

        container graceful-restart {
            description "RFC 3623 (OSPFv2) / RFC 5187 (OSPFv3) Graceful Restart: keep forwarding across a control-plane restart. Family-neutral: this container drives both address families (the OSPFv3 family inherits it unless it configures its own).";
            container restarter {
                description "This router's restarting-router role: originate Grace-LSAs and preserve the FIB across a restart.";
                leaf support {
                    type enumeration {
                        enum disabled { description "Never originate a Grace-LSA; restart normally (default)."; }
                        enum planned { description "Originate Grace-LSAs on an operator-triggered (planned) restart only."; }
                        enum planned-and-unplanned { description "Originate Grace-LSAs on planned and on unplanned (cold) restarts (RFC 3623 sec 5); the unplanned reason is restricted to unknown/redundant-cp."; }
                    }
                    default "disabled";
                    description "RFC 3623 Appendix B.1 RestartSupport.";
                }
                leaf restart-interval {
                    type uint16 { range "1..1800"; }
                    units seconds;
                    default 120;
                    description "RFC 3623 Appendix B.1 RestartInterval: the grace period neighbors keep advertising this router as fully adjacent. Should not exceed LSRefreshTime (1800 s) or this router's own LSAs age out mid-restart.";
                }
            }
            container helper {
                description "This router's helper role: hold an adjacency to a restarting neighbor and suppress LSDB churn for its grace period.";
                leaf support { type boolean; default true; description "RFC 3623 Appendix B.2 RestartHelperSupport: act as a helper for a restarting neighbor."; }
                leaf strict-lsa-checking { type boolean; default true; description "RFC 3623 Appendix B.2 RestartHelperStrictLSAChecking (sec 3.2): terminate helper mode when a changed LSA that would flood to the restarting router is installed."; }
            }
        }

        uses ospf-segment-routing;

        list redistribute {
            key "source";
            description "Route sources redistributed as AS-external LSAs.";
            leaf source {
                type enumeration {
                    enum connected;
                    enum static;
                    enum kernel;
                    enum bgp;
                    enum isis;
                }
                description "Route source to import.";
            }
            leaf metric { type uint32 { range "0..16777215"; } default 20; description "Injected metric."; }
            leaf metric-type {
                type enumeration {
                    enum type-1;
                    enum type-2;
                }
                default "type-2";
                description "External metric type.";
            }
            leaf tag { type uint32; default 0; description "External route tag."; }
        }

        container areas {
            description "OSPF areas.";
            list area {
                key "area-id";
                description "Per-area configuration.";
                leaf area-id {
                    type string {
                        pattern "([0-9]{1,10}|([0-9]{1,3}\\.){3}[0-9]{1,3})";
                    }
                    ze:validate "ospf-area-id";
                    description "Area identifier as uint32 or dotted quad; 0.0.0.0 is the backbone.";
                }
                leaf area-type {
                    type enumeration {
                        enum normal;
                        enum stub;
                        enum nssa;
                    }
                    default "normal";
                    description "Area type; semantics land in the stub/NSSA spec.";
                }
                leaf no-summary { type boolean; default false; description "Suppress Type 3 summaries in totally-stubby or totally-NSSA areas."; }
                leaf default-cost { type uint32 { range "0..16777215"; } default 1; description "Default summary metric for stub/NSSA areas."; }
                container nssa {
                    description "NSSA-specific configuration (applies when area-type is nssa).";
                    leaf translate-role {
                        type enumeration {
                            enum candidate;
                            enum always;
                            enum never;
                        }
                        default "candidate";
                        description "Type 7 to Type 5 translator role: candidate (elect by highest router-id), always (force translate), never (do not translate).";
                    }
                    leaf stability-interval {
                        type uint16 { range "0..65535"; }
                        units seconds;
                        default 40;
                        description "Hysteresis before a newly elected translator stops translating after losing the role (RFC 3101 section 3.5).";
                    }
                    leaf default-originate { type boolean; default false; description "Originate a Type 7 default route (0.0.0.0/0) into the NSSA."; }
                }
                container authentication {
                    description "Area-level authentication defaults.";
                    leaf key-chain { type string { length "1..63"; } description "Default key chain inherited by interfaces."; }
                }
                container ranges {
                    description "Inter-area summary ranges.";
                    list range {
                        key "prefix";
                        description "One area summary range.";
                        leaf prefix { type zt:prefix-ipv4; description "IPv4 aggregate prefix, for example 10.0.0.0/16."; }
                        leaf advertise {
                            type enumeration {
                                enum advertise;
                                enum not-advertise;
                            }
                            default "advertise";
                            description "Advertise the aggregate or suppress specifics.";
                        }
                        leaf cost { type uint32 { range "0..16777215"; } description "Override cost for the aggregate Type 3 LSA."; }
                    }
                }
                list virtual-link {
                    key "remote-router-id";
                    description
                        "Virtual link through this (transit) area to a backbone area-border
                         router (RFC 2328 section 15). The virtual link belongs to the backbone;
                         its output cost is computed from the transit-area SPF, never configured.
                         The transit area must not be the backbone, a stub, or an NSSA.";
                    leaf remote-router-id {
                        type string {
                            pattern "([0-9]{1,3}\\.){3}[0-9]{1,3}";
                        }
                        ze:validate "ospf-router-id";
                        description "Router ID of the far virtual-link endpoint (the other area-border router).";
                    }
                    leaf hello-interval { type uint16 { range "1..65535"; } default 10; units seconds; description "Hello interval on the virtual link."; }
                    leaf dead-interval { type uint16 { range "1..65535"; } default 40; units seconds; description "Router-dead interval on the virtual link."; }
                    leaf retransmit-interval { type uint16 { range "1..65535"; } default 5; units seconds; description "LSA retransmit interval on the virtual link."; }
                    leaf transmit-delay { type uint16 { range "1..3600"; } default 1; units seconds; description "Estimated LSA transmission delay (RFC 2328 InfTransDelay must be > 0)."; }
                }
            }
        }

        container interfaces {
            description "OSPF-enabled interfaces.";
            list interface {
                key "name";
                description "Per-interface OSPF configuration.";
                leaf name { type string { length "1..255"; } description "Interface name."; }
                leaf area {
                    type string {
                        pattern "([0-9]{1,10}|([0-9]{1,3}\\.){3}[0-9]{1,3})";
                    }
                    ze:validate "ospf-area-id";
                    description "Declared area this interface belongs to.";
                }
                leaf enabled { type boolean; default true; description "OSPF enabled on this interface."; }
                leaf network-type {
                    type enumeration {
                        enum broadcast;
                        enum point-to-point;
                        enum nbma;
                        enum point-to-multipoint;
                        enum loopback;
                    }
                    default "broadcast";
                    description "Interface network type. nbma elects a DR/BDR over a configured neighbor list with unicast/poll Hellos (RFC 2328); point-to-multipoint treats the link as a collection of point-to-point links with per-neighbor host routes and no DR.";
                }
                leaf cost { type uint16 { range "1..65535"; } description "Interface output cost."; }
                leaf hello-interval { type uint16 { range "1..65535"; } default 10; units seconds; description "Hello interval."; }
                leaf dead-interval { type uint16 { range "1..65535"; } default 40; units seconds; description "Router-dead interval."; }
                leaf poll-interval { type uint16 { range "1..65535"; } default 120; units seconds; description "NBMA poll interval: the slower Hello rate sent to a configured but silent neighbor (RFC 2328 App C.5)."; }
                leaf priority { type uint8 { range "0..255"; } default 1; description "DR/BDR election priority; 0 means ineligible."; }
                list nbma-neighbor {
                    key "address";
                    description "Statically configured NBMA neighbor (RFC 2328 App C.6); required for network-type nbma or the non-broadcast point-to-multipoint variant.";
                    leaf address { type string; ze:validate "ipv4-address"; description "Neighbor IPv4 interface address (the unicast Hello destination)."; }
                    leaf priority { type uint8 { range "0..255"; } default 0; description "Neighbor DR/BDR eligibility; 0 means ineligible (polled, never elected)."; }
                }
                leaf passive { type boolean; default false; description "Advertise the interface but form no adjacency."; }
                leaf mtu-ignore { type boolean; default false; description "Skip DD MTU mismatch rejection."; }
                leaf retransmit-interval { type uint16 { range "1..65535"; } default 5; units seconds; description "LSA retransmit interval."; }
                leaf transmit-delay { type uint16 { range "1..3600"; } default 1; units seconds; description "Estimated LSA transmission delay (RFC 2328 InfTransDelay must be > 0)."; }
                leaf-list instance-id {
                    type uint8 { range "0..255"; }
                    description
                        "OSPFv2 Multi-Instance Instance ID(s) this interface participates in
                         (RFC 6549 section 3). Absent means the base instance 0 only, which is
                         bit-for-bit compatible with base OSPFv2. Each listed value runs a
                         separate OSPFv2 instance demultiplexed on this subnet; a received packet
                         whose Instance ID matches none of them is discarded (section 2/3.1).";
                }
                container authentication {
                    description "Per-interface authentication settings.";
                    leaf mode {
                        type enumeration {
                            enum inherit;
                            enum none;
                            enum simple;
                            enum md5;
                            enum hmac-sha-1;
                            enum hmac-sha-256;
                            enum hmac-sha-384;
                            enum hmac-sha-512;
                        }
                        default "inherit";
                        description "Authentication mode; inherit uses the area default.";
                    }
                    leaf key-chain { type string { length "1..63"; } description "Key chain reference."; }
                }
                container ldp-sync {
                    description
                        "LDP-IGP synchronization (RFC 5443, RFC 6138): hold the link at
                         maximum cost (point-to-point) or withhold its transit link
                         (broadcast, non-cut-edge) until LDP is synchronized, so transit
                         traffic is not black-holed before the label bindings exist.";
                    leaf enable { type boolean; default false; description "Enable LDP-IGP synchronization on this interface."; }
                    leaf holddown {
                        type uint16 { range "0..65535"; }
                        default 0;
                        units seconds;
                        description
                            "Seconds to wait after the LDP session is established before
                             declaring the link synchronized (RFC 5443 section 2 estimation
                             that all label bindings are exchanged; the RFC defines no
                             universal default). 0 restores the cost immediately once the
                             session is up (allowed but discouraged).";
                    }
                }
                container traffic-engineering {
                    description
                        "RFC 3630 / RFC 5392 Traffic Engineering link attributes advertised
                         in a Type 10 (or, for inter-as, Type 10/11) opaque TE LSA. Requires
                         the top-level opaque leaf. The TE metric is independent of cost.";
                    leaf enable { type boolean; default false; description "Advertise a TE Link LSA for this interface."; }
                    leaf te-metric {
                        type uint32 { range "0..4294967295"; }
                        description "RFC 3630 sec 2.5.5 Traffic Engineering metric; independent of the OSPF cost, defaulting to it when unset.";
                    }
                    leaf max-bandwidth {
                        type uint64;
                        units bytes/second;
                        description "RFC 3630 sec 2.5.6 Maximum Bandwidth (true link capacity) in bytes/second.";
                    }
                    leaf max-reservable-bandwidth {
                        type uint64;
                        units bytes/second;
                        description "RFC 3630 sec 2.5.7 Maximum Reservable Bandwidth in bytes/second; defaults to max-bandwidth.";
                    }
                    leaf admin-group {
                        type uint32;
                        description "RFC 3630 sec 2.5.9 Administrative Group (Resource Class/Color) 32-bit mask; LSB is group 0.";
                    }
                    container inter-as {
                        presence "Advertise this as an RFC 5392 inter-AS TE link (Opaque type 6).";
                        description
                            "RFC 5392 inter-AS TE link. No OSPF adjacency is formed on the link
                             (sec 4); the ASBR proxies the link into its own AS. Requires
                             remote-as and at least one remote-asbr address.";
                        leaf remote-as {
                            type zt:asn;
                            mandatory true;
                            description "RFC 5392 sec 3.3.1 Remote AS Number of the neighboring AS the link connects to.";
                        }
                        leaf remote-asbr-ipv4 {
                            type zt:ipv4-address;
                            description "RFC 5392 sec 3.3.2 IPv4 Remote ASBR ID (sub-TLV 22); recommended to be the remote TE Router ID.";
                        }
                        leaf remote-asbr-ipv6 {
                            type zt:ipv6-address;
                            description "RFC 5392 sec 3.3.3 IPv6 Remote ASBR ID (sub-TLV 24, not 23); a stable global IPv6 address.";
                        }
                        leaf scope {
                            type enumeration {
                                enum area;
                                enum as;
                            }
                            default "area";
                            description "RFC 5392 sec 3.1.1 flooding scope policy: area (Type 10, limited to the ASBR's area) or as (Type 11, AS-wide).";
                        }
                    }
                }
            }
        }

        list key-chains {
            key "name";
            description "Named authentication key chains for hitless rotation.";
            leaf name { type string { length "1..63"; } description "Key-chain name."; }
            leaf extended-sequence { type boolean; default false; description "Use RFC 7474 AuType 3 (extended 64-bit cryptographic sequence numbers) instead of AuType 2; applies to the HMAC-SHA algorithms."; }
            list key {
                key "key-id";
                description "Keys in this chain.";
                leaf key-id { type uint32; description "Key identifier."; }
                leaf algorithm {
                    type enumeration {
                        enum simple;
                        enum md5;
                        enum hmac-sha-1;
                        enum hmac-sha-256;
                        enum hmac-sha-384;
                        enum hmac-sha-512;
                    }
                    default "md5";
                    description "Authentication algorithm.";
                }
                leaf secret {
                    type string { length "1..255"; }
                    ze:sensitive;
                    description "Shared secret, masked and $9$-encoded at rest.";
                }
                container send-lifetime {
                    description "When this key may be used to sign.";
                    leaf start { type string { length "1..64"; } description "RFC3339 start timestamp."; }
                    leaf end { type string { length "1..64"; } description "RFC3339 end timestamp."; }
                }
                container accept-lifetime {
                    description "When this key is accepted on receive.";
                    leaf start { type string { length "1..64"; } description "RFC3339 start timestamp."; }
                    leaf end { type string { length "1..64"; } description "RFC3339 end timestamp."; }
                }
                container bfd {
                    description "RFC 5880 / RFC 5881 single-hop BFD for this OSPF interface. When enabled, a Full adjacency opens a single-hop BFD session; a BFD-detected failure declares the neighbor down far faster than the router-dead interval.";
                    leaf enabled { type boolean; default false; description "Enable single-hop BFD failure detection on this interface."; }
                    leaf min-tx { type uint32 { range "1..10000"; } default 50; units milliseconds; description "Desired minimum BFD transmit interval (RFC 5880 Desired Min TX Interval)."; }
                    leaf min-rx { type uint32 { range "1..10000"; } default 50; units milliseconds; description "Required minimum BFD receive interval (RFC 5880 Required Min RX Interval)."; }
                    leaf multiplier { type uint8 { range "1..255"; } default 3; description "BFD detection multiplier (RFC 5880 Detect Mult)."; }
                }
            }
        }

        container address-family {
            description
                "Additional OSPF address families. The IPv6 (OSPFv3, RFC 5340) family runs as a
                 second engine instance over the ospfv3 transport (ff02::5/6), sharing the
                 FSM/flooding/SPF machinery with the IPv4 family.";
            container ipv6 {
                description "Default IPv6-unicast OSPFv3 address family (RFC 5340); the bare
                    `ipv6` spelling is the IPv6-unicast AF. Presence enables the v6 instance.";
                leaf instance-id {
                    type uint8 { range "0..31"; }
                    default 0;
                    description "OSPFv3 Instance ID (RFC 5340 §2.5); the RFC 5838 §2.1 IPv6-unicast range is 0-31.";
                }
                uses ospf-af-topology;
            }
            container ipv6-unicast {
                description "IPv6-unicast OSPFv3 address family (RFC 5838 §2.1: Instance ID 0-31).";
                leaf instance-id {
                    type uint8 { range "0..31"; }
                    default 0;
                    description "OSPFv3 Instance ID; the RFC 5838 §2.1 IPv6-unicast range is 0-31.";
                }
                uses ospf-af-topology;
            }
            container ipv6-multicast {
                description "IPv6-multicast OSPFv3 address family (RFC 5838 §2.1: Instance ID 32-63).
                    Reachability is computed unicast-shaped; MOSPF tree computation is not implemented.";
                leaf instance-id {
                    type uint8 { range "32..63"; }
                    default 32;
                    description "OSPFv3 Instance ID; the RFC 5838 §2.1 IPv6-multicast range is 32-63.";
                }
                uses ospf-af-topology;
            }
            container ipv4-unicast {
                description "IPv4-unicast over OSPFv3 (RFC 5838 §2.1/§2.7: Instance ID 64-95). Carries
                    IPv4 prefixes in the address-free OSPFv3 LSA model; routes install into the IPv4 RIB.";
                leaf instance-id {
                    type uint8 { range "64..95"; }
                    default 64;
                    description "OSPFv3 Instance ID; the RFC 5838 §2.1 IPv4-unicast range is 64-95.";
                }
                uses ospf-af-topology;
            }
            container ipv4-multicast {
                description "IPv4-multicast over OSPFv3 (RFC 5838 §2.1: Instance ID 96-127).
                    Reachability is computed unicast-shaped; MOSPF tree computation is not implemented.";
                leaf instance-id {
                    type uint8 { range "96..127"; }
                    default 96;
                    description "OSPFv3 Instance ID; the RFC 5838 §2.1 IPv4-multicast range is 96-127.";
                }
                uses ospf-af-topology;
            }
        }
    }
}
```

## Policy (`policy`, 1 plugins)

### policy-routes

source: `internal/plugins/policyroute` -- config root: `policy` -- depends on: `firewall`

Policy-based routing: nftables packet marking and ip rule table selection

`ze-policyroute-cmd.yang`

```yang
module ze-policyroute-cmd {
    namespace "urn:ze:policyroute:cmd";
    prefix policyroutecmd;
    import ze-extensions { prefix ze; }
    description "show policy-routes command tree. Owned by the policyroute plugin so that removing it removes the command node together with the handler. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated show policy-routes out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container policy-routes {
            config false;
            ze:command "ze-show:policy-routes";
            description "Show policy-based routing rules.
Lists PBR rules with match criteria and routing actions.";
        }
    }
}
```

`ze-policyroute-conf.yang`

```yang
module ze-policyroute-conf {
    namespace "urn:ze:policyroute:conf";
    prefix policyroute;

    import ze-extensions { prefix ze; }

    description
        "Policy-based routing configuration for Ze.
         Steers packets to alternate routing tables or next-hops
         based on L3/L4 match criteria. Implemented via nftables
         packet marking and ip rule table selection.";

    revision 2026-04-23 {
        description "Initial revision.";
    }

    container policy {
        description "Policy routing configuration.";

        list route {
            key "name";
            description "A named policy route applied to ingress interfaces.";

            leaf name {
                type string;
                description "Policy route name.";
            }

            leaf-list interface {
                type string;
                description
                    "Ingress interface(s) to match. A trailing '*' enables
                     prefix (wildcard) matching (e.g. 'l2tp*').";
            }

            list rule {
                key "name";
                ordered-by user;
                description "Ordered list of match/action rules.";

                leaf name {
                    type string;
                    description "Rule name.";
                }

                leaf order {
                    type uint32;
                    description
                        "Evaluation order (lower values first). Rules with
                         equal order are sorted by name. If omitted,
                         defaults to 0.";
                }

                container from {
                    description "Packet match criteria.";

                    leaf source-address {
                        type string;
                        ze:validate "ipv4-prefix|ipv6-prefix|set-ref";
                        description "Source IP prefix or @set reference.";
                    }

                    leaf destination-address {
                        type string;
                        ze:validate "ipv4-prefix|ipv6-prefix|set-ref";
                        description "Destination IP prefix or @set reference.";
                    }

                    leaf source-port {
                        type string;
                        ze:validate "port-spec";
                        description "Source port or port range (e.g. '80,443').";
                    }

                    leaf destination-port {
                        type string;
                        ze:validate "port-spec";
                        description "Destination port or port range.";
                    }

                    leaf protocol {
                        type string;
                        description "L4 protocol name (tcp, udp, icmp).";
                    }

                    leaf tcp-flags {
                        type string;
                        description
                            "Comma-separated TCP flags to match
                             (fin, syn, rst, psh, ack, urg).";
                    }
                }

                container then {
                    description "Action for matching packets.";

                    leaf accept {
                        type empty;
                        description "Skip this policy (packet routes normally).";
                    }

                    leaf drop {
                        type empty;
                        description "Drop matching packets.";
                    }

                    leaf table {
                        type uint32 {
                            range "1..999|3000..max";
                        }
                        description
                            "Route matching packets via this kernel routing
                             table. Range 1000-2999 is reserved for ze
                             internal use (VRF and policy-routing auto tables).";
                    }

                    leaf next-hop {
                        type string;
                        ze:validate "ipv4-address|ipv6-address";
                        description
                            "Redirect matching packets to this next-hop.
                             Ze auto-allocates a kernel routing table
                             from range 2000-2999 and manages the default
                             route, fwmark, and ip rule internally.";
                    }

                    leaf tcp-mss {
                        type uint16 {
                            range "1..65535";
                        }
                        description "Clamp TCP MSS to this value (bytes).";
                    }
                }
            }
        }
    }
}
```

## Redistribute (`redistribute`, 1 plugins)

### redistribute-orchestrator

source: `internal/component/bgp/plugins/redistribute_egress` -- config root: `redistribute` -- depends on: `bgp`

Redistribute orchestrator: dispatches protocol route events to registered consumers

No YANG module of its own (reads config defined by another plugin, or has none).

## Rib (`rib`, 1 plugins)

### rib

source: `internal/component/sysrib` -- config root: `rib`

System RIB: selects best route across protocols by admin distance

`ze-rib-conf.yang`

```yang
module ze-rib-conf {
    namespace "urn:ze:rib:conf";
    prefix rib;

    description
        "System RIB configuration for Ze.
         Admin distance settings control cross-protocol route selection.
         Lower distance wins. Used by the rib plugin to override
         incoming priority values from protocol RIBs.";

    revision 2026-04-04 {
        description "Add admin-distance container.";
    }

    container rib {
        description "System RIB configuration.";

        container admin-distance {
            description
                "Administrative distance per protocol.
                 Lower value wins. Used by the system RIB to select
                 the best route across protocols.";

            leaf connected {
                type uint8;
                default 0;
                description "Directly connected networks.";
            }

            leaf static {
                type uint8;
                default 10;
                description "Static routes.";
            }

            leaf ebgp {
                type uint8;
                default 20;
                description "External BGP routes.";
            }

            leaf ospf {
                type uint8;
                default 110;
                description "OSPF routes.";
            }

            leaf isis {
                type uint8;
                default 115;
                description "IS-IS routes.";
            }

            leaf ibgp {
                type uint8;
                default 200;
                description "Internal BGP routes.";
            }
        }
    }
}
```

## Routing Table (`routing-table`, 1 plugins)

### routing-table

source: `internal/plugins/routingtable` -- config root: `routing-table`

Named routing table registry: maps names to kernel table IDs

`ze-routing-table-conf.yang`

```yang
module ze-routing-table-conf {
    namespace "urn:ze:routing-table:conf";
    prefix routing-table;

    import ze-extensions { prefix ze; }

    description
        "Named routing table registry for Ze.
         Maps human-readable names to kernel table IDs.
         Used by static routes and policy routing to reference
         tables by name. The name 'default' is built-in and
         maps to table 0 (kernel RT_TABLE_MAIN 254).";

    revision 2026-05-15 {
        description "Initial revision.";
    }

    container routing-table {
        description
            "Named routing table definitions. Each entry maps
             a name to a kernel routing table ID.";

        list table {
            key "name";
            description "A named routing table.";

            leaf name {
                type string;
                description
                    "Table name referenced by static routes
                     and policy routing configuration.";
            }

            leaf id {
                type uint32 {
                    range "1..252 | 256..4294967295";
                }
                mandatory true;
                description
                    "Kernel routing table ID. Reserved IDs
                     excluded: 0 (use 'default'), 253-255
                     (kernel reserved).";
            }
        }
    }
}
```

## Rsvp Te (`rsvp-te`, 1 plugins)

### rsvp-te-rawsock

source: `internal/plugins/rsvpte` -- config root: `rsvp-te` -- depends on: `fib-kernel`

RSVP-TE: Resource Reservation Protocol - Traffic Engineering (RFC 3209)

`ze-rsvp-te-cmd.yang`

```yang
module ze-rsvp-te-cmd {
    namespace "urn:ze:rsvp-te:cmd";
    prefix rsvptecmd;
    import ze-extensions { prefix ze; }
    description "show rsvp-te ... command tree. Owned by the rsvp-te component so that removing it removes these command nodes together with the handlers. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated show rsvp-te ... out of the central show schema (plugin self-containment)."; }
    revision 2026-06-19 { description "Add show rsvp-te fast-reroute (RFC 4090 protection state)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container rsvp-te {
            config false;
            description "RSVP-TE LSPs, tunnels, and bandwidth reservations";

            container lsp {
                config false;
                ze:command "ze-show:rsvp-te-lsp";
                description "Show RSVP-TE label-switched paths.
Returns state, role (ingress/transit/egress), reserved bandwidth,
and in/out labels for each LSP.";
            }

            container interface {
                config false;
                ze:command "ze-show:rsvp-te-interface";
                description "Show RSVP-TE bandwidth allocation per interface.
Returns reserved, available, and maximum bandwidth for each
TE-enabled interface.";
            }

            container tunnel {
                config false;
                ze:command "ze-show:rsvp-te-tunnel";
                description "Show configured RSVP-TE tunnels and their current state.
Returns tunnel name, endpoints, signaling state, and active LSP.";
            }

            container fast-reroute {
                config false;
                ze:command "ze-show:rsvp-te-fast-reroute";
                description "Show RSVP-TE Fast Reroute (RFC 4090) protection state.
Returns each configured facility-backup bypass LSP and each protected
LSP with its armed bypass, mode, and whether local protection is
available and in use.";
            }
        }
    }
}
```

`ze-rsvp-te-conf.yang`

```yang
module ze-rsvp-te-conf {
    namespace "urn:ze:rsvp-te:conf";
    prefix rsvpteconf;
    import ze-extensions { prefix ze; }
    description "RSVP-TE protocol configuration (RFC 3209)";
    revision 2026-05-28 { description "Initial revision"; }

    container rsvp-te {
        description "RSVP-TE traffic engineering configuration";

        leaf router-id {
            type string;
            description "Router identifier (IPv4 address format)";
        }

        leaf refresh-period {
            type uint16 {
                range "1..65535";
            }
            default 30;
            units seconds;
            description "PATH/RESV refresh interval (RFC 2205 soft-state)";
        }

        leaf refresh-multiplier {
            type uint8 {
                range "1..255";
            }
            default 3;
            description "Number of missed refreshes before state cleanup";
        }

        list interface {
            key "name";
            description "Per-interface RSVP-TE configuration";

            leaf name {
                type string;
                description "Interface name";
            }

            leaf max-bandwidth {
                type string;
                description "Maximum link bandwidth (bps)";
            }

            leaf max-reservable-bandwidth {
                type string;
                description "Maximum reservable bandwidth (bps)";
            }

            leaf address {
                type string;
                description "Local link prefix (IPv4 CIDR, e.g. 10.0.0.4/30). When more than one interface is configured, admission control maps an LSP to this interface when the neighbor address falls within the prefix.";
            }
        }

        list tunnel {
            key "name";
            description "RSVP-TE LSP tunnel definition";

            leaf name {
                type string;
                description "Tunnel name";
            }

            leaf destination {
                type string;
                description "Tunnel endpoint (IPv4 address)";
            }

            leaf tunnel-id {
                type uint16;
                description "Tunnel identifier";
            }

            leaf bandwidth {
                type string;
                description "Requested bandwidth (bps)";
            }

            leaf setup-priority {
                type uint8 {
                    range "0..7";
                }
                default 7;
                description "Setup priority (0 = highest)";
            }

            leaf hold-priority {
                type uint8 {
                    range "0..7";
                }
                default 7;
                description "Hold priority (0 = highest)";
            }

            container fast-reroute {
                presence "request RFC 4090 local protection for this tunnel";
                description "Fast Reroute (RFC 4090) local protection for this LSP";

                leaf backup {
                    type enumeration {
                        enum facility;
                        enum one-to-one;
                    }
                    default facility;
                    description "Backup method: facility (one bypass protects many LSPs) or one-to-one (a detour per LSP)";
                }

                leaf node-protection {
                    type boolean;
                    default false;
                    description "Request NNHOP (node) protection rather than NHOP (link) protection";
                }

                leaf bandwidth-protection {
                    type boolean;
                    default false;
                    description "Request a backup that guarantees the reserved bandwidth";
                }

                leaf hop-limit {
                    type uint8 {
                        range "0..255";
                    }
                    default 16;
                    description "Maximum number of hops the backup path may take";
                }
            }

            list explicit-route {
                key "index";
                ordered-by user;
                description "Explicit route hops";

                leaf index {
                    type uint16;
                    description "Hop index";
                }

                leaf address {
                    type string;
                    description "Hop address (IPv4 prefix)";
                }

                leaf type {
                    type enumeration {
                        enum strict;
                        enum loose;
                    }
                    default strict;
                    description "Hop type (strict or loose)";
                }
            }
        }

        list bypass {
            key "name";
            description "Facility-backup bypass LSP (RFC 4090 Section 3.2): an LSP from this PLR to a merge point, explicitly routed to avoid the protected resource. A protected transit LSP whose next hop (link protection) or next-next hop (node protection) is this bypass's merge point is redirected onto it on a local failure. Explicit because ze has no IGP/CSPF to auto-compute a backup path.";

            leaf name {
                type string;
                description "Bypass name";
            }

            leaf merge-point {
                type string;
                description "Merge point address (IPv4): the protected LSPs' next hop (link protection) or next-next hop (node protection) where this bypass rejoins them";
            }

            leaf node-protection {
                type boolean;
                default false;
                description "This bypass merges at a next-next hop, providing node protection";
            }

            list explicit-route {
                key "index";
                ordered-by user;
                description "Bypass path hops (must avoid the protected resource)";

                leaf index {
                    type uint16;
                    description "Hop index";
                }

                leaf address {
                    type string;
                    description "Hop address (IPv4 prefix)";
                }

                leaf type {
                    type enumeration {
                        enum strict;
                        enum loose;
                    }
                    default strict;
                    description "Hop type (strict or loose)";
                }
            }
        }
    }
}
```

## Service (`service`, 5 plugins)

### as112

source: `internal/plugins/as112` -- config root: `service`

AS112 anycast DNS node: authoritative sink for misdirected RFC 1918 / link-local reverse-DNS queries (RFC 7534, RFC 7535)

`ze-as112-cmd.yang`

```yang
module ze-as112-cmd {
    namespace "urn:ze:as112:cmd";
    prefix as112cmd;

    import ze-extensions { prefix ze; }

    description
        "show as112 and as112 health command tree. Owned by the as112 plugin
         because both handlers read the as112 server's live state. See
         ai/rules/plugin-self-containment.md.";

    revision 2026-07-01 {
        description "Initial revision";
    }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container as112 {
            config false;
            ze:command "ze-show:as112";
            description
                "AS112 node status: enabled, address-family, hostname/
                 facility/location, allow-from count, served zone count, and
                 the current SOA serial.";
        }
    }

    container as112 {
        config false;
        description "AS112 operational commands";

        container health {
            config false;
            ze:command "ze-as112:health";
            description
                "One-shot authoritative query against an anycast service
                 address (or the given target), exit 0 iff the expected
                 AS112 answer comes back. Finding M4: the tool child 3's
                 healthcheck probe calls, since dig is not on the gokrazy
                 appliance and 'ze resolve dns' cannot target a specific
                 server. Usage: as112 health [target <ip>].";

            leaf target {
                type string;
                description "Anycast service address to query; defaults to the
                     address-family-appropriate on-box loopback when omitted
                     (127.0.0.1, or ::1 when address-family is ipv6-only)";
            }
        }
    }
}
```

`ze-as112-conf.yang`

```yang
module ze-as112-conf {
    namespace "urn:ze:as112:conf";
    prefix as112;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description
        "AS112 anycast DNS node configuration for Ze: authoritative sink for
         misdirected RFC 1918 / link-local reverse-DNS queries (RFC 7534) and
         the EMPTY.AS112.ARPA DNAME-redirection sink (RFC 7535). The four
         anycast host addresses and every served zone are fixed -- no
         operator-typed IP address anywhere in this module.";

    revision 2026-07-01 {
        description "Initial revision";
    }

    container service {
        description "Service settings";

        container as112 {
            description "AS112 anycast DNS node";

            leaf enabled {
                type boolean;
                default false;
                description "Enable the AS112 anycast DNS node";
            }

            leaf address-family {
                type enumeration {
                    enum both {
                        description "Serve on both IPv4 and IPv6 anycast addresses (default)";
                    }
                    enum ipv4-only {
                        description "Serve on the two IPv4 anycast addresses only";
                    }
                    enum ipv6-only {
                        description "Serve on the two IPv6 anycast addresses only";
                    }
                }
                default "both";
                description
                    "Restrict the service to one address family (RFC 7534
                     Section 3.4 / RFC 7535 Section 3.1 single-stack option).
                     The anycast addresses themselves are fixed constants,
                     never operator-typed.";
            }

            leaf hostname {
                type string {
                    length "0..63";
                }
                description
                    "Node identification string surfaced in the
                     HOSTNAME.AS112.NET/ARPA TXT answers (RFC 7534 Section
                     3.5), so operators can tell which anycast instance
                     answered a given query. Empty omits the TXT string.";
            }

            leaf facility {
                type string {
                    length "0..100";
                }
                description
                    "Facility/site name surfaced alongside location in the
                     HOSTNAME.AS112.NET/ARPA TXT answers, e.g. 'Example
                     Datacenter'.";
            }

            leaf location {
                type string {
                    length "0..100";
                }
                description
                    "City/country surfaced alongside facility in the
                     HOSTNAME.AS112.NET/ARPA TXT answers, e.g. 'London, UK'.";
            }

            leaf-list allow-from {
                type zt:ip-prefix;
                description
                    "Optional client-source access list. Empty/unset (default)
                     answers every source, matching standard AS112 public-sink
                     behavior. When non-empty, only queries whose source IP is
                     contained in one of these prefixes are answered; all
                     others are silently dropped (no response). Loopback/
                     on-box sources are always implicitly permitted regardless
                     of this list, so the 'as112 health' probe is never
                     blocked. Setting this makes the node non-public --
                     correct for a local-use mirror, wrong for a
                     globally-reachable AS112 contributor.";
            }

            // Schema anchors for cross-service port-conflict detection
            // (finding L3): these two lists sit directly under container
            // as112 (NOT inside a wrapper container) because a `config
            // false` container with zero operator-typeable content never
            // materializes in the parsed config Tree (empirically verified:
            // internal/component/config's tree walk stops at the first
            // GetContainer() that returns nil), which silently made the
            // former ipv4-anycast/ipv6-anycast wrapper-container shape a
            // permanent no-op regardless of RegisterListenerDefault calls.
            // container as112 itself always materializes once `enabled` is
            // committed, so anchoring the lists there -- one level up --
            // fixes materialization. As a side effect the derived
            // listenerService now inherits as112's "enabled" leaf as its
            // gate (hasEnabledLeaf=true, matching geodns's own listener
            // gating precedent): the conflict check only fires while as112
            // is actually enabled, which is the correct semantic (a
            // disabled service is not occupying the port).
            list ipv4-anycast-listener {
                key "name";
                config false;
                ze:listener;
                description
                    "Presence-only anchor: as112 always occupies port 53
                     on its fixed IPv4 anycast addresses while enabled.
                     The Go-level default (RegisterListenerDefault) fills
                     in the representative address, since this list is
                     never populated by any operator-facing command.";

                leaf name {
                    type string;
                    description "Fixed entry name";
                }

                uses zt:listener {
                    refine ip {
                        default "192.175.48.1";
                    }
                    refine port {
                        default 53;
                    }
                }
            }

            list ipv6-anycast-listener {
                key "name";
                config false;
                ze:listener;
                description
                    "Presence-only anchor: as112 always occupies port 53
                     on its fixed IPv6 anycast addresses while enabled.";

                leaf name {
                    type string;
                    description "Fixed entry name";
                }

                uses zt:listener {
                    refine ip {
                        default "2620:4f:8000::1";
                    }
                    refine port {
                        default 53;
                    }
                }
            }
        }
    }
}
```

### dhcpserver

source: `internal/plugins/dhcpserver` -- config root: `service`

DHCP server: address assignment for LAN clients (RFC 2131)

`ze-dhcp-server-conf.yang`

```yang
module ze-dhcp-server-conf {
    namespace "urn:ze:dhcp-server:conf";
    prefix dhcpsrv;

    description "DHCP server configuration for Ze (RFC 2131/2132)";

    revision 2026-01-01 {
        description "Initial revision";
    }

    container service {
        description "Service settings";

        container dhcp-server {
            description "DHCP server settings";

            leaf enabled {
                type boolean;
                default false;
                description "Enable DHCP server";
            }

            leaf-list listen-interface {
                type string;
                description "Interfaces to serve DHCP on";
            }

            container pxe {
                description "PXE boot server settings (RFC 4578)";

                leaf enabled {
                    type boolean;
                    default false;
                    description "Enable PXE boot option injection";
                }

                leaf tftp-server {
                    type string;
                    description "TFTP server IP address for PXE boot (option 66, siaddr)";
                }

                leaf bootfile-bios {
                    type string;
                    description "Boot file path for BIOS clients (option 67)";
                }

                leaf bootfile-uefi {
                    type string;
                    description "Boot file path for UEFI clients (option 67)";
                }

                leaf boot-script-url {
                    type string;
                    description "HTTP URL for iPXE boot script (sent as option 67 to iPXE clients)";
                }
            }

            list shared-network {
                key "name";
                description "Named network grouping of subnets";

                leaf name {
                    type string;
                    description "Shared network name";
                }

                list subnet {
                    key "prefix";
                    description "Subnet with address pool and options";

                    leaf prefix {
                        type string;
                        description "Subnet prefix (e.g. 192.168.1.0/24)";
                    }

                    list range {
                        key "name";
                        description "Named dynamic address pool range";

                        leaf name {
                            type string;
                            description "Range name";
                        }

                        leaf start {
                            type string;
                            description "First allocatable address";
                        }

                        leaf stop {
                            type string;
                            description "Last allocatable address";
                        }
                    }

                    leaf lease-time {
                        type uint32 {
                            range "60..604800";
                        }
                        default 86400;
                        description "Lease duration in seconds";
                    }

                    leaf default-router {
                        type string;
                        description "Default gateway address (option 3)";
                    }

                    leaf-list dns-server {
                        type string;
                        description "DNS server addresses (option 6)";
                    }

                    leaf domain-name {
                        type string;
                        description "Domain name for clients (option 15)";
                    }

                    list static-mapping {
                        key "name";
                        description "Static MAC-to-IP binding";

                        leaf name {
                            type string;
                            description "Mapping name";
                        }

                        leaf mac-address {
                            type string;
                            description "Client MAC address (xx:xx:xx:xx:xx:xx)";
                        }

                        leaf ip-address {
                            type string;
                            description "Fixed IP address for this client";
                        }
                    }
                }
            }
        }
    }
}
```

### geodns

source: `internal/plugins/geodns` -- config root: `service`

GeoDNS server: DNS answers selected by client source IP (RFC 1035, RFC 7871 client-subnet)

`ze-geodns-cmd.yang`

```yang
module ze-geodns-cmd {
    namespace "urn:ze:geodns:cmd";
    prefix geodnscmd;

    import ze-extensions { prefix ze; }

    description
        "show geodns command tree. Owned by the geodns plugin because the handler
         reads the geodns server's live state. See ai/rules/plugin-self-containment.md.";

    revision 2026-06-26 {
        description "Initial revision";
    }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container geodns {
            config false;
            ze:command "ze-show:geodns";
            description
                "GeoDNS server status: enabled, bind addresses/port, client-IP
                 source mode, zones, nameserver/host-set/source counts, and the
                 current SOA serial.";
        }
    }
}
```

`ze-geodns-conf.yang`

```yang
module ze-geodns-conf {
    namespace "urn:ze:geodns:conf";
    prefix geodns;

    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }

    description
        "GeoDNS server configuration for Ze: DNS answers are selected by the
         client source IP (RFC 1035 records, RFC 7871 EDNS0 client-subnet).
         Ported from the EXA SurfProtect geodns daemon; the per-source host
         files become YANG host-sets referenced by source prefixes.";

    revision 2026-06-26 {
        description "Initial revision";
    }

    container service {
        description "Service settings";

        container geodns {
            description "GeoDNS server: per-source-IP DNS answers";

            leaf enabled {
                type boolean;
                default false;
                description "Enable the GeoDNS server";
            }

            list listener {
                key "name";
                ze:listener;
                description
                    "UDP+TCP listen endpoints. Each named entry binds one IP
                     (IPv4 or IPv6) and port; repeat with the same port for a
                     dual-stack pair. Defaults to 127.0.0.1:5300 and ::1:5300
                     when no entry is configured. The ze:listener marking
                     enables config-time port-conflict detection across services.";

                leaf name {
                    type string {
                        length "1..255";
                    }
                    description "Listener instance name";
                }

                uses zt:listener {
                    refine ip {
                        default "127.0.0.1";
                    }
                    refine port {
                        default 5300;
                    }
                }
            }

            leaf default-ttl {
                type uint32 {
                    range "1..2147483647";
                }
                default 300;
                description "Default record TTL (seconds) when a host omits its own. 1..2147483647 (RFC 2181 section 8); a zero default is not allowed.";
            }

            leaf client-ip-source {
                type enumeration {
                    enum edns0 {
                        description "Use only the EDNS0 client-subnet option (RFC 7871); no answer without it";
                    }
                    enum packet {
                        description "Use the UDP/TCP packet source IP";
                    }
                    enum edns0-then-packet {
                        description "Prefer EDNS0 client-subnet, fall back to the packet source IP";
                    }
                }
                default "edns0-then-packet";
                description "Where the client IP used for source selection is read from";
            }

            leaf-list zone {
                type string {
                    length "1..255";
                }
                description "Zones served (FQDN). A query name must end in one of these.";
            }

            leaf-list nameserver {
                type zt:ipv4-address;
                max-elements 9;
                description "Nameserver IPv4 addresses; ns1..nsN.<zone> A glue is synthesised (max 9)";
            }

            container soa {
                description "SOA record fields (synthesised; geodns serves no AXFR)";

                leaf mname {
                    type string {
                        length "1..255";
                    }
                    description "Primary nameserver (SOA MNAME). Defaults to ns1.<first-zone> when unset.";
                }
                leaf contact {
                    type string {
                        length "1..255";
                    }
                    default "hostmaster";
                    description "Responsible-party mailbox label (SOA RNAME), e.g. hostmaster";
                }
                leaf serial-mode {
                    type enumeration {
                        enum auto-epoch {
                            description "max(Unix seconds at commit, previous serial + 1); strictly increases at any rate";
                        }
                        enum auto-datetime {
                            description "YYYYMMDDnn (RFC 1912); capped at 100 revisions/day by the 32-bit serial";
                        }
                        enum fixed {
                            description "Use the serial leaf verbatim";
                        }
                    }
                    default "auto-epoch";
                    description "How the 32-bit SOA serial is generated";
                }
                leaf serial {
                    type uint32;
                    description "SOA serial used when serial-mode is fixed";
                }
                leaf refresh {
                    type uint32;
                    default 3600;
                    description "SOA refresh seconds";
                }
                leaf retry {
                    type uint32;
                    default 600;
                    description "SOA retry seconds";
                }
                leaf expire {
                    type uint32;
                    default 300;
                    description "SOA expire seconds";
                }
                leaf minimum {
                    type uint32;
                    default 300;
                    description "SOA minimum / negative-cache TTL seconds";
                }
            }

            list host-set {
                key "name";
                description "Named, reusable set of host records; referenced by source entries";

                leaf name {
                    type string {
                        length "1..255";
                    }
                    description "Host-set name (referenced by source/host-set)";
                }

                list host {
                    key "name";
                    description "A hostname and its records";

                    leaf name {
                        type string {
                            length "1..255";
                        }
                        description "Fully-qualified hostname (must end in a configured zone)";
                    }
                    leaf ttl {
                        type uint32 {
                            range "0..2147483647";
                        }
                        description "Record TTL; defaults to default-ttl when unset";
                    }
                    leaf type {
                        type enumeration {
                            enum A;
                            enum AAAA;
                            enum SRV;
                        }
                        description "Record type; omit to auto-detect A vs AAAA per address";
                    }
                    leaf-list address {
                        type zt:ip-address;
                        description "One or more A/AAAA addresses (v4 => A, v6 => AAAA when type omitted)";
                    }
                    container srv {
                        description "SRV record fields (when type is SRV)";
                        leaf priority {
                            type uint16;
                            description "SRV priority";
                        }
                        leaf weight {
                            type uint16;
                            description "SRV weight";
                        }
                        leaf port {
                            type uint16;
                            description "SRV port";
                        }
                        leaf target {
                            type string {
                                length "1..255";
                            }
                            description "SRV target hostname";
                        }
                    }
                }
            }

            list source {
                key "prefix";
                description
                    "Maps a client-IP prefix to a host-set. Longest prefix wins;
                     0.0.0.0/0 and ::/0 are the catch-all (external) default.";

                leaf prefix {
                    type zt:ip-prefix;
                    description "Client source prefix in CIDR (e.g. 82.219.0.0/16, 82.219.4.10/32, 0.0.0.0/0)";
                }
                leaf host-set {
                    type string {
                        length "1..255";
                    }
                    description "Name of the host-set to answer for clients matching this prefix";
                }
            }
        }
    }
}
```

### imageserver

source: `internal/plugins/imageserver` -- config root: `service`

Image server: HTTP provisioning for disk images and boot files

`ze-image-server-conf.yang`

```yang
module ze-image-server-conf {
    namespace "urn:ze:image-server:conf";
    prefix imgsrv;

    description "Image server configuration for Ze provisioning";

    revision 2026-01-01 {
        description "Initial revision";
    }

    container service {
        description "Service settings";

        container image-server {
            description "HTTP image server for PXE provisioning";

            leaf enabled {
                type boolean;
                default false;
                description "Enable image server";
            }

            leaf-list listen-interface {
                type string;
                description "Interfaces to serve on";
            }

            leaf listen-port {
                type uint16 {
                    range "1..65535";
                }
                default 80;
                description "HTTP listen port";
            }

            leaf image-directory {
                type string;
                description "Directory containing gokrazy disk images";
            }

            leaf boot-directory {
                type string;
                description "Directory containing installer kernel, initrd, iPXE config";
            }

            leaf ssh-username {
                type string;
                description "Admin username for installed target (written to served zefs)";
            }

            leaf ssh-password-hash {
                type string;
                description "Bcrypt hash of admin password (written to served zefs)";
            }

            leaf shell-auth-sha256 {
                type string;
                description "Lowercase hex sha256 of the admin password. Emitted on the installer kernel cmdline (ze.shell-auth) to gate the rescue shell; empty means the installer fails closed (no shell on fatal).";
            }
        }
    }
}
```

### tftpserver

source: `internal/plugins/tftpserver` -- config root: `service`

TFTP server: read-only file serving for PXE boot (RFC 1350, RFC 2347 option negotiation)

`ze-tftp-server-conf.yang`

```yang
module ze-tftp-server-conf {
    namespace "urn:ze:tftp-server:conf";
    prefix tftpsrv;

    description "TFTP server configuration for Ze (RFC 1350)";

    revision 2026-01-01 {
        description "Initial revision";
    }

    container service {
        description "Service settings";

        container tftp-server {
            description "Read-only TFTP server (RFC 1350)";

            leaf enabled {
                type boolean;
                default false;
                description "Enable TFTP server";
            }

            leaf-list listen-interface {
                type string;
                description "Interfaces to serve TFTP on";
            }

            leaf root-directory {
                type string;
                description "Directory to serve files from";
            }

            leaf max-transfers {
                type uint16 {
                    range "1..1000";
                }
                default 10;
                description "Maximum concurrent TFTP transfers";
            }
        }
    }
}
```

## Static (`static`, 1 plugins)

### static

source: `internal/plugins/static` -- config root: `static` -- depends on: `routing-table`

Static routes: config-driven kernel/VPP route programming with ECMP

`ze-static-cmd.yang`

```yang
module ze-static-cmd {
    namespace "urn:ze:static:cmd";
    prefix staticcmd;
    import ze-extensions { prefix ze; }
    description "show static command tree. Owned by the static plugin so that removing it removes the command node together with the handler. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-03 { description "Relocated show static out of the central show schema (plugin self-containment)."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container static {
            config false;
            ze:command "ze-show:static";
            description "Show static routes defined in the configuration.
Lists each static route with its prefix, next-hop, and interface.";
        }
    }
}
```

`ze-static-conf.yang`

```yang
module ze-static-conf {
    namespace "urn:ze:static:conf";
    prefix static;

    import ze-extensions { prefix ze; }

    description
        "Static route configuration for Ze.
         Routes are programmed directly to the kernel (netlink) or VPP
         data plane. Multiple next-hops produce ECMP with optional
         per-next-hop weights. BFD profiles enable fast failover.
         Routes are grouped under named tables (resolved via the
         routing-table registry). The 'default' table maps to the
         kernel main table (254).";

    revision 2026-05-15 {
        description "Add table grouping and interface-only next-hops.";
    }

    revision 2026-04-23 {
        description "Initial revision.";
    }

    container static {
        description "Static route configuration.";

        list table {
            key "name";
            description
                "A named routing table grouping. Table name is resolved
                 to a kernel table ID via the routing-table registry.
                 Use 'default' for the main routing table.";

            leaf name {
                type string;
                description
                    "Table name. Must match an entry in the
                     routing-table registry, or 'default' for
                     the main table.";
            }

            list route {
                key "prefix";
                description "A static route entry.";

                leaf prefix {
                    type string;
                    ze:validate "ipv4-prefix|ipv6-prefix";
                    description "Destination prefix in CIDR notation.";
                }

                leaf description {
                    type string { length "0..255"; }
                    description "Operator note for this route.";
                }

                leaf metric {
                    type uint32;
                    default 0;
                    description
                        "Route metric. Used as kernel route priority
                         (lower is preferred) and carried in redistribute.";
                }

                leaf tag {
                    type uint32;
                    description
                        "Opaque tag for route policy matching.
                         Carried in redistribute events.";
                }

                choice action {
                    mandatory true;
                    description "What to do with matching packets.";

                    case forward {
                        container next {
                            description
                                "Forward action. Contains gateway and interface
                                 next-hops that may be combined for ECMP.";

                            list hop {
                                key "address";
                                description
                                    "Forward via gateway. Multiple entries produce
                                     ECMP with traffic distributed by weight.";

                                leaf address {
                                    type string;
                                    ze:validate "ipv4-address|ipv6-address";
                                    description "Gateway address.";
                                }

                                leaf interface {
                                    type string;
                                    description
                                        "Outgoing interface. Required when the
                                         next-hop is a link-local IPv6 address.";
                                }

                                leaf weight {
                                    type uint16 { range "1..65535"; }
                                    default 1;
                                    description
                                        "ECMP weight. Higher values receive
                                         proportionally more traffic. Default 1
                                         gives equal distribution.";
                                }

                                leaf bfd-profile {
                                    type string;
                                    description
                                        "BFD profile name (from bfd/profile list).
                                         When the BFD session to this next-hop goes
                                         down, the next-hop is removed from the ECMP
                                         group and the route is reprogrammed with
                                         remaining active next-hops.";
                                }
                            }

                            list interface {
                                key "name";
                                description
                                    "Forward via interface only (no gateway address).
                                     Used for point-to-point links (PPPoE, GRE tunnels)
                                     where the next-hop is implicit. Multiple entries
                                     can coexist with gateway next-hops for mixed ECMP.";

                                leaf name {
                                    type string;
                                    description "Outgoing interface name.";
                                }

                                leaf weight {
                                    type uint16 { range "1..65535"; }
                                    default 1;
                                    description
                                        "ECMP weight. Higher values receive
                                         proportionally more traffic.";
                                }
                            }
                        }
                    }

                    case discard {
                        container blackhole {
                            presence "Silently discard matching packets.";
                        }
                    }

                    case unreachable {
                        container reject {
                            presence
                                "Discard and reply ICMP unreachable.";
                        }
                    }
                }
            }
        }
    }
}
```

## Sysctl (`sysctl`, 1 plugins)

### sysctl

source: `internal/component/sysctl` -- config root: `sysctl`

Kernel tunable management: three-layer precedence, restore on stop

`ze-sysctl-conf.yang`

```yang
module ze-sysctl-conf {
    namespace "urn:ze:sysctl:conf";
    prefix sysctl;

    import ze-types { prefix zt; }

    description
        "Sysctl configuration for Ze.
         Generic key/value list for kernel tunables.
         Known keys get type/range validation; unknown keys
         are validated by attempting the write on commit.";

    revision 2026-04-13 {
        description "Initial sysctl plugin schema.";
    }

    container sysctl {
        description "Kernel tunable management.";

        list setting {
            key "name";
            description
                "A kernel tunable to set persistently.
                 Uses kernel-native naming (e.g., net.ipv4.conf.all.forwarding).";

            leaf name {
                type string;
                description "Kernel-native sysctl key name.";
            }

            leaf value {
                type string;
                mandatory true;
                description "Value to set for this key.";
            }
        }

        list profile {
            key "name";
            max-elements 50;
            description
                "User-defined sysctl profile: a named collection of
                 kernel tunables applied together to interface units.
                 Overrides built-in profiles with the same name.";

            leaf name {
                type zt:node-name;
                description "Profile name (lowercase alphanumeric with hyphens).";
            }

            list setting {
                key "name";
                max-elements 50;
                description "Kernel tunables in this profile.";

                leaf name {
                    type string;
                    description
                        "Kernel-native sysctl key name.
                         Use <iface> for per-interface keys
                         (substituted at apply time).";
                }

                leaf value {
                    type string;
                    mandatory true;
                    description "Value to set for this key.";
                }
            }
        }
    }
}
```

## Traffic Control (`traffic-control`, 1 plugins)

### traffic

source: `internal/component/traffic` -- config root: `traffic-control` -- optional: `vpp`

Traffic control (tc) qdisc, class, and filter management

`ze-traffic-control-conf.yang`

```yang
module ze-traffic-control-conf {
    namespace "urn:ze:traffic:conf";
    prefix tc;

    description "Traffic control (tc) configuration for Ze.
                 Configures queueing disciplines, classes, and filters
                 per network interface.";

    revision 2026-04-13 {
        description "Initial revision";
    }

    typedef qdisc-type {
        type enumeration {
            enum htb { description "Hierarchical Token Bucket"; }
            enum hfsc { description "Hierarchical Fair Service Curve"; }
            enum fq { description "Fair Queue"; }
            enum fq_codel { description "Fair Queue Controlled Delay"; }
            enum sfq { description "Stochastic Fair Queue"; }
            enum tbf { description "Token Bucket Filter"; }
            enum netem { description "Network Emulator"; }
            enum prio { description "Priority qdisc"; }
            enum clsact { description "Classifier-Action qdisc"; }
            enum ingress { description "Ingress qdisc"; }
        }
        description "Queueing discipline type";
    }

    typedef rate-bps {
        type string {
            pattern '[0-9]+(mbit|kbit|gbit|bit|mbps|kbps|gbps|bps)';
        }
        description "Rate in bits or bytes per second (e.g., 10mbit, 100kbps)";
    }

    typedef filter-type {
        type enumeration {
            enum mark { description "Match by packet mark (fw filter)"; }
            enum dscp { description "Match by DSCP value"; }
            enum protocol { description "Match by protocol"; }
        }
        description "Traffic filter match type";
    }

    container traffic-control {
        description "Per-interface traffic control configuration";

        leaf backend {
            type string;
            default "tc";
            description "Traffic control backend implementation. Default is
                         tc (Linux iproute2 queueing disciplines). Future
                         backends can declare themselves via
                         traffic.RegisterBackend. The ze:backend YANG
                         extension on feature nodes declares per-feature
                         backend support so the commit-time gate rejects
                         configs that try to use unsupported qdiscs or
                         filter types.";
        }

        list interface {
            key "name";
            description "Traffic control for a named interface";

            leaf name {
                type string;
                description "Interface name (must exist in interface config)";
            }

            container qdisc {
                description "Root queueing discipline";

                leaf type {
                    type qdisc-type;
                    mandatory true;
                    description "Qdisc type";
                }

                leaf default-class {
                    type string;
                    description "Name of the default class for unclassified traffic";
                }

                list class {
                    key "name";
                    description "Traffic class within the qdisc";

                    leaf name {
                        type string {
                            length "1..255";
                        }
                        description "Class name";
                    }

                    leaf rate {
                        type rate-bps;
                        mandatory true;
                        description "Guaranteed rate";
                    }

                    leaf ceil {
                        type rate-bps;
                        description "Maximum rate (defaults to rate if omitted)";
                    }

                    leaf priority {
                        type uint8;
                        description "Scheduling priority (0 = highest)";
                    }

                    list match {
                        key "type";
                        description "Filter to classify packets into this class";

                        leaf type {
                            type filter-type;
                            description "Match type";
                        }

                        leaf value {
                            type string;
                            mandatory true;
                            description "Match value (mark hex, dscp name, protocol name)";
                        }
                    }
                }
            }
        }
    }
}
```

## Traffic Usage (`traffic-usage`, 1 plugins)

### traffic-usage

source: `internal/plugins/trafficusage` -- config root: `traffic-usage` -- depends on: `interface`

eBPF TCX per-port and per-IP byte accounting

`ze-traffic-usage-cmd.yang`

```yang
module ze-traffic-usage-cmd {
    namespace "urn:ze:traffic-usage:cmd";
    prefix trafficusagecmd;
    import ze-extensions { prefix ze; }
    description "show traffic-usage command tree. Owned by the traffic-usage plugin so that removing the plugin removes the command node together with its handler. See ai/rules/plugin-self-containment.md.";
    revision 2026-06-24 { description "Initial: show traffic-usage displays per-interface eBPF byte counters."; }

    container show {
        config false;
        description "Read-only commands to inspect system, protocol, and network state";

        container traffic-usage {
            config false;
            ze:command "ze-show:traffic-usage";
            description "Show per-interface traffic byte counters captured by eBPF TCX.
Per destination/source port and protocol counters are always present; per-IP
top-talker counters appear when track-ip is enabled. Without arguments, lists
all monitored interfaces. With 'name <interface>', shows that one interface.";
            leaf name {
                type string;
                description "Interface name (used with 'name <interface>')";
            }
        }
    }
}
```

`ze-traffic-usage-conf.yang`

```yang
module ze-traffic-usage-conf {
    namespace "urn:ze:traffic-usage:conf";
    prefix trafficusageconf;

    description "eBPF TCX per-port and per-IP byte accounting configuration.";
    revision 2026-06-24 { description "Initial revision."; }

    container traffic-usage {
        description "Per-interface eBPF TCX byte accounting (port/protocol always; per-IP via track-ip).";

        leaf enabled {
            type boolean;
            default false;
            description "Enable traffic-usage accounting.";
        }

        container interfaces {
            description "Interfaces to account on.";
            list interface {
                key "name";
                description "Per-interface traffic-usage accounting.";
                leaf name {
                    type string {
                        length "1..255";
                        pattern '[A-Za-z0-9][A-Za-z0-9._@-]*';
                    }
                    description "ze interface name; resolved to its OS/kernel device (honoring the os-name / mac-match selectors) before attaching.";
                }
                leaf enabled {
                    type boolean;
                    default true;
                    description "Account traffic on this interface.";
                }
                leaf track-ip {
                    type boolean;
                    description "Override the global track-ip for this interface (inherits the global value when unset).";
                }
                leaf stale-timeout {
                    type uint32;
                    description "Override the global stale-timeout (milliseconds; 0 disables) for this interface (inherits when unset).";
                }
                leaf max-entries {
                    type uint32 {
                        range "1..4294967295";
                    }
                    description "Override the global max-entries (per-map LRU capacity) for this interface (inherits when unset).";
                }
            }
        }

        leaf interval {
            type uint32 {
                range "100..3600000";
            }
            default 1000;
            description "Map poll interval in milliseconds (100ms..1h).";
        }

        leaf stale-timeout {
            type uint32;
            default 300000;
            description "Remove a metric series unseen within this many milliseconds; 0 disables cleanup.";
        }

        leaf track-ip {
            type boolean;
            default false;
            description "Also account bytes per source (ingress) / destination (egress) IPv4. Off by default to bound metric cardinality.";
        }

        leaf max-entries {
            type uint32 {
                range "1..4294967295";
            }
            default 10240;
            description "Per-map LRU capacity; least-recently-used entries are evicted beyond this, keeping top talkers.";
        }
    }
}
```

## Ungrouped (`ungrouped`, 1 plugins)

### loop

source: `internal/component/bgp/reactor/filter`

Route loop detection (RFC 4271 S9, RFC 4456 S8)

`ze-loop-detection.yang`

```yang
module ze-loop-detection {
    namespace "urn:ze:loop-detection";
    prefix ld;

    import ze-bgp-conf { prefix bgp; }
    import ze-extensions { prefix ze; }
    import ze-types { prefix zt; }

    description "Loop detection filter type for the BGP policy framework.
                 Facade over the in-process LoopIngress wire-bytes filter.
                 RFC 4271 Section 9 (AS loop), RFC 4456 Section 8 (cluster-list loop).";

    revision 2026-04-09 {
        description "Initial revision";
    }

    augment "/bgp:bgp/bgp:policy" {
        list loop-detection {
            ze:filter;
            key "name";
            description "Named loop detection filter instance.
                         Configures AS loop and cluster-list loop detection per-peer.";

            leaf name {
                type string;
                description "Filter instance name (referenced in peer filter chains)";
            }

            leaf allow-own-as {
                type uint8 {
                    range "0..10";
                }
                default 0;
                description "Number of own-AS occurrences to allow in AS_PATH before rejecting.
                             0 means reject on first occurrence (default, RFC 4271 Section 9).";
            }

            leaf cluster-id {
                type zt:ipv4-address;
                description "Override Router ID for CLUSTER_LIST loop detection.
                             If not set, uses the BGP Router ID (RFC 4456 Section 7).";
            }
        }
    }
}
```

## Vpn (`vpn`, 1 plugins)

### ike

source: `internal/component/ike/engine` -- config root: `vpn, pki`

IKEv2 engine for native IPsec VPN

No YANG module of its own (reads config defined by another plugin, or has none).

## Vpp (`vpp`, 1 plugins)

### vpp

source: `internal/component/vpp` -- config root: `vpp`

VPP data plane lifecycle management

`ze-vpp-conf.yang`

```yang
module ze-vpp-conf {
    namespace "urn:ze:vpp:conf";
    prefix zvpp;

    description
        "VPP (Vector Packet Processing) lifecycle configuration for Ze.
         Manages VPP process startup, DPDK NIC binding, GoVPP connection,
         and LCP (Linux Control Plane) plugin settings.";

    revision 2026-04-14 {
        description "Initial revision.";
    }

    container vpp {
        description "VPP data plane configuration.";

        leaf enabled {
            type boolean;
            default false;
            description
                "Enable VPP integration. When true, ze manages VPP
                 lifecycle: generates startup.conf, binds DPDK NICs,
                 starts VPP process, connects via GoVPP.";
        }

        leaf external {
            type boolean;
            default false;
            description
                "When true, ze does NOT exec or supervise the VPP
                 process. Ze still generates startup.conf (for the
                 external supervisor to use) and connects via GoVPP
                 to api-socket. Use this for systemd-managed VPP,
                 container sidecar deployments, or the
                 `ze-test vpp` stub harness. Default: false (ze owns
                 VPP lifecycle).";
        }

        leaf api-socket {
            type string;
            default "/run/vpp/api.sock";
            description
                "GoVPP binary API Unix socket path.";
        }

        container cpu {
            description "VPP CPU pinning configuration.";

            leaf main-core {
                type uint8;
                description
                    "CPU core for VPP main thread.
                     Omit for automatic assignment.";
            }

            leaf workers {
                type uint8;
                description
                    "Number of VPP worker threads.
                     Omit for automatic (one per available core).";
            }
        }

        container memory {
            description "VPP memory and buffer configuration.";

            leaf main-heap {
                type string;
                default "1G";
                description
                    "VPP main heap size (e.g. 512M, 1G, 1536M).
                     Production with full DFZ: 1536M.";
            }

            leaf hugepage-size {
                type enumeration {
                    enum 2M;
                    enum 1G;
                }
                default "2M";
                description "Hugepage size for VPP buffers.";
            }

            leaf buffers {
                type uint32;
                default 128000;
                description
                    "Number of packet buffers per NUMA node.
                     128000 is proven for full DFZ at 10G.";
            }
        }

        container dpdk {
            description "DPDK NIC configuration.";

            list interface {
                key "pci-address";
                description "DPDK-managed network interface.";

                leaf pci-address {
                    type string;
                    description
                        "PCI bus address in DDDD:DD:DD.D format
                         (e.g. 0000:03:00.0).";
                }

                leaf name {
                    type string;
                    description
                        "Short interface name (e.g. xe0, e1).
                         Used in ze CLI and config.";
                }

                leaf rx-queues {
                    type uint8;
                    description
                        "Number of receive queues.
                         Omit for VPP default.";
                }

                leaf tx-queues {
                    type uint8;
                    description
                        "Number of transmit queues.
                         Omit for VPP default.";
                }
            }
        }

        container stats {
            description "VPP stats segment configuration.";

            leaf segment-size {
                type string;
                default "512M";
                description "Stats segment shared memory size.";
            }

            leaf socket-path {
                type string;
                default "/run/vpp/stats.sock";
                description "Stats segment Unix socket path.";
            }

            leaf poll-interval {
                type uint16 {
                    range "1..3600";
                }
                default 30;
                description
                    "Stats polling interval in seconds. Controls how often
                     ze reads VPP's stats segment for Prometheus metrics.";
            }
        }

        container lcp {
            description
                "Linux Control Plane plugin. Creates TAP mirrors
                 in Linux for VPP interfaces, enabling routing daemons
                 (ze BGP) to use Linux TCP on VPP-managed NICs.";

            leaf enabled {
                type boolean;
                default true;
                description "Enable LCP plugin in VPP.";
            }

            leaf sync {
                type boolean;
                default true;
                description
                    "Sync VPP state changes (link, MTU, IP)
                     to Linux TAP mirrors.";
            }

            leaf auto-subint {
                type boolean;
                default true;
                description
                    "Auto-create sub-TAPs for dot1q/QinQ
                     sub-interfaces.";
            }

            leaf netns {
                type string;
                default "dataplane";
                description
                    "Network namespace for LCP TAP interfaces.
                     Routing daemons run in this namespace.";
            }
        }
    }
}
```
