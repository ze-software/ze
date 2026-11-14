---
kind: directive
level: MUST
stage:
rationale: ai/rationale/rfc-compliance.md
---
**A `third-party` or `mixed` declaration MUST name the implementer, and MUST NOT stop at "the kernel".** Name the component and the mechanism a reader can go and check: Linux XFRM for the ESP and AH datapath, the Linux TCP stack for the transport, the Go standard library for the cipher. An unnamed implementer is the claim without the showing, and on the public page it reads as work nobody owns.
**A requirement out of the count MUST still carry a test wherever Ze can observe the behavior.** Ze installs the state the layer below acts on, so the boundary Ze owns stays testable where the packet handling is not: assert the selector, the security association, the socket option or the kernel counter Ze produced. Leaving the count is a statement about whose code it is. It is never a license to prove nothing.
**This qualifies the whole-stack directive above; it MUST NOT be read as voiding it.** That one governs a requirement whose role Ze fills and which Ze meets by configuring a layer below: it stays met, stays counted, and still owes its test. This one governs a document Ze does not write Go for at all. Where both could apply the requirement-level ruling wins, because a document Ze configures is a document Ze implements part of, which is `mixed`.
