// Design: docs/architecture/linux-limitations.md

// Package linuxlimit reads the Linux limitations register for tooling and tests.
package linuxlimit

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Entry is a textual register record, not a determination that a limitation
// qualifies for an RFC annotation. Consumers MUST judge its fields for their
// own purpose. Its zero value represents no entry.
type Entry struct {
	ID       string
	Kind     string
	Effect   string
	Status   string
	Checked  string
	Kernel   string
	Module   string
	Version  string
	Blocks   string
	Evidence string
	ReCheck  string
}

var (
	limitID     = regexp.MustCompile(`^LL-[A-Z0-9-]+$`)
	limitField  = regexp.MustCompile(`^- \*\*([^*]+):\*\*(?: (.*))?$`)
	limitKernel = regexp.MustCompile(`^Linux (\d+\.\d+(?:\.\d+)?), (\S+)$`)
	limitModule = regexp.MustCompile(`^module (\S+) (v\S+), (\S+)$`)
)

// The field names an entry carries, as the register page spells them.
const (
	fieldKind    = "Kind"
	fieldEffect  = "Effect"
	fieldStatus  = "Status"
	fieldChecked = "Checked"
)

var limitFieldValues = map[string][]string{
	fieldKind:   {"kernel", "library"},
	fieldEffect: {"blocks", "constrains design"},
	fieldStatus: {"verified", "unverified"},
}

var limitRequiredFields = []string{fieldKind, fieldEffect, fieldStatus, fieldChecked}

// ValidID reports whether id uses the register's capitals, digits and dashes.
func ValidID(id string) bool {
	return limitID.MatchString(id)
}

// Parse reads entries between ## Entries and the next level-two heading.
// Indented prose continuations are preserved, separated by newlines. It returns
// no entries if any form is invalid; callers MUST report the returned problems.
// Missing prose is retained as empty text for the consumer's eligibility checks.
func Parse(page string) ([]Entry, []string) {
	var entries []Entry
	var problems []string
	ids := map[string]bool{}
	var fields map[string]string
	inEntries := false
	current := ""
	finish := func() {
		if fields == nil {
			return
		}
		e := &entries[len(entries)-1]
		for _, field := range limitRequiredFields {
			if _, ok := fields[field]; !ok {
				problems = append(problems, "entry "+e.ID+" has no "+field+
					" line; add `- **"+field+":** ...` in the form the page header gives")
			}
		}
		e.Kind, e.Effect, e.Status = fields[fieldKind], fields[fieldEffect], fields[fieldStatus]
		e.Checked = fields[fieldChecked]
		e.Blocks, e.Evidence, e.ReCheck = fields["Blocks"], fields["Evidence"], fields["Re-check"]
		for _, field := range limitRequiredFields {
			value, ok := fields[field]
			if !ok {
				continue
			}
			if field == fieldChecked {
				problems = append(problems, readLimitChecked(e, value)...)
				continue
			}
			if !slices.Contains(limitFieldValues[field], value) {
				problems = append(problems, "entry "+e.ID+" has "+field+" "+strconv.Quote(value)+
					", which is not one of "+strings.Join(limitFieldValues[field], ", "))
			}
		}
		fields = nil
	}
	for line := range strings.SplitSeq(page, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "## ") {
			finish()
			inEntries = line == "## Entries"
			current = ""
			continue
		}
		if !inEntries {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			finish()
			current = ""
			id := strings.TrimPrefix(line, "### ")
			if !ValidID(id) {
				problems = append(problems, "heading "+strconv.Quote(line)+
					" under ## Entries is not an entry id; write `### LL-<NAME>` in capitals, digits and dashes, "+
					"or move the section after the entries")
				continue
			}
			if ids[id] {
				problems = append(problems, "entry "+id+" has more than one heading; keep one entry per id")
			}
			ids[id] = true
			entries = append(entries, Entry{ID: id})
			fields = map[string]string{}
			continue
		}
		if fields == nil {
			continue
		}
		if line != strings.TrimLeft(line, " \t") {
			if strings.HasPrefix(strings.TrimSpace(line), "- **") {
				problems = append(problems, "entry "+entries[len(entries)-1].ID+
					" has indented field "+strconv.Quote(line)+"; start field lines with `- **Field:** value`")
				current = ""
				continue
			}
		}
		if strings.HasPrefix(line, "  ") && current != "" {
			fields[current] += "\n" + strings.TrimSpace(line)
			continue
		}
		current = ""
		m := limitField.FindStringSubmatch(line)
		if m == nil {
			if strings.HasPrefix(strings.TrimSpace(line), "- **") {
				problems = append(problems, "entry "+entries[len(entries)-1].ID+
					" has malformed field "+strconv.Quote(line)+"; write `- **Field:** value`")
			}
			continue
		}
		switch m[1] {
		case fieldKind, fieldEffect, fieldStatus, fieldChecked, "Limitation", "Blocks", "Evidence", "Alternatives", "Re-check", "Ze meanwhile":
		default:
			problems = append(problems, "entry "+entries[len(entries)-1].ID+" has unknown field "+
				strconv.Quote(m[1])+"; use a field named in the page header")
			continue
		}
		if _, ok := fields[m[1]]; ok {
			problems = append(problems, "entry "+entries[len(entries)-1].ID+" has more than one "+m[1]+
				" line; keep the one that is true and delete the others")
			continue
		}
		fields[m[1]] = m[2]
		current = m[1]
	}
	finish()
	if len(problems) != 0 {
		return nil, problems
	}
	return entries, nil
}

// readLimitChecked records a Checked value and returns its problems.
func readLimitChecked(e *Entry, value string) []string {
	date := ""
	if m := limitKernel.FindStringSubmatch(value); m != nil {
		e.Kernel, date = m[1], m[2]
	}
	if m := limitModule.FindStringSubmatch(value); m != nil {
		e.Module, e.Version, date = m[1], m[2], m[3]
	}
	if date == "" {
		return []string{"entry " + e.ID + " has a Checked line in no recognized form: " + strconv.Quote(value) +
			"; write `Linux X.Y[.Z], YYYY-MM-DD` or `module <path> vX.Y.Z, YYYY-MM-DD`"}
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return []string{"entry " + e.ID + " has a Checked date " + strconv.Quote(date) +
			" that is not a calendar date (YYYY-MM-DD): " + err.Error()}
	}
	return nil
}
