package rfc

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: the `discriminate` verb is published by the action table, takes its selectors
// keyword-before-value, and refuses an invocation that names neither selector or both.
// PREVENTS: a recorded-proof surface that exists as a function nobody can reach, and a
// grammar where a bare value is read as a stem.
func TestRFCActionsCarryDiscriminateVerb(t *testing.T) {
	var found bool
	for _, action := range Actions().Actions {
		if action.Verb != "discriminate" {
			continue
		}
		found = true
		if action.Writes {
			t.Error("discriminate is published as a writer; it reads the corpus and writes nothing")
		}
	}
	if !found {
		t.Fatal("the RFC action catalog does not publish discriminate")
	}

	refused := map[string][]string{
		"a stem before its keyword":           {"discriminate", "rfc9999"},
		"no selector at all":                  {"discriminate"},
		"both selectors at once":              {"discriminate", keyStem, "rfc9999", keyID, "RFC9999-2-1"},
		"a stem keyword with no stem":         {"discriminate", keyStem},
		"an id keyword with no id":            {"discriminate", keyID},
		"a keyword the verb does not declare": {"discriminate", "report", "somewhere.json"},
	}
	for what, args := range refused {
		if _, code := Answer(args); code != 2 {
			t.Errorf("%s answered %d, want refusal 2", what, code)
		}
	}

	// The whole path, from the typed words to the corpus reader. The checkout is the
	// tree, so this also proves the answer survives the real artifact directory,
	// whether or not it holds anything yet.
	answer, code := Answer([]string{"discriminate", keyStem, selftestStem})
	if code != 0 {
		t.Fatalf("a well-formed selector answered %d, want 0", code)
	}
	status, isStatus := answer.(discriminationStatus)
	if !isStatus {
		t.Fatalf("the answer is %T, want a discriminationStatus the pipe operators can render", answer)
	}
	if status.Selector != selftestStem {
		t.Errorf("the answer names selector %q, want the one that was typed", status.Selector)
	}
}

// VALIDATES: the selector splits the corpus -- a record answers its own stem, and a tag
// with no record for that requirement, polarity and carrier file is answered as unproven.
// PREVENTS: a status answer that reports every record for every selector, which would read
// as proof for a requirement nobody proved.
func TestDiscriminationStatusSeparatesProvenFromUnproven(t *testing.T) {
	files, proof := discriminationFixture(t)
	escape := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDDrop, Polarity: PolarityNegative,
		Unit: selftestCIPath, Route: RouteNoBreak,
		Reason: escapeDeclaration, Producer: selftestCITablePath,
	})
	// One further carrier whose tag no record covers, so the answer has to
	// separate what is proven from what is not.
	files["test/plugin/gadget.ci"] = "# RFC requirement: " + selftestRIDSend + " negative\n"
	files[selftestWorkflowRel] = selftestWorkflow
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof, escape)
	root := discriminationTree(t, files)

	status, err := discriminationStatusOf(root, selftestStem, "", func(rid string) bool {
		return hasRIDStem(rid, selftestStem)
	})
	if err != nil {
		t.Fatalf("read the fixture corpus: %v", err)
	}
	if len(status.Records) != 2 {
		t.Fatalf("the stem answered %d record(s), want the 2 the fixture carries", len(status.Records))
	}
	if len(status.Unproven) != 1 {
		t.Fatalf("the stem answered %d unproven tag(s), want 1: %+v", len(status.Unproven), status.Unproven)
	}
	if status.Unproven[0].Polarity != PolarityNegative {
		t.Errorf("the unproven tag is %+v, want the negative polarity no record covers", status.Unproven[0])
	}

	other, err := discriminationStatusOf(root, "RFC9999-2-2", "", func(rid string) bool {
		return rid == selftestRIDDrop
	})
	if err != nil {
		t.Fatalf("read the fixture corpus by id: %v", err)
	}
	if len(other.Records) != 1 || other.Records[0].RID != selftestRIDDrop {
		t.Fatalf("the id selector answered %+v, want only its own record", other.Records)
	}
}

// VALIDATES: `le rfc check` answers a payload that renders the gate's own violation page,
// which leroot selects by asserting a `Text() string` method on whatever the action answered.
// PREVENTS: an answer returned BY VALUE, whose pointer-receiver Text sits outside its method
// set, so the dispatcher falls back to the generic table and the violation list a person reads
// disappears with no error, no log line and no change of exit code.
//
// The interface is spelled structurally rather than imported, for the reason leaction spells
// its own copy that way: the action package must not depend on the dispatcher that runs it.
func TestCheckAnswerRendersItsOwnPage(t *testing.T) {
	answer, _ := Answer([]string{"check"})
	page, renders := answer.(interface{ Text() string })
	if !renders {
		t.Fatalf("le rfc check answered %T, which the dispatcher renders as a table rather than the gate page", answer)
	}
	if !strings.HasPrefix(page.Text(), "rfc-requirements") {
		t.Errorf("the rendered page does not open with the gate's own summary:\n%s", page.Text())
	}
}

// VALIDATES: the wiring of `./le rfc audit-stamp ... mode rejudge`. The typed
// words reach the re-judge path through Answer; no mode and `mode new` reach
// the default path, which refuses a judged id; `mode rejudge` refuses an
// unjudged one; any other mode is refused with exit 2 and writes nothing.
// METHOD: one fixture tree per invocation, rooted through ZE_REPO_ROOT, holding
// one recorded verdict, and a pending file re-judging it or judging the other
// row. The exit code and the audit file's bytes tell the paths apart.
// PREVENTS: a mode parameter the action table publishes and the answer never
// reads, and an unknown mode silently read as the default.
func TestRFCActionsAuditStampRejudgeMode(t *testing.T) {
	judged := map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWrong, "note": "the negative asserts nothing",
	}}
	unjudged := map[string]any{selftestRIDDrop: map[string]any{
		"verdict": VerdictNotApplicable, "note": "binds the document's authors",
		"no_code_path": "no code runs for an obligation on authors",
	}}
	cases := []struct {
		name    string
		mode    []string
		pending map[string]any
		code    int
	}{
		{"rejudge over a judged id", []string{"mode", "rejudge"}, judged, 0},
		{"no mode over a judged id", nil, judged, 2},
		{"new over a judged id", []string{"mode", "new"}, judged, 2},
		{"new over an unjudged id", []string{"mode", "new"}, unjudged, 0},
		{"rejudge over an unjudged id", []string{"mode", "rejudge"}, unjudged, 2},
		{"unknown mode", []string{"mode", "replace"}, judged, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := stampTree(t, nil)
			stampRecorded(t, root, map[string]any{
				selftestRIDSend: map[string]any{"verdict": VerdictWeak, "note": "a floor assertion"},
			})
			// env.Get answers from a cache built once from os.Environ(), so
			// the Setenv alone would leave Answer on the developer's checkout.
			t.Setenv("ZE_REPO_ROOT", root)
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			before := readStampAudit(t, root)
			from := stampPending(t, root, "rfc9999", tc.pending)

			args := append([]string{"audit-stamp", keyStem, "rfc9999", keyFrom, from}, tc.mode...)
			answer, code := Answer(args)
			if code != tc.code {
				t.Fatalf("%v answered %d, want %d", tc.mode, code, tc.code)
			}
			wrote := !bytes.Equal(readStampAudit(t, root), before)
			if wrote != (tc.code == 0) {
				t.Errorf("%v answered %d and wrote the audit file: %v", tc.mode, code, wrote)
			}
			if tc.code != 0 {
				return
			}
			if _, isReport := answer.(AuditStampReport); !isReport {
				t.Errorf("the answer is %T, want an AuditStampReport", answer)
			}
		})
	}
}

// VALIDATES: the rename verb is in the table, published as a writer, with the
// five keywords its three forms use.
func TestRFCActionsCarryRenameVerb(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != "rename" {
			continue
		}
		if !action.Writes {
			t.Error("rename is not published as a writer")
		}
		var keywords []string
		for _, parameter := range action.Parameters {
			keywords = append(keywords, parameter.Keyword)
		}
		for _, want := range []string{keyFrom, keyTo, keyPlan, keyPropose, keyUnder} {
			if !slices.Contains(keywords, want) {
				t.Errorf("rename does not declare the keyword %s: %v", want, keywords)
			}
		}
		return
	}
	t.Fatal("the RFC action catalog does not publish rename")
}

// VALIDATES: the wiring row -- `./le rfc rename from <old> to <new>` reaches the
// rename through Answer and moves the file in the checkout ZE_REPO_ROOT names.
func TestRFCActionsRenameThroughAnswer(t *testing.T) {
	root := renameFixture(t)
	setRenameRoot(t, root)
	answer, code := Answer([]string{"rename", keyFrom, selftestTestPath, keyTo, renameTarget})
	if code != 0 {
		t.Fatalf("the rename answered %d", code)
	}
	report, isReport := answer.(RenameReport)
	if !isReport || len(report.Moves) != 1 {
		t.Fatalf("the answer is %#v, want a report of one move", answer)
	}
	if existsRel(root, selftestTestPath) || !existsRel(root, renameTarget) {
		t.Error("the file did not move")
	}
}
