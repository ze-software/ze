---
kind: note
level:
stage:
---
NEVER modify test data (golden files, expected output, fixtures, `.ci` expectations) to make a failing test pass without explicit user authorization. When output changes, the default assumption is that the code is wrong, not the test data. Ask the user before updating any test data, even if the new output looks plausible.
