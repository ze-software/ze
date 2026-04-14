---
name: gokrazy appliance deployment
description: Ze targets gokrazy appliance with no systemd/OS - must own full process lifecycle for VPP
type: project
---

Ze deploys as a gokrazy appliance. gokrazy has no systemd, no init system, no package manager.
Ze must own the full lifecycle of any external process it depends on (like VPP).

**Why:** "we create an appliance with gokrazy which has no systemd or os system so we have no choice but to take ownership"

**How to apply:** When designing process management for VPP or any future external dependency,
ze must exec, supervise, and clean up the process itself. Never assume systemd, supervisor,
or any OS-level process manager is available.
