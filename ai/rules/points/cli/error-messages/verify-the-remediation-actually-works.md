---
kind: directive
level: MUST
stage:
---
**Leg 3 MUST be TRUE, not merely present.** A remediation that names a command
MUST name one that actually produces the promised effect. A command that looks
plausible but does not do what the message claims is worse than no advice: the
reader trusts it, follows it, and loses the time twice, then stops trusting the
tool's output at all. MUST verify the producer before you print the instruction -- if
the message says "re-run X to refresh Y", MUST read the code that writes Y and confirm
X writes it (a lint target does not rewrite a verify record; only a verify run
does). This is the `doctor-vpp-lcp-netns` class of bug: advice that cannot work.
