# Stock pppd offering IPv6CP to Ze's IPv4-only pool

The home-line case. xl2tpd 1.3.20 and pppd 2.5.5 dial Ze's LNS through the
transparent relay on `172.29.0.4`. pppd runs its default options, so it offers
IPv6CP: `ipv6cp.c` initialises `ipv6cp_protent` with `enabled_flag` 1, and
this `ppp-options` carries no `noipv6`. Ze runs its IPv4-only pool, the only
pool it ships, so its own IPv6CP never starts and it answers pppd's IPv6CP
with an LCP Protocol-Reject naming 0x8057. `ze.conf` sets `ncp timeout 8`.

The checker requires, from the relay's complete capture:

1. IPCP opens and a ping crosses the session.
2. pppd sent an IPv6CP Configure-Request, Ze answered it with an LCP
   Protocol-Reject naming 0x8057, and Ze sent no IPv6CP packet.
3. Past `ncp timeout` and one Restart time beyond it, a ping still crosses,
   both peers hold the PPP interface they had, pppd sent no IPv6CP after Ze's
   Protocol-Reject (pppd `ipv6cp_protrej` calls `fsm_lowerdown`, which parks
   IPv6CP in Starting with its timer cancelled and calls no `np_finished`),
   and the capture holds no CDN from Ze and no LCP Terminate-Request.
4. After the checker creates `/run/ccp-configure-request`, the relay injects
   one CCP Configure-Request (Protocol 0x80fd, Code 1, Identifier 0x3c, no
   option) behind the next LAC datagram. Ze sends no CCP and answers it with
   an LCP Protocol-Reject naming 0x80fd whose Rejected-Information is the
   injected packet, and the session stays up.

It is goal evidence: it passes before and after the NCP lifetime work of
`spec-ppp-ncp-independent-lifetime`, because the pool decline and Ze's
unsupported-protocol answer exist without it. It owes no revert walk.
Assertions live in `internal/le/interoplab/l2tp/ncp_lifetime.go`
(`checkIPv6CPRejectedByLNS`), the injection in
`internal/le/interoplab/l2tp/avprelay/pppinject.go` (`injectCCP`).

Run with `ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-pppd-ipv6cp-rejected-by-lns ./le --name session test deployment docker-l2tp-ppp`.
This directory supplies inputs and carries no script.
