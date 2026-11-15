---
spec: runtime-peer-plugin-capabilities
topic: Plugins
feature: configuration-reload
found-by: review
---

## Before

A reload that added a BGP peer carrying a plugin capability could be refused.
Capability edits could leave a running session using its previous declaration.
A reconnect during a failed reload could also be compared with a declaration
that differed from the OPEN actually sent.

## After

Accepted reloads install each plugin's candidate declaration before peers
apply it. New peers send their configured capabilities. A changed capability
restarts a session when the peer advertised that code; otherwise the session
continues. A failed reload restores the previous declarations. Each OPEN and
its retained declaration now come from one snapshot, so rollback decisions
describe the bytes sent. A plugin's own configuration refusals still apply.

## Corrects

The 2026-01-19 update described live peer addition. This change makes that
behavior apply to accepted plugin-capability declarations too.

## Exposure

none

## Upstream

none

## Asked by

none
