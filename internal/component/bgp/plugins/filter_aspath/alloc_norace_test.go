//go:build !race

package filter_aspath

// regexPoolRaceOverhead is zero without the race detector: the regexp
// matcher pool keeps what it is handed, and a warm run allocates nothing.
const regexPoolRaceOverhead = 0
