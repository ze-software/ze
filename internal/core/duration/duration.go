// Design: docs/architecture/diagnostics/debug-filtering.md -- shared duration parsing for CLI input

package duration

// ParseMinutes parses a duration string with an explicit unit suffix into minutes.
// Accepted: "30m" (minutes), "1h" (hours), "90s" (seconds, rounded up), "0" (zero/disabled).
// Bare numbers without a unit are rejected to prevent ambiguity.
func ParseMinutes(s string) (int, bool) {
	if s == "0" {
		return 0, true
	}
	if len(s) < 2 {
		return 0, false
	}

	unit := s[len(s)-1]
	digits := s[:len(s)-1]

	n := 0
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > 1<<31-1 {
			return 0, false
		}
	}

	switch unit {
	case 's':
		return (n + 59) / 60, true
	case 'm':
		return n, true
	case 'h':
		if n > (1<<31-1)/60 {
			return 0, false
		}
		return n * 60, true
	default:
		return 0, false
	}
}
