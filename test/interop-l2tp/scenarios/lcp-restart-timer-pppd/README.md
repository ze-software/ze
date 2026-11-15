# Configured LCP Restart timing against pppd

Ze terminates L2TP with a 500 ms LCP Restart timer and Max-Configure of three.
The real xl2tpd/pppd peer first completes IPCP and carries a successful ping.
The typed checker then arms the transparent relay, which injects one empty
Configure-Request on that live session and drops the LAC's subsequent LCP
packets. L2TP control traffic continues normally.

The checker reads the relay's wire observations and monotonic timestamps. It
requires exactly three Ze Configure-Requests, intervals following the configured
500 ms period, at least one real pppd Configure reply observed and dropped,
then a Ze CDN for the same session after the final timeout. It finally stops
xl2tpd and requires both peers' kernel tunnel and PPP resources to disappear.

pppd 2.5.5's `fsm_rconfreq` restarts negotiation in Opened and answers the
request; `fsm_sconfreq` loads the request budget and arms retransmission. See
[the pinned peer source](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/fsm.c).
Assertions live in `internal/le/interoplab/l2tp/lcp_restart.go` and the relay
policy in `internal/le/interoplab/l2tp/avprelay/lcprestart.go`.

Run with `ZE_L2TP_INTEROP_SCENARIO=lcp-restart-timer-pppd ./le --name session test deployment docker-l2tp-ppp`.
An unchanged three-second interval or an ignored Max-Configure setting fails
the wire assertions. This directory supplies inputs and carries no script.
