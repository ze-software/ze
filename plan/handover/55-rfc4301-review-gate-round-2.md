# Handover 55: spec-rfc4301-architecture-gaps, from round-1 fixes to closure

Spec: `plan/release/fixes/spec-rfc4301-architecture-gaps.md`, in-progress. Claim it
with `./le spec claim spec-rfc4301-architecture-gaps.md`. Thomas ordered it closed.
Supersedes handover 54, whose items 1 to 11 are now committed in the progress commit
"ike: rfc4301 review round 1, policy ownership and catch-all yield" (find it with
`git log --grep 'rfc4301 review round 1'`). That commit names its evidence and reds.

## State at hand-off

- Round-1 fixes committed: XFRM policy ownership both ways, catch-all yield to a
  wildcard 0/0 peer (`dataplane/policy_final.go`), VPP IKE bypass, per-socket IKE
  bypass moved to `dataplane.BypassIKESocket` (transport copies deleted), E14
  transport-mode test, the E13 notes, AC-23 in the spec for the catch-all fix.
- Judge pass 1 done: every unit approved except the `probe-esp-discard` retarget,
  which was refused and reverted (the probe targets the device address again).
- VPP unknown-SPI ESP passes the inbound IKE BYPASS (VPP's inbound matcher compares
  addresses alone): design-needed, journal row in
  `plan/journal/escape-hatch-scoped-wider-than-its-justification.md`, owned by
  `plan/post-release/features/spec-ipsec-vpp-policy-interface.md` AC-10. Latent.
  `probe-esp-discard` in the VPP deployment evidence is red until that lands.

## Owed, in order

1. **AC-10 cell fix** in `spec-ipsec-vpp-policy-interface.md`: its Notes cell still
   says `vppCleartextOtherHost` (100.64.45.20) aims the probe away; that constant is
   gone and the probe targets 100.64.45.1 and is red. (The spec-edit hook wants the
   forwarding, system-plugins and routing-protocols doors read first.)
2. **Judge pass 2** (`ze-test-judge`, independent agent) on the five proposal patches
   in `tmp/session/2026-11-20-233558/scratch/proposed-E{3,8,10,11,13}.patch`
   (each passes `git apply --check`; E8 and E13 both touch
   `rfc4301_fragment_forward_integration_linux_test.go`, apply E8 then E13). They are
   RFC-tagged: on approval `./le rfc approve unit engine.<Test> reason "..."`, then
   `git apply`. E3 renames a tagged unit: its six records (RFC4301-4.4.2-1, 5.2-2,
   5.2-11) are re-taken, then `/ze-rfc-audit rfc4301`, `./le rfc update`, `./le rfc check`.
3. **Record re-takes** (`./le rfc record`) in Ze's arm64 guest for every tagged unit
   changed: E3's six, E8 RFC4301-7.4-2, E10 RFC4301-4.4.1-3, E11 RFC4301-5-1, the two
   judge-approved tagged units (`TestRFC4301BareESPOnTemplatedSAIsPolicyChecked`,
   `TestRFC4301OutboundPacketTakesTheDispositionOfTheEntryItMatches`), and whatever
   `./le commit create` lists as stale. Kernel
   `~/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-53c57c37-5d7f6651/vmlinuz`,
   image `~/.cache/ze/docker-host/arm64/c88a4387bff259f1/docker-host.img`.
4. **Scoped runs still owed**: `./le go lint run scope` for ike/engine, ike/dataplane,
   ike/transport (should be clean now the dead files are gone) and le/test/qemu
   (aborted: a file changed under it); `./le arch tier check`; guest integration
   tests (`./le test qemu`): `TestOperatorGateNeverReplacesALiveChildSAPolicy`,
   `TestWildcardPeerStandsInForTheCatchAllAndGivesItBack` (untried, a 0/0 Child SA
   may trip the lab), `TestRFC4301TransportChildSANeverAppliesESPToAnIPv4Fragment`,
   `TestMHChildPoliciesSelectIncomingIPv6Packets`,
   `TestRFC4301ForwardedICMPWithoutStateCheckRequiresOwnHeadersToMatch`,
   `TestRFC4301LocalGeneratedICMPWithoutMappingFlagDoesNotUseReturnSA`,
   `TestRFC4301ForwardedFragmentsRespectOperatorDisposition` (E6/E8 reassembly
   assumption: +2 on the DISCARD row means it is wrong),
   `TestRFC4301SiblingSAsKeepInboundSelectorsBound`, the D1 file's three tests;
   `./le test functional parse` for `ipsec-icmp-type-selector-refused`; VPP
   deployment evidence where the x86 image exists.
5. **Second review round** over the fixes only (independent `ze-read` reviewers).
6. **Record the Review Gate**: `./le spec review record spec rfc4301-architecture-gaps
   verdict CLEAN rounds <n> file <path>...` over every path `judgeReviewCoverage`
   names (its refusal from `./le commit create append ... remove <spec>` lists them),
   then `./le spec review check`; fill the spec's Review Gate with every finding.
7. **Close**: commit A (spec, `rfc/short/rfc4301.md`, the fragment-receive spec,
   website fragment, audit files, remaining fixes), commit B (`remove` the spec),
   `./le spec citation`, `./le spec release`.

## Open notes

- `rfc/short/rfc4301.md` rows RFC4301-4.1-3 and 4.1-4 still say the child-SA path
  hardcodes tunnel mode; false since transport mode exists. Fix with their re-judge.
- Auditor note: the 6-1, 6.2-3, 6.2-4, 6.2-5 reasons cite Section 6's first
  paragraph for ICMPv6; it is the second. Correct and re-take the four verdicts.
- E13 side effect: `fragmentChildrenWithDataplane` now removes the child; tests in
  `fragment_sa_lifetime_integration_linux_test.go` may remove `children[0]` twice
  (second removal goes to a discard logger). Confirm in the guest run.
- E14 does not read back the Scope `installChildSA` hands the backend.
- Reds outside this spec: le/test/qemu `TestEveryIntegrationPackageIsNamed`
  (container/imagestore, cos), le/test/deployment
  `TestRefusedProofStartRemovesItsContainer` (image cache from 4d8f0c0dd1),
  vpppppoe lint.
