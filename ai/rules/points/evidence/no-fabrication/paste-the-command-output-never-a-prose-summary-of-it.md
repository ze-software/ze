---
kind: directive
level: MUST
stage:
---
**When a claim's evidence is what a command printed, you MUST write the command or paste
what it printed. You MUST NOT write a sentence describing the output.** "`git grep -n
familiesSent -- '*.go'` returns nothing" is evidence because the reader can run it.
"The grep returns only the guard's own literal" is a claim about a command made
from memory.

This is the same discipline as reading the producing function, applied to your own
terminal. A sentence about output you did not re-read is a recollection, and a
recollection presented in an evidence cell is indistinguishable from a measurement.

| Writing this | Instead |
|--------------|---------|
| "the suite passes" | the target's own verdict line, pasted |
| "18 files match" | the command, so the count is re-derivable, and the date it was true |
| "this block is the whole output" | say what was cut, or make no claim about completeness. An exhaustiveness claim over hand-edited text needs re-checking every time the text moves |
| "the anchors are unaffected" | name each anchor and what it asserts |

**A number you did not just compute is the highest-risk sentence in a closure
record.** Counts drift as the work continues: a survey run before you added a file
no longer describes the tree. Re-run it, or date it.
