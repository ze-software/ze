//go:build race

package filter_aspath

// regexPoolRaceOverhead is what the race detector adds to the allocation
// count of one run. The regexp package keeps its matcher state in a
// sync.Pool, and under -race sync.Pool.Put drops a random quarter of what it
// is handed (src/sync/pool.go, "Randomly drop x on floor"), so a run then
// allocates a fresh matcher about one time in four. It read 1.0 per run on
// 2026-11-01; 2 leaves room for the variance of the random drops. The pass
// without -race reads the exact 0.
const regexPoolRaceOverhead = 2
