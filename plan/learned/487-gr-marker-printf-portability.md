# 487 -- GR Marker Printf Portability

## Context

The `gr-marker-restart` and `gr-marker-expired` functional tests were failing silently. The tests create a binary GR marker file using a shell script (`printf '\x00...'`), but `/bin/sh` on the CI host does not support `\x` hex escapes in printf. The marker file contained 32 bytes of literal ASCII (`\x00\x00...`) instead of 8 bytes of binary, causing `grmarker.Read` to reject it (wrong length). A separate test (`inactive-peer`) was also failing because peer validation was tightened to require `local { ip auto }` after the test was written.

## Decisions

- Used POSIX octal escapes (`\000`) over hex escapes (`\x00`) because `\x` is a bash extension, not guaranteed in `/bin/sh`. Octal is POSIX-portable.
- Fixed the `inactive-peer` test to include the now-required `local { ip auto }` block rather than relaxing the validation.

## Consequences

- All `.ci` scripts that write binary data via `/bin/sh` must use octal escapes, never hex.
- Tests that exercise config parsing can break when validation is tightened in later commits. New validation rules should grep for affected `.ci` files.

## Gotchas

- The failure was invisible: `grmarker.Read` returned false (marker too long), the else branch was silent, and the test failed on a byte mismatch in the OPEN message. Required adding temporary debug logging to trace the actual root cause.
- `printf '\x41'` works in bash but produces literal `\x41` in dash/sh. No warning, no error.

## Files

- `test/plugin/gr-marker-restart.ci` -- octal escapes
- `test/plugin/gr-marker-expired.ci` -- octal escapes
- `test/parse/inactive-peer.ci` -- add missing `local { ip auto }`
