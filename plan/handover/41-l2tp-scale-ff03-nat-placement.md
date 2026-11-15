# Handover 41: L2TP scale interop, FF 03 proof, NAT placement at verify

Written on the Mac, 2026-11-15, for a session on the Linux server (`claude`).
The work was recorded as owed work; nothing is implemented. The Linux server
is where it can run: Docker labs with PPPoL2TP and `/dev/ppp`, and the nft
integration tests in Ze's guest.

## Landed

| Commit | Content |
|--------|---------|
| 415e001f2f | The three items recorded in their specs, D-14's recommendation, `rfc/full/rfc1662.txt`; on origin |
| b5dc6cfb54 | The LAC peer chosen, AC-3 rewritten against the existing checker, AC-33 rewritten against the v7.3 kernel validators; on the Mac only until Thomas pushes or bundles it |

## The work, by spec

| Item | Spec | AC | State | Blocks on |
|------|------|----|-------|-----------|
| Many-sessions-per-tunnel LNS interop | `plan/release/validation/spec-finish-l2tp.md` | AC-2 | skeleton | Thomas approves the design before the spec moves to ready; it goes red until `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` lands, by design |
| FF 03 proof for RFC1661-6.6-1 and 6.6-2 | same spec | AC-3 | skeleton | Thomas approves the design |
| NAT and nat-chain placement refused at verify | `plan/release/fixes/spec-network-services-one-off-defects.md` | AC-33 | design | Thomas approves the design; the kernel rule set is already settled in the AC |
| D-14 route observer and `pppoe` redistribute source | `plan/release/fixes/spec-lifecycle-invariants.md` | AC-23, phase 7 | ready | Thomas's ruling on the recommended design; phase 7 also waits for the D-10 keyspace phase |

Each AC row is self-contained: the deciding source lines are quoted in it, so
start from the AC, not from this file.

## Facts established, with where they came from

- **LAC peer**: katalix go-l2tp `kl2tpd` v0.1.8 with pppd 2.5.5. Read at the
  tag: `cmd/kl2tpd/kl2tpd.go` `application.run` opens every
  `[tunnel.X].Sessions` entry inside one tunnel; `cmd/kl2tpd/pppd.go` runs
  pppd with `plugin pppol2tp.so` per session; `l2tp/msg.go` `newV2Sccrq`
  sends no Receive Window Size. A per-session `sid` key forces equal session
  IDs across tunnels.
- **xl2tpd set aside**: one call per tunnel (`l2tp_call`,
  `control_handle_lac_connect`); its numeric `c <tunnel-id>` path dereferences
  `c->lac` unguarded in `control_finish`. Read, not run.
- **accel-ppp set aside**: its LAC mode places many sessions in one tunnel,
  but its PPP only authenticates and assigns, so no session reaches IPCP
  against Ze. Read in a local checkout older than tag 1.15.0.
- **FF 03**: `pppInformation` (`internal/le/interoplab/l2tp/ppp_code_reject.go`)
  already refuses a relayed frame with no `ff 03`; the relay logs only after
  the checker's marker, so the LCP negotiation is unseen, and nothing is
  tagged. Ze acks a peer's ACFC (`lcp_options.go` `negotiatePeerOption`).
- **NAT**: read at Linux 7.3 (`internal/appliance/kernel.version`). The
  refused and accepted shapes are listed in AC-33 with their deciding lines.
  `lowerNAT` sets the NAT family from the address, so the table-family
  mismatch is reachable from config today.

## Open, not checked

- `pppd/plugins/pppoe/discovery.c` in pppd 2.5.5 is unread; read it before any
  PPPoE-at-scale check relies on pppd as the PPPoE client.
- `nft_chain_route.c` is unread; no NAT verdict depends on it.

## Traps

- Another session is reworking the PPPoE data plane
  (`internal/component/l2tp/pppoe/dataplane/`, uncommitted on the Mac on
  2026-11-15). D-14 touches the same package: check it has landed before any
  D-14 code.
- kl2tpd never reads Ze's advertised window: set its `window_size` no higher
  than Ze advertises, or a red is the peer's fault.
- On the server, lab and VM scratch goes on the second disk
  (`TMPDIR=$HOME/.cache/<name>`): the root disk aborts writes under QEMU load.
