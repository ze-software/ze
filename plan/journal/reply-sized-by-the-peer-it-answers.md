# A reply whose size follows the packet it answers, into a buffer sized for one packet

A protocol that answers a request option by option lets the sender choose how
many entries the reply carries. The reply is then assumed to be no larger than
the request, and the writer is given a caller-must-ensure-capacity contract on
the strength of that assumption. Both halves are wrong when one refused entry
draws more octets than it occupied, because the sender can repeat that entry
until the reply passes the frame.

The repair has two parts and the second is the one that gets skipped. Bound the
writer against the room it actually has and report the refusal, so the overflow
becomes a decision instead of an index. Then ask what the reply owes when it
does not fit, because a reply carrying the prefix that fit is a different packet
from the one the specification describes.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-31 | - | ppp LCP Configure-Nak construction, RFC 1661 Sections 5.3 and 6.4 | `NegotiatePeerOptions` (`internal/component/l2tp/ppp/lcp_options.go`) appends one reply entry per received option with no check that the Type is already listed, so a Configure-Request repeating one option draws a reply repeating it too. A Magic-Number received at Length 2 is refused and answered with the six octets RFC 1661 Section 6.4 gives the option, so 700 of them in a 1406-octet frame ask for a 4200-octet Configure-Nak. `WriteLCPOptions` then wrote every entry into the 1500-octet `frameBufPool` buffer with no bound at all, and `writeLCPOption` took the session goroutine down on `buf[off]`. Measured before the fix at `panic: runtime error: index out of range [1500] with length 1500`, reached from `handleFrame` through `sendConfigureNakOrReject`, so any unauthenticated PPP peer could crash the session. `appendUnlessListed` (`session_run.go`) already states the rule its own doc quotes from RFC 1661 Section 6, "(None of the Configuration Options in this specification can be listed more than once.)", but only the entry the option-Length fault adds passes through it | the bound is fixed: `writeLCPOption` and `WriteLCPOptions` return whether the option fit, write nothing when it does not, and both senders log and send no frame rather than a prefix. The duplicate-Type half is NOT fixed. It is a second declaration of the Section 6 rule that `appendUnlessListed` already holds, and moving every reply entry through that helper changes what ze answers a peer that repeats an option, which is a negotiation change rather than a bound. Regression: `TestLCPReplyLargerThanAFrameIsNotSent` (`internal/component/l2tp/ppp/lcp_test.go`), proven red against the unbounded writer with the panic quoted above |
