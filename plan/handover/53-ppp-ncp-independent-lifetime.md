# Handover: spec-ppp-ncp-independent-lifetime (phases 1-4 landed, phase 2b and 5 open)

Spec: `plan/release/fixes/spec-ppp-ncp-independent-lifetime.md` (in-progress,
design approved at digest ed8980599aff, claimed above the WIP cap at Thomas's
order). The next session claims it (`./le spec claim <path>`; it is already
in-progress, so no cap override is needed) and reads the phase handoffs in
`tmp/session/2026-11-20-040327/state/session-state-f0538251-726d-40f5-958b-8ca5e2e442c0.md`
(the phase agents wrote there; the spec's own state file is
`session-state-ppp-ncp-independent-lifetime-f0538251-...md` in the same folder).

## Landed

| Commit | What |
|--------|------|
| 1ad6c96575 | design sections, judged; new skeleton `spec-pppoe-ncp-timeout-leaf`; journal rows |
| 89be20c65e | spec in-progress, `Design-approved:` trailer |
| 5719b2877c | (another session's commit) carried early Phase 1 edits: `ppp.EventSessionIPReleased`, parts of `ncp.go`/`session_run.go`, three pages |
| d22b547b51 | Phases 1-4: per-family release (observer, registry, reactor `assignedAddr`, RADIUS Interim, `ipv6-interface-id`), PPPoE default arm, Protocol-Reject Down event, no-NCP bound, Phase 4 proofs; 13 judged test conversions; RFC1661-5.7-2/5.7-5 records; 12 re-judged audit stamps |
| 216f756d01 | `ppp-code-reject-xl2tpd` asserts the IPCP withdrawal and reopen; 7 re-taken records; 6 stamps |

Evidence: `./le test unit pkg path` green for `l2tp/ppp`, `l2tp`, `l2tp/pppoe`,
`l2tp/plugins/authradius`, `l2tp/subscriber/{cmd,events}`, `l2tp/events`;
the three linux-only full-chain tests (`TestL2TPFamilyRelease*`,
`TestL2TPSubscriberNetworkLifetime`) PASS on Ze's kernel via
`./le test qemu docker-lab kernel <runtime-docker vmlinuz> harness command`
with a host-cross-compiled `go test -c` binary (the guest has no Go);
`ppp-code-reject-xl2tpd` interop PASS.

## Phase 2b landed too

438bb234e4 closes RFC1661-4.3-3's Code-Reject half: a Code-Rejected
Echo-Request stops the echo probes (for the session, not only the LCP
lifetime: a choice Thomas may want to confirm), a Code-Rejected Echo-Reply
stops Ze answering, and a Code-Reject of codes 1-8 (LCP) / 1-7 (an NCP) or a
Protocol-Reject of LCP is RXJ- with NAS Error. Where pppd 2.5.5
`fsm_rcoderej` treats every Code-Reject as RXJ+, Ze follows RFC 1661
Section 4.3. `ppp` package green; three 4.3-3 records taken. Not yet
re-checked: `./le rfc check` for audit verdicts this change staled (run it
first and re-judge any RFC 1661 row it names).

## Waiting on Thomas: the interop proof, (a) or (b)

Ze has no dual-stack pool: the only pool handler declines IPv6
(`plugins/pool/register.go` `poolPlugin.handle`: "IPv6 not supported by
static pool"), so the two approved IPv6 interop scenarios cannot run on a
shipped Ze. Options put to him:

- (a), recommended: three IPv4-reachable L2TP scenarios: stock pppd (IPv6CP
  on by default) vs IPv4-only Ze (his home-line case: Protocol-Reject of
  IPv6CP and CCP, IPv4 up past `ncp timeout`; goal evidence); pppd `noip`
  (Protocol-Reject of IPCP: red today, discriminates the dispatch); IPCP
  Terminate on the FRR redistribution scenario (/32 leaves FRR and returns:
  red if the release event is reverted). The IPv6 scenarios move to
  `spec-l2tp-ipv6-subscriber`.
- (b): pull the IPv6 pool slice into this spec.

Either way the spec's interop section changes: re-judge (`ze-design-judge`)
and Thomas re-runs `./le spec design approve`. His first approval recorded
the placeholder words; he may restate them.

## Then

Phase 5 per his answer, spec bookkeeping (TDD file names, Files to Create:
`ncp_bound.go`, the new test files), `./le doc check all`, then `/ze-close`.
The design check refuses three Design documents rows whose quoted sentences
the implemented departures rewrote (11.3 x2, bng-1 row 1); `/ze-close`
decides how the gate wants a departs row after implementation.

## Noted, not this spec's

- `ike/engine/remote_id.go` carried another session's uncommitted
  `return true` in `textIdentityMatches` (subtree remote-id matches any
  peer): an authentication bypass if committed. Told Thomas.
- Lint in `ppp/kernel/` (other session) and a pre-existing always-true
  `runParallelNCPPeer` parameter.
- RFC1661-4.6-4 (`Dialer.Dial`, PPPoE client) audit verdict stale from
  another session's change.
