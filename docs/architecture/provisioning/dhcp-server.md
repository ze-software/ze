# DHCP server

The DHCP server serves leases to LAN clients per RFC 2131 and RFC 2132:
the DISCOVER, OFFER, REQUEST, ACK, NAK, RELEASE, and DECLINE state machine,
address pools, lease expiry, static MAC-to-IP mappings, and the standard
options. It also carries the PXE options the provisioning path needs.

<!-- source: internal/plugins/dhcpserver/register.go -- plugin registration and engine lifecycle -->
<!-- source: internal/plugins/dhcpserver/handler.go -- the DHCP state machine and reply building -->
<!-- source: internal/plugins/dhcpserver/pool.go -- address allocation -->
<!-- source: internal/plugins/dhcpserver/lease.go -- lease tracking and expiry -->
<!-- source: internal/plugins/dhcpserver/config.go -- config parsing and validation -->

## Decisions

- **A plugin, not a component.** It owns a config subtree and a lifecycle, and
  it is coupled to neither the BGP engine nor interface management. It registers
  from `init()` and the generated composition root discovers it.
- **Its own bitmap pool, not the L2TP pool.** The L2TP pool assigns IPCP
  addresses to PPP sessions. DHCP needs static-mapping exclusion, MAC-to-address
  tracking, reservation on INIT-REBOOT, and a way to mark an address
  unavailable. Sharing would couple two unrelated domains.
- **One socket per listen interface, serving one segment per request.** Each
  interface gets one UDP socket, bound with `SO_BINDTODEVICE` on Linux. A
  `listen-interface` names a logical interface, which an `os-name` or
  `mac-match` selector can bind to a kernel device of another name, so the
  server resolves it through `iface.ResolveDevice` first and binds the socket,
  and writes reply neighbor entries, on the kernel device that answers, the
  way VRRP binds each group's parent. A name whose selector answers no single
  device is refused, never bound by its logical name, and a refused reload
  keeps the device each previous listener was bound to. The `ze doctor`
  listen-interface check runs in the CLI process, where that resolver table
  is empty, so it reads the selector from the config tree through
  `iface.SelectedNetDevice`, the helper the interface check uses, and probes
  the device it names. An unmatched selector is a warning, an ambiguous one an
  error.
- **A request is served from its segment, which a shared network names.** The
  subnets of one `shared-network` share one physical segment, the meaning ISC
  dhcpd and Kea give it. For each request the serving goroutine resolves the
  segment (`resolveSegment` in `segment.go`): with `giaddr` set, the shared
  network holding a subnet that contains the relay's address; with `giaddr`
  zero, every shared network holding a subnet that contains an address of the
  receiving device. The device's IPv4 addresses are read from netlink
  (`readDeviceAddrs`, beside the neighbor reads) for each request, relayed or
  not, because they also choose the server identifier, so an address added to
  the listen interface is served from the next request on, with no reload. A device that holds no address in any
  configured subnet answers nothing and logs one warning a minute naming the
  interface, its addresses and the fix, and `ze doctor` reports the same
  (`doctor-dhcp-segment-address`). The home subnet is the one holding
  `giaddr`, else the one holding the device's primary address.
- **Within the segment, reservations first, then the request's state.** A
  static mapping for the client's MAC in any subnet of the segment wins over
  every hint and every pool, the administrative choice RFC 2131 Section 4.3.1
  allows on a segment with several subnets. A DISCOVER is then offered the
  address a pool already holds for the client, then a free requested address,
  then a new address from the home subnet's pool and, when that is full, the
  other subnets' pools in the order Ze sorts them (shared networks by name,
  subnets by prefix text). A DHCPREQUEST is classified as
  RFC 2131 Section 4.3.2 defines it: SELECTING (server identifier present) is
  ignored when it names none of the server's addresses on the segment;
  INIT-REBOOT (requested address, no server identifier) is refused with a
  DHCPNAK when the address is in no subnet of the segment or differs from the
  client's reservation or binding; RENEWING and REBINDING (`ciaddr` only) are
  refused when `ciaddr` is in no subnet of the segment, differs from the
  client's reservation or binding, or is held by another client, and a free
  `ciaddr` is bound again, as after a restart that lost the lease table. A
  client holds one binding per segment: the DHCPACK for one subnet releases an
  address it held in another. An INIT-REBOOT for an address on the segment
  from a client with neither a reservation nor a binding draws no reply, as
  RFC 2131 Section 4.3.2 requires, so after a restart that lost the lease table
  a rebooting client waits for its INIT-REBOOT to time out and sends a
  DHCPDISCOVER.
- **The server identifier is an address the server holds, chosen per reply.**
  RFC 2131 Section 4.1 asks for a server identifier "that, to the best of the
  server's knowledge, is reachable from the client", so option 54 (and siaddr,
  unless PXE names a TFTP server) is never derived from the configuration
  alone (`segment.serverID`). It is the subnet's `server-identifier` leaf when
  the receiving device holds that address; else the device's address inside
  the subnet the reply is built from; else, for a relayed request to a subnet
  the device holds no address in, the device's primary address: for a relayed
  message RFC 2131 Section 4.1 has the server choose an address of the
  interface the message arrived on. A `server-identifier` the device does not hold withholds the
  reply, with one warning a minute naming the subnet, the address and the fix,
  rather than falling back to another address in silence; the check runs
  before any pool address is claimed. The commit refuses a leaf value that is
  not a unicast host address, or that is the subnet's network or broadcast
  address, and accepts one outside the subnet, which a relay-only subnet
  needs. A SELECTING REQUEST is the server's when its option 54 is any of the
  identifiers the segment's subnets would send or any device address inside a
  subnet of the segment (`segment.serverIDOwner`), as RFC 2131 Section 4.1
  requires of a server with several addresses. The `default-router` leaf plays
  no part.
- **In-process only.** The selector table `iface.ResolveDevice` reads is
  filled by the interface plugin inside the daemon's process. A forked copy
  holds an empty table and would bind every logical name as if it were the
  kernel device's name, so the plugin refuses to start as an external process
  and names `internal dhcpserver` as the fix, as VRRP does.
- **A static-only subnet gets an empty pool, not a missing one.** A zero-size
  pool with guards in every method is simpler than a nil pool with a branch at
  every call site.
- Non-Linux builds bind to all interfaces. The interface-specific path is a
  Linux socket option.
- **Reply delivery follows RFC 2131 Section 4.1.** A reply to a relayed request
  goes to giaddr on port 67. With giaddr zero, a DHCPNAK is always broadcast.
  An OFFER or ACK goes to a non-zero ciaddr, is broadcast when the client set
  the BROADCAST bit, and otherwise goes to the client's hardware address and
  yiaddr. The client does not hold yiaddr yet, so it would never answer the ARP
  request a plain unicast triggers. Before that unicast the server writes the
  yiaddr to chaddr neighbor entry on the interface the request arrived on, over
  netlink, in the NUD_REACHABLE state dnsmasq's ARP-cache injection also
  produces. The write needs CAP_NET_ADMIN, which the daemon's systemd unit
  grants. When it fails (no capability, an interface gone, a chaddr the link
  layer cannot hold, or a non-Linux build), the reply is broadcast instead and
  an info line names the reason. The server reads the neighbor table first
  and never replaces a PERMANENT or NOARP entry for yiaddr (`neighborHeld`;
  the kernel neither ages nor resolves either by ARP). An operator writes
  PERMANENT; NOARP comes from an operator or from the kernel itself, for a
  multicast or broadcast address and on a device that does no ARP. One that
  names chaddr already delivers the unicast, so nothing is written; one that
  names another hardware address would send the reply to that host, so the
  reply is broadcast. A table that cannot be read is broadcast too, because
  the write could replace an entry the server could not see. The socket is a kernel socket under every
  dataplane, so the kernel neighbor table chooses the frame's destination;
  whether VPP's linux-cp pair carries that frame unchanged to the wire has not
  been proven by a VPP lab run.

<!-- source: internal/plugins/dhcpserver/socket_linux.go -- SO_BINDTODEVICE binding -->
<!-- source: internal/plugins/dhcpserver/socket_other.go -- the non-Linux fallback -->
<!-- source: internal/plugins/dhcpserver/register.go -- listenInterface, the iface.ResolveDevice step before the bind -->
<!-- source: internal/plugins/dhcpserver/register.go -- runDHCPServerPlugin, the external-process refusal -->
<!-- source: internal/plugins/dhcpserver/doctor.go -- checkDHCPInterfaces, the selector read from the config tree -->
<!-- source: internal/component/iface/doctor_selector.go -- SelectedNetDevice -->
<!-- test: internal/plugins/dhcpserver/listen_resolve_test.go -- TestBindListenersRefusesAnUnresolvedSelector, TestRunDHCPServerPluginRefusesExternalProcess -->
<!-- source: internal/plugins/dhcpserver/register.go -- responseAddr, replyDestination, toClientHardware, broadcastReply -->
<!-- source: internal/plugins/dhcpserver/neighbor_linux.go -- installNeighbor, readNeighbor, neighborHeld -->
<!-- test: internal/plugins/dhcpserver/neighbor_permanent_test.go -- TestOfferKeepsPermanentNeighbor -->
<!-- test: internal/plugins/dhcpserver/neighbor_state_linux_test.go -- TestOperatorHeldNeighborStates -->
<!-- test: internal/plugins/dhcpserver/neighbor_integration_linux_test.go -- TestInstallNeighborWritesKernelEntry, TestReadNeighborFindsPermanentEntry -->

## Address ranges

A subnet carries a keyed list of ranges, so disjoint pools on one subnet are
expressible. The parser detects the old single-range shape and accepts it, so no
config migration is needed.

- **A composite pool with one bitmap per segment**, not one flat bitmap spanning
  the whole address space. A flat bitmap wastes memory on the gap between
  disjoint ranges.
- Pool statistics are summed across segments, so a caller sees one total.
- **Overlap detection compares `start[i] <= stop[i-1]`.** Ranges are inclusive,
  so adjacency requires `start2 > stop1` strictly. Reading it as `<` puts one
  address in two segments.
- The segment lookup runs under the pool lock and scans linearly. That is fine
  at the current limit of ten segments. A larger limit needs a sorted search.

## Option 6 and the lease time

Option 6 is the subnet's `dns-source` list, evaluated when each OFFER or ACK
is built (`dhcpHandler.offerDNS`). The `configured` class is `dns-server`, the
`received` class reads the newest `system-dns` snapshot the plugin follows
(`followLearnedResolvers`: a startup subscription, then a snapshot request at
`OnStarted`, so a server that starts after the leases still sees them), and
`self` is the receiving device's address inside the subnet
(`segment.selfAddress`), which a relayed request does not have. The list is a
handed-on one (`dnssource.AudienceHandedOn`), so a loopback or zone-less
link-local server is dropped and logged once per change. The selection is cut
at 63 addresses, the most one option holds.

While the list reaches `received` and finds no usable entry there
(`awaitsReceived`), the reply carries a 60-second lease, or the configured one
when it is shorter, and the lease table records what the ACK carried. RFC 5625
Section 5.3 offers this as one way to keep LAN clients from holding no server
while the WAN is down. A client that leased while the WAN was up keeps its
server until it renews, because the server sends no FORCERENEW.

<!-- source: internal/plugins/dhcpserver/handler.go -- offerDNS, awaitsReceived -->
<!-- source: internal/plugins/dhcpserver/register.go -- followLearnedResolvers -->

## Config reload

A reload that changes nothing under `service dhcp-server` leaves the server
alone: `sdk.ConfigTx` finds the parsed config equal to the running one and
installs nothing. A reload that changes it builds a new handler for every
subnet, with the new options and PXE settings. A subnet whose address plan is
unchanged (the same prefix, the same range bounds, the same static MAC to
address bindings) keeps its pool and lease table, so its leases survive an
edit of its options, its lease time or the PXE settings; every other subnet
starts with an empty lease table. The listeners are closed and bound again
over the new handler set, so a packet that arrives during the swap is lost and
the client retransmits it. The apply logs
`dhcpserver: config applied subnets-kept=N subnets-restarted=M`.

A reload binds every `listen-interface` or none. When one cannot be bound, the
apply binds the replaced config's listeners again over the replaced handlers
and returns an error naming the interface and the fix, so the commit fails and
rolls back with the leases intact. The replaced listeners are bound one by one,
so when another process takes the port of one of them in that window, the
others still serve. The error then wraps `sdk.ErrRunningConfigLost`, says how
many of the interfaces that served are lost and names each one with its cause,
and ends with `sdk.RunningConfigLostNext`: the rollback binds the running
config again and restarts the plugin if that fails too, the reload answer's
`Rollback:` line says which happened, and the fix is to free the port, then,
for a listener a restarted plugin could not bind, to stop the whole daemon and
start it again, which restarts every protocol and drops every session. The text
names the restart routes of `sdk.RestartSteps()`, one per line: no command
restarts one plugin, and `ze signal restart` leaves ze stopped. A commit of
the running config is no fix: it has no diff, so no apply reaches the plugin.
The transaction is broken: its rollback installs the running config again, and
when that fails too it answers an error, so the bridge acks `CodeBroken` and
the engine restarts the plugin; until an install succeeds, a later apply of the
running config installs it rather than skipping it as unchanged. The boot
configure is not strict: it has no previous config to keep, and a configure
error at boot fails the plugin and aborts the whole daemon start, so it logs
`listen failed` for an interface and serves on the others, or logs `no
interfaces bound` when none binds. A VLAN, bridge or dummy that ze's own
`interface {}` config creates exists by then, because the server starts after
the interface plugin (`StartAfter: ["interface"]`). An interface that appears
later, by hotplug for example, is not bound until the next reload.
<!-- source: internal/plugins/dhcpserver/config_tx.go -- dhcpServer.install, buildHandlers, sameAddressPlan, dhcpServer.restore; pkg/plugin/sdk/config_tx.go -- ConfigTx.Apply, ConfigTx.Rollback -->
<!-- source: internal/plugins/dhcpserver/register.go -- bindListeners -->

## Traps

**Exhaustion on DISCOVER is answered with silence, not NAK.** RFC 2131 section
4.3.1: no address available means no response. NAK is valid for REQUEST only.
The instinct to "reject the client somehow" is the wrong one.

**DECLINE marks the address unavailable; it does not release it.** RFC 2131
section 4.3.3. Releasing it hands the same conflicting address to the next
client. The address to mark comes from the requested-IP option, not from the
client's current lease.

**Replacing a lease must release the old address.** Removing the client from the
address index alone leaks the address out of the pool forever.

**An option write must be bounds-checked.** Many DNS servers or a long domain
name overflow the response buffer. The option length field is one byte as well,
so a value longer than 255 bytes would carry a length octet that disagrees with
its data, and the client would read the rest of the value as options (RFC 2131
section 4.1: every option is entirely contained in its field). The DNS server
count, the domain name, both PXE bootfile names and the boot script URL are
rejected at config parse time. At write time, `safeAppendOption` omits an
option whose data is longer than 255 bytes or does not fit before the End
option.

**A received option can sit in the `file` or `sname` field.** RFC 2131 section
4.1 lets a client overflow its options into those two fields, announced by the
option overload option (52) in the options field. Every option reader
(`parseMsgType`, `parseOptionAddr`, `parseOptionBytes`, `parseOptionString`)
goes through one walk, `findOption`: the options field first, then `file` when
option 52 says 1 or 3, then `sname` when it says 2 or 3, and the first match
wins. Only the option 52 found in the options field counts, and it must be one
octet long; one planted inside `file` opens nothing. Without option 52, `file`
and `sname` hold a boot file name and a host name, so they are never read as
options. Each field is walked within its own bounds: an option whose length
octet runs past the end of its field ends the walk of that field, so a peer
cannot make the reader take bytes from the next field or past the packet.
`FuzzFindOption` checks that every answer lies inside a field the overload
value opened. The server never emits option 52 in its own replies.

<!-- source: internal/plugins/dhcpserver/options.go -- findOption -->

**Every configured address must be refused unless it is IPv4.** This server
speaks RFC 2131, and the reply builder narrows each address it sends to four
bytes with `netip.Addr.As4`, which panics on an IPv6 address. The panic lands on
the DISCOVER path, so the trigger is an ordinary client packet and the config
that caused it committed clean. `parseSubnet` refuses an IPv6 subnet prefix, and
`parseIPv4Option` refuses an IPv6 `default-router`, `dns-server` or
`pxe tftp-server`. A range bound and a static mapping need no family check of
their own: each must sit inside the subnet prefix, and `netip.Prefix.Contains`
is false across families. An IPv4-mapped IPv6 spelling is refused with the rest,
which is what `config.IPv4AddressValidator` accepts for every other module.

The YANG module carries `ze:validate "ipv4-address"` on the same leaves. That
annotation declares the family. It does not enforce it, because
`ValidateCustomSections` walks a fixed list of top-level sections and `service`
is not one of them.

**The zero-value address panics.** Converting `netip.Addr{}` to four bytes
panics, which is why every pool mutation guards the empty-pool case. The first
test suite covered DISCOVER only, whose allocation path returns early, and
missed REQUEST, whose reservation path does not.

**A defect met while answering one request costs that request only.** Each
listener reads in `serveRequests`, which the `dhcp` guard wraps as task
`request`. A panic there is logged, counted in
`ze_task_panics_total{component="dhcp",task="request"}` and crash-recorded with
the client's address; the request gets no reply, and `serveMulti` starts the
next pass on the same socket. RFC 2131 section 4.1 makes the client responsible
for retransmission, so the lost reply costs one retry. The pool and lease
tables release their mutexes by `defer`, so the unwind leaves no lock held.
The server has no DHCPv6 path, so there is no second listener to guard.
<!-- source: internal/plugins/dhcpserver/panic_guard.go -- dhcpGuard, requestTask -->
<!-- source: internal/plugins/dhcpserver/register.go -- serveMulti, serveRequests -->

## Functional tests

A `.ci` under `test/dhcp/` drives the server through a real listener with the
compiled probe `internal/test/dhcpprobe`, because no DHCP client can be told
which RFC 2131 Section 4.3.2 state to be in. `le test fixture dhcp/link
<name> <cidr>...` creates the veth pair `<name>`/`<name>p` before the daemon
starts and addresses the server end in the order given, so the first is the
primary. `le test fixture dhcp/probe <name> <step>...` then writes each
request on the probe end with AF_PACKET and reads the server's reply on the
wire. A step `x,<label>,type=...,mac=...,expect=<type|none>` sends one
request carrying only the fields it names (`ciaddr`, `giaddr`, `requested`,
`server-id`, `broadcast`) and checks the reply fields it names (`yiaddr`,
`sid`, `router`, `mask`, `dns`, `to`); MAC addresses use hyphens, because a
`cmd=` line refuses a `:<word>=` span. Other steps add or remove a server
address, route a relay prefix, write a permanent neighbour entry for a
unicast reply, name a sentinel client, or reload (`config2.conf` over
`ze-bgp.conf`, then SIGHUP). An expected silence is proven by the sentinel's
DHCPDISCOVER, which the listener answers after the silent request, or else by
retransmitting across a three-second window. The probe needs CAP_NET_ADMIN and
CAP_NET_RAW, so these tests carry `option=needs-linux:caps=net-admin,net-raw`.
<!-- source: internal/test/dhcpprobe/step.go -- ParseStep, Exchange.Check -->
<!-- source: internal/test/dhcpprobe/probe_linux.go -- CreateLink, Probe.Run, Probe.silent -->
<!-- source: internal/test/fixture/dhcp_fixture_probe_linux.go -- dhcpLinkDriver, dhcpProbeDriver -->

## Related

- `pxe-staging.md` for the two-stage PXE path this handler implements
- `image-server.md` and `tftp-server.md` for the other provisioning services
