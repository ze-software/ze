# Handover: ike-virtual-ip-assignment

Spec: `plan/release/fixes/spec-ike-virtual-ip-assignment.md` (in-progress, Phase 1/9 recorded,
approval line added to the metadata). Written 2026-10-26 by the first implementation agent,
which stopped at its 100-call budget.

## Done

| SHA | Content |
|-----|---------|
| `4d4c6cb473` | Codec (spec step 1, AC-19): `PayloadCP.ReadFrom` refuses a 1-3 octet remnant (`ErrTruncated`) and more than 64 attributes (`ErrCPTooManyAttrs`); constants 5, 13, 14, 15. `TestCPReadFromBounds` |

## In the working tree, NOT committed (steps 2, 3, 4)

The commit was prepared and REFUSED by the weakening gate. It needs an independent
`ze-test-judge` verdict on five tests, then `./le test weakened approve unit <test> reason "..."`
for each, then `./le commit create` again with the same arguments (file list
`tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch/vip-chunk2.list`, plus
`hunks` patches for `ze-ipsec-conf.yang`, `ipsec/config.go`, `docs/guide/ipsec.md` in the same
scratch directory, named `<path with _>.patch`; rebuild them from `git diff` if the tree moved).

| Test flagged | Why it changed (the case for the judge) |
|--------------|------------------------------------------|
| `eap.TestVirtualIPPoolNeverHandsOneAddressTwice` | API move `Pool.Allocate` to `Registry.Acquire`; the distinctness assertion is kept; the nil-result checks no longer exist because `Acquire` returns a lease or an error |
| `eap.TestPoolV6ReleaseRejectsAliasedAddress` | the registry keys leases by full address, so the aliasing defect class is gone; the test now asks for the alias as a requested address and asserts both leases stay distinct and independently releasable |
| `ipsec.TestValidateEAPUserPassword`, `ipsec.TestValidateEAPTLSUserCertificate` | their fixture pool had no range; a range-less pool is now refused first, so the incidental pool line was removed to keep the asserted error reachable |
| `engine.TestReloadRebuildsTheVirtualIPPoolWhenTheOperatorEditsIt` | rewritten to the registry API; keeps edited-range and DNS-rebuild assertions and adds an unchanged-pool assertion; the gate counts three config literals folded into one helper |

Uncommitted files: `internal/core/eap/pool.go` (registry + descriptor), `pool_test.go`,
`pool_release_test.go`; `ike/ipsec/types.go`, `validate.go`, `config.go`, `config_test.go`,
`yang/ze-ipsec-conf.yang`; `ike/engine/virtual_ip.go` (new), `apply.go`, `apply_test.go`,
`config.go`, `table.go`, `reconcile.go`; `test/parse/ipsec-virtual-ip-pool-{config,domain,
initiate,overlap,selector-coverage,unknown}.ci` (new), `test/parse/ipsec-remote-access.ci`
(domain line removed: now refused); docs `ipsec-9-ikev2-eap-nat.md`, `ipsec-7-ikev2-engine.md`,
`docs/guide/ipsec.md` (source anchor only).

Green: `go test -race` over `internal/core/eap`, `internal/component/ike/ipsec`,
`internal/component/ike/engine` (engine takes about 200 s). NOT run: the six new `.ci` files
(`./le test functional parse` is owed; the hook refuses `go build` outside `bin/`).

## Left, per AC

| AC | State |
|----|-------|
| AC-8, 17, 18, 20, 25, 27 | pool-layer tests written and green (uncommitted) |
| AC-30, wiring `TestPeerBindingSelectsNamedPool` | green (uncommitted) |
| AC-31 | unit `TestVirtualIPPoolCommitRefusals` green; `.ci` files unrun |
| AC-1..7, 7b, 9..16, 21..24, 26, 29, 32 | not started: `cp.go` consumer (steps 5-7). Insertion point `selectAuthChildSA` (`responder.go`), substitution in `narrowChildSelectors`, release in `(*SA).forgetKeys`, rekey move in `established.go`. Use `ps.boundVirtualIPPool()` and `ps.virtualIP.leases` |
| AC-28, 33 | not started: doctor check, metrics, `show vpn ipsec sa` `virtual-ip` |
| Interop, RFC rows | not started: both scenarios, `RFC3948-5.1-1` tagged pair and discrimination record, RFC 7296 rows, `./le rfc check` |
| Docs | owed: `ipsec-3-data-model.md` and `docs/config-reference.md` carry no pool text at all and need the new leaf; guide claims stay true until the consumer lands |

## Traps met

- `SiteToSitePeer` sits at the gocritic hugeParam edge (`types_size_test.go`): a 16-byte string
  does not fit. `ConnectionType` became `uint8` in the tail padding and the binding is a
  pointer `VirtualIP *VirtualIPBinding`, read through `VirtualIPPoolName()`, placed before `IfID`.
- The Bash hook refuses any command whose text contains `plan/` writes; it also refused a
  Python script whose replacement string named a `plan/` path in a YANG description.
- Another session (esp-fixed work) has live hunks in `ze-ipsec-conf.yang`, `ipsec/config.go`,
  `docs/guide/ipsec.md` and several engine files; commit only through `hunks` there.
- The spec test name `TestRequestedAddressHonoured` trips `misspell`; it is `TestRequestedAddressHonored`.
