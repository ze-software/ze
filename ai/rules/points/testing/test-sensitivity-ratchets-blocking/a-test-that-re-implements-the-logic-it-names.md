---
kind: directive
level: MUST NOT
stage:
---
**A test MUST NOT re-implement the logic it names, and no gate catches one that
does.** Such a test builds the production algorithm again inside itself, in a
local variable, then asserts on its own copy. It DOES assert, so the
assert-nothing detector never sees it, and it is green against the correct
implementation and the broken one alike.
**The tell is mechanical and it is the only one there is: the test names a
function it never calls.** Before writing a test, name the function under test
and check the body calls it. When reviewing one, MUST read what the assertion
reads FROM: a local the test itself filled is the defect, whatever the test is
called. A broad detector would also flag correct table-driven tests that build
local fixtures, so this stays a review obligation.
