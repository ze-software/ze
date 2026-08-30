---
kind: directive
level: MUST NOT
stage:
---
**Without explicit user authorization, test data (a golden file, expected output, a fixture, a `.ci` expectation) MUST NOT be modified to make a failing test pass.**
When output changes, the default assumption is that the code is
wrong, not the data. MUST ask the user before updating any test data, even when
the new output looks plausible.
