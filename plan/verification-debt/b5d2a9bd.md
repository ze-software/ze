# Verification debt -- commit session b5d2a9bd

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-08 | b5d2a9bd | feat(eap): EAP-TLS 1.3 issues a ticket the next exchange redeems | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:25:15Z) | open |
| 2026-09-08 | b5d2a9bd | feat(eap): EAP-TLS 1.3 issues a ticket the next exchange redeems | native structural checks (red) | verify lint/run is red across the tree and no finding belongs to this change. The run reports 71 issues over roughly 57 files, led by 19 misspell and 12 goconst, in packages this commit does not touch (bgp/plugins, le/verify, test/fixture, plugins/fib, plugins/iface). Three land in files this commit edits and all three sit at lines it does not: responder_eap.go:336 errorlint, whose hunks are at 92 and 303; rekey.go:543 unconvert, whose hunks are at 995 and 1142; and sa.go:205 modernize atomictypes, whose hunk is at 289 and whose flagged line carries a standing comment explaining that atomic.Int64 cannot be used because a test copies the struct. child_test.go:109 unused is HEAD's, from 16e0d9522, in a file this commit does not carry. Zero findings name resumption.go, peer_chain.go, eap_tls.go, peer.go or any new test file. Repairing 57 unrelated files would be the gate becoming the session, which ai/rules/pre-release.md forbids. | open |
| 2026-09-08 | b5d2a9bd | feat(eap): EAP-TLS 1.3 issues a ticket the next exchange redeems | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-08 | b5d2a9bd | feat(eap): EAP-TLS 1.3 issues a ticket the next exchange redeems | discovery-index freshness | Ran ./le discovery-index update for this population: it rewrote ai/PACKAGE-MAP.md to byte-identical content (765 packages, no diff against HEAD). The five new files join existing packages -- internal/core/eap, internal/component/ike/engine and internal/component/ike/ipsec -- so no package was created and the index has nothing to record. | open |
