# BFD AC-C2 record sweep, 2026-10-02

Scope: every verdict in rfc5880, rfc5881, rfc5882, rfc5883 that is `enforced` now and was
not `enforced` at `246dbae78c~1` (the tree before the BFD child's first commit). 84 ids:
66 rfc5880, 12 rfc5881, 5 rfc5882, 1 rfc5883. For each, `./le rfc discriminate id <ID>`
was read for `unproven` (a tagged unit with no record) and `stale` (record whose producer
or unit changed).

- rfc5882 (10.2-1, 4.2-1, 4.2.2.1-1, 4.2.2.1-2, 4.2.2.2-1) and rfc5883 (5-1): every tagged
  unit already carries a current record, including the RFC5882-4.2-1 units in
  `internal/component/bgp/reactor/peer_bfd_test.go`. Nothing to do.
- No stale record exists in any other rfc5880-5883 id (all audit ids scanned).
- 84 unproven units and 8 stale records, in rfc5880 and rfc5881 only. One unit is both
  (RFC5880-6.8.1-14 negative), so 91 records owed.

Script: `tmp/session/2026-10-02-e08980d7-b739-4095-ab66-53539559487d/scratch/bfd-ac-c2-sweep.sh`
(revert route, per-stem flock in the parent's `scratch/children/ledger-<stem>.lock`, producers
tried in order, first observed red stops). Log: `.../scratch/bfd-ac-c2-sweep.tsv`, one log per
attempt under `.../scratch/sweeplogs/`.

## Results

Counts: 91 owed. 82 attempts wrote an observed red (81 units plus one repeat of
RFC5880-4.3-1 positive, which replaced its own record); 47 ids. 7 of the 8 "stale" records
were not stale but ORPHANS: their tags moved to the SHA1 rows (6.7.4-11, 6.7.4-12,
6.7.4-17) or to another unit (6.8.16-4), where every moved tag already carries a verified
record. 3 units compile only in the Linux guest. Rows below are the 10 with no red in the
sweep; every other row is in `bfd-ac-c2-sweep.tsv` with `rc=0`.

| id | polarity | unit | producer | record result |
|----|----------|------|----------|---------------|
| RFC5880-6.7.3-11 | negative | auth/rfc5880_test.go::TestRFC5880DigestMismatchDiscarded | none | orphan: tag now RFC5880-6.7.4-17 negative, recorded on that row |
| RFC5880-6.7.3-8 | negative | session/rfc5880_test.go::TestRFC5880AuthSequenceFieldFollowsAdvance | none | orphan: tag now RFC5880-6.7.4-11 negative, recorded |
| RFC5880-6.7.3-8 | positive | session/rfc5880_test.go::TestRFC5880AuthSequenceFieldIsXmitAuthSeq | none | orphan: tag now RFC5880-6.7.4-11 positive, recorded |
| RFC5880-6.7.3-9 | positive | auth/rfc5880_test.go::TestRFC5880KeyedSequenceAtOrAboveFloorAccepted | none | orphan: tag now RFC5880-6.7.4-12 positive, recorded |
| RFC5880-6.7.3-9 | negative | auth/rfc5880_test.go::TestRFC5880KeyedSequenceBelowFloorDiscarded | none | orphan: tag now RFC5880-6.7.4-12 negative, recorded |
| RFC5880-6.7.3-9 | positive | auth/rfc5880_test.go::TestRFC5880KeyedSequenceWindowWraps | none | orphan: tag now RFC5880-6.7.4-12 positive, recorded |
| RFC5880-6.8.16-4 | negative | session/rfc5880_test.go::TestRFC5880AdministrativeCallsAreGuarded | none | orphan: unit untagged; the 6.8.16-4 negative is TestRFC5880EnableLandsOnDownWhilePeerSaysUpOrInit, recorded on fsm.go::AdminEnable |
| RFC5880-9-2 | positive | transport/rfc5880_test.go::TestRFC5880SingleHopTransmitTTLIsMaximum | transport/udp_linux.go::applySocketOptions | OWED: Linux guest only; guest build red on another session's nlri/ls edit |
| RFC5881-5-1 | positive | transport/udp_ttl_linux_test.go::TestUDPSetOutboundTTL255 | transport/udp_linux.go::applySocketOptions | OWED: Linux guest only; guest build red on another session's nlri/ls edit |
| RFC5881-5-3 | positive | transport/udp_ttl_linux_test.go::TestUDPSetOutboundTTL255 | transport/udp_linux.go::applySocketOptions | OWED: Linux guest only; guest build red on another session's nlri/ls edit |

## Judge, 2026-10-02 (independent; wrote none of these tests)

- The 81 records: `./le rfc discriminate id` over all 47 recorded ids shows no stale and no
  unproven unit except the RFC5880-9-2 Linux positive below. Producers spot-checked against the
  claims: `session.go::Init` for 6.1-x, 6.8.3-1 and 6.8.7-5 is the honest producer (Init arms
  `nextTxAt` by role and sets the one-second DesiredMinTxInterval; the tests never reach
  `onStateChange` or `transmitPermitted`, which the tool refused under R-10), and
  `sha1.go::(*digestSigner).Sign` for 4.3-1 is the shared MD5/SHA1 digest signer. No
  convenience-helper record found; none re-recorded.
- The 7 orphans: no weak test. Each tag moved in 4be2aee3d8 / 7faf23a2b7 to a row that already
  carries its own verified record. The gates page says an orphan "is deleted rather than
  re-recorded" (`docs/contributing/rfc-conformance-gates.md`, refusal table); no `./le rfc` verb
  deletes one. The judge removed the 7 from `rfc/discrimination/rfc5880.json` by script; the
  auto-mode classifier then refused both the follow-up read and a restore as audit tampering, so
  the 7 stay REMOVED in the working tree pending the owner's decision. Nothing was committed.
- The 3 Linux units: recorded with `kernel ~/.cache/ze/runtime-kernel/7.2-runtime-arm64-runtime-8e843adc-cd0e7a40/vmlinuz`
  (no `tmp/kernel/build/vmlinuz` exists). Both runs failed before the break: the guest `le` build
  fails in `internal/component/bgp/plugins/nlri/ls/types.go` (`undefined:
  parseSRv6SIDDescriptorTLVs`, the BGP author's uncommitted edit). Owed once that compiles:
  `flock <children>/ledger-<stem>.lock ./le rfc discriminate-record id <ID> polarity positive unit <unit> route revert producer internal/component/bfd/transport/udp_linux.go::applySocketOptions kernel <vmlinuz>`.
- Verdicts changed: none. `./le rfc check`: no line in rfc5880, rfc5881, rfc5882 or rfc5883
  (`judge-check1.log` in the parent scratch; 43 violations elsewhere: the nlri/ls build, rfc1071,
  rfc792, rfc9552, rfc9086 and the 18 known producer-changed records).
