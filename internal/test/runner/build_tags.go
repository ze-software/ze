// Design: docs/architecture/testing/ci-format.md -- conditions on the selected DUT
// Related: parallel.go -- admission before resource acquisition

package runner

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"unicode"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const optionNeedsBuildTag = "needs-build-tag"

// buildTagState is a declaration's required feature state. Zero is invalid.
type buildTagState uint8

const (
	buildTagUnspecified buildTagState = iota
	buildTagPresent
	buildTagAbsent
)

// buildTagRequirement is created only by the shared option parser. Its zero
// value is invalid; the private state is a validated present/absent choice.
type buildTagRequirement struct {
	name  string
	state buildTagState
}

// parseOptionNeedsBuildTag binds the declaration vocabulary to the manifest
// above this actual CI file, rather than the runner's working directory.
func parseOptionNeedsBuildTag(record *Record, ciFile string, kv map[string]string) error {
	name := kv[keyValue]
	if name == "" {
		return errors.New("option=needs-build-tag: value must name a feature build tag")
	}
	if !validDUTBuildTag(name) {
		return fmt.Errorf("option=needs-build-tag: invalid feature build tag %q", name)
	}
	var state buildTagState
	switch kv["state"] {
	case "present":
		state = buildTagPresent
	case "absent":
		state = buildTagAbsent
	default:
		return errors.New("option=needs-build-tag: state must be present or absent")
	}
	root, err := repoRootFrom(ciFile)
	if err != nil {
		return fmt.Errorf("option=needs-build-tag: locate the CI checkout: %w", err)
	}
	names, err := repofeaturetags.DaemonTags(root)
	if err != nil {
		return fmt.Errorf("option=needs-build-tag: read feature build tags: %w", err)
	}
	if !slices.Contains(names, name) {
		return fmt.Errorf("option=needs-build-tag: unknown feature build tag %q (declared in %s: %s)",
			name, featureGatesFile, strings.Join(names, ", "))
	}
	for _, requirement := range record.buildTags {
		if requirement.name == name {
			return fmt.Errorf("option=needs-build-tag: this test already declares %q; keep one present or absent condition per tag", name)
		}
	}
	record.buildTags = append(record.buildTags, buildTagRequirement{name: name, state: state})
	return nil
}

// dutBuildTags owns one selected artifact snapshot for one Run. Safe for
// concurrent use. An empty requirement set MUST NOT read the artifact; the
// lazy read is shared only by otherwise eligible declaring cases.
type dutBuildTags struct {
	path string
	read func(string) (*debug.BuildInfo, error)
	once sync.Once
	tags map[string]struct{}
	err  error
}

// newDUTBuildTags binds the final DUT path. A nil reader selects the real Go
// artifact reader; tests can model corrupt metadata without compiling a DUT.
func newDUTBuildTags(path string, read func(string) (*debug.BuildInfo, error)) *dutBuildTags {
	if read == nil {
		read = buildinfo.ReadFile
	}
	return &dutBuildTags{path: path, read: read}
}

// admit returns an empty reason only when execution is permitted. A nonempty
// reason is a known predicate mismatch; errors mean metadata could not answer.
func (dut *dutBuildTags) admit(requirements []buildTagRequirement) (string, error) {
	if len(requirements) == 0 {
		return "", nil
	}
	dut.once.Do(func() {
		info, err := dut.read(dut.path)
		if err != nil {
			dut.err = fmt.Errorf("read selected artifact: %w", err)
			return
		}
		dut.tags, dut.err = explicitDUTBuildTags(info)
	})
	if dut.err != nil {
		return "", fmt.Errorf("DUT build tags for %q: %w; rebuild the selected DUT with explicit Go -tags", dut.path, dut.err)
	}
	for _, requirement := range requirements {
		_, present := dut.tags[requirement.name]
		switch requirement.state {
		case buildTagPresent:
			if !present {
				return fmt.Sprintf("DUT %q does not satisfy option=needs-build-tag:value=%s:state=present (tag is absent)", dut.path, requirement.name), nil
			}
		case buildTagAbsent:
			if present {
				return fmt.Sprintf("DUT %q does not satisfy option=needs-build-tag:value=%s:state=absent (tag is present)", dut.path, requirement.name), nil
			}
		case buildTagUnspecified:
			return "", fmt.Errorf("DUT build tags for %q: condition %q has no validated state", dut.path, requirement.name)
		default:
			return "", fmt.Errorf("DUT build tags for %q: condition %q has invalid state %d", dut.path, requirement.name, requirement.state)
		}
	}
	return "", nil
}

// explicitDUTBuildTags accepts only a single explicit nonempty Go -tags setting.
// The manifest limits declarations, not artifact tags: personality tags and
// unrelated valid Go build tags are legitimate members of the observed set.
func explicitDUTBuildTags(info *debug.BuildInfo) (map[string]struct{}, error) {
	if info == nil {
		return nil, errors.New("artifact has no Go build information")
	}
	var value string
	found := false
	for _, setting := range info.Settings {
		if setting.Key != "-tags" {
			continue
		}
		if found {
			return nil, errors.New("artifact has duplicate Go -tags settings")
		}
		found = true
		value = setting.Value
	}
	if !found {
		return nil, errors.New("artifact has no explicit Go -tags setting; feature absence is unknown")
	}
	if value == "" {
		return nil, errors.New("artifact has an empty Go -tags setting")
	}
	tags := make(map[string]struct{})
	for tag := range strings.SplitSeq(value, ",") {
		if !validDUTBuildTag(tag) {
			return nil, fmt.Errorf("artifact has malformed Go build tag %q", tag)
		}
		if _, duplicate := tags[tag]; duplicate {
			return nil, fmt.Errorf("artifact repeats Go build tag %q", tag)
		}
		tags[tag] = struct{}{}
	}
	return tags, nil
}

// validDUTBuildTag uses Go's build-constraint tag alphabet. Input is bounded by
// the embedded -tags setting; no registry can enumerate all valid custom tags.
func validDUTBuildTag(tag string) bool {
	if tag == "" {
		return false
	}
	for _, letter := range tag {
		if unicode.IsLetter(letter) {
			continue
		}
		if unicode.IsDigit(letter) {
			continue
		}
		if letter == '_' {
			continue
		}
		if letter == '.' {
			continue
		}
		return false
	}
	return true
}
