// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the audit half of the port answers
// what rfc_requirements.py answers, over the WHOLE checkout, at the level of
// the internals rather than the page.
// PREVENTS: a re-seal that agrees with the script only because nothing in this
// tree is shifted. The gate prints one line over a clean corpus whether the
// freshness rule fires or not, so a rule that stopped firing is invisible from
// its output. Here the four states, the fingerprint keys, the file-level shas
// and the unit-level shas are compared element by element, for every tag in the
// tree and every verdict on record.
//
// This file is a MIGRATION artifact and dies with the script at step 14 of the
// spec, beside constants_python_test.go.

package rfc

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// pythonAuditFreshness is one requirement's recorded state, as the module
// answers it.
type pythonAuditFreshness struct {
	RID   string   `json:"rid"`
	State string   `json:"state"`
	Moved []string `json:"moved"`
}

// pythonKeys is one requirement's fingerprint keys, in tag order.
type pythonKeys struct {
	RID  string   `json:"rid"`
	Keys []string `json:"keys"`
}

// pythonSHA is one entry of a fingerprint map.
type pythonSHA struct {
	Key string `json:"key"`
	SHA string `json:"sha"`
}

// pythonSHAMap is one requirement's fingerprint map, or the refusal that
// stopped it being computed.
type pythonSHAMap struct {
	RID   string      `json:"rid"`
	Map   []pythonSHA `json:"map"`
	Error string      `json:"error"`
}

// pythonVerdicts is one RFC's recorded requirement ids.
type pythonVerdicts struct {
	Stem string   `json:"stem"`
	RIDs []string `json:"rids"`
}

// pythonAudit is everything the module is asked for beyond the page.
type pythonAudit struct {
	Stems    []string               `json:"stems"`
	Verdicts []pythonVerdicts       `json:"verdicts"`
	States   []pythonAuditFreshness `json:"states"`
	Keys     []pythonKeys           `json:"keys"`
	Tagged   []pythonSHAMap         `json:"tagged"`
	Units    []pythonSHAMap         `json:"units"`
}

// pythonAuditDump is the program the interpreter runs. It imports the module by
// path and prints the answers the gate never publishes.
//
// The keys and both fingerprint maps are asked for EVERY requirement id a tag
// carries, not only the 52 that have a verdict on record: minting a key runs
// the span walk over the enclosing file, so asking for all 3602 tags is what
// compares the two tagged-unit definitions across the whole tree.
const pythonAuditDump = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("rr", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
enrolled, reqs, parse_errs, tags, _ = m._collect_for_check()
audits = m.load_audits(enrolled)
states = m.audit_freshness(reqs, tags, enrolled, audits)
by_rid = {}
for t in tags:
    by_rid.setdefault(t.rid, []).append(t)

def sha_map(rid, computed):
    return {"rid": rid, "map": [{"key": k, "sha": v} for k, v in sorted(computed.items())], "error": ""}

keys, tagged, units = [], [], []
for rid in sorted(by_rid):
    k = m.tag_keys(by_rid[rid])
    keys.append({"rid": rid, "keys": k})
    tagged.append(sha_map(rid, m.tagged_unit_shas(by_rid[rid])))
    try:
        units.append(sha_map(rid, m.unit_shas(k, where="w")))
    except m.ParseError as exc:
        units.append({"rid": rid, "map": [], "error": str(exc)})
print(json.dumps({
  "stems": sorted(m.audit_stems()),
  "verdicts": [{"stem": s, "rids": sorted(v)} for s, v in sorted(audits.items()) if v],
  "states": [{"rid": rid, "state": s, "moved": mv} for rid, (s, mv) in sorted(states.items())],
  "keys": keys,
  "tagged": tagged,
  "units": units,
}))
`

// readPythonAudit runs the driver over this checkout's module.
func readPythonAudit(t *testing.T, root string) pythonAudit {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 10*pythonTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", pythonAuditDump, // #nosec G204 -- a tracked script path
		filepath.Join(root, "scripts", "dev", "rfc_requirements.py"))
	cmd.Dir = root
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading the Python audit internals: %v: %s", err, errOut.String())
	}
	var found pythonAudit
	if err := json.Unmarshal(out.Bytes(), &found); err != nil {
		t.Fatalf("the driver answered no JSON: %v: %s", err, out.String())
	}
	return found
}

// commandAudit answers the same from this package.
func commandAudit(t *testing.T, root string) pythonAudit {
	t.Helper()

	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	audits, err := LoadAudits(root, collected.Enrolled)
	if err != nil {
		t.Fatalf("LoadAudits: %v", err)
	}
	states := AuditFreshness(AuditFreshnessInput{
		Tree: root, Requirements: collected.Requirements, Tags: collected.Tags,
		Enrolled: collected.Enrolled, Audits: audits,
	})
	stems, err := AuditStems(root)
	if err != nil {
		t.Fatalf("AuditStems: %v", err)
	}

	byRID := map[string][]Tag{}
	for _, tag := range collected.Tags {
		byRID[tag.RID] = append(byRID[tag.RID], tag)
	}
	reader := newSourceReader(root)
	index := newScopeIndex()

	found := pythonAudit{Stems: sortedSet(stems)}
	for _, stem := range sortedKeysOf(audits) {
		if len(audits[stem].Verdicts) == 0 {
			continue
		}
		found.Verdicts = append(found.Verdicts,
			pythonVerdicts{Stem: stem, RIDs: sortedKeysOf(audits[stem].Verdicts)})
	}
	for _, rid := range sortedKeysOf(states) {
		moved := states[rid].Moved
		if moved == nil {
			moved = []string{}
		}
		found.States = append(found.States,
			pythonAuditFreshness{RID: rid, State: states[rid].State, Moved: moved})
	}
	for _, rid := range sortedKeysOf(byRID) {
		keys := tagKeys(byRID[rid], reader, index)
		found.Keys = append(found.Keys, pythonKeys{RID: rid, Keys: keys})
		found.Tagged = append(found.Tagged,
			asSHAMap(rid, taggedUnitSHAs(byRID[rid], reader, index), nil))
		computed, err := unitSHAs(keys, reader, index, "w")
		found.Units = append(found.Units, asSHAMap(rid, computed, err))
	}
	return found
}

// asSHAMap renders one fingerprint map the way the driver renders it.
func asSHAMap(rid string, computed map[string]string, err error) pythonSHAMap {
	if err != nil {
		return pythonSHAMap{RID: rid, Map: []pythonSHA{}, Error: err.Error()}
	}
	out := pythonSHAMap{RID: rid, Map: []pythonSHA{}}
	for _, key := range sortedKeysOf(computed) {
		out.Map = append(out.Map, pythonSHA{Key: key, SHA: computed[key]})
	}
	return out
}

func TestTheAuditHalfAgreesWithTheModuleOverTheCheckout(t *testing.T) {
	root := checkoutRoot(t)
	script := readPythonAudit(t, root)
	command := commandAudit(t, root)

	if !slices.Equal(script.Stems, command.Stems) {
		t.Errorf("audit stems differ:\nscript:  %v\ncommand: %v", script.Stems, command.Stems)
	}
	if len(script.Verdicts) != len(command.Verdicts) {
		t.Fatalf("the script loaded %d record(s) and the command %d",
			len(script.Verdicts), len(command.Verdicts))
	}
	for i, want := range script.Verdicts {
		got := command.Verdicts[i]
		if want.Stem != got.Stem || !slices.Equal(want.RIDs, got.RIDs) {
			t.Errorf("record %d differs:\nscript:  %v\ncommand: %v", i, want, got)
		}
	}

	// The corpus measured something. A comparison over an empty tree passes
	// every assertion above and proves nothing.
	if len(script.States) == 0 || len(script.Keys) < 100 {
		t.Fatalf("the driver read %d state(s) and %d tagged requirement(s); this checkout has more",
			len(script.States), len(script.Keys))
	}

	if len(script.States) != len(command.States) {
		t.Fatalf("the script judged %d verdict(s) and the command %d",
			len(script.States), len(command.States))
	}
	for i, want := range script.States {
		got := command.States[i]
		if want.RID != got.RID || want.State != got.State || !slices.Equal(want.Moved, got.Moved) {
			t.Errorf("state %d differs:\nscript:  %+v\ncommand: %+v", i, want, got)
		}
	}

	if len(script.Keys) != len(command.Keys) {
		t.Fatalf("the script minted keys for %d requirement(s) and the command for %d",
			len(script.Keys), len(command.Keys))
	}
	for i, want := range script.Keys {
		got := command.Keys[i]
		if want.RID != got.RID || !slices.Equal(want.Keys, got.Keys) {
			t.Errorf("the keys of %s differ:\nscript:  %v\ncommand: %v",
				want.RID, want.Keys, got.Keys)
		}
	}
	compareSHAMaps(t, "the file-level fingerprints", script.Tagged, command.Tagged)
	compareSHAMaps(t, "the unit-level fingerprints", script.Units, command.Units)
}

// compareSHAMaps fails unless both halves computed the same map, or refused for
// the same reason, for every requirement.
func compareSHAMaps(t *testing.T, what string, script, command []pythonSHAMap) {
	t.Helper()

	if len(script) != len(command) {
		t.Fatalf("%s: the script answered %d map(s) and the command %d",
			what, len(script), len(command))
	}
	for i, want := range script {
		got := command[i]
		if want.RID != got.RID || want.Error != got.Error {
			t.Errorf("%s of %s differ:\nscript:  %q\ncommand: %q",
				what, want.RID, want.Error, got.Error)
			continue
		}
		if len(want.Map) != len(got.Map) {
			t.Errorf("%s of %s: the script answered %d entr(ies) and the command %d",
				what, want.RID, len(want.Map), len(got.Map))
			continue
		}
		for j, entry := range want.Map {
			if entry != got.Map[j] {
				t.Errorf("%s of %s entry %d differs:\nscript:  %+v\ncommand: %+v",
					what, want.RID, j, entry, got.Map[j])
			}
		}
	}
}

func TestTheAuditClosedSetsHoldTheValuesTheModuleHolds(t *testing.T) {
	root := checkoutRoot(t)
	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	const dump = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("rr", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
print(json.dumps({
  "verdicts": sorted(m.AUDIT_VERDICTS),
  "file_keys": sorted(m._AUDIT_FILE_KEYS),
  "verdict_keys": sorted(m._VERDICT_KEYS),
  "fingerprint_maps": list(m._FINGERPRINT_MAPS),
  "states": [m.FRESH, m.SHIFTED, m.STALE_UNIT, m.STALE_REQUIREMENT],
  "not_applicable": m.VERDICT_NOT_APPLICABLE,
  "reseal_note": m._RESEAL_NOTE,
  "staging_prefix": m._STAGING_PREFIX,
  "staging_stale_seconds": m._STAGING_STALE_SECONDS,
  "scope_func": m.rfc_tagged_scope.SCOPE_FUNC,
  "scope_file": m.rfc_tagged_scope.SCOPE_FILE,
  "audit_dir": m.os.path.relpath(m.AUDIT_DIR, m.PROJECT_DIR).replace(m.os.sep, "/"),
}))
`
	cmd := exec.CommandContext(ctx, "python3", "-c", dump, // #nosec G204 -- a tracked script path
		filepath.Join(root, "scripts", "dev", "rfc_requirements.py"))
	cmd.Dir = root
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading the Python audit constants: %v: %s", err, errOut.String())
	}
	var python struct {
		Verdicts            []string `json:"verdicts"`
		FileKeys            []string `json:"file_keys"`
		VerdictKeys         []string `json:"verdict_keys"`
		FingerprintMaps     []string `json:"fingerprint_maps"`
		States              []string `json:"states"`
		NotApplicable       string   `json:"not_applicable"`
		ResealNote          string   `json:"reseal_note"`
		StagingPrefix       string   `json:"staging_prefix"`
		StagingStaleSeconds int      `json:"staging_stale_seconds"`
		ScopeFunc           string   `json:"scope_func"`
		ScopeFile           string   `json:"scope_file"`
		AuditDir            string   `json:"audit_dir"`
	}
	if err := json.Unmarshal(out.Bytes(), &python); err != nil {
		t.Fatalf("the driver answered no JSON: %v: %s", err, out.String())
	}

	for _, one := range []struct {
		name        string
		want, found []string
	}{
		{"AUDIT_VERDICTS", python.Verdicts, AuditVerdicts()},
		{"_AUDIT_FILE_KEYS", python.FileKeys, AuditFileKeys()},
		{"_VERDICT_KEYS", python.VerdictKeys, VerdictKeys()},
		{"_FINGERPRINT_MAPS", python.FingerprintMaps, FingerprintMaps()},
		{"the four states", python.States, FreshnessStates()},
	} {
		if !slices.Equal(one.want, one.found) {
			t.Errorf("%s: the module holds %v and this package holds %v",
				one.name, one.want, one.found)
		}
	}
	for _, one := range []struct {
		name, want, found string
	}{
		{"VERDICT_NOT_APPLICABLE", python.NotApplicable, verdictNotApplicable},
		{"_RESEAL_NOTE", python.ResealNote, resealNote},
		{"_STAGING_PREFIX", python.StagingPrefix, stagingPrefix},
		{"SCOPE_FUNC", python.ScopeFunc, scopeFunc},
		{"SCOPE_FILE", python.ScopeFile, scopeFile},
		{"AUDIT_DIR", python.AuditDir, auditRel},
	} {
		if one.want != one.found {
			t.Errorf("%s: the module holds %q and this package holds %q",
				one.name, one.want, one.found)
		}
	}
	if python.StagingStaleSeconds != stagingStaleSeconds {
		t.Errorf("_STAGING_STALE_SECONDS: the module holds %d and this package holds %d",
			python.StagingStaleSeconds, stagingStaleSeconds)
	}
}
