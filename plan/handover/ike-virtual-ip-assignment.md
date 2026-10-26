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

## Test rewrite mapping for the judge (second agent, 2026-10-26)

Old = `git show HEAD:internal/core/eap/pool_test.go` / `pool_release_test.go` (API
`NewPool(v4, v6, dns, domain)`, `Pool.Allocate`, `Pool.Release(net.IP)`). New = the
working tree (API `NewPool(name, v4, v6, dns)`, `Registry.Acquire`, `Registry.Release(*Lease)`).
Every test keeps its old name; the three deleted ones are restored under their old names.

### `eap.TestPoolV6ReleaseRejectsAliasedAddress` (REFUSED, now restored)

| Old assertion | Where it is asserted now |
|---------------|--------------------------|
| NewPool /48 succeeds | same, `t.Fatalf("build the pool")` |
| Allocate succeeds | `r.Acquire(p, "alice", ...)` error check |
| lease has an IPv6 address | implied: `Acquire(..., FamilyIPv6, ...)` returns a lease or an error; `a := leased.Addr().As16()` |
| fixture alias differs from lease | restored: `if alias == leased.Addr()` fatal |
| fixture alias is inside the prefix | `if !p.Contains(alias)` fatal |
| `Release(alias)` returns ErrNotAllocated | restored as `r.Release(forged)` of a `&Lease{addr: alias}` the registry never granted, `errors.Is(err, ErrNotAllocated)`; then `r.IsHeld(leased.Addr())` still true |
| next Allocate does not hand out the held address | `other.Addr() == leased.Addr()` fatal, after bob asks for the alias |
| (judge) bob gets the alias he asked for | added: `if other.Addr() != alias` fatal |
| (judge) both alias and original held | added: `!r.IsHeld(alias)` fatal and `!r.IsHeld(leased.Addr())` fatal (split, one fact per guard) |
| genuine holder releases | `r.Release(leased)` error check (plus: releasing bob's alias lease leaves alice held) |
| second release returns ErrNotAllocated | same, `errors.Is(err, ErrNotAllocated)` |

### `eap.TestVirtualIPPoolAllocate` (7 -> 7)

| Old assertion | Now |
|---------------|-----|
| NewPool error | inline `NewPool` error check |
| Allocate error | inline `r.Acquire` error check |
| `IPv4 != nil` | `!l.Addr().Is4()` fatal |
| not the network address | `l.Addr() == p.Prefix4().Addr()` fatal |
| first lease is 10.10.0.1 | same |
| DNS4 is exactly [8.8.8.8] | `p.DNS(FamilyIPv4)` length 1 and value 8.8.8.8 |
| Domain == example.com | behavior removed by the approved spec (Q-1: commit refuses `domain`, RFC 7296 Section 3.15.1 has no attribute for it). The refusal is asserted in `ipsec.TestVirtualIPPoolCommitRefusals` and `test/parse/ipsec-virtual-ip-pool-domain.ci` (green). Added instead: DNS6 is exactly [2001:4860:4860::8888] |

### `eap.TestVirtualIPPoolRelease` (7 -> 7)

| Old | Now |
|-----|-----|
| NewPool /30 error | inline error check |
| r1 Allocate error | inline `Acquire` error check |
| r2 Allocate error | inline `Acquire` error check |
| third Allocate is ErrPoolExhausted | same |
| Release(r1) error | `r.Release(l1)` error check |
| r3 Allocate error | inline `Acquire` error check |
| r3 equals r1 | `l3.Addr() != l1.Addr()` fatal |

### `eap.TestVirtualIPPoolExhausted` (4 -> 4)

| Old | Now |
|-----|-----|
| NewPool error | inline error check |
| each of 2 Allocates succeeds | inline `Acquire` error check in the loop |
| third is ErrPoolExhausted | same |
| Available() == 0 | `r.Available(p) == 0` |

### `eap.TestVirtualIPPoolDualStack` (no longer flagged by `./le test weakened check`)

| Old | Now |
|-----|-----|
| NewPool error | `mustPool` (helper fatals) |
| Allocate error | `mustAcquire` per family |
| IPv4 leased | `v4.Addr().Is4()` |
| IPv6 leased | `v6.Addr().Is6()` |
| DNS4 count 1 | restored: `len(p.DNS(FamilyIPv4)) != 1` fatal, same mixed DNS list as before |
| DNS6 count 1 | restored: `len(p.DNS(FamilyIPv6)) != 1` fatal |
| (new) an IPv4 request leases one address only; Available drops by two | kept |

### `eap.TestVirtualIPPoolV6LeasesStayInsidePrefix` (deleted, restored; 7 -> 7)

Same five prefixes (/48, /64, /96, /112, /126), one subtest each.

| Old | Now |
|-----|-----|
| NewPool error | inline error check |
| ParseCIDR error | `netip.ParsePrefix` error check (fatal) |
| Allocate error | inline `Acquire` error check |
| IPv6 non-nil | `!l.Addr().Is6()` fatal |
| inside the prefix | `!prefix.Contains(l.Addr())` |
| not the network address | `l.Addr() == prefix.Masked().Addr()` |
| Release round trip for each prefix, /96 and /126 included | `r.Release(l)` error check, plus added `r.IsHeld(l.Addr())` false after it |

`TestIPv6LongPrefixInRange` (spec name, three leases per prefix) stays beside it.

### `eap.TestVirtualIPPoolV6RejectsUnrepresentableRange` (deleted, restored)

| Old | Now |
|-----|-----|
| NewPool of `2001:db8::1/128` errors | same, under the old name; also in the `TestNewPoolRefusesUnrepresentable` table |

### `eap.TestVirtualIPPoolV6ReleaseRejectsForeignAddress` (deleted, restored)

Same /96 pool and the same fixture shape (low 32 host bits of the lease, prefix octets
8-11 set to 0x99).

| Old | Now |
|-----|-----|
| NewPool error, Allocate error | `mustPool`, `mustAcquire` |
| fixture parses | fixture built from the lease; added guard `p.Contains(foreign)` fatal |
| `Release(foreign)` is ErrNotAllocated | `r.Release(&Lease{addr: foreign})` is ErrNotAllocated, and the lease is still held |
| real lease still releases | `r.Release(leased)` error check |
| (added) | bob asking for the foreign address (which `inPrefixCandidate` folds onto alice's address) is NOT given alice's lease, and is leased inside the pool |

### `eap.TestVirtualIPPoolReleaseNotAllocated` (not flagged, strengthened)

Old: `Release(10.10.0.5)` never allocated is ErrNotAllocated. Restored as a forged
`&Lease{addr: 10.10.0.5}` released into a fresh registry, beside the existing
other-registry and nil cases.

## Defect found and fixed: dual-stack selector lists were refused (uncommitted)

Every new `test/parse/ipsec-virtual-ip-pool-*.ci` failed first: `ze config validate`
refused `peer "rw"` with "traffic-selectors name local 192.168.0.0/16 and remote
2001:db8::/32 in different entries but never together". Cause at the producer:
`crossPairs` (`engine/ts_narrow.go`, in HEAD since `9580045d2f`) skipped protocol-disjoint
combinations but paired IPv4 selectors with IPv6 ones, so `validateSelectorCombinations`
refused every dual-stack peer and `answerPairs` refused every dual-stack answer (a v4-v6
pair has no dataplane form). A packet carries one family, so under RFC 7296 Section 3.13
such a pair matches nothing. Fix: `crossPairs` skips a pair whose prefixes differ in
family (`sameFamily`, by mask length). Test first: `TestMixedFamilyCombinationsMatchNoPacket`
in `engine/rfc7296_ts_combinations_test.go` (red before the fix: "crossPairs yielded 4
pairs, want the 2 same-family ones"; green after). Docs: `ipsec-8-ikev2-child-xfrm.md`
and `docs/guide/ipsec.md` (one sentence each). This is its own logical change and its
own commit; `docs/guide/ipsec.md` carries esp-fixed hunks, so commit it through `hunks`.

Also split the compound guard `!Is6() || Is4In6()` in `NewPool`, which `./le arch
compound-guard check` flagged; the IPv4-mapped case now has its own error and its own
row `{"", "::ffff:10.0.0.0/120"}` in `TestNewPoolRefusesUnrepresentable`.

After these edits `./le test weakened check` flags no unit in `internal/core/eap`,
`ike/engine` or `ike/ipsec/{config,types,validate}`; its remaining rows are other
sessions' files.
