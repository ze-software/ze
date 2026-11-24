# A buffer fed by a stream keeps every line the stream ever sent

A view that shows a live stream appends each new line to one buffer and hands
the whole buffer to the renderer on every update. Nothing removes old lines, so
memory grows for as long as the stream runs, and so does the work each update
does. A short test never sees it. It shows up on a monitor that was left open
for hours.

The fix is a bound chosen on purpose, such as a ring of lines with a cap, and
an update that touches only what changed.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-27 | spec-web-cli-config-one-off-defects (AC-22) | CLI monitor/command output and ping/traceroute log views | outputBuf kept every event and rendered the whole growing transcript | fixed: `b49fbcf2f8` bounded `outputBuffer` to 512 KiB with UTF-8-safe eviction; `TestOutputBuffer*` passed in `internal/component/cli` (-race). |
