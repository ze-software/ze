# Column Sorted by a Stand-in Value

A table orders its rows by a value the column does not show. The order is
deterministic, so nothing looks wrong, and the reader concludes the column
means something it does not.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-01 | - | cli dashboard | The Rate column of `monitor bgp` sorted by updates-received, not by the rate it prints (comparePeers, model_dashboard_sort.go). Two peers with one counter and two rates came out in an order the column contradicts. The measured rate lived in dashboardState.rates as a formatted string, so the number was discarded and the comparison had nothing to read | fixed on the owner's instruction, in the change after the one that found it. peerRateEntry holds the rate as the number measured, peerRate formats it where the table prints it, and the sort takes dashboardState.rateValue as a lookup. A peer with no measured rate orders after every peer that has one, rather than reading as a rate of zero |
