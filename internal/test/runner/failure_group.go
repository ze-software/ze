// Design: docs/functional-tests.md -- functional failure routing groups
// Related: display.go -- suite summaries and rerun hints
package runner

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	nativeFailureGroupPrefix = "VERIFY FAILURE GROUP:"
	defaultFailureSuite      = "encode"
	editorSuite              = "editor"
)

// failureGroup is the native suite-local failure routing unit emitted by ze-test.
type failureGroup struct {
	Stage     string    `json:"stage"`
	GroupID   string    `json:"group-id"`
	Kind      string    `json:"kind"`
	Related   []string  `json:"related"`
	Summary   string    `json:"summary"`
	Rerun     string    `json:"rerun"`
	DetailLog string    `json:"detail-log"`
	Parallel  string    `json:"parallel"`
	HostLoad  *HostLoad `json:"host-load,omitempty"`
}

// groupFunctionalFailures groups failed records by suite, failure kind, and a
// suite-specific routing key. When load is non-nil and contended, each group
// gets the load context for downstream classification.
func groupFunctionalFailures(suite string, records []*Record, load *HostLoad) []failureGroup {
	if suite == "" {
		suite = defaultFailureSuite
	}
	byKey := map[string]*failureGroup{}
	order := []string{}
	subjects := map[string]string{}
	for _, rec := range records {
		kind := recordFailureKind(rec)
		target := failureGroupTarget(suite, rec)
		var tb textbuf.Buffer
		key := tb.Str(suite).Byte(':').Str(kind).Byte(':').Str(target.Key).String()
		group, ok := byKey[key]
		if !ok {
			group = &failureGroup{
				Stage:    suite,
				GroupID:  key,
				Kind:     kind,
				Parallel: "group",
			}
			byKey[key] = group
			order = append(order, key)
			subjects[key] = target.Summary
		}
		group.Related = append(group.Related, target.Related)
	}
	var contendedLoad *HostLoad
	if load != nil && load.Contended() {
		contendedLoad = load
	}
	groups := make([]failureGroup, 0, len(order))
	for _, key := range order {
		group := byKey[key]
		group.Rerun = FormatRerunCommand(group.Stage, group.Related)
		var stb textbuf.Buffer
		group.Summary = stb.Str(group.Stage).Byte(' ').Str(group.Kind).Byte(' ').Str(subjects[key]).Str(": ").Int(int64(len(group.Related))).Str(" test(s)").String()
		group.HostLoad = contendedLoad
		groups = append(groups, *group)
	}
	return groups
}

type groupTarget struct {
	Key     string
	Summary string
	Related string
}

func failureGroupTarget(suite string, rec *Record) groupTarget {
	if usesFullNameGrouping(suite) {
		name := strings.TrimSpace(strings.TrimSuffix(rec.Name, ".ci"))
		if name == "" {
			name = rec.Nick
		}
		related := rec.Nick
		if suite == editorSuite {
			related = name
		}
		if related == "" {
			related = name
		}
		return groupTarget{
			Key:     normalizeGroupToken(name),
			Summary: name,
			Related: related,
		}
	}

	prefix := subsystemPrefix(rec.Name)
	related := rec.Nick
	if related == "" {
		related = prefix
	}
	return groupTarget{
		Key:     prefix,
		Summary: prefix,
		Related: related,
	}
}

func usesFullNameGrouping(suite string) bool {
	switch suite {
	case "decode", editorSuite, "firewall", "install", "l2tp", "managed", "parse", "policy", "ui", "web":
		return true
	default:
		return false
	}
}

func recordFailureKind(rec *Record) string {
	if rec.State == StateTimeout {
		return stateTimeout
	}
	kind := rec.FailureType
	if kind == "" {
		kind = stateUnknown
	}
	return normalizeGroupToken(kind)
}

func subsystemPrefix(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return stateUnknown
	}
	name = strings.TrimSuffix(name, ".ci")
	for _, sep := range []string{"/", "-", "_", "."} {
		if idx := strings.Index(name, sep); idx > 0 {
			name = name[:idx]
			break
		}
	}
	return normalizeGroupToken(name)
}

var groupTokenRE = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeGroupToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = groupTokenRE.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return stateUnknown
	}
	return value
}

// FormatRerunCommand returns the smallest suite-specific rerun command.
func FormatRerunCommand(suite string, args []string) string {
	if suite == "" {
		suite = defaultFailureSuite
	}
	command := []string{"ze-test"}
	switch {
	case suite == editorSuite:
		command = append(command, editorSuite)
		command = append(command, args...)
	case isBGPSuite(suite):
		command = append(command, "bgp", suite)
		command = append(command, args...)
	default:
		command = append(command, suite)
		command = append(command, args...)
	}
	return textbuf.Join(quoteCommand(command), " ")
}

func formatRecordRerunCommand(suite string, rec *Record) string {
	if rec == nil {
		return FormatRerunCommand(suite, nil)
	}
	if suite == editorSuite {
		arg := rec.Nick
		if arg == "" {
			arg = strings.TrimSpace(rec.Name)
		}
		return FormatRerunCommand(suite, []string{arg})
	}
	arg := rec.Nick
	if arg == "" {
		arg = strings.TrimSpace(rec.Name)
	}
	return FormatRerunCommand(suite, []string{arg})
}

func isBGPSuite(suite string) bool {
	switch suite {
	case defaultFailureSuite, "plugin", "decode", "parse", "reload", "chaos-web", "chaos":
		return true
	default:
		return false
	}
}

func supportsServerClientDebug(suite string) bool {
	switch suite {
	case defaultFailureSuite, "plugin", "reload", "chaos-web", "chaos":
		return true
	default:
		return false
	}
}

var safeCommandArgRE = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

func quoteCommand(args []string) []string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		if safeCommandArgRE.MatchString(arg) {
			quoted[i] = arg
			continue
		}
		var tb textbuf.Buffer
		quoted[i] = tb.Byte('\'').Str(strings.ReplaceAll(arg, "'", "'\\''")).Byte('\'').String()
	}
	return quoted
}

func (r *Report) printFailureGroups(tests *Tests) {
	groups := groupFunctionalFailures(r.label, tests.failedRecords(), r.hostLoad)
	if len(groups) == 0 {
		return
	}
	r.writeln(r.colors.LineSeparator())
	if r.hostLoad != nil && r.hostLoad.Contended() {
		r.writeln(r.colors.Yellow("VERIFY FAILURE INDEX (CONTENDED RUN):"))
		r.writef("  %s\n", r.colors.Yellow(r.hostLoad.String()))
	} else {
		r.writeln(r.colors.Yellow("VERIFY FAILURE INDEX:"))
	}
	r.writeln(r.colors.LineSeparator())
	for _, group := range groups {
		payload, err := json.Marshal(group)
		if err == nil {
			r.writef("%s %s\n", nativeFailureGroupPrefix, payload)
		}
		r.writef("  group: %s\n", group.GroupID)
		r.writef("  kind: %s\n", group.Kind)
		r.writef("  related: %s\n", textbuf.Join(group.Related, ", "))
		r.writef("  rerun: %s\n", group.Rerun)
	}
	r.writeln("")
}
