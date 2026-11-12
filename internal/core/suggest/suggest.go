// Design: (none — utility package)
//
// Package suggest provides "did you mean?" suggestions for CLI commands.
package suggest

// Command returns the closest match from candidates for the given input,
// or "" if no candidate is close enough (distance > len(input)/2).
func Command(input string, candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	best := ""
	bestDist := len(input)/2 + 1 // max acceptable distance
	for _, c := range candidates {
		d := levenshtein(input, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

// Correction returns the accepted value an operator most likely meant by
// input, or "" when none is close enough to put in a fix line, which a
// refusal then writes as the alternation of every value.
//
// The rule: a value is proposed only within one edit for an input of up to
// four characters, and within a third of the input's length (rounded down)
// beyond that. It is tighter than Command's half, because a correction goes
// into the line the operator types: "true" is two edits from "blue", and
// `fix: color blue;` would state a value nobody meant, while "requre" (one
// edit from "require") and "custmer" (one from "customer") stay corrected.
func Correction(input string, candidates []string) string {
	distMax := 1
	if len(input) > 4 {
		distMax = len(input) / 3
	}
	best := ""
	bestDist := distMax + 1
	for _, c := range candidates {
		d := levenshtein(input, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
