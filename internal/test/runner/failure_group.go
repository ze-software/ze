// Design: docs/functional-tests.md -- functional failure routing groups
// Related: display.go -- suite summaries and rerun hints
package runner

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	nativeFailureGroupPrefix = "VERIFY FAILURE GROUP:"
	defaultFailureSuite      = "encode"
	editorSuite              = "editor"
)

// FailureGroup is the native suite-local failure routing unit emitted by ze-test.
type FailureGroup struct {
	Stage     string   `json:"stage"`
	GroupID   string   `json:"group_id"`
	Kind      string   `json:"kind"`
	Related   []string `json:"related"`
	Summary   string   `json:"summary"`
	Rerun     string   `json:"rerun"`
	DetailLog string   `json:"detail_log"`
	Parallel  string   `json:"parallel"`
}

// GroupFunctionalFailures groups failed records by suite, failure kind, and a
// suite-specific routing key.
func GroupFunctionalFailures(suite string, records []*Record) []FailureGroup {
	if suite == "" {
		suite = defaultFailureSuite
	}
	byKey := map[string]*FailureGroup{}
	order := []string{}
	subjects := map[string]string{}
	for _, rec := range records {
		kind := recordFailureKind(rec)
		target := failureGroupTarget(suite, rec)
		key := suite + ":" + kind + ":" + target.Key
		group, ok := byKey[key]
		if !ok {
			group = &FailureGroup{
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
	groups := make([]FailureGroup, 0, len(order))
	for _, key := range order {
		group := byKey[key]
		group.Rerun = FormatRerunCommand(group.Stage, group.Related)
		group.Summary = fmt.Sprintf("%s %s %s: %d test(s)", group.Stage, group.Kind, subjects[key], len(group.Related))
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
		if len(args) > 0 {
			command = append(command, "-p", args[0])
		}
	case isBGPSuite(suite):
		command = append(command, "bgp", suite)
		command = append(command, args...)
	default:
		command = append(command, suite)
		command = append(command, args...)
	}
	return strings.Join(quoteCommand(command), " ")
}

func FormatRecordRerunCommand(suite string, rec *Record) string {
	if rec == nil {
		return FormatRerunCommand(suite, nil)
	}
	if suite == editorSuite {
		name := strings.TrimSpace(rec.Name)
		if name == "" {
			name = rec.Nick
		}
		return FormatRerunCommand(suite, []string{name})
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
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return quoted
}

func (r *Report) PrintFailureGroups(tests *Tests) {
	groups := GroupFunctionalFailures(r.label, tests.FailedRecords())
	if len(groups) == 0 {
		return
	}
	r.writeln(r.colors.LineSeparator())
	r.writeln(r.colors.Yellow("VERIFY FAILURE INDEX:"))
	r.writeln(r.colors.LineSeparator())
	for _, group := range groups {
		payload, err := json.Marshal(group)
		if err == nil {
			r.writef("%s %s\n", nativeFailureGroupPrefix, payload)
		}
		r.writef("  group: %s\n", group.GroupID)
		r.writef("  kind: %s\n", group.Kind)
		r.writef("  related: %s\n", strings.Join(group.Related, ", "))
		r.writef("  rerun: %s\n", group.Rerun)
	}
	r.writeln("")
}
