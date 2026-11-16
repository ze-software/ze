// Design: docs/contributing/rfc-implementation-guide.md -- complete approval-history validation.
package rfc

import (
	"bufio"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestApprovalHistoryBatchMatchesIndependentQueries proves real ordinary and
// exact-target queries retain independent newest carriers. Method: publish
// distinct messages in a real repository, then compare the batch with each
// existing single-query entry point, including an absent and a bound-only row.
func TestApprovalHistoryBatchMatchesIndependentQueries(t *testing.T) {
	root, target := fixtureCommit(t, map[string]string{
		"pkg/identity_test.go": "package pkg\nfunc TestIdentity(t *testing.T) { check(1) }\n",
	})
	ordinary := "Weakening-approved: pkg.TestOrdinary: judge: ordinary"
	bound := ApprovalTrailerLine("pkg.TestIdentity", "judge: bound")
	parent := target
	for _, message := range []string{
		"ordinary\n\n" + ordinary + "\n",
		"bound\n\n" + bound + "\n" + ApprovalForTrailerLine(target, "pkg.TestIdentity") + "\n",
		"newest ordinary\n\n" + ordinary + "\n",
	} {
		commit := fixtureGit(t, root, message, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test",
			"commit-tree", "HEAD^{tree}", "-p", parent)
		fixtureGit(t, root, "", "update-ref", "HEAD", commit, parent)
		parent = commit
	}
	requests := []ApprovalHistoryRequest{
		{Trailer: bound, Target: target, Unit: "pkg.TestIdentity"},
		{Trailer: ordinary},
		{Trailer: "Weakening-approved: pkg.TestAbsent: judge: absent"},
		{Trailer: bound},
	}
	want := make([]string, len(requests))
	for index, request := range requests {
		var problem string
		if request.Target == "" {
			want[index], _, problem = TrailerLanded(root, request.Trailer)
		} else {
			want[index], _, problem = BoundApprovalLanded(root, request.Trailer, request.Target, request.Unit)
		}
		if problem != "" {
			t.Fatal(problem)
		}
	}
	if want[0] == "" || want[1] == "" || want[0] == want[1] || want[2] != "" || want[3] != "" {
		t.Fatalf("independent controls = %q; want distinct bound/ordinary carriers and two absences", want)
	}
	got, problem := ApprovalsLanded(root, requests)
	if problem != "" || !slices.Equal(got, want) {
		t.Fatalf("batch = %q, %q; want independent answers %q", got, problem, want)
	}
}

// TestApprovalHistoryBatchRefusesMalformedTail proves a match for every query
// grants no authority before the final frame and EOF validate. Method: feed
// exact length-framed commit payloads, then corrupt only data after both
// matches. The successful control also proves each query keeps its own newest
// ID while the compatibility reader keeps the original first-query ordering.
func TestApprovalHistoryBatchRefusesMalformedTail(t *testing.T) {
	firstID, secondID, lastID := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	first := "Weakening-approved: pkg.TestA: judge: A"
	second := "Weakening-approved: pkg.TestB: judge: B"
	queries := []approvalHistoryQuery{newApprovalHistoryQuery(second, ""), newApprovalHistoryQuery(first, "")}
	frame := func(id, message string) string {
		payload := "tree " + strings.Repeat("d", 40) + "\n\n" + message + "\n"
		return fmt.Sprintf("%s commit %d\n%s\n", id, len(payload), payload)
	}
	prefix := frame(firstID, first) + frame(secondID, second+"\n"+first)
	history := firstID + "\n" + secondID + "\n" + lastID + "\n"
	valid := prefix + frame(lastID, "no verdict")
	got, selected, err := readApprovalHistoryAll(bufio.NewReader(strings.NewReader(valid)), history, queries)
	if err != nil || !slices.Equal(got, []string{secondID, firstID}) || selected != 1 {
		t.Fatalf("complete stream = %q, %d, %v; want each newest ID and first matched query 1", got, selected, err)
	}
	carrier, query, err := readApprovalHistory(bufio.NewReader(strings.NewReader(valid)), history, queries)
	if err != nil || carrier != firstID || query != 1 {
		t.Fatalf("compatibility reader = %q, %d, %v; want original first match", carrier, query, err)
	}
	for name, stream := range map[string]string{
		"missing later header": prefix,
		"wrong later kind":     prefix + lastID + " blob 0\n\n",
		"truncated later body": prefix + lastID + " commit 100\nshort\n",
		"bad later terminator": strings.TrimSuffix(valid, "\n") + "!",
		"trailing payload":     valid + "forged record\n",
	} {
		t.Run(name, func(t *testing.T) {
			matches, first, err := readApprovalHistoryAll(bufio.NewReader(strings.NewReader(stream)), history, queries)
			if err == nil || matches != nil || first != -1 {
				t.Fatalf("malformed tail = %q, %d, %v; want no authorized matches", matches, first, err)
			}
		})
	}
}
