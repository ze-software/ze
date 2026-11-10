# A warning repeated on every retry or every dropped item

A loop that retries, or a queue that drops, logs one Warn line per attempt or
per item with no bound. One lasting condition (an unreachable server, a slow
disk) then writes a line every second or every record, and buries the lines
that name a new problem. The condition needs one line when it starts, a count
while it lasts, and a doctor row that reports it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-10 | doctor-coverage-14-system-plugins | NTP initial sync, `syncWorker.run` and `doSync` (`internal/plugins/ntp/ntp.go`) | Phase 2 retries `doSync` every second until the first sync, and each try Warns once per failing server plus "no reachable servers" (or "no servers configured"), so an unreachable server writes several Warn lines a second until it answers | open: Warn on a server's first failure and on each change of its error, keep the rest in `serverState`, and report the lasting state through the NTP runtime doctor check |
| 2026-11-10 | doctor-coverage-14-system-plugins | MRT async writer, `asyncWriter.Write` and `drain` (`internal/plugins/mrt/async_writer.go`) | A full write channel Warns once per dropped record, and a failing file write Warns once per record, so a slow or full disk writes one Warn line per BGP message | open: count drops and write errors in the writer, Warn on the first of a run with the count at its end, and report both through the MRT runtime doctor check |
