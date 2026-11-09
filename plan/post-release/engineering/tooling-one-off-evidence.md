# Tooling one-off defects: recovered historical evidence

Companion to [the design candidate](spec-tooling-one-off-defects.md).
These are historical diagnostics, not current lint results, executed repair
proof, or evidence that contention caused a failure. Counts measure the retained
selection only. Described commands are historical producers, not rerun instructions.
No build or test ran during this recovery.

## Transcript provenance

Source directory:
`/home/thomas/.claude/projects/-home-thomas-Code-github-com-ze-software-ze-main/ed626c22-d6a4-4293-a5f7-287a2deb8cf2/subagents/`.
Record numbers identify JSONL records. Producer matching uses the earliest
timestamped assistant call with the result's tool-use ID, not duplicate progress
records. Only diagnostic and failure payloads are retained. Conversation text,
unrelated listings and private context are omitted.

| Key | Incident | Transcript | SHA256 |
|-----|----------|------------|--------|
| A | `0723d7d26a` | `agent-aaa7b7fb8c8f1d346.jsonl` | `0036a334bfbaa346737e9d75c97cd6476ff9121e326b833f565303ba45d7f1e7` |
| B | `e6474aa200` | `agent-a44298c6a9a5c4f6e.jsonl` | `f494c2931f4324a96574dd38ee98c01e62c7ed28dcd9b2fec1ca19a6a3b30400` |
| C | `2f36373081` | `agent-a682d689fbed3ad75.jsonl` | `037d80489b23a07a122ab07628d05c68dd801ff0891a6a58b0109e2b607345d4` |
| D | `043c682d24` | `agent-a4ebbbfe3603ca451.jsonl` | `3236fd28a527c5a8cd005bac30c8a02b9320dff96989871afc179194daecc85b` |

Original log roots, relative to `tmp/verify-worktree-logs/`:

| Key | Root |
|-----|------|
| A | `20261027T022750.337309046Z-p800009-r1-0723d7d26a13/changed-1676726763` |
| B | `20261027T064148.663533702Z-p2790354-r1-e6474aa20051/changed-1333725753` |
| C | `20261027T121702.432354012Z-p871905-r1-2f36373081fd/changed-4011048196` |
| D | `20261027T162248.310939435Z-p3115964-r1-043c682d241d/changed-1950980948` |

These paths identify inputs, not presently available logs. Raw contexts, flavor
boundaries, omitted diagnostics and uncaptured cases remain unavailable.

## Bounded October naming reconstruction

This is a reconstruction, not the recovered
`job-enum-post-framing-canonical-rfc-check-f8aecd88.log` or its indexed final-proof
archive. Those originals remain missing. E reports applying the historical
`1720f2b6cd` naming semantics to 4995 committed eligible files and four documented
probes: seven committed findings plus two probe findings; the other two probes
pass naming. That census was not rerun here and remains audit-reported.

Direct inspection in this recovery read historical `judgeTestFileName`,
`CarrierFor` and `scanGoTags` in `internal/le/rfc/`, the marker/rename diffs below,
and archive headers and ancestry. These support the bounded reconstruction, not
authenticity of the missing run or proof that any runtime defect was repaired.

| Historical carrier | Disposition / evidence boundary |
|--------------------|---------------------------------|
| BGP CLI `rfc8277_vpn_consumer_test.go` | Naming marker at `d3e5b4a770`; diff inspected |
| FSM `rfc4271_start_ignore_test.go` | Naming marker at `d3e5b4a770`; diff inspected |
| RIB `rfc8277_json_withdraw_test.go` | Naming marker at `d3e5b4a770`; diff inspected |
| Reactor `flowspec_forward_mixed_test.go` | Rename at `a184a563d1`; diff inspected; stored RFC8955-4-3 negative discrimination names renamed carrier, inspected as stored evidence only |
| PPP `rfc1661_transition_cells_test.go` | Naming marker at `39798d536a`; diff inspected |
| IPC `rpc_decode_nlri_context_test.go` | Rename at `d3e5b4a770`; diff inspected; stored RFC8277-2.4-1 positive/negative records name renamed carrier, inspected as stored evidence only |
| OSPF `rfc5250_dd_capability_test.go` | Naming marker at `875eaecb9b`; diff inspected |
| EAP `rfc5216_resumption_defect_test.go` probe | Marker at `f201867a36`; deletion at `37dbe54095` inspected. Replacement implementation/proof at that commit is E-reported, not inspected here |
| RS `rfc4271_replacement_red_test.go` probe | Preserved red probe; runtime obligation remains separately owned by `plan/release/fixes/spec-bgp-one-off-defects.md`. Naming disposition cannot close it |

The archive is `plan/handover/rfc-verdict-test-fix-pass/data/`.
RS archive commit `a9ddcd495ec822e6d252e07dbad984730ca82b12`
(`2026-10-01T10:13:58+01:00`) and EAP/two-YANG archive commit
`0f7c52a0150b99bcf0b5b0e6a89152707d64ee3d`
(`2026-10-02T14:29:06+01:00`) were directly confirmed ancestors of `1720f2b6cd`.
The RS header calls the probe untagged and red on purpose. EAP and the two YANG
headers identify copies of untracked red tests and their owning specs. The two
YANG probes are `loader_rfc7950_structural_red_test.go.txt` and
`validator_mandatory_rfc7950_red_test.go.txt`. E's reported EAP/YANG byte equality
after header removal and RS October 6 transcript equality were not independently
checked here. Archives predating the incident are bounded inputs, not the
incident's authentic final inventory.

For the two June backfill obligations, directly read history shows
`361e7c0b8f` adds one generic user-reported pattern and `2cff2050ac` creates two
journal rows without exact inventories. This establishes neither two distinct
original incidents nor completed backfills. Both still need the original June
authoring/August migration conversation or owner-held technique, predicate and
inventory evidence; current naming repairs cannot supply that census.

## Coordinate lint excerpts

A producer 89 at `2026-10-27T05:48:14.033Z` selected coordinate diagnostics
from `11-go-lint-run-changed.log`, cut to 260 characters, then sorted uniquely.
Result 93 at `2026-10-27T05:48:16.740Z` retained these 11 lines. Typecheck
headers were excluded by the coordinate pattern. Result 100 at
`2026-10-27T05:48:22.475Z`, from producer 96 at `05:48:19.777Z`, separately
showed `internal/component/ike/engine/apply.go:1: : #` typecheck headers.

```text
internal/appliance/cache_download_test.go:16:29: `Cancelling` is a misspelling of `Canceling` (misspell)
internal/component/gnmi/get_test.go:166:6: stringXbytes: suggestion: !bytes.Equal(val.GetJsonVal(), ietf.GetJsonIetfVal()) (gocritic)
internal/component/ike/engine/ike_auth_algorithms_test.go:207:91: undefined: ikecrypto.ENCR_CHACHA20_POLY1305
internal/component/ike/engine/ike_auth_algorithms_test.go:221:58: undefined: ikecrypto.ENCR_CHACHA20_POLY1305 (typecheck)
internal/component/l2tp/subsystem.go:95:1: typeDefFirst: definition of type 'Subsystem' should appear before its methods (gocritic)
internal/le/test/deployment/actions.go:34:38: string `run` has 11 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go:106:12: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go:66:47: string `run` has 16 occurrences, make it a constant (goconst)
internal/le/test/qemu/alltests.go:228:9: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/install.go:116:10: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/stress.go:97:12: string `run` has 16 occurrences, make it a constant (goconst)
```

B producers 252 at `2026-10-27T10:16:59.768Z` and 260 at
`2026-10-27T10:17:07.065Z` selected `^[a-z].*\.go:[0-9]+:[0-9]+: ` from
`09-go-lint-run-changed.log`, cut to 200 characters, sorted and deduplicated.
Results 256 at `2026-10-27T10:17:03.231Z` and 264 at
`2026-10-27T10:17:10.101Z` displayed the first 60 and positions 61–200.
The union below contains 184 coordinate lines, or 153 after coordinate removal
and deduplication. Neither number certifies the historical estimate of 138 or
the complete 1272-line log. The `orderedTransportChild` suffix was cut and is
not reconstructed. Flavor boundaries and source context remain missing.

```text
internal/appliance/cache_download_test.go:16:29: `Cancelling` is a misspelling of `Canceling` (misspell)
internal/component/bgp/plugins/nlri/mvpn/types.go:192:14: newMVPN - routeType always receives MVPNIntraASIPMSIAD (1) (unparam)
internal/component/bgp/plugins/nlri/vpn/types.go:128:32: parseVPNAnnounce - safi always receives SAFIVPN (128) (unparam)
internal/component/bgp/plugins/rib/rib_pipeline_bestfact_test.go:113:4: badLock: defer is missing, mutex is unlocked immediately (gocritic)
internal/component/bgp/reactor/delay_open_regression_test.go:38:10: Error return value is not checked (errcheck)
internal/component/bgp/reactor/delay_open_test.go:53:26: Error return value is not checked (errcheck)
internal/component/cli/editor_commit.go:740:2: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:102:3: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:163:2: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:184:6: slicesbackward: backward loop over slice can be modernized using slices.Backward (modernize)
internal/component/cli/editor_create_entry.go:250:2: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:287:3: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:293:2: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:370:2: rangeValCopy: each iteration copies 168 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:42:2: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:528:2: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:531:2: rangeValCopy: each iteration copies 168 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:56:3: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:588:3: return both a `nil` error and an invalid value: use a sentinel error instead (nilnil)
internal/component/cli/editor_create_entry.go:616:3: rangeValCopy: each iteration copies 168 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:68:4: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go:732:3: ruleguard: negated condition with an else -- swap the branches and test changeUser(e.originalPath, path) == replacedUser (gocritic)
internal/component/cli/editor_create_entry.go:849:3: ifElseChain: rewrite if-else to switch statement (gocritic)
internal/component/cli/editor_create_entry_review_test.go:297:5: ruleguard: negated condition with an else -- swap the branches and test content == "" (gocritic)
internal/component/config/meta.go:440:18: appendAssign: append result not assigned to the same slice (gocritic)
internal/component/config/meta.go:442:18: appendAssign: append result not assigned to the same slice (gocritic)
internal/component/gnmi/get_test.go:166:6: stringXbytes: suggestion: !bytes.Equal(val.GetJsonVal(), ietf.GetJsonIetfVal()) (gocritic)
internal/component/ike/crypto/esp_fixed_test.go:108:5: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/engine/child.go:214:2: atomictypes: var policyRestoreFailed uint32 may be simplified using atomic.Uint32 (modernize)
internal/component/ike/engine/child.go:680:6: slicesbackward: backward loop over slice can be modernized using slices.Backward (modernize)
internal/component/ike/engine/esp_fixed_test.go:103:5: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/engine/esp_fixed_test.go:137:5: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/engine/esp_fixed_test.go:142:5: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/engine/esp_fixed_test.go:89:5: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/engine/esp_fixed_test.go:94:5: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/engine/rfc3948_inner_selector_ordered_transport_test.go:190:63: orderedTransportChild - result 0 (*github.com/ze-software/ze/internal/component/ike/engine.ChildSA) is never used
internal/component/ike/engine/rfc3948_inner_selector_ordered_transport_test.go:25:81: orderedTransportPeers - priorityB always receives 1000 (unparam)
internal/component/ike/engine/rfc3948_inner_selector_transport_test.go:45:51: mustNamePeersAndFix - tunnelPeer always receives "a" (unparam)
internal/component/ike/ipsec/algorithm_support_test.go:246:11: Error return value is not checked (errcheck)
internal/component/l2tp/subsystem.go:95:1: typeDefFirst: definition of type 'Subsystem' should appear before its methods (gocritic)
internal/component/plugin/server/poststartup_report_test.go:16:34: `cancelled` is a misspelling of `canceled` (misspell)
internal/le/ai/rules/actions.go:108:42: string `report` has 4 occurrences, make it a constant (goconst)
internal/le/commit/owed_consumers_test.go:117:1: File is not properly formatted (gofmt)
internal/le/doc/yangcontract/actions_test.go:21:1: File is not properly formatted (gofmt)
internal/le/hookruntime/postbash.go:66:23: stringsseq: Ranging over FieldsSeq is more efficient (modernize)
internal/le/interoplab/l2tp/checkers.go:59:74: string `session IP assigned` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/l2tp/l2tp.go:272:17: string `lac` has 4 occurrences, make it a constant (goconst)
internal/le/interoplab/l2tp/mandatory_avp.go:120:25: string `lac` has 4 occurrences, make it a constant (goconst)
internal/le/interoplab/l2tp/mandatory_avp.go:130:40: mandatoryAVPRefusal - result 0 (uint16) is never used (unparam)
internal/le/interoplab/l2tp/mandatory_avp.go:50:35: string `session IP assigned` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/l2tp/mandatory_avp_test.go:20:2: appendCombine: can combine chain of 2 appends into one (gocritic)
internal/le/interoplab/pppoe/check_ac.go:124:26: string `eth0` has 3 occurrences, but such constant `replayInterface` already exists (goconst)
internal/le/interoplab/pppoe/check_ac.go:248:21: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_helpers.go:56:59: string `link` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_helpers.go:85:45: string `dev` has 5 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go:169:18: string `add` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go:170:25: string `dev` has 5 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go:171:33: string `eth0` has 3 occurrences, but such constant `replayInterface` already exists (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go:182:65: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go:281:17: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_lifecycle.go:59:62: string `/proc/net/pppoe` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle_wire.go:256:12: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_lifecycle_wire.go:259:12: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_lifecycle_wire.go:262:12: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_lifecycle_wire.go:265:12: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_padr_replay.go:242:64: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/job/contention_test.go:896:37: age - by always receives 2 * StallDefault (3600000000000) (unparam)
internal/le/repo/changed/actions.go:21:12: string `list` has 3 occurrences, make it a constant (goconst)
internal/le/repo/rewrite/actions.go:20:15: string `rewrite` has 3 occurrences, make it a constant (goconst)
internal/le/rfc/check_status.go:439:33: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/lowerlayer_corpus_test.go:85:58: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go:330:7: hugeParam: m is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go:350:7: hugeParam: m is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go:353:7: hugeParam: m is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go:363:7: hugeParam: m is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go:832:33: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/priority.go:182:2: ifElseChain: rewrite if-else to switch statement (gocritic)
internal/le/rfc/profilecoverage_test.go:142:24: embedlit: embedded field type can be removed from struct literal (modernize)
internal/le/rfc/provenshare.go:119:17: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/session/actions.go:20:15: string `ensure` has 3 occurrences, make it a constant (goconst)
internal/le/setup/actions.go:71:12: string `install` has 3 occurrences, but such constant `installVerb` already exists (goconst)
internal/le/site/rfcprofiles_test.go:32:28: embedlit: embedded field type can be removed from struct literal (modernize)
internal/le/site/terminaldemo/actions.go:81:25: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/image_pins.go:18:42: regexpSimplify: can re-write `^[0-9]+\.[0-9]+\.[0-9]+$` as `^\d+\.\d+\.\d+$` (gocritic)
internal/le/site/terminaldemo/image_pins.go:32:15: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/manifest.go:249:36: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/manifest.go:251:56: string `.png` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go:176:8: string `--timeout` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go:293:147: string `Source` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go:46:2: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/readme_carrier.go:133:43: string `--user` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/readme_carrier.go:191:17: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/readme_select.go:210:16: string `.png` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/render.go:161:22: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/render.go:661:18: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go:151:7: string `cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go:152:10: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go:156:10: string `.webm` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go:158:10: string `.png` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go:38:3: rangeValCopy: each iteration copies 216 bytes (consider pointers or indexing) (gocritic)
internal/le/site/terminaldemo/scenarios.go:466:185: string `203.0.113.1` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios.go:509:6: func terminatePID is unused (unused)
internal/le/site/terminaldemo/scenarios.go:524:6: func runBounded is unused (unused)
internal/le/site/terminaldemo/scenarios.go:535:5: var execCommandContext is unused (unused)
internal/le/site/terminaldemo/scenarios.go:539:6: type osExecCmd is unused (unused)
internal/le/site/terminaldemo/scenarios.go:543:21: func (*osExecCmd).CombinedOutput is unused (unused)
internal/le/site/terminaldemo/scenarios.go:545:6: func commandContext is unused (unused)
internal/le/site/terminaldemo/scenarios_routing.go:201:19: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_routing.go:234:47: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_routing.go:90:7: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_control.go:151:41: filepathJoin: "/proc" contains a path separator (gocritic)
internal/le/site/terminaldemo/scenarios_showcase_control.go:57:34: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_control.go:71:15: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/scenarios_showcase_control.go:90:14: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/scenarios_showcase_control.go:97:24: stringsseq: Ranging over FieldsSeq is more efficient (modernize)
internal/le/site/terminaldemo/scenarios_showcase_control.go:99:3: slicescontains: Loop can be simplified using slices.Contains (modernize)
internal/le/site/terminaldemo/scenarios_showcase.go:144:16: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_network.go:159:159: string `--max-time` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_network.go:68:16: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/terminaldemo_test.go:206:44: filepathJoin: "../../../.." contains a path separator (gocritic)
internal/le/site/terminaldemo/validate_policy_evidence.go:198:19: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_policy_evidence.go:254:46: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_policy_evidence.go:289:17: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/validate_runtime.go:191:45: string `--user` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go:157:51: string `bgp-rib` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go:41:4: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go:42:11: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase_topics.go:66:14: string `Source` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go:109:14: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go:131:23: string `172.29.0.2` has 6 occurrences, but such constant `showcaseFRRRouterID` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go:152:52: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_ipsec.go:244:75: string `172.30.0.1` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_ospf.go:33:57: string `172.29.0.2` has 6 occurrences, but such constant `showcaseFRRRouterID` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_ospf.go:43:14: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topics.go:57:19: string `--timeout` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_traffic.go:108:7: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_traffic.go:155:83: string `--max-time` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go:176:22: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go:58:24: string `203.0.113.1` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go:86:14: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/actions.go:34:38: string `run` has 11 occurrences, make it a constant (goconst)
internal/le/test/deployment/l2tp.go:233:30: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/l2tp.go:247:30: string `add` has 4 occurrences, but such constant `ipAdd` already exists (goconst)
internal/le/test/deployment/vppiface.go:186:30: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/vppifacescenarios.go:222:17: string `wireguard` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go:25:36: string `left` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go:28:11: string `wireguard` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go:29:27: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go:41:36: filepathJoin: "internal/le/test/deployment/testdata/Dockerfile.wireguard" contains a path separator (gocritic)
internal/le/test/deployment/wireguardproof.go:88:25: string `10.88.0.2` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go:92:19: string `add` has 4 occurrences, but such constant `ipAdd` already exists (goconst)
internal/le/test/qemu/actions.go:106:12: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go:66:47: string `run` has 16 occurrences, make it a constant (goconst)
internal/le/test/qemu/alltests.go:228:9: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/install.go:116:10: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/stress.go:97:12: string `run` has 16 occurrences, make it a constant (goconst)
internal/le/test/qemu/vpnproof.go:58:17: G304: Potential file inclusion via variable (gosec)
internal/le/test/qemu/vpnproof.go:63:20: G304: Potential file inclusion via variable (gosec)
internal/le/verify/actions.go:38:12: string `run` has 3 occurrences, make it a constant (goconst)
internal/le/verify/deps/actions.go:26:13: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/verify/deps/actions.go:40:13: string `run` has 3 occurrences, make it a constant (goconst)
internal/le/workflowcheck/workflowcheck_test.go:235:3: ruleguard: negated condition with an else -- swap the branches and test third == "" (gocritic)
internal/plugins/iface/dhcp/dhcp_linux.go:206:53: `cancelled` is a misspelling of `canceled` (misspell)
internal/plugins/iface/dhcp/dhcp_linux.go:49:13: `cancelled` is a misspelling of `canceled` (misspell)
internal/plugins/iface/dhcp/dhcp_v4_rig_linux_test.go:304:32: (*logCapture).countWith - level always receives slog.LevelWarn (4) (unparam)
internal/plugins/iface/netlink/manage_linux.go:662:5: errorsastype: errors.As can be simplified using AsType[netlink.LinkNotFoundError] (modernize)
internal/plugins/iface/vpp/health_test.go:54:23: net.Listen must not be called. use (*net.ListenConfig).Listen (noctx)
internal/plugins/iface/vpp/health_test.go:56:2: Error return value is not checked (errcheck)
internal/plugins/iface/vpp/health_test.go:71:23: net.Listen must not be called. use (*net.ListenConfig).Listen (noctx)
internal/plugins/ldp/rfc5036_init_unknown_tlv_test.go:38:6: errorsastype: errors.As can be simplified using AsType[*unknownTLVError] (modernize)
internal/plugins/log/cmd/handlers_test.go:339:2: Error return value is not checked (errcheck)
internal/plugins/log/cmd/handlers_test.go:355:14: Error return value is not checked (errcheck)
internal/plugins/log/cmd/handlers_test.go:384:12: Error return value is not checked (errcheck)
internal/plugins/log/cmd/handlers_test.go:388:12: Error return value is not checked (errcheck)
internal/plugins/log/cmd/handlers_test.go:399:14: Error return value is not checked (errcheck)
internal/plugins/log/cmd/handlers_test.go:400:3: Error return value is not checked (errcheck)
internal/plugins/trafficusage/attach_linux.go:33:12: string `GPL` has 3 occurrences, make it a constant (goconst)
internal/plugins/trafficusage/program_linux.go:90:19: string `GPL` has 3 occurrences, make it a constant (goconst)
internal/test/fixture/plugin_fixture_16_network.go:35:1: File is not properly formatted (gofmt)
internal/test/fixture/register_bgp_delay_open.go:124:80: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors (errorlint)
internal/test/fixture/register_bgp_delay_open.go:73:80: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors (errorlint)
internal/test/fixture/ui_fixture_display_fill_completion.go:324:25: displayFillOpenPTY - rows always receives 50 (unparam)
internal/test/fixture/ui_fixture_le_evidence_answers.go:223:28: runLEEvidenceAnswers$2 - status always receives "" (unparam)
internal/test/fixture/ui_fixture_le_rfc_answers.go:252:3: stringsbuilder: using string += string in a loop is inefficient (modernize)
```

## Functional failure excerpts

Only failure lines and relevant failure details are retained below. Source-log
line prefixes are retained where the original grep emitted them. Passing cases,
progress counters, listings and unrelated output are omitted. Repeated names in
different incidents remain separate observations. IDs are local to that run and
suite. These excerpts do not establish the cause of a failure.

### A functional

Producer 110 at `2026-10-27T05:48:37.241Z` → result 114 at `2026-10-27T05:48:40.232Z`: unique
`TEST FAILURE: ID name` selection from `12-test-functional-gating.log`.

```text
TEST FAILURE: 12 appliance-mpls-qemu
TEST FAILURE: 142 bgp-rr-rpki-pending-withdraw-relay
TEST FAILURE: 143 bgp-rr-rpki-revalidate-relay
TEST FAILURE: 166 bgp-rs-rpki-pending-withdraw-relay
TEST FAILURE: 167 bgp-rs-rpki-revalidate-relay
TEST FAILURE: 16 crash-capture-panic-harvest-qemu
TEST FAILURE: 17 verify-scope-freshness-scoped
TEST FAILURE: 206 le-binary-dispatches
TEST FAILURE: 208 le-checks-answers
TEST FAILURE: 20 bgp-bfd-shared-session-widens
TEST FAILURE: 210 le-completion-shells
TEST FAILURE: 213 le-discovery-answers
TEST FAILURE: 219 le-feature-answers
TEST FAILURE: 230 concurrent-config-commit
TEST FAILURE: 239 le-rules-answers
TEST FAILURE: 243 le-vendor-web-answers
TEST FAILURE: 299 web-only-interrupt-graceful
TEST FAILURE: 301 web-tool-decode
TEST FAILURE: 32 ipsec-unbound-inner-address-unique
TEST FAILURE: 41 api-peer-create-delete-rib
TEST FAILURE: 46 api-peer-save
TEST FAILURE: 516 login-on-tree
TEST FAILURE: 839 rpki-revalidate-late-sync
TEST FAILURE: 86 audit-config-commit
```

Producer 298 at `2026-10-27T05:52:40.251Z` → result 302 at
`2026-10-27T05:52:43.082Z`: first three TACACS matches cut200, selected web
context cut220/head14. Producer 310 at `05:52:53.619Z` → result 314 at
`05:52:56.880Z`: web IDs 3/47/69 cut200, then 12-line web and 10-line TACACS
contexts cut240. This recovers five web names, not the complete seven-case set.

```text
2695:16.5s    344/375  FAIL  344  tacacs-key-required
4805:102.9s   47/103  FAIL  47  test/web/interface-configured-display.wb
4806:102.9s   3/103  FAIL  3  test/web/admin-command-posts-argument.wb
4846:11.5s    69/103  FAIL  69  test/web/nav-telemetry.wb
90.2s    49/103  FAIL  49  test/web/lg-error-fragment.wb
✗ Failed to connect: No such file or directory (os error 2) (after 5 retries - daemon may be busy or unresponsive)
91.0s    48/103  FAIL  48  test/web/interface-mac-override.wb
```

### B functional

Producer 278 at `2026-10-27T10:17:25.171Z` → result 282 at
`2026-10-27T10:17:28.649Z`: failure grep excluding `send failed` and `level=`,
cut220/head40 plus tail30 cut220, from `12-test-functional-gating.log`.
The seven reload timeout IDs are 13, 19, 30, 41, 42, 46 and 104.

```text
1400:TEST FAILURE: 142 bgp-rr-rpki-pending-withdraw-relay
1505:TEST FAILURE: 143 bgp-rr-rpki-revalidate-relay
1612:TEST FAILURE: 166 bgp-rs-rpki-pending-withdraw-relay
1714:TEST FAILURE: 167 bgp-rs-rpki-revalidate-relay
1819:TEST FAILURE: 841 rpki-revalidate-late-sync
1902:TEST FAILURE: 863 session-editor-commit-confirmed-accept
1967:TEST FAILURE: 925 storage-bfd-restart
2832:TEST FAILURE: 3 bfd-pinned-profile-edit-up-live
2868:TEST FAILURE: 13 bgp-bfd-profile-edit-live
2994:TEST FAILURE: 19 bgp-bfd-removed-live
3121:TEST FAILURE: 20 bgp-bfd-shared-session-widens
3157:TEST FAILURE: 30 bgp-filter-irr-loaded-over-ssh
3273:TEST FAILURE: 41 bgp-rpki-added-live
3342:TEST FAILURE: 42 bgp-rpki-attach-live
3430:TEST FAILURE: 46 commit-verify-reject
3484:TEST FAILURE: 104 request-reload-promotes-operator-file
```

Producer 287 at `2026-10-27T10:17:33.313Z` → result 291 at
`2026-10-27T10:17:36.654Z`: selected failure lines, first 200 positions,
additional numeric exclusions, cut200/head60. Those numeric exclusions removed
web ID 3 too. The summary retains 22 web IDs but only 21 names. ID 3 remains
unidentified in this run: A's ID 3 is not evidence of B's name.

```text
4072:TEST FAILURE: 131 deprecated-bare-config
4105:TEST FAILURE: 206 le-binary-dispatches
4136:TEST FAILURE: 208 le-checks-answers
4167:TEST FAILURE: 213 le-discovery-answers
4198:TEST FAILURE: 219 le-feature-answers
4229:TEST FAILURE: 230 le-placement-answers
4260:TEST FAILURE: 239 le-rules-answers
4291:TEST FAILURE: 243 le-vendor-web-answers
5039:138.8s   28/103  FAIL  28  test/web/commit-smoke.wb
5041:139.9s   17/103  FAIL  17  test/web/cli-copy-rename-existing.wb
5042:139.9s   4/103  FAIL  4  test/web/admin-nav.wb
5043:139.9s   45/103  FAIL  45  test/web/fragment-list.wb
5061:63.2s    11/103  FAIL  11  test/web/chaos-stream-swaps.wb
5116:59.7s    47/103  FAIL  47  test/web/interface-configured-display.wb
5186:46.1s    81/103  FAIL  81  test/web/set-value-persists.wb
5219:1.3s     74/103  FAIL  74  test/web/scenario-interface-setup.wb
5221:1.8s     9/103  FAIL  9  test/web/bgp-summary-live.wb
5222:1.9s     88/103  FAIL  88  test/web/web-i18n-fr.wb
5224:1.0s     89/103  FAIL  89  test/web/web-mobile-layout.wb
5225:1.0s     87/103  FAIL  87  test/web/web-error-toast.wb
5226:977ms    85/103  FAIL  85  test/web/tool-bgp-decode.wb
5227:576ms    101/103  FAIL  101  test/web/workbench-shell.wb
5229:603ms    90/103  FAIL  90  test/web/web-on-tree.wb
5230:10.6s    84/103  FAIL  84  test/web/theme-toggle.wb
5232:299ms    75/103  FAIL  75  test/web/scenario-multi-peer.wb
5233:0ms      10/103  FAIL  10  test/web/chaos-error-fragment.wb
5234:360ms    43/103  FAIL  43  test/web/fragment-container.wb
5235:435ms    29/103  FAIL  29  test/web/config-download-edit-gated.wb
5236:396ms    18/103  FAIL  18  test/web/cli-edit-nav.wb
5237:fail  81/103  78.6%  603.8s  failed 22 [3, 4, 9, 10, 11, 17, 18, 28, 29, 43, 45, 47, 74, 75, 81, 84, 85, 87, 88, 89, 90, 101]
5835:TEST FAILURE: 17 verify-scope-freshness-scoped
```

### C functional

Producer 237 at `2026-10-27T14:52:31.020Z` → result 241 at
`2026-10-27T14:52:34.601Z`: `TEST FAILURE` grep cut120, selected source-log
ranges cut300, from `12-test-functional-gating.log`.

```text
1396:TEST FAILURE: 41 api-peer-create-delete-rib
1508:TEST FAILURE: 142 bgp-rr-rpki-pending-withdraw-relay
1613:TEST FAILURE: 143 bgp-rr-rpki-revalidate-relay
1720:TEST FAILURE: 166 bgp-rs-rpki-pending-withdraw-relay
1822:TEST FAILURE: 167 bgp-rs-rpki-revalidate-relay
1926:TEST FAILURE: 576 modify-oversize-suppress
2028:TEST FAILURE: 782 rfc9234-rs-client-learned-not-upward
2130:TEST FAILURE: 841 rpki-revalidate-late-sync
3411:TEST FAILURE: 206 le-binary-dispatches
3442:TEST FAILURE: 208 le-checks-answers
3473:TEST FAILURE: 213 le-discovery-answers
3504:TEST FAILURE: 219 le-feature-answers
3535:TEST FAILURE: 230 le-placement-answers
3566:TEST FAILURE: 239 le-rules-answers
3597:TEST FAILURE: 243 le-vendor-web-answers
3628:TEST FAILURE: 250 pipe-local-command
3978:TEST FAILURE: 3 client-backup-start
4861:TEST FAILURE: 17 verify-scope-freshness-scoped
  cmd seq=1 (le test local-data-coverage): foreground helper did not exit within 2m15s; a process the test does not wait for is cmd=background
  cmd seq=1 (le test fixture managed/client-backup-start): foreground helper did not exit within 10s; a process the test does not wait for is cmd=background
```

### D functional

Producer 166 at `2026-10-27T18:53:01.501Z` → result 174 at
`2026-10-27T18:53:07.210Z`: broad `FAIL|fail|timeout|TIME` grep,
cut250/head50, from `12-*.log`. Only six plugin cases were captured, not the
whole 4639-line log. Relevant observer details remain cut at the original limit.

```text
1407:TEST FAILURE: 41 api-peer-create-delete-rib
1519:TEST FAILURE: 289 debug-toggle
1553:TEST FAILURE: 618 peer-admin-shutdown
1636:TEST FAILURE: 639 plugin-command-document-too-wide
1670:TEST FAILURE: 721 rbac-web-config-deny
1705:TEST FAILURE: 985 update-text-sr-policy-addpath
1645:observer reported runtime failure: time=runtime level=ERROR msg="ZE-OBSERVER-FAIL: initialize isolated CLI state: exit=124 stdout= stderr=process timed out after 15 seconds" subsystem=test.observer
1679:observer reported runtime failure: time=runtime level=ERROR msg="ZE-OBSERVER-FAIL: web show page did not become available" subsystem=test.observer
1712:FAILED CHECK PEERS: stdin=peer
1725:failed: message mismatch
```

## Selected unit failure payloads

These are selected named failures and their relevant diagnostics, not complete
unit logs. Indentation and tab spacing in selected unit diagnostics were normalized
for readability; these selections are not byte-exact log copies. The normalized
old/new lint blocks below are byte-exact recovered files. All times in the table
are on 2026-10-27, UTC. Result timestamps are
transcript timestamps, not the tests' execution times.

| Key/result | Producer timestamp and selection | Result timestamp |
|------------|----------------------------------|------------------|
| A/40 | 36, `05:47:18.744Z`; failure/panic sort-unique/head60 per 08/09/10 log | `05:47:21.639Z` |
| A/67 | 63, `05:47:45.547Z`; selected tests plus six context lines from 08, cut300 | `05:47:48.235Z` |
| B/52 | 48, `10:12:24.193Z`; failure/panic sort-unique/head40 per 05/08/10/11 log | `10:12:27.295Z` |
| B/77 | 73, `10:12:52.101Z`; DelayOpen context from 11, cut300/head50 | `10:12:56.476Z` |
| B/153 | 149, `10:14:16.491Z`; selected tests from 11/08/10, eight lines, cut260 | `10:14:19.554Z` |
| B/187 | 183, `10:14:46.213Z`; selected CLI contexts from 11, cut200 and cut300/head | `10:14:49.365Z` |
| C/36 | 32, `14:49:04.436Z`; failure/panic grep from 08, head80 | `14:49:07.714Z` |
| C/54 | 50, `14:49:22.477Z`; selected ranges from 08, cut400 | `14:49:25.413Z` |
| D/36 | 32, `18:48:58.430Z`; failure/race/panic grep from 02, head40 | `18:49:01.647Z` |
| D/42 | 38, `18:49:03.778Z`; source-log range 940–1000 from 02 | `18:49:06.815Z` |
| D/67 | 63, `18:49:23.303Z`; failure/panic/timeout grep from 09, head60 | `18:49:26.093Z` |
| D/74 | 70, `18:49:30.288Z`; selected ranges from 09, some cut300 | `18:49:33.162Z` |

### A unit selection

From A/40 and A/67. The timeout is a package-budget failure, not proof of an
individual test's cause. The separate `TestLeafListSequencesAgree` lead remains
in the spec pending its complete trace.

```text
--- FAIL: TestAnswerForwardsWordsAndExitCode (0.00s)
--- FAIL: TestEditorManagerRenameListEntry (2.63s)
--- FAIL: TestEveryStoredDigestMatchesTheTreeItRatchets (0.00s)
--- FAIL: TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify (0.51s)
--- FAIL: TestPartialHealthExplainsScopedEvidence (0.03s)
--- FAIL: TestQEMUActionsIncludeTheHostRun (0.00s)
--- FAIL: TestQuoteBackfillRealCorpusHasNothingLeft (1.97s)
--- FAIL: TestRFC8221ESPEncryptionIsAuthenticated (0.00s)
--- FAIL: TestRootLaunchersArePOSIXShellWithoutPython (0.00s)
--- FAIL: TestSessionStartRendersNoDeferredArtifact (0.24s)
panic: test timed out after 20m0s
    run_test.go:36: action 0 = "vpp-hugepages", want "vpp-hugepages-test"
    run_test.go:36: action 1 = "mpls-boot", want "mpls-boot-test"
    run_test.go:36: action 5 = "installer", want "install-test"
    run_test.go:36: action 6 = "installer-iso", want "install-iso-test"
    run_test.go:36: action 7 = "installer-scenarios", want "install-scenarios-test"
    run_test.go:36: action 8 = "installer-ventoy", want "install-ventoy-test"
        root_launcher_test.go:642: le expands argv as $*, which re-splits an argument on whitespace
    lifecycle_test.go:310: the fixture cannot render plan/release/roadmap.md at all, so nothing above was a decision: roadmap revision unavailable: "HEAD"; fetch the requested history: git rev-parse: exit status 128: fatal: not a git repository (or any of the parent directories): .git
```

### B unit selection

From B/52, B/77 and B/153. B/187 additionally retained the CLI failure
message `one change entry per loaded leaf` and a truncated pending-change parse
error ending `unknown field: listen (did`. Those truncated values cannot be
reconstructed into full expected/actual state.

```text
--- FAIL: TestActionsCarryTheirContracts (0.00s)
    terminaldemo_test.go:44: actions = 5, want 4
--- FAIL: TestChildInstallRollbackFailureIsRecordedInTheSPDLedger (0.00s)
    spd_left_test.go:72: the failed rollback removal was not logged:
--- FAIL: TestNoDirectTimeCalls (0.05s)
    audit_test.go:53: reactor_connection.go contains direct time.Now() call — use clock.Clock instead
--- FAIL: TestSRv6SubscriptionWaitsForPublication (31.05s)
    register_test.go:161: withdrawal did not finish after publication
--- FAIL: TestDiscardDeletesNewlyAddedValue (0.24s)
--- FAIL: TestHomeHeroDerivesFromTheManifest (0.03s)
    home_test.go:800: the homepage template names demonstration ipsec literally
    home_test.go:800: the homepage template names demonstration showcase literally
--- FAIL: TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue (0.26s)
    --- FAIL: TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue/session_mode_records_read_back_with_the_same_keys (0.25s)
--- FAIL: TestRootLaunchersArePOSIXShellWithoutPython (0.00s)
--- FAIL: TestSessionLoadLargeInputBatched (0.64s)
--- FAIL: TestEveryActionWordIsAVerbOrADeclaringName (0.00s)
--- FAIL: TestNoNameEmbedsAVerbOrTest (0.00s)
--- FAIL: TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify (0.23s)
--- FAIL: TestTheRealCheckoutPassesAndWasRead (4.35s)
--- FAIL: TestNativeRecorderDrivesARealPTYAndWritesAsciicastV2 (20.39s)
    pty_test.go:116: record tape: timed out waiting for the shell to produce output
795:--- FAIL: TestRFC4271DelayOpenBothSidesHold (0.28s)
796-    delay_open_test.go:273: io: read/write on closed pipe
--- FAIL: TestAnswerForwardsWordsAndExitCode (0.01s)
--- FAIL: TestEditorManagerRenameListEntry (0.85s)
--- FAIL: TestEveryStoredDigestMatchesTheTreeItRatchets (0.00s)
--- FAIL: TestPartialHealthExplainsScopedEvidence (0.01s)
--- FAIL: TestQuoteBackfillRealCorpusHasNothingLeft (1.14s)
--- FAIL: TestSessionStartRendersNoDeferredArtifact (0.41s)
```

### C unit selection

From C/36 and C/54. The timing failure is retained as the test's historical
assertion, not a performance measurement or proof of load attribution.

```text
--- FAIL: TestInProcessSpeed (18.10s)
    runner_test.go:254:
         Error:       "18.09543116s" is not less than "15s"
         Messages:    in-process 30s scenario should complete in <15s
--- FAIL: TestRFC4271DelayOpenBothSidesHold (0.30s)
    delay_open_test.go:273: io: read/write on closed pipe
--- FAIL: TestRootLaunchersArePOSIXShellWithoutPython (0.00s)
5021:--- FAIL: TestEditorManagerRenameListEntry (2.65s)
5289:--- FAIL: TestEveryActionWordIsAVerbOrADeclaringName (0.00s)
5292:--- FAIL: TestNoNameEmbedsAVerbOrTest (0.00s)
5295:--- FAIL: TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify (0.19s)
5311:--- FAIL: TestEveryStoredDigestMatchesTheTreeItRatchets (0.00s)
5342:--- FAIL: TestTheRealCheckoutPassesAndWasRead (3.86s)
5441:--- FAIL: TestSessionStartRendersNoDeferredArtifact (0.27s)
5483:--- FAIL: TestNumberParseAllowlistMatchesThisTree (0.26s)
5597:--- FAIL: TestQuoteBackfillRealCorpusHasNothingLeft (1.17s)
5730:--- FAIL: TestAnswerForwardsWordsAndExitCode (0.00s)
5759:--- FAIL: TestPartialHealthExplainsScopedEvidence (0.01s)
--- FAIL: TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser (12.08s)
    build_tag_ssh_absent_test.go:282: no-SSH REST listener did not start:
```

### D unit selection

From D/36, D/42, D/67 and D/74. The BFD failure occurred in both race and
ordinary unit selections. Its read-loop diagnostic must not disappear behind
the GR survivor failure. This capture alone does not establish a data race.

```text
--- FAIL: TestBFDNegotiatedPublicationJoinsSessionEnd (5.12s)
    peer_run_test.go:880: read loop did not finish while publication was paused
1562:--- FAIL: TestBFDNegotiatedPublicationJoinsSessionEnd (5.05s)
--- FAIL: TestOwedKeyGRPurgeRecoversSurvivor (12.72s)
    --- FAIL: TestOwedKeyGRPurgeRecoversSurvivor/end-of-rib/no-survivor (0.11s)
        rs_withdrawal_convergence_test.go:1690: purge-stale purged 0 of A's routes, want 1
47:--- FAIL: TestRootLaunchersArePOSIXShellWithoutPython (0.01s)
732:--- FAIL: TestRFC4271DelayOpenBothSidesHold (0.28s)
4881:--- FAIL: TestSessionLoadLargeInputBatched (0.53s)
4888:--- FAIL: TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue (0.32s)
4889:    --- FAIL: TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue/session_mode_records_read_back_with_the_same_keys (0.31s)
4900:--- FAIL: TestDiscardDeletesNewlyAddedValue (0.42s)
4986:--- FAIL: TestEditorManagerRenameListEntry (2.64s)
5255:--- FAIL: TestEveryActionWordIsAVerbOrADeclaringName (0.00s)
5258:--- FAIL: TestNoNameEmbedsAVerbOrTest (0.00s)
5261:--- FAIL: TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify (0.18s)
5277:--- FAIL: TestEveryStoredDigestMatchesTheTreeItRatchets (0.00s)
5309:--- FAIL: TestTheRealCheckoutPassesAndWasRead (5.50s)
5409:--- FAIL: TestSessionStartRendersNoDeferredArtifact (0.23s)
5453:--- FAIL: TestNumberParseAllowlistMatchesThisTree (0.29s)
5567:--- FAIL: TestQuoteBackfillRealCorpusHasNothingLeft (1.78s)
5700:--- FAIL: TestAnswerForwardsWordsAndExitCode (0.00s)
5729:--- FAIL: TestPartialHealthExplainsScopedEvidence (0.01s)
```

## Complete retained normalized lint lists

D producer 213 (`2026-10-27T18:54:01.704Z`) read C's `*lint*.log` and D's
`11-*.log`. Result 217 (`2026-10-27T18:54:19.513Z`) confirmed the old root and
counts. It selected `^[a-zA-Z_./-]+\.go:[0-9]+:[0-9]+: .*`, removed line and
column fields, then sorted uniquely. That path alphabet excludes digits and
other characters. Typecheck headers without both coordinates and nonmatching
messages are excluded. Normalization loses occurrences and flavor identity.
Only the displayed `comm` difference was cut to 220 characters, not these files.

Recovered originals were under
`/home/thomas/.cache/tmp/claude-1000/-home-thomas-Code-github-com-ze-software-ze-main/ed626c22-d6a4-4293-a5f7-287a2deb8cf2/scratchpad/`.
The blocks retain all lines in original order, including their final newline.

### old.txt — 2f36373081

121 lines, 13718 bytes. SHA256:
`b134c08f0057345fc178f344e97cc50228657d5c33db177ec64518066b39ded2`.

```text
internal/appliance/cache_download_test.go: `Cancelling` is a misspelling of `Canceling` (misspell)
internal/component/bgp/reactor/delay_open_regression_test.go: Error return value is not checked (errcheck)
internal/component/bgp/reactor/delay_open_test.go: Error return value is not checked (errcheck)
internal/component/ike/engine/child.go: atomictypes: var policyRestoreFailed uint32 may be simplified using atomic.Uint32 (modernize)
internal/component/ike/engine/child.go: slicesbackward: backward loop over slice can be modernized using slices.Backward (modernize)
internal/component/ike/engine/esp_fixed_test.go: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/plugin/server/poststartup_report_test.go: `cancelled` is a misspelling of `canceled` (misspell)
internal/le/ai/rules/actions.go: string `report` has 4 occurrences, make it a constant (goconst)
internal/le/commit/owed_consumers_test.go: File is not properly formatted (gofmt)
internal/le/doc/yangcontract/actions_test.go: File is not properly formatted (gofmt)
internal/le/hookruntime/postbash.go: stringsseq: Ranging over FieldsSeq is more efficient (modernize)
internal/le/interoplab/lab.go: ruleguard: negated condition with an else -- swap the branches and test s.ImagesOnDemand (gocritic)
internal/le/interoplab/pppoe/check_ac.go: string `eth0` has 3 occurrences, but such constant `replayInterface` already exists (goconst)
internal/le/interoplab/pppoe/check_ac.go: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_helpers.go: string `dev` has 5 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_helpers.go: string `link` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_lifecycle.go: string `add` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `dev` has 5 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `eth0` has 3 occurrences, but such constant `replayInterface` already exists (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `/proc/net/pppoe` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle_wire.go: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_padr_replay.go: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/radiussubscriber/checkers.go: (*scenarioLab).waitRecord - status always receives "Start" (unparam)
internal/le/interoplab/radiussubscriber/checkers.go: (*scenarioLab).waitStop - result 0 (github.com/ze-software/ze/internal/le/interoplab/radiussubscriber.accountingRecord) is never used (unparam)
internal/le/job/contention_test.go: age - by always receives 2 * StallDefault (3600000000000) (unparam)
internal/le/repo/changed/actions.go: string `list` has 3 occurrences, make it a constant (goconst)
internal/le/repo/rewrite/actions.go: string `rewrite` has 3 occurrences, make it a constant (goconst)
internal/le/rfc/check_status.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/lowerlayer_corpus_test.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go: hugeParam: m is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/priority.go: ifElseChain: rewrite if-else to switch statement (gocritic)
internal/le/rfc/profilecoverage_test.go: embedlit: embedded field type can be removed from struct literal (modernize)
internal/le/rfc/provenshare.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/session/actions.go: string `ensure` has 3 occurrences, make it a constant (goconst)
internal/le/setup/actions.go: string `install` has 3 occurrences, but such constant `installVerb` already exists (goconst)
internal/le/site/home.go: File is not properly formatted (goimports)
internal/le/site/home_test.go: File is not properly formatted (goimports)
internal/le/site/rfcprofiles_test.go: embedlit: embedded field type can be removed from struct literal (modernize)
internal/le/site/terminaldemo/actions.go: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/image_pins.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/image_pins.go: regexpSimplify: can re-write `^[0-9]+\.[0-9]+\.[0-9]+$` as `^\d+\.\d+\.\d+$` (gocritic)
internal/le/site/terminaldemo/manifest.go: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/manifest.go: string `.webp` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go: string `Source` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go: string `--timeout` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/render.go: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/render.go: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go: rangeValCopy: each iteration copies 216 bytes (consider pointers or indexing) (gocritic)
internal/le/site/terminaldemo/retire.go: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go: string `cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go: string `.webp` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios.go: func commandContext is unused (unused)
internal/le/site/terminaldemo/scenarios.go: func (*osExecCmd).CombinedOutput is unused (unused)
internal/le/site/terminaldemo/scenarios.go: func runBounded is unused (unused)
internal/le/site/terminaldemo/scenarios.go: func terminatePID is unused (unused)
internal/le/site/terminaldemo/scenarios.go: string `203.0.113.1` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios.go: type osExecCmd is unused (unused)
internal/le/site/terminaldemo/scenarios.go: var execCommandContext is unused (unused)
internal/le/site/terminaldemo/scenarios_routing.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_routing.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_routing.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_control.go: filepathJoin: "/proc" contains a path separator (gocritic)
internal/le/site/terminaldemo/scenarios_showcase_control.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/scenarios_showcase_control.go: slicescontains: Loop can be simplified using slices.Contains (modernize)
internal/le/site/terminaldemo/scenarios_showcase_control.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_control.go: stringsseq: Ranging over FieldsSeq is more efficient (modernize)
internal/le/site/terminaldemo/scenarios_showcase.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_network.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/scenarios_showcase_network.go: string `--max-time` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/terminaldemo_test.go: filepathJoin: "../../../.." contains a path separator (gocritic)
internal/le/site/terminaldemo/validate_policy_evidence.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/validate_policy_evidence.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_policy_evidence.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `bgp-rib` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase_topics.go: string `Source` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `172.29.0.2` has 6 occurrences, but such constant `showcaseFRRRouterID` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `exec` has 4 occurrences, but such constant `commandExec` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_ipsec.go: string `172.30.0.1` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_ipsec.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_ospf.go: string `172.29.0.2` has 6 occurrences, but such constant `showcaseFRRRouterID` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_ospf.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topics.go: string `--timeout` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_traffic.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_traffic.go: string `--max-time` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go: string `203.0.113.1` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/test/deployment/actions.go: string `run` has 11 occurrences, make it a constant (goconst)
internal/le/test/deployment/vppiface.go: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/vppifacescenarios.go: string `wireguard` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: filepathJoin: "internal/le/test/deployment/testdata/Dockerfile.wireguard" contains a path separator (gocritic)
internal/le/test/deployment/wireguardproof.go: string `10.88.0.2` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: string `add` has 4 occurrences, but such constant `ipAdd` already exists (goconst)
internal/le/test/deployment/wireguardproof.go: string `left` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: string `wireguard` has 4 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go: string `run` has 15 occurrences, but such constant `runVerb` already exists (goconst)
internal/le/test/qemu/alltests.go: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/install.go: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/stress.go: string `run` has 15 occurrences, but such constant `runVerb` already exists (goconst)
internal/le/test/qemu/vpnproof.go: G304: Potential file inclusion via variable (gosec)
internal/le/verify/actions.go: string `run` has 3 occurrences, make it a constant (goconst)
internal/le/verify/deps/actions.go: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/verify/deps/actions.go: string `run` has 3 occurrences, make it a constant (goconst)
internal/le/workflowcheck/workflowcheck_test.go: ruleguard: negated condition with an else -- swap the branches and test third == "" (gocritic)
internal/test/fixture/register_bgp_delay_open.go: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors (errorlint)
internal/test/fixture/ui_fixture_display_fill_completion.go: displayFillOpenPTY - rows always receives 50 (unparam)
internal/test/fixture/ui_fixture_le_evidence_answers.go: runLEEvidenceAnswers$2 - status always receives "" (unparam)
internal/test/fixture/ui_fixture_le_rfc_answers.go: stringsbuilder: using string += string in a loop is inefficient (modernize)
internal/test/runner/exclusive_lease_test.go: ifElseChain: rewrite if-else to switch statement (gocritic)
```

### new.txt — 043c682d24

163 lines, 19035 bytes. SHA256:
`724375a3fe11bf4c7143ff927d4c595be90b42f63345fbaa95fe387e6a16ad9b`.

```text
internal/appliance/cache_download_test.go: `Cancelling` is a misspelling of `Canceling` (misspell)
internal/component/bgp/fsm/delay_open.go: ruleguard: negated condition with an else -- swap the branches and test f.timers.HoldTime() == 0 (gocritic)
internal/component/bgp/plugins/rib/rib_pipeline_bestfact_test.go: badLock: defer is missing, mutex is unlocked immediately (gocritic)
internal/component/bgp/reactor/delay_open_regression_test.go: Error return value is not checked (errcheck)
internal/component/bgp/reactor/delay_open_test.go: Error return value is not checked (errcheck)
internal/component/bgp/reactor/peer.go: (*Peer).teardownAutomatic - subcode always receives message.NotifyCeaseOutOfResources (8) (unparam)
internal/component/bgp/reactor/session.go: `signalled` is a misspelling of `signaled` (misspell)
internal/component/cli/editor_commit.go: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go: ifElseChain: rewrite if-else to switch statement (gocritic)
internal/component/cli/editor_create_entry.go: rangeValCopy: each iteration copies 160 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go: rangeValCopy: each iteration copies 168 bytes (consider pointers or indexing) (gocritic)
internal/component/cli/editor_create_entry.go: return both a `nil` error and an invalid value: use a sentinel error instead (nilnil)
internal/component/cli/editor_create_entry.go: ruleguard: negated condition with an else -- swap the branches and test changeUser(e.originalPath, path) == replacedUser (gocritic)
internal/component/cli/editor_create_entry.go: slicesbackward: backward loop over slice can be modernized using slices.Backward (modernize)
internal/component/cli/editor_create_entry_review_test.go: ruleguard: negated condition with an else -- swap the branches and test content == "" (gocritic)
internal/component/config/meta.go: appendAssign: append result not assigned to the same slice (gocritic)
internal/component/gnmi/get_test.go: stringXbytes: suggestion: !bytes.Equal(val.GetJsonVal(), ietf.GetJsonIetfVal()) (gocritic)
internal/component/ike/engine/child.go: atomictypes: var policyRestoreFailed uint32 may be simplified using atomic.Uint32 (modernize)
internal/component/ike/engine/child.go: slicesbackward: backward loop over slice can be modernized using slices.Backward (modernize)
internal/component/ike/engine/esp_fixed_test.go: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/component/ike/ipsec/algorithm_support_test.go: Error return value is not checked (errcheck)
internal/component/plugin/server/poststartup_report_test.go: `cancelled` is a misspelling of `canceled` (misspell)
internal/le/ai/rules/actions.go: string `report` has 4 occurrences, make it a constant (goconst)
internal/le/commit/owed_consumers_test.go: File is not properly formatted (gofmt)
internal/le/doc/yangcontract/actions_test.go: File is not properly formatted (gofmt)
internal/le/hookruntime/postbash.go: stringsseq: Ranging over FieldsSeq is more efficient (modernize)
internal/le/interoplab/bgp/check_delayopen.go: string `evidence` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_delayopen.go: string `frr-sink.conf` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_delayopen.go: string `network` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_delayopen.go: string `passed` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_delayopen.go: string `router bgp 65002` has 3 occurrences, but such constant `policyModifyMPRouter` already exists (goconst)
internal/le/interoplab/bgp/check_delayopen.go: string `show version` has 4 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_delayopen.go: string `/var/run/frr/bgpd.pid` has 4 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_delayopen_test.go: comparing with == will fail on wrapped errors. Use errors.Is to check for a specific error (errorlint)
internal/le/interoplab/bgp/check_delayopen_test.go: `signalling` is a misspelling of `signaling` (misspell)
internal/le/interoplab/bgp/check_delayopen_wire.go: stringsseq: Ranging over SplitSeq is more efficient (modernize)
internal/le/interoplab/bgp/check_extended_message.go: string `router bgp 65002` has 3 occurrences, but such constant `policyModifyMPRouter` already exists (goconst)
internal/le/interoplab/bgp/check_extras.go: string `-ec` has 4 occurrences, but such constant `shellErrexitCommand` already exists (goconst)
internal/le/interoplab/bgp/check_parsed_empty_mp.go: string `evidence` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_parsed_empty_mp.go: string `network` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_parsed_empty_mp.go: string `passed` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_parsed_empty_mp.go: string `show version` has 4 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_parsed_empty_mp.go: string `/var/run/frr/bgpd.pid` has 4 occurrences, make it a constant (goconst)
internal/le/interoplab/bgp/check_special.go: string `router bgp 65002` has 3 occurrences, but such constant `policyModifyMPRouter` already exists (goconst)
internal/le/interoplab/bgp/prepare.go: string `frr-sink.conf` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/lab.go: ruleguard: negated condition with an else -- swap the branches and test s.ImagesOnDemand (gocritic)
internal/le/interoplab/pppoe/check_ac.go: string `eth0` has 3 occurrences, but such constant `replayInterface` already exists (goconst)
internal/le/interoplab/pppoe/check_ac.go: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_helpers.go: string `dev` has 5 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_helpers.go: string `link` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_lifecycle.go: string `add` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `dev` has 5 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `eth0` has 3 occurrences, but such constant `replayInterface` already exists (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `/proc/net/pppoe` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle.go: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/pppoe/check_lifecycle_wire.go: ST1005: error strings should not be capitalized (staticcheck)
internal/le/interoplab/pppoe/check_padr_replay.go: string `-TERM` has 3 occurrences, make it a constant (goconst)
internal/le/interoplab/radiussubscriber/checkers.go: (*scenarioLab).waitRecord - status always receives "Start" (unparam)
internal/le/interoplab/radiussubscriber/checkers.go: (*scenarioLab).waitStop - result 0 (github.com/ze-software/ze/internal/le/interoplab/radiussubscriber.accountingRecord) is never used (unparam)
internal/le/job/contention_test.go: age - by always receives 2 * StallDefault (3600000000000) (unparam)
internal/le/repo/changed/actions.go: string `list` has 3 occurrences, make it a constant (goconst)
internal/le/repo/rewrite/actions.go: string `rewrite` has 3 occurrences, make it a constant (goconst)
internal/le/rfc/check_status.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/lowerlayer_corpus_test.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/meta.go: hugeParam: m is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/rfc/priority.go: ifElseChain: rewrite if-else to switch statement (gocritic)
internal/le/rfc/profilecoverage_test.go: embedlit: embedded field type can be removed from struct literal (modernize)
internal/le/rfc/provenshare.go: hugeParam: meta is heavy (320 bytes); consider passing it by pointer (gocritic)
internal/le/session/actions.go: string `ensure` has 3 occurrences, make it a constant (goconst)
internal/le/setup/actions.go: string `install` has 3 occurrences, but such constant `installVerb` already exists (goconst)
internal/le/site/home.go: File is not properly formatted (goimports)
internal/le/site/home_test.go: File is not properly formatted (goimports)
internal/le/site/rfcprofiles_test.go: embedlit: embedded field type can be removed from struct literal (modernize)
internal/le/site/terminaldemo/actions.go: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/image_pins.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/image_pins.go: regexpSimplify: can re-write `^[0-9]+\.[0-9]+\.[0-9]+$` as `^\d+\.\d+\.\d+$` (gocritic)
internal/le/site/terminaldemo/manifest.go: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/manifest.go: string `.webp` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go: string `Source` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/pty.go: string `--timeout` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/render.go: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/render.go: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go: rangeValCopy: each iteration copies 216 bytes (consider pointers or indexing) (gocritic)
internal/le/site/terminaldemo/retire.go: string `.cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go: string `cast` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/retire.go: string `.webp` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios.go: func commandContext is unused (unused)
internal/le/site/terminaldemo/scenarios.go: func (*osExecCmd).CombinedOutput is unused (unused)
internal/le/site/terminaldemo/scenarios.go: func runBounded is unused (unused)
internal/le/site/terminaldemo/scenarios.go: func terminatePID is unused (unused)
internal/le/site/terminaldemo/scenarios.go: string `203.0.113.1` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios.go: type osExecCmd is unused (unused)
internal/le/site/terminaldemo/scenarios.go: var execCommandContext is unused (unused)
internal/le/site/terminaldemo/scenarios_routing.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_routing.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_routing.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_control.go: filepathJoin: "/proc" contains a path separator (gocritic)
internal/le/site/terminaldemo/scenarios_showcase_control.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/scenarios_showcase_control.go: slicescontains: Loop can be simplified using slices.Contains (modernize)
internal/le/site/terminaldemo/scenarios_showcase_control.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_control.go: stringsseq: Ranging over FieldsSeq is more efficient (modernize)
internal/le/site/terminaldemo/scenarios_showcase.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/scenarios_showcase_network.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/scenarios_showcase_network.go: string `--max-time` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/terminaldemo_test.go: filepathJoin: "../../../.." contains a path separator (gocritic)
internal/le/site/terminaldemo/validate_policy_evidence.go: G304: Potential file inclusion via variable (gosec)
internal/le/site/terminaldemo/validate_policy_evidence.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_policy_evidence.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `bgp-rib` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `customer` has 9 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_showcase_topics.go: string `Source` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `172.29.0.2` has 6 occurrences, but such constant `showcaseFRRRouterID` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_bfd.go: string `exec` has 4 occurrences, but such constant `commandExec` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_ipsec.go: string `172.30.0.1` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_ipsec.go: string `established` has 7 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_ospf.go: string `172.29.0.2` has 6 occurrences, but such constant `showcaseFRRRouterID` already exists (goconst)
internal/le/site/terminaldemo/validate_topic_ospf.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topics.go: string `--timeout` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_traffic.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_traffic.go: string `--max-time` has 3 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go: string `203.0.113.1` has 5 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go: string `absent` has 4 occurrences, make it a constant (goconst)
internal/le/site/terminaldemo/validate_topic_vrrp.go: string `down` has 7 occurrences, make it a constant (goconst)
internal/le/test/deployment/actions.go: string `run` has 11 occurrences, make it a constant (goconst)
internal/le/test/deployment/vppiface.go: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/vppifacescenarios.go: string `wireguard` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: filepathJoin: "internal/le/test/deployment/testdata/Dockerfile.wireguard" contains a path separator (gocritic)
internal/le/test/deployment/wireguardproof.go: string `10.88.0.2` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: string `add` has 4 occurrences, but such constant `ipAdd` already exists (goconst)
internal/le/test/deployment/wireguardproof.go: string `left` has 4 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: string `NET_ADMIN` has 3 occurrences, make it a constant (goconst)
internal/le/test/deployment/wireguardproof.go: string `wireguard` has 4 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/actions.go: string `run` has 15 occurrences, but such constant `runVerb` already exists (goconst)
internal/le/test/qemu/alltests.go: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/install.go: string `installer` has 3 occurrences, make it a constant (goconst)
internal/le/test/qemu/stress.go: string `run` has 15 occurrences, but such constant `runVerb` already exists (goconst)
internal/le/test/qemu/vpnproof.go: G304: Potential file inclusion via variable (gosec)
internal/le/verify/actions.go: string `run` has 3 occurrences, make it a constant (goconst)
internal/le/verify/deps/actions.go: string `check` has 3 occurrences, make it a constant (goconst)
internal/le/verify/deps/actions.go: string `run` has 3 occurrences, make it a constant (goconst)
internal/le/workflowcheck/workflowcheck_test.go: ruleguard: negated condition with an else -- swap the branches and test third == "" (gocritic)
internal/plugins/iface/netlink/manage_linux.go: errorsastype: errors.As can be simplified using AsType[netlink.LinkNotFoundError] (modernize)
internal/plugins/iface/vpp/health_test.go: Error return value is not checked (errcheck)
internal/plugins/iface/vpp/health_test.go: net.Listen must not be called. use (*net.ListenConfig).Listen (noctx)
internal/plugins/trafficusage/attach_linux.go: string `GPL` has 3 occurrences, make it a constant (goconst)
internal/plugins/trafficusage/program_linux.go: string `GPL` has 3 occurrences, make it a constant (goconst)
internal/test/fixture/register_bgp_bfd_delay_open.go: deferInLoop: Possible resource leak, 'defer' is called in the 'for' loop (gocritic)
internal/test/fixture/register_bgp_bfd_delay_open.go: Error return value is not checked (errcheck)
internal/test/fixture/register_bgp_bfd_delay_open.go: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors (errorlint)
internal/test/fixture/register_bgp_delay_open.go: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors (errorlint)
internal/test/fixture/ui_fixture_display_fill_completion.go: displayFillOpenPTY - rows always receives 50 (unparam)
internal/test/fixture/ui_fixture_le_evidence_answers.go: runLEEvidenceAnswers$2 - status always receives "" (unparam)
internal/test/fixture/ui_fixture_le_rfc_answers.go: stringsbuilder: using string += string in a loop is inefficient (modernize)
internal/test/runner/exclusive_lease_test.go: ifElseChain: rewrite if-else to switch statement (gocritic)
```

## Retired action assertions: retained approval and proof boundaries

These are coverage dispositions, not new runtime results. The current carriers
below were identified from source; their presence does not claim a new passing
run, and a behavioral test is not described as preserving a retired snapshot.

| Retired assertion | Approval and current disposition |
| --- | --- |
| `qemu.TestQEMUActionsIncludeTheHostRun`: exactly 17 actions in fixed order | Commit `22f782e1c2` records the owner's approval to remove the list-pinning test after focused replacements. The count and order snapshot is deliberately retired, not replaced by successful dispatch or dynamic membership. |
| Its names `vpp-hugepages-test`, `mpls-boot-test`, `crash-capture-panic-harvest`, `crash-capture-ota-unaffected`, `run` | Current action table: `vpp-hugepages`, `mpls-boot`, the unchanged two crash names, and `run` (`internal/le/test/qemu/actions.go`). Existing focused source carriers include `TestHugepagesFallsBackToTheLinuxFirmwarePath`, `TestTheMPLSSeedKeepsTheDefaultAndPutsMPLSInUse`, `TestCrashQEMUArgsAddMonitorAndForward` and `TestParseRunArgumentsAndBoundaries`. These cover their stated behavior, not this five-name membership snapshot or every VM route. |
| Its names `install-test`, `install-iso-test`, `install-scenarios-test`, `install-ventoy-test` | Current names are `installer`, `installer-iso`, `installer-scenarios`, `installer-ventoy`. `TestInstallerActionsAreNativeAndComplete` retains all four named membership assertions without the total count or ordering. |
| Its names `vrrp-keepalived-test`, `ipsec-mobike-test`, `docker-lab`, `pppoe-accel-test`, `netns-test`, `pppoe-test` | Current names drop `-test`. `TestGuestEvidenceActionsAreNative` retains named membership for `vrrp-keepalived`, `pppoe-accel`, `netns`, `pppoe`; `TestDockerLabActionIsRegistered` covers `docker-lab` registration/grammar. No dedicated `ipsec-mobike` membership replacement is asserted here. The later `TestVPNKernelActionRefusesGuestOverrides` tests `vpn-kernel`, a different action, and is not a replacement for these six names. |
| Its names `all-tests`, `stress` | Current names are `all`, `stress`. `TestTheNeedsLinuxSelectionReachesEveryChild` and `TestOneTestRunsOnlyTheSuiteThatWalksItsDirectory` cover selected all-suite behavior; `TestStressActionIsRegistered` and `TestStressScenarioPlaceholderIsTheRegistry` cover stress registration and derived scenario grammar. These do not restore the original total/order assertion. |
| `terminaldemo.TestActionsCarryTheirContracts`: exact four-row order/count, area identity, `check/build/render/pty` names, `Writes` flags and exact `Why` prose | Commit `b0da8037c5` records owner-approved removal and explicitly states that the replacements do not preserve the deliberately removed metadata/prose snapshot. The fifth `readme-carrier` action caused the old fixed-count red. Exact prose is retired rather than re-pinned. Generic dynamic action registration/dispatch tests are additional coverage, not proof of those deleted metadata values. |
| Its `check` and `build` target grammar, `render` argument grammar and `pty` forwarding | `TestCheckAndBuildDeclareExactlyTheirTargets`, `TestCheckAndBuildRefuseAMissingOrUndeclaredTarget`, `TestEveryDeclaredTargetParses`, `TestRenderAllAloneIsAccepted`, `TestRenderWithoutADemoIdIsRefused`, `TestRenderAllWithADemoIdIsRefused` and `TestPTYActionPreservesRecorderErrors` retain focused parser/refusal behavior. `TestPTYActionPreservesRecorderErrors` reaches the real option/file refusal path through `Answer`; it does not independently certify the removed `Forwards` metadata assertion. |

`internal/le/actions_test.go` derives its population from registered areas:
`TestEveryRegisteredAreaProvidesActions` checks that each publishes a table;
`TestEveryPublishedVerbIsDispatchedThroughItsOwnTable` checks invalid-grammar
dispatch parity. Neither proves that a historically named action still exists,
that an accepted line executes successfully, or that a writing action has the
correct effect metadata. P-34/P-38 must retain those separate boundaries when
recording current native entry-point results.

P-34 carriers added 2026-11-08, written and not yet run. Each is membership
over the live producer, with no count or order:

| Obligation | Carrier | Boundary |
| --- | --- | --- |
| Action contracts | `TestActionsCarryTheirContracts` (`internal/le/site/terminaldemo/actions_contract_test.go`), same name as the retired snapshot, membership form | Asserts the area name, `check` read-only and `build`/`render` writing, `render` takes arguments, `pty` alone forwards. No `Why` prose, no `pty` writes claim. Does not restore the retired four-row snapshot |
| Renamed callers | `TestRenamedCallersReachRegisteredActions` (`internal/le/one_off_proof_test.go`) | Every rename-map row that names a new action reaches a verb its command's table declares, and no retired spelling is still served. The three callers the 2026-10-27 row named were already moved: `test/runner/verify-scope-freshness-scoped.ci` calls `verify status record`, `ui_fixture_le_binary_dispatches.go` requires doc index `check`/`update`, `le-discovery-answers` calls `update`. No scan of `.ci` or fixture Go argv; workflows resolve through `workflowcheck.actionExists` |
| Lab grouping | `TestLabPinReportDoesNotWrite` (`internal/le/lab/pin/group_test.go`) | `lab pin` is registered under `GroupGenerate` (register.go); the test refuses `GroupReport` while a row writes. `TestReportAreasWriteNothing` remains the all-area rule |

## Retired workflow, discovery and probe assertions (P-19)

Found with `git log -S` on each name. Coverage dispositions read from source,
not new runtime results. None of the three commits carries a
`Weakening-approved:` trailer, so each row still owes an independent
`ze-test-judge` verdict.

| Retired assertion | Removed in | Approval and current disposition |
| --- | --- | --- |
| `workflowcheck.requireEveryJobAdvisory` in `TestEvidenceNightlyScheduleActionsAndPrivileges`: every `evidence-nightly.yml` job carries `continue-on-error: true` | `6df62d6f73` (2026-09-13) | The commit body shows the old expectation wrong against its producer: `7682286f9b` removed the key on 2026-09-02 because the workflow reported success while five of six jobs failed. The property the name meant (no merge waits on the job) is `requireScheduledOnly`: `schedule:` present, no `push:` or `pull_request:`. The inverted key is now `requireEveryJobStatesItsOwnVerdict`, which refuses job-level `continue-on-error: true`. Independent judge, 2026-11-08: APPROVE (b) |
| The same helper in `TestQEMUNightlyScheduleActionsCachesAndBudgets`: every `qemu-nightly.yml` job carries the key | `6df62d6f73` | Independent judge, 2026-11-08: REFUSE, because the key had no assertion either way while the three jobs still carried it, so a failed nightly job reported success. Repaired: the key is removed from all three jobs and the test now calls `requireEveryJobStatesItsOwnVerdict` after `requireScheduledOnly` |
| `le-discovery-answers` `gitChangedFiles`: the router's `changed` list equals live-checkout `git status --porcelain` paths | `3815463353` (2026-10-05) | The old expectation was wrong against its producer: `616e397902` made `le doc wiring` judge the unpushed range, so any unpushed commit reddened it. Replaced by `leDiscoveryWiringTree` (`internal/test/fixture/ui_fixture_le_discovery_answers.go`, called at line 53, compared at line 177). It builds an owned repository with a published `origin/HEAD` baseline, one unpushed commit, a staged, an unstaged and an untracked file, and requires the routed list to equal exactly those four. This proves more than the old live comparison, which could not hold an unpushed commit. Independent judge, 2026-11-08: APPROVE (a) |
| `TestProbeUnclaimedRoutes`, `TestProbeSharedHeader`, `TestProbeAuthoredPages` (`internal/le/site/zzprobe_p10b_test.go`) and `TestScratchDocsDest`, `TestScratchProducerCensus` (`zzscratch_census_test.go`) | `16efe2aa68` (2026-09-13) | Every finding went to `t.Logf`, which cannot fail. Most `t.Fatal` calls checked the probe's own I/O; the three that reached the product (errors from `docsProducerPages`, `sharedHeaderHTML`, `renderAuthoredPages`) are still asserted in `docs_test.go`, `nav_test.go` and `authored_test.go`. Independent judge, 2026-11-08: APPROVE (a); the units are deleted, so the verdict rides in the commit body |

## Contemporary review follow-up: not recovered historical evidence

The following observations concern the current repair, not the missing June or
October originals. Complete log paths are under
`tmp/session/2026-11-02-121815/scratch/`; a short command hash is not a unique
identity across differently labeled runs.

| Current observation | Retained evidence and boundary |
| --- | --- |
| Kernel worker ownership landed | Commit `10c07359d9`; real Docker isolation/removal `56aa4530`, scoped contracts `7351afb1`. Native fixture assertions `2dfcf692` passed, but that invocation failed its then-in-flight BGP vet prerequisite. No kernel build or boot proof. |
| Approval replay and captured journal defects | `job-tooling-journal-approval-replay-first-baselines-20261104a-472625a5.log`: actual duplicate target publication, ordinary historical verdict renewal, and live-checkout validation after immutable capture. Source repairs are not restored proof. |
| Native capability order is not session identity | `job-tooling-gobgp-capability-multiset-first-baseline-20261104a-e67cf478.log`: three unchanged permutations falsely refuse; original and five changed-content/multiplicity controls pass. No native GoBGP reload scenario is proven. |
| Hook admission and startup false successes | `job-tooling-new-hook-native-consumer-first-baselines-20261104a-e8b5e9c1.log`: actual panic/tool-routing, WIP and observed-source defects; all eight invalid startups earn success, while real startup passes. Exemption cases initially lack documentation prerequisites, not product evidence. |
| New permission and Hugepages review gates | `job-tooling-pending-review-gates-first-baselines-20261104a-eb0b747a.log`: denied subjects falsely allow after readable controls; unique carrier moves falsely refuse with duplicate negatives passing; old exit-status formatting, cooperative shutdown and canceled-verdict checks fail. Ancestry is blocked before publication by a new tree-resolution prerequisite, not observed as a semantic failure. |
| Canonical catalog normal control remains limited | `job-tooling-yang-normal-timeout-producer-diagnostic-20261104a-1c8348e6.log`: actual native invocation completes; helper is still validating sibling rendered publication at the unchanged normal bound. This is not fixed by fixture isolation. |
| Isolated native catalog normal control | `job-tooling-yang-same-checkout-isolated-normal-control-20261104a-1c8348e6.log`: actual Answer/build/run/parse passes under independently judged same-checkout symlink-root isolation. No source copy or export; no sibling publication/full-tree WalkDir equivalence is claimed. |
| Vendor preservation landed | Commit `47340d61c5`; eight Go paths and only the reviewed vendor documentation hunks. No repeated checks or live repository setup install; the unrelated standard-library discovery documentation change remains uncommitted. |
| Current authored browser workflow | `job-tooling-authored-browser-config-save-current-smoke-20261104a-d08a0009.log`: all35 login/edit/save/diff/confirm/readback steps pass against rebuilt DUT `0e87be15`; exact executable identities in `scratch/browser-authored-workflow-current-identities-20261104.json`. No recorder/image/poster result. |
| Native, Hugepages and isolated catalog restoration | `job-tooling-native-hugepages-yang-scoped-restoration-20261104a-5b794f03.log`: all selected assertions in seven packages pass, including the previously unreached injection assertions and stable lint parity. No real filesystem mount, PPP, image, kernel or VM proof. |
| Subsequent unsolicited-signal and framing defects | `job-tooling-unsolicited-signal-framing-and-live-clock-proof-20261104a-155fa6e4.log`: actual unsolicited SIGKILL/stale-build/held-pipe and hidden Linux-limit obligation REDs, with their opposite controls. After the minimal repairs, every selected named case passes in `job-tooling-causal-signal-and-approval-framing-restoration-20261104a-f3688fe7.log`; two independent narrow source reviews are CLEAN. |
| Current live-clock image boundary | The same `155fa6e4` log passes fixed20/15, unequal real waits and advance40/35; only equal-clock PNG bytes fail. Retained `chaos-live-clock-2534779146/` images differ at28 opaque pixels in rounded borders, confirmed by standard image/png decoding in `job-tooling-retained-live-clock-changed-pixels-20261104a-8b8dd5cf.log`. Painted regions identified; raster cause not established, no masks/tolerance or golden refresh. Historical080d remains unattributed. |
| Native/Hugepage process owners landed | Commit `cd3474d5d2`, 52 paths with 14 selected shared-file patches. Exact native script executed; browser/clock, guest/run, unrelated runner and foreign FreeBSD hunks preserved. Host-process regression/control evidence is not full PPP/kernel/image/VM/mount proof. |
| Publication restoration and maintenance distinction | `job-tooling-prospective-and-bound-publication-scoped-restoration-20261104a-b71f872d.log` reaches all 39 selected functions without skips; its three failures restore in `job-tooling-publication-failures-and-new-owner-normal-controls-20261104a-35a38758.log`. Two exact exit-expectation corrections are independently judged maintenance; the non-Git discovery-prefix fix is the product correction. |
| Compact metadata and opaque approval preparation | `job-tooling-compact-status-and-rs-replay-first-baselines-20261104a-19ffeeed.log` proves compact-status mechanisms bypasses at three consumer layers and opaque-reason replay preparation admission. The actual second CAS script prevents publication. Those failures restore in `job-tooling-compact-rs-alias-startup-failure-restoration-20261104a-1b640361.log`, whose later fixture build fails before startup-observer execution. |
| Current committed inventory remains blocked | `job-tooling-retained-source-five-fixes-and-current-root-restoration-20261104a-b9af02d8.log` passes all five isolated groups, then actual real-root public Plugins refuses the named bodyless unix.Syscall boundary. Revised retained-linkage design is admitted for new tests only; no full-report completeness or assembler implementation claim. |
| Observed clock geometry, not raster attribution | `job-tooling-live-clock-readonly-geometry-baseline-20261104a-717e9fa6.log` still fails exact bytes while retained sidecars show matching render geometry/styles, page identity, DPR/viewport and backend Chrome147.0.7727.57. Diagnostic elapsed time differs; reads can alter repaint timing. No flag/default/mask/tolerance change; old080d remains unattributed. |
| Bounded source population discriminator | `job-tooling-authority-package-first-baselines-and-startup-observer-20261104a-bbaf0e73.log` observes old fingerprint false accepts for external content, membership/mode/identity and framing. The accepted 15-package replacement and exact judged fixture cutover are staged; stored seal/native80 proof remain owed. It is not exhaustive registered-hook/toolchain closure. |
| Startup observer restoration | The same `bbaf0e73` run passes the seven previously failing cases after the foreign import collision independently disappears. Existing unknown/real-startup proofs are retained, not repeated. The timeout observer proves serialized deadline text, not in-process error identity. |
| First assembly/report controls | `job-tooling-committed-assembly-new-boundaries-baseline-20261104a-7a5443bb.log` exposes a consumed CI-buffer coverage defect and interface-callback partial inventories. Positive assembly inputs still refuse missing bodies; negative assembly/header refusals remain prerequisite-masked. No full inventory or linkage proof is inferred. |
| Temporary observable Chrome control | `job-tooling-clock-owned-argv-automation-control-20261104a-fb41fc6f.log` observes actual browser argv with temporary automation enabled and still fails exact pixels. The added Skia probe `job-tooling-clock-skia-runtime-disabled-control-20261104a-3125f8cb.log` fails before capture with no effective argv observation. No raster attribution, production flag change or golden refresh follows. |
| Full P5 review exposes ordinary verdict reuse | `job-tooling-ordinary-append-exact-unit-and-byte-debt-baselines-20261104a-ae8358ac.log` actually publishes both RFC and Weakening A/B contents with one ordinary verdict. It also observes unchanged one-line function issuance/selection and writes. These are distinct from preparation-only RS replay; source repairs remain in flight. |
| Raw RFC history restoration and Git body limit | The same `ae8358ac` run restores both RS/RSUS approval preparation refusals and affected ordinary/bound controls after length-framed raw commit reads. Normal Git rejects the two NUL-body objects before debt execution; exact independently judged retirement preserves that limitation, not a debt RED or native NUL-publication claim. |
| Literal paths and owner-execute review pins | `job-tooling-native-prospective-literal-and-mode-first-baselines-20261104a-a84b7b49.log` exposes the two product defects; `job-tooling-native-prospective-edge-and-help-restoration-20261104a-f749daa9.log` restores all new cases. Independently judged help-count retirement is separate maintenance. |
| Actual native compiler descendant leak | `job-tooling-current-native-qemu-root-toolexec-smoke-20261104a-960411b6.log` observes the real tool surviving its Go parent. `job-tooling-qemu-real-go-descendant-first-baseline-20261104a-5db6a848.log` reproduces all4 controlled consumers. Main cleans only its verified isolated orphan group; no foreign process is touched. |
| Nonterminal descendant restoration | `job-tooling-qemu-owned-descendants-and-policy-restoration-20261104a-325dad55.log` passes descendants, prior direct-root and stdin/exit0/37/137 controls. Two narrow source reviews are CLEAN. Fresh rebuilt native smoke, VM and own-system guest remain unproven. |
| Assembly certificate progress remains bounded | `job-tooling-committed-assembly-certificates-and-real-root-restoration-20261104a-4d317371.log` passes all8 new groups but real-root Plugins refuses selected xxhash preprocessing. New `job-tooling-committed-assembly-lexical-first-baselines-20261104a-3a2c1e9e.log` exposes label-census and block-comment defects; repairs and macro admission remain separate. |
| Additional clock controls do not attribute the raster defect | `job-tooling-clock-disable-gpu-controlled-capture-20261104a-1e6c4988.log` observes requested argv and still fails exact pixels. Startup-only `job-tooling-native-skia-aboutblank-startup-diagnostic-20261104a-1d09b1de.log` opens/closes its owned blank session; it does not explain the earlier dashboard deadline. No production default changes; old080d remains unattributed. |
| Detailed AI reference reaches its real entry | `job-tooling-detailed-operator-ai-reference-native-smoke-20261104a-cc836d48.log` passes actual core+distro help ai all and prints current recipes and corrected unmatched-family guidance. It is not proof of every recipe's execution or remote MCP availability. |
| Ordinary replay, exact-unit and public-route restoration | `job-tooling-approval-authority-restoration-and-compiler-bindings-baseline-20261104a-7d8540f0.log` observes all38 selected top-level functions:35 authority/parser/consumer cases pass, the retained-input diagnostic passes, and both compiler-binding groups fail their six cases. The authority restoration includes actual A/B refusal, exact one-line boundaries, raw RS/US history and the repaired single-word public `approval-check` route; it is not yet the fresh native P5 composition-root smoke. |
| Macro census restoration and observed compiler-binding defects | `job-tooling-committed-macro-lexical-certificates-and-real-root-restoration-20261104a-13525e92.log` restores the selected assembly/macro/lexical/callback groups while actual-root Plugins still refuses `funcdata.h`. The subsequent `7d8540f0` run proves three compiler-header and three target-predefine false accepts. The retained17-file catalogue at `af8729c7c3f907dc824c2d7ae8fbb92d7b57e0b9` contains exactly `textflag.h`, `funcdata.h`, `go_asm.h`; generated headers occur in both cpuinfo and lz4block. This is not full inventory, conditional-assembly or entered-CPU-probe proof. |
| Fresh native QEMU cancellation reaches the real entry | Native build `b825cd27`, SHA256 `e6358dfed3f1fa1d6f7f35da3c1012c2a0b34387a24a1a02300ca4d1f9813574`. `job-tooling-restored-native-qemu-root-descendant-cli-smoke-20261104b-50f3761b.log` proves status129, compiler socket EOF, pidfd-observed tool death, direct Go disappearance and unchanged inherited parent. The actual Go tool-version probe is fixture-blocked: no guest code-generation, image, VM or own-system boot proof. |
| Native smoke correction is test maintenance | The earlier `20261104a-50f3761b` invocation remains FAILED at its unsupported mandatory tool-TERM acknowledgement, before the later assertions. Independent exact criterion-b judgment `C863` corrects the scratch probe to the existing leader-TERM/group-retirement contract, adds pidfd death proof and preserves its other oracles. No product repair or GREEN is attributed to that failed invocation. |
| Authored AI guidance landed with bounded evidence | Main commit `a8803f84fc` carries the help source, two canonical AI/MCP pages and independently reviewed feature-description attestation. Wiki commit `e2065a8` carries only mcp/introspection and their generated llms sections; unrelated changes and the whole-wiki source stamp are preserved. Existing detailed native smoke `cc836d48` is the actual-entry evidence; no tests repeated for either commit, no whole-wiki freshness, remote MCP or recipe-execution claim. |
| Header restoration exposes the next real-root boundary | `job-tooling-canonical-header-producer-and-impacted-assembly-restoration-20261104a-22e75a16.log` passes the new header matrix, six prior compiler-binding REDs and reached assembly/macro/lexical/callback controls. Actual-root Plugins now refuses Zstd `ifdef`; it is not complete inventory proof. The subsequent independently reviewed generic-parameter test actually false-accepts all three cases in `job-tooling-generic-parameter-first-baseline-and-artifact-restoration-20261104a-c24c0f66.log`; the later parameter-scope restoration is recorded below. |
| Frozen native P5 semantic proof retains its original read-only failure | `job-tooling-frozen-native-publication-and-owed-linux-dns-proof-20261104a-1dcf3e0d.log` verifies original inode/hash continuity for binary-only capsule `9a6aeb92…eed5` before/after every CLI/script. All six prospective publication cases, targeted opaque approval-only publication and both ordinary A/B replay refusals pass. Public fresh0/spent1 results are correct, but spent-query fixture bytes change. The original changed paths/writer remain unattributed; later diagnostic and standard-test-isolation results do not rewrite this failed observation. |
| Named executable replacement is attributed, not hidden | Earlier `ac5a2100` native preflight/name failures remain prerequisite failures, not publication REDs. Main's prior QEMU commit script invokes root `./le` under the inherited named build; launcher lines738–767 intentionally rebuild and replace that mutable output every call. Original `e635…3574`/native50b evidence remains historical, not current retained bytes. The later independently approved fixture isolation and frozen binary capsule establish a new explicit provenance boundary rather than silently repinning the old one. |
| DNS and guest fixture maintenance restored | The same `1dcf3e0d` run passes the two owed unchanged Linux resolver-writer tests and the already-approved DNS seed fixture. `job-tooling-own-system-callee-fixture-maintenance-restoration-20261104a-645c04b0.log` passes only the three previously failing, independently judged FAT-root-placement/obsolete-label cases. Neither is a product writer repair, and unit assertions do not prove guest boot. Ordinary-user KVM API12 is separately observed; image/kernel/PID1/writeback remain unproven. |
| Controlled compositor result is not a repaired capture path | Diagnostic c-overlay plus observed automation/full-stage flags passes `a3b8015d`; same-context A-back fails exact PNG bytes in `757aab1a`. Both original no-overlay controls (`11cace51`, `c4994571`) still fail only those bytes, as does the read-only pre-frame layout probe `a35b5f50`. Diagnostic timing/layout remains a confound; no flag/default, budget, CSS, mask, tolerance or golden is changed, and historical080d remains unattributed. |
| Artifact baseline exposes an incorrect fixture expectation | The `c24c0f66` run passes22 of23 named artifact/producer controls, including both safety fixes. `TestArtifactSeedPrunesOnlyAfterSuccessfulOverlay` expects `.DS_Store` to be deployment-cleaned, but the producing source classifies `CACHEDIR.TAG`, not `.DS_Store`, as source-only. Independent exact judgment `E705` approves only the three fixture-name replacements; no product repair is attributed to this maintenance. |
| Generic-parameter shadowing is repaired and narrowly restored | `job-tooling-generic-scope-fix-and-single-artifact-fixture-restoration-20261104a-6a476918.log` passes the three prior false accepts and reached generated/imported-shape controls. The producer checks lexical type-parameter scope before package, predeclared and imported names; both independent source lenses accept the final delta. Actual-root Zstd conditional handling and entered CPU-probe analysis remain owed. |
| All23 named artifact controls now have passing evidence | The same `6a476918` run reruns only the one corrected artifact case and passes; the earlier22 greens are not repeated. Canonical gh-pages fetch/upstream count0 is established separately, but canonical build/check and native publication checks remain unrun. The inventory-root refusal still blocks the canonical build. |
| A nonreproducing query diagnostic does not erase the failure | `job-tooling-native-spent-query-state-diagnostic-20261104a-417dda96.log` passes its sole public-query case with unchanged fixture bytes. Its added metadata walk changes the observation context; the earlier `1dcf3e0d` byte mutation remains unattributed, not fixed. The Phase5 publication chunk may land without claiming phase closure or read-only restoration. |
| Current native binaries are explicitly rebuilt with Go1.27.3 | `job-tooling-current-product-go1273-native-proof-build-20261104a-ca98b5ce.log` builds current DUT SHA256 `66cbc9a3f26c30709bab782a65f13beeda5da32bf14885d7b6ea77d42b8ee5a9`; `job-tooling-current-native-go1273-hint-and-platform-build-20261104b-76a43dfe.log` builds LE SHA256 `40cfa5c92fb835a46d3faf991ee250c13c3041dfab776bbfef0ee49f065f11f0`. Both use CGO0 and established feature gates; neither implies a VM, kernel or image run. |
| The public spent-verdict refusal gives the actual recovery command | `job-tooling-current-native-ordinary-refusal-hint-smoke-20261104a-98b6392a.log` observes expected status1 for a real landed verdict, preserves its original reason and prints the correct independent-judge approval command plus a fresh `le commit create` instruction. This is an actual-entry message smoke, not a repeat of the earlier semantic publication proofs. |
| Additional settling controls still do not repair exact captures | Pre-frame metadata `8134b10a`, after-frame layout `55ed36a4`, after-frame metadata `917f604c`, post-settle extra-transport layout `bbce0d7a` and its matched metadata control `3078dae2` all fail only the original exact-PNG assertion. No production settling change, flag/default, extra frame, budget, mask, tolerance, CSS or golden is released. The complete diagnostic-c semantic-read ablation is a separate, still-unrun causal pair. |
| Renamed and spaced watchdog executables reach the native UI fixture | Draft `744ed7ab` passes; load `11659b2a` records exactly80 sequential actual invocations, each exit0 with the required stdout assertion passing; promoted native `8aac34c4` passes the named live case. Commit `2496442cb3` lands only the four-path source/fixture/carrier/documentation chunk. The earlier `unknown command: test` failure remains attributed; ordinary wire execution is proved, not a VM/image or separate foreground-debug run. |
| Current native stress capture reaches one real child invocation | `job-tooling-stress-current-native-one-iteration-smoke-20261104a-b1bcb869.log` returns expected nonreproduction status1. Its retained capture records the exact JSON argv and invocation1 exit0 with the actual `version-extended` exit-code and stdout assertions passing. This one-invocation entry smoke is not the owed80-iteration watchdog or hook load proof. |
| Captured publication and approval binding land without phase closure | Commit `8c64ad2d4a` carries the85 selected paths, with two shared documentation patches excluding foreign hunks. Exact retained independent verdicts are recorded under the gate's canonical RFC identities; no test/tag rewrite or repeated semantic review clears the preparation refusals. The commit message explicitly preserves the unattributed `1dc` byte mutation and nonreproducing `271` observation. No push or seal/phase-closure claim. |
| External Git tracing observes only history reads | `job-tooling-native-spent-query-external-trace2-diagnostic-20261104a-fae1ff2d.log` passes the sole public fresh0/spent1 case with strict complete snapshots and original frozen-image checks. Its21 TRACE2 events cover exactly `git log` history, `git cat-file --batch`, and the carrier-name `git log`, all status0. No index/status writer is observed. TRACE2 is not a filesystem-write audit, and this attempt remains nonreproduction rather than attribution or repair. |
| Conditional namespace baseline is masked, not false acceptance | `job-tooling-conditional-first-baseline-and-owed-checker-restoration-20261104a-f7c3fab8.log` fails all eight new conditional groups at their positive prerequisite: unsupported `ifdef`/`ifndef`. Their negative cases are masked. The independently admitted branch-neutral namespace design is released only after this observed baseline; entered conditional bodies and unknown registration conditions still owe refusal. |
| Actual stdlib producer smoke and consumer restorations pass separately | The same `f7c3fab8` run passes the throwaway real-installed-GOROOT listing/comparison, with exact `runtime`, `unsafe` and `cmd/go` metadata; it is a valid-input producer smoke, not a new runtime empty-output rejection. LP reverse-barrier4, EOR attribution14, NSSA first-epoch19 and both named DelayOpen capture tests pass. The overall invocation remains FAILED at inventory; these consumer results prove no native peer image, scenario, producer cut or RFC discrimination record. |
| Cancellation restoration exposes a private-fixture prerequisite | `job-tooling-phase6a-owned-cancel-and-slot-restoration-20261104a-d3ffb6b9.log` passes the job identity package but fails dispatcher `cancel-slot` before readiness: its private `git init` has no HEAD for admission's fingerprint. After the exact independently judged empty-commit fixture repair, `job-tooling-cancel-slot-and-eor-bespoke-scoped-restoration-20261104a-0fc494ea.log` reruns only that failed subcase and passes; its other package passes the separately owed Bespoke EOR subcase. This is fixture maintenance, not a product cancellation RED or repair. Test-only race uses the explicit CGO1 exception; no shipping build evidence follows. |
| Native lint guard and combined source chunk land | Native `21c4576e` passes registered listing, then stops before the target guard because the private fixture wrongly ignores `/cache/` as a directory while real startup creates a cache symlink. Correcting only that fixture literal to canonical `/cache`, `job-tooling-p37-frozen-native-tracked-excluded-restoration-20261104a-19c69f5d.log` reruns only the failed case and observes exit2 for the exact excluded `docs/probe/probe.go`, with its live holder unchanged. Commit `7e21d1c4ed` lands the reviewed12-path selection, preserving seven shared-file hunk boundaries. Its canonical file-scope receipt reuses the existing exact guard-deletion verdict and all producer prerequisites. No real linter or full matrix runs; listing is not positive planning proof. |
| Whole-case PID observation finds a permitted preceding writer, not the original mutation | `job-tooling-p5-public-owned-syscall-observation-20261104a-6a4fa820.log` passes the strict public case within its stated observer limits. Raw syscall records identify an earlier native invocation's detached `le scratch trim background` process alive across both snapshots, with fixture `tmp/store-trim/last.log` descriptors; both snapshots see the same57 bytes. This is an observed lifetime hazard, not an observed changed snapshot or attribution of `1dc`. Environment expansion is deliberately omitted, strings are bounded to4096 bytes, and ptrace changes timing. |
| Standard test isolation is restored without changing production | `runner.childEnv` and its existing contract explicitly give test children `ze_le_store_trim=off`; the custom native smoke helper had omitted that isolation. Exact independently judged patch `6A3C` restores the same normalized-alias replacement while retaining all whole-root, semantic and frozen-image assertions. `job-tooling-p5-standard-test-isolation-public-restoration-20261104a-fae1ff2d.log` passes only the affected public case in this corrected fixture context. Default production trim semantics and the original `1dc` attribution limit remain unchanged; this is test maintenance, not a product repair. |
| Conditional certificates restore; actual root reports another boundary | `job-tooling-conditional-namespace-eight-restoration-20261104a-d58f141c.log` passes all eight new groups; `job-tooling-conditional-four-reached-old-controls-20261104a-68515b50.log` passes only the four reached legacy subcases. The subsequent once-only actual-root consumer `job-tooling-conditional-repaired-real-root-next-boundary-20261104a-10b1bf28.log` refuses method execution through parameter `err` at `internal/component/command/local_data.go:367`. Root inventory, native collection and canonical publication remain unproved; no leaf-effect code is yet admitted. |
| Five actual RTR clean carriers pass on pinned artifacts | Named native plugin runs `d8ef42f8`, `af7d0304`, `18ef1991`, `28177bee`, and `2a704202` pass RR/RS pending-withdraw, RR/RS revalidation and late-sync respectively, using frozen LE40cf and DUT66cb. These are current artifact-only CLEAN observations. No protocol-source input digest is inferred from empty build logs; source-bound mutation builds, exact semantic REDs and rebuilt restorations for all five remain owed. |
| Three lifecycle migrations expose a still-unattributed early exit | `job-tooling-phase6a-three-owed-lifecycle-migrations-20261104a-a1129486.log` observes cleanup-override and Git-bounds PASS, then the printed-verdict case returns1 instead of0 or3 in two branches. Its two nominal expected1 branches may be masked by the same early exit, and cleanup-override's intended cause also needs report evidence. A delegating diagnostic overlay is prepared; no assertion, timeout, production or fixture repair is justified before its actual report identifies the producer. |
| Empty stdlib and stress capture chunks land independently | Commit `c98d76f37f` carries only the three-path empty-listing source/test/docs chunk, preserving foreign GOROOT permission/gosec changes. Commit `86cb0aa071` carries the five-path exact-argv/exclusive bounded-capture chunk, preserving other functional-documentation edits. Their retained scoped negative/control/native proofs are not repeated for landing. Neither commit closes the original schema or historical populations. |
| Printed-verdict diagnostics do not reproduce the earlier failures | `job-tooling-phase6a-printed-verdict-two-failure-diagnostic-20261104a-95645e7f.log` passes only the two previously failed branches, with delegating diagnostics:56 actual injected-stage calls, green code0, and device-full code3 with the original ENOSPC log-save cause. Its Git operations are quick. The original sequence included two predecessor tests and lacked reports; instrumentation also changes timing. No product repair or original-failure explanation is inferred. A predecessor-qualified passive-state experiment remains prepared, not run. |
| Native hook composition remains failed despite passing children | `job-tooling-hook-current-native-unit-consumer-proof-20261104a-89fae41d.log` returns2:28 of30 probes pass, but behavioral-fixture population and hook-source drift both fail. Source tracing identifies the former hash's changed input as the already-corrected canonical metadata header in `probeCompleteSpec`, not a changed category declaration. Exact prior approval provenance and final seal refresh remain required; no hash is blindly repinned or overall GREEN claimed. |
| Current schema CLI family has bounded passing evidence | `job-tooling-current-schema-cli-family-not-legacy-seventeen-20261104a-7518e4ba.log` runs actual named deactivate, activation-round-trip, diff and migration cases under the current single config/cli family selector and passes. It does not identify or reconstruct the original17, resolve the historical FlowSpec pair, or erase the separate L2TP membership limit. |
| Derived-artifact lifecycle reaches its real native draft entry | `job-tooling-hook-current-native-derived-draft-proof-20261104a-12a39e8c.log` passes the one actual draft, including exit0 and all six lifecycle stdout assertions. The native unit's two drift failures remain separate. Required80-iteration loaded proof, independent promotion judgment, promoted run and final seal-backed unit proof remain owed. |
| Derived-artifact load and promotion complete on the pinned native | `b55fc042` completes exactly80 actual draft invocations, reproduced=false, with all child exit/status assertions retained. Independent judgment approves only the lifecycle startup/read-render/invalidate/readback carrier; promoted native `6fc7d940` passes all six markers. This is not the whole Runner suite or proof that startup workers are joined. |
| Separate hook ratchets restore one exact native composition | Header-only permission and category-golden approval `856C` remain distinct. Actual source observation `51d99c31` first reports the changed fifteen-direct-package hash, then the stored-ratchet test passes after that exact refresh. Build `affa8eb3` yields native SHA256 `768aa55d40efb8ef2737173e7ca94c8d403e1cdb455c40eae12ea41d777288a7`; its actual `6468934b` unit command passes30/30. The old28/30 failure remains recorded. The seal excludes its own implementation, transitive/toolchain inputs, commit debt, derived implementation, root admission and rules/mirrors. |
| Combined hook review finds two uncovered integration bypasses | Independent `HookPhase7CombinedReview` identifies malformed metadata returning success from `mechanisms.Problem` through the actual commit consumer, and lexical test/extension exemptions preceding canonical producer-subject resolution. Main reads and verifies both producers. Test-first repairs are pending; the fifteen-package source freeze is reopened. The separate spec audit establishes requirement/evidence alignment only, not combined correctness or Phase7 closure. Native30/30 is historical proof for its artifact, not post-repair evidence. |
| Callable ownership baseline demonstrates silent partial publication | `job-tooling-callable-owner-first-semantic-baseline-20261104a-445a7db2.log` observes46 semantic failures across helper aliases, parenthesized calls, opaque/shadowed/indexed/returned/generic callees and the initial snapshot membership checks. Both plugin and YANG consumers publish ordinary-only results or omit `owner.yang`; ordinary and direct-installer controls reach their complete reports. All four argument-registration cases pass. Dirty/HEAD-retention assertions are not reached. Only the independently admitted normalized-callable-owner repair is released; assembly leaf effects and Root-method handling remain unreleased. |
| IPsec logging and build-only staging restore their exact consumers | `job-tooling-ipsec-debug-and-build-only-staging-restoration-20261104a-9a32fd8b.log` passes the actual prepared-peer logger-environment consumer and both build-only staging/failure-stop tests after the earlier semantic REDs. This proves neither a live IPsec peer, image build, kernel behavior nor a performance result. |
| First real own-system carrier stops before image creation | `job-tooling-own-system-real-kernel-image-command-carrier-20261104a-c35499a0.log` fails at `gokrazy/kernel/kernel.config` lookup. The canonical fragment exists; the actual runtime profile resolver joins relative paths while this bare-Answer Go test runs from its package directory. Whether this is missing carrier initialization or an actual CLI defect is not yet established. No image, guest boot, supplied-kernel acceptance, mount or command behavior is proved. |
| Five source-causal RTR mutation artifacts are prepared, not exercised | Scratch manifest `rtr-cuts-4f95a2b5-20261104-f7962abeed77/manifest.json` retains exact single-file overlays, two source anchors, original/replacement/mapping hashes and unchanged live-source checks for the five approved cuts. Source-bound clean builds, semantic REDs and newly rebuilt restorations remain unrun; artifact-only native CLEANs are not substituted for them. |
| Both combined hook findings have discriminating restoration and independent clearance | `6bd61437` observes four malformed-metadata false accepts at the real commit consumer and canonical-subject exemption failures at the registered design gate. `1562e794` restores exactly the four metadata branches and all three affected canonical-classifier roots, including all18 unknown-identity negatives, with no skips. Independent D531/BFFC delta review closes both P2 findings; existing predicates, tests and oracles are preserved. This is the repair delta, not whole-spec closure. |
| Post-review hook seal and new native composition pass separately | Actual `355b5c59` observation reports source hash `ec7960bc467e66ab78e48e3346c0acdb369b9ee6f54ca71ed987d85717e16374`; only the source-literal bytes change, then its scoped restoration passes. Build `f2fc9c6b` produces native SHA256 `bc2cc14366cbe496b61532827ec0a09a59c81790ed7789948c3b0ce7100de6ad`; actual native `83d5fcf1` passes30/30. Category approval856C is unchanged. Native323's28/30 and native335's earlier-artifact30/30 remain distinct historical evidence; all fifteen-package exclusions and mutable-root limitations remain. |
| Captured publication uses its own prospective source fingerprint | The live `ec7960…16374` population includes seven foreign watched-file changes excluded from the55-path selection. `d22c786b` reuses the existing serializer through a temporary one-file compiler overlay, cross-checks that live digest, then reads185 immutable source records in fifteen scopes from prospective tree `bd3eb985dd88f1d7c9263ef28cd6e03e61ae1d61`. Its observed fingerprint is `1b44e4e255fe9cdee6327d0b3965a15c5a9498a7e5b63496443604685cfac76b`; only the expected source literal is bound to that selection. No foreign files, shared index or source-tree copy/export are used. Native349 remains live-tree evidence, not prospective native composition; a dirty-tree mismatch against the prospective seal is expected. |
| Passive original-order lifecycle diagnostic reaches every intended cause | `0856e3b8` passes the original three-test order and all four printed-verdict branches. Reports show cleanup overriding completed green stages, branch refusal before stages, green0, stage-failed1, ENOSPC log-save3 and cleanup1. All56 stage actions are injected callbacks, not whole verification. This is another nonreproduction; it does not attribute or repair `a1129486`'s earlier exit. |
| Callable-owner restoration exposes distinct stale-initializer publication | `7ae35318` passes the five new ownership roots, the two reached legacy alias/callback roots and the explicit immediate-closure control; retained-snapshot assertions now execute. Its six new mutation cases publish uncalled A rather than executed B through reused short declarations and pointer/address escape. Conservative shared-scanner repairs are source-reviewed and frozen, but post-fix restoration remains unrun. No Root-method, assembly-leaf or whole-inventory claim follows. |
| Real Ze image boots, but command identity validation still refuses | `7999f7ca` builds image `a8fcfd282e805faf`, boots under KVM and reaches ordinary-user SSH, then refuses the compound identity check before the requested command. Retained console SHA256 `4ceceb0760fdf7b3a9b5d9f98346cb222a7611b11a4e5c30764cf4854bbb3e78` independently records kernel7.3.0-ze, `/gokrazy/init` and squashfs. The failed primitive is unknown; unassigned command-exit0 is not command execution. Source review also finds missing root propagation and runtime resolution staging over the requested kernel before comparison. Those repairs remain pending; no input-preservation, scratch-writeback, PPP or RFC guest proof is claimed. |
| Existing RFC outcomes are recovered rather than repeated | Retained `cd55e29e` contains the actual third-party/foundation writer-reader RED; `488991fc` restores P23 and passes all eight named P25 dispositions. The original explicit-kernel selection root now passes both subcases in `6bd61437`; the earlier compatibility proof is not rerun. Actual GTSM clean/broken/restored/paired-solo execution on Ze's system remains owed. |
| Current whole-wrapper clock control does not reach capture | `30fc6cdd` fails current A at viewport setup with killed/deadline output, before any PNG or geometry sidecar. B remains unrun and the A/B comparison is inconclusive. The earlier whole-wrapper success is not substituted for this control; no geometry, raster or original080d attribution follows. |
| First actual authored-poster attempt requires isolated placement | `5f31c27e` fails before daemon readiness. Diagnostic-only `a99dd9f8` exposes host docker0 setup refusing permission without CAP_NET_ADMIN. No poster is captured. No host privileges, host networking, authored steps, image assertions or rendering tolerances are changed; the existing private renderer is the next execution target. |
| Current standalone renderer binaries are built, not yet browser proof | Native `6b708516` passes `site terminal-demo build binaries`. SHA256: ze `a94f301c907c3b8aed70558665305e8f4d4e518737b715baae10581f226889fe`, ze-demo `52016265594e5bcb8cac6153480273626efaa895e4d0c5ddeb48587c8421332a`, le `a9dde4e2246aff1d641c170f4a8060692c5f1813d9913532d9c4bda50e0edaff`. Actual public validator/recorder execution remains owed. |
| Existing isolated renderer refuses its absent image before execution | `077eaf09` reaches actual Engine/manifest/kernel/image preflight and refuses the missing prescribed renderer image. No container, validation journey, recording or three-WebP comparison runs. Native `site terminal-demo build image` is started to satisfy this exact prerequisite; no admission check is bypassed. |
| Required renderer image is now built | Native `64e5f4c7` completes the prescribed `site terminal-demo build image` successfully. This supplies the missing execution prerequisite; it is not validation, recording or poster proof. |
| First isolated public entry run separates validator success from recorder failure | `job-tooling-browser-real-validator-recorder-and-routed-posters-20261104b-077eaf09.log` reports the validator command successful. The actual authored Node recorder then fails at line33 Save: finite requests do not settle within the existing five-second wait bound. The three-capture subtest is not reached. No budget, authored step, image assertion or rendering flag changes; the recorder failure's cause remains unestablished. |
| Independent current browser review exposes and clears carrier-placement defects | Capture review is source-scoped CLEAN. Lifecycle review finds the new integration package unclassified, then finds that its container child cannot rely on a forwarded host session identity. The current carrier explicitly uses the existing owned Docker renderer and parent-mounted TMPDIR; the documented host invocation and justified QEMU exclusion resolve both reviewed deltas. Independent test judgment approves that written-reason contract, but its approval recording and actual poster execution remain owed. |
| First actual self-routed three-poster proof passes | `job-tooling-browser-save-idle-facts-fixed-scope-first-routed-posters-20261104b-ef74ab26.log` passes the unchanged WebP-container checks, exact equal-input byte comparison and hostname-only different-byte control. The inner test uses the normal native browser without the outer diagnostic CLI wrapper. Its retained `web-config-poster-renderer-1962626889/scratch/web-config-poster-4046265881/{first,same-input,hostname-only}.webp` payloads are actual1680×1008 captures; the first is visually inspected. The outer authored Node recorder also passes under the scalar-facts diagnostic wrapper. This is not an attribution or repair of the earlier normal-recorder Save timeout, nor of original080d. |
| Final workspace corrections have separate reviewed source and pending runtime proof | Startup failure now reads its final daemon log after the existing owned Stop/join. Relative state overrides are made absolute before private-directory/config/import-operand derivation, and setup/daemon children use the private cwd. Independent reviews clear these applied deltas. The initial build admission stops on the owned audit artifact, not compiler failure; after preserving its data and restoring only ownership changed by the run, `9b1209db` builds the final runtime. Actual relative-root busy-port RED/GREEN and normal owned-output recording remain unrun at this entry. |
| Real public-native startup diagnostic loss has a discriminating repair | `77189ba0` binds old runtime SHA256 `52016265594e5bcb8cac6153480273626efaa895e4d0c5ddeb48587c8421332a` and final runtime `4f56ea02930ccce2589a3d96fcbf82f648aed9910197bfe5d615ad5ab49caf14`. With the same owned private8443 listener and relative state-root override, the old child reaches the specific `STARTUP_CAUSE_MISSING` assertion RED; the final child preserves the actual occupied-listener cause after private workspace removal and passes. Both retain the original authored90-second deadline. |
| Final normal native recorder proves relative-root and output isolation | The separate unwrapped `TestToolingActualOwnedRuntimeRecord` in `77189ba0` passes the actual final native `run web-config record` path, all authored transitions and real WebP emission from an owned caller cwd with relative state override. The caller's audit sentinel remains byte-identical. Retain `artifacts/final-native-record-2744574168`; the earlier normal Save5s failure remains unattributed, not erased by this later positive. |
| Integration population consumer finds additional unclassified carriers | First named consumer `d8784819` reports five unexcused packages: chaos/web, component/web/testing, le/site, test/cli and test/pppoeac. Current browser and already-landed CLI host routes receive explicit documented classifications; unpublished other-phase and pre-existing foreign populations are not silently excluded. No overall coverage-gate PASS is claimed. |
| Browser integration classifications are independently approved without waiving other populations | `job-tooling-browser-classified-integration-declaration-consumer-20261104b-d8784819.log` retains only chaos/web, le/site and test/pppoeac as unexcused. The three current browser/CLI routes are explicitly documented and independently approved; the official writer records `qemu.TestEveryIntegrationPackageIsNamed` in Main's `tmp/commit-weakening-approved-4f95a2b5.md`. The overall gate remains RED for the three separately owned populations; no blanket exemption or false GREEN. |
| Permanent store-consumer regression discriminates relative-root normalization | `156a955d` retains all four original absolute alias inputs and adds four relative overrides under an owned caller cwd. A single compiler-only removal of `filepath.Abs` leaves the four absolute controls passing but makes every relative child fail its actual private-store write. Restored production code passes all eight, including the unchanged foreign-store sentinel checks. No production mutation or path-only/echo assertion substitutes for consumption. |
| Ownership review exposes shared-entrypoint effects; first repair does not pass overall | The default renderer entrypoint rewrites shared demo-home SSH config and recursively chowns shared runtime scratch. Earlier SSH contents were not captured, so preservation cannot be claimed. Protected `e1ecce0c` observes the specific shared SSH write refusing on a read-only mount; its direct-entry child passes all three WebP assertions, but the extra ten-second ownership-return container times out and the whole test fails. That extra container is removed, not given a larger budget. No guessed restore or foreign-file deletion. |
| Private-root restoration exposes the native Unix-socket pathname bound | `57944538` again observes the specific protected shared-SSH failure. Its restored existing-entrypoint/private-root route then fails before capture because the native browser socket would be107 bytes, exceeding its explicit103-byte maximum. The fixture now uses its writable artifact mount directly as the existing demo root, removing16 unnecessary pathname bytes without renaming sessions, adding browser controls or changing budgets. Independent source review clears the private-root isolation and shortened layout; actual restored three-poster execution remains pending at this entry. |
| Private-root isolation reaches real captures and returns owned files; poster gate remains RED | `0119f42f` reaches the first and same-input authored journeys and emits both WebPs under the private mount. After the normal failing child returns, both actual files are uid/gid1000:1000, mode0400, size61282 and readable with identical SHA256 `673b056da7d0671cb629c259c0e294653d0f456d21f4f387897daa4822e3decf`. No reclamation timeout recurs. The hostname-only third journey fails at line33 Save request settling and the entire test is RED, not a three-poster PASS. This exercises the reviewed root/path/ownership repair but does not attribute or fix the earlier Save failure; P-19 remains open. Forced-container-cancellation artifact readability is unproven. |
| Reviewed browser component is committed without phase closure | Exact generated-script execution `94a61fd0` publishes `553f9543af`,35 files. The component carries shared authored execution, SDK request observation, capture/lifetime ownership, private renderer placement and the four owned journal rows. Eight direct fixture approvals and the existing independent integration-classification approval are carried as trailers. Known poster/three-population reds and all historical limits are explicit; no full Phase4C/spec closure or push. |
| Scalar failure diagnostic observes a real positive pending request | `269b542d` fails the first journey at line42 Confirm Commit: observer installed=true, numeric inflight=1, document complete and quiet4946.8ms. This rules out missing observation or a negative counter for this observed failure; the request endpoint and backend cause remain unknown. Only false poll answers gain scalar facts, with no added command, retry, wait, frame or altered capture predicate. The entire diagnostic run is RED, not a normal three-poster proof or attribution of earlier Save failures. |
| First pinned clock attempt is a different root, not the frozen open-page control | `229b1864` runs current `TestChaosCaptureUsesControlledProducerClock` (source6827), observes unequal same-input PNG bytes, then cannot retain diagnostics because the child has no owned session root. This is not frozen95B3 `TestChaosOpenPageUsesProducerClock`, whose1700/3300ms waits were absent. The invocation's intended-target label was wrong; no frozen-root, geometry, raster or original080d attribution follows. The exact95B3 identity and existing Main-session prerequisite are recovered before another clock execution. |
| Request-metadata diagnostic passes commit but exhausts the original case budget later | `212a7363` reaches line46's authored wait, then the original90-second case expires. It does not reproduce the five-second settling refusal or identify that earlier pending endpoint. Retained private audit `3ED5` records the actual web config commit successful; joined private daemon log `7830` is retained. The diagnostic preserves the three-journey order/counters/deadlines but deliberately retains workspaces after unchanged Stop/join, so it is not cleanup/restoration proof. No timeout increase or failure attribution. |
| Exact frozen open-page root now retains its failing pinned-renderer evidence | `5d65e311` executes unmodified95B3 `TestChaosOpenPageUsesProducerClock` from the already compiled binary, in a distinct private pinned-renderer placement with the actual Main identity and read-only real ownership marker. Its original1700/3300ms waits,30/35-second boundaries,1280×900 inputs and byte/HTML oracles are unchanged. The fixed-input pair fails; producer advance also captures. All three actual PNGs are retained under `web-config-poster-renderer-405578849/demos/terminal/artifacts/scratch/chaos-live-clock-2420962463`. Both fixed-input PNGs are1280×900 and differ at exactly28 RGB pixels, bounding box314,86–1268,446; exact coordinates/values are retained in `clock-pinned-private-renderer/actual381-pixel-diff.json`. The page and crops are visually inspected. DOM geometry and the cause of those corner/control-edge changes remain unproven; original080d and prior host controls remain distinct. |
| Foreign IPC vet error is not clock evidence or a browser patch | `5d65e311` also prints `filter_batch.go:142` unresolved `filterBatcher.exchange` during vet, although the frozen child runs and fails its own image oracle. Current source is a concurrent split: `filter_batch_bytes.go`0EF7 now declares that method without build constraints; the stale workspace symbol still pointed into the old file. No foreign source is edited, no vet is disabled and no green wrapper composition is claimed. The next new Save diagnostic uses the present split source rather than retrying the old clock check. |
| Passive step trace stops at a distinct first-navigation error | `979545a5` reaches the browser with86.669s of the original case remaining, then native first open fails with `net::ERR_SOCKET_NOT_CONNECTED` at line13; Save and request-settling diagnosis are not reached. Owned daemon log `A0AA` shows startup/listening followed by normal joined shutdown, with no TLS cause. The owned native-browser directory retains only engine/pid/version files, not a network log. No blind retry, endpoint attribution, timeout change or browser override follows. |
| Written clock polling repair is not yet a verified pixel fix | The source owner changes17 periodic polling sites, initial and returned markup, to bundled HTMX4 `innerMorph`/`outerMorph`; ordinary click/navigation/SSE modes, CSS and frozen95B3 remain unchanged. Old polling paths replace unchanged nodes; native Morph has existing unchanged-node/text/attribute retention. Source review and exact expected-output judgment are independent and pending. Goldens/tests are not edited; fresh candidate rebuild plus the unchanged root is running. No repaired-pixel or historical080d claim yet. |
| Unchanged open-page clock proof passes after the polling-mode repair | Fresh current-source binary in `f19a37fa` passes exact95B3 and its outer private renderer invocation. The same original case/outer deadlines,1700/3300ms waits,1280×900 viewport, fixed20s/15s expectations, exact PNG-byte equality and40s/35s producer-only advance remain mandatory. Before `5d65e311` was RED; after `f19a37fa` is GREEN in the same owned pinned placement. Independent polling-source review is CLEAN. This proves that current open-page path, not universal screenshot determinism or original080d attribution. |
| Separate three-session PNG consumer still has a retained four-pixel mismatch | `34e69c0b` runs only older6827 `TestChaosCaptureUsesControlledProducerClock` against the fresh383 binary and remains RED. Actual first/same/advanced PNGs are retained under `web-config-poster-renderer-4050669302/demos/terminal/artifacts/scratch/chaos-clock-capture-2097851314`. All are1280×900; the first/same pair differs at exactly4 RGB pixels, bounds355,127–386,189. Exact values are retained in `clock-pinned-private-renderer/actual384-old-consumer-pixel-diff.json`. This is distinct from the green same-page root, the earlier28 pixels and original080d; cause and geometry remain unproven. No oracle, mask, tolerance, CSS, timing or browser-flag change. |
| Independent clock maintenance preserves stronger exact contracts | Official Main ledger carries cumulative NativeMorph/051E authority for the three golden consumers and the two specific polling/event units. The root-opening-only OOB proposal is refused and remains unapplied; exact A78F instead requires complete nonempty producer uptime spans in both responses, the actual stats stream-root target and whole-response OOB counts1/2. `c88d12ce` passes independent-render determinism, fidelity, grid polling and strict event integration; only two timeline golden subcases fail at fixed-clock boundaries. This is test maintenance, not a product fix or PNG-oracle approval. |
| Two exact fixed-clock boundary expectations are restored without rerunning passing consumers | Read producer window/percentage/tick arithmetic justifies50% midpoints,100% endpoints and the inclusive2h59m0s left tick. Independent087F approval covers only four literal rules, five sites per file, in two named goldens plus one shared explanation on those two existing fidelity keys. `5023e1c2` passes only the two previously failing golden subcases and the fidelity validator whose explanations changed. All other385 passes are reused, not repeated; no clock shift, normalization, tolerance or wholesale refresh. |
| Broader producer-clock review finds a separate broadcast snapshot race | Source review identifies `broadcastDirty` sampling time before its state lock and later rendering stats/events from a second live-state lock window. A concurrent newer event can therefore be rendered against older `now`; actual `formatElapsed` preserves negative milliseconds. The single P2 is being repaired at the snapshot boundary; other reviewed clock consumers, ordering and ticker teardown have no additional concrete findings. No verified-fix or negative-age runtime proof is claimed yet. |
| Broadcast snapshot repair clears independent review and changed consumer paths | `broadcastDirty` now takes one state write lock before `Now`, through dirty consumption, rendering, existing nonblocking broker enqueues and transient clearing. Independent delta review closes its P2 with no new concrete findings or lock-order cycle; slow-client HTTP writes remain outside that path. `4999689d` passes the four changed broadcast/ticker/HTTP-SSE/peer-order roots, then builds a fresh final clock binary and passes unchanged95B3 plus its private-renderer wrapper. No controlled reproduction of the old interleaving was added: that concurrency-coverage limit is retained, rather than reporting an unobserved race RED. Prior exact-golden passes are not rerun. |
| Clock host classification removes only its own coverage gap | Exact29FF host-route proposal is independently approved and recorded cumulatively with the prior three browser/CLI routes. The documented clock invocation keeps both integration tests and their exact PNG assertions, requiring native browser dependencies and an actual owned session root. `6e62e4f1` still fails only for `internal/le/site` and `internal/test/pppoeac`; those populations remain owed in their approved phases. No overall coverage-gate PASS, guest-browser claim, four-pixel waiver or full Phase4C closure. |
| Clock component is published after the external build prerequisite is repaired | Official preparation `4bf6e52b` captures44 sources with known limits and six independent approval trailers. Exact script execution `148e8637` first refuses before publication because a foreign reactor import is unused; that source is left untouched. After the foreign file changes fromA152 to2EBD and removes the import, the same printed script publishes `368d42ae36`,44 files. No bare-launcher HEAD export, shim, bypass, repeated browser check or push is used. The component lands; full Phase4C and its remaining capture/Save evidence obligations stay open. |
| Frozen callable-mutation repair passes its previously unrun restoration | `df14bc51` runs current F5C9 semantics and passes all six plugin/YANG mutation cases for reused short declaration, value-spec pointer alias and helper-parameter address escape, the three shared declaration-input mutants, and reached alias/callback/immediate-closure controls. Actual refusals name mutated or escaping callable ownership rather than publishing uncalled A. Earlier five owner baselines are reused, not repeated. No whole inventory/site, native completeness, rejected normal-return design or assembly-v3 claim follows. |
| Old kernel-library API reproduces input mutation and missing output ownership | First baseline `e36477e2` fails both new regression roots: warm resolution replaces the requested fixture kernel bytes and changes0600 to0644; all three cold cases pass Root="" and relative shared `tmp/kernel/build` to the actual default builder boundary, mutate the requested input and miss owned cleanup/retention context. Tests substitute only the existing native backend on cold paths; no real kernel/image/guest is built. Production remained unchanged for this RED. The staged root/ownership repair is now released, with assertions intact; no fix or original339 identity attribution yet. |
| Tiny committed corpus reaches the actual native inventory command | Current named working-source nativeSHA11ae4f532f7bde5c3e607cf70e150310ccf6cb7f3644932e05150ae322856a70 records Go1.27.3, ze_le/all feature tags and revision1df2a04+dirty after392. New throwaway-only overlay executes its actual `repo inventory` with the JSON pipe from a separate empty cwd and canonical owned fixture root. `c907b110` first stops before inventory at the correctly enforced inherited build-name mismatch; producerE617 confirms the cause. Correcting only the harness name/LE_BIN gives `186c611f`: four exact direct/named-helper plugin/YANG reports, then six exit1/empty-stdout refusals naming caller, ownership and mutation/address cause. No assertion, budget, frozen producer or product change; earlier owner tests are not rerun. Carrier4005/overlay11E1 and source fingerprints remain retained solely for this proof. Narrow publication is still blocked by5354's uncommitted evaluator/registration/assembly prerequisite closure; this is not full Root, assembly-v3, site, source-completeness or phase closure. |
| Kernel-library ownership restoration passes before any real guest replay | `76804da0` passes the two actual393 regressions and four new appliance behavior roots: warm requested bytes/mode remain intact; cold publish, requirement refusal and builder error use owned selected-root output; opposing decoy manifests cannot determine enforcement; equal inputs retain prefix-independent cache identity; all18 warm/cold/default/internal-override CLI-handler staging/cache controls pass. The approved0136 cache-refusal test preserves all three stages and now checks selected-root data, empty key and typed config cause/path instead of repair prose. Only the existing native backend is substituted; no real Go/kernel/image/guest command is proven. Independent staging review is CLEAN, with artifact-absence, cache-publication-error and cleanup-error fault injection still unexercised and no locking/no-clobber claim. |
| Root-chain review catches and reproduces an unnecessary argument rewrite | Independent P2 identifies unconditional `absoluteParentDirArgs` rewriting consumed flag values and tokens after `--`. New throwaway `e0a73c3f` observes both corruptions on the actual old helper; genuine separated/equal parent controls pass. Main reads `ExecuteGok`/`keepWorkingDir`: entry stays in invocation cwd, restoration occurs only afterwards. The helper and its sole Run call are cleanly removed, retaining existing building-parent and modcache normalization; fix-only independent review accepts the source delta. The old-helper carrier is retained as RED evidence only, not rerun after deletion or treated as post-fix proof. Actual Run/editor consumer smoke and stronger nested-root test judgment remain pending. |
| Previously unanswered foundation roots now have exact execution witnesses | `7bafbe40` selects only E252's ten gaps and passes all ten, including first recovered restoration of the actual3533f51f full-report RED, standalone module reachability, duplicate schema identity, conflicting Darwin/ARM64 context normalization and six independently approved changed Collect roots, including all three unreadable-area subcases. Prior callable/mutation/header/assembly/native controls are reused rather than repeated. No full Root, selected-tree, site or arbitrary native-body support claim follows. |
| Nested source roots and the real mutating editor consumer pass after the clean deletion | `472c77b4` passes all four D535-approved behavioral roots: native/foreign preparation reads the selected missing manifest; Run preflight and both Docker-host routes refuse selected rather than decoy source inputs. These replace callback/root-equality echoes with real path/cause/effect checks. Its separate bounded throwaway consumer executes actual Run, gok edit and an execed fixture editor with real pinned Go version metadata: both literal `--parent_dir=lab` and ordinary instances are edited under invocation-relative parents; source-root decoys remain unchanged. Current-source native build `d037e62d` also passes. No kernel/image/guest command, actual339 identity attribution or broader platform closure is inferred; passing396 appliance roots are not rerun. |
| Standalone reclamation reaches the actual native command without unrelated setup gates | `f1a4edc8` runs named native observed asSHA55c5825ea8aa42ddde188e3d0bde8a63cc67124a464bb95a0c7334fda7bca36d against only an authored private Git administrative fixture. Actual `reclaim worktrees` JSON returns matching exit/report0 and accounts for both locked/dead directoryless removals while preserving live/outside registrations and sentinels. Malformed and unreadable ownership then yield matching exit/report1, meaningful cleanup causes and unchanged protected state; no verification stages or job admission appear. Full stdout/stderr/state and results remain in `cleanup-phase6a-native-a4tzulbs`. The commit launcher later rebuilt that mutable named path asSHAdda803eaaeaf043f89d6e4a99442708d85293047252d0e50018ee2b73e09d7a1; an immutable copy of the earlier55c image was not retained, so the recorded image identity is historical, not the current file. Normal startup remains intact; successful stdpatch activation is not a reclamation prerequisite and is not claimed. Retained10dd/720/0fc/342 proofs are reused. Original304 remains unattributed,319 remains nonreproduction,342 uses injected actions; guest/VM/six-route and whole-verification success remain unproven. |

The original missing artifacts, unattributed pixel mismatch, unverified native
command/kernel/PPP workflows, original live-checker obligations and foreign CodeQL
missing-file limit remain unchanged. No push or deployment occurred.

### Continuation on 2026-11-05

The following logs are retained under
`tmp/session/2026-11-05-001121/scratch/`. Main ran all builds and tests through
job admission; subagents performed source work only. These results do not close
the historical evidence obligations above.

| Observation | Evidence and limit |
|-------------|--------------------|
| Approval companions published | `c3209670ec` lands the four previously untracked approval identity/publication companions, reusing their retained scoped proofs. |
| Commit publication and prospective content | `job-commit-publication-missing-proofs-4deff013.log` passes all ten selected roots under race instrumentation and full feature tags. The new replacement/append test proves distinct message ownership, exact publication order and one legitimate Git-derived obligation despite a foreign legacy shard. It lands in `efe2fad70b`. |
| Canonical ASN text parsing | `job-asn-element-boundary-baseline-e2101eb5.log` passes the existing AS-absence verdict and new numeric/text boundary controls before the parser replacement. `job-inventory-and-asn-repair-scoped-8f76a6fc.log` passes those same roots after the fix; that mixed job is red in inventory. The duplicate numeric-parser pattern no longer matches the reached source. `6325dbe70d` publishes only the ASN hunks and new test, preserving other route-checker edits. |
| Inventory mutation policy | `job-inventory-mutation-policy-1eeb0701.log` passes all six selected mutation/owner roots with race instrumentation and full features. Earlier public Collect migration proof used only ze_le tags and remains narrower. |
| Additional inventory omissions reproduced | `job-inventory-registration-loss-baseline-f66d901a.log` observes stale metadata through a ValueSpec pointer alias and missing plugin/YANG installers inside registration arguments. The repair run `8f76a6fc` passes both new roots and reached callable/closure controls, but fails two existing family-helper cases at unresolved method ownership. The foundation remains uncommitted and incomplete. |
| Browser Save endpoint identified | `job-browser-save-request-attribution-a76c95a5.log` completes the first journey, then the second fails at Save: POST /config/form/ remains pending for 4914ms. Retained daemon logs do not identify a blocked handler. |
| Instrumented poster journey passes without attribution | `job-browser-failed-save-owned-stacks-c8567278.log` passes all three journeys, equal-input image equality and the hostname negative control. Its failure-only owned SIGQUIT hook never runs, so it yields no stack attribution or timeout repair. Artifacts remain in this session's web-config-poster-renderer-2818059241 directory. |
| Native editor capability admission | `e4eef31a2d` publishes the companion. `job-editor-native-capability-positive-073e5e3f.log` passes TestEditorSuiteNativeCapabilityTarget through the registered editor command. The owned user namespace reports the required effective capability; the authored dirty assertion and post-join work cleanup pass. This proves admission and editor execution, not privileged host-network operations. |
| Canonical RFC disposition accounting published | `37cf1d1d03` publishes the declined-index writer, four accounting companions and selected documentation. `job-retained-method-baseline-and-accounting-99ccaf5e.log` passes the four selected RFC roots under race instrumentation and full feature tags; inventory fails independently in that mixed job. No own-system guest or explicit per-kind header-count assertion is claimed. |
| Concrete receiver restoration | `job-retained-method-restoration-kernel-baseline-ca2c40c9.log` passes constructor, defining-package, interface/unsupported-owner and registration API collision controls, plus both registration omission regressions. Range controls and flowspec projection fail. After correcting the parser's synthetic range binding, `job-retained-range-kernel-payload-restored-fc37fb3d.log` passes all four selected inventory roots, including every range and family-helper case. This does not establish full-root inventory support. |
| Kernel payload drift reproduced and restored | The same `ca2c40c9` baseline passes identity diagnostics and sudo PID1 checks, but both new kernel payload roots fail: QEMU consumes changed requested bytes and starts after a pre-acquisition replacement. After the private streamed and hashed payload fix, `fc37fb3d` passes all four selected QEMU roots, including caller preservation and refusal cleanup. Real guest execution remains owed. |
| Chaos screenshot difference narrowed | `job-chaos-capture-geometry-runtime-ee2dff7d.log` fails original equal-input PNG equality. Main's decoded comparison finds two differing pixels at (374,188) and (385,188), within a rounded remove control. Observed geometry agrees except quiet timing. This does not identify a raster producer or establish a repair. A diagnostic with one additional capture per existing session built successfully in `job-chaos-capture-pair-build-7589221c.log`; its runtime result follows below. |
| Pixel drift also occurs within a browser session | `job-chaos-capture-pair-runtime-c958ece0.log` fails the original equality assertion and retains all six PNGs. Main's decoded comparison finds six differing pixels between the first session's captures; the same-input and advanced-clock sessions each have zero within-session differences. The two original equal-input images differ at (355,127) and (355,168). Adjacent observations show renewed request activity after readiness. Neither request activity nor rounded-control geometry is yet an attributed raster cause. |
| Actual-root inventory remains incomplete | `job-inventory-current-root-once-0ff56994.log` reaches TestAnswerAnswersTheInventory with the current producer and committed main inputs, then refuses internal/analyze/convert.go:21. Source tracing identifies the direct retained pointer method (*subdispatch.Dispatcher).Register. This is a current pointer-method boundary, not evidence of the previously observed err.Error failure or a successful report. |
| Native kernel and own-system command replay | The native host-driver build succeeds. `job-own-system-runtime-kernel-stage-475f79dd.log` reports kernel ready at the keyed 7.3 runtime amd64 Docker-backend cache tree. Image preparation reuses key baba9c16fe28d14c. `job-own-system-command-real-replay-a4c53cfc.log` passes TestDockerLabCommandRunsInZeSystem: ordinary UID1000, exact PID1 executable, squashfs root, runtime release, no Docker socket, readable inherited job token, exact private scratch writeback, and VM shutdown. The printed hvf:kvm:tcg is a candidate list, not proof of selected acceleration. This current replay does not attribute the original November 4 identity failure or complete RFC guest discrimination and six-route cancellation. |
| Validated kernel payload published | `2694c76025` publishes the private payload producer, acquisition regressions and selected documentation after the scoped and real guest proofs. Concurrent `934e7d8c03` already published the shared transport, so this commit carries only the payload delta. |
| Pointer-method compatibility baseline | `job-retained-pointer-method-baseline-168ccdbc.log` refuses the imported pointer and range positive controls and all four pointer API-shadow cases at unresolved receivers. All sixteen unsupported-owner controls pass. The reviewed narrow direct-pointer implementation is now authored; restoration remains pending. |
| Lexical registration guard discriminated | `job-inventory-import-guard-discrimination-be37a8d2.log` uses a one-file overlay restoring only the old raw-import-alias classification. All four concrete API-shadow cases fail with incorrect registration argument refusals. Live-source equivalents passed in `fc37fb3d`; no live producer was reverted for this observation. |

The inventory foundation is published in `5a6bd238c1`. Pointer-method
restoration passes in `job-pointer-restoration-rfc-transport-baseline-3a5de1f2.log`.
The next actual-root refusal is `internal/appliance/homebrew.go:82`, on
`info.IsDir()` after `os.Stat`. The owner chose to retain Git-only inputs and
design a general external-effect proof. That design is in progress; no external
source population or standard-library exemption is authorized. This is not a
successful full-root inventory report.

The RFC unit transport passes four scoped roots with full feature tags and the
race detector in `job-rfc-own-system-unit-transport-restored-258e3840.log`.
`job-rfc5082-ipv4-own-system-record-d89cbcf5.log` then runs the exact IPv4 GTSM
unit in the own-system guest: clean PASS, disclosed hand-written metric-zero
break FAIL at route readback, zero skips and owned VM shutdown. The kernel
SHA256 is `329167c42bdff84d8cba8f3e1f5bdfadef22611836ea1da5e6748b8196f5b750`.
This failure observes the missing installed metric, not a non255 emitted packet;
the broken run does not reach the packet assertion.

The corresponding IPv6 observation passes clean and fails at the metric-zero
route readback in `job-rfc5082-ipv6-own-system-record-correct-id-22fa20b7.log`.
Clean compilation succeeds in `job-gtsm-restored-packet-build-f1e9bc73.log`;
`job-gtsm-restored-and-negative-packets-36ead9b2.log` then passes both restored
positive units and both unchanged no-metric negative packet units in the same
native guest, with zero skips and owned VM shutdown. Commit `e4bfeb7789`
publishes the transport, placement regressions, documentation and both exact
unit records. This completes those GTSM observations, not the separate paired
proof or the whole phase.

The owner approved retaining every GTSM check while moving P-39's paired-break
carrier to a separate native AS_PATH test using existing redundant guards.
The concrete proposal is retained in
`tmp/session/2026-11-05-001121/scratch/phase8-p39-paired-carrier-design.md`.
`job-rfc4271-native-paired-record-a36e0947.log` now proves the exact new AS_PATH
unit clean PASS, combined FAIL with an actual added AS_PATH and changed
withdrawal bytes, and both solo PASS. The advertisement control remains green
under the combined break. Clean build `4e555aeb` and native restored run
`job-aspath-restored-native-packet-d3b5ecfe.log` pass both subtests with zero
skips. All five VM runs stop their owned VM and retain the same kernel identity.
Commit `cd0b42213c` publishes the test and exact native paired record, preserving
the earlier records. These are hand-written breaks with independent judgment;
the observation is full relay dispatch bytes, not a captured network packet.

The next clock paint diagnostic, `job-chaos-capture-paint-trace-runtime-7d49a503.log`,
fails during first navigation after observer registration, before capture or
trace start. No pixel or paint cause follows from that result. Its retained
renderer is `web-config-poster-renderer-2106760395`; bounded navigation and HTTP
observations are prepared for the next discriminating run.

The cache-session search recovered the November fixture results in
`tmp/session/2026-11-03-231620/scratch/original-tooling-ui-live-results.json`.
It did not recover the damaged-binary archives, original post-OSPF lint
population, June inventories or authentic October naming archive. A later
fixture failure is not attribution of the earlier incident.

### Git-only evaluator continuation on 2026-11-06

Evidence availability changed later on November 6. The earlier session's raw
logs, profiles, omission overlays and native captures became unavailable during
an automatic cleanup run. Its log does not identify every removed directory.
The results below remain observations recorded in the conversation, but their
former scratch paths no longer supply the original bytes. The
[recovery record](tooling-one-off-evidence-recovery-2026-11-06.md) identifies the
surviving source snapshots, preserved cleanup log and private archive for new
retained evidence. A recreated session directory does not recover earlier proof.

The earlier design-in-progress inventory statement is superseded. Thomas
approved the Git-only external-effect design and its explicit bounded trust
contract. The evaluator is implemented in the working tree, including retained
caller provenance, public-entry and cycle certificates, and exact field ownership.
It does not inspect or copy host standard-library sources.

MAIN observed and repaired callback ownership loss through a channel receive.
The source-selected inert control passed before the repair, while the received
whole value incorrectly reused its earlier inert field. The unchanged regression
now passes. Four additive type-identity controls also pass, covering defining
files, lexical shadows, ambiguous package names and receiver depth/refusals.
Two lexical controls preserve synthetic-span containment, exact bindings and
fresh scope identity. Two flattening controls preserve nested callback effects,
scalar-field precision, cutoff behavior and output ownership.
Their canonical tests are `TestExternalReceivedStructFieldSolver` and
`TestExternalTypeIdentity*`, `TestExternalLexicalParameterIndex*` and
`TestExternalFlat*` in `internal/le/repo/inventory/`.

The allocation repair also passes `TestExternalJoinMergeCompatibility` and
`TestExternalJoinEmptyAllocationBudget`. The eleven source-selected roots passed
before the terminal change (historical run `d137f4b9`). They cover merge ownership,
unknown order, empty-input allocations and the earlier source controls.

The terminal guards in `externaleffects.go` `externalEffects.refuseUnknown` and
the reached expression, call, binding, entry and literal producers have independent
source review. Expansion stops only after the ambient pool has a permanent
unknown. Local unknowns and provisional failures still receive ordinary analysis;
the existing issue ordering preserves any currently recorded preferred issue.
The three former red refusal roots pass: `TestExternalTerminalUnknownStopsCallbackExpansion`,
`TestExternalTerminalUnknownDiagnosticPrecedesUnvisitedUnsupported` and
`TestExternalTerminalUnknownPreservesRecordedUnsupported` (historical restoration
run `13ef966d`). The following broader run also passes both valid survivor roots
and all eleven prior source controls, sixteen roots in total (`ea039571`). It
includes no signature tests.

MAIN used native job admission for this passing child command. The feature list
was read from `feature-gates.txt`, prefixed with `ze_le integration`; race testing
explicitly set `CGO_ENABLED=1`.

```sh
go test -race -tags "$tooling_test_tags" \
  -run '^(TestExternalTerminalUnknown.*|TestExternalJoin.*|TestExternalFlat.*|TestExternalLexicalParameterIndex.*|TestExternalTypeIdentity.*|TestExternalReceivedStructFieldSolver)$' \
  -count=1 -timeout=3m -v ./internal/le/repo/inventory
```

Separate signature baselines found twenty-two unsafe admissions. The five paired
scenarios in `external_signature_solver_test.go`
`TestExternalSignatureTypeCapabilitiesSolver` have five passing scalar controls
and five incorrectly admitted exposed signatures (`83f1b0d2`). The thirty-two
cases in `external_signature_authority_solver_test.go`
`TestExternalSignatureAuthoritySourceSolver` add seventeen unsafe admissions and
fifteen passing controls (`de0f8bb2`). These source-selected cases retain real sink
witnesses and metadata. The repair separates reflected type authority from
executable callable ownership. MAIN restored both complete roots (`8810cb05`):
all twenty-two formerly unsafe cases refuse, and twenty controls retain their
expected outcomes. Under native job admission, both canonical roots can be
selected together with this child command:

```sh
go test -tags "$tooling_test_tags" \
  -run '^TestExternalSignature(TypeCapabilitiesSolver|AuthoritySourceSolver)$' \
  -count=1 -timeout=3m -v ./internal/le/repo/inventory
```

The six roots in `external_reflection_domain_test.go` `TestExternalReflection*`
passed (`dbbe5fe7`). Generic nil-interface precision also has a discriminating
record: a single-file overlay of the first descriptor implementation gives
`nilAlias` and the direct instantiated nil interface unwanted reflected methods,
while `withOpaque` retains its expected methods (`d820c95d`). The corrected live
`TestExternalReflectionDescriptorBoundaries` passes those cases (`b511bc67`).
Independent source review found the revised implementation clean within that
scope. The earlier sixteen source roots also pass after the signature repair in
`inventory-signature-prior-source-controls-ea039571`, a separate job from the
terminal run above. Their canonical selection command is unchanged.

The `external_signature_return_solver_test.go` baseline found seven
unsafe admissions (`e10a8ff7`). `TestExternalSignatureReturnedValueAuthoritySolver`
lost declared return-type authority for direct and inferred typed nil functions
and pointers, a function alias, an instantiated function returned by a
nongeneric producer, and a named scalar constant. The same root's scalar and
inert-result controls pass. `TestExternalSignatureReturnedNilCallableKeepsOwnership`
also passes both controls, preserving refusal when a nil result is invoked.
Across the two roots, fifteen control leaves had their expected outcomes and
seven unsafe returned-value leaves failed. Their canonical child selection is:

```sh
go test -tags "$tooling_test_tags" \
  -run '^TestExternalSignatureReturned(ValueAuthoritySolver|NilCallableKeepsOwnership)$' \
  -count=1 -timeout=3m -v ./internal/le/repo/inventory
```

The return repair is implemented in `externalreflectionreturns.go`
`externalEffects.returnedReflection` and its normal/assembly siblings in
`externalcalls.go` `externalEffects.call`. It supplies only declared Results
type reflection authority after the original retained-call ownership checks and,
when applicable, assembly certification. Callee parameters, receiver and actual
callable ownership are unchanged. MAIN restored the seven unsafe leaves in the
targeted run (`953a614c`); independent review found the actual change clean within
that scope.

The full affected selection then passed all twenty-six roots with full feature
tags, race instrumentation and a three-minute test bound (`12c3c7f1`). That covers
all four signature roots, with twenty-nine restored unsafe leaves and thirty-five
control leaves at their expected outcomes, all six reflection-domain roots and
the sixteen prior controls. No existing test changed. Its canonical child
selection is:

```sh
go test -race -tags "$tooling_test_tags" \
  -run '^(TestExternalSignature.*|TestExternalReflection.*|TestExternalTerminalUnknown.*|TestExternalJoin.*|TestExternalFlat.*|TestExternalLexicalParameterIndex.*|TestExternalTypeIdentity.*|TestExternalReceivedStructFieldSolver)$' \
  -count=1 -timeout=3m -v ./internal/le/repo/inventory
```

The source-reviewed closure and assembly-sibling tests are now live in
`external_signature_return_siblings_test.go`, and MAIN's formatting check passes
(`77d127b0`). Their old-hook overlay baseline preserved both scalar controls and
exposed both unsafe returned-value cases (`d2887897`). The unchanged live
`TestExternalSignatureReturnedClosureAuthority` and
`TestExternalSignatureReturnedAssemblyAuthority` pass all four cases (`3854cfb3`).
The assembly root uses the supported direct-JMP source certificate and a named
scalar result. It proves source-certificate/result composition without native ABI
execution. These tests were promoted after the first reflection-repaired
actual-root binary had started, so they are outside that earlier run's inputs.

The last observed pre-terminal run of `inventory_test.go`
`TestAnswerAnswersTheInventory` reached its unchanged ten-minute bound. Completed
profiles identified repeated mutation, declaration-owner and lexical type-parameter
scans, then cumulative
allocation in value merging. The indexes, flattening accumulator and allocation
repair are implemented. The last native command, after the allocation repair and
before the terminal guards, ended at its fifteen-minute diagnostic deadline:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=0 \
./le --name tooling-resume-01a1a907 job run \
  label inventory-native-after-join-allocation quiet command \
  timeout 15m ./le --name tooling-resume-01a1a907 repo inventory
```

MAIN observed exit 124, an empty log and no report (`6735622a`). The named launcher
compiled the working evaluator, including the allocation repair. The canonical
command is registered by `register.go` `init` and reaches `inventory.go` `Answer`.
The first `inventory_test.go` `TestAnswerAnswersTheInventory` attempt after the
reflection repairs also timed out at its unchanged ten-minute bound under normal
`CGO_ENABLED=0` execution (`4a8b7bef`), without producing an inventory report.
The exact test binary, runtime backup, CPU/memory profiles and original
inventory-package source SHA256 manifest are retained. All originally recorded
source hashes still match, and HEAD remains
`dc3514eaec71d5c8ab9d487e40a3a09e66d8f73b`.

That bound CPU profile points to repeated reflected-method, receiver, definition
and package type-binding lookups. The allocation profile samples 21.67 GiB of
cumulative allocations, including reflected-union merging and method or recursive
summary construction; this figure measures allocations over the run. These
profile attributions led to the next source repair.

The completed package-binding tables in `externaltypebindings.go`
`externalTypeIdentityIndex.indexPackageBindings` preserve scope map keys, exact
object identity, non-type shadows, nil objects and sticky ambiguity. All four
lookup consumers retain their original lexical/import precedence and the pure
fallback for an unindexed package. The collector in `externalreflectiontypes.go`
`externalReflectionWalk.value` gives each query a fresh output and collects into
it through recursive descriptor edges. It preserves sibling traversal, source
order, active-path checks, the 128-edge bound and the original unknown-only
invalid-argument result. Independent review found the actual producer clean
within this scope. No receiver-answer cache or descriptor DAG pruning was added.

Four additive roots in `external_reflection_cost_solver_test.go` cover descriptor
allocation, package binding compatibility, output independence and parser-scope
edge cases. Their old-source baseline preserved all compatibility expectations,
while `TestExternalReflectionOutputUnionAllocationBudget` failed with nine
allocations for the shallow descriptor and thirty-one for the nested descriptor
(`a44cdc9a`). The unchanged allocation test passes with three allocations for each
descriptor (`89bbc47e`). The full affected selection passes all thirty-two roots
with full feature tags and race instrumentation (`7200088d`): the earlier
twenty-six, both returned-value siblings and the four new roots.

The second actual-root attempt, including the package-binding and collector
repairs, still timed out at the unchanged ten-minute bound (`22466500`). The
inventory-package source/config manifest
`inventory-root-after-binding-union-source.sha256` matched after the run. The
exact compiled test binary and CPU/allocation profiles are retained. The CPU
profile attributes 62.50% of samples cumulatively to
`externaleffecttypeidentity.go` `externalTypeWalk.receiver`, and 84.92% to
`externalreflectiontypes.go` `externalReflectionWalk.collectMethods` (`a015ff29`).
The allocation profile samples 6550.01 MB of cumulative allocations (`cf499b42`).
That total measures allocated bytes over this incomplete run. It establishes
neither peak memory use nor an end-to-end performance comparison.

The receiver-query repair is now implemented in `externalreceiverquery.go`
`externalTypeWalk.receiver` and independently source-reviewed. One private table
per effects reader stores only completed declaration/generic/refusal tuples for
exact selected receiver sources and incoming depths zero through 127. Synthetic,
unindexed and exhausted queries retain the original resolver. The raw algorithm
and both method censuses remain unchanged. No capability, ambient or active-path
answer enters this table.

The old-source `external_receiver_query_test.go` baseline failed
`TestExternalReceiverQueryAllocationBudget` with five allocations for the repeated
query against zero for its tuple control (`10376f53`). Both
`TestExternalReceiverDepthOutcomeCompatibility` and
`TestExternalReceiverSourceIdentityCompatibility` passed. The producer was frozen,
formatted (`c32ba899`) and independently source-reviewed clean. The unchanged
allocation test now passes with zero allocations for both the repeated query and
tuple control (`e0452074`). All thirty-five affected source roots pass with full
feature tags and race instrumentation (`38737907`).

The additive `external_receiver_fallback_test.go`
`TestExternalReceiverRawFallbackCompatibility` passes all four controls
(`7371e197`). Removing only receiver-source eligibility wrongly reuses stale
declarations for synthetic and unindexed sources (`14071fe2`). The unchanged live
selection passes this root and the three receiver-query roots, with every block
in `externalreceiverquery.go` executed (`bdfece96`).

The third actual-root attempt after reflection repair still timed out at the
unchanged ten-minute bound (`e49d23b5`), without an inventory report. MAIN checked
all 147 source/config manifest entries after the run and confirmed the same HEAD.
The matching saved-binary CPU profile still attributes 84.80% of samples
cumulatively to `externalReflectionWalk.collectMethods` and 63.07% to
`externalTypeWalk.receiver` (`37c9106c`). Map access accounts for 57.90%
cumulatively. These overlapping attributions describe the incomplete run and
do not establish an end-to-end performance comparison.

The next method-census repair is under source-only design. No new producer
implementation is released, and the receiver-query increment remains frozen.
The passing source controls do not establish a complete inventory report. Native
inventory and canonical site publication remain unproved.

Native GoBGP validation also exposed automatic Docker cleanup that could remove
resources from a different run. The repair is implemented and independently
source-reviewed in `interop-ownership-actual-source-review.md`. Startup now uses
dry inspection, name reclamation requires the exact creating process identity,
and signal cleanup requires an inherited launch identity plus the ordinary owner
checks. Each child retains its separate process run identity. The independent
test judge approved the actual existing-test changes. MAIN restored all five
previously red ownership roots with full feature tags and race instrumentation
(`f80a5b11`):

- `automatic_ownership_test.go` `TestSuiteRunPreservesOtherRunsResources`
  preserves foreign containers, networks and tags across all three dead-owner
  states, while the owned scenario still runs and cleans its own resources.
- `automatic_ownership_test.go` `TestPreCleanPreservesDeadOtherRunsNames`
  preserves both foreign container and network names.
- `signal_ownership_test.go` `TestSignalSweepPreservesOtherRunsResources`
  preserves a different launch's resources.
- `signal_owned_launch_test.go` `TestSignalSweepPreservesResourcesWithoutLaunch`
  preserves resources with no launch attribution.
- `signal_owned_launch_test.go` `TestDockerChildInheritsOwnedRunIdentity`
  proves inherited launch attribution through an actual hermetic child process.

The broader affected run passed forty-five roots but failed
`suite_sweep_error_test.go` `TestSuiteRunOrphanSweepFailureNamesWhatWhyAndFix`
because the startup diagnostic omitted the registered inspection retry command
(`28e428b0`). The producer now names `./le test integration clean dry` after the
daemon refusal and before the suite retry. The unchanged diagnostic root passes
(`b8fe758c`), and independent source review found the narrow correction clean.
The selected count-coverage run also passes (`901539bf`). The reject-before-launch
error path now has `child_environment_refusal_test.go`
`TestDockerChildEnvironmentRefusalPreventsLaunch`. Its ordinary runner control
creates the expected marker, while refused attribution prevents the child from
starting and reports the actual cause (`10376f53`). A single-file omission of
that guard keeps the positive control but wrongly starts the refused child and
returns success (`4ccf0cb9`). The unchanged live test passes, and count coverage
records the error branch once (`029f4e53`).

The owned native Docker root `orphans_docker_test.go`
`TestKilledLabRunIsCleanedByNextStart` also passes (`53fd048f`). Its first sweep
removes the killed child's container, network and retained image tag while
preserving the live sibling's three resources. After the second child is killed,
the second sweep removes its three resources. Both sweeps report no failed or
unknown removals.

The following native GoBGP run reached its initial barrier, then failed during
the first actual SIGHUP `accept` phase. Both FRR receivers lacked
`10.240.1.0/24`, and GoBGP's sent bookkeeping retained six prefixes against seven
expected. This is `job-gobgp-native-export-reload-owned-fe673b84.log`, distinct
from the earlier canceled setup-only run with the same hash suffix. The retained
proof and wire capture belong to `bgp-export-reload-gobgp-520019689`.

The deciding upstream source, GoBGP v4.11.0 `pkg/config/config.go` `UpdateConfig`,
requests an IN-only soft reset after policy changes. Its SIGHUP handler in
`cmd/gobgpd/main.go` calls that producer. The reviewed two-hunk repair is now a
build-time patch under `test/interop/`. One hunk requests BOTH directions after
policy changes. The other sends a withdrawal for a previously advertised
secondary destination when import removes it and current export policy rejects
its old candidates. The Dockerfile builds the pinned release from the patched
source and retains the applied patch for runtime identification.

The first zero-fuzz patch dry-run failed because the server hunk had incomplete
context (`gobgp-upstream-patch-dry-run-def7d886`). After a context-only correction,
both hunks apply with zero fuzz
(`gobgp-upstream-patch-dry-run-corrected-def7d886`). Independent source review
found the corrected build and provenance increments clean.

Two new roots in `check_gobgp_reload_patch_test.go` bind the runtime patch bytes
and digest to the checkout source and require unknown provenance to stop before
configuration observations. Their initial baseline failed the positive artifact
checks and incorrectly admitted all eleven unknown-provenance cases
(`gobgp-patch-provenance-baseline-ade2ffd5`). After the first repair, stronger
configuration-read accounting exposed all eleven negative cases reading the
configuration hash too early, while the positive passed
(`gobgp-patch-order-baseline-ade2ffd5`). Moving the unchanged hash query after
`gobgpReloadProof.identifyPatch` restores both roots and all eleven negatives
(`gobgp-patch-order-restoration-ade2ffd5`).

All fourteen affected provenance and reload-control roots pass (`278c52b1`).
Count coverage executes every block in `check_gobgp_reload_patch.go`
(`21c4953e`). The patched native run has now failed its final wire check:
`job-gobgp-native-export-reload-patched-fe673b84.log` reports
`DelayOpen capture timestamps missing or reversed`. Its exact retained artifact
is `bgp-export-reload-gobgp-368321296`. The retained queries cover the initial
state and all nine reload phases, but the failed wire check prevents an overall
native pass.

The safe `gobgp-patched-capture-digest.json` records 376 complete packets, zero
capture drops and the observed applied patch digest. Four adjacent record
timestamps reverse by one or two microseconds, each across different TCP
connections. Every connection's own record order remains monotone. The deciding
guard in `check_delayopen_wire.go` `delayOpenHistory` currently requires global
record-time order before it groups connections. A correction to enforce the
clock per connection, while preserving all other packet guards and explicitly
refusing encoded-zero times, is under source-only design. No implementation has
been released. The unpatched accept failure remains the semantic baseline,
neither complete native successor is proved, and the original four-peer captures
remain missing.

The earlier signed-fixture attempt stopped during its initial commit because the
owner's GPG unlock expired, before semantic assertions. Thomas has now unlocked
signing. MAIN's real committed `TestCommittedExternalReceivedStructFieldRemainsUnknown`
passes for both plugin and YANG registrations
(`inventory-unlocked-received-field-proof-3c78ea9b`). The three signed public-entry
roots also pass (`5ed52cf2`): all eight capability cases, both certain-initializer
cases and the both-sink-witness controls.

The current static-export omission exposes unsafe report publication in both
plugin and YANG wrapper cases (`e1b29b12`). The separate possible-initializer
omission exposes the same defect in both blank-initializer cases (`3968a2fb`).
Both runs retain the inert controls and emit the intended mutation markers.
These single-file overlays preserve terminal guards, cycle certificates and
origin propagation, and leave the live producer unchanged. The signed
public-entry restoration is in progress with no verdict yet.

The full signed-fixture regression, remaining discriminators, measured coverage,
documentation checks and isolated publication remain owed. Tests, formatting and
profiling ran only in MAIN through serialized native admission. The broader spec
stays open.

### Phase 12 P-14 and P-17 tmpfs source census on 2026-11-08

Source-only census by a subagent of MAIN session
`4d1dfc24-d1d9-4943-9205-0cedaf520caf`. No test, build, lint or formatter ran
for these rows; each names the command MAIN owes. A row records what source
holds today, not a passing run.

**P-14 L2TP `unused` population.** The original finding is the
`bulk-rename-corruption` 2026-08-10 row: `golangci-lint` red on
`writeAVPUint8` and `readAVPUint8` after commit `3c9644e15` unexported them.
That row named every package the rename touched as the wider risk, so the L2TP
population is every identifier `3c9644e15` unexported under
`internal/component/l2tp/` (`git show 3c9644e15 -- internal/component/l2tp/`,
lowercase words on `+` lines that no `-` line carries, which holds every
renamed identifier and some comment words): 91 names. Each
was counted against current non-test source under `internal/component/l2tp/`.

| Member | Current source | Resolution |
|--------|----------------|------------|
| `writeAVPUint8`, `readAVPUint8` (`avp.go`) | Absent; `avp.go` keeps the comment "No writeAVPUint8 exists" | Removed under the 2026-08-12 `unwired-feature` row: RFC 2661 defines no 1-octet Attribute Value. The only two `unused` members of the population |
| `unregisterShaperHandler` (`subscriber/handler_registry.go`) | Defined, called only by `handler_registry_test.go` | Not an `unused` finding: `.golangci.yml` sets no `tests:`, so golangci-lint loads test files and sees the caller. Test-only helper, left as it is |
| `getPoolStatsProvider`, `registerPoolStatsProvider`, `unregisterPrefixHandler`, `unregisterPrefixReleaser`, `lookupByMAC`, `iPCPOptions`, `lCPNegPolicy` | No occurrence anywhere in `internal/` or `cmd/` | Removed or renamed after `3c9644e15`; nothing left to be unused |
| The other 81 names | Two or more whole-word occurrences in non-test source under `internal/component/l2tp/` | Live, or a comment word; no `unused` shape. A name used only inside its own body would also count twice, so the lint run below stays the proof |

No member is still present and unused, so no source change was owed. The
proof that the package is green is `./le go lint run scope "./internal/component/l2tp/..."`,
which MAIN owes. The `bulk-rename-corruption` row's other packages are outside
this L2TP population and are not certified here.

**P-14 schema registration census.** The `_ = l.LoadRegistered()` and
`_ = l.Resolve()` discards the `silent-fall-through` 2026-09-04 row named are
gone: `resolveDefault` (`internal/component/config/yang/loader.go`) joins both
errors, and `TestDefaultLoaderReportsEveryRegisteredFailure` proves an import
nobody registers is an error. Two schema-resolving test binaries still
discarded that error: `buildTestWireToPath` in
`internal/component/plugin/server/all_import_test.go` and
`internalBuildTestWireToPath` in `benchmark_support_test.go`. With a nil
loader `TestYANGPathsAreUnique` passed with nothing checked. Both now fail on
the error. `TestEverySchemaResolvingCallerChecksTheResolveError` derives the
schema-answering functions from the producers' source and refuses any
discarded resolve error in the tree; `TestThisBinaryLinksEveryModuleItsSchemaImports`
resolves the config/cli binary's own registrations. The generated `register.go`
of each `yang/` package blank-imports the packages registering its modules'
imports, so dropping the `bgp/config` blank import from config/cli does not
unlink `ze-bgp-conf` today (`plugin/all` links `bgp/yang` directly and through
`rib/yang`); that test's recorded break is a renamed module statement instead.

**P-17 tmpfs.** `TestParsingSuiteHonorsTmpfsMode` derived executability from
the `.sh` extension and checked no bytes; `TestOrchestratedSuiteHonorsTmpfsMode`
checked the rebuild, not runTest's execution, and no bytes.
`TestTmpfsContentAndModeReachBothRoutes` (`internal/test/runner/one_off_proof_test.go`)
adds the missing carrier: an extensionless `mode=755` shim and a control with no
`mode=`, exact bytes and execute bits through `setupWorkDir` and through
`runTest`, which executes the shim. `docs/architecture/testing/ci-format.md`,
"Mode Defaults", said every file defaults to `0644`; it now states the
extension-derived default the code applies.

The CodeQL Go-extraction sentinel of P-17 is not attempted here: it is proven
only by a GitHub `CodeQL` workflow run on a pushed commit whose Go extraction
log shows an authored sentinel extracted and a `gokrazy/modcache` sentinel
absent.

## P-12 (c): IPsec lab log key (2026-11-09)

The repair is `163a5d1b88`: the IPsec lab sets `ZE_LOG=debug` for the Ze
peer (`zeEnvironment`, `internal/le/interoplab/ipsec/ipsec.go`); the former
`ZE_LOG_LEVEL` resolved to a logger subsystem named "level". The carrier is
`TestPreparedIPsecPeerEnablesDebugLogging`
(`internal/le/interoplab/ipsec/environment_consumer_test.go`), which hands the
prepared container environment to the real logger consumer. The spec names
`TestLogKeyReachesPeerCheck`, which does not exist; this test is its carrier.

| Step | Result |
| --- | --- |
| Green | passes (`go test -tags ze_le`) |
| Attributed red | `ZE_LOG` set back to `ZE_LOG_LEVEL`: fails as "prepared Ze peer environment does not enable IKE debug logging: [{ZE_LOG_LEVEL debug}]" |
| Restored green | bytes restored by the red-walk tool |

Still owed for P-12 (c): the checker half, a scenario whose `waitLog` needle
on the Ze peer is logged at debug level with no `ze-env` override, run in the
interop lab.

## P-12 (d): build-only bench staging (2026-11-09)

The repair is `8664a8ee44`: `Bench.Run` (`internal/le/perf/bench.go`) stages
Ze for a build-only run as well as a measured one, and probes the kernel only
for a measured run. Carrier: `internal/le/perf/bench_build_staging_test.go`.

| Step | Result |
| --- | --- |
| Green | `TestBuildOnlyStagesCurrentZeBeforeImageBuild` and `TestBuildOnlyStagingFailureStopsImageBuild` pass (`./le job run`, `go test -tags ze_le`) |
| Attributed red | Staging condition `(steps.Build \|\| steps.Test) && runsZe(duts)` set back to `steps.Test && runsZe(duts)`: the first test fails as "image build consumed "obsolete-input", want the current staged input", the second as "image build ran after input staging failed" and "failed input staging was reported as a successful build" |
| Restored green | Bytes restored by the red-walk tool; both pass again |

`TestBuildAndMeasuredRunsShareOneValidatedStage` asserts the shared stage: a
build-only and a measured run each make one Stage call over the checkout root,
the image consumer reads that stage, and only the measured run's kernel probe
runs, on the staged file. Attributed reds: probing `b.Root` instead of
`b.stagedZe()` fails the measured case ("kernel probe read ..., want the
staged ..."); staging only for a measured run fails the build-only case, whose
image consumer then finds no staged file ("image consumer read "" (open
...)"), which also covers a missing staged file reaching the consumer. Both
restored green.
