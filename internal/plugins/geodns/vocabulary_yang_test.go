// Design: config.go -- validClientIPSources and validSerialModes are the one Go declaration of each set
//
// Goal: prove the client IP sources and the SOA serial modes an operator can
// write are the words this plugin accepts and acts on, so a word cannot exist
// on one side alone. Method: read each enumeration out of the loaded model with
// configyang.EnumValues, which fails on a leaf that declares no enumeration,
// hold it to the table in both directions, and drive the parser with each
// word through the JSON the hub sends.

package geodns

import (
	"slices"
	"testing"
	"time"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

const (
	clientIPSourceLeaf = "service/geodns/client-ip-source"
	serialModeLeaf     = "service/geodns/soa/serial-mode"
)

// TestClientIPSourcesMatchTheModel holds validClientIPSources to the model.
func TestClientIPSourcesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(clientIPSourceLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", clientIPSourceLeaf, err)
	}

	accepted := sortedWords(validClientIPSources)
	if !slices.Equal(model, accepted) {
		t.Errorf("the client IP sources disagree: the model at %s holds %v and config.go accepts %v. "+
			"A word only the model carries is refused at parse time, and a word only Go carries is one no operator can ask for",
			clientIPSourceLeaf, model, accepted)
	}

	for _, source := range model {
		cfg, err := parseConfig(`{"service":{"geodns":{"zone":["g.example."],"client-ip-source":"` + source + `"}}}`)
		if err != nil {
			t.Errorf("%q is offered at %s and parseConfig refuses it: %v", source, clientIPSourceLeaf, err)
			continue
		}
		if cfg.ClientIPSource != source {
			t.Errorf("%q parsed as %q", source, cfg.ClientIPSource)
		}
	}
}

// TestSerialModesMatchTheModel holds validSerialModes to the model, and proves
// each word selects a serial of its own rather than falling to the default.
func TestSerialModesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(serialModeLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", serialModeLeaf, err)
	}

	accepted := sortedWords(validSerialModes)
	if !slices.Equal(model, accepted) {
		t.Errorf("the serial modes disagree: the model at %s holds %v and config.go accepts %v. "+
			"A word only the model carries is refused at parse time, and a word only Go carries is one no operator can ask for",
			serialModeLeaf, model, accepted)
	}

	// 2026-09-14 12:00 UTC: the epoch serial, the date serial and a fixed
	// serial of 7 are three different numbers, so a mode that fell to the
	// default arm would answer the epoch and be caught.
	now := time.Date(2026, time.September, 14, 12, 0, 0, 0, time.UTC)
	seen := map[uint32]string{}
	for _, mode := range model {
		cfg, err := parseConfig(`{"service":{"geodns":{"zone":["g.example."],"soa":{"serial-mode":"` + mode + `","serial":"7"}}}}`)
		if err != nil {
			t.Errorf("%q is offered at %s and parseConfig refuses it: %v", mode, serialModeLeaf, err)
			continue
		}
		serial := computeSerial(cfg.SOA, 0, now)
		if other, dup := seen[serial]; dup {
			t.Errorf("modes %q and %q both answered serial %d, so one of them reaches no arm of its own", mode, other, serial)
		}
		seen[serial] = mode
	}
}
