# Week of 2026-06-15

Native IS-IS landed, MPLS gained fast reroute, and firewall rules can now pull straight from the IRR.

## 🛰️ IS-IS & MPLS

- A native IS-IS link-state IGP (ISO/IEC 10589, RFC 1195/5305/5308/5301/5303/5304/5310/2966): full PDU/TLV codec, adjacency FSM for point-to-point and LAN, LSDB flooding, DIS election, SPF with ECMP install into the Loc-RIB, HMAC auth, and dual-stack IPv6, interop-tested against FRR's isisd. Interface config now nests under `interfaces { }`, matching OSPF's convention.
- RSVP-TE fast reroute (RFC 4090 facility backup): a Point of Local Repair redirects a protected LSP onto a pre-armed bypass on link failure without tearing down the LSP, and tunnels reconcile correctly across a config reload.
- BGP-LU labeled unicast now has a full label path through the kernel FIB, LDP, and RSVP-TE.

## 🛰️ BGP

- SR-Policy routes: decode, JSON output, and CLI `--nlri decode` support
- A pass fixing eight RFC 9252 compliance bugs in SRv6 Prefix-SID, found during external review (wire-format field sizes, TLV bounds checks, SID length validation)
- Per-family ADD-PATH mode enforcement, plus a new PATHS-LIMIT capability (draft-abraitis-idr-addpath-paths-limit) with add-path config unified into a single block
- FlowSpec rate-limit now accepts an explicit `:bytes` unit (RFC 8955), and the ExaBGP-compatibility bridge moved to version 6.0.0 to match ExaBGP's own unified rate-limit syntax
- A route-server race that could send duplicate or misordered End-of-RIB markers on reconnect is fixed

## 🔒 Firewall / IRR

A new firewall plugin resolves ASN and AS-SET references against the Internet Routing Registry and turns them into nftables prefix-list filters automatically, with dual-stack support, per-interface source validation, and commit-time verification that rejects any reference that hasn't been cached yet.

## 🔌 Interfaces

Interfaces can now bind to the underlying kernel device by hardware MAC address instead of by OS-assigned name, so the binding survives a NIC rename or a MAC override. `show interface` also now exposes the permanent factory MAC and OS device name.

## 🖥️ CLI

Running `ze` with no arguments now opens a hierarchical TUI menu of every command, with type-ahead filtering and drill-down into sub-commands. Environment-key values now autocomplete in both the operational CLI and the shell, and help output is consistently colored across every surface.

## 💿 Appliance & provisioning

Provisioning now logs per-request detail (DHCP lease, TFTP read, served image identity) at a visible level by default, so a PXE boot sequence isn't silent, and an installed appliance can report which build it's running via `ze version`. PXE-provisioned interfaces get their address auto-configured. The runtime kernel picked up fixes for L2TP support on kernel 7.0 and for the serial console driver, and the installer now shows console output on headless serial hardware and recovers from a foreign DHCP lease that has no route back to the install server.
