# CLI Reference

386 commands across 45 groups, generated straight from `ze help command --json` -- the same live command registry the binary itself uses, so this list cannot drift from what the binary actually supports. Full machine-readable list (path, mode, description for every command, one JSON array): [data/cli-commands.json](https://ze-software.net/data/cli-commands.json).

## announce (1)

| Command | Mode | Description |
| --- | --- | --- |
| `announce` | Daemon | Announce a route on demand to selected peers. Usage: announce <unicast\|blackhole\|flowspec> <args> [tag <key> <value>] [for <duration>] |

## clear (19)

| Command | Mode | Description |
| --- | --- | --- |
| `clear bgp rib in` | Daemon | Remove all routes received from a peer. Wipes the Adj-RIB-In for matched peers. They will need to re-advertise everything (or you can send a route-refresh). Selector: IP, name, AS pattern, glob, or *. |
| `clear bgp rib out` | Daemon | Re-advertise all routes to a peer. Triggers a full Adj-RIB-Out replay to the selected peers. Useful after a policy change to push updated attributes without tearing down the session. Selector: IP, name, AS pattern, glob, or *. |
| `clear debug` | Offline | Clear the default debug profile. |
| `clear dns cache` | Daemon | Flush all DNS cache entries and reset all DNS cache counters. |
| `clear dns cache record` | Daemon | Evict DNS cache entries for one record name, or one name and type when a type is provided. |
| `clear dns cache stats` | Daemon | Reset DNS cache hit, miss, eviction, and expiry counters without removing cached entries. |
| `clear interface counters` | Daemon | Zero the Rx/Tx counters for every managed interface. Usage: clear interface counters. |
| `clear interface name counters` | Daemon | Zero the Rx/Tx counters for one interface. Usage: clear interface name <name> counters. |
| `clear isis adjacency` | Daemon | Tear down every IS-IS adjacency so neighbors re-form. Usage: clear isis adjacency. Adjacencies re-learn from the next Hello; the circuit is not closed and the configuration is unchanged. |
| `clear isis counters` | Daemon | Reset IS-IS observational counters and the SPF log. Usage: clear isis counters. Monotonic Prometheus series are not reset; the SPF-run history is cleared. |
| `clear l2tp session all` | Daemon | Disconnect every L2TP session on this box. Sends CDN for all sessions across all tunnels. Tunnels themselves stay up. Use with care. |
| `clear l2tp session id` | Daemon | Disconnect one subscriber session. Sends a CDN to gracefully close the session. Pass the local session ID: clear l2tp session id <id> [reason <text>] [cause <code>]. |
| `clear l2tp tunnel all` | Daemon | Tear down every L2TP tunnel on this box. Sends StopCCN for all tunnels. Every subscriber session will be disconnected. Use with care during maintenance. |
| `clear l2tp tunnel id` | Daemon | Gracefully tear down one L2TP tunnel. Sends a StopCCN to the peer. All sessions on this tunnel will be disconnected. Pass the local tunnel ID: clear l2tp tunnel id <id>. |
| `clear ospf counters` | Daemon | Reset the OSPF SPF-run history. Usage: clear ospf counters. Monotonic Prometheus series are not reset; the SPF-run log is cleared. |
| `clear ospf neighbor` | Daemon | Tear down every OSPF adjacency so neighbors re-form. Usage: clear ospf neighbor. Adjacencies re-learn from the next Hello. |
| `clear ospf process` | Daemon | Full OSPF reset: tear down every adjacency and re-run SPF. Usage: clear ospf process. Adjacencies re-form from the next Hello; the configuration is unchanged. |
| `clear vpn ipsec sa` | Daemon | Tear down IKE Security Associations. Without arguments, terminates all SAs. Use 'peer <name>' to clear just one peer. The tunnel will renegotiate automatically if the config is still active. |
| `clear vrrp statistics` | Daemon | Reset every VRRP virtual router's counters to zero. Protocol state is untouched: clearing counters never triggers a failover. |

## create (9)

| Command | Mode | Description |
| --- | --- | --- |
| `create interface address` | Daemon | Add an IP address to an interface. Usage: create interface address <name> <prefix>. Interface must already exist. |
| `create interface bridge name` | Daemon | Create a Linux bridge for L2 forwarding. Usage: create interface bridge name <name>. |
| `create interface bridge name address` | Daemon | Add an IP address to the bridge. Usage: create interface bridge name <name> address <prefix>. |
| `create interface bridge name unit` | Daemon | Add a VLAN sub-interface to the bridge. Usage: create interface bridge name <name> unit <vid>. |
| `create interface dummy name` | Daemon | Create a dummy (loopback-style) interface. Usage: create interface dummy name <name>. |
| `create interface dummy name address` | Daemon | Add an IP address to the dummy. Usage: create interface dummy name <name> address <prefix>. |
| `create interface dummy name unit` | Daemon | Add a VLAN sub-interface to the dummy. Usage: create interface dummy name <name> unit <vid>. |
| `create interface unit` | Daemon | Add a VLAN sub-interface (802.1Q tagged). Usage: create interface unit <parent> <vid>. Parent must already exist. |
| `create interface veth name` | Daemon | Create a veth pair (two linked virtual Ethernet interfaces). Usage: create interface veth name <name> <peer>. |

## debug (4)

| Command | Mode | Description |
| --- | --- | --- |
| `debug ip ospf inject opaque` | Daemon | Inject a crafted IPv4 opaque LSA into the local LSDB (RFC 5250). Usage: debug ip ospf inject opaque scope <link\|area\|as> id <opaque-id> [type <128-255>] [hex <body> \| tlv <type> <value-hex> ...] [withdraw]. The default Opaque Type is Private-Use so a test LSA never collides with a standards-track consumer. Requires `debug ospf inject enable`. |
| `debug ipv6 ospf inject lsa` | Daemon | Inject a crafted OSPFv3 LSA into the local LSDB (RFC 5340). Usage: debug ipv6 ospf inject lsa scope <link\|area\|as> type <ls-type> id <link-state-id> [hex <body>] [withdraw]. The flooding scope is derived from the LS Type S2/S1 bits (a reserved scope is rejected). Requires `debug ospf inject enable`. |
| `debug ospf inject disable` | Daemon | Disable OSPF debug LSA injection. Usage: debug ospf inject disable. |
| `debug ospf inject enable` | Daemon | Enable OSPF debug LSA injection (shared across both address families). Off by default. Usage: debug ospf inject enable. |

## delete (6)

| Command | Mode | Description |
| --- | --- | --- |
| `delete bgp peer` | Daemon | Remove a peer from the running config. Tears down the TCP session and deletes the peer from the running configuration. Does not modify the config file on disk. |
| `delete debug module` | Offline | Disable debug for a subsystem, or remove one of its flags/scopes. |
| `delete debug profile name` | Offline | Delete a named debug profile. |
| `delete interface name` | Daemon | Delete an interface from the kernel. Usage: delete interface name <name>. |
| `delete interface name address` | Daemon | Remove an IP address from an interface. Usage: delete interface name <name> address <prefix>. |
| `delete interface name unit` | Daemon | Remove a VLAN sub-interface. Usage: delete interface name <name> unit. |

## doctor (1)

| Command | Mode | Description |
| --- | --- | --- |
| `doctor` | Offline | Verify kernel features, file descriptor limits, sockets, and required dependencies. Run this before first start or after platform changes. |

## explain (1)

| Command | Mode | Description |
| --- | --- | --- |
| `explain` | Offline | Print the meaning, likely cause, and recommended fix for a Ze diagnostic code. Pass the code you saw in a log or error message. |

## generate (1)

| Command | Mode | Description |
| --- | --- | --- |
| `generate wireguard keypair` | Offline | Generate a WireGuard keypair. Prints private and public keys to stdout for use in your config. |

## help (3)

| Command | Mode | Description |
| --- | --- | --- |
| `help` | Read-only | Show available commands at this level. Lists every registered command verb with a brief description. |
| `help ai` | Offline | AI reference generated from the binary. Sections: cli, api, mcp, dispatch, all (add --json). |
| `help command` | Offline | List every command with its description. Use a filter to narrow the list. |

## monitor (8)

| Command | Mode | Description |
| --- | --- | --- |
| `monitor bgp` | Read-only | Live BGP peer dashboard that refreshes automatically. Shows all peers with state, uptime, and prefix counts. State changes highlight as they happen. Ctrl-C to stop. |
| `monitor event` | Read-only | Stream live events as they happen. Shows a real-time feed of internal events. Filter with include <pattern> or exclude <pattern> to focus on what matters. Patterns match event type names. |
| `monitor interface rate` | Read-only | Stream per-second traffic rates for your interfaces. Shows rx/tx bytes and packets per second, updating every second. Optionally pass an interface name to watch just one link. |
| `monitor ping` | Read-only | Continuous ping with live loss and RTT statistics. Pings <target> until you stop it. Adjust interval and timeout as needed. Shows running min/avg/max RTT and packet loss. |
| `monitor system netlink` | Read-only | Watch kernel networking changes in real time. Streams netlink events: route adds/deletes, link state changes, address assignments. Filter with route, link, address, or all. |
| `monitor traceroute` | Read-only | Live mtr-style traceroute that updates continuously. Shows each hop with running RTT statistics. Keeps probing so you can watch path changes and latency shifts over time. |
| `monitor traffic stat` | Read-only | Start streaming traffic monitor (per-second snapshots). Without arguments, shows all interfaces. With 'name <interface>', filters to one interface. |
| `monitor vpn ipsec` | Read-only | Watch IPsec SA events as they happen. Streams sa-up, sa-down, child-up, child-down, and child-rekey events. Useful for debugging tunnel flaps or rekey issues. |

## peer (2)

| Command | Mode | Description |
| --- | --- | --- |
| `peer raw` | Daemon | Send raw bytes into a peer's TCP stream (dangerous). Injects arbitrary bytes with no BGP framing or validation. Intended for conformance testing and fuzzing only. Will likely break the session if used carelessly. |
| `peer update` | Daemon | Send a pre-built BGP UPDATE to a peer. Payload can be text (human-readable route syntax), hex, or base64. Use 'show bgp encode' to build the payload, then send it here. |

## plugin (10)

| Command | Mode | Description |
| --- | --- | --- |
| `plugin ack` | Read-only | Choose sync or async event delivery. sync: Ze waits for your plugin to acknowledge each event before sending the next one. Safer but slower. async: events fire without waiting, giving higher throughput at the cost of backpressure control. |
| `plugin command complete` | Read-only | Complete command/args |
| `plugin command help` | Read-only | Show command details |
| `plugin command list` | Read-only | List plugin commands |
| `plugin encoding` | Read-only | Choose json or text encoding for plugin events. Controls how events are serialized in this session. JSON is structured and parseable; text is more compact. |
| `plugin format` | Read-only | Choose how BGP message bytes appear in events. hex and base64 are compact wire representations. parsed decodes attributes into structured fields. full includes both wire bytes and parsed content. |
| `plugin help` | Read-only | List plugin subcommands |
| `plugin session bye` | Read-only | Disconnect |
| `plugin session ping` | Read-only | Health check (returns PID) |
| `plugin session ready` | Read-only | Signal plugin init complete |

## request cache (4)

| Command | Mode | Description |
| --- | --- | --- |
| `request cache expire` | Daemon | Remove a cached message immediately. Usage: request cache expire <id>. |
| `request cache forward` | Daemon | Forward a cached UPDATE to peers matching a selector. Usage: request cache forward <id> <selector>. |
| `request cache release` | Daemon | Ack without forwarding (cache consumer) or undo retain (API). Usage: request cache release <id>. |
| `request cache retain` | Daemon | Prevent eviction of a cached message. Usage: request cache retain <id>. |

## request interface (5)

| Command | Mode | Description |
| --- | --- | --- |
| `request interface down` | Daemon | Shut down an interface. Usage: request interface <name> down. |
| `request interface mac` | Daemon | Set the MAC address on an interface. Usage: request interface <name> mac <aa:bb:cc:dd:ee:ff>. |
| `request interface migrate` | Daemon | Move IP addresses between interfaces with minimal downtime. Takes a source interface, a target interface, and the address to move. Adds addresses to the target before removing them from the source (make-before-break). |
| `request interface mtu` | Daemon | Set the MTU on an interface. Usage: request interface <name> mtu <bytes>. Range: 68 to 65535. |
| `request interface up` | Daemon | Bring an interface up. Usage: request interface <name> up. |

## request peer (9)

| Command | Mode | Description |
| --- | --- | --- |
| `request peer borr` | Daemon | Start an Enhanced Route Refresh cycle (RFC 7313). Tells the peer to mark existing routes as stale. After re-sending, send EORR to purge anything not refreshed. |
| `request peer clear soft` | Daemon | Soft-clear a peer without dropping the session. Sends ROUTE-REFRESH for every negotiated AFI/SAFI, causing the peer to re-send all routes. No session bounce, no traffic impact. |
| `request peer eorr` | Daemon | Finish an Enhanced Route Refresh cycle (RFC 7313). The peer purges any routes not re-advertised since the matching BORR. Only send this after the peer has finished re-advertising. |
| `request peer flush` | Daemon | Wait until all queued updates for a peer are sent. Usage: request peer <selector> flush. |
| `request peer pause` | Daemon | Pause reading from a peer's TCP socket. Usage: request peer <selector> pause. |
| `request peer plugin session ready` | Daemon | Signal that per-peer plugin setup is complete. Usage: request peer <selector> plugin session ready. |
| `request peer refresh` | Daemon | Ask a peer to re-send all routes (RFC 2918). Sends a ROUTE-REFRESH message for the specified AFI/SAFI. The peer will re-advertise its entire Adj-RIB-Out. |
| `request peer resume` | Daemon | Resume reading from a previously paused peer. Usage: request peer <selector> resume. |
| `request peer teardown` | Daemon | Tear down a peer session. Usage: request peer <selector> teardown [cease-subcode]. |

## request (other) (15)

| Command | Mode | Description |
| --- | --- | --- |
| `request as112 healthcheck` | Daemon | One-shot authoritative query against an anycast service address (or the given target), exit 0 iff the expected AS112 answer comes back. Finding M4: the tool a healthcheck probe calls, since dig is not on the gokrazy appliance and 'ze resolve dns' cannot target a specific server. Usage: request as112 healthcheck [target <ip>]. |
| `request bgp rib inject` | Daemon | Inject a synthetic route into the Adj-RIB-In. Behaves as if the route was received from a peer. Use this for testing policy filters or simulating route announcements. |
| `request bgp rib withdraw` | Daemon | Withdraw a route from the Adj-RIB-In. Removes a previously injected or received route from a peer's Adj-RIB-In, triggering best-path recomputation. |
| `request commit` | Daemon | Group route changes into named atomic commits. Actions: start (begin a commit), end (finalize), eor (signal end of RIB), rollback (undo), show (inspect), withdraw (remove all routes in a commit), list (show all commits). Grammar: request commit <action> <name> [args]. |
| `request config archive` | Daemon | Save a snapshot of the current running configuration. Captures the config into the store for later rollback or comparison. Optional name labels the snapshot; defaults to a timestamp. |
| `request halt` | Daemon | Dump goroutine stacks to stderr and terminate immediately. |
| `request l2tp outgoing-call remote called` | Daemon | Place an LNS-side outgoing call (RFC 2661 S10.4). Usage: request l2tp outgoing-call remote <name> called <number>. Dials the named remote (which must have outgoing-calls enabled), sends OCRQ, and blocks until the call establishes or fails. On failure the cause and RFC 2661 Result Code are reported (auth reject, tie-breaker loss, peer CDN, or timeout). |
| `request log level` | Daemon | Change a subsystem's log level without restarting. Usage: request log level <logger> <level>. Takes effect immediately. Set to debug when troubleshooting, then back to info when you are done. |
| `request ospf graceful-restart` | Daemon | Trigger a planned OSPFv2 graceful restart (RFC 3623 section 2.1). Usage: request ospf graceful-restart. The engine originates one Grace-LSA per interface, persists the non-volatile restart fact, and suppresses route churn so the FIB is retained across the ensuing control-plane restart. Refused when graceful-restart is not configured. |
| `request quiesce` | Daemon | Block until every subsystem has drained pending async work, then reply. A test/operator barrier that replaces a fixed sleep. |
| `request reboot` | Daemon | Gracefully shutdown then reboot the system. |
| `request reload` | Daemon | Reload the configuration without restarting. |
| `request shutdown` | Daemon | Gracefully shutdown: drain connections, close peers, exit. |
| `request subscribe` | Daemon | Start receiving events of one or more types. Events are delivered asynchronously to your plugin session until you unsubscribe. Use 'show event list' to see available event types. |
| `request unsubscribe` | Daemon | Stop receiving events you previously subscribed to. Removes the subscription for the specified event type from your current plugin session. |

## resolve (11)

| Command | Mode | Description |
| --- | --- | --- |
| `resolve cymru asn-name` | Read-only | Find out who owns an AS number. Queries Team Cymru DNS to return the organization name for the ASN. Usage: resolve cymru asn-name <asn>. |
| `resolve dns a` | Read-only | Look up IPv4 addresses (A records) for a hostname. Usage: resolve dns a <hostname>. |
| `resolve dns aaaa` | Read-only | Look up IPv6 addresses (AAAA records) for a hostname. Usage: resolve dns aaaa <hostname>. |
| `resolve dns ptr` | Read-only | Reverse-lookup an IP address to its hostname (PTR). Usage: resolve dns ptr <ip-address>. |
| `resolve dns txt` | Read-only | Look up TXT records for a hostname. Usage: resolve dns txt <hostname>. Returns all TXT strings. |
| `resolve irr expand` | Read-only | Expand an AS-SET into its member AS numbers. Recursively resolves nested AS-SET objects via WHOIS into a flat list. Useful for building prefix filters from IRR data. |
| `resolve irr prefix` | Read-only | Get all prefixes announced by an AS-SET's members. Expands the AS-SET, then returns every route/route6 object for each member ASN. Use this to build or verify prefix filters. |
| `resolve peeringdb as-set` | Read-only | Find the IRR AS-SET registered for an ASN in PeeringDB. Usage: resolve peeringdb as-set <asn>. Feed the result into 'resolve irr expand' to get the full member list. |
| `resolve peeringdb max-prefix` | Read-only | Get max-prefix limits for an ASN from PeeringDB. Returns IPv4 and IPv6 prefix limits. Apply via the config editor. Usage: resolve peeringdb max-prefix <asn>. |
| `resolve ping` | Read-only | Ping from the router with optional source binding. Usage: resolve ping <target> [source <ip>] [count <n>] [size <bytes>]. |
| `resolve traceroute` | Read-only | Traceroute from the router with optional source binding. Usage: resolve traceroute <target> [source <ip>] [max-hops N] [timeout D] [probes N]. |

## set (5)

| Command | Mode | Description |
| --- | --- | --- |
| `set debug active name` | Offline | Load a named debug profile and apply it to the running daemon. |
| `set debug module` | Offline | Enable debug for a subsystem; optionally set level/flag/scope. E.g. 'set debug module bgp.reactor level debug'. |
| `set debug profile name` | Offline | Save the current debug state as a named profile. |
| `set debug timeout` | Offline | Set the debug auto-disable timer (e.g. 30m, 1h, 90s; 0 disables). |
| `set system file-descriptors` | Daemon | Raise the file descriptor limit for the daemon process. Pass a number or 'max' to go to the hard limit. Takes effect immediately. Check current limits with 'show system file-descriptors'. |

## show bfd (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show bfd profile` | Read-only | Show BFD timer profiles with effective values. Returns min-tx, min-rx, and detect-multiplier after inheritance. Use 'show bfd profile' for every profile or 'show bfd profile name <name>' for one profile. |
| `show bfd profile name` | Read-only | Show one BFD profile by name. |
| `show bfd session address` | Read-only | Show full detail for one BFD session. Pass the peer address. Returns local/remote discriminators, negotiated timers, detection time, and packet counters. |
| `show bfd sessions` | Read-only | List all active BFD sessions. One line per session: peer address, state, negotiated tx/rx intervals, and detect multiplier. |

## show bgp (18)

| Command | Mode | Description |
| --- | --- | --- |
| `show bgp decode` | Read-only | Decode a hex-encoded BGP message into readable JSON. Paste a hex BGP UPDATE and get back parsed attributes, NLRI, and withdrawn prefixes. Handy for reading pcap captures or debugging wire issues. Also available in the web UI under tools. |
| `show bgp encode` | Read-only | Turn a route announcement into wire-format hex. Takes a route in API syntax and returns the BGP UPDATE as a hex string. Useful for building test payloads, feeding to ze-test, or verifying that your announcement encodes correctly. |
| `show bgp health` | Read-only | Quick health check for all your BGP peers. Lists every peer with address, state, ASN, and uptime. Reports how many are not Established. Much faster than 'show bgp peer *' when you just need a status overview. |
| `show bgp irr` | Read-only | Show IRR filter status per ASN. Lists each enrolled ASN with its resolved AS-SET, prefix counts, last refresh time, and error status. Use this to confirm that IRR prefix-lists are loaded and current. |
| `show bgp irr check` | Read-only | Check if a prefix is accepted by the IRR filter. Usage: show bgp irr check <peer> <prefix>. Reports whether the prefix would be accepted or rejected, and which entry matches. |
| `show bgp irr prefix` | Read-only | Show IRR-resolved prefixes for a peer. Usage: show bgp irr prefix <peer>. Lists all IPv4 and IPv6 prefixes in the IRR-resolved prefix-list for the given peer address. |
| `show bgp peer capabilities` | Read-only | Show what capabilities were negotiated with a peer. Usage: show bgp peer <selector> capabilities. |
| `show bgp peer detail` | Read-only | Show full detail for one or more peers. Usage: show bgp peer <selector> detail. The selector can be an IP, peer name, AS pattern (as65001), glob, or *. |
| `show bgp peer history` | Read-only | Show FSM state transitions for a peer over time. Usage: show bgp peer <selector> history. |
| `show bgp peer list` | Read-only | List your peers, one line each. Shows name, address, ASN, state, and uptime. Quick overview without the detail of 'show bgp peer <selector> detail'. |
| `show bgp peer rib` | Read-only | Show RIB data scoped to one peer. Usage: show bgp peer <selector> rib [scope\|filters\|terminal]. |
| `show bgp peer statistics` | Read-only | Show UPDATE throughput for your peers. Usage: show bgp peer <selector> statistics. |
| `show bgp rib` | Read-only | Query routes in the BGP RIB. Look at received or advertised routes with flexible filters: peer, family, prefix, AS path regex, community, match expression. Pipe operators: \| count, \| prefix-summary, \| graph. This is the main route inspection command. |
| `show bgp rib best` | Read-only | Show the winning route for each prefix. Same filters as 'show bgp rib'. Use '\| reason' to see why each path was selected (local-pref, AS path length, MED, etc.). |
| `show bgp rib best status` | Read-only | Check whether best-path computation is still running. Reports idle, pending, or running, plus the last run duration. |
| `show bgp rib rpf` | Read-only | Reverse-path forwarding lookup in the Loc-RIB. Performs a longest-prefix-match and returns the best-path entry. Use this to verify RPF checks would pass for a given source. |
| `show bgp rib status` | Read-only | Get a quick RIB overview without dumping routes. Shows total peers, received/accepted/advertised route counts, and per-family breakdowns. Use this to confirm convergence after a peer comes up. |
| `show bgp summary` | Read-only | Show a one-line-per-peer BGP summary. Lists every peer with state, ASN, prefixes received, and uptime. Optionally scope by address family: ipv4, ipv6, or l2vpn. |

## show bmp (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show bmp collectors` | Read-only | Show BMP collector connection status. Lists configured collectors with connection state, sent message counts, and error statistics. Check here if your collector is not receiving data. |
| `show bmp peers` | Read-only | Show BGP peers as seen through BMP monitoring. Lists peers reported via BMP with their state and route statistics. |
| `show bmp rib` | Read-only | Show routes received via BMP monitoring sessions. Returns the BMP RIB content. Use this to verify what your collector is seeing from remote peers. |
| `show bmp sessions` | Read-only | Show active BMP receiver sessions. Lists each session with connection state and message counters. Check here to confirm your BMP collector is receiving data. |

## show config (7)

| Command | Mode | Description |
| --- | --- | --- |
| `show config cat` | Read-only | Print the full text of a stored configuration snapshot. Usage: show config cat <id>. Outputs the config as-is. |
| `show config diff` | Read-only | Compare two configuration versions side by side. Shows what was added, removed, or changed. Commonly used with rollback revisions to review what changed before you roll back. |
| `show config dump` | Read-only | Show the fully resolved configuration tree. Parses the config and outputs it after includes, defaults, and group inheritance have been applied. What you see here is exactly what the daemon is using. |
| `show config fmt` | Read-only | Pretty-print the configuration with consistent formatting. Normalizes indentation and ordering. Output goes to stdout (read-only). To rewrite the file in place, use 'ze config fmt -w' from the CLI. |
| `show config graph` | Offline | Show how components and peers depend on each other (DOT graph format). |
| `show config history` | Read-only | List available configuration rollback points. Shows revisions with timestamps and commit metadata. Pair with 'show config diff' to review changes before rolling back. |
| `show config ls` | Read-only | List all configuration files stored in the database. Shows archived snapshots and the active config. |

## show ddos (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show ddos flowspec` | Read-only | Show the upstream FlowSpec/RTBH DDoS mitigation status: whether a rule is currently announced, the target vector it covers, and whether the leak-probe is running. |
| `show ddos incidents` | Read-only | Show the recent DDoS incident ring (newest first): per incident the target vector (prefix/proto/port), attack family, top source addresses, peak pps/bps, start/end time, and whether it is still active. |
| `show ddos local` | Read-only | Show the on-host DDoS mitigation status: whether an nft drop rule is currently installed and the target vector (prefix / proto / port) it covers. |
| `show ddos status` | Read-only | Show DDoS observation status: whether the incident store is running, the number of currently active attacks, and the number of incidents retained in the ring. |

## show dns (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show dns cache list` | Read-only | List all non-expired DNS cache entries, sorted by shortest TTL first. |
| `show dns cache record` | Read-only | Show DNS cache entries for one record name. |
| `show dns cache stats` | Read-only | Show DNS cache hit, miss, eviction, expiry, and hit-rate counters without changing cache contents. |
| `show dns lookup` | Read-only | Look up a DNS name from the router. Resolves <hostname> using the daemon's DNS resolver (falls back to the system resolver if no DNS component is configured). Default type is A. Returns records, TTL, and query time. Supports A, AAAA, MX, NS, TXT, CNAME, and PTR. |

## show firewall (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show firewall group` | Read-only | Show members of a firewall address/port group. Without arguments, lists all known groups. With a name, shows the set elements. Reads from the last applied config, not the kernel. |
| `show firewall irr` | Read-only | Show IRR filter status for all cached ASN/AS-SET entries. Lists each cached entry with prefix counts, last refresh time, and error status. Use this to confirm that IRR prefix-lists are loaded and current before committing firewall config. |
| `show firewall irr prefix` | Read-only | Show IRR-resolved prefixes for a cached entry. Usage: show firewall irr prefix <asn-or-as-set>. Lists all IPv4 and IPv6 prefixes in the cached prefix-list for the given ASN or AS-SET. |
| `show firewall ruleset` | Read-only | Show the live firewall ruleset with per-term counters. Usage: show firewall ruleset <name>. Joins applied desired state with kernel counters from the nft backend. |

## show host (9)

| Command | Mode | Description |
| --- | --- | --- |
| `show host all` | Read-only | Show the full hardware inventory in one shot. Returns every section (cpu, nic, dmi, memory, thermal, storage, kernel, platform) as a single JSON response. Ideal for support bundles or automated inventory collection. |
| `show host cpu` | Read-only | Show what CPUs are in this box. Returns vendor, model, core/thread topology, hybrid layout, scaling driver, current/min/max frequencies, and throttle counts. |
| `show host dmi` | Read-only | Show the box's identity from SMBIOS/DMI. Returns system vendor, board name, BIOS version, and chassis type. Useful for inventory or confirming which hardware model you are on. |
| `show host kernel` | Read-only | Show the running kernel version and boot parameters. Returns kernel release, command line, CPU microcode revision, boot time, and security-relevant CPU flags (spectre mitigations, etc.). |
| `show host memory` | Read-only | Show installed memory and ECC health. Returns DIMM sizes and, when the edac driver is present, correctable and uncorrectable error counters. Non-zero ECC counts mean you should plan a DIMM replacement. |
| `show host nic` | Read-only | Show physical NICs installed in this box. Returns driver, PCI vendor/device IDs, link speed, queue counts, and firmware version. Virtual interfaces are excluded. Use this to confirm NIC firmware before an upgrade. |
| `show host platform` | Read-only | Show platform capabilities and constraints. Reports read-only root, privilege level, systemd presence, gokrazy update socket, reboot-allowed flag, persistent-storage writability, and fd limits. Helps you understand what operations are possible on this particular deployment. |
| `show host storage` | Read-only | Show storage devices attached to this box. Returns size, model, transport type (nvme, sata, mmc, virtio), rotational flag, and NVMe firmware version where applicable. |
| `show host thermal` | Read-only | Show temperature sensors and thermal throttle events. Returns hwmon sensor readings and per-CPU throttle counters. Non-zero throttle counts mean the box has been running hot enough to slow down. |

## show interface (8)

| Command | Mode | Description |
| --- | --- | --- |
| `show interface` | Read-only | Show network interfaces on this box. Without arguments, returns all interfaces with full detail. Subcommands: brief, type <t>, errors, rate [<name>], name <name> detail, name <name> counters. |
| `show interface brief` | Read-only | One-line summary per interface: name, state, IP, and MTU. Quick way to see what is up and what addresses are assigned. |
| `show interface errors` | Read-only | Show interfaces that have errors or drops. Filters to only interfaces with non-zero Rx/Tx error or drop counters. Quick way to find troubled links. |
| `show interface name counters` | Read-only | Show counters for one interface. Usage: show interface name <name> counters. |
| `show interface name detail` | Read-only | Show full detail for one interface. Usage: show interface name <name> detail. |
| `show interface rate` | Read-only | Show per-second traffic rates on your interfaces. Returns rx/tx bytes and packets per second. Pass an interface name to narrow the output. Requires the rate tracker. For continuous monitoring, use 'monitor interface rate' instead. |
| `show interface scan` | Read-only | Discover and classify all OS interfaces. Returns name, Ze type (ethernet, bridge, vxlan, etc.), and MAC for each interface found. Pipe to table, yaml, or json for different views. Useful during initial setup to see what the box has. |
| `show interface type` | Read-only | Show only interfaces of a given type. Usage: show interface type <type>. Types include ethernet, bridge, vxlan, wireguard, tunnel, bond, and more. If you pick an invalid type, the error lists all valid ones. |

## show isis (8)

| Command | Mode | Description |
| --- | --- | --- |
| `show isis database` | Read-only | Show the IS-IS link-state database. Lists each LSP with its LSP ID, sequence number, remaining lifetime, checksum, and overload bit, across Level-1 and Level-2. |
| `show isis database detail` | Read-only | Show the IS-IS link-state database with TLV detail. Expands each LSP into its decoded TLVs (type, length, value) so you can read exactly what each node advertises. |
| `show isis hostname` | Read-only | Show the IS-IS dynamic-hostname mapping (RFC 5301). Maps each System ID to the hostname it advertises in TLV 137. |
| `show isis interface` | Read-only | Show IS-IS-enabled circuits. Returns level, circuit type, metric, hello interval, hold multiplier, passive flag, DIS state, and the count of Up adjacencies per circuit. |
| `show isis neighbor` | Read-only | Show IS-IS adjacencies. Returns the neighbor System ID, interface, level, adjacency state, and hold time for each IS-IS neighbor. |
| `show isis route` | Read-only | Show IS-IS-computed routes. Lists each prefix the SPF installed with its metric, level, up/down bit, and next-hops (address and outgoing interface). |
| `show isis route ipv6` | Read-only | Show IS-IS-computed IPv6 routes (RFC 5308). Lists each IPv6 prefix the SPF installed with its metric, level, and next-hops (link-local address and outgoing interface). |
| `show isis spf-log` | Read-only | Show recent IS-IS SPF runs. Returns the most recent SPF runs with their timestamp, level, trigger, duration, and node count. |

## show l2tp (16)

| Command | Mode | Description |
| --- | --- | --- |
| `show l2tp` | Read-only | L2TP tunnel, session, and subscriber state. Without a subcommand, shows a summary of tunnels and sessions. |
| `show l2tp config` | Read-only | Show the resolved L2TP configuration. Returns the effective config after defaults and overrides. Confirms what the daemon is actually using. |
| `show l2tp cqm` | Read-only | Show subscriber line quality (CQM latency buckets). Pass a login name for one subscriber or 'summary' for an overview. Helps diagnose poor subscriber experience. |
| `show l2tp echo` | Read-only | Show LCP echo health for a subscriber session. Returns echo request/reply counters and round-trip times. Rising loss or high RTT indicates a degraded line. |
| `show l2tp health` | Read-only | Find your worst L2TP sessions at a glance. Sorts sessions by echo loss ratio (worst first). Shows subscriber login, session state, echo count, average RTT, and CQM bucket count. Reports how many sessions are degraded. |
| `show l2tp listeners` | Read-only | Show which UDP sockets are listening for L2TP. Lists each bound address, port, and the number of tunnels on it. |
| `show l2tp observer` | Read-only | Show recent events for a session (debug aid). Returns the event ring buffer for one session ID or 'all'. Useful for understanding why a session failed to establish. |
| `show l2tp reliable` | Read-only | Show the reliable transport window for a tunnel. Returns send/receive sequence numbers, window size, and retransmit queue depth. Check here when tunnel control messages seem stuck. |
| `show l2tp session history` | Read-only | Show state transitions for a session over time. Timestamped FSM entries for session establishment. Use this when a subscriber's session fails to come up. |
| `show l2tp session id` | Read-only | Show full detail for one L2TP session. Pass the local session ID. Returns PPP state, assigned addresses, negotiated LCP/NCP options, and traffic counters. |
| `show l2tp session traffic` | Read-only | Show traffic counters for a subscriber's PPP interface. Returns byte and packet counts, error counters, and current rates. Compare with CQM data to get the full picture of subscriber health. |
| `show l2tp sessions` | Read-only | List all active L2TP sessions. One line per session: local/remote ID, parent tunnel, subscriber login, and uptime. |
| `show l2tp statistics` | Read-only | Show aggregate L2TP protocol counters. Tunnels and sessions established, control messages sent/received, retransmits, and errors. Your first stop for L2TP health. |
| `show l2tp tunnel history` | Read-only | Show state transitions for a tunnel over time. Timestamped FSM entries showing how the tunnel reached its current state. Use this to diagnose tunnel establishment failures. |
| `show l2tp tunnel id` | Read-only | Show full detail for one L2TP tunnel. Pass the local tunnel ID. Returns control channel state, peer endpoint, hello interval, and all assigned sessions. |
| `show l2tp tunnels` | Read-only | List all active L2TP tunnels. One line per tunnel: local/remote ID, peer address, session count, and uptime. |

## show metrics (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show metrics list` | Read-only | List all registered metric names (no values). Useful for discovering what metrics exist before querying them. |
| `show metrics name` | Read-only | Show one Prometheus metric by name. Usage: show metrics name <name> [label=value ...]. Returns matching time series from the internal registry. Multiple label filters are ANDed. More targeted than the full metrics dump. |
| `show metrics pool` | Read-only | Show attribute pool memory usage and dedup efficiency. Returns allocated entries, reference counts, and deduplication hit rates per attribute type. Watch the dedup rate to gauge how much memory pooling is saving you. |
| `show metrics values` | Read-only | Dump all metrics in Prometheus text format. Outputs every registered metric with labels and values. Suitable for feeding into Prometheus, Grafana, or curl-based monitoring. |

## show ospf (49)

| Command | Mode | Description |
| --- | --- | --- |
| `show ospf` | Read-only | OSPFv2 process summary: router-id, areas, ABR/ASBR status, and stub-router (max-metric) state (RFC 2328). |
| `show ospf border-routers` | Read-only | Show routes to OSPF area-border and AS-boundary routers. Lists each reachable ABR/ASBR with its router-id, cost, next-hops, and area. |
| `show ospf database` | Read-only | Show the OSPF link-state database. Lists each LSA with its LS Type, Link State ID, Advertising Router, sequence number, age, and checksum. |
| `show ospf database asbr-summary` | Read-only | Show only ASBR-Summary-LSAs (Type 4). |
| `show ospf database external` | Read-only | Show only AS-external-LSAs (Type 5). |
| `show ospf database network` | Read-only | Show only Network-LSAs (Type 2). |
| `show ospf database nssa-external` | Read-only | Show only NSSA-external-LSAs (Type 7, RFC 3101). |
| `show ospf database opaque-area` | Read-only | Show only area-scope opaque-LSAs (Type 10, RFC 5250). |
| `show ospf database opaque-area detail` | Read-only | Decode each area-scope opaque LSA body into its typed TLVs (TE / Router-Information / Extended / Segment-Routing) or a generic type/length/hex view (spec-ospf-ext-14, RFC 5250). |
| `show ospf database opaque-as` | Read-only | Show only AS-scope opaque-LSAs (Type 11, RFC 5250). |
| `show ospf database opaque-as detail` | Read-only | Decode each AS-scope opaque LSA body into its typed TLVs (TE / Router-Information / Extended / Segment-Routing) or a generic type/length/hex view (spec-ospf-ext-14, RFC 5250). |
| `show ospf database opaque-link` | Read-only | Show only link-local opaque-LSAs (Type 9, RFC 5250). |
| `show ospf database opaque-link detail` | Read-only | Decode each link-local opaque LSA body into its typed TLVs (TE / Router-Information / Extended / Segment-Routing) or a generic type/length/hex view (spec-ospf-ext-14, RFC 5250). |
| `show ospf database router` | Read-only | Show only Router-LSAs (Type 1). |
| `show ospf database router-information` | Read-only | Show the Router Information LSAs (RFC 7770) for both address families -- OSPFv2 opaque type 4 and OSPFv3 function code 12 -- decoded into the advertised informational capability bits and the TLV list. |
| `show ospf database summary` | Read-only | Show only Summary-LSAs (Type 3, inter-area network). |
| `show ospf graceful-restart` | Read-only | Show OSPFv2 (IPv4) Graceful Restart state (RFC 3623): the restarter state (in-restart or not, grace end, reason) and the per-neighbor helper sessions (which neighbors are being helped and their remaining grace). |
| `show ospf instance` | Read-only | Show the configured OSPFv2 instances (RFC 6549 Multi-Instance). Lists each Instance ID with its router-id and the size of its isolated area, interface, neighbor, and link-state database state. |
| `show ospf interface` | Read-only | Show OSPF-enabled interfaces. Returns area, network-type, cost, ISM state, DR/BDR, hello/dead intervals, priority, and passive flag per interface. |
| `show ospf interface detail` | Read-only | Show full per-interface state (spec-ospf-ext-14): ISM, DR/BDR election detail, all three timers, and the opaque-capable neighbour count. |
| `show ospf ipv6` | Read-only | Show the OSPFv3 (IPv6) address-family instances (RFC 5838). Lists each configured address family (ipv6-unicast, ipv6-multicast, ipv4-unicast, ipv4-multicast) with its Instance ID, router-id, and neighbor/interface counts, so multiple AF instances on a link are distinguishable. |
| `show ospf ipv6 database` | Read-only | Show the OSPFv3 (IPv6) link-state database with each native scope-aware LSA decoded (RFC 5340). Base types decode into named fields; unknown function codes fall back to a scope-aware header + body-hex view (spec-ospf-ext-14). |
| `show ospf ipv6 database detail` | Read-only | Decode every OSPFv3 LSA body with its scope-aware header (RFC 5340 section A.4.2.1). |
| `show ospf ipv6 database extended` | Read-only | Show the RFC 8362 extended OSPFv3 LSAs (E-Router / E-Network / E-Inter-Area / E-AS-External / E-Link / E-Intra-Area-Prefix) decoded into named TLVs. |
| `show ospf ipv6 database router detail` | Read-only | Decode each OSPFv3 Router-LSA body. |
| `show ospf ipv6 database router-information` | Read-only | Show the OSPFv3 Router Information LSAs (RFC 7770, function code 12) decoded into capability bits and TLVs. |
| `show ospf ipv6 database scope area` | Read-only | Show only area-scope (S2/S1 = 01) LSAs. |
| `show ospf ipv6 database scope as` | Read-only | Show only AS-scope (S2/S1 = 10) LSAs. |
| `show ospf ipv6 database scope link` | Read-only | Show only link-local (S2/S1 = 00) LSAs, including the per-interface Link-LSA store. |
| `show ospf ipv6 database segment-routing` | Read-only | Summarise the OSPFv3 Segment Routing content (RFC 8666) carried in the RI and extended LSAs. |
| `show ospf ipv6 graceful-restart` | Read-only | Show OSPFv3 (IPv6) Graceful Restart state (RFC 5187): the restarter state (in-restart or not, grace end, reason) and the per-neighbor helper sessions (which neighbors are being helped and their remaining grace). |
| `show ospf ipv6 instance` | Read-only | Enumerate the active OSPFv3 address-family instances (RFC 5838 section 2): each with its address family, Instance ID, area count, and neighbor count. |
| `show ospf ipv6 interface` | Read-only | Show OSPFv3 (IPv6-family) interfaces and their RFC 4552 IPsec status. Returns per interface whether IPsec is configured, the protocol (ah/esp) and SPI, and whether the kernel SA is installed. The key is never shown. |
| `show ospf ipv6 interface detail` | Read-only | Show full per-interface OSPFv3 state (spec-ospf-ext-14): ISM, DR/BDR by Router ID, timers, the local Interface ID and Instance ID. |
| `show ospf ipv6 neighbor` | Read-only | Show OSPFv3 (IPv6) neighbors: the link-local address as identity, adjacency state, DR/BDR by Router ID, and dead time. |
| `show ospf ipv6 neighbor detail` | Read-only | Show full per-neighbor OSPFv3 state (spec-ospf-ext-14): the advertised Interface ID, DD sequence, decoded Options (R/V6/E/N/AF), list sizes, last NSM event, and timers. |
| `show ospf ipv6 segment-routing` | Read-only | Show OSPFv3 (IPv6) Segment Routing state (RFC 8666): the configured SRGB/SRLB label ranges, the advertised SR-Algorithm, this node's node Prefix-SIDs, and the Adjacency-SIDs allocated per adjacency. |
| `show ospf ipv6 spf` | Read-only | Show the OSPFv3 (IPv6) per-area SPF run history. |
| `show ospf ipv6 spf detail` | Read-only | Explain why each OSPFv3 route won (spec-ospf-ext-14), AF/Instance-ID tagged; read-only. |
| `show ospf ldp-sync` | Read-only | Show OSPF LDP-IGP synchronization state (RFC 5443, RFC 6138). Lists each ldp-sync interface with its state (not-synchronized / hold-down / synchronized), remaining hold-down, effective metric, and whether it is stuck not-synchronized after having been synchronized. |
| `show ospf neighbor` | Read-only | Show OSPF neighbors. Returns each neighbor's router-id, interface, adjacency state, DR/BDR, priority, dead time, and address. |
| `show ospf neighbor detail` | Read-only | Show full per-neighbor state (spec-ospf-ext-14): DD sequence, decoded Options (incl. the RFC 5250 O-bit), request/summary list sizes, last NSM event, and timers. |
| `show ospf route` | Read-only | Show OSPF-computed routes. Lists each prefix with its path type (intra/inter/external-1/2), cost, next-hops, and area. |
| `show ospf route fast-reroute` | Read-only | Show OSPF fast-reroute (LFA / TI-LFA) backups (RFC 5286). Lists each prefix's primary next-hops with their pre-computed loop-free backup, protection class (node/link/downstream), and TI-LFA repair label stack. Unprotected primaries are shown as unprotected. |
| `show ospf segment-routing` | Read-only | Show OSPFv2 (IPv4) Segment Routing state (RFC 8665): the configured SRGB/SRLB label ranges, the advertised SR-Algorithm, this node's node Prefix-SIDs, and the Adjacency-SIDs allocated per adjacency. |
| `show ospf spf` | Read-only | Show recent OSPF SPF runs. Returns the most recent per-area SPF runs with their timestamp, duration, node count, and pending state. |
| `show ospf spf detail` | Read-only | Explain why each route won (spec-ospf-ext-14): the candidate paths considered per prefix, the winning cost, and the RFC 2328 section 16.4 path-preference tie-break. Read-only; the route table and SPF run count are unchanged. |
| `show ospf te-database` | Read-only | Show the OSPF Traffic Engineering Database (RFC 3630 / RFC 5392): router addresses plus TE links with their Link ID, local/remote address, link type, TE metric, bandwidths, admin group, and (for inter-AS links) remote AS and remote ASBR. |
| `show ospf virtual-links` | Read-only | Show OSPF virtual links (RFC 2328 section 15). Lists each configured virtual link with its transit area, remote router-id, adjacency state, computed cost, and transit next hop. |

## show policy (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show policy chain peer` | Read-only | Show the import/export filter chain applied to a peer. Usage: show policy chain peer <selector> [import\|export]. The selector (IP, name, as<N>) and the optional direction are parsed by the handler. Shows the effective chain after group inheritance is resolved. Without a direction keyword, shows both import and export. |
| `show policy list` | Read-only | List all available filter types and named instances. Shows each filter type and its implementing plugin. Check here when building a new policy chain to see what filters you can use. |
| `show policy routes` | Read-only | Show policy-based routing rules. Lists PBR rules with match criteria and routing actions. |
| `show policy test peer` | Read-only | Test what your policy does to a specific UPDATE. Feed a hex-encoded BGP UPDATE through a peer's filter chain and see the accept/reject result plus attribute modifications at each stage. Read-only: no routes are actually forwarded. Great for validating policy changes before you commit. Usage: show policy test peer <selector> import\|export [filter <name>] update <hex> [source-asn4 true\|false]. The selector and the import/export/filter/update/source-asn4 tokens are parsed by the handler so the peer selector can be a free-form name or address. |

## show pppoe (5)

| Command | Mode | Description |
| --- | --- | --- |
| `show pppoe` | Read-only | PPPoE session and protocol state. Without a subcommand, shows a summary of active sessions. |
| `show pppoe interfaces` | Read-only | Show which interfaces are accepting PPPoE sessions. Lists each PPPoE-enabled interface with its service name, session limit, and how many sessions are currently active. |
| `show pppoe session id` | Read-only | Show full detail for one PPPoE session. Pass the session ID. Returns discovery tags, LCP/NCP state, assigned addresses, and traffic counters. |
| `show pppoe sessions` | Read-only | List all active PPPoE sessions. One line per session: session ID, MAC, subscriber login, uptime, and assigned addresses. |
| `show pppoe statistics` | Read-only | Show PPPoE protocol message counters. Returns PADI, PADO, PADR, PADS, PADT counts, active sessions, and errors. A rising PADI count with flat PADS means sessions are not completing. |

## show rsvp-te (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show rsvp-te fast-reroute` | Read-only | Show RSVP-TE Fast Reroute (RFC 4090) protection state. Returns each configured facility-backup bypass LSP and each protected LSP with its armed bypass, mode, and whether local protection is available and in use. |
| `show rsvp-te interface` | Read-only | Show RSVP-TE bandwidth allocation per interface. Returns reserved, available, and maximum bandwidth for each TE-enabled interface. |
| `show rsvp-te lsp` | Read-only | Show RSVP-TE label-switched paths. Returns state, role (ingress/transit/egress), reserved bandwidth, and in/out labels for each LSP. |
| `show rsvp-te tunnel` | Read-only | Show configured RSVP-TE tunnels and their current state. Returns tunnel name, endpoints, signaling state, and active LSP. |

## show schema (5)

| Command | Mode | Description |
| --- | --- | --- |
| `show schema events` | Read-only | List all notification types defined in YANG API modules. Shows which events a plugin can subscribe to. |
| `show schema handlers` | Read-only | Show which handler serves each YANG module. Maps module names to their implementing Go handler. |
| `show schema list` | Read-only | List all YANG schemas loaded by the daemon. Shows module name, namespace, and revision for each schema. |
| `show schema methods` | Read-only | List all RPC methods defined in YANG API modules. Useful for plugin developers to discover available operations. |
| `show schema protocol` | Read-only | Show the wire protocol version and format details. Useful for checking compatibility between Ze versions. |

## show system (15)

| Command | Mode | Description |
| --- | --- | --- |
| `show system conntrack` | Read-only | Show the kernel connection tracking table. Returns conntrack entry count, table size, timeouts, and loaded modules. Requires the nft backend. Check this when you suspect conntrack table exhaustion is dropping traffic. |
| `show system cpu` | Read-only | Show CPU utilization context for the daemon. Returns goroutine count, logical CPU count, and GOMAXPROCS setting. Useful when the box feels sluggish and you want to see if Ze is hogging threads. |
| `show system date` | Read-only | Show the daemon's current wall-clock time and timezone. Useful for correlating log timestamps when the box is in a different timezone than you are. |
| `show system file-descriptors` | Read-only | Show how many file descriptors the daemon has open. Summary mode: totals by type (socket, pipe, file). Detail mode: every fd with its path and type. Linux only (reads /proc/self/fd). Check this when you suspect fd exhaustion. |
| `show system goroutines` | Read-only | Dump goroutine stacks for debugging hangs or deadlocks. Modes: summary (groups by state), blocked (only lock/channel waiters), full (all stacks). Default: summary. Share the output with support when the daemon stops responding. |
| `show system kernel-log` | Read-only | Show kernel log messages (dmesg-style). Reads from /dev/kmsg. Filter by syslog level (emerg through debug) and limit with count. Without count, you get everything available. Linux only. Useful for spotting NIC errors or OOM events. |
| `show system memory` | Read-only | Show how much memory the daemon is using, from the OS's view. Returns VmRSS, VmSize, VmSwap, and thread count from /proc/self/status (Linux only). This is what the operating system reports the process is using. For the Go runtime allocator view (heap, GC), use 'show runtime memory'. |
| `show system ntp` | Read-only | NTP clock synchronization status |
| `show system ntp peers` | Read-only | Show NTP peers with offset, RTT, stratum, and reachability. Tells you whether your clock is synced and how far off each NTP server thinks you are. |
| `show system platform` | Read-only | Show what kind of platform the daemon is running on. Reports whether this is gokrazy, systemd, container, plain-linux, or darwin, along with platform-specific capabilities. |
| `show system profile` | Read-only | Capture a runtime profile for performance analysis. Types: cpu (requires duration, e.g. 30s), heap, goroutine, allocs (instant snapshots). Output is pprof format you can open with 'go tool pprof'. Send the file to support for deep analysis. |
| `show system sockets` | Read-only | Show open TCP and UDP sockets on this box. Filters: [tcp\|udp] [state <STATE>] [port <N>], all optional and combinable. States use kernel names (ESTABLISHED, LISTEN, TIME_WAIT). Linux only. Good for confirming listeners or spotting stuck connections. |
| `show system subsystem list` | Read-only | List every registered subsystem and whether it is running. Shows you which components (bgp, dns, web, l2tp, etc.) are active, stopped, or failed. |
| `show system update` | Read-only | Check if a firmware update is available. Shows the running version, latest available version, and when the last check ran. Use 'update system firmware check' to trigger an immediate re-check. |
| `show system update history` | Read-only | Show recent firmware update activity. Lists the last 20 update events: checks, downloads, installs, and rollbacks with timestamps and outcomes. |

## show traffic (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show traffic control` | Read-only | Show traffic control (QoS) configuration per interface. Without arguments, lists every interface with its qdisc type and class/filter counts. With an interface name, shows the full qdisc and class breakdown. Use this to verify your shaping is applied. |
| `show traffic feature` | Read-only | Show neutral per-source traffic feature signals: fan-out (distinct destinations), out/in byte ratio (exfiltration), destination-port entropy, new-peer, rare-port/proto, and coarse beaconing. Without arguments, shows the top source entities. With 'name <address>', filters to one source. |
| `show traffic stat` | Read-only | Show aggregated traffic snapshot (interface rates, top talkers, top ports, severity). Without arguments, shows all interfaces. With 'name <interface>', filters to one interface. |
| `show traffic usage` | Read-only | Show per-interface traffic byte counters captured by eBPF TCX. Per destination/source port and protocol counters are always present; per-IP top-talker counters appear when track-ip is enabled. Without arguments, lists all monitored interfaces. With 'name <interface>', shows that one interface. |

## show vpp (4)

| Command | Mode | Description |
| --- | --- | --- |
| `show vpp runtime` | Read-only | Show VPP graph node processing statistics. Returns per-node packet counts, vectors, clocks, and suspends. Helps you find which node is the bottleneck. Requires the VPP backend. |
| `show vpp trace clear` | Read-only | Discard the captured VPP trace buffer. Clears all packets so you can start a fresh trace. Requires the VPP backend. |
| `show vpp trace show` | Read-only | Retrieve packets captured since the last trace start. Shows per-packet VPP graph node traversal. Requires the VPP backend. |
| `show vpp trace start` | Read-only | Start capturing packets in the VPP dataplane. Default input node is dpdk-input, default count is 100 (max 10000). After starting, use 'show vpp trace show' to retrieve the captured packets. Requires the VPP backend. |

## show (other) (68)

| Command | Mode | Description |
| --- | --- | --- |
| `show aaa accounting` | Read-only | Show AAA accounting counters and any dropped records. Tells you whether TACACS+ accounting is working or if records are being lost due to server unreachability. |
| `show announcements` | Read-only | List active on-demand announcements. Usage: show announcements [tag <key>] [selector <pattern>] [family <fam>] |
| `show anomaly detect` | Read-only | Show recent behavioral anomaly incidents (report-only): source entity, cohort, fired features with their deviation z-scores, combined score, and severity. The detector reports; the anomaly/shape responder (Spec 2b) acts. |
| `show anomaly shape` | Read-only | Show the shadow-first anomaly responder status: mode (shadow/armed), action, kill-switch state, and the currently armed source entities with live firewall actions. |
| `show arp` | Read-only | Show the IPv4 ARP table (shortcut for 'show neighbor ipv4'). Lists IPv4 ARP entries with MAC address and state. ARP is IPv4-only; use 'show neighbor' for both families or 'show neighbor ipv6' for the IPv6 ND table. |
| `show as112` | Read-only | AS112 node status: enabled, address-family, hostname/ facility/location, allow-from count, served zone count, and the current SOA serial. |
| `show audit` | Read-only | Show who did what and when on this box. Returns audit log entries with timestamps, actors, and actions. Filters (all optional, combinable): action <type>, actor <name>, surface <name> (cli, web, api), since/until <RFC3339>, count <N>. Actions include config-commit, login, peer-teardown, and more. |
| `show cache` | Read-only | List cached BGP UPDATE message IDs with their retain and consumer state. |
| `show capture` | Read-only | Show captured control-plane messages. Returns protocol messages you previously enabled capture for. Without a protocol keyword, shows all protocols. Filters: tunnel-id (L2TP), peer (remote address), count (limit entries). Use this to debug session establishment issues. |
| `show capture interface` | Read-only | Capture live packets on an interface (like tcpdump). Uses AF_PACKET for zero-copy capture. Filter by protocol and port. Limit with count or duration. Output as pcap (for Wireshark) or text. Snap-len controls how many bytes per packet are captured. |
| `show capture raw` | Read-only | Control raw byte capture for protocol debugging. Actions: start (begin capturing), stop (halt), dump (retrieve). Protocols: l2tp, bgp. Output formats: pcap (for Wireshark), json. Limit with count <N>. |
| `show command complete` | Read-only | Get tab-completion candidates for a partial command. Returns possible completions for the given input. Used internally by the CLI editor, but also callable for scripting. |
| `show command help` | Read-only | Show usage and arguments for a specific command. Gives you the full description, expected arguments, and usage pattern for one command. |
| `show command list` | Read-only | List every command the daemon knows about. Returns dispatch key and description for each. Useful for scripting or discovering commands not shown in the top-level help. |
| `show crashes` | Read-only | View saved crash reports from panics. Without arguments, lists available crash files. Use 'latest' to see the newest crash or 'name <filename>' to print one specific report. Send the output to support when reporting a crash. |
| `show data cat` | Read-only | Print the raw content of a blob store entry. Usage: show data cat <key>. Outputs the value for the given key, like 'cat' for ZeFS. |
| `show data ls` | Read-only | List everything stored in the ZeFS blob store. Shows all keys and their sizes. Use 'show data cat <key>' to see the content of a specific entry. |
| `show data registered` | Read-only | List the key patterns registered by all subsystems. Shows you what types of data ZeFS knows about. |
| `show debug` | Read-only | Show live debug state from the running daemon. Lists every registered subsystem with its current log level and any active flag or scope filters. Unlike 'debug show' (which reads the stored profile), this reflects actual runtime state. |
| `show debug profile` | Offline | Show stored debug profiles (list, 'name <name>' for one, add 'module <prefix>' to filter). |
| `show doctor` | Read-only | Check if this box is ready to run Ze. Verifies runtime dependencies: required files, sockets, ports, and kernel modules. Each check reports pass or fail with a reason. Run this before first start or after changing the platform setup. |
| `show env get` | Read-only | Show one environment variable in detail. Returns the variable name, current value, default, and what it controls. Usage: show env get <name>. |
| `show env list` | Read-only | List all Ze environment variables with their current values. Shows which env vars are set and their defaults. |
| `show env registered` | Read-only | List every registered environment variable with metadata. Includes type, default, description, and whether it is currently set. |
| `show errors` | Read-only | Show recent errors across all subsystems, newest first. This is the first place to look when something goes wrong. Filter with source <name> to narrow to one subsystem, count <N> to limit output. |
| `show event list` | Read-only | List every event type you can subscribe to. Shows event name, category, and payload structure. Use this to discover what events are available before subscribing. |
| `show event namespaces` | Read-only | List all event namespaces and how many events each has logged. Tells you which subsystems are generating events and how active they are. |
| `show event recent` | Read-only | Show recent events, newest first. Each event includes timestamp, namespace, and type. Filter with namespace <name> to focus on one area, count <N> to limit output. Useful for reconstructing what happened before an incident. |
| `show flow export` | Read-only | Show flow export (NetFlow/IPFIX) collector status. Without arguments, lists all configured collectors. With 'name <name>', shows details for that collector including protocol stats and errors. Returns not-configured when no exporter is active. |
| `show flow recent` | Read-only | Show recent conntrack flow records from the bounded recent-flow ring. Without arguments, returns every ring record (oldest to newest, up to the configured recent-flow-ring capacity). With 'dst <prefix>', filters to flows whose destination is inside that prefix. The ring is fed only while conntrack export is enabled; the filter is by destination prefix (conntrack is host-global and carries no ingress interface). |
| `show geodns` | Read-only | GeoDNS server status: enabled, bind addresses/port, client-IP source mode, zones, nameserver/host-set/source counts, and the current SOA serial. |
| `show gnmi` | Read-only | Show whether the gNMI server is running and how it is configured. Returns listen address, TLS details, authentication mode, and the number of active streaming subscribers. |
| `show health` | Read-only | Is this box healthy? One command to find out. Returns per-component health (bgp, fib, iface, plugins, l2tp, etc.) plus an overall status. Each component reports healthy, degraded, or unhealthy with a reason. Start here when troubleshooting. |
| `show ldp binding` | Read-only | Show LDP FEC-to-label bindings. Lists local and remote label bindings for each FEC (prefix). Use this to verify label distribution is working. |
| `show ldp neighbor` | Read-only | Show LDP neighbors and their session state. Returns peer address, transport address, session state, and hold time for each LDP neighbor. |
| `show log levels` | Read-only | Show what log level each subsystem is using. Lists every registered logger with its current level. Use 'request log level' to change a level at runtime without restarting. |
| `show log recent` | Read-only | Show recent log entries from the in-memory ring. Filters (all optional): level <lvl>, component <name>, count <N>. Newest entries first. Useful when you cannot access the log file directly. |
| `show mpls forwarding` | Read-only | Show MPLS forwarding entries installed in the kernel. Each entry shows the incoming label, swap/push/pop operation, and outgoing next-hop. Pass 'limit N' to cap large tables. Linux only. |
| `show neighbor` | Read-only | Show the ARP and neighbor discovery table. Lists IPv4 ARP and IPv6 ND entries with MAC addresses and states. Pass ipv4 or ipv6 to filter by address family; no argument shows both. For the IPv4-only view, 'show arp' is a shortcut. |
| `show ping` | Read-only | Ping a target from the router itself. Sends ICMP echo requests to <dest> (IP or hostname). Default count is 5. Timeout uses Go duration syntax (e.g. 3s, 500ms). Confirms reachability from this box, not from your workstation. |
| `show pki certificate name` | Read-only | Inspect a specific certificate in detail. Usage: show pki certificate name <name> [pem \| bundle pem \| fingerprint [sha256\|sha384\|sha512]]. Use 'pem' to export for another system, 'fingerprint' to verify identity. |
| `show pki certificates` | Read-only | List all loaded certificates with expiry dates. Shows name, type (CA or device), subject, issuer, expiry, and validity status. Check here to find certificates approaching expiration. |
| `show probe-round` | Read-only | Run a parallel traceroute probe round to a target. Sends all probes concurrently for faster results than sequential traceroute. Returns per-hop RTT and IP. Use probes and max-hops to tune accuracy vs speed. |
| `show reload-status` | Read-only | Show how many config reloads the daemon has processed. Returns a generation counter, the outcome of the most recent reload (applied or failed), and when it finished. The counter advances on every processed reload, including one that rejected or changed nothing, so you can confirm a SIGHUP was acted on even when it deliberately left the running config alone. |
| `show route` | Read-only | Show the kernel routing table. Lists installed routes with next-hop, interface, protocol, and metric. Pass a CIDR prefix or 'default' to filter, or a route limit to cap the output. |
| `show route lookup` | Read-only | Look up which route the kernel would use for a given IP. Performs a longest-prefix-match and returns the matching route with gateway, interface, protocol, and metric. Usage: show route lookup <ip>. |
| `show rr peers` | Read-only | Show route reflector client peers. Lists each RR client with session state and reflected route counts. |
| `show rr status` | Read-only | Show whether the route reflector is active. Returns cluster ID, running state, and summary statistics (reflected routes, client count). |
| `show runtime memory` | Read-only | Show the Go runtime allocator memory stats. Returns allocated bytes, heap in-use, total allocations, GC cycles, and last GC pause duration. Compare over time to spot leaks. For the OS-level process memory (RSS/VSZ) use 'show system memory'. |
| `show static` | Read-only | Show static routes defined in the configuration. Lists each static route with its prefix, next-hop, and interface. |
| `show status` | Read-only | Show process status, uptime, and resource usage. |
| `show storage smart` | Read-only | Show disk health via SMART data. Returns health status, temperature, power-on hours, and self-test schedule for each block device. Replace drives that report failing health before they cause data loss. |
| `show subscriber` | Read-only | Show a summary of all subscriber sessions. Counts by access type (PPPoE, L2TP, IPoE) with totals. Quick way to see how many subscribers are online. |
| `show subscriber id detail` | Read-only | Show everything about one subscriber session. Pass the session ID. Returns access type, assigned addresses, authentication state, uptime, and traffic counters. |
| `show tcp-check` | Read-only | Test TCP connectivity to a remote host and port. Tries to open a TCP connection and reports success or failure with the connection time. Use 'source <IP>' to bind a specific local address. Quick way to verify a peer's BGP port is reachable. |
| `show traceroute` | Read-only | Trace the network path from this router to a target. Shows each hop with its IP and round-trip time. Dest can be an IP or hostname. Defaults: 30 max hops, 3 probes per hop. Increase probes for more reliable RTT measurements. |
| `show uptime` | Read-only | Show how long the daemon has been running. Returns the start time and elapsed uptime. Handy after a maintenance window to confirm the process restarted. |
| `show version` | Read-only | Show the running Ze version and build date. You can verify which release is deployed on this box. |
| `show vpn ipsec peer name` | Read-only | Show full detail for one IPsec peer. Returns IKE SA state, all child SAs with traffic selectors, and byte counts. Usage: show vpn ipsec peer name <name>. |
| `show vpn ipsec sa` | Read-only | Show all IKE and Child Security Associations. Lists every SA with peer, negotiated algorithms, byte counts, rekey timers, and uptime. Includes SPIs, NAT detection, and child SA traffic selectors. Your main IPsec status command. |
| `show vpn ipsec status` | Read-only | Quick IPsec health check. Reports whether the IKE engine is running, how many peers are configured, and how many IKE SAs are Established. |
| `show vrrp` | Read-only | Show every VRRP virtual router: its group name, VRID, address family, state (initialize, backup, master), configured and effective priority, virtual addresses, and the macvlan device that carries the virtual MAC. |
| `show vrrp interface name` | Read-only | Show the VRRP virtual routers on one parent interface. Pass the interface name: show vrrp interface name <interface>. |
| `show vrrp statistics` | Read-only | Show per-virtual-router counters: advertisements sent and received, priority-zero advertisements, gratuitous ARP and unsolicited neighbor advertisement bursts, receive-validation errors by reason, and the derived skew and master-down timers in microseconds (a VRRPv3 skew is sub-millisecond). |
| `show warnings` | Read-only | Show active warnings across all subsystems. Displays any conditions that need your attention (degraded peers, resource limits approaching, etc.). Use 'source <name>' to filter to a single subsystem. |
| `show yang completion` | Read-only | Show YANG paths available for tab completion. Lists every valid completion point in the command tree. |
| `show yang doc` | Read-only | Generate command reference docs from YANG schemas. Produces structured documentation with descriptions, arguments, and usage patterns for every registered command. |
| `show yang tree` | Read-only | Print the YANG tree for a module in a readable hierarchy. Shows node types, data types, and config-vs-state annotations. Similar to 'pyang -f tree'. Useful for understanding the config or command structure. |

## skills (1)

| Command | Mode | Description |
| --- | --- | --- |
| `skills` | Offline | List or retrieve agent skill definitions matching this Ze version. Use 'get <name>' to fetch a specific skill. |

## support (1)

| Command | Mode | Description |
| --- | --- | --- |
| `support` | Offline | Bundle logs, config, state, and diagnostics into one archive file. Send the result to support when reporting an issue. |

## system (8)

| Command | Mode | Description |
| --- | --- | --- |
| `system command complete` | Read-only | Complete command/args |
| `system command help` | Read-only | Show command details |
| `system command list` | Read-only | List all commands |
| `system dispatch` | Read-only | Dispatch a text command |
| `system help` | Read-only | Show available commands |
| `system subsystem list` | Read-only | List available subsystems |
| `system version api` | Read-only | Show IPC protocol version |
| `system version software` | Read-only | Show ze version |

## update (12)

| Command | Mode | Description |
| --- | --- | --- |
| `update bgp irr all` | Daemon | Refresh all IRR prefix-lists immediately. Re-queries the IRR server for every enrolled ASN and atomically swaps prefix-lists on success. Failed refreshes preserve the existing prefix-list and report an error. |
| `update bgp irr as-set` | Daemon | Refresh IRR prefix-list for a specific AS-SET. Usage: update bgp irr as-set <as-set>. Re-queries the IRR server for all peers using the given AS-SET name. |
| `update bgp irr asn` | Daemon | Refresh IRR prefix-list for a specific ASN. Usage: update bgp irr asn <asn>. Re-queries the IRR server for the given ASN only. |
| `update bgp peer prefix` | Daemon | Refresh max-prefix limits from PeeringDB. Usage: update bgp peer <selector> prefix. Queries PeeringDB for each matched peer's ASN, applies the configured margin, and writes the result to the config draft. Run 'config commit' to apply. |
| `update firewall irr all` | Daemon | Refresh all cached IRR prefix-lists. Re-queries the IRR server for every cached ASN/AS-SET entry and updates the zefs cache on success. Failed refreshes preserve the existing cache and report an error. |
| `update firewall irr as-set` | Daemon | Fetch or refresh IRR prefix-list for an AS-SET. Usage: update firewall irr as-set <as-set>. Queries the IRR server and saves resolved prefixes to the zefs cache. |
| `update firewall irr asn` | Daemon | Fetch or refresh IRR prefix-list for an ASN. Usage: update firewall irr asn <asn>. Queries the IRR server and saves resolved prefixes to the zefs cache. Creates the cache entry if it does not exist. |
| `update system firmware apply` | Daemon | Full upgrade: download, verify, stage, and restart. Runs the complete update cycle in one command. Only available on platforms where Ze owns the update lifecycle (e.g. gokrazy). The box will reboot into the new version. |
| `update system firmware check` | Daemon | Check for a new firmware version right now. Bypasses the scheduled interval timer and contacts the update server immediately. Compare the result with 'show system update'. |
| `update system firmware download` | Daemon | Download the latest firmware image right now. Bypasses the maintenance window and spread timers. The image is staged but not applied. Use 'update system firmware apply' or 'restart' to activate it. |
| `update system firmware restart` | Daemon | Reboot into the already-staged firmware. No download happens. Use this after 'update system firmware download' when you are ready to activate the new version. |
| `update system firmware rollback` | Daemon | Roll back to the previous firmware and restart. Reverts to the prior image. Only available on platforms with A/B partitioning (e.g. gokrazy). Use this if the new version has issues. |

## validate (1)

| Command | Mode | Description |
| --- | --- | --- |
| `validate config` | Offline | Check your config for errors without applying anything. Reports syntax and semantic issues. |

## withdraw (1)

| Command | Mode | Description |
| --- | --- | --- |
| `withdraw` | Daemon | Withdraw on-demand announcements. Usage: withdraw tag <key> <value\|*> \| withdraw tag * \| withdraw id <N> \| withdraw all |
