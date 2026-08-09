---
kind: note
level:
stage:
---
**No gate catches a test that re-implements the logic it names, and none is
planned.** Such a test builds the production algorithm again inside itself, in a
local variable, then asserts on its own copy. It DOES assert, so the
assert-nothing detector never sees it, and its name and `VALIDATES:` comment read
as coverage of the real thing. It is green against every implementation, the
correct one and the broken one alike.

Three of them stood in `internal/component/bgp/reactor/peer_test.go` until
2026-08-09, each building a local `familiesSent` map for RFC 4724 End-of-RIB
family tracking. `familiesSent` existed in no production code. One session read
them and was about to escalate a conformance violation that does not exist.

The tell is mechanical and it is the only one there is: **the test names a
function it never calls.** Before writing a test, name the function under test
and check the body calls it. When reviewing one, read what the assertion reads
from -- a local the test itself filled is the defect, whatever the test is
called. Widening the sensitivity ratchet to this class was tried and rejected:
every table-driven test builds local fixtures, so the detector would fire on
hundreds of correct tests, and a noisy detector gets switched off.
