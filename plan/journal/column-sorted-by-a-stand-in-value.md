# Column Sorted by a Stand-in Value

A table orders its rows by a value the column does not show. The order is
deterministic, so nothing looks wrong, and the reader concludes the column
means something it does not.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-01 | - | cli dashboard | The Rate column of `monitor bgp` sorts by updates-received, not by the rate it prints (comparePeers, model_dashboard_sort.go). Two peers with one counter and two rates come out in an order the column contradicts. The measured rate lives in dashboardState.rates, which the comparison cannot reach | not fixed: found when the other columns were changed to sort by value |
