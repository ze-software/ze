# Handover: lint follow-ups of the capability work

Written 2026-11-23 on the server (`claude`), main checkout, by the session that
closed handover 57. Thomas ordered: fix the other lint issues, improve the
IPsec messages. Landed so far: 4862cfa6ec (lint in L2TP, sysctl,
traffic-usage) and 71402a6126 (a refused IKE socket bypass names its own
cause). The rest is below.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Lint half B: 36 findings in `internal/component/bgp/reactor`, `internal/component/plugin/server`, `pkg/plugin/sdk`, plus 11 more in reactor and the dead `rsRelaySendGranted` | Written, uncommitted, in the working tree. Tests of every changed reactor test pass; plugin server and SDK packages pass and lint clean. An independent judge approved every test change; the RFC approvals for `reactor.TestRFC4724EstablishedGRCollisionClosesOldWithoutNotification`, `reactor.TestRFC7947ClientDeniedUpdateRetainedInAdjRIBIn` and `reactor.TestRFC7947OrdinaryPeerDeniedUpdateAbsentFromAdjRIBIn` sit in `tmp/commit-rfc-approved-e72405d7.md` (session namespace e72405d7) | Re-take the 6 stale records (`./le commit create` prints each `./le rfc record` line when it refuses), run `./le go lint run scope "./internal/component/bgp/reactor/..."`, then commit. `peer_forward_facts.go` carries another session's hunk at `precomputeRedirectNextHop`: commit that file by hunks |
| 2 | `compileRFC8893ExportChain` (`internal/component/bgp/reactor/rfc8893_egress_validation_test.go`) takes `final`, always `policy.FinalAccept` (unparam) | Not done: its 22 callers are in all 8 RFC-tagged tests of the file | Drop the parameter; judge the change; re-take the 8 records under `rfc/discrimination/rfc8893/` |
| 3 | `policyview.Session` (288 bytes) passed by value per route (gocritic hugeParam) in `filter_library.go` (4), `filter_ordered.go` (2), `policy_dryrun.go`, `policy_dryrun_kind.go` | Not done: about 70 call sites, many passing non-addressable values, RFC-tagged callers in six test files. Nothing in the chain mutates the session, so a pointer is alias-safe | Prove with `-gcflags=-m` that a pointer does not move the value to the heap on the per-route path, then convert and re-take the stale records; if it escapes, keep the value and annotate with that evidence |
| 4 | IPsec messages with the same unclassified cause | Listed, not fixed: `engine/unmatched.go` `catchAllInstallHint` and `unmatchedStartRefusal` (netlink errnos), `engine/unmatched_xfrm_interface.go` (412-414, 445, 478, 502), `engine/source_guard.go` (259-261, 414), `engine/application_override.go` 120-130 (the commit refusal lists every cause while `overrideGuardFix(err)` beside it classifies), `cmd/show_dataplane.go` 69-70 (EACCES read as CAP_NET_ADMIN) | Classify each by its producer's errno the way `transport.bypassFaultOf` does, with a test per class and membership |

## Blocked when written

Another session's restore (`backups/work-20261123-040348-agent-handmerge.patch`,
`...-040358-agent-wholefile-edits.patch`) left conflict markers (`<<<<<<< HEAD`
/ `>>>>>>> restored`) in eight working-tree files: `internal/component/config/{loader,parser,parser_list,setparser}.go`,
`internal/component/config/cli/validate_corpus_test.go`,
`internal/component/ike/engine/unmatched.go`, `internal/component/policy/register.go`,
`internal/plugins/redistribute/redistribute.go`. Until that session resolves
them nothing that imports `internal/component/config` builds in the working
tree, so items 1 to 4 cannot be tested. They are that session's files: do not
resolve them.

## Known reds outside this work

The full `./internal/component/bgp/reactor/` run has 19 reds (RFC 2545
neighbour re-advertisement, MRAI candidates, RFC 9012 storage, batch
cancellation, paths-limit election, stale rail) in areas where seven reactor
production files carry other sessions' uncommitted edits.
