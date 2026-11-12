---
spec: rfc-verdict-fix-access
topic: BNG
found-by: review
---

## Before

An L2TP peer's unknown mandatory attribute could leave a tunnel or session
running. PPP termination could acknowledge the peer without releasing the
PPPoE session, and a PPPoE client could ignore CHAP reauthentication after
the connection began carrying traffic.

RADIUS could authorize an unsupported subscriber framing protocol. A change
of authorization could receive CoA-ACK even when the traffic backend refused
its rate, leaving the previous rate installed. A failed class-of-service
update could also replace the recorded session profile before its interface
maps were installed. Conflicting session selectors or an old accounting ID
could select a live session, and unimplemented identification attributes were
accepted without checking them.

<!-- source: internal/component/l2tp/avp.go -- AVPIterator.Next -->
<!-- source: internal/component/l2tp/tunnel_fsm.go -- handleHello -->
<!-- source: internal/component/l2tp/ppp/session_run.go -- signalLayerFinished -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- networkPhaseCHAP.handle -->
<!-- source: internal/component/l2tp/plugins/authradius/handler.go -- doRADIUS -->
<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- handleCoA, applySubscriberCoA -->
<!-- source: internal/plugins/cos/handler.go -- onCoSChange -->

## After

Ze clears the affected L2TP session or tunnel when it receives an unknown
mandatory attribute. PPP termination releases the lower-layer session, and
the PPPoE client answers CHAP reauthentication while preserving its existing
connection. RADIUS authorization for unsupported framing is refused before
the subscriber session opens.

CoA waits for the shaping and class-of-service owners to confirm their
updates before it sends ACK. An unavailable owner or a failed update produces
NAK. Compensation restores the shaper's last successfully applied complete
configuration and the class-of-service maps read before the change. A failed
restoration remains an error; it cannot earn ACK. A class-of-service change
is refused when its previous interface maps cannot be read.

CoA and Disconnect support Acct-Session-Id, User-Name and NAS-Port. Every
selector sent must match, including repeated attributes. Accounting IDs are
compared in full with the current registry or accounting owner. Other
identification attributes, including NAS-Identifier, receive NAK 401 instead
of being ignored. Configure authorization requests to use the supported
selectors. A later request can recover an abandoned rate transaction after
one minute, and later class-of-service events can apply changed profile maps.

Linux subscriber shaping programs download and upload rates with the correct
bit-to-byte conversion. TBF uses a classless root rate and refuses rates the
backend cannot represent. Appliance kernel requirements include the upload
policing action.

<!-- source: internal/component/l2tp/tunnel_fsm.go -- handleHello -->
<!-- source: internal/component/l2tp/session_fsm.go -- handleWEN, handleSLI -->
<!-- source: internal/component/l2tp/ppp/session_run.go -- applyTransition, signalLayerFinished -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- networkPhaseCHAP.handle -->
<!-- source: internal/component/l2tp/plugins/authradius/handler.go -- doRADIUS -->
<!-- source: internal/component/l2tp/plugins/authradius/coa_transaction.go -- confirmCoA -->
<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- sessionSelectorsMatch, findSessions, unsupportedAttr -->
<!-- source: internal/component/l2tp/plugins/shaper/rate_transaction.go -- rateOperation, restoreRate -->
<!-- source: internal/plugins/cos/session_transaction.go -- prepareSessionChange, applySessionChange, rollbackSessionChange -->
<!-- source: internal/plugins/cos/handler.go -- onCoSChange, consumeCommandReceipt -->
<!-- source: internal/plugins/traffic/netlink/translate_linux.go -- translateQdisc, rateBytesPerSecond -->
<!-- source: internal/appliance/kernelreq.go -- runtimeKernelRequirements -->

## Corrects

none

## Exposure

Subscriber deployments using L2TP, PPPoE or RADIUS authorization were affected
by the corresponding negotiation and policy paths. CoA requests still have
to pass the configured source and authenticator checks. The authorization
defects could give an authenticated RADIUS client a successful answer for a
policy change that was not installed.

<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- admitRequest, handleCoA -->

## Upstream

none

## Asked by

none
