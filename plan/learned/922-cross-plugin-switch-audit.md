# 922 -- cross-plugin-switch-audit

## Context
Spec `cross-plugin-switch-audit` audited every `switch` that dispatches on data
originating in a different plugin/component (~45 in-scope sites across 10
boundaries) and asked which are structurally necessary vs. which leak
producer-owned behavior into consumers. The headline result: **most cross-plugin
switches are correct Go and must stay.** Backend lowering (a RouteAction, a
firewall `Match`, a `QDiscType` turned into a concrete kernel/VPP/P4/nftables
call) has no virtual-dispatch alternative; per-consumer interpretation whose body
genuinely diverges cannot be hoisted without inventing a fake abstraction. Only
**6 of ~45** were real smells, and 3 of those were hiding latent bugs.

## Decisions
- **Producer-owned dispatch.** When several consumers switch *identically* on a
  producer's enum, put the mapping on the producer's type as a method and let the
  dependents call it. This is Ze's registration/ownership philosophy applied to
  value types, not premature abstraction: the consumers already import the
  producer. Implemented as `RouteAction.Verb()` (5 FIB backends), `Path.IsEBGP`
  carry-through (sysrib), `AddPathMode.Label()` (rs + decode), `formatFamily` ->
  family registry, `host.ValidPlatformName` (doctor + diagnostic),
  `firewall.ProtocolNumber` (nft + vpp classify + vpp nat).
- Kept the FIB install *bodies* per-backend; only the action->verb *dispatch* was
  shared. The initial assumption that the switches were "identical enough to
  consolidate" was half-right and half-wrong (see Gotchas).
- Named `AddPathMode.Label()` not `String()`: a Stringer returning "" for the
  None case would silently corrupt `fmt` output.

## Consequences
- Three latent bugs fixed for free by removing the duplication that hid them:
  - **R-B5**: an operator `admin-distance` override (eBGP != 20 / iBGP != 200)
    made the *startup-replay* path re-derive the class from admin distance, fall
    through to `Unspecified`, and drop the per-type override -- disagreeing with
    the live event-bus classification. sysrib now reads `Path.IsEBGP` instead.
  - **R-B3**: `buildDNATMapping` programmed protocol 0 for every protocol other
    than tcp/udp (icmp, sctp, gre, ...) because its inline switch only knew two.
  - **R-B10a**: `formatFamily` hardcoded `flowspec`, drifting from the registry's
    canonical `flow`.

## Gotchas
- **"Cross-plugin switch" is not automatically a violation.** The audit's value
  was the *triage*, not the refactors: separating the 6 producer-owned smells
  from the ~39 switches that are structurally necessary or legitimately local.
  Resist the urge to "fix" backend-lowering switches -- there is nothing to fix.
- **A type can be fully wired and still fail `ze-validate`.** A type is often
  reached without ever spelling its bare name in another package: callers switch
  on its constants (`RouteVerbInstall`) or read it through a struct field
  (`inv.CPU`, `cap.Families`), so the gate's whole-word `grep` finds no
  cross-package hit and flags it as dead. Fixed at the source:
  `scripts/dev/validate.py` now treats a type as wired when (a) any of its
  exported constants is referenced cross-package (iota-inheritance aware) or
  (b) it is used as a struct field type within its own package (serialized/wire
  structs). It also exempts `*ForTest` helpers, which are test-only by
  convention and the caller search excludes test files. The wrong fix would have
  been to unexport these types, which fights revive's exported-return rule and
  muddies legitimate public API. (What the gate legitimately keeps flagging:
  exported funcs with genuinely no production cross-package caller -- e.g. an
  API-symmetry convenience func that nothing consumes yet -- which is an
  over-export signal, not a false positive.)
- **A carry-through field must be excluded from identity.** `Path.IsEBGP` (like
  `Path.Labels`) is metadata, not part of best-path identity, so it is excluded
  from `Path.Equal`/`key` -- otherwise it would perturb best-path arbitration.
- **Concurrent sessions interleave shared files.** R-B5's production code
  (`candidate.go`, `rib_bestchange.go`, `sysrib.go`) was committed by a parallel
  MPLS session that co-edited the same files; only the R-B5 *test* remained for
  this spec to commit. Audit work by symbol/behavior, not by which commit it
  lands in.

## Files
- `internal/component/bgp/types/action.go` (`RouteVerb`, `RouteAction.Verb()`);
  `internal/plugins/fib/{kernel/fibkernel,vpp/fibvpp,vpp/srv6,p4/fibp4}.go`
- `internal/component/firewall/protocol.go` (`ProtocolNumber`);
  `internal/plugins/firewall/{nft/lower_linux,vpp/classify_linux,vpp/nat_linux}.go`
- `internal/core/bgp/capability/capability.go` (`AddPathMode.Label()`);
  `internal/component/bgp/format/decode.go`; `internal/component/bgp/plugins/rs/server.go`
- `internal/component/bgp/plugins/rib/rib_nlri.go` (`formatFamily`)
- `internal/component/host/inventory.go` (`ValidPlatformName`);
  `internal/component/doctor/registry.go`; `internal/core/diagnostic/doctor_registry.go`
- `internal/component/sysrib/sysrib_protocoltype_test.go` (R-B5 test; production landed via 919-921 MPLS series)
- `scripts/dev/validate.py` + `validate_test.py` (typed-enum-wired-via-constants gate fix)
