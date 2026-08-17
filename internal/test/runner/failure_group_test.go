package runner

import (
	"strings"
	"testing"
)

func TestFunctionalFailureGroupsUseSuiteTypeAndSubsystemPrefix(t *testing.T) {
	records := []*Record{
		{Name: "bfd-session-timeout", Nick: "1", State: StateTimeout},
		{Name: "bfd-peer-timeout", Nick: "2", State: StateTimeout},
		{Name: "fib-observer-mismatch", Nick: "3", State: StateFail, FailureType: FailTypeMismatch},
	}
	groups := groupFunctionalFailures("plugin", records, nil)
	if len(groups) != 2 {
		t.Fatalf("expected bfd timeout and fib mismatch groups, got %+v", groups)
	}
	if groups[0].GroupID != "plugin:timeout:bfd" || strings.Join(groups[0].Related, ",") != "1,2" {
		t.Fatalf("unexpected first group: %+v", groups[0])
	}
	if groups[0].Rerun != "ze-test bgp plugin 1 2" {
		t.Fatalf("unexpected BGP rerun: %s", groups[0].Rerun)
	}
	if groups[1].GroupID != "plugin:mismatch:fib" || groups[1].Rerun != "ze-test bgp plugin 3" {
		t.Fatalf("unexpected second group: %+v", groups[1])
	}
}

func TestTopLevelCIFailureGroupsDoNotMergeByFirstToken(t *testing.T) {
	records := []*Record{
		{Name: "cli-schema-protocol", Nick: "1", State: StateFail, FailureType: FailTypeMismatch},
		{Name: "cli-bgp-encode-show", Nick: "2", State: StateFail, FailureType: FailTypeMismatch},
	}
	groups := groupFunctionalFailures("ui", records, nil)
	if len(groups) != 2 {
		t.Fatalf("expected separate groups for unrelated cli failures, got %+v", groups)
	}
	if groups[0].GroupID != "ui:mismatch:cli-schema-protocol" || groups[0].Rerun != "ze-test ui 1" {
		t.Fatalf("unexpected first ui group: %+v", groups[0])
	}
	if groups[1].GroupID != "ui:mismatch:cli-bgp-encode-show" || groups[1].Rerun != "ze-test ui 2" {
		t.Fatalf("unexpected second ui group: %+v", groups[1])
	}
}

func TestEditorFailureGroupsUsePositionalReruns(t *testing.T) {
	records := []*Record{
		{Name: "test/editor/commands/show-full.et", Nick: "7", State: StateFail, FailureType: stateUnknown},
		{Name: "test/editor/navigation/up-one-level.et", Nick: "8", State: StateFail, FailureType: stateUnknown},
	}
	groups := groupFunctionalFailures("editor", records, nil)
	if len(groups) != 2 {
		t.Fatalf("expected exact editor test groups, got %+v", groups)
	}
	if groups[0].Related[0] != "test/editor/commands/show-full.et" || groups[0].Rerun != "ze-test editor test/editor/commands/show-full.et" {
		t.Fatalf("unexpected first editor group: %+v", groups[0])
	}
	if groups[1].Related[0] != "test/editor/navigation/up-one-level.et" || groups[1].Rerun != "ze-test editor test/editor/navigation/up-one-level.et" {
		t.Fatalf("unexpected second editor group: %+v", groups[1])
	}
}

func TestParseFailureGroupsUseExactTestNames(t *testing.T) {
	records := []*Record{
		{Name: "invalid/peer-as.conf", Nick: "P", State: StateFail, FailureType: stateUnknown},
		{Name: "invalid/router-id.conf", Nick: "Q", State: StateFail, FailureType: stateUnknown},
	}
	groups := groupFunctionalFailures("parse", records, nil)
	if len(groups) != 2 {
		t.Fatalf("expected exact parse groups, got %+v", groups)
	}
	if groups[0].GroupID != "parse:unknown:invalid-peer-as-conf" || groups[0].Rerun != "ze-test bgp parse P" {
		t.Fatalf("unexpected first parse group: %+v", groups[0])
	}
	if groups[1].GroupID != "parse:unknown:invalid-router-id-conf" || groups[1].Rerun != "ze-test bgp parse Q" {
		t.Fatalf("unexpected second parse group: %+v", groups[1])
	}
}

func TestContendedLoadAttachedToFailureGroups(t *testing.T) {
	records := []*Record{
		{Name: "add-remove", Nick: "1", State: StateFail, FailureType: stateUnknown},
	}
	contended := &HostLoad{LoadAvg1: 20.0, CPUs: 8, ZeProcs: 3, GoTestProcs: 2}
	groups := groupFunctionalFailures("plugin", records, contended)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].HostLoad == nil {
		t.Fatal("expected HostLoad to be set on contended run")
	}
	if groups[0].HostLoad.LoadAvg1 != 20.0 {
		t.Errorf("HostLoad.LoadAvg1 = %v, want 20.0", groups[0].HostLoad.LoadAvg1)
	}
}

func TestQuietLoadNotAttachedToFailureGroups(t *testing.T) {
	records := []*Record{
		{Name: "add-remove", Nick: "1", State: StateFail, FailureType: stateUnknown},
	}
	quiet := &HostLoad{LoadAvg1: 1.0, CPUs: 8, ZeProcs: 1, GoTestProcs: 0}
	groups := groupFunctionalFailures("plugin", records, quiet)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].HostLoad != nil {
		t.Fatal("expected HostLoad to be nil on quiet run")
	}
}

func TestFormatRerunCommandUsesSuiteSpecificCommands(t *testing.T) {
	if got := FormatRerunCommand("plugin", []string{"A"}); got != "ze-test bgp plugin A" {
		t.Fatalf("BGP rerun mismatch: %s", got)
	}
	if got := FormatRerunCommand("ui", []string{"A"}); got != "ze-test ui A" {
		t.Fatalf("top-level rerun mismatch: %s", got)
	}
	if got := FormatRerunCommand("editor", []string{"7"}); got != "ze-test editor 7" {
		t.Fatalf("editor rerun mismatch: %s", got)
	}
}
