# Week of 2026-05-25

MPLS grew a full label-switching stack, flow export and gNMI landed, and config commits became transactional.

## 🛰️ MPLS

Label switching across three layers, verified live in QEMU against FRR:

- Kernel MPLS dataplane: push, swap, and pop programmed via netlink, with relabel support that updates a route ze already owns without clobbering another writer's
- LDP (RFC 5036): per-interface Hello discovery, a TCP session FSM, a label information base, and downstream-unsolicited distribution
- RSVP-TE with FRR interop
- Per-interface MPLS input and MPLS sysctl keys on interfaces

## 📊 Observability & telemetry

- Flow export: sFlow v5, NetFlow v9 (RFC 3954), and IPFIX (RFC 7011), with per-flow records and interface counter export to external collectors
- A new gNMI component (Capabilities, Get, Set, Subscribe over gRPC) for YANG-modeled config management
- `show bgp peer` output enriched to NOS-grade detail, and all IANA well-known BGP community names are now recognised
- `ze support` generates a tech-support bundle, and SMART disk health reporting moved to a pure-Go ioctl reader (no more shelling out to smartctl)
- `ze doctor` picked up more readiness checks: BGP MD5 support detection, VPP socket path validation, NTP/RPKI/BMP checks, and a semantic validation bridge

## 🖥️ CLI

All plugin-owned commands were renamed to a consistent verb-first grammar. Command arguments are now declared as typed YANG leaves, driving completion, validation, and docs from one source. Other additions: `ze help command` for a self-documenting command catalogue, `| first N` and `| last N` pipe operators, session transcript recording (so a crash or dropped SSH session still leaves a local record), a configurable default output format, and ANSI colour support in the terminal buffer library.

## 🔒 Config & commit reliability

Config commits are now transactional: changes go through candidate, active and rollback versions, and are promoted only after a runtime reload succeeds. Failed candidates are cleaned up automatically. A new operation-graph solver orders config changes across components, so an address and the peer that depends on it apply in the right sequence. Config schemas now carry a release-based stamp for future migrations, and YANG `type empty` leaves can be used as presence flags. Separately, an auth bug was fixed: PXE and bootstrap-provisioned appliances were not loading the zefs "power user," which broke SSH login and the web/API per-user auth path on freshly provisioned boxes.

## 🛰️ BGP policy & RIB efficiency

New policy actions: an AS-PATH-length filter, increment/decrement on numeric attributes (local-preference, MED, AIGP), community add/remove, and an RFC 6996 remove-private-as filter. Policy chains can now use plain, operator-facing filter names instead of `<plugin>:<filter>` refs, and `show policy test` runs a chain against a supplied UPDATE without touching live state. On the RIB side, peer reconnect replay and `bgp rib clear out` now use cursor-based grouped resend instead of per-route sends, cutting reconnect replay time on large tables from seconds to roughly 100-200ms.

## 💿 Appliance & provisioning

PXE boot option support (RFC 4578) rounds out last week's DHCP/TFTP/image-server work. A busybox-based initrd now handles bare-metal PXE installs end to end (fetch, verify, write, reboot), and a first-boot bootstrap mode brings up DHCP and SSH automatically when a box has zefs but no config yet. Elsewhere: OTA image push via a vendored gokrazy updater, a unified update backend with platform-aware dispatch, and `ze service` for installing Ze as a systemd service on standard (non-gokrazy) Linux hosts.
