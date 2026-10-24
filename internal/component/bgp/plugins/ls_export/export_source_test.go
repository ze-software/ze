package ls_export

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestDomainNamingAnUnpublishedSourceIsRefused proves that a `domain` entry
// whose source no plugin in this build can publish is refused at every door,
// naming the entry, the value, and the sources the build does publish, while a
// published source and an IGP source the build left out (refused by its config
// need instead, with the build tag) pass this check. Method: parse real config
// text and run it through the two calls the doors make:
// config.ValidateCustomSections (`ze config validate`, the editor commit) and
// config.LoadConfig (daemon boot and SIGHUP reload). This test binary links
// bgp-epe but neither IS-IS nor OSPF.
//
// VALIDATES: verifyDomainSources, registered as an original-tree validator.
// PREVENTS: `source rsvp-te` or a typo accepted and exporting nothing.
func TestDomainNamingAnUnpublishedSourceIsRefused(t *testing.T) {
	published := linkstateevents.Sources()
	if len(published) == 0 {
		t.Fatal("no BGP-LS source registered in this test binary; bgp-epe should be")
	}
	known := "\n  sources this build publishes: " + textbuf.Join(published, ", ")

	for _, source := range []string{"rsvp-te", "isiss"} {
		t.Run("refused/"+source, func(t *testing.T) {
			setting := `config: "bgp-ls-export domain core source ` + source +
				`" names a BGP-LS source that no plugin in this build publishes`
			fix := "\n  fix: set the source of bgp-ls-export domain core to one of them"
			for door, err := range doorErrors(t, domainText("core", source)) {
				if err == nil {
					t.Fatalf("%s accepted source %q, which no plugin publishes", door, source)
				}
				for _, want := range []string{setting, known, fix} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("%s refusal lacks %q:\n%v", door, want, err)
					}
				}
			}
		})
	}

	t.Run("every refused entry is named", func(t *testing.T) {
		text := "bgp-ls-export {\n" + domainEntry("core", "rsvp-te", 7) + domainEntry("edge", "bgp-epe", 8) +
			domainEntry("west", "typo", 9) + "}\n"
		err := validateSections(t, text)
		if err == nil {
			t.Fatal("two unpublished sources accepted")
		}
		for _, want := range []string{"domain core source rsvp-te", "domain west source typo"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal does not name %q:\n%v", want, err)
			}
		}
		if strings.Contains(err.Error(), "domain edge") {
			t.Errorf("refusal names the published bgp-epe entry:\n%v", err)
		}
	})

	t.Run("published source accepted", func(t *testing.T) {
		for door, err := range doorErrors(t, domainText("edge", "bgp-epe")) {
			if err != nil {
				t.Errorf("%s refused bgp-epe, which this build publishes: %v", door, err)
			}
		}
	})

	// The IGP sources this build left out pass this check; their config need
	// refuses them with the build tag (TestDomainNamingACompiledOutSourceIsRefused).
	t.Run("compiled-out IGP left to its need", func(t *testing.T) {
		for _, source := range []string{isisSource, ospfSource} {
			if err := validateSections(t, domainText("core", source)); err != nil {
				t.Errorf("source %s refused as unknown, hiding its build tag: %v", source, err)
			}
		}
	})
}

// doorErrors runs text through the validate and commit door and through the
// boot and reload door, keyed by door.
func doorErrors(t *testing.T, text string) map[string]error {
	t.Helper()
	_, loadErr := config.LoadConfig(text, "ls-export-source.conf", nil)
	return map[string]error{
		"validate and commit (ValidateCustomSections)": validateSections(t, text),
		"boot and reload (LoadConfig)":                 loadErr,
	}
}

// validateSections returns the registered original-tree validators' verdict on
// text, failing the test when the walk reports anything else.
func validateSections(t *testing.T, text string) error {
	t.Helper()
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	failures, err := config.ValidateCustomSections(tree)
	if len(failures) != 0 {
		t.Fatalf("section failures for %q: %v", text, failures)
	}
	return err
}

func domainText(name, source string) string {
	return "bgp-ls-export {\n" + domainEntry(name, source, 7) + "}\n"
}

// domainEntry writes one `domain` entry carrying the leaves the exporter
// requires; instance keeps entries in one config distinct.
func domainEntry(name, source string, instance int) string {
	id := string(rune('0' + instance))
	return "\tdomain " + name + " {\n\t\tsource " + source + ";\n\t\tprotocol-id 2;\n\t\tnative-instance " + id +
		";\n\t\tnative-area 0;\n\t\tinstance-id " + id + ";\n\t}\n"
}
