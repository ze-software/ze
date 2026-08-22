# A bound that adds before it compares is defeated by the value it bounds

A guard reads a length or a count out of untrusted input, adds a header or an
offset to it, and compares the SUM against a maximum. Every ordinary input is
refused correctly. One input near the top of the integer's range wraps the sum
to a small number, passes the comparison, and the guard is inoperative for
exactly the value it exists to refuse.

The repair is to bound the value the input STATES, before any arithmetic touches
it, and to return it unmodified once it fails. Then read the CALLERS: one
guarded addition proves nothing if a caller adds an offset to the result. The
test case is the type's limit, never a plausible large number.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | record-answers-2-only-encoding | `answerFieldWidth` (`pkg/plugin/rpc/message.go`), the width a counted text states | the function returned `uint64(header) + size` for a counted text whose `size` came off the wire unbounded. A count near the range of a uint64 wraps that sum to a number under 21, which is inside `MaxMessageSize`, so `answerLineWidth` reported a small width and `scanStatedLine` never reached its refusal. Measured before the fix: `#7 row 18446744073709551595:x` stated a width of 7. It is reachable from any peer on the plugin mux connection and from the SSH exec channel. It cannot be steered onto a newline today, because the wrapped width always lands inside the count's own 21-byte header, so the line was refused for the wrong reason rather than mis-framed. The bound was still inoperative. Found by the independent review in the NEWEST layer of the change, not in the parts three phases had already reversed and re-read | fixed in `7c58d7487`. A count past `MaxMessageSize` is reported as stated and the header is never added to it, and `answerLineWidth` returns that width rather than `at + width`, which was the second place the same sum could wrap. `TestCountedTextPastTheMaximumDoesNotWrapItsWidth` (`pkg/plugin/rpc/framing_test.go`) is the regression, mutation-verified: removing the guard reddens it with `"7" is not greater than "16777216"` |
