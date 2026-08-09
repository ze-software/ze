# OSPF graceful restart

Non-stop forwarding across a control-plane restart (RFC 3623 for IPv4, RFC 5187
for IPv6), in both roles. The restarter floods Grace-LSAs, keeps its FIB,
suppresses self-LSA origination and re-syncs without flapping. A helper holds
the adjacency at Full for the grace period.

## Decisions

- **The state machines are SHARED and only the Grace-LSA carriage differs.**
  IPv4 uses a link-scope Type-9 opaque LSA, Opaque type 3, riding the opaque
  carrier with three TLVs. IPv6 uses a NATIVE link-scope LSA with function code
  11 and two TLVs, routed through the link store.
  <!-- source: internal/plugins/ospf/gr_lsa.go -- grV4Body, grV4Parse -->
  <!-- source: internal/plugins/ospf/packet/grace_lsa.go -- GraceLSA -->
  <!-- source: internal/plugins/ospf/v3/packet/lsa_grace.go -- GraceLSA -->
  <!-- source: internal/plugins/ospf/v3/packet/tlv.go -- tlv, writeTLVs, tlvIterator -->
- **FIB retention is a suppression predicate on the installer.** Apply and
  remove-all are no-ops while the engine is restarting or stopping gracefully, so
  a graceful stop leaves the kernel routes in place. Self-LSA origination is
  gated at the shared origination chokepoint, which covers both address
  families.
  <!-- source: internal/plugins/ospf/gr.go -- suppressInstall -->
  <!-- source: internal/plugins/ospf/spf/install.go -- Installer -->
- **The restarter exits on three triggers**: every pre-restart adjacency is Full
  again, an inconsistent LSA arrives, or the grace period expires. The re-Full
  trigger is wired through the production neighbour Full sink, and the grace
  timer is the always-armed backstop.
  <!-- source: internal/plugins/ospf/gr_restarter.go -- exitRestart -->
  <!-- source: internal/plugins/ospf/gr_helper.go -- helper -->
- **The grace period is measured from the Grace-LSA LS age against the Grace
  Period TLV.** The age starts at zero, is not reset on retransmit, and DoNotAge
  is clear.
- **The restart facts are persisted in non-volatile state**: the restarting
  flag, the grace end, the reason, the IPv6 Interface-ID map and the prefix to
  LSA-ID map. A doctor check guards that path.
  <!-- source: internal/plugins/ospf/gr_nvs.go -- restartFact, writeRestartFact -->
  <!-- source: internal/plugins/ospf/gr_preserve.go -- captureInterfaceIDs, capturePrefixLSIDs -->
  <!-- source: internal/plugins/ospf/gr_show.go -- grSnapshot -->

## Traps

- **OSPFv3 Grace function code 11 (`0x000B`) equals the OSPFv2 Opaque-AS type 11
  (`0x000B`) in the address-family-neutral LS type.** Broadening the link-type
  predicate for `0x000B` would hijack OSPFv2 Type-11 opaque routing. A distinct
  internal sentinel `0x800B` is mapped ONLY at the v6 codec seam and never
  reaches the wire. The opaque and AS-wide predicates are false for it.
  <!-- source: internal/plugins/ospf/types/lstype.go -- LSTypeGraceV6 -->
- **An exit trigger needs PRODUCTION wiring, not a test caller.** With the
  adjacency-Full note wired only in tests, the restarter exits at grace expiry
  alone. The kernel route sweep then runs at about 30 seconds against a 120
  second grace period, which is the exact blackhole graceful restart exists to
  prevent.
- The exit path runs on the grace-timer goroutine. Snapshot the mutex-guarded
  fields INSIDE the lock before the post-unlock work.
- The carrier gained an additive LS-age field on a received opaque delivery, so
  the IPv4 helper can honour the grace clock. The v6 native path reads the LSA
  header age already. Other consumers ignore the field.
