# 1139: Per-Family Address Configuration

**Spec:** `spec-iface-1-per-family-address`
**Date:** 2026-05-17

## What Changed

Moved interface addresses from a flat mixed-family list at the unit level into
per-family `ipv4 { address [...]; }` and `ipv6 { address [...]; }` containers,
following the Junos/Nokia model.

## Key Decisions

- **No backward compatibility.** The flat `address` leaf-list at unit level was
  removed from YANG and the Go parser. No legacy parsing, no deprecation warning,
  no migration path. Per `ai/rules/compatibility.md`.
- **Renamed structs.** `ipv4Sysctl`/`ipv6Sysctl` became `ipv4Settings`/`ipv6Settings`
  since they now hold addresses alongside sysctl knobs.
- **Merged flat list preserved.** `unitEntry.Addresses` is still populated as the
  union of per-family addresses for the apply path (`config_apply.go:desiredState()`),
  so the reconciliation and netlink paths are unchanged.
- **Family validation in Go.** `netip.ParsePrefix` + `Is4()`/`Is6()` reject
  wrong-family addresses at parse time. YANG patterns provide a coarse first filter.

## What Went Well

- Error propagation through `parseIfaceEntry`/`parseUnits` was a clean change;
  all callers already had error handling paths from `parseIfaceConfig`.
- Keeping `u.Addresses` as a derived merge meant zero changes to `config_apply.go`.

## Mistakes

- Initially implemented legacy migration (split flat addresses by family) before
  the user clarified no backward compatibility was wanted. Removed cleanly.
- Created a memory entry for "no backward compat" without checking that
  `ai/rules/compatibility.md` already covers it. Rule: always grep rules before
  creating memories.
- Left 7 `.ci` parse tests using the old flat `address` in `unit`. Tests that
  reference the changed schema must be updated in the same commit as the schema
  change. Rule: grep `test/` for any field you remove or relocate in YANG.

## Follow-Up

- `spec-iface-3-unit-naming` (skeleton): change unit key from numeric ID to
  freeform name (e.g., `firewall-3`, `supplier-acme`).

## Files

None recorded.
