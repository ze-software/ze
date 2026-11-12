---
spec: doctor-coverage-12-plugin-infra
topic: Diagnostics
feature: system-readiness
---

## Before

When a plugin stopped, crashed, lost its event subscriptions, missed a
configuration commit or failed in one of many other ways, `ze doctor` said
nothing: the only record was a log line. A plugin that stayed down could be brought back only by restarting ze. A
plugin whose failure policy stopped ze left no trace for the next start, a
refused configuration reload was a log line only, and an environment variable
holding a value its type could not parse was silently replaced by the default.
A plugin running as a separate process had no way to report a problem to the
daemon. A managed configuration hub accepted a client that presented a name it
did not know with an empty token.

## After

`ze doctor` and `show doctor` report each of these problems with its cause and
the command that fixes it: a configured plugin that is down and why, a plugin
process that ended unasked, the plugin whose failure stopped ze (read on the
next run), a plugin whose setup failed, plugin dependencies this build cannot
resolve, dropped subscriptions, a post-startup callback never delivered, a
recovered panic, failed or slow event delivery, an unavailable `show` enricher,
a configuration commit a plugin was not told about, an external plugin name the
connect-back refuses, a plugin that only works in-process configured as
external, a managed listener that could not bind, a managed client that
rejected its configuration, a removal or capability restore a refused reload
left undone, a failed audit write, a producer with no data-plane writer, an
unusable environment value, a full report bus and a refused configuration
reload. The new command `ze request plugin restart <name>` starts a down plugin
again, or stops and restarts a running one, and is the fix line those rows
print. A plugin in a separate process can raise and clear its own problems on
the daemon's report bus through the SDK. The managed hub no longer
authenticates an empty token.

## Corrects

none

## Exposure

A hub running the managed configuration server accepted a client that sent a
name the hub did not know and an empty token. Edges, and daemons with no
managed server configured, were not exposed.

## Upstream

none

## Asked by

none
