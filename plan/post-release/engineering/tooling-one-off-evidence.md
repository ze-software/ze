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
