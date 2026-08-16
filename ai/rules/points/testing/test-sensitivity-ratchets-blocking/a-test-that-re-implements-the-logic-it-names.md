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

The tell is mechanical and it is the only one there is: **the test names a
function it never calls.** Before writing a test, name the function under test
and check the body calls it. When reviewing one, read what the assertion reads
from -- a local the test itself filled is the defect, whatever the test is
called. A broad detector would also flag correct table-driven tests that build
local fixtures, so this remains a review obligation.
