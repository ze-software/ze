# IPCP Terminate-Request withdraws and reinstalls the redistributed /32

The one topology with both the relay and FRR. xl2tpd 1.3.20 and pppd 2.5.5
(`noipv6`, so IPCP is the only NCP) dial Ze's LNS through the transparent relay
on `172.29.0.4`, and FRR on `172.29.0.6` peers IPv4 unicast BGP with Ze from
this directory's `frr.conf`. Ze redistributes the subscriber `/32`, as in
`02-ppp-bgp-redistribute-frr`, whose FRR on `172.29.0.4` the relay displaces
here.

`ze.conf` sets `lcp restart-timer 10000` and `ncp timeout 20`. Ze holds IPCP in
Stopping for one Restart time after the Terminate-Request and ignores pppd's
Configure-Requests until Stopped, so the reopen waits for pppd's next one, up
to its 3 s IPCP retransmission later. The Restart time is the window FRR is
polled in, and the bound exceeds the Restart time plus that retransmission, so
the reopen cancels the no-NCP bound armed when IPCP left Opened.

The checker requires:

1. BGP is Established, IPCP opens, a ping crosses, and FRR holds
   `10.100.0.2/32`.
2. Phase A. After the checker creates `/run/ipcp-terminate`, the relay
   injects one IPCP Terminate-Request with Identifier 0x5a. Ze's IPv4 address
   leaves its `pppN` (polled every 100 ms) and FRR loses the `/32` (polled
   every 250 ms inside the Restart time), Ze's Terminate-Ack carries 0x5a,
   and the relay injected nothing else.
3. Phase B. pppd renegotiates IPCP (pppd 2.5.5 `fsm_rtermack`,
   `case OPENED:`), an IPCP Configure-Ack crosses each way, the address
   returns on the same `pppN`, FRR holds the `/32` again, and a ping crosses.
4. Past `ncp timeout` plus one Restart time from the trigger, the session,
   both PPP interfaces, the `/32` and the BGP session remain, and the capture
   holds no CDN from Ze and no LCP Terminate-Request.

Revert walk: remove the release event emission (`publishNCPRelease`,
`internal/component/l2tp/ppp/ncp.go`), rebuild the Ze image, and assertion 2
goes red: FRR keeps the `/32` although the address left `pppN`. Assertions
live in `internal/le/interoplab/l2tp/ncp_lifetime.go`
(`checkIPCPTerminateRedistribute`).

Run with `ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-pppd-ipcp-terminate-redistribute ./le --name session test deployment docker-l2tp-ppp`.
This directory supplies inputs and carries no script.
