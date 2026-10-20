| Test | Reason |
|---|---|
| readLinuxLimits | Renamed, not a weakening: its parsing moved into parseLinuxLimits(page), which takes the register text so TestParseLinuxLimitsForm can feed it malformed pages. Every check it ran (LL- heading, Kind, Checked form) still runs, now as a returned problem that TestLinuxLimitsCheckedAtShippedVersions reports, and the parser adds the Effect, Status, repeat, stray-heading and date checks. |
