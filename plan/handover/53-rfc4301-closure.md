# Handover 53: close spec-rfc4301-architecture-gaps

Spec: `plan/release/fixes/spec-rfc4301-architecture-gaps.md` (in-progress; the
session that wrote this releases its claim at hand-off; claim it again with
`./le spec claim`). Thomas ordered "close it then" on 2026-11-20. The first
ze-close run found the spec not closable; everything below is the owed
implementation-phase remainder, then ze-close again.

Per-spec state file of the previous session (full history, closer's findings):
`tmp/session/2026-11-20-035940/state/session-state-rfc4301-architecture-gaps-1f8958df-d9ae-41ea-80a8-e372f336140e.md`.

## Uncommitted work in the main checkout (this spec's; carry it whole)

- `rfc/short/rfc4301.md`: the 15 rows that named this spec re-judged. Proven
  and tagged: 4.4.2.1-14, 6-1, 6.2-3, 6.2-4, 6.2-5; 4.4.2.1-16 narrowed to
  `{partial}`; 6.2-3/-4/-5 now `{partial}` (ICMPv4 tested, ICMPv6 gap); 6.2-2 a
  rollup reading as gap; 4.4.2.1-17, 5.1.2.1-1, 8.1-2 re-homed to
  `plan/post-release/kernel/spec-xfrm-outer-df-set-dscp.md` (AC-6, AC-7);
  5.2-2, 5.2-11, 4.4.1.1-3, 7.3-4, 7.3-5, the ICMPv6 case (AC-8) and the
  non-initial-ICMP-fragment classification on 4.4.1.1-2 (AC-9) re-homed to
  `plan/release/validation/spec-ipsec-fragment-receive-and-kernel-proof.md`.
- Tags added on existing tests in `internal/component/ike/dataplane/`
  (`rfc4301_df_linux_test.go`, `rfc4301_dscp_linux_test.go`) and
  `internal/component/ike/engine/` (`rfc4301_icmp_return_integration_linux_test.go`,
  `rfc4301_forward_icmp_integration_linux_test.go`); the stale
  "RFC naming: untagged" markers removed from the two engine files.
- New test `TestRFC4301OperatorBypassForwardsTransitEchoInClear` (6-1
  positive) in `engine/rfc4301_icmp_return_integration_linux_test.go`; the
  discard test's 6-1 claim is now the negative; helper `transitArrived` in
  `engine/spd_forward_integration_linux_test.go`.
- Lint fix owned by this spec: `icmpErrSNMP(t, name)` became `icmpInMsgs(t)`
  in `dataplane/rfc4301_icmp_error_integration_linux_test.go` (lint now clean).
- Citers repointed (ipsec-8, linux-limitations, 12 `.ci` headers, 8 specs);
  `LL-XFRM-ICMP-TRANSIT` rewritten for Ze's kernel bits 5, 6 and 7.
- Four journal rows marked fixed (guard-added-to-one-half-of-a-pair,
  field-carries-two-meanings, guard-enumerates-instead-of-subtracting,
  test-asserts-superseded-behaviour); two rows moved to the fragment spec.
- The spec's own record updated (items 13, 16, 17, Work Not Done in template
  form, A-7 and A-8, documentation row 14).
- `rfc/discrimination/rfc4301/*` and `rfc/audit/rfc4301/*`: records and
  verdicts taken on 2026-11-20 (below).

Other sessions' uncommitted edits sit in the same tree (l2tp, command, thanks,
the rfc7296 2.23-2 audit, several specs): do not carry them.

## Done on 2026-11-20 (evidence)

- An independent ze-test-judge approved 10 units
  (`tmp/commit-rfc-approved-84e4d213.md`, the previous session's commit
  namespace: the approvals may need re-recording under the new session's
  namespace; the verdict reasons are in that file).
- `./le rfc record` written, each red under its break in Ze's arm64 guest
  (kernel `docker-53c57c37`): 4.4.2.1-14 both polarities, 4.4.2.1-16 both,
  6-1 both on the discard test (now superseded: its old entries become
  orphans), 6.2-3 positive on two units and negative
  (`unmatched.go::spdCatchAllEntries`), 6.2-4 positive, 6.2-5 both, and the
  5.2-11 re-take on the non-error test.
- ze-rfc-audit stamped 12 verdicts; 5.2-11 waits for its stamp (its record is
  now taken).
- Unit tests `ike/dataplane` and `ike/engine` green; `ike/dataplane` lint
  clean; `TestLinuxLimitsCheckedAtShippedVersions` green.

## Owed next, in order

1. An independent ze-test-judge on the engine test changes: the new bypass
   test, the 6-1 claim moved to negative, the deleted inferred negative;
   record with `./le rfc approve`.
2. Lint the engine scope; then the 6-1 records and the four mutant re-takes on
   the non-error test. The mutant report is hand-written in gomu format
   because gomu takes no build tags; the commit body must say so. Before
   recording, check that `catchAllYieldsToICMPErrorCheck`'s return,
   `xfrmStateFromParams`'s `state.ICMP = true` and its selector line still sit
   where the report says.

```
K=/Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-53c57c37-5d7f6651/vmlinuz
I=/Users/thomas/.cache/ze/docker-host/arm64/c88a4387bff259f1/docker-host.img
R=tmp/session/2026-11-20-035940/scratch/nonerror-icmp-mutants.json
U1=internal/component/ike/engine/rfc4301_icmp_return_integration_linux_test.go
U2=internal/component/ike/dataplane/rfc4301_icmp_nonerror_integration_linux_test.go
./le go lint run scope "./internal/component/ike/engine"
./le rfc record id RFC4301-6-1 polarity positive unit $U1::TestRFC4301OperatorBypassForwardsTransitEchoInClear route revert producer internal/component/ike/engine/spd_policy.go::spdTransitPolicies kernel $K image $I
./le rfc record id RFC4301-6-1 polarity negative unit $U1::TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA route revert producer internal/component/ike/engine/spd_policy.go::spdTransitPolicies kernel $K image $I
./le rfc record id RFC4301-6.1.2-1 polarity positive unit $U2::TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded route mutant report $R mutant internal/component/ike/dataplane/xfrm_linux.go:886:2#1 kernel $K image $I
./le rfc record id RFC4301-6.1.2-1 polarity negative unit $U2::TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded route mutant report $R mutant internal/component/ike/dataplane/xfrm_linux.go:886:2#2 kernel $K image $I
./le rfc record id RFC4301-6.2-4 polarity positive unit $U2::TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded route mutant report $R mutant internal/component/ike/dataplane/xfrm_linux.go:300:3#1 kernel $K image $I
./le rfc record id RFC4301-6.2-4 polarity negative unit $U2::TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded route mutant report $R mutant internal/component/ike/dataplane/xfrm_linux.go:251:3#1 kernel $K image $I
./le rfc audit-stamp stem rfc4301 from tmp/session/2026-11-20-035940/scratch/rfc4301-audit-rejudge-5.2-11.json mode rejudge
```

   A clean-run failure of the new bypass test before the entry is installed
   means the catch-all is not dropping transit Echo: a finding, not a flake.
3. ze-rfc-audit (independent) on 6-1, 6.2-3, 6.2-4, 6.2-5 and 4.4.1.1-2, then
   `./le rfc audit-stamp`.
4. `./le rfc update` and `./le rfc check`: no rfc4301 finding except the
   removable orphan 6-1 entries (remove them). `./le doc check all` and
   `./le spec citation`: the remaining citation hits are the spec's own
   references, gone with the spec.
5. Commit the work as one coherent commit carrying the `RFC-approved:` lines,
   then rerun `/ze-close`: Review Gate, commits A and B, `./le spec release`.

Known reds outside this spec: `./le doc check all` (another session's
`registry.Meta.DaemonPath`, thanks), `./le cli grammar` (another session's
`arch error-inventory` and `error-wrap`), and the VPP `fib` deployment
scenario ("real VPP FIB route not observed", unowned, not investigated).
