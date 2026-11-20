# pppd with noip rejects Ze's IPCP

xl2tpd 1.3.20 and pppd 2.5.5 dial Ze's LNS through the transparent relay on
`172.29.0.4`. pppd runs `noip`, which `ipcp.c` binds to
`ipcp_protent.enabled_flag`, so `main.c` `get_input` matches no protocol for
Ze's IPCP and answers it with an LCP Protocol-Reject naming 0x8021. pppd keeps
the link, because `auth.c` `continue_networks` still opens IPv6CP and counts it
in `num_np_open`. Ze runs its IPv4-only pool, so it Protocol-Rejects pppd's
IPv6CP, and no NCP opens on either side.

`ze.conf` sets `lcp restart-timer 2000` and `ncp timeout 8`. The bound exceeds
several Restart times on purpose: a Ze that ignored the Protocol-Reject would
retransmit its IPCP Configure-Request at each Restart time before the session
ends, so the revert walk can show its red.

The checker requires, from the relay's complete capture and Ze's log:

1. pppd's LCP Protocol-Reject naming 0x8021 carries Ze's IPCP
   Configure-Request.
2. Ze ends the call with a CDN, and its log names the no-NCP bound
   (`ncp: timeout after 8s with no NCP opened`).
3. Ze sent no IPCP packet after pppd's Protocol-Reject (RFC 1661 Section 5.7).
4. Ze Protocol-Rejected pppd's IPv6CP, sent no LCP Terminate-Request, and never
   logged `PPP session up`.

Revert walk: remove the Protocol-Reject dispatch (`receiveProtocolReject`,
`internal/component/l2tp/ppp/session_run.go`), rebuild the Ze image, and
assertion 3 goes red with Ze's IPCP Configure-Requests after pppd's
Protocol-Reject. Assertions live in
`internal/le/interoplab/l2tp/ncp_lifetime.go` (`checkNoIPRejectsIPCP`).

Run with `ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-pppd-noip-rejects-ipcp ./le --name session test deployment docker-l2tp-ppp`.
This directory supplies inputs and carries no script.
