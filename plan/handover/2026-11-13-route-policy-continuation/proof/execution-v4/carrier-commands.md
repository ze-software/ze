This is a source-only refresh of the original 24 carrier populations. All cuts remain UNAPPLIED. Use a freshly built binary environment; retained example binary paths below are historical and must be replaced with the builder output. Supplements 25/26/27 use their separate unit/native recipes.

# Exact native carrier commands

Set `rp5_kernel` to the matching absolute runtime vmlinuz from the current image build. The latest directly read native log names `/home/thomas/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-docker-16f81fff-c79de00c/vmlinuz`; the native action still checks its bytes and image identity. Set `rp5_env` from the current guest-binaries report, not a stale binary directory. For this native amd64 source:

```bash
rp5_kernel=/home/thomas/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-docker-16f81fff-c79de00c/vmlinuz
rp5_env='ZE_BIN=tmp/qemu/linux-amd64/bin/ze ZE_STRIPPED_BIN=tmp/qemu/linux-amd64/bin/ze-stripped ZE_STRIPPED_FLOW_BIN=tmp/qemu/linux-amd64/bin/ze-stripped-flow'
```

For each row below, use source-cycle.md with its source/hash/patch, build the cut once, and run every exact listed path. For each carrier, set `rp5_carrier` to its literal path and `rp5_stage=red`. After guarded restoration and rebuild, use the same path with `rp5_stage=green`. These commands preserve the existing OneCICommand shape and add only the implemented selected save keyword.

```bash
rp5_carrier=test/draft/plugin/rib-import-central-bogons.ci
rp5_stage=red
rp5_stem="${rp5_carrier##*/}"
rp5_stem="${rp5_stem%.ci}"
rp5_guest_command="$rp5_env tmp/qemu/linux-amd64/le test qemu all test $rp5_carrier save $rp5_attempt_rel/$rp5_stage/$rp5_stem"
if CGO_ENABLED=0 ./le --name session job run label "rp5-$rp5_cut-$rp5_stage-$rp5_stem" quiet command ./le --name session test qemu docker-lab kernel "$rp5_kernel" harness timeout 3000s command "$rp5_guest_command"; then
  rp5_carrier_exit=0
else
  rp5_carrier_exit=$?
fi
printf 'carrier=%s stage=%s actual_exit=%s\n' "$rp5_carrier" "$rp5_stage" "$rp5_carrier_exit"
```

An expected test failure gives a nonzero job result. Read the complete returned quiet log and the saved runner capture before proceeding; do not pipe the test through a filter. A setup/compiler/startup failure does not satisfy the semantic target. The red must retain the specified earlier controls. The green must finish the entire same carrier, including later controls.


## 01-central

Source: `internal/core/rib/policy/policy.go`; SHA256 `c0d8e2514799c3b2bb25fe4e4155e7652a8599c25c24b791e7691380c95c8176`; patch `execution-v4/01-central.current.patch`.

Earlier control: Both static and BGP admitted winners are awaited before rejected-prefix visibility is checked.

Required RED: Judged central prefix becomes selectable; original reject assertion fails.

- `test/draft/plugin/rib-import-central-bogons.ci`
- `test/draft/plugin/rib-import-central-bogons-forked.ci`

## 02-source

Source: `internal/core/rib/policy/policy.go`; SHA256 `c0d8e2514799c3b2bb25fe4e4155e7652a8599c25c24b791e7691380c95c8176`; patch `execution-v4/02-source.current.patch`.

Earlier control: Permitted connected winner and its full candidate metadata pass before rejected connected candidate.

Required RED: Rejected connected candidate becomes selectable; no claim of later kernel checks executing.

- `test/draft/plugin/connected-import-filter-hides.ci`
- `test/draft/plugin/connected-import-filter-hides-forked.ci`

## 03-policy-readmit

Source: `internal/core/rib/locrib/policy.go`; SHA256 `b88230cb7b1b10a1758b36010948a8dab46e93ff45dceca962bdf8b1e0273423`; patch `execution-v4/03-policy-readmit.current.patch`.

Earlier control: Original hidden static, admitted BGP/static controls and initial FIB outcome pass before relaxing policy.

Required RED: First relaxed-chain reload retains old hidden static instead of re-admitting it.

- `test/draft/plugin/static-import-filter-distance.ci`
- `test/draft/plugin/static-import-filter-distance-forked.ci`

## 04-distance-reselect

Source: `internal/component/sysrib/route_view_distances.go`; SHA256 `f1e3851c6b0a4016628c61f4caba70e29ce85891a1eb6b4279a91aa573469855`; patch `execution-v4/04-distance-reselect.current.patch`.

Earlier control: Initial candidate/kernel winners, own-distance-3 route and connected kernel control pass.

Required RED: First owner-only distance reload leaves existing candidate distance/winner stale.

- `test/draft/plugin/sysrib-distance-reload.ci`
- `test/draft/plugin/sysrib-distance-reload-forked.ci`
- `test/draft/plugin/sysrib-distance-reload-configured.ci`
- `test/draft/plugin/sysrib-distance-reload-configured-forked.ci`

## 05-candidate-attribution

Source: `internal/component/sysrib/candidates.go`; SHA256 `fca4c9a23ab1620c9277bea47b049bc5c6c2cc8014ce094659a12ef89d184eec`; patch `execution-v4/05-candidate-attribution.current.patch`.

Earlier control: Original central scenario awaits both admitted winners before stored hidden attribution.

Required RED: Hidden flag remains but hidden-by/chain disappear; later extra kernel block is not reached.

- `test/draft/plugin/rib-import-central-bogons.ci`
- `test/draft/plugin/rib-import-central-bogons-forked.ci`

## 06-candidate-tag-presence

Source: `internal/component/sysrib/candidates.go`; SHA256 `fca4c9a23ab1620c9277bea47b049bc5c6c2cc8014ce094659a12ef89d184eec`; patch `execution-v4/06-candidate-tag-presence.current.patch`.

Earlier control: Initial tag4242 hidden row, selected explicit zero, plain candidate and both permitted kernel routes pass.

Required RED: NOZERO reload hides zero but omits its explicit-zero candidate tag.

- `test/draft/plugin/rib-import-tag-internal-internal.ci`
- `test/draft/plugin/rib-import-tag-internal-forked.ci`
- `test/draft/plugin/rib-import-tag-forked-internal.ci`
- `test/draft/plugin/rib-import-tag-forked-forked.ci`

## 07-wire-tag-presence

Source: `internal/component/plugin/server/dispatch_route.go`; SHA256 `a753c04ead58895cd0511cf0e5bb59397ae000b0d9e951468fe95b0d89454ab4`; patch `execution-v4/07-wire-tag-presence.current.patch`.

Earlier control: Producer carrier first proves a selected connected cover and the hidden tag4242 static candidate. Wire zero row arrives without presence; initial plain Ze kernel barrier has not yet executed.

Required RED: Explicit-zero route-install presence is lost. Supplemental boundary proof, not completed producer lifecycle.

- `test/draft/plugin/sysrib-forked-producer-kernel.ci`
- `test/draft/plugin/rib-import-tag-internal-forked.ci`
- `test/draft/plugin/rib-import-tag-forked-forked.ci`

## 08-hidden-lpm-cover

Source: `internal/core/rib/locrib/manager.go`; SHA256 `f5bd2a647684165ce563de9a2a7803f0ecd07a81d8f54f9a61138b5e7a553c08`; patch `execution-v4/08-hidden-lpm-cover.current.patch`.

Earlier control: Both connected cover winners and hidden more-specific candidates are present before subjects. Other nonmatching prefixes remain resolvable.

Required RED: Any matching hidden-only group makes LPM return unresolved regardless of random shard placement. Internal forwarding or external authoritative metric assertion fails.

- `test/draft/plugin/sysrib-recursive-nexthop.ci`
- `test/draft/plugin/sysrib-recursive-nexthop-forked.ci`

## 09-resolver-dependency

Source: `internal/component/sysrib/fibimport.go`; SHA256 `8ecfa4c890a7c440ac22d5de779f30a2df0973a234993b718ea82dfd86835b3c`; patch `execution-v4/09-resolver-dependency.current.patch`.

Earlier control: All initial IPv4/IPv6/cross-family exact FIB/kernel routes and survivor checks pass.

Required RED: Cover-only removal leaves recursive dependent output instead of withdrawing it.

- `test/draft/plugin/sysrib-recursive-nexthop.ci`
- `test/draft/plugin/sysrib-recursive-nexthop-forked.ci`

## 10-member-labels

Source: `internal/component/sysrib/ecmp.go`; SHA256 `05b0da7dc1f709ad5035f6d9cdad3102cddbea263914637424599edb3c5c11ad`; patch `execution-v4/10-member-labels.current.patch`.

Earlier control: Initial plain ECMP typed event passes before labeled member assertion. Later initial kernel block is not reached.

Required RED: Selected member labels are missing in the exact FIB entry.

- `test/draft/plugin/sysrib-ecmp-replay.ci`
- `test/draft/plugin/sysrib-ecmp-replay-forked.ci`

## 11-member-sid

Source: `internal/component/sysrib/ecmp.go`; SHA256 `05b0da7dc1f709ad5035f6d9cdad3102cddbea263914637424599edb3c5c11ad`; patch `execution-v4/11-member-sid.current.patch`.

Earlier control: Complete initial weighted/device/label/SID/backup FIB and kernel checks pass.

Required RED: Cover loss promotes wrong member SID instead of the member's own SID.

- `test/draft/plugin/sysrib-ecmp-replay.ci`
- `test/draft/plugin/sysrib-ecmp-replay-forked.ci`

## 12-replay

Source: `internal/component/sysrib/route_view_worker.go`; SHA256 `6e302d1f6beaf381c704dfdc4527509b687642e6815b909f4820264b8c669c51`; patch `execution-v4/12-replay.current.patch`.

Earlier control: Every live Add remains delivered, including startup replay. Each scenario's initial real FIB/kernel controls and state transitions precede its explicit replay check.

Required RED: Fresh replay entries arrive as incremental updates; replay correlation/capture fails while kernel routes remain useful.

- `test/draft/plugin/sysrib-recursive-nexthop.ci`
- `test/draft/plugin/sysrib-recursive-nexthop-forked.ci`
- `test/draft/plugin/sysrib-ecmp-replay.ci`
- `test/draft/plugin/sysrib-ecmp-replay-forked.ci`
- `test/draft/plugin/sysrib-slow-reader-reset.ci`

## 13-reset-fence

Source: `internal/component/sysrib/route_view_feed.go`; SHA256 `6018ab3f861710e4837d800a94f9b508772589e6ff87c61e2241f0ddb2badfb4`; patch `execution-v4/13-reset-fence.current.patch`.

Earlier control: Old output and control kernel routes, ready gauge, real child identity and stopped T state pass; all producer batches remain acknowledged.

Required RED: Old control remains installed while actual consumer is still SIGSTOPed.

- `test/draft/plugin/sysrib-slow-reader-reset.ci`

## 14-owner-output-withdraw

Source: `internal/component/sysrib/route_view_output.go`; SHA256 `10baf967e32ef6ef76b8675ad070a501ad339ad2403ee8c01e922c3afcb988af`; patch `execution-v4/14-owner-output-withdraw.current.patch`.

Earlier control: Both initial useful routes, kernel output, healthy readiness and survivor checks pass.

Required RED: Forked kill leaves old kernel output; internal operator replacement lacks old-generation withdrawal counts.

- `test/draft/plugin/sysrib-feed-restart.ci`
- `test/draft/plugin/sysrib-feed-restart-forked.ci`

## 15-producer-withdraw

Source: `internal/component/plugin/server/dispatch_route.go`; SHA256 `a753c04ead58895cd0511cf0e5bb59397ae000b0d9e951468fe95b0d89454ab4`; patch `execution-v4/15-producer-withdraw.current.patch`.

Earlier control: Both real producer candidate/kernel sets and observer/OS/foreign/withheld survivors pass.

Required RED: Killed static producer retains candidate; SDK kill leg is not reached under this broad cut.

- `test/draft/plugin/sysrib-forked-producer-kernel.ci`

## 16-recursive-static-metric

Source: `internal/core/rib/igpcost/igpcost.go`; SHA256 `ca801bacd3f1835b3214f965f351a5a1a7f810ebfacedb6d0bca5545dc0cf611`; patch `execution-v4/16-recursive-static-metric.current.patch`.

Earlier control: Connected selected cover is awaited. First observer metric vector sees recursive static cost999; actual BGP subjects have not yet been injected.

Required RED: Authoritative SDK metric semantics fail; this is not actual BGP caller discrimination.

- `test/draft/plugin/sysrib-bgp-metric-callers.ci`
- `test/draft/plugin/sysrib-bgp-metric-callers-forked.ci`

## 17-missing-aigp

Source: `internal/core/rib/igpcost/igpcost.go`; SHA256 `ca801bacd3f1835b3214f965f351a5a1a7f810ebfacedb6d0bca5545dc0cf611`; patch `execution-v4/17-missing-aigp.current.patch`.

Earlier control: Connected selection plus earlier positive recursive cost11 and received-AIGP cost18 vector rows pass.

Required RED: Missing-AIGP row loses its presence distinction before actual BGP subjects are injected.

- `test/draft/plugin/sysrib-bgp-metric-callers.ci`
- `test/draft/plugin/sysrib-bgp-metric-callers-forked.ci`

## 18-forked-metric-invalidation

Source: `internal/component/bgp/plugins/rib/rib_remote.go`; SHA256 `73cc5518cfd62b1d4f3d0c881b2004c38999d12b3c10e8b6c2ef8804b0012c71`; patch `execution-v4/18-forked-metric-invalidation.current.patch`.

Earlier control: Initial observer metrics and both actual forked BGP winners/selected next hops pass.

Required RED: After zero-cover loss, authoritative RPC vector changes but actual BGP winner remains stale.

- `test/draft/plugin/sysrib-bgp-metric-callers-forked.ci`

## 19-internal-metric-invalidation

Source: `internal/component/bgp/plugins/rib/rib_aigp.go`; SHA256 `5c3bced9aee21805a4f50992b6393c6bf4152f437137736560c715d7283ea6c8`; patch `execution-v4/19-internal-metric-invalidation.current.patch`.

Earlier control: Initial observer metrics and both actual internal BGP winners/selected next hops pass.

Required RED: After zero-cover loss, authoritative vector changes but internal BGP reselection is not triggered.

- `test/draft/plugin/sysrib-bgp-metric-callers.ci`

## 20-ready-metric

Source: `internal/component/sysrib/doctor_route_view.go`; SHA256 `a2a94b6844107dafed3b542739cc99f39cca3aabc000bc9647951fb573728b98`; patch `execution-v4/20-ready-metric.current.patch`.

Earlier control: Both candidate/selected static controls and real blackhole proto250 routes pass.

Required RED: Ready gauge remains zero after useful output; doctor later check may not be reached.

- `test/draft/plugin/sysrib-execution-health.ci`
- `test/draft/plugin/sysrib-execution-health-forked.ci`

## 21-resync-counter

Source: `internal/component/sysrib/doctor_route_view.go`; SHA256 `a2a94b6844107dafed3b542739cc99f39cca3aabc000bc9647951fb573728b98`; patch `execution-v4/21-resync-counter.current.patch`.

Earlier control: Initial ready/kernel controls, stopped-reader fence and same-process complete current kernel reconstruction pass.

Required RED: Recovery resync total never advances.

- `test/draft/plugin/sysrib-slow-reader-reset.ci`

## 22-failed-owner-recovery-command

Source: `internal/component/doctor/cmd/show.go`; SHA256 `342875269bd3ca8aa3268beb29458e49c5b79fa4a6adabd93a409b0ba3a7a589`; patch `execution-v4/22-failed-owner-recovery-command.current.patch`.

Earlier control: Initial healthy kernel controls pass; actual child is killed; scoped removals and OS/foreign/withheld survivors pass.

Required RED: Daemon unreachable doctor finding lacks exact configured-owner restart command.

- `test/draft/plugin/sysrib-feed-restart-forked.ci`

## 23-withholding

Source: `internal/component/sysrib/fibimport.go`; SHA256 `8ecfa4c890a7c440ac22d5de779f30a2df0973a234993b718ea82dfd86835b3c`; patch `execution-v4/23-withholding.current.patch`.

Earlier control: Complete initial weighted/device/label/SID/backup FIB and kernel checks pass.

Required RED: Declared withheld BGP winner is programmed instead of staying absent from kernel.

- `test/draft/plugin/sysrib-ecmp-replay.ci`
- `test/draft/plugin/sysrib-ecmp-replay-forked.ci`

## 24-output-action

Source: `internal/component/sysrib/route_view_forwarding.go`; SHA256 `4ea6d84b061e8535ff79ac084d86eea31449e68c8047dc44666b9aa658e9c2e3`; patch `execution-v4/24-output-action.current.patch`.

Earlier control: Both candidate command forms and both selected static winners pass before kernel readback. Plain untagged forwarding remains unchanged.

Required RED: Tagged blackhole is emitted as valid unreachable; exact tagged kernel route action fails.

- `test/draft/plugin/sysrib-execution-parity.ci`
- `test/draft/plugin/sysrib-execution-parity-forked.ci`

## Final load and promotion remain actual work

Once repaired final source and all ordinary/cut/restored checks pass, a small native stress admission run precedes the exact final request. The current action admits `stress 80 stress-parallel 1` (or the independently approved worker count up to 4) on each same draft path. Do not add `save` to stress: source explicitly rejects that combination; the existing stress report owns its capture. Set outer timeout above the checked budget in selectedStress.budget plus boot/preparation. Run serially in the main executor. Require Iterations=Completed=Executed=80, no failed child, setup error, reproduction, interruption or skip, exact selected argv and actual selected-test summaries. These are count requirements, not performance figures. Promotion/native registration and the affected scoped suites follow only after this proof.
