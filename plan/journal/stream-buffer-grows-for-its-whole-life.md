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
| 2026-09-27 | spec-web-cli-config-one-off-defects (AC-22) | CLI monitor/command output and ping/traceroute log views | outputBuf kept every event and rendered the whole growing transcript | open: model_output.go bounds raw and sanitized display to 512 KiB with UTF-8-safe eviction, visible omission and clear. All four retention roots and full CLI race pass; monitor/save UI controls pass. Measured coverage and final lint/docs acceptance pending. |
