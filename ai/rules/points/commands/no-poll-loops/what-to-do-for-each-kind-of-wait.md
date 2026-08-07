---
kind: table
level:
stage:
---
| Waiting for | Mechanism |
|-------------|-----------|
| A command this session launched in the background | Nothing. The completion notification is the wake-up |
| A file or a log line one of your own commands will produce | ONE bounded loop in `run_in_background`: `timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'`. It notifies once, then it is gone |
| A repeated event (every ERROR line, every CI step) | The `Monitor` tool, with `persistent` left false so its `timeout_ms` deadline applies. `persistent: true` disables that deadline and rebuilds the problem this rule exists to stop |
| Another session's `ze-verify` to release the lock | Do other work. `tmp/.ze-verify.lock.owner` names the holder, and `scripts/dev/verify-status.sh check` reports the last run's verdict. Never a watcher |
| Nothing in particular | Do not wait at all |
