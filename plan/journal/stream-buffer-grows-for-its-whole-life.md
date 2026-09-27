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
| 2026-09-27 | - | CLI live views, `outputBuf` (`internal/component/cli/model.go`) with monitor, ping and traceroute | Each batch appends to an unbounded strings.Builder, then `setViewportText(outputBuf.String())` sanitizes and sets the whole buffer again. Only `clear` resets it. A long `monitor event` session grows memory and per-batch CPU with no limit. Found by reading the code while comparing Ze with tuios, which caps scrollback at 10000 lines | not fixed |
