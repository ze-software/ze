---
kind: directive
level: MUST
stage:
---
**A change to what the daemon PUTS ON THE WIRE, installs, or shows MUST run the
functional suite that owns that surface before commit. The package's unit tests
are not that evidence.** A unit test proves the function answers correctly. Only a
running daemon proves the rail carries the answer to a peer.

**The fixture that catches the regression is named after ANOTHER feature.** A rail
every feature crosses is observed by every fixture that crosses it, so the file
that goes red carries a name with no connection to what you changed. Searching the
suite for a fixture named after your change finds nothing, and finding nothing is
not evidence. Run the whole suite that owns the surface.

**A guard is the case this bites hardest.** It changes the answer for every caller
of the rail at once, including the callers whose fixtures were written before it
existed and assert the answer it now refuses.
