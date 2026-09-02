# A failing gate prints no cause

A command runs a gate, the gate fails, and the operator gets the verdict with
nothing under it. The cause exists: the gate built a structured report and
named the assertion that failed. Something between the gate and the terminal
dropped it, usually a wrapper whose own summary assumed the gate had already
written to the terminal itself.

The tell is a wrapper that renders only its own summary and stores the inner
answer in a field nothing reads on the default path. Ask, of every layer above
a gate: when this layer prints, has the layer under it printed anything at all?

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-01 | eap-notification-and-nak | `le integration`, a native gate's report | `Area.Sweep` (`internal/le/leaction/leaction.go`) keeps each action's answer in `SweepRow.Answer` and renders only `Sweep.Text`, whose comment reads "Each action's own output has already streamed to the terminal by the time this is read". That holds for a `commandRunner` action, which streams a subprocess. It is false for a `nativeRunner` action, whose answer is a `leroot.Prose` report nothing else prints. Both integration gates are native, so a failing `./le integration interop-ipsec` printed exactly two lines, "Failed: interop-ipsec", while `ipsec.Report.Text` held the scenario name and the assertion that timed out. Measured here: the same run through the `json` pipe operator answered `wait for strongswan log "EAP/RES/NAK" timed out before the peer became ready`. The same holds for `./le integration interop` | not fixed. It blocked nothing: the `json` pipe operator renders the payload the gate already returns, and every RED in this spec's discrimination proof was read that way. The repair is a decision rather than a line, because the sweep cannot tell a streaming action from a Prose one without asking the action, and printing an inner report for every row would double the output of the areas whose actions do stream |
| 2026-09-02 | none (dependabot alert 28) | `le verify deps vulnerability`, a native gate's report | `./le verify deps vulnerability` printed one blank line and "Failed: vulnerability", and exited 127. The cause was that `govulncheck` is not on PATH, which a shell reports as 127 and the sweep discards with everything else the action said. The action's own description states the tool must be installed ("scan the Linux/amd64 dependency graph with an installed govulncheck"), and `./le setup --check` names no govulncheck, so a fresh machine reaches this gate with the tool absent and is told only that a stage failed. `go install golang.org/x/vuln/cmd/govulncheck@latest` made the same command pass | not fixed. It blocked nothing here: installing the tool cleared it, and the dependency bump the gate was asked to judge is verified. The row sits in this class rather than in a setup class, because the missing tool is nameable in one line and the gate withheld that line |
