// Design: docs/architecture/testing/tracked-build-gate.md -- the tracked-build gate's answer
//
// report.go holds what `le repository-tracked-build check` ANSWERS, apart from
// what produced it.
//
// The payload is an object, because the counts and the commit are the point: a
// run that compiled six flavors and a run that judged none both list zero
// failures, and only the fields tell them apart. One key holds rows, so the row
// operators act on the flavors.
//
// The DIAGNOSIS is not part of the answer. It is written to stderr by the
// action, exactly as the script wrote it, because it tells a person what to do
// next rather than saying what is true of the commit.

package repositorytrackedbuild

import "github.com/ze-software/ze/internal/core/textbuf"

// Result is one flavor's outcome, and it is one ROW of the report.
type Result struct {
	Name     string  `json:"name"`
	Tags     string  `json:"tags"`
	GOOS     string  `json:"goos,omitempty"`
	Anchor   string  `json:"anchor"`
	Packages int     `json:"packages"`
	OK       bool    `json:"ok"`
	Seconds  float64 `json:"seconds"`
	Output   string  `json:"output,omitempty"`
}

// Report is the whole answer of one run.
type Report struct {
	Rev      string   `json:"rev"`
	Commit   string   `json:"commit"`
	Tree     string   `json:"tree"`
	Features []string `json:"features"`
	Results  []Result `json:"results"`
	// PackageFloor is published so a green run pasted as evidence carries the
	// threshold it was judged against. A lowered floor makes a green
	// indistinguishable from a real one unless the number travels with it.
	PackageFloor int `json:"package-floor"`
	// Incomplete means the run stopped before judging every flavor, so OK is a
	// statement about the flavors listed and about nothing else.
	Incomplete bool `json:"incomplete,omitempty"`
	// Keep says the extracted tree was left in place, which changes the remedy
	// a reader is given.
	Keep bool `json:"keep,omitempty"`
	OK   bool `json:"ok"`
}

// The column widths the flavor table is printed in. They keep six rows aligned,
// which is the whole reason a reader can scan the page.
const (
	// shortCommit is how many characters of the sha the page prints.
	shortCommit = 12
	// nameWidth is the flavor-name column, left-aligned.
	nameWidth = 12
	// secondsWidth is the elapsed-time column, right-aligned.
	secondsWidth = 5
	// packagesWidth is the package-count column, right-aligned.
	packagesWidth = 4
)

// Text renders the run for a person: the commit line, one row per flavor, and
// the verdict. It ends in a newline.
//
// The DIAGNOSIS that follows a failure is Diagnosis, written to stderr: it is
// advice rather than an answer, and a caller piping this into `| json` has no
// use for it.
func (r Report) Text() string {
	var tb textbuf.Buffer
	short := r.Commit
	if len(short) > shortCommit {
		short = short[:shortCommit]
	}

	tb.Str("tracked-build: ").Str(r.Rev).Str(" (").Str(short).Str("), ").
		Int(int64(len(r.Features))).Str(" feature tags, ").Str(buildPackages)
	switch {
	case r.PackageFloor < DefaultPackageFloor:
		// Say so loudly: the shrink detector was WEAKENED for this run, and a
		// green line pasted as evidence would otherwise not show it.
		tb.Str(", package floor LOWERED to ").Int(int64(r.PackageFloor))
	case r.PackageFloor > DefaultPackageFloor:
		tb.Str(", package floor raised to ").Int(int64(r.PackageFloor))
	}
	tb.Byte('\n')

	for _, result := range r.Results {
		state := "OK  "
		if !result.OK {
			state = "FAIL"
		}
		tb.Str("  ").Str(state).Byte(' ').PadRight(result.Name, nameWidth).Byte(' ').
			PadLeft(seconds(result.Seconds), secondsWidth).Str("s  ").
			PadLeft(count(result.Packages), packagesWidth).Str(" pkgs  -tags '").Str(result.Tags).Str("'\n")
	}

	if r.OK {
		tb.Str("tracked-build: OK (every flavor of the committed tree compiles)\n")
	}
	return tb.String()
}

// Diagnosis renders what a reader does next about a failing run. It is empty
// for a run that passed, and it ends in a newline.
func (r Report) Diagnosis() string {
	if r.OK {
		return ""
	}

	var tb textbuf.Buffer
	if r.Incomplete {
		tb.Str("\ntracked-build: INCOMPLETE. The flavors listed were judged; the rest were not.\n")
		tb.Str("  A FAIL below is a real break. The absence of one is not a clean commit.\n")
	} else {
		tb.Str("\ntracked-build: the tree GIT HOLDS does not compile.\n")
	}

	// Printed for an incomplete run too: a flavor that failed BEFORE the
	// deadline found a real break, and its compiler output names the symbol.
	for _, result := range r.Results {
		if result.OK {
			continue
		}
		tb.Str("\n  --- ").Str(result.Name).Str(" --- ").Str(buildMatrix.whyOf(result.Name)).Byte('\n')
		for line := range splitLines(result.Output) {
			tb.Str("    ").Str(line).Byte('\n')
		}
	}
	if r.Incomplete {
		return tb.String()
	}

	tb.Str("\n  Your working tree compiles; the commit does not. The usual cause is a\n")
	tb.Str("  CONSUMER committed without its PRODUCER: a symbol named above still lives\n")
	tb.Str("  in a file that is untracked, or modified but not committed.\n")
	tb.Str("    git status --short          # find the file holding the named symbol\n")
	tb.Str("    git log -1 --stat           # what the last commit actually took\n")
	tb.Str("  Commit the producer. Do not revert the consumer.\n")
	if !r.Keep {
		tb.Str("  To keep the extracted tree for inspection, set ").Str(KeepKey).Str("=true.\n")
	}
	return tb.String()
}
