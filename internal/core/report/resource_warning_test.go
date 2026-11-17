// Design: docs/architecture/doctor-and-health-checks.md -- retained resource problems
package report

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Goal: source shutdown clears ordinary warnings while retaining the exact
// original resource issue, including its cause, fix and first-raised time.
func TestResourceWarningSurvivesOrdinarySourceCleanup(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "native-original-episode")
	if err != nil {
		t.Fatal(err)
	}
	episode, err := owner.NewEpisode("doctor-cos-cleanup-unconfirmed", "original native restore failed", "request class-of-service cleanup")
	if err != nil {
		t.Fatal(err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	first := Warnings()[0]
	RaiseWarningFix("cos", "ordinary", "other-resource", "ordinary issue", "show warnings")
	ClearSource("cos")
	entries := Warnings()
	if len(entries) != 1 {
		t.Fatalf("source cleanup kept %d warnings, want resource only", len(entries))
	}
	if !reflect.DeepEqual(entries[0], first) {
		t.Fatal("source cleanup changed original cause/fix/identity/time")
	}
	if err := episode.Clear(); err != nil {
		t.Fatal(err)
	}
	if len(Warnings()) != 0 {
		t.Fatal("same resource episode failed to clear")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: ordinary callers cannot delete or replace a concretely owned issue,
// and their error-returning transport path receives a refusal, not false ACK.
func TestResourceWarningRefusesForeignClearAndOverwrite(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	episode, err := owner.NewEpisode("cleanup", "original cause", "request class-of-service cleanup")
	if err != nil {
		t.Fatal(err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := TryClearWarning("cos", "cleanup", "original"); !errors.Is(err, ErrResourceWarningOwned) {
		t.Fatalf("ordinary clear=%v", err)
	}
	if err := TryRaiseWarningFix("cos", "cleanup", "original", "replacement cause", "replacement fix"); !errors.Is(err, ErrResourceWarningOwned) {
		t.Fatalf("ordinary overwrite=%v", err)
	}
	ClearWarning("cos", "cleanup", "original")
	RaiseWarning("cos", "cleanup", "original", "replacement cause")
	entries := Warnings()
	if len(entries) != 1 {
		t.Fatal("ordinary operation dropped original issue")
	}
	if entries[0].Message != "original cause" {
		t.Fatal("ordinary operation changed original cause")
	}
	if err := owner.Close(); err == nil {
		t.Fatal("active resource scope released its reservation")
	}
	if err := episode.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: an old clear capability cannot clear a newer failure on the same
// retained resource; repeated old clear and publication cannot resurrect it.
func TestResourceWarningOldEpisodeCannotClearNewFailure(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	first, err := owner.NewEpisode("cleanup", "first failure", "first repair")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := first.Clear(); err != nil {
		t.Fatal(err)
	}
	second, err := owner.NewEpisode("cleanup", "second failure", "second repair")
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := first.Clear(); err != nil {
		t.Fatal("same old clear should acknowledge idempotently:", err)
	}
	if err := first.Publish(); err == nil {
		t.Fatal("old episode publication resurrected a cleared failure")
	}
	entries := Warnings()
	if len(entries) != 1 {
		t.Fatal("old episode cleared new warning")
	}
	if entries[0].Message != "second failure" {
		t.Fatal("old episode changed new cause")
	}
	if err := second.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: reservation happens before native work, and unrelated bus pressure
// cannot evict the original resource's retained cause.
func TestResourceWarningReservesBoundedCapacityBeforePublication(t *testing.T) {
	resetWithCaps(1, 1)
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewResourceWarning("cos", "other"); err == nil {
		t.Fatal("resource reservation exceeded bus cap")
	}
	RaiseWarning("ordinary", "test", "other", "unrelated pressure")
	if len(Warnings()) != 0 {
		t.Fatal("ordinary warning consumed reserved resource slot")
	}
	episode, err := owner.NewEpisode("cleanup", "retained original cause", "repair original")
	if err != nil {
		t.Fatal(err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	RaiseWarning("ordinary", "test", "later", "later pressure")
	entries := Warnings()
	if len(entries) != 1 {
		t.Fatal("bus pressure changed reserved visibility")
	}
	if entries[0].Message != "retained original cause" {
		t.Fatal("bus pressure evicted resource cause")
	}
	if err := episode.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	successor, err := NewResourceWarning("cos", "other")
	if err != nil {
		t.Fatal("completed original reservation was not freed:", err)
	}
	if err := successor.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: a healthy reserved owner never needs a synthetic raise or clear.
func TestResourceWarningHealthyOwnerReleasesUnusedReservation(t *testing.T) {
	resetWithCaps(1, 1)
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "healthy")
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	RaiseWarning("cos", "ordinary", "other", "ordinary condition")
	if len(Warnings()) != 1 {
		t.Fatal("healthy close leaked a warning reservation")
	}
	ClearSource("cos")
	if len(Warnings()) != 0 {
		t.Fatal("ordinary source cleanup semantics changed")
	}
}

// Goal: another source or subject retains ordinary report behavior; the scope
// protects only its exact resource namespace and never the whole plugin.
func TestResourceWarningProtectsOnlyItsExactNamespace(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	if err := TryRaiseWarningFix("other-source", "cleanup", "original", "independent source", "repair source"); err != nil {
		t.Fatal(err)
	}
	if err := TryRaiseWarningFix("cos", "cleanup", "other-subject", "independent subject", "repair subject"); err != nil {
		t.Fatal(err)
	}
	if err := TryClearWarning("other-source", "cleanup", "original"); err != nil {
		t.Fatal(err)
	}
	if err := TryClearWarning("cos", "cleanup", "other-subject"); err != nil {
		t.Fatal(err)
	}
	if len(Warnings()) != 0 {
		t.Fatal("unrelated ordinary warnings did not clear")
	}
	if _, err := NewResourceWarning("cos", "original"); !errors.Is(err, ErrResourceWarningOwned) {
		t.Fatalf("same subject acquired competing custody: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: a reserved failure is retained before publication and an invalid or
// closed capability never creates an issue or releases another owner's slot.
func TestResourceWarningPendingEpisodeRetainsCustody(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	episode, err := owner.NewEpisode("cleanup", "unpublished original cause", "repair original")
	if err != nil {
		t.Fatal(err)
	}
	ClearSource("cos")
	if len(Warnings()) != 0 {
		t.Fatal("pending episode was published by source cleanup")
	}
	if err := owner.Close(); err == nil {
		t.Fatal("pending episode lost reserved custody")
	}
	if _, err := owner.NewEpisode("cleanup", "replacement cause", "replacement fix"); err == nil {
		t.Fatal("new failure replaced pending original episode")
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	if len(Warnings()) != 1 {
		t.Fatal("publication retry duplicated original issue")
	}
	if err := episode.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.NewEpisode("cleanup", "closed owner cause", "repair"); err == nil {
		t.Fatal("closed owner produced another episode")
	}
	if err := new(ResourceWarningEpisode).Publish(); err == nil {
		t.Fatal("zero episode published")
	}
	if err := new(ResourceWarning).Close(); err == nil {
		t.Fatal("zero owner released custody")
	}
}

// Goal: later genuine failures retain both bounded original and latest causes,
// while report-only retries preserve the first timestamp and original fix.
func TestResourceWarningRetainsBoundedFirstAndLatestCauses(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	first := FitResourceWarningCause(strings.Repeat("a", MessageMax))
	latest := FitResourceWarningCause(strings.Repeat("b", MessageMax))
	episode, err := owner.NewEpisode("cleanup", first, "repair original")
	if err != nil {
		t.Fatal(err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	raised := Warnings()[0].Raised
	if err := episode.Update(latest); err != nil {
		t.Fatal(err)
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	entry := Warnings()[0]
	if !strings.Contains(entry.Message, first) {
		t.Fatal("latest update lost original cause")
	}
	if !strings.Contains(entry.Message, latest) {
		t.Fatal("latest actual cause was lost")
	}
	if len(entry.Message) > MessageMax {
		t.Fatal("first/latest report exceeded bounded message")
	}
	if entry.Raised != raised {
		t.Fatal("report retry replaced first-raised time")
	}
	if entry.Fix != "repair original" {
		t.Fatal("latest update replaced original fix")
	}
	if err := episode.Update(strings.Repeat("x", MessageMax)); err == nil {
		t.Fatal("unbounded latest cause accepted")
	}
	if err := episode.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := episode.Update("old late cause"); err == nil {
		t.Fatal("cleared episode accepted an old update")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: copying an exported capability never creates a second registered owner
// or episode, including a stale copy retained after the original Close.
func TestResourceWarningCopiesCannotProduceOrMutateEpisodes(t *testing.T) {
	reset()
	t.Cleanup(reset)
	owner, err := NewResourceWarning("cos", "original")
	if err != nil {
		t.Fatal(err)
	}
	copied := *owner
	if _, err := copied.NewEpisode("cleanup", "copied cause", "repair copied"); err == nil {
		t.Fatal("value copy produced a competing episode")
	}
	episode, err := owner.NewEpisode("cleanup", "actual original cause", "repair original")
	if err != nil {
		t.Fatal(err)
	}
	copiedEpisode := *episode
	if err := copiedEpisode.Publish(); err == nil {
		t.Fatal("copied episode published")
	}
	if err := copiedEpisode.Update("copied latest"); err == nil {
		t.Fatal("copied episode changed actual cause")
	}
	if err := copiedEpisode.Clear(); err == nil {
		t.Fatal("copied episode cleared actual failure")
	}
	if err := episode.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := episode.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := copied.NewEpisode("cleanup", "stale cause", "repair stale"); err == nil {
		t.Fatal("stale copied owner produced episode after original Close")
	}
	if err := copied.Close(); err == nil {
		t.Fatal("copied owner released original reservation")
	}
	if len(Warnings()) != 0 {
		t.Fatal("copied owner left a foreign issue")
	}
}
