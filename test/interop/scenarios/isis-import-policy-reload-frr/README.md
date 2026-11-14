# Scenario: isis-import-policy-reload-frr (relaxing and tightening an IS-IS import chain by reload)

Design: `docs/guide/route-policy.md`, "A reload re-judges every stored path",
and `docs/architecture/rib/unified-locrib.md`. The `-forked` twin runs the same
checks with the system RIB in its own process.

## What it proves

A reload that changes `isis filter import` re-judges the IS-IS path the Loc-RIB
already holds, and needs nothing from IS-IS to do it:

- FRR advertises 10.77.1.0/24 and 10.77.2.0/24 from its loopback. Under
  `ze.conf` Ze's chain rejects 10.77.1.0/24: its one IS-IS path is hidden by
  the `isis import` chain, and it is absent from `show rib` and the kernel FIB
  beside the installed control 10.77.2.0/24.
- Ze's adjacencies and LSDB (every LSP's level, ID, sequence number and
  checksum) are read once they have settled.
- The reload into `ze-reload.conf`, which is `ze.conf` without the chain,
  installs 10.77.1.0/24: its path is selected, and the prefix is in
  `show rib` and the kernel FIB.
- The reload back into `ze.conf` hides it again, at every layer.
- After each reload the adjacencies and the LSDB settle to exactly the first
  reading. FRR's config never changes and a sequence number only grows, so no
  LSP was originated again and no adjacency went down: the re-admission and
  the withdrawal came from the reload alone.

## Topology

One FRR and Ze on the shared lab bridge, IS-IS point-to-point on `eth0`. No
link is declared.

## Assertions

`checkISISImportReload` in
`internal/le/interoplab/bgp/check_import_policy_reload.go`.

## Status

Runs under the Linux Docker interop harness only (raw L2 and FRR isisd).
