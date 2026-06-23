# 973 - OSPFv3 interop-coverage completion + v6 stub origination + ze-validate export hygiene

## Context

After the unified OSPF engine (af-unify), three OSPFv3 interop-coverage gaps and the ospfv3 export-hygiene item remained. The spec framed all four as "tests + export hygiene only, no control-plane change." Investigation broke that premise twice: OSPFv3 stub-area DEFAULT origination was NOT implemented (only the v4 path called `spf.applyAreaTypePolicy`), and `make ze-validate` was red on 45 symbols, only 8 in ospfv3 (the rest belonged to other in-progress specs). The shared-LAN "no unique prefix to advertise" blocker turned out to be solvable: a Ze passive dummy interface with a configured global v6 address makes a route assertable on the one-bridge harness.

## Decisions

- Implemented OSPFv3 stub default origination (`v6ApplyAreaTypePolicy`) rather than the tests-only Ze-as-stub-internal topology, per user choice, for full v4 parity. Kept it in a NEW file with only a one-line call in the contested shared file.
- Taught `scripts/dev/validate.py` an interface-seam exemption (a method on an UNEXPORTED receiver type satisfying a same-package exported interface) instead of unexporting `transport.LinkLocalSource`, which a cross-package test backend implements (an interface method cannot be unexported without breaking external implementors).
- Provisioned a unique advertisable prefix with `interface { dummy zeloop { unit 0 { ipv6 { address [...] } } } }` + a passive OSPF interface, so FRR installs a real inter-area / intra-area route — stronger than the v4 LSDB-only assertion.
- Scoped AC-4 to the ospfv3 packages (the spec's literal wording); the 37 non-ospfv3 validate issues belong to other specs and were left untouched.

## Consequences

- A v6 stub ABR now originates a single `::/0` Inter-Area-Prefix default at default-cost, suppresses Inter-Area-Router-LSAs into stub/NSSA, and (totally-stubby) suppresses other inter-area prefixes — symmetric with `spf.applyAreaTypePolicy`.
- `validate.py` no longer false-flags interface-seam methods on unexported receivers; the same blind spot affects other packages (e.g. OSPFv2 transport) that other specs can now clear.
- Interop route assertions on the single-bridge harness no longer need a 3-container topology: a passive dummy with a unique global prefix suffices (OSPFv3 advertises passive-interface global prefixes; link-local/loopback are filtered).

## Gotchas

- A parallel session sharing the working tree repeatedly reverted `origination_v6_summary.go` and its test; LSP in-memory buffers disagreed with on-disk content (lint/`go test` read disk, the authoritative source). Minimise edits to a contested file and keep new work in new files; a Monitor on the call line confirmed stability.
- `validate.py changed_files()` includes untracked files (`git ls-files --others`), so a wholly-new untracked tree IS scanned; but `make ze-validate` exiting non-zero can be 90% other specs' symbols — always scope the count before believing a spec's "~N in package X".
- `LSType.Scope()`/`FloodScope` were only test-used; unexporting `FloodScope` broke cross-package `packet` tests that named `types.FloodScope(1)` — rewrite them to compare `.Scope()` against a known area-scoped type's `.Scope()` instead of naming the unexported type.
- The stub adjacency forming at all proves the v6 Hello E-bit options are cleared for stub areas (address-family-neutral area options), not just receive-suppression.

## Files

- `internal/plugins/ospf/origination_v6_stub.go` (+`_test.go`), one-line call in `internal/plugins/ospf/origination_v6_summary.go`
- `internal/plugins/ospfv3/{types,packet,transport}/*` (export unexports: decode* helpers, floodScope, wordLen, enableInterfaceInstance)
- `scripts/dev/validate.py` (interface-seam exemption)
- `test/interop/interop.py` (FRROSPF6 `has_inter_area_prefix_lsa`, `inter_area_prefix_dump`, `has_as_external_lsa`)
- `test/interop/scenarios/ospf-v6-{multiarea,stub}-frr/` (new), `ospf-v6-broadcast-frr/{ze.conf,check.py}` (route assertion)
- `docs/functional-tests.md` (OSPF interop scenarios paragraph)

## Review-fix + regression follow-up (2026-06-24)

A 6-reviewer OSPF code review (1 BLOCKER, 8 ISSUEs, lower items) was fixed on top of the spec. All fixed with regression tests; E1/E2 ordering and a couple of items were assessed as deliberate/not-bugs and kept.

- **#5 tentative-link-local was a self-inflicted regression caught ONLY by interop.** Skipping any `IFA_F_TENTATIVE` link-local (RFC 4862-correct) broke OSPFv3 binding in the bridged-container harness, where IPv6 DAD never completes so the link-local stays tentative-yet-usable. It passed every unit test, `GOOS=linux vet`, and lint; FRR's empty neighbor table (zero Hellos) exposed it. Fix: `interfaceLinkLocal` PREFERS a DAD-complete link-local but FALLS BACK to a tentative one. **Lesson: a v6 source-selection change MUST be interop-revalidated; unit tests cannot see container DAD reality.**
- **Self-Link-LSA fight-back (RFC 2328 §13.4) can't mirror the area path.** `normaliseLSA` re-encodes a `RawBytes`-nil LSA through the OSPFv2 codec, which misparses a v6 Link-LSA. So the link path reclaims by bumping its own sequence record past the received instance (next origination wins) instead of reinstalling a re-stamped copy. Watch the OSPF signed-sequence trap: a zero ownRecord compares "newer" than the negative `InitialSequenceNumber`.
- **v6 OSPF->BGP redistribution-OUT is a real, deliberate phase boundary**, not a bug: `wireRedistProducer` is wired only to the v4 engine `eng`, never `eng6`. The hardcoded `AFIIPv4` in `emitDelta` is correct for the v4-only export path.
- The four pure-perf review NOTEs (per-tick Fletcher/decode, triple topology snapshots, O(N^2) iface walks, SPF v6NetworkVertexRef) iterate OSPF's BOUNDED per-interface/per-area sets — negligible cost; left unoptimized (no benchmark, regression risk).
- **Commit BLOCKED by multi-session entanglement.** This spec's delta is interleaved with the parallel `spec-ospf-af-unify` session's entire UNTRACKED OSPFv3 implementation (385 of 455 uncommitted entries) AND with non-OSPF parallel work in shared infra files (validators, sysrib, locrib, iface, all.go, codes) that cannot be split without `git add -p`. A clean OSPFv3-only commit that also builds is infeasible from one session; closure needs the user to bundle both sessions' OSPFv3 work or have af-unify commit the implementation first.

Follow-up fixes (all with tests): B1 Link-LSA retransmit, I1 nssaMu TOCTOU, I2 Instance-ID demux, I3/I4 SPF E1-ECMP + range coverage, I5 DD-seq, I6 over-long-count negatives, I8 v6 redist injector, #1 LS-Length/packet clamp, #2 v6 metric mask, #3 ISM BackupSeen, #4 DD retransmit, #6 config enum/cost validation, #7-link MaxSequence flush, #17 4-in-6 family guard, #18 stale d.own + Install link-type guard, AuType-2 ESN counter-wrap.
