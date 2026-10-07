# VPP Firewall Backend

## Meta

| Field | Value |
|-------|-------|
| Name | VPP Firewall Backend |
| Kind | daemon |
| Scope | complete |
| Level | stub-backed |
| Components | internal/plugins/firewall/vpp |
| Real-path tests | internal/plugins/firewall/vpp/acl_restart_linux_test.go::TestACLRestartAdoptsAndUpdatesSingleton, internal/plugins/firewall/vpp/acl_restart_linux_test.go::TestACLProgrammingRollbackFailureRemainsOwned |
| Docs | docs/guide/vpp.md |
| Doc review | 2026-10-07: every source anchor resolves; the verifier and backend registrations are in register.go and verify.go |
| Defect review | 2026-10-07: journal bound-wraps-before-it-refuses.md row names the VPP ACL path; journal rows naming a Component, not each re-verified here: gate-excludes-part-of-its-population.md:138, guard-added-to-one-half-of-a-pair.md:28, guard-added-to-one-half-of-a-pair.md:90, unwired-feature.md:70 |
| Stub evidence | internal/plugins/firewall/vpp/acl_restart_linux_test.go::TestACLRestartAdoptsAndUpdatesSingleton, internal/plugins/firewall/vpp/acl_restart_linux_test.go::TestACLProgrammingRollbackFailureRemainsOwned |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Registered as `firewall { backend vpp }`. Filter chains translate ze Match/Action types to VPP ACL rules via GoVPP binapi (source/destination prefix, port range, protocol, ICMP type/code, TCP flags, permit/deny/reflect). Connection-state `established,related` maps to `ACL_ACTION_API_PERMIT_REFLECT` (VPP reflexive ACL). NAT chains configure VPP NAT44-ED: masquerade via output-interface mode, SNAT via address pool + inside interface feature, DNAT via static mappings with tagged cleanup. SetMark and Limit actions use VPP's classify pipeline: classify tables match traffic by packet header fields, SetMark sets opaque metadata via `CLASSIFY_API_ACTION_SET_METADATA`, Limit creates a policer bound to the classify table via `PolicerClassifySetInterface`. Expression types without a faithful VPP representation are rejected at commit via `firewall.RegisterVerifier("vpp", Verify)`: interface matches, connection marks, DSCP, sets, packet modification (connmark/dscp/tcp-mss), counters, log, chain traversal. <!-- source: internal/plugins/firewall/vpp/verify.go -- per-expression rejection matrix --> <!-- source: internal/plugins/firewall/vpp/backend_linux.go -- Apply with read-merge-write binding --> <!-- source: internal/plugins/firewall/vpp/translate.go -- ze types to VPP ACLRule translation --> <!-- source: internal/plugins/firewall/vpp/nat_linux.go -- NAT44-ED integration (masquerade, SNAT, DNAT) --> <!-- source: internal/plugins/firewall/vpp/classify_linux.go -- classify pipeline (SetMark, Limit+policer) -->
