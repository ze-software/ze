# Verification debt -- commit session 3f5fcb0a

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | 3f5fcb0a | rfc(2545): re-record RFC2545-3-1 and 3-2 after buildRelayUpdate moved (+6 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-17T10:46:25Z) | open |
| 2026-10-18 | 3f5fcb0a | rfc(2545): re-record RFC2545-3-1 and 3-2 after buildRelayUpdate moved (+2 more) | native structural checks (red) | recorded reds come from a verify that predates the 2026-10-18 rebase; ./le arch tier check and ./le arch iface-resolution both pass over this tree now | open |
| 2026-10-18 | 3f5fcb0a | test(reactor): drop the RFC naming marker from decoders_test.go (+50 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-18 | 3f5fcb0a | test(bgp): RFC 9494 4.3 units pin the advertised trigger and cache rail (+3 more) | native structural checks (red) | recorded reds come from a verify that predates the 2026-10-18 rebase; ./le arch tier check and ./le arch iface-resolution both pass over this tree; a full verify is in progress | open |
| 2026-10-18 | 3f5fcb0a | journal: cache disk full during the 2026-10-18 full verify (+32 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-10-18T13:57:23Z) | open |
| 2026-10-18 | 3f5fcb0a | journal: cache disk full during the 2026-10-18 full verify | native structural checks (red) | recorded reds come from a verify that predates the 2026-10-18 rebase; a full verify is in progress | open |
| 2026-10-18 | 3f5fcb0a | fix(exabgp): bridge writes the selector into clear adj-rib (+90 more) | native structural checks (red) | arch tier/check and iface-resolution reds are stale; both gates pass when run now | open |
| 2026-10-18 | 3f5fcb0a | feat(aaa): name the login channel to RADIUS and TACACS+ | owner approval for an RFC-tagged test change | owner 2026-10-18: implement RFC3579-3-2 and tag the aaa-radius-admin functional proof | open |
| 2026-10-18 | 3f5fcb0a | feat(vrrp): RFC 9568 IPv4 checksum by default, keepalived opt-in | owner approval for an RFC-tagged test change | owner ordered rows 16 and 17 done on 2026-10-18 (plan/rfc-gap-difficulty-2026-10-18.md, Owner decisions); the IPv4 goldens follow the new default checksum form | open |
| 2026-10-18 | 3f5fcb0a | bgp: announce a session's down after its buffered UPDATEs (+58 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-18T17:16:37Z) | open |
| 2026-10-18 | 3f5fcb0a | fix(config/yang): linear reference walk, defaults in use, key prefixes (+1 more) | owner approval for an RFC-tagged test change | owner decision implement on the bccd1e7cf5 review: the refusal tests assert their own message, validateYangType callers move to ValidateType | open |
| 2026-10-18 | 3f5fcb0a | feat(ipsec): unmatched defaults to discard, RFC 4301 4.4.1 | owner approval for an RFC-tagged test change | owner decision 2026-10-18 (plan/rfc-gap-difficulty-2026-10-18.md, #2: 'Follow the RFC: R27 reversed, unmatched defaults to discard') flips the default these two tagged tests pin; explicit approval of the test edits owed | open |
| 2026-10-18 | 3f5fcb0a | ipsec: discard-default follow-up, ICMP and ND named, 4.4.1-3 negative | owner approval for an RFC-tagged test change | owner 2026-10-18: follow RFC 4301 4.4.1 (#2); test added/adjusted for the discard default | open |
| 2026-10-18 | 3f5fcb0a | bgp: decide each owed key on its own record, delete the sequence fence | owner approval for an RFC-tagged test change | owner 2026-10-18 ordered spec-bgp-rs-replacement-on-withdrawal (Q-4 pre-approved its tag changes); guard migrated per the spec's Guards of deleted code table | open |
| 2026-10-18 | 3f5fcb0a | test(dhcp): option 52=2 leaves file unread; tag options-first | owner approval for an RFC-tagged test change | owner 2026-10-18: implement RFC2131-4.1-7/4.1-15; tags added for the auditor's missing negative | open |
| 2026-10-18 | 3f5fcb0a | bgp: delete the departure cut end to end | owner approval for an RFC-tagged test change | owner 2026-10-18 ordered spec-bgp-rs-replacement-on-withdrawal (Q-4 pre-approved its tag changes) | open |
