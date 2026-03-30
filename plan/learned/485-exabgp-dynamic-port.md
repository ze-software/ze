# 485 -- ExaBGP Dynamic Port Allocation

## Context

ExaBGP compatibility tests used a static `Port` class that handed out sequential port numbers starting from 1900. This caused port collisions when running concurrent test instances, and the hardcoded port range could conflict with user services. The goal was to replace this with OS-assigned dynamic ports via binding to port 0.

## Decisions

- Chose server-reports-port-via-stdout over environment variables or return channels because the test architecture already captures subprocess stdout to temp files, making the data immediately accessible to the parent process.
- Used `PORT <N>` as a simple sentinel line format over JSON or structured output because the port discovery only needs a single integer and the temp file already contains unstructured text output.
- Changed `--port` CLI default from 1900 to None (dynamic) over keeping 1900 as default because static defaults defeat the purpose of dynamic allocation for normal test runs.
- Added `State.FAIL` skip in client start loop over allowing client start to proceed because starting a client against port 0 (failed discovery) would always fail and waste time.

## Consequences

- Concurrent test runs no longer risk port collisions -- each server gets a unique OS-assigned port.
- The concurrent-run warning no longer mentions port conflicts (only process cleanup and resource contention remain as concerns).
- Manual `--server` mode now shows the assigned port in output, requiring the user to pass it explicitly to `--client`. This is a minor workflow change but aligns with the dynamic port model.
- Retry logic gets fresh ports per retry, preventing stale port reuse after test failures.

## Gotchas

- The `bgp` mock prints diagnostic messages (sink/echo mode announcements) before the PORT line. The `_discover_port` method correctly handles this by scanning for lines starting with `PORT ` rather than reading the first line.
- The stdout temp file is opened in binary mode (`w+b`) by `Exec.run()`, but `_discover_port` reads it in text mode (`r`). This works because Python handles the text/binary conversion transparently for ASCII content.
- The `SO_REUSEPORT` socket option is set but does not interfere with port 0 binding -- the OS still assigns unique ports.

## Files

- `test/exabgp-compat/bin/bgp` -- port 0 validation, `PORT <N>` output after bind
- `test/exabgp-compat/bin/functional` -- `Port` class removed, `_discover_port()` added, `run_selected()` and `_retry_failed_tests()` updated
- `docs/functional-tests.md` -- CLI reference and dynamic port section
