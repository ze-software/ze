// Design: docs/architecture/testing/interop.md -- lab peer pins and their upstream
// Overview: upstream.go -- what produced the latest column.
//
// report.go holds what `le lab pin check` answers: one row per pin, with what
// upstream answered, and the report renders itself for a person.

package pin

import (
	"context"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// Row states, stable for `| match`.
const (
	// StateCurrent: the pin is upstream's latest stable release.
	StateCurrent = "current"
	// StateBehind: upstream has a newer stable release.
	StateBehind = "behind"
	// StateFloating: the pin is a moving tag such as latest.
	StateFloating = "floating"
	// StateUnresolved: upstream did not answer, so nothing is known.
	StateUnresolved = "unresolved"
)

// Row is one pin and upstream's answer for it.
type Row struct {
	Pin
	Kind   string `json:"kind"`
	Latest string `json:"latest,omitempty"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// Result is the whole answer of one check.
type Result struct {
	Rows   []Row `json:"pins"`
	Behind int   `json:"behind"`
}

// Compare asks upstream about every pin and answers one row per pin.
func Compare(ctx context.Context, up *upstream, pins []Pin) Result {
	var result Result
	for _, pin := range pins {
		row := up.judge(ctx, pin)
		if row.State == StateBehind {
			result.Behind++
		}
		result.Rows = append(result.Rows, row)
	}
	return result
}

// judge answers one pin's row.
func (u *upstream) judge(ctx context.Context, pin Pin) Row {
	row := Row{Pin: pin, Kind: pin.Kind.String()}
	latest, err := u.latest(ctx, pin)
	if err != nil {
		row.State, row.Detail = StateUnresolved, err.Error()
		return row
	}
	row.Latest = latest
	switch {
	case latest == "":
		row.State, row.Detail = StateUnresolved, "upstream lists no stable release in the pin's shape"
	case len(parseVersion(pin.Version).numbers) == 0 && pin.Kind == KindImage:
		row.State = StateFloating
	case latest == pin.Version:
		row.State = StateCurrent
	default:
		row.State = StateBehind
	}
	return row
}

// latest answers upstream's latest stable release for one pin.
func (u *upstream) latest(ctx context.Context, pin Pin) (string, error) {
	key := pin.Kind.String() + " " + pin.Name
	if err, failed := u.failed[key]; failed {
		return "", err
	}
	switch pin.Kind {
	case KindImage:
		tags, cached := u.tags[key]
		if !cached {
			fetched, err := u.registryTags(ctx, pin.Name)
			if err != nil {
				u.failed[key] = err
				return "", err
			}
			tags = fetched
			u.tags[key] = tags
		}
		return latestTag(pin.Version, tags), nil
	case KindGoModule:
		return u.remember(key, func() (string, error) { return u.goLatest(ctx, pin.Name, pin.Version) })
	case KindGitTag:
		return u.remember(key, func() (string, error) { return u.githubRelease(ctx, pin.Name) })
	case KindSourceCommit:
		return u.remember(key, func() (string, error) {
			latest, err := u.latestCommit(ctx, pin.Name)
			return latest.Commit, err
		})
	case KindUnspecified:
	}
	return "", nil
}

// remember asks once per key and keeps the answer.
func (u *upstream) remember(key string, ask func() (string, error)) (string, error) {
	if tags, cached := u.tags[key]; cached {
		return tags[0], nil
	}
	answer, err := ask()
	if err != nil {
		u.failed[key] = err
		return "", err
	}
	u.tags[key] = []string{answer}
	return answer, nil
}

// Text renders the result for a person: the rows behind first, then the
// moving tags, then what could not be resolved, then a count of the rest.
func (r Result) Text() string {
	var tb textbuf.Buffer
	tb.Str("# Lab peer pins against upstream latest stable\n\n")
	current := 0
	for _, state := range []string{StateBehind, StateFloating, StateUnresolved} {
		for _, row := range r.Rows {
			if row.State != state {
				continue
			}
			tb.Str("  [").Str(row.State).Str("] ").Str(row.File).Byte(':').Int(int64(row.Line)).
				Str(" ").Str(row.Kind).Str(" ").Str(row.Name).Str(" ").Str(display(row.Kind, row.Version))
			if row.Latest != "" {
				tb.Str(" -> ").Str(display(row.Kind, row.Latest))
			}
			if row.Detail != "" {
				tb.Str(" (").Str(row.Detail).Str(")")
			}
			tb.Byte('\n')
		}
	}
	for _, row := range r.Rows {
		if row.State == StateCurrent {
			current++
		}
	}
	tb.Str("\nPins read: ").Int(int64(len(r.Rows))).Str(", current: ").Int(int64(current)).
		Str(", behind: ").Int(int64(r.Behind)).Str("\n")
	if r.Behind > 0 {
		tb.Str("Fix: move each pin above to the version after the arrow; a source commit moves with ").
			Str(interoplab.SourceBumpCommand).Str("<Dockerfile>\n")
	}
	return tb.String()
}

// display shortens a commit for the report.
func display(kind, value string) string {
	if kind == KindSourceCommit.String() {
		return interoplab.ShortCommit(value)
	}
	return value
}
