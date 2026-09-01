package rfc

import (
	"testing"
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
		Reason: escapeDeclaration, Producer: selftestTablePath,
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
