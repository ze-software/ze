# Handover: close spec-ike-eap-rfc-defects

Written 2026-10-21 by the agent that drove `plan/immediate/spec-ike-eap-rfc-defects.md`
toward closure. The code is complete and reviewed; what is left needs the one heavy
slot on the Mac (a QEMU docker-lab), which another session's `./le verify worktree`
held for over three hours.

## Done this session (evidence a machine can reach)

| Item | Evidence |
|------|----------|
| Independent `/ze-review`, four rounds, converged 0 BLOCKER 0 ISSUE | round 1 found a BLOCKER (a rekey after a NAT rebind kept the retired pair's peer ADDRESS) and two ISSUEs; fixed in `5754eac4f4`; round 2's four ISSUEs fixed in the same commit; round 3's two ISSUEs fixed in `25938b3000`; round 4 clean (three NOTEs: the "ze defect" refusal in `installChildSA` names no next step, the lost-tunnel row has no configured-remote qualifier, a constant comment says "the peer declined it" which fits only the initiator) |
| Red then green for every new unit | eight mutations in a `git archive` export, each reddening its test (`rebind_address_test.go`), green restored; `./internal/component/ike/...` passes |
| RFC4301-5.1-7 re-judged by an independent auditor after `installChildSA` changed | `rfc/audit/rfc4301.json` in `5754eac4f4`: `unimplemented`, gap stands |
| A-5, A-7, A-9 confirmed from strongSwan 5.9.14 source | spec "Assumptions" rows (uncommitted spec edit, see below) |
| CI runs that carry the patched-tree steps | qemu-nightly run 39402173269 (jobs needs-linux and runtime-kernel-labs, step "Select the patched Go standard library": success), CodeQL Advanced 39576850399 (Analyze go: success), govulncheck 39407444997 (success) |
| QEMU guest under the patched tree | `f0493deda2` body: a real `./le test qemu` run passed with the guest tree in its tmpfs |
| D2 unit evidence | `go test` of `internal/core/eap`, `internal/le/go/stdpatch/...`, `internal/le/go/patchstamps`, `internal/le/go/toolchain`, `internal/appliance`: all ok on 2026-10-21 |

## Still owed before `/ze-close` (in this order, one heavy run at a time)

| # | Item | How |
|---|------|-----|
| 1 | Green run of the four new interop scenarios and the three existing real-nat ones | `./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=<name>"` for `real-nat-port-translated-ze-responder`, `real-nat-port-translated-transport-ze-responder`, `real-nat-port-translated-eap-ze-responder`, `responder-eap-tls12`, `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` |
| 2 | Revert-rebuild-red walk per new scenario | in a `git archive HEAD` export (never the shared tree): ports (`childEncapPorts` answering `transport.NATTPort` twice) reddens phase one of the tunnel and transport scenarios; a no-op `rebindChildSAs` reddens both rebind steps; the D1b adoption order reverted in `handleResponderEAP` reddens the EAP scenario; `responder-eap-tls12` has no reachable red (A-5 confirmed: charon never logs the id), record that |
| 3 | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (integration, linux, root) | `./le test qemu run kernel ~/.cache/ze/runtime-kernel/<entry>/vmlinuz command "<go test -tags integration -run ... ./internal/component/ike/engine/>"` |
| 4 | `docker/Dockerfile` proof build under the patched tree | `docker build -f docker/Dockerfile -t ze-d2-stdpatch-probe:<id> .`, then remove that tag by name |
| 5 | Full `./le verify worktree` once, attribute any red in the commit body | skip if `./le verify status check` is FRESH |
| 6 | `/ze-close` | append `plan/TEMPLATE-CLOSURE.md`, add the missing Deliverables Checklist, fill Goal Validation with the runs above, `./le spec review record ... rounds 4` (findings files of rounds 1 to 4 were scratch: their content is summarised in the row above), commit A then commit B, `./le spec release` |

The spec edits of 2026-10-21 (Assumptions A-5, A-7, A-9) and this handover are
committed together.
