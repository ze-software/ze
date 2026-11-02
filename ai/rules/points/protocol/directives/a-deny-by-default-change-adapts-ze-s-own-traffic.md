---
kind: directive
level: MUST
stage:
---
**A change that makes something deny by default (a firewall default, an XFRM catch-all, a policy default reject, a new refusal) MUST, in the same work, find every Ze subsystem that sends or receives its own traffic or input under it, and exempt or adapt each one.** Ze's own control traffic crosses the same policy as the operator's: a UDP connect used as a route lookup, an ICMP probe, a keepalive, a plugin's own session. Each one then fails with a permission error that reads as a defect of that subsystem, far from the change that caused it. Find the subsystems from the code that opens sockets or consumes the input, never from memory, and give each one a test that runs under the new default.
