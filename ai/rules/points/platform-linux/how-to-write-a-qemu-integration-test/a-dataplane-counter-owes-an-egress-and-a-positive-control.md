---
kind: directive
level: MUST
stage:
---
**A probe that asserts on a counter sitting behind state written for a remote
peer MUST send its traffic over an egress that really carries it, and MUST carry
a positive control.** An `ip xfrm` byte counter is the case in hand. Two network
namespaces, two VMs or two containers satisfy the first requirement. A host
addressing itself does not.

**The reason is the SELECTOR, not the interface.** An `ip xfrm` byte counter
belongs to a security association whose policy names a remote peer. A packet a
host sends to its own address matches no such policy, so no SA encrypts it and
the counter stays at zero. The counter then reads zero for a working dataplane
and zero for a broken one. A counter that names no peer is outside this
directive: a plain nftables rule counter in an input or output chain does advance
for a self-addressed packet, so a probe MAY read it from one host.

**Without the positive control the zero is unreadable.** A run known to move the
counter is what separates "the mechanism is broken" from "this setup never moves
this counter". It is the absence-assertion trap
`ai/rules/interop-and-goal-validation.md` names: ask what would still be absent
if the mechanism were deleted.

**Prefer the PEER's counter to your own.** A local outbound counter advances on
any key, including a wrong one, because sending is not proof of acceptance. The
receiver's inbound counter advances only after it has accepted what arrived, so
it is the one that answers the question the probe is really asking.
