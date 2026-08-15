// Design: (none -- test utility, no architecture doc)

package golden

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PrePortRef is the commit that captured every fixture before the templ port.
// Both internal/component/web and internal/component/lg captured theirs there,
// in the wiring phase of plan/spec-web-templ-migration.md.
//
// It is a default, not a baseline. The ref is a parameter of the comparison, so
// a later port names its own commit and reuses the same instrument.
const PrePortRef = "80f0b8b57"

// portRef overrides PrePortRef. `make ze-templ-port-check REF=<sha>` passes it.
var portRef = flag.String("port-ref", "",
	"the git ref holding the fixtures a port is compared against, empty for golden.PrePortRef")

// PortRef returns the ref the comparison reads its pre-port bytes from.
func PortRef() string {
	if *portRef == "" {
		return PrePortRef
	}

	return *portRef
}

// PortKind says how one fixture is read.
type PortKind int

const (
	// PortMarkup is a rendered template body, the whole file.
	PortMarkup PortKind = iota
	// PortResponse is what Response writes: a status line, sorted headers, a
	// blank line, then the body.
	PortResponse
)

// AssertPortFidelity compares every fixture under root against the bytes the
// same path held at ref, through NormalizeHTML.
//
// It is the instrument AC-2 of plan/spec-web-templ-migration.md requires, and
// the reason it exists is that the comparison was hand-run once. A hand-run
// comparison cannot be repeated by a reader, and it cannot fail on the next
// change. This one takes the ref as a parameter and reads the fixtures on disk,
// so nothing here holds a copy that outlives the port.
//
// The fixtures on disk are the current render. make ze-web-golden-check
// re-renders every unit and compares it against them byte for byte. A fixture
// that has drifted from the code fails there, before it is read here.
//
// Three verdicts, and only one of them passes in silence.
//
//   - a path on both sides whose normalized forms differ is named, with the
//     first difference. That is the port breaking a unit.
//   - a path at ref with no fixture today is named. A unit that stops being
//     captured stops being compared, which is the cheapest way to make this
//     pass.
//   - a path with no counterpart at ref is reported and passes. A new unit
//     adds coverage and cannot regress the port.
//
// explained clears one unit of the first two verdicts, and it states why. A
// port restructures a component. A port also carries the odd deliberate
// change. So the answer cannot be that every difference is a defect.
//
// It is fail-closed in both directions. An entry naming a unit that does not
// differ is a finding. So is an entry naming a path neither side holds. An
// exemption that outlives what it exempted is how a table like this rots.
func AssertPortFidelity(t *testing.T, ref, root string, kind PortKind, explained map[string]string) {
	t.Helper()

	findings, notes, err := portFindings(t.Context(), ".", ref, root, kind, explained)
	if err != nil {
		t.Fatalf("compare %s against %s: %v", root, ref, err)
	}

	for _, note := range notes {
		t.Log(note)
	}

	for _, finding := range findings {
		t.Error(finding)
	}
}

// portFindings runs the comparison in dir and reports what it found. It returns
// the findings and the notes apart, so its own test can judge each one.
func portFindings(ctx context.Context, dir, ref, root string, kind PortKind, explained map[string]string,
) ([]string, []string, error) {
	before, err := gitFixtures(ctx, dir, ref, root)
	if err != nil {
		return nil, nil, err
	}

	after, err := diskFixtures(dir, root)
	if err != nil {
		return nil, nil, err
	}

	var (
		findings []string
		notes    []string
		used     = make(map[string]bool, len(explained))
		tb       textbuf.Buffer
	)

	for _, name := range sortedFixtureNames(before, after) {
		old, atRef := before[name]
		now, onDisk := after[name]

		reason, declared := explained[name]

		tb.Reset()

		switch {
		case atRef && onDisk:
			differences := comparePortUnit(name, kind, old, now)
			if len(differences) == 0 {
				continue
			}

			if !declared {
				findings = append(findings, differences...)

				continue
			}

			used[name] = true

			notes = append(notes, tb.Str(name).Str(": ").Str(reason).String())
		case atRef:
			if !declared {
				findings = append(findings, tb.Str(root).Str(" held ").Str(name).Str(" at ").Str(ref).
					Str(" and no fixture holds it today, so the unit is no longer compared").String())

				continue
			}

			used[name] = true

			notes = append(notes, tb.Str(name).Str(": ").Str(reason).String())
		default:
			notes = append(notes, tb.Str(name).Str(": new since ").Str(ref).
				Str(", so it has no pre-port bytes to match").String())
		}
	}

	return append(findings, staleExplanations(root, explained, used)...), notes, nil
}

// staleExplanations names each declared exemption that exempted nothing. An
// entry survives its unit either way: the difference it named is gone, or the
// path it named is.
func staleExplanations(root string, explained map[string]string, used map[string]bool) []string {
	var (
		findings []string
		tb       textbuf.Buffer
	)

	names := make([]string, 0, len(explained))
	for name := range explained {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if used[name] {
			continue
		}

		tb.Reset()

		findings = append(findings, tb.Str(root).Str(" explains ").Str(name).
			Str(", which matches no difference and no missing fixture, so the entry is stale").String())
	}

	return findings
}

// comparePortUnit compares one fixture. A response is split first: its status
// and its headers are bytes a handler chose, not markup an engine wrote, so
// they are compared exactly.
func comparePortUnit(name string, kind PortKind, old, now []byte) []string {
	if kind == PortMarkup {
		return portDifference(name, "", string(old), string(now), true)
	}

	oldHead, oldBody := splitResponse(old)
	nowHead, nowBody := splitResponse(now)

	findings := portDifference(name, "status and headers", oldHead, nowHead, false)

	// A body no engine rendered is compared byte for byte. JSON and an event
	// stream both reach this, and normalizing either one would erase a
	// difference rather than an encoding.
	markup := isHTMLResponse(nowHead) && isHTMLResponse(oldHead)

	return append(findings, portDifference(name, "body", oldBody, nowBody, markup)...)
}

// portDifference reports one difference, or nothing. The markup argument says
// whether the two sides are markup, which is the only case an encoding fold
// applies to.
func portDifference(name, part, old, now string, markup bool) []string {
	wantText, gotText := old, now
	if markup {
		wantText, gotText = NormalizeHTML(old), NormalizeHTML(now)
	}

	if wantText == gotText {
		return nil
	}

	var tb textbuf.Buffer

	tb.Str(name)

	if part != "" {
		tb.Byte(' ').Str(part)
	}

	tb.Str(" differs from its pre-port bytes\n").Str(firstDifference(wantText, gotText))

	return []string{tb.String()}
}

// firstDifference reports the offset of the first differing byte, with a window
// of each side around it. A failure then names the change, instead of printing
// two whole pages.
func firstDifference(want, got string) string {
	at := 0
	for at < len(want) && at < len(got) && want[at] == got[at] {
		at++
	}

	// Half a window of agreement before the difference gives the reader the
	// element it sits in. The rest of the window follows it.
	const window = 60

	from := max(at-window/2, 0)

	var tb textbuf.Buffer

	return tb.Str("first difference at byte ").Int(int64(at)).
		Str("\n  pre-port: ").Quoted(portWindow(want, from, at+window)).
		Str("\n  today:    ").Quoted(portWindow(got, from, at+window)).String()
}

// portWindow cuts a window out of one side, with both ends inside the string.
func portWindow(s string, from, to int) string {
	return s[min(from, len(s)):min(to, len(s))]
}

// splitResponse cuts a captured response into its head and its body. Response
// writes a blank line between them.
func splitResponse(raw []byte) (string, string) {
	head, body, found := strings.Cut(string(raw), "\n\n")
	if !found {
		return string(raw), ""
	}

	return head, body
}

// isHTMLResponse reports whether a captured head declares HTML. A head that
// declares nothing reads as not HTML, which compares its body exactly.
func isHTMLResponse(head string) bool {
	for line := range strings.SplitSeq(head, "\n") {
		if strings.HasPrefix(strings.ToLower(line), "header: content-type: text/html") {
			return true
		}
	}

	return false
}

// gitFixtures reads every file under root as it was at ref. One `git archive`
// answers the whole tree, so the walk costs one process rather than one per
// fixture. git writes each path relative to dir, which is the package the
// fixtures belong to.
func gitFixtures(ctx context.Context, dir, ref, root string) (map[string][]byte, error) {
	//nolint:gosec // the ref and the root are arguments of one git call, and no shell reads either
	cmd := exec.CommandContext(ctx, "git", "archive", ref, "--", root)
	cmd.Dir = dir

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git archive %s -- %s: %w: %s", ref, root, err, strings.TrimSpace(stderr.String()))
	}

	files := make(map[string][]byte)
	reader := tar.NewReader(bytes.NewReader(out))

	var tb textbuf.Buffer

	prefix := tb.Str(filepath.ToSlash(root)).Byte('/').String()

	for {
		header, readErr := reader.Next()
		if errors.Is(readErr, io.EOF) {
			break
		}

		if readErr != nil {
			return nil, fmt.Errorf("read the archive of %s at %s: %w", root, ref, readErr)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		content, readErr := io.ReadAll(reader)
		if readErr != nil {
			return nil, fmt.Errorf("read %s at %s: %w", header.Name, ref, readErr)
		}

		files[strings.TrimPrefix(header.Name, prefix)] = content
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%s holds no fixture at %s: the ref is wrong, or the capture is younger than it",
			root, ref)
	}

	return files, nil
}

// diskFixtures reads every fixture under root as it stands today.
func diskFixtures(dir, root string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	from := filepath.Join(dir, root)

	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		content, readErr := os.ReadFile(path) //nolint:gosec // path comes from a walk of the package's own testdata
		if readErr != nil {
			return readErr
		}

		name, relErr := filepath.Rel(from, path)
		if relErr != nil {
			return relErr
		}

		files[filepath.ToSlash(name)] = content

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", from, err)
	}

	return files, nil
}

// sortedFixtureNames returns every name either side holds, in one order, so a
// run reports the same findings in the same sequence.
func sortedFixtureNames(before, after map[string][]byte) []string {
	seen := make(map[string]bool, len(before)+len(after))

	names := make([]string, 0, len(before)+len(after))

	for _, side := range []map[string][]byte{before, after} {
		for name := range side {
			if seen[name] {
				continue
			}

			seen[name] = true

			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names
}
