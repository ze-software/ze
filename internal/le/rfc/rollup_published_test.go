package rfc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestAPublishedRollupCountsOSVerifiedRows checks that the rollup a profile
// publishes (data/rfc-requirements.json, `./le rfc tree | json`, `./le rfc
// next | json`) counts an OS verified MUST row as proven, as every other
// published proven count does, and reads back to the live rollup.
//
// Method: a rollup over osVerifiedCorpus (one Ze-proven row, one OS verified
// row) is encoded held by value in a TreeTier, the way the optional tier and
// NextReport hold one. The JSON must publish `mandatory-proven` 2 and no
// `mandatory` Figure object; Ze proven alone publishes 1. Decoded back, the
// rollup must hold Ze proven 1, the sections whole and no intent counts.
func TestAPublishedRollupCountsOSVerifiedRows(t *testing.T) {
	figures := osVerifiedCorpus()
	rollup := rollupOf(map[string]StemFigures{"rfc1": figures}, []string{"rfc1"})
	if rollup.Mandatory.Proven != 1 || rollup.Sections.Must.OSVerified != 1 {
		t.Fatalf("fixture rollup Ze proven %d, OS verified %d, want 1 and 1",
			rollup.Mandatory.Proven, rollup.Sections.Must.OSVerified)
	}
	live := ScopeRollup{Rollup: rollup, MandatoryComplete: rollup.mandatoryComplete()}
	encoded, err := json.Marshal(TreeTier{Rollup: live, Rows: []*TreeRow{}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"mandatory-proven":2,`) {
		t.Errorf("the published rollup does not count the OS verified row as proven: %s", text)
	}
	for _, retired := range []string{`"mandatory":`, `"optional":{`, `"attainable":`, `"excluded":`} {
		if strings.Contains(text, retired) {
			t.Errorf("the published rollup still carries %s: %s", retired, text)
		}
	}
	var decoded TreeTier
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	read := decoded.Rollup
	if read.Mandatory.Proven != 1 || read.Mandatory.Total != rollup.Mandatory.Total {
		t.Errorf("read back mandatory %+v, want Ze proven 1 of %d", read.Mandatory, rollup.Mandatory.Total)
	}
	if !reflect.DeepEqual(read.Sections, rollup.Sections) {
		t.Errorf("read back sections %+v, want %+v", read.Sections, rollup.Sections)
	}
	if read.Sections.Must.Proven() != 2 {
		t.Errorf("read back sections prove %d MUST rows, want 2", read.Sections.Must.Proven())
	}
	if read.intent != nil {
		t.Errorf("read back intent %+v, want nil: no intent count is published", read.intent)
	}
	if read.RFCs != 1 || read.NotWalked != 0 || read.MandatoryComplete != live.MandatoryComplete {
		t.Errorf("read back %d RFCs, %d not walked, complete %v; want 1, 0, %v",
			read.RFCs, read.NotWalked, read.MandatoryComplete, live.MandatoryComplete)
	}
}

// TestAPublishedRollupWhoseProvenCountLacksItsOSRowsIsRefused checks that a
// rollup read back whose sections hold more OS verified rows than its
// published proven count is refused, not decoded into a negative Ze-proven
// count.
//
// Method: a hand-written rollup with `mandatory-proven` 1 and sections naming
// two OS verified rows, decoded as a ScopeRollup and as a bare Rollup. No
// MarshalJSON writes it, because the published count holds the OS verified
// rows, so each decode must answer an error naming the OS verified rows and
// the command that regenerates the file.
func TestAPublishedRollupWhoseProvenCountLacksItsOSRowsIsRefused(t *testing.T) {
	body := `{"rfcs":1,"not-walked":0,"mandatory-proven":1,"mandatory-total":4,"optional-proven":0,` +
		`"optional-total":0,"sections":{"must":{"total":4,"kept":4,"ze-proven":1,"os-verified":2,"not-built":1}},` +
		`"mandatory-complete":false}`
	var scope ScopeRollup
	var bare Rollup
	for label, target := range map[string]any{"scope rollup": &scope, "rollup": &bare} {
		err := json.Unmarshal([]byte(body), target)
		if err == nil {
			t.Errorf("%s: a rollup whose proven count is smaller than its OS verified rows decoded to %+v", label, target)
			continue
		}
		if !strings.Contains(err.Error(), "OS verified") || !strings.Contains(err.Error(), "./le rfc tree") {
			t.Errorf("%s: refusal %q does not name the OS verified rows and the fix", label, err)
		}
	}
}

// TestAPublishedScopeRollupKeepsItsOwnFields checks that ScopeRollup still
// publishes and reads back its own `mandatory-complete`: a JSON method it
// promoted from Rollup would encode the rollup alone and drop it.
//
// Method: a complete ScopeRollup held by pointer, as TreeReport holds the
// required tier's, is encoded and decoded; the key must read true both ways,
// beside the rollup's own `rfcs` and `not-walked`.
func TestAPublishedScopeRollupKeepsItsOwnFields(t *testing.T) {
	live := &ScopeRollup{RFCs: 3, NotWalked: 1, MandatoryComplete: true}
	encoded, err := json.Marshal(&TreeReport{Profile: "edge", Rollup: live})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"mandatory-complete":true`, `"rfcs":3`, `"not-walked":1`, `"mandatory-total":0`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("the published scope rollup lacks %s: %s", key, encoded)
		}
	}
	var decoded TreeReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Rollup == nil {
		t.Fatal("the scope rollup did not read back")
	}
	if !decoded.Rollup.MandatoryComplete || decoded.Rollup.RFCs != 3 || decoded.Rollup.NotWalked != 1 {
		t.Errorf("read back %+v, want 3 RFCs, 1 not walked, complete", *decoded.Rollup)
	}
}
