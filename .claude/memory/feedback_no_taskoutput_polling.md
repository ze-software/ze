---
name: Never stack TaskOutput polls on long-running background commands
description: Stop polling TaskOutput with multiple back-to-back long timeouts; read the log directly instead
type: feedback
originSessionId: be717b13-9ccf-4314-b8c1-022885c5b6bf
---
When a Bash command runs in the background (`run_in_background: true`),
do NOT call TaskOutput repeatedly with 600s timeouts in succession.
That pattern stacked SIX 10-minute polls (60 minutes total) on a 4-minute
job because the prior job in the queue was still running.

**Why:** The runtime sends a notification when the background job
completes -- exactly one. Calling TaskOutput inside a fresh long-timeout
window after that notification has been delivered just blocks for the
full timeout with no signal to break out.

**How to apply:**

1. After `run_in_background: true`, do NOT immediately call TaskOutput
   with a long timeout. Either work on something else, or call
   TaskOutput with a short timeout (60-120s) and on timeout READ
   `tmp/<your-log>.log` directly to inspect progress.
2. If the log shows the job is still in queue (waiting for
   `tmp/.ze-verify.lock`), do not keep polling -- check the lock owner
   via `ps aux | grep verify-lock` and read the active log to estimate
   ETA.
3. The verify lock chain (`scripts/dev/verify-lock.sh`) means a second
   `make ze-verify-fast` queues behind any prior call, including ones
   from earlier in the same session that you forgot. ALWAYS check
   `ps aux | grep verify-lock` before kicking off a new verify; if any
   `flock` is running, your new call will wait for ALL ahead of it.
4. The notification system delivers exactly one event per background
   job. Stacking polls inside successive 10-min windows means you
   miss the notification window and just burn wall clock.

**Past failure:** 2026-04-19 verify-fast wait, six TaskOutput calls
each with 600s timeout, 60 minutes wasted while job was queued behind
two prior verify runs from earlier in the session.
