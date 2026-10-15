# Verification debt -- commit session b2ec5abb

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-15 | b2ec5abb | test(exabgp): api-vpnv4 withdraw carries no label, per RFC 8277 2.4 (+14 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-15T19:20:52Z) | open |
| 2026-10-15 | b2ec5abb | test(exabgp): api-vpnv4 withdraw carries no label, per RFC 8277 2.4 (+1 more) | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, fix in progress); iface-resolution reruns OK now; this commit touches only a .ci and a doc | open |
| 2026-10-15 | b2ec5abb | fix(interop): BFD two-link prestarts are compiled Go, not shell files | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, fix in progress); this commit touches only interop harness code and docs | open |
| 2026-10-15 | b2ec5abb | fix(interop): BFD two-link prestarts are compiled Go, not shell files (+4 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-15 | b2ec5abb | test(rs): worker-queue-size tests expect the applied log line (+4 more) | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, fix in progress); these commits touch only tests | open |
| 2026-10-15 | b2ec5abb | journal(full-disk): TMPDIR leftovers of killed runs have no reaper | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, fix in progress); this commit is a journal row | open |
| 2026-10-15 | b2ec5abb | test(le): stand-in trees carry the stdpatch set le derives from | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, fix in progress); these commits touch only test fixtures and a journal row | open |
| 2026-10-15 | b2ec5abb | docs: DNS harness anchor and retired ze-peer name in five specs | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, owner decision pending); this commit is docs and spec text | open |
| 2026-10-15 | b2ec5abb | rfc: reseal, re-record and rejudge DNS and MUP records after rework | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (35 cross-plugin imports, owner decision pending); this commit is RFC records | open |
| 2026-10-15 | b2ec5abb | rfc(2545): re-record next-hop proofs against the FRR interop scenario | native structural checks (red) | arch tier/check is red at HEAD since 9f316482c1 (cross-plugin imports, fix in progress); this commit is RFC records | open |
| 2026-10-15 | b2ec5abb | plan: spec config-yang-compiled-at-build-time (design) | native structural checks (red) | arch tier/check is red at HEAD (cross-plugin imports, spec-plugin-ownership-imports in progress); this commit is a spec | open |
| 2026-10-15 | b2ec5abb | docs(plugin): external plugins cannot extend the config schema yet | native structural checks (red) | arch tier/check is red at HEAD (cross-plugin imports, spec-plugin-ownership-imports in progress); this commit is docs | open |
